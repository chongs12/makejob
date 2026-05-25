# MakeJob 后端可观测性优化报告

## 概述

本次优化分阶段补齐了 MakeJob 后端在链路追踪、日志、监控三大支柱上的缺口。目前已完成 Phase 1（请求链路追踪 + 日志增强）和 Phase 2（Prometheus Metrics）。

---

## Phase 1：请求链路追踪 + 日志增强

### 1.1 请求链路追踪

**目标：** 每个 HTTP 请求生成唯一 `request_id`，贯穿 handler → service → AI runtime → repository 全链路。

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/middleware/requestid.go` | 新建 | 生成/透传 `X-Request-ID`，注入 Gin context 和 std context |
| `internal/middleware/auth.go` | 修改 | Auth 中间件将 `user_id` 写入 std context，新增 `GetUserIDFromContext()` |
| `internal/middleware/logger.go` | 重写 | 从 context 读取 `request_id` 和 `user_id`，所有日志自动携带 |

**关键实现：**
- 优先使用客户端传入的 `X-Request-ID`，否则自动生成 UUID
- `request_id` 同时注入 Gin context（供中间件使用）和 `context.Context`（供 service/runtime 使用）
- `user_id` 在 Auth 中间件执行后写入 context，Logger 中间件在 `c.Next()` 之后读取

### 1.2 Recovery 中间件

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/middleware/recovery.go` | 新建 | 替换 `gin.Recovery()`，panic 日志走 zap 结构化管道 |

**关键实现：**
- panic 时记录完整 stack trace、request_id、user_id、method、path、client IP
- 返回标准 500 JSON 响应，不暴露内部错误详情
- 提供 `Recovery()` 和 `RecoveryWithWriter()` 两个版本

### 1.3 日志系统增强

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `pkg/logger/logger.go` | 修改 | 新增 JSON encoder、日志轮转（lumberjack） |
| `internal/config/config.go` | 修改 | 新增 `LoggingConfig` 配置段 |

**关键实现：**
- production 模式默认 JSON encoder，development 模式保留 Console encoder（带颜色）
- 日志文件轮转：默认 100MB/文件，保留 5 个旧文件，最长 30 天，自动压缩
- 配置项：`logging.level` / `logging.format` / `logging.output` / `logging.file_path` / `logging.max_size_mb` / `logging.max_backups` / `logging.max_days`

### 1.4 中间件注册顺序

```
RequestID → Logger → CORS → RateLimit → Recovery
```

Recovery 放最后，确保能捕获前面中间件的 panic。

---

## Phase 2：Prometheus Metrics

### 2.1 指标定义

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/metrics/collector.go` | 新建 | Prometheus 注册表 + 自定义指标 + Gin 中间件 |

**注册的指标：**

| 指标名 | 类型 | Labels | 说明 |
|--------|------|--------|------|
| `http_requests_total` | Counter | method, path, status | HTTP 请求总数 |
| `http_request_duration_seconds` | Histogram | method, path | HTTP 请求延迟分布 |
| `ai_calls_total` | Counter | scene, provider, model, status | AI 调用总数 |
| `ai_call_duration_seconds` | Histogram | scene, provider, model | AI 调用延迟分布 |
| `active_websocket_connections` | Gauge | — | 当前活跃 WebSocket 连接数 |

**附加：** Go runtime 指标（`collectors.NewGoCollector()`）和进程指标（`collectors.NewProcessCollector()`）。

**基数控制：** `NormalizePath()` 将动态路径段归一化（`/api/interview/123` → `/api/interview/:id`，UUID 同理），避免 label 基数爆炸。

### 2.2 HTTP Metrics 中间件

`GinMetricsMiddleware()` 在每个请求完成后自动记录：
- `http_requests_total`：method + 归一化 path + status code
- `http_request_duration_seconds`：请求耗时

### 2.3 AI 调用指标采集

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/ai/runtime/logger.go` | 修改 | `Record()` 方法末尾调用 `metrics.RecordAICall()` |

每次 AI 调用落库后同步递增 Prometheus 指标，包括 scene、provider、model、成功/失败状态和耗时。

### 2.4 健康检查端点

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/handler/health_handler.go` | 新建 | 健康检查 + Prometheus scrape 端点 |

| 端点 | 用途 | 行为 |
|------|------|------|
| `GET /api/health` | Liveness 探针 | 始终返回 200，携带 version 和 timestamp |
| `GET /api/health/ready` | Readiness 探针 | 检查 DB ping + Redis ping，任一失败返回 503 |
| `GET /metrics` | Prometheus scrape | 标准 Prometheus text format |

### 2.5 main.go 接入

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `cmd/server/main.go` | 修改 | 注册 metrics 中间件，挂载健康检查和 /metrics 端点 |

**最终中间件链：**
```
RequestID → Logger → Metrics → CORS → RateLimit → Recovery
```

---

## Phase 3：AI 日志补全 + 错误脱敏

### 3.1 CompanionAgent 接入调用日志

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/ai/runtime/companion_agent.go` | 新建 | 从 builder.go 拆出，新增 `logger` 字段 + `recordCall` |
| `internal/ai/runtime/builder.go` | 修改 | 改用 `newCompanionAgent()` 构造，注入 logger |

CompanionAgent 是唯一没有调用日志的 agent。改造后其 `Chat()` 方法每次 LLM 调用都会写入 `ai_call_logs` 表（source=`companion_runtime`）并递增 Prometheus 指标。

### 3.2 ResumeParser 接入调用日志

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/ai/runtime/resume_parser.go` | 修改 | 新增 `logger` 字段 + `recordCall`，Parse 方法记录调用日志 |
| `internal/ai/runtime/builder.go` | 修改 | `newResumeParser` 增加 logger 参数 |

### 3.3 Live2DDirector 补全 recordCall

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/ai/runtime/live2d_director.go` | 修改 | `GenerateDirective` 增加 `startedAt` + `recordCall` |

Live2DDirector 之前有 logger 字段但未使用，现已补全。

### 3.4 错误信息脱敏

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/common/response.go` | 修改 | `InternalError` 服务端记录完整错误日志，客户端只返回通用文案 |

改动前 `InternalError(c, "xxx: "+err.Error())` 会将数据库错误、AI provider 错误、API key 等内部信息直接暴露给客户端。改动后：
- 详细错误信息（含 method、path、IP、request_id）写入 zap 日志
- 客户端只收到 `"服务器内部错误"` 固定文案

---

## Phase 4：Token 用量采集

### 4.1 AIProvider 接口扩展

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/ai/provider.go` | 修改 | 新增 `ChatResponse` 和 `TokenUsage` 结构体，`Chat` 返回 `(*ChatResponse, error)` |

```go
type ChatResponse struct {
    Content      string
    InputTokens  int
    OutputTokens int
}
```

### 4.2 Eino Provider 提取 Token Usage

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/ai/eino/provider.go` | 修改 | 从 `resp.ResponseMeta.Usage` 提取 PromptTokens/CompletionTokens |

Eino SDK 底层已有完整的 token usage 数据（`schema.Message.ResponseMeta.Usage`），之前在 provider 层被丢弃，现已提取并返回。

### 4.3 全量 Chat 调用方适配

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/ai/runtime/structured_output.go` | 修改 | `callStructuredJSON` 返回值新增 `TokenUsage`，累计初始+修复请求的 token |
| `internal/ai/runtime/interview_agent.go` | 修改 | 3 个 `callStructuredJSON` 调用 + 6 个 `recordCall` 传入 token |
| `internal/ai/runtime/plan_agent.go` | 修改 | 2 个 `callStructuredJSON` + 1 个直接 Chat + 6 个 `recordCall` |
| `internal/ai/runtime/quiz_agent.go` | 修改 | 2 个 `callStructuredJSON` + 2 个直接 Chat + 7 个 `recordCall` |
| `internal/ai/runtime/companion_agent.go` | 修改 | 1 个直接 Chat + 1 个 `recordCall` |
| `internal/ai/runtime/resume_parser.go` | 修改 | 1 个 `callStructuredJSON` + 2 个 `recordCall` |
| `internal/ai/runtime/live2d_director.go` | 修改 | 1 个 `callStructuredJSON` + 2 个 `recordCall` |
| `internal/ai/runtime/builder.go` | 修改 | `namedProvider`、`providerWithFallback`、`unavailableProvider` 适配新签名 |
| `internal/ai/runtime/debugger.go` | 修改 | 适配 `*ChatResponse` 返回值 |
| `internal/ai/runtime/dynamic_client.go` | 修改 | `runtimeProvider.Chat` 适配新签名 |
| `internal/ai/runtime/structured_output_test.go` | 修改 | mock 适配新签名 |
| `internal/ai/runtime/quiz_agent_test.go` | 修改 | mock 适配新签名 |

### 4.4 Token 数据模型

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/model/ai_call_log.go` | 修改 | 新增 `InputTokens`、`OutputTokens` 字段 |
| `internal/ai/runtime/logger.go` | 修改 | `runtimeCallLogEntry` 新增 token 字段，`Record()` 写入 DB + Prometheus |

GORM AutoMigrate 会自动为 `ai_call_logs` 表新增 `input_tokens` 和 `output_tokens` 列。

### 4.5 Prometheus Token 指标

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/metrics/collector.go` | 修改 | 新增 `ai_tokens_total` Counter（labels: scene, direction） |

```go
ai_tokens_total{scene="interview", direction="input"}  1234
ai_tokens_total{scene="interview", direction="output"}  567
```

---

## 当前观测能力矩阵

| 维度 | 优化前 | 优化后 |
|------|--------|--------|
| 请求追踪 | 无 | `X-Request-ID` 贯穿全链路，日志自动关联 |
| 用户标识 | 日志中无 user_id | 认证请求日志自动携带 user_id |
| Panic 日志 | gin 默认输出到 stderr | zap 结构化日志 + request_id + stack trace |
| 日志格式 | 仅 console | production 默认 JSON，支持文件轮转 |
| HTTP 指标 | 无 | 请求计数、延迟分布、状态码统计 |
| AI 调用指标 | 仅 DB 日志（部分 agent） | 全量 agent Prometheus 计数/延迟 + DB 日志双重记录 |
| AI Token 用量 | 不可观测 | input/output token 写入 DB + Prometheus `ai_tokens_total` |
| WebSocket 连接 | 无观测 | Gauge 实时追踪活跃连接数 |
| 健康检查 | 仅简单 200 | Liveness + Readiness（DB/Redis 连通性） |
| Go 运行时 | 无 | goroutine、GC、内存等自动采集 |
| 错误信息 | 内部错误直接暴露给客户端 | 服务端日志记录详情，客户端只收到通用文案 |
| AI 日志覆盖 | Interview/Plan/Quiz 有日志，Companion/ResumeParser/Live2D 无日志 | 全量 6 个 agent 均有调用日志 |

---

## 实施状态

| 阶段 | 优先级 | 状态 | 说明 |
|------|--------|------|------|
| Phase 1: 请求链路追踪 + 日志增强 | P0 | ✅ 已完成 | request_id 贯通、Recovery 中间件、JSON 日志、日志轮转 |
| Phase 2: Prometheus Metrics | P0 | ✅ 已完成 | HTTP/AI/WebSocket 指标、健康检查、/metrics 端点 |
| Phase 3: AI 日志补全 + 错误脱敏 | P1 | ✅ 已完成 | 全量 agent 接入日志、错误信息脱敏 |
| Phase 4: Token 用量采集 | P1 | ✅ 已完成 | Provider 接口扩展、token 写入 DB + Prometheus |

---

## 配置参考

```yaml
# config.yaml 新增配置段
logging:
  level: info          # debug / info / warn / error
  format: json         # console / json
  output: file         # stdout / file
  file_path: ./logs/app.log
  max_size_mb: 100
  max_backups: 5
  max_days: 30
```

## 依赖新增

```
github.com/prometheus/client_golang    # Prometheus 客户端
gopkg.in/natefinch/lumberjack.v2       # 日志轮转
```

## 关键接口变更

`AIProvider.Chat()` 返回类型从 `(string, error)` 变更为 `(*ChatResponse, error)`，其中 `ChatResponse` 包含 `Content`、`InputTokens`、`OutputTokens`。此变更影响所有 AI provider 实现和调用方。
