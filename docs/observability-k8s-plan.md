# MakeJob 可观测性 + pprof + K8s 部署 实施方案

> 本文档供多智能体交叉评审使用。请评审者在阅读后给出：设计缺陷、遗漏风险、更优替代方案、工作量误判。

---

## 一、项目背景

### 1.1 项目概况

MakeJob 是 AI 驱动的求职面试准备平台，已从单体架构完整迁移为 15 个 Kratos 微服务。

- 后端：Go 1.25 + Kratos v2.8.3 + gRPC v1.81.1 + Protobuf
- 前端：React 19 + Vite + TanStack Router/Query + Ant Design + Live2D + Monaco Editor
- AI：火山引擎 Ark（豆包 LLM）+ cloudwego/eino + RAG（Milvus 向量检索）
- 基础设施：PostgreSQL + Redis + RabbitMQ(amqp091-go v1.10.0) + etcd + MinIO + Piston 代码沙箱

### 1.2 15 个微服务清单

| 服务 | gRPC 端口 | HTTP 端口 | 有 MQ | 有出站 gRPC 调用 | 说明 |
|------|-----------|-----------|:------:|:-----------------:|------|
| gateway | - | 8082 | 否 | 是（全部下游） | Gin HTTP 入口，非 Kratos app |
| user | 9101 | 8001 | 否 | 否 | 注册/登录/Token 黑名单 |
| membership | 9002 | 8002 | 否 | 否 | 会员/支付 |
| question | 9003 | 8003 | 是 | 是（AI Gateway, CodeRunner） | 题库 22 RPC |
| interview | 9004 | 8004 | 是 | 是（AI Gateway, RAG, Archive, Membership, CodeRunner） | 面试全流程 |
| realtime | 9005 | 8085(WS) | 否 | 是（Interview, RAG） | 实时语音 WS 中继 |
| growth | 9006 | 8006 | 否 | 是（Archive, Interview, Plan, Question） | 只读聚合 |
| plan | 9007 | 8007 | 是 | 是（AI Gateway, Archive, Interview） | 学习计划 |
| companion | 9008 | 8008 | 否 | 是（AI Gateway, Growth, Interview, Plan） | AI 陪伴 |
| community | 9009 | 8009 | 否 | 否 | 帖子/评论 |
| learning_archive | 9010 | 8010 | 是 | 否 | 学习归档枢纽 |
| ai_gateway | 9011 | 8011 | 否 | 否（纯 gRPC 服务端，出站为 HTTP 调 Ark） | LLM 统一入口 25+ RPC |
| rag | 9012 | 8012 | 是 | 否（Milvus 客户端） | 向量检索 |
| coderunner | 9013 | - | 否 | 否（HTTP 调 Piston） | 代码沙箱 |
| admin | 9014 | 8014 | 是 | 是（全部下游） | 管理后台 BFF |

### 1.3 核心链路（已核实调用方向）

```
同步链路：
  HTTP Gateway(Gin) ──► Interview ──► AI Gateway ──► 火山 Ark(HTTP)
                      │     │            ▲
                      │     │            └─ 纯 gRPC 服务端，零 grpc.Dial
                      │     ├──► LearningArchive（WriteEntry 同步 gRPC）
                      │     ├──► RAG（Retrieve 同步 gRPC）
                      │     ├──► Membership（CheckFeatureAccess）
                      │     └──► CodeRunner（Execute 同步 gRPC）

异步链路：
  Interview ──interview.finished──► LearningArchive（报告完成 -> 写档案）
  Question  ──rag.sync.question──► RAG（向量索引同步）
  Plan      ──plan.generate──► Plan 自消费
  Plan      ──plan.feedback.diagnosis──► Plan 自消费
```

### 1.4 现有可观测性资产

| 资产 | 位置 | 状态 |
|------|------|------|
| Jaeger OTLP 收集器 | `observability/docker-compose.yml` | 已运行，:4317 OTLP gRPC 开启 |
| Prometheus | `observability/docker-compose.yml` | 已运行，但只抓取 gateway:8082 |
| Grafana + 预置仪表盘 | `observability/grafana/dashboards/makejob-overview.json` | 已运行，10 个面板，引用业务指标但代码中未定义 |
| OpenTelemetry 依赖 | go.mod（全部 indirect） | v1.44.0，otelgrpc v0.59.0，未被活代码引用 |
| prometheus/client_golang | go.mod（direct） | v1.23.2，仅 gateway 使用 |
| go-grpc-prometheus | go.mod（indirect） | v1.2.0，未使用 |
| 单体时代参考实现 | `docs/backend/internal/telemetry/`（.gitignore 排除） | Gin 版 OTLP + TracerProvider，可参考思路 |
| Gateway /metrics 端点 | `handler.go:2196` | 仅 Go runtime 默认指标 |
| Gateway /api/health | `handler.go:2193` | liveness + readiness 已实现 |

---

## 二、任务目标

### 任务 1：OpenTelemetry 可观测性

- **Tracing**：gRPC 跨服务传播、RabbitMQ 异步 trace 传递、AI/RAG 手动埋点
- **Metrics**：gRPC 指标（Kratos middleware + Prometheus exporter）、AI 调用指标
- **结构化日志**：注入 trace_id
- **优先链路**：HTTP Gateway -> Interview ->（AI Gateway -> Ark HTTP）+ RAG + LearningArchive

### 任务 2：Go pprof

- 独立端口（6060）暴露，不挂业务 Server
- K8s 中通过 port-forward 或 ClusterIP Service 访问

### 任务 3：K8s 部署

- kind 本地集群 + Helm Chart 标准化交付
- Deployment（探针、资源限制、优雅停机）、HPA、Ingress、ConfigMap/Secret
- 可观测性栈（OTel Collector + Jaeger + Prometheus + Grafana）部署在同一集群

---

## 三、兼容性分析

### 3.1 框架层兼容性

| 维度 | 现状 | 兼容性 | 说明 |
|------|------|--------|------|
| gRPC 拦截器 | 标准 `grpc.UnaryServerInterceptor` 数组 | 高 | 直接追加 otelgrpc + prometheus 拦截器 |
| OTel 依赖 | go.mod 全部 indirect v1.44.0 | 高 | 提升为 direct 即可 |
| Jaeger OTLP | 已运行 :4317 | 高 | 零配置对接 |
| 优雅停机 | Kratos 内置 SIGTERM 处理 | 高 | 与 K8s preStop 天然兼容 |
| 配置注入 | Bootstrap struct + yaml 手动反序列化 | 高 | 加 Telemetry 字段即可 |

### 3.2 需要改造的 5 个关键点

#### 坑 1：14 个服务无 HTTP server

当前 14 个业务微服务只有 gRPC transport。没有 `/metrics`、`/healthz`、`/debug/pprof` 端点。

**方案**：创建 `pkg/telemetry/http_server.go`，提供独立 `net/http.Server`（:6060），各服务在 main.go 中一行代码启动。

#### 坑 2：gRPC 拦截器链顺序

当前链：`Recovery -> Logging -> Auth`

**必须改为**：`otelgrpc -> prometheus -> Recovery -> Logging -> Auth`

**原因**：Kratos 的 `tracing.TraceID()` 是 `log.Valuer`，在日志求值时从 context 取 span。如果 otel 在 Logging 之后，span 还没进 ctx，日志拿不到 trace_id。

#### 坑 3：gRPC 客户端无 trace 传播

当前各服务 `grpc.Dial` 只有 `insecure` credentials（companion 刚加了 `ForwardTokenClientInterceptor`）。

**方案**：创建 `pkg/middleware/dial_options.go` 提供 `CommonDialOptions()`，统一 `[otelgrpc client, ForwardToken, insecure]`。

#### 坑 4：MQ trace context 完全断裂

`TaskMessage` 无 trace 字段；`publisher.go` 不注入 traceparent；`consumer.go` 用 `Start(ctx)` 的共享 ctx 跑所有消息的 handler，无法做 per-message 传播。

**方案**：
- Publisher：用 `amqp.Publishing.Headers`（`amqp.Table`）作为 W3C carrier，`propagator.Inject(ctx, carrier)` 注入。不改 `TaskMessage` JSON schema。
- Consumer：`propagator.Extract(ctx, carrier)` 提取，为每条消息创建 consumer span，handler 用派生的 `msgCtx`。
- 新增 `pkg/mq/propagator.go` 实现 `propagation.TextMapCarrier` 接口适配 `amqp.Table`。

#### 坑 5：14 处复制粘贴的 grpc.go

每个服务的 `internal/server/grpc.go` 结构几乎完全相同（拦截器链、地址配置、超时），只有 service 注册不同。

**方案**：借 OTel 接入之机，抽取 `pkg/server/grpc.go` 提供统一 `NewGRPCServer(addr, timeout, authInterceptor, register, extraOpts...)`（签名见决策 6），一次性替换 14 处复制粘贴。同时解决拦截器顺序、metrics 注册、优雅停机三个问题。

### 3.3 服务分级改造清单

**A 类 - 核心链路，需业务级手动埋点（6 个）**：

| 服务 | 改动点 |
|------|--------|
| interview | 3 个出站客户端加 client interceptor；MQ interview.finished 发布端注入 trace；GenerateReport 手动 span；异步 goroutine（SubmitAnswer -> WriteEntry，当前用 context.Background()）需显式 span |
| ai_gateway | ark_client.go 火山 Ark HTTP 调用手动埋点（LLM span + token 计数）；openai_client.go fallback 同理；注意 context.WithoutCancel（:208）保留 trace values 需验证 |
| learning_archive | MQ consumer handleInterviewFinished 的 trace 提取（per-message ctx）；WriteEntry 服务端 span |
| rag | 服务端 span + Milvus 检索 span；MQ consumer rag.sync.question trace 提取 |
| gateway | Gin otelgin 中间件（HTTP span + 传播）；补业务 metrics（http_requests_total / http_request_duration_seconds）；修优雅停机 |
| realtime | WS 握手时从 HTTP header 提取 traceparent；WS 消息内传 traceparent；gRPC 客户端正常接；注册 active_websocket_connections Gauge |

**B 类 - 标准接入，只改 main.go + grpc.go（9 个）**：

user、membership、plan、companion、admin、community、growth、question、coderunner。统一加 telemetry.Init + otelgrpc interceptor + metrics 端口 + pprof。无业务级埋点。

### 3.4 K8s 落地的隐性工作量

| 差距 | 现状 | 工作量 |
|------|------|--------|
| 镜像 | 0 个 Dockerfile | 15 个 Go 服务 + 前端 nginx 镜像 |
| 配置外置 | 15 个 config.yaml 硬编码 localhost | ConfigMap 挂载 config.yaml，加载机制零改动；K8s 用 service DNS peer 地址 |
| 中间件 | PostgreSQL/Redis/Piston 不在 compose 里 | 需容器化或 external IP |
| 本地可观测性栈 | observability/docker-compose.yml 中 Jaeger 直连 :4317 | 需加 OTel Collector 容器，Jaeger 改从 Collector 收数据（不再直连 :4317） |
| Ingress | 无 | 单 Ingress：/api/v1(含 WS) -> gateway，/ -> 前端 |
| 优雅停机 | Kratos 14 服务隐式有；Gateway 没有 | Gateway 需补 |
| Helm | 空 | 从零写 |

---

## 四、架构设计决策

### 决策 1：OTel SDK 直接用，不引入 Kratos contrib

**选择**：直接用 OTel SDK + otelgrpc 拦截器 + prometheus client_golang
**不选**：kratos/contrib/metrics/prometheus、tracing/jaeger
**理由**：contrib 不在依赖图中且 jaeger contrib 是过时思路。直接用 OTel SDK 符合行业标准，依赖最少。

### 决策 2：trace 与 metrics 管线拆分

**选择**：
- Traces -> OTLP -> OTel Collector -> Jaeger
- Metrics -> 各服务 /metrics 端点 -> Prometheus 直接 scrape（不走 Collector）

**不选**：服务直连 Jaeger（无 Collector 汇聚）；Metrics 走 Collector 导出
**理由**：Prometheus 原生 pull 模型与现有 promhttp 资产兼容；Collector 职责单一化（trace 汇聚）。demo 用 AlwaysOn 全量 trace，Collector 的 sampling 作为运维展示点配置好但默认关闭。

### 决策 3：路径 A - otelgrpc 拦截器直接塞入现有链

**选择**：`otelgrpc.UnaryServerInterceptor` 直接加入 `kratosgrpc.UnaryInterceptor(...)` 数组
**不选**：路径 B - 转为 Kratos middleware 链（`kratosgrpc.Middleware(middleware.Tracing(), ...)`)
**理由**：路径 B 需把 14 个服务的 `pkg/middleware` raw interceptor 全部改写成 kratos middleware，属于为接 OTel 重构中间件体系，风险不成比例。

### 决策 4：MQ trace 用 AMQP Headers，不用 TaskMessage 字段

**选择**：`amqp.Publishing.Headers` 作为 W3C traceparent carrier
**不选**：在 `TaskMessage` struct 加 `TraceParent` 字段
**理由**：不修改 JSON schema，不影响序列化/反序列化代码，AMQP Headers 天然是 carrier。

### 决策 5：pprof + metrics + health 共用 :6060 HTTP server

**选择**：一个 `net/http.Server` 挂载 `/metrics`、`/healthz`、`/readyz`、`/debug/pprof/`
**不选**：pprof 独立端口
**理由**：减少端口管理复杂度。K8s 中每 Pod 各占 :6060 不冲突；本地 dev 用端口偏移（6060+序号）。

### 决策 6：抽 pkg/server 统一 NewGRPCServer（签名参数化）

**选择**：抽取 `pkg/server/grpc.go`，14 处复制粘贴替换为统一构造函数，签名参数化处理三类差异：

```go
func NewGRPCServer(
    addr string,
    timeout time.Duration, // 0 或负值 → 用默认（如 10s），learning_archive 未配 timeout 时兜底
    authInterceptor grpc.UnaryServerInterceptor, // nil = 无 auth（rag/coderunner）
    register func(*grpc.Server),
    extraOpts ...grpc.ServerOption,
) *kratosgrpc.Server
```

- Auth 差异：`authInterceptor` 为 nil 表示无 auth（rag/coderunner）；user 的 blacklist 变体由各服务自行构造 `auth.NewInterceptor(secret, WithBlacklist(...)).UnaryServerInterceptor()` 传入；**不要**用 `extraOpts` 注入 auth（grpc.ServerOption 只会 append 到链尾，导致双重 auth）
- Timeout 差异：从参数传入，各服务 config.yaml 中配置（与现有 6 个服务的 if/else 行为一致：config 优先、缺省兜底）
- 自定义 ServerOption：通过 `extraOpts` 透传

**理由**：14 个服务的 grpc.go 几乎完全相同（只有 service 注册不同）。统一后：拦截器顺序一处控制、metrics（grpc_prometheus.Register）一处注册、新增服务不用复制粘贴。签名参数化处理 Auth/Timeout/自定义 ServerOption 三类差异，避免过度抽象。

### 决策 7：Helm 用通用模板 + range 遍历（95% 统一 + 5% overrides）

**选择**：`_deployment.tpl` 模板 + values.yaml 中服务列表 + `range` 生成；values.yaml 用 `overrides` map 做例外覆盖

```gotemplate
{{- range $svc, $cfg := .Values.services }}
  ...
  {{- if $cfg.ingress.websocket }}
  # WS 专用注解/端口
  {{- end }}
{{- end }}
```

模板 95% 统一 range + 5% 例外覆盖（gateway WS 注解、realtime WS 端口）。

> **coderunner 说明**：coderunner 缺业务 HTTP Service 端口（1.2 表中 HTTP 端口为 "-"），但 :6060 telemetry 端口和探针照常，与其他 14 个服务一致，不列入例外清单。

**不选**：每个服务单独写 Deployment YAML
**理由**：15 套重复 YAML 不可维护。range + overrides 兼顾统一性与例外定制能力。

### 决策 8：配置外置用 ConfigMap/Secret 挂载 config.yaml

**选择**：K8s 用 ConfigMap 挂载到 -conf flag 指向的 config.yaml 路径，加载机制零改动；Secret（DB密码/JWT密钥/ARK API Key）单独挂载为文件，config.yaml 引用文件路径
**不选**：env 覆盖（需为每个服务实现 env 读取逻辑，代码量大）；完全改用 Kratos config source（env/consul/etcd）
**理由**：本地用仓库里的 config.yaml，K8s 用挂载的 ConfigMap（内容不同：peer 地址是 service DNS）。加载机制零改动，省掉整个阶段 4.1 的 env 覆盖代码量。

### 补充：关键实现细节约束

以下细节在落地时必须遵守，否则可观测性数据不完整或语义错误：

1. **telemetry.Init 必须显式设 globals**：`otel.SetTracerProvider(tp)` + `otel.SetTextMapPropagator(propagation.TraceContext{})`（含 Baggage）+ `otel.SetMeterProvider(mp)`，否则 otelgrpc 使用 noop provider，trace/metrics 静默丢失。

2. **每个服务的 resource 必须带 service.name**（如 `makejob.interview`、`makejob.ai_gateway`），Jaeger 才能按服务分组。在 `pkg/telemetry/tracer.go` 中通过 `resource.NewWithAttributes(semconv.ServiceNameKey, svcName)` 统一设置。

3. **前端 JS 无 OTel 埋点**，trace root 在 gateway（otelgin 创建 root span），这是预期行为。前端不注入 traceparent，gateway 是 W3C trace 的起点。

4. **Consumer 重试共享同一 consume span**：重试用 span events 记录（attempt/delay/err），不创建新 span；但 handler 内部的 gRPC 子调用仍建 child span。避免重试产生多条独立 trace。

5. **go-grpc-prometheus v1.2.0**（已在 go.mod indirect）用于 gRPC 指标拦截器（`grpc_prometheus.UnaryServerInterceptor`）；AI/HTTP 业务指标用 client_golang 自写（`prometheus.NewCounter` / `prometheus.NewHistogram`）。

---

## 五、实施计划（5 阶段 20 任务）

### 阶段 1：pkg 层基础设施（无业务代码依赖）

| # | 任务 | 产出文件 | 依赖 |
|---|------|---------|------|
| 1.1 | 创建 `pkg/telemetry/` 包 | tracer.go, meter.go, http_server.go, telemetry.go | 无 |
| 1.2 | 创建 gRPC 追踪+指标拦截器 | pkg/middleware/tracing.go, metrics.go | 无 |
| 1.3 | 统一 gRPC 客户端 Dial 选项 | pkg/middleware/dial_options.go | 1.2 |
| 1.4 | 抽取 pkg/server 统一 NewGRPCServer（签名参数化） | pkg/server/grpc.go | 1.2 |
| 1.5 | MQ trace context 传递 | 改 publisher.go, consumer.go + 新增 propagator.go | 1.1 |
| 1.6 | 结构化日志注入 trace_id | 改 logging.go + logger/ | 1.2 |

**拦截器链最终顺序**：`otelgrpc(UnaryServer) -> prometheus -> Recovery -> Logging -> Auth`

### 阶段 2：各微服务接入（依赖阶段 1）

| # | 任务 | 涉及服务 | 改动量 |
|---|------|---------|--------|
| 2.1 | gRPC server 拦截器链扩展 | 14 个 Kratos 服务 | 每个服务替换为 pkg/server.NewGRPCServer 或手动加 2 行 |
| 2.2 | gRPC client Dial 替换 | 8 个有跨服务调用的服务 | 每个客户端 grpc.Dial 替换为 CommonDialOptions |
| 2.3 | 各服务 main.go 接入 telemetry | 全部 15 个服务 | Init + defer cleanup + HTTP server goroutine |
| 2.4 | Gateway 接入 | gateway | otelgin 中间件 + 优雅停机 + 业务 metrics（http_requests_total / http_request_duration_seconds，不含 active_websocket_connections） |
| 2.5 | config.yaml 增加 telemetry 配置 | 全部 15 个服务 | telemetry 段 + conf.go struct |
| 2.6 | realtime WebSocket trace 传播 | realtime | WS 握手 traceparent + 消息头传播 |

> **说明**：`active_websocket_connections` 指标归 realtime 服务负责（WS 连接数由 realtime 管理），gateway 不暴露此指标。realtime 服务需在 WS 连接建立/断开时更新 Gauge。

### 阶段 3：AI/RAG 手动埋点（依赖阶段 2）

| # | 任务 | 涉及服务 | 埋点内容 |
|---|------|---------|---------|
| 3.1 | AI Gateway LLM 调用 span | ai_gateway | ark_client.go Chat() span(model/tokens/latency) + openai_client.go |
| 3.2 | AI Gateway 调用指标 | ai_gateway | ai_calls_total + ai_tokens_total + ai_call_duration_seconds |
| 3.3 | RAG 检索 span | rag | Retrieve() span(query/topk/results) + Milvus span |
| 3.4 | Interview 报告生成 span | interview | GenerateReport() span(type/id) + 异步 goroutine 显式 span |

### 阶段 4：容器化 + 配置外置（依赖阶段 2，与阶段 3 可并行）

| # | 任务 | 产出 | 说明 |
|---|------|------|------|
| 4.1 | 配置外置（ConfigMap 挂载） | K8s ConfigMap + Secret YAML | ConfigMap 挂载 config.yaml（-conf flag 路径不变）；Secret 挂载 DB密码/JWT密钥/ARK API Key 为文件，config.yaml 引用文件路径 |
| 4.2 | 统一多阶段 Dockerfile | Dockerfile（根目录） | CGO_ENABLED=0, distroless/alpine, ARG SERVICE_NAME |
| 4.3 | 前端 Dockerfile | frontend-react/Dockerfile + nginx.conf | node build -> nginx 静态 + 反代 /api/v1 |
| 4.4 | Makefile 补全 | 改 Makefile | 补 8 个缺失服务 + docker-build 目标 |
| 4.5 | docker-compose 全栈编排 | 改 docker-compose.yml | 15 服务 + 前端 + 基础设施 + 可观测性 |

### 阶段 5：K8s 部署（依赖阶段 4）

| # | 任务 | 产出 | 说明 |
|---|------|------|------|
| 5.1 | kind 集群 | deploy/kind-cluster.yaml + setup.sh | kind + ingress-nginx + image load + metrics-server（--kubelet-insecure-tls） |
| 5.2 | Helm Chart 骨架 | deploy/helm/makejob/ | Chart.yaml + values.yaml + _helpers.tpl |
| 5.3 | 通用 Deployment 模板 | templates/deployment.yaml | range 遍历 + 探针 + 资源限制 + 优雅停机 |
| 5.4 | Service + Ingress | templates/service.yaml + ingress.yaml | ClusterIP + 单 Ingress(含 WS) |
| 5.5 | ConfigMap + Secret | templates/configmap.yaml + secret.yaml | 非敏感 ConfigMap + 敏感 Secret |
| 5.6 | HPA | templates/hpa.yaml | gateway/companion/ai_gateway/interview；Deployment 必须设置 resources.requests.cpu（否则 HPA 计算百分比无基准） |
| 5.7 | 可观测性栈 | charts/observability/ + collector.yaml | OTel Collector（只导出 trace 到 Jaeger，不导出 metrics）+ Jaeger + Prometheus（直接 scrape）+ Grafana |
| 5.8 | Prometheus 服务发现 | 改 prometheus 配置 | static_configs -> kubernetes_sd_configs |

**阶段 5.7 补充 - collector.yaml 交付物**：

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: :4317

processors:
  batch: {}
  # tail_sampling 配置好但默认关闭（demo 用 AlwaysOn 全量 trace）
  # tail_sampling:
  #   decision_wait: 10s
  #   policies: [...]

exporters:
  otlp/jaeger:          # 导出 trace 到 Jaeger
    endpoint: jaeger:4317
    tls:
      insecure: true
  debug:                # 本地调试用
    verbosity: basic
```

- Collector 只导出 trace 到 Jaeger，**不导出 metrics**（Prometheus 直接 scrape 各服务 /metrics）
- sampling 作为运维展示点配置好但默认关闭，demo 走 AlwaysOn 全量 trace

---

## 六、关键风险点

| # | 风险 | 严重度 | 缓解措施 |
|---|------|--------|---------|
| 1 | 拦截器顺序错误导致日志无 trace_id | 高 | otelgrpc 必须在最外层；阶段 1 统一在 pkg/server 中控制顺序 |
| 2 | MQ consumer 共享 ctx 导致 per-message trace 断链 | 高 | consumer.go 改为 per-message 派生 ctx；影响 6 个消费服务需回归 |
| 3 | 14 处复制粘贴，批量改易漏 | 中 | 抽 pkg/server 统一构造函数，一处改全部生效 |
| 4 | 异步 goroutine（SubmitAnswer -> WriteEntry）用 context.Background() 断链 | 中 | 用 otel.Start(ctx) 显式 span |
| 5 | Gateway 无优雅停机，K8s 滚动更新硬杀 | 中 | 阶段 2.4 改为 http.Server + Shutdown |
| 6 | realtime WS 无法用 gRPC metadata 传播 trace | 中 | WS 消息头传 traceparent |
| 7 | Grafana 仪表盘引用的业务指标无代码定义 | 中 | 阶段 3.2 从归档单体移植 metrics 定义到各服务 |
| 8 | 8 个服务超时行为变更（user/membership/learning_archive/rag/realtime/growth/community/coderunner 当前跑在 Kratos 1s 默认超时下，config.yaml 中 timeout 从未生效，pkg/server 统一后变为配置值，是行为变更不是无感重构） | 中 | 阶段 2 验收需确认这 8 个服务超时生效且不引入新问题；learning_archive 需补 timeout 字段 |
| 9 | K8s 配置外置工作量 | 低 | 改用 ConfigMap 挂载 config.yaml，加载机制零改动；阶段 4.1 只需生成 ConfigMap/Secret YAML |
| 10 | ai_gateway context.WithoutCancel 是否保留 trace values | 低 | 落地时验证；WithoutCancel 保留 values 但取消 timeout/cancel |
| 11 | 本地 dev pprof 端口冲突 | 低 | 6060+服务序号偏移或仅 K8s 内启用 |

---

## 七、数据流设计

### 7.1 Trace 传播路径

```
用户 HTTP 请求
  -> Gateway(Gin): otelgin.Middleware 创建 root span，注入 traceparent 到 gRPC metadata
    -> Interview(gRPC server): otelgrpc.UnaryServerInterceptor 提取 traceparent，创建 server span
      -> AI Gateway(gRPC server): otelgrpc 提取，创建 server span
        -> Ark(HTTP): 手动 span，记录 model/tokens
      -> RAG(gRPC server): otelgrpc 提取，创建 server span
        -> Milvus: 手动 span 或 Milvus 自带 OTel
      -> LearningArchive(gRPC server): otelgrpc 提取，创建 server span
      -> MQ publish: propagator.Inject(ctx, amqp.Headers)
        -> MQ consume: propagator.Extract(ctx, amqp.Headers), 创建 consumer span
          -> LearningArchive handler: 在 consumer span 下执行
```

### 7.2 Metrics 流

```
各服务 gRPC server: grpc_requests_total + grpc_request_duration_seconds
  -> /metrics 端点(:6060)
    -> Prometheus 直接 scrape（kubernetes_sd_configs，不走 Collector）

AI Gateway: ai_calls_total + ai_tokens_total + ai_call_duration_seconds
  -> /metrics 端点(:6060)
    -> Prometheus 直接 scrape

Gateway: http_requests_total + http_request_duration_seconds
  -> /metrics 端点(:8082)
    -> Prometheus 直接 scrape

realtime: active_websocket_connections
  -> /metrics 端点(:6060)
    -> Prometheus 直接 scrape
```

### 7.3 日志流

```
各服务 gRPC handler:
  otelgrpc 拦截器将 span 注入 ctx（最外层）
    -> Logging 拦截器从 ctx 取 trace_id（log.Valuer 求值）
      -> zap 输出 {service: "interview", trace_id: "abc123", span_id: "def456", ...}
```

---

## 八、验收标准

### 阶段 1 验收
- `go build ./pkg/...` 编译通过
- `pkg/telemetry/` 有单元测试
- MQ trace 传递有集成测试（publish 注入 -> consume 提取 -> span 链路连通）

### 阶段 2 验收
- `go build ./app/...` 全部 15 个服务编译通过
- 启动 interview + ai_gateway + learning_archive，Jaeger UI 看到完整 trace 链
- 日志中出现 trace_id 字段
- 各服务 :6060/metrics 返回 Prometheus 格式数据
- 各服务 :6060/healthz 返回 200
- 各服务 :6060/debug/pprof/ 可访问
- 确认 8 个服务（user/membership/learning_archive/rag/realtime/growth/community/coderunner）超时配置生效且不引入新问题（对应风险 #8）

### 阶段 3 验收
- Jaeger 中看到 ark.chat span，含 model/token 属性
- Grafana ai_calls_total / ai_tokens_total 有数据
- MQ 链路 interview.finished 的 trace 从 Interview 跨到 LearningArchive 不断链
- curl 冒烟脚本：打一条面试链路（HTTP -> Gateway -> Interview -> AI Gateway -> Ark），断言 Jaeger API 出现该 trace

### 阶段 4 验收
- docker build 能构建全部 15 个 Go 服务 + 前端镜像
- 镜像大小 < 50MB（distroless）或 < 100MB（alpine）
- ConfigMap 挂载的 config.yaml 生效（K8s 中服务读取挂载路径的配置）

### 阶段 5 验收
- kind create cluster + helm install 一键部署
- kubectl port-forward 后前端可正常访问
- Jaeger UI 有完整 trace
- Grafana 有指标面板
- kubectl delete pod 后 Pod 优雅停机 + 自动重建
- HPA 生效（压测后 replicas 自动扩展）
- 端到端验证脚本：curl 打一条面试链路，断言 Jaeger API 出现该 trace（与阶段 3 同一脚本，目标改为 K8s Service 地址）

---

## 九、请评审者重点评估

1. **拦截器顺序**：otelgrpc -> prometheus -> recovery -> logging -> auth 是否正确？是否有遗漏的拦截器需要加入链？
2. **MQ trace 方案**：AMQP Headers 作为 carrier 是否可靠？consumer per-message ctx 派生是否会影响消息重试机制？
3. **pkg/server 抽象**：签名参数化（addr/timeout/authInterceptor=grpc.UnaryServerInterceptor/extraOpts）是否覆盖所有差异场景？user 的 blacklist 变体经 `auth.NewInterceptor(secret, WithBlacklist(...)).UnaryServerInterceptor()` 传入是否清晰？（已与 checklist 1.4 对齐：不用 extraOpts 注入 auth）
4. **配置外置策略**：ConfigMap 挂载 config.yaml 是否足够？是否需要考虑热更新（ConfigMap reload）或 Kratos config source？
5. **OTel Collector**：Collector 只做 trace 汇聚、Metrics 走 Prometheus 直接 scrape 的拆分是否合理？sampling 默认关闭是否合适？
6. **Helm 模板化**：overrides map 的 5% 例外覆盖是否足够？gateway WS 注解和 realtime WS 端口的特殊处理是否应该抽成独立模板而非条件分支？
7. **工作量评估**：20 个任务的工作量分布是否合理？哪个阶段被低估或高估？
8. **遗漏项**：是否有未考虑到的集成点或风险？
