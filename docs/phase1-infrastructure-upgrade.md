# Phase 1 基础设施改造说明文档

> 改造时间：2026-05-31
> 改造目标：补齐简历描述中的基础设施能力（OpenTelemetry、分布式限流、Grafana 监控）

---

## 一、改造背景

### 1.1 问题描述

项目简历中描述了以下能力，但实际代码中未实现：

| 简历描述 | 实际状态 | 本次改造 |
|----------|----------|----------|
| OpenTelemetry 全链路追踪 | ❌ 仅有自定义 traceID | ✅ 已实现 |
| 分布式限流 | ❌ 本地令牌桶限流 | ✅ 已实现 |
| Prometheus + Grafana 监控 | ⚠️ Prometheus 已有，无 Grafana | ✅ 已补充 |

### 1.2 改造原则

- **向后兼容**：所有新功能默认 `enabled: false`，不影响现有行为
- **渐进式改造**：新功能可选启用，支持配置开关
- **降级策略**：Redis 不可用时自动降级到本地限流

---

## 二、改造内容概览

### 2.1 三大模块

```
┌─────────────────────────────────────────────────────────────┐
│                    Phase 1 基础设施改造                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐│
│  │  OpenTelemetry│     │ 分布式限流    │     │ Grafana 监控 ││
│  │  全链路追踪   │     │ Redis 滑动窗口│     │ Dashboard    ││
│  └──────┬───────┘     └──────┬───────┘     └──────┬───────┘│
│         │                    │                    │        │
│         ▼                    ▼                    ▼        │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              配置层 (config.go + config.yaml)         │  │
│  │  - TelemetryConfig                                    │  │
│  │  - DistributedRateLimitConfig                         │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 文件变更统计

| 操作 | 数量 | 文件列表 |
|------|------|----------|
| 新增 | 10 | telemetry.go, tracing.go, lua_scripts.go, redis_limiter.go, distributed_ratelimit.go, docker-compose.yml, prometheus.yml, datasource.yml, dashboard.yml, makejob-overview.json |
| 修改 | 6 | config.go, config.yaml, main.go, logger.go, ai/runtime/logger.go, go.mod |

---

## 三、详细实现说明

### 3.1 OpenTelemetry 全链路追踪

#### 3.1.1 新增文件

**`backend/internal/telemetry/telemetry.go`**

```go
// 核心函数
func Init(ctx context.Context, cfg config.TelemetryConfig) (func(), error)
func GetTracer(name string) trace.Tracer
```

功能说明：
- 初始化 OTel TracerProvider
- 创建 OTLP gRPC exporter（连接 Jaeger/Tempo 等后端）
- 设置全局 Propagator（W3C TraceContext + Baggage）
- 返回 cleanup 函数用于优雅关闭
- 当 `cfg.Enabled = false` 时返回 noop，不创建任何 exporter

**`backend/internal/middleware/tracing.go`**

```go
// Gin 中间件
func Tracing() gin.HandlerFunc
```

功能说明：
- 从 HTTP Header 提取上游 trace context（支持分布式追踪链路传递）
- 为每个 HTTP 请求创建 span
- 记录 HTTP 方法、URL、客户端 IP 等属性
- 将 traceID 注入 Gin context，供日志中间件使用
- 请求结束后记录状态码，5xx 错误标记为 Error 状态

#### 3.1.2 修改文件

**`backend/internal/middleware/logger.go`**

新增 traceID 注入逻辑：
```go
// 从 context 注入的 OTel trace_id
if traceID, exists := c.Get("trace_id"); exists {
    if tid, ok := traceID.(string); ok && tid != "" {
        fields = append(fields, zap.String("trace_id", tid))
    }
}
```

修改位置：`Logger()` 和 `LoggerWithSkipPaths()` 两个函数中均已添加。

**`backend/internal/ai/runtime/logger.go`**

修改 traceID 生成逻辑，优先从 OTel span 获取：
```go
if traceID == "" {
    // 优先从 OTel span context 获取 traceID，实现 HTTP 请求与 AI 调用的链路关联
    spanCtx := trace.SpanFromContext(ctx).SpanContext()
    if spanCtx.HasTraceID() {
        traceID = spanCtx.TraceID().String()
    } else {
        traceID = uuid.NewString()
    }
}
```

效果：AI 调用日志中的 traceID 与 HTTP 请求的 traceID 关联，可在 Jaeger 中查看完整链路。

**`backend/cmd/server/main.go`**

新增 OTel 初始化：
```go
otelShutdown, otelErr := telemetry.Init(context.Background(), cfg.Telemetry)
if otelErr != nil {
    applogger.Warn("otel init failed, continuing without tracing", zap.Error(otelErr))
} else {
    defer otelShutdown()
}
```

中间件链调整：
```go
r.Use(middleware.RequestID())
r.Use(middleware.Tracing())        // 新增：在 Logger 之前
r.Use(middleware.Logger())
r.Use(metrics.GinMetricsMiddleware())
r.Use(middleware.CORS())
r.Use(middleware.DistributedRateLimit(rdb, cfg.DistributedRateLimit))  // 替换原 RateLimit()
r.Use(middleware.Recovery())
```

#### 3.1.3 链路追踪数据流

```
用户请求
    │
    ▼
┌─────────────────────────────────────────────────────────┐
│ Tracing 中间件                                          │
│ 1. 从 Header 提取 trace context                         │
│ 2. 创建 span (makejob-http)                             │
│ 3. 设置 span attributes (method, url, client_ip)        │
│ 4. 将 trace_id 存入 Gin context                         │
└─────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────┐
│ Logger 中间件                                           │
│ 1. 读取 trace_id                                        │
│ 2. 输出结构化日志（含 trace_id 字段）                     │
└─────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────┐
│ Handler → Service → AI Runtime                          │
│ 1. AI 调用时从 span context 获取 trace_id               │
│ 2. AI 调用日志关联同一 trace_id                          │
└─────────────────────────────────────────────────────────┘
    │
    ▼
Jaeger UI (http://localhost:16686) 查看完整链路
```

---

### 3.2 分布式限流（Redis 滑动窗口）

#### 3.2.1 新增文件

**`backend/internal/ratelimit/lua_scripts.go`**

Redis Lua 脚本实现滑动窗口限流：
```lua
-- 使用 ZSET 存储请求记录
-- ZREMRANGEBYSCORE 清理窗口外记录
-- ZADD 记录当前请求
-- ZCARD 统计窗口内请求数
```

脚本特点：
- 原子性操作（单个 Lua 脚本执行）
- 返回值：`{allowed(0/1), remaining, retry_after_ms}`
- 自动清理过期记录（EXPIRE）

**`backend/internal/ratelimit/redis_limiter.go`**

```go
// 核心结构体
type RedisLimiter struct {
    rdb    *redis.Client
    script *redis.Script
    rules  map[string]config.RateLimitRuleConfig
}

// 核心方法
func NewRedisLimiter(rdb *redis.Client, rules []config.RateLimitRuleConfig) *RedisLimiter
func (l *RedisLimiter) Allow(ctx context.Context, ruleName, identifier string) AllowResult
func (l *RedisLimiter) AllowByIP(ctx context.Context, ruleName, ip string) AllowResult
func (l *RedisLimiter) AllowByUserID(ctx context.Context, ruleName string, userID uint) AllowResult
func (l *RedisLimiter) GetRule(name string) (config.RateLimitRuleConfig, bool)
```

Redis Key 格式：`ratelimit:{rule_name}:{identifier}`

示例：
- `ratelimit:default:192.168.1.1` （按 IP 限流）
- `ratelimit:strict:uid:123` （按用户 ID 限流）

**`backend/internal/middleware/distributed_ratelimit.go`**

```go
// 中间件函数
func DistributedRateLimit(rdb *redis.Client, cfg config.DistributedRateLimitConfig) gin.HandlerFunc
func DistributedRateLimitByRule(rdb *redis.Client, cfg config.DistributedRateLimitConfig, ruleName string) gin.HandlerFunc
func StrictDistributedRateLimit(rdb *redis.Client, cfg config.DistributedRateLimitConfig) gin.HandlerFunc
func PublicDistributedRateLimit(rdb *redis.Client, cfg config.DistributedRateLimitConfig) gin.HandlerFunc
```

降级逻辑：
```
rdb == nil || !cfg.Enabled
    │
    ▼
fallbackLocalRateLimit()
    │
    ├─ 找到规则 → RateLimitWithParams(rule.Rate, rule.Capacity)
    │
    └─ 未找到规则 → RateLimit()（默认 100/s, 突发 200）
```

#### 3.2.2 限流响应格式

被限流时返回 HTTP 429：
```json
{
    "code": 429,
    "message": "请求过于频繁，请稍后再试"
}
```

响应头：
```
X-RateLimit-Limit: 200
X-RateLimit-Remaining: 0
Retry-After: 1
```

---

### 3.3 Grafana Dashboard

#### 3.3.1 新增目录结构

```
observability/
├── docker-compose.yml                          # 服务编排
├── prometheus/
│   └── prometheus.yml                          # Prometheus 配置
└── grafana/
    ├── provisioning/
    │   ├── datasources/
    │   │   └── datasource.yml                  # 数据源配置
    │   └── dashboards/
    │       └── dashboard.yml                   # Dashboard 自动加载
    └── dashboards/
        └── makejob-overview.json               # Dashboard 面板定义
```

#### 3.3.2 服务组件

| 服务 | 镜像 | 端口 | 用途 |
|------|------|------|------|
| Prometheus | prom/prometheus:v2.53.0 | 9090 | 指标采集与存储 |
| Grafana | grafana/grafana:11.1.0 | 3000 | 可视化 Dashboard |
| Jaeger | jaegertracing/all-in-one:1.57 | 16686, 4317 | 分布式追踪存储与查询 |

#### 3.3.3 Dashboard 面板

**HTTP 指标**
- HTTP Requests per Second：`sum(rate(http_requests_total[5m]))`
- HTTP Requests by Status Code：`sum by (status) (rate(http_requests_total[5m]))`
- HTTP Request Latency：P50/P95/P99 分位数

**AI 指标**
- AI Calls per Second：`sum(rate(ai_calls_total[5m]))`
- AI Call Success Rate：成功请求数 / 总请求数
- AI Token Usage：按 input/output 分类统计

**系统指标**
- Active WebSocket Connections：`active_websocket_connections`
- Go Goroutines：`go_goroutines`
- Memory Usage：`go_memstats_alloc_bytes`
- GC Pause Time：`rate(go_gc_duration_seconds_sum[5m]) / rate(go_gc_duration_seconds_count[5m])`

---

## 四、配置说明

### 4.1 配置结构变更

**`backend/internal/config/config.go`**

新增配置结构体：
```go
// TelemetryConfig OpenTelemetry 配置
type TelemetryConfig struct {
    Enabled     bool    `mapstructure:"enabled"`      // 是否启用
    Endpoint    string  `mapstructure:"endpoint"`      // OTLP gRPC 端点
    ServiceName string  `mapstructure:"service_name"`  // 服务名
    SampleRate  float64 `mapstructure:"sample_rate"`   // 采样率 0.0-1.0
}

// DistributedRateLimitConfig 分布式限流配置
type DistributedRateLimitConfig struct {
    Enabled       bool                  `mapstructure:"enabled"`        // 是否启用
    FallbackLocal bool                  `mapstructure:"fallback_local"` // 是否降级到本地限流
    Rules         []RateLimitRuleConfig `mapstructure:"rules"`          // 限流规则列表
}

// RateLimitRuleConfig 限流规则配置
type RateLimitRuleConfig struct {
    Name     string  `mapstructure:"name"`     // 规则名称
    Rate     float64 `mapstructure:"rate"`     // 每秒请求数
    Capacity int     `mapstructure:"capacity"` // 突发容量
    ByKey    string  `mapstructure:"by_key"`   // 限流维度：ip 或 user_id
}
```

### 4.2 配置文件变更

**`backend/config.yaml`**

新增配置段：
```yaml
telemetry:
  enabled: false                              # 设为 true 启用 OTel 追踪
  endpoint: "localhost:4317"                  # OTLP gRPC 收集器地址
  service_name: "makejob-backend"             # 服务名
  sample_rate: 1.0                            # 采样率

distributed_ratelimit:
  enabled: false                              # 设为 true 启用 Redis 分布式限流
  fallback_local: true                        # Redis 不可用时降级到本地限流
  rules:
    - name: "default"                         # 默认规则（全局中间件）
      rate: 100
      capacity: 200
      by_key: "ip"
    - name: "strict"                          # 严格规则（登录/注册）
      rate: 1
      capacity: 3
      by_key: "ip"
    - name: "public"                          # 公开接口规则
      rate: 20
      capacity: 50
      by_key: "ip"
```

### 4.3 环境变量覆盖

所有配置支持环境变量覆盖，格式：`SECTION_KEY`

示例：
- `TELEMETRY_ENABLED=true`
- `TELEMETRY_ENDPOINT=jaeger:4317`
- `DISTRIBUTED_RATELIMIT_ENABLED=true`

---

## 五、使用指南

### 5.1 启用 OpenTelemetry

**步骤 1：启动 Jaeger**
```bash
cd observability
docker compose up -d jaeger
```

**步骤 2：修改配置**
```yaml
# backend/config.yaml
telemetry:
  enabled: true
  endpoint: "localhost:4317"
```

**步骤 3：重启后端服务**

**步骤 4：访问 Jaeger UI**
- 地址：http://localhost:16686
- 服务名：makejob-backend
- 查看 HTTP 请求链路和 AI 调用关联

### 5.2 启用分布式限流

**步骤 1：确保 Redis 可用**

**步骤 2：修改配置**
```yaml
# backend/config.yaml
distributed_ratelimit:
  enabled: true
  fallback_local: true
```

**步骤 3：重启后端服务**

**步骤 4：验证限流**
```bash
# 压测触发限流
wrk -t12 -c400 -d30s http://localhost:8082/api/health

# 检查响应头
curl -I http://localhost:8082/api/health
# X-RateLimit-Limit: 200
# X-RateLimit-Remaining: 199
```

### 5.3 启动 Grafana Dashboard

**步骤 1：启动监控栈**
```bash
cd observability
docker compose up -d
```

**步骤 2：访问 Grafana**
- 地址：http://localhost:3000
- 用户名：admin
- 密码：makejob123

**步骤 3：查看 Dashboard**
- 自动加载 "MakeJob Backend Overview"
- 包含 HTTP、AI、WebSocket、系统指标面板

---

## 六、技术细节

### 6.1 依赖版本

| 依赖 | 版本 | 用途 |
|------|------|------|
| go.opentelemetry.io/otel | v1.44.0 | OTel 核心 API |
| go.opentelemetry.io/otel/trace | v1.44.0 | Trace API |
| go.opentelemetry.io/otel/sdk | v1.44.0 | OTel SDK |
| go.opentelemetry.io/otel/exporters/otlp/otlptrace | v1.44.0 | OTLP exporter |
| go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc | v1.44.0 | gRPC exporter |

### 6.2 中间件执行顺序

```
RequestID → Tracing → Logger → Metrics → CORS → DistributedRateLimit → Recovery
```

- **Tracing** 必须在 **Logger** 之前，确保 traceID 可被日志读取
- **DistributedRateLimit** 替换了原有的 **RateLimit**

### 6.3 降级策略

| 场景 | 行为 |
|------|------|
| `telemetry.enabled = false` | OTel 初始化返回 noop，无任何开销 |
| OTel exporter 连接失败 | 服务正常启动，日志 warn |
| `distributed_ratelimit.enabled = false` | 使用本地令牌桶限流 |
| Redis 连接失败 + `fallback_local = true` | 降级到本地限流，日志 warn |
| Redis 连接失败 + `fallback_local = false` | 所有请求放行（不限流） |

### 6.4 Redis Key 设计

限流相关 Key：
```
ratelimit:{rule_name}:{identifier}
```

示例：
- `ratelimit:default:192.168.1.1` （默认规则，按 IP）
- `ratelimit:strict:uid:123` （严格规则，按用户 ID）

数据结构：ZSET
- Member：`{timestamp}-{random}` （确保唯一）
- Score：请求时间戳（微秒）

TTL：等于窗口大小（秒）

---

## 七、后续工作

### 7.1 Phase 2：架构改造层（待实施）

1. **Proto 定义 + gRPC 接口抽象**
   - 定义 InterviewService、UserService 等 proto 文件
   - 生成 Go 代码

2. **Kratos 框架集成**
   - 将单体应用改造为 Kratos App
   - 支持 gRPC + HTTP 双协议

3. **核心服务拆分**
   - InterviewService 拆分为独立 gRPC 服务
   - 主服务通过 gRPC 调用

### 7.2 优化建议

1. **RedisLimiter 单元测试**
   - 测试滑动窗口逻辑
   - 测试降级策略

2. **Tracing 中间件优化**
   - 路由模式归一化（避免高基数 span name）
   - 添加请求体大小、响应体大小等属性

3. **Grafana Dashboard 增强**
   - 添加告警规则
   - 添加业务指标面板（面试完成率、题目正确率等）

---

## 八、问题排查

### 8.1 OTel 连接失败

**症状**：日志输出 `otel init failed`

**排查**：
1. 检查 Jaeger 是否启动：`docker ps | grep jaeger`
2. 检查端口是否可达：`telnet localhost 4317`
3. 检查配置是否正确：`cat config.yaml | grep -A 4 telemetry`

### 8.2 限流不生效

**症状**：请求未被限流

**排查**：
1. 检查配置：`distributed_ratelimit.enabled = true`
2. 检查 Redis 连接：`redis-cli ping`
3. 检查 Redis Key：`redis-cli keys "ratelimit:*"`

### 8.3 Dashboard 无数据

**症状**：Grafana 面板显示 No data

**排查**：
1. 检查 Prometheus 是否采集：http://localhost:9090/targets
2. 检查后端 `/metrics` 端点：`curl http://localhost:8082/metrics`
3. 检查 Prometheus 配置：`cat observability/prometheus/prometheus.yml`

---

## 九、参考资料

- [OpenTelemetry Go 文档](https://opentelemetry.io/docs/languages/go/)
- [Redis 滑动窗口限流](https://redis.io/docs/latest/develop/interact/programmability/eval-intro/)
- [Grafana Dashboard JSON 模型](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/view-dashboard-json-model/)
- [Prometheus 查询语言 PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/)
