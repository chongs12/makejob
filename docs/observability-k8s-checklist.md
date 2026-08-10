# 可观测性 + pprof + K8s 部署 实施检查清单

> 核心链路：HTTP Gateway -> Interview -> AI Gateway(+Ark HTTP) + RAG + LearningArchive
> 异步链路：Interview --interview.finished--> LearningArchive

---

## 阶段 1：pkg 层基础设施

### 1.1 创建 `pkg/telemetry/` 包

- [x] `pkg/telemetry/tracer.go`
  - [x] `InitTracerProvider(serviceName, otlpEndpoint string, sampleRatio float64) (*sdktrace.TracerProvider, func(), error)`
  - [x] 使用 `otlptracegrpc` 导出器连接 OTel Collector（:4317）
  - [x] `resource` 包含 `service.name`、`service.version`、`deployment.environment`
  - [x] `TraceIDRatioBased` 采样器，比率可配置
  - [x] 设置 W3C TraceContext + Baggage Propagator
  - [x] 返回 cleanup 函数（`tp.Shutdown(ctx)`）
- [x] `pkg/telemetry/meter.go`
  - [x] `InitMeterProvider(serviceName string) (*sdkmetric.MeterProvider, func(), error)`
  - [x] 不使用 OTLP 导出 metrics，metrics 走 Prometheus 直接 scrape `/metrics` 端点
- [x] `pkg/telemetry/http_server.go`
  - [x] `NewHTTPServer(port int, registerer prometheus.Registerer) *http.Server`
  - [x] 挂载 `/metrics`（`promhttp.Handler()`）
  - [x] 挂载 `/healthz`（liveness，返回 200）
  - [x] 挂载 `/readyz`（readiness，可扩展检查逻辑）
  - [x] 挂载 `/debug/pprof/`（`pprof.Index`/`pprof.Profile`/`pprof.Cmdline` 等）
  - [x] 实现优雅停机（`Shutdown(ctx)`）
- [x] `pkg/telemetry/telemetry.go`
  - [x] `Config struct { OTLPEndpoint string; ServiceName string; SampleRatio float64; HTTPPort int }`
  - [x] `Init(cfg Config) (cleanup func(), err error)` 一站式初始化 tracer + meter + HTTP server
  - [x] 启动 HTTP server 在独立 goroutine
  - [x] `telemetry.Init` 必须显式设 globals：`otel.SetTracerProvider` + `otel.SetTextMapPropagator(TraceContext+Baggage)`
  - [x] 每个服务的 resource 必须带 `service.name`

**交付物**：`pkg/telemetry/` 目录下 4 个文件，可独立编译，有单元测试

### 1.2 创建 gRPC 追踪 + 指标拦截器

- [x] `pkg/middleware/tracing.go`
  - [x] `TracingServerInterceptor() grpc.UnaryServerInterceptor`（封装 `otelgrpc.UnaryServerInterceptor`）
  - [x] `TracingClientInterceptor() grpc.UnaryClientInterceptor`（封装 `otelgrpc.UnaryClientInterceptor`）
- [x] `pkg/middleware/metrics.go`（**选型：gRPC 指标用 go-grpc-prometheus，不自写 gRPC 指标拦截器**）
  - [x] 引入 `grpc_prometheus.UnaryServerInterceptor()` + `grpc_prometheus.Register(srv.Server)`（在 `pkg/server.NewGRPCServer` 内统一注册）
  - [x] 指标名用 go-grpc-prometheus **标准名**：`grpc_server_handled_total`、`grpc_server_handling_seconds` 等（Grafana 查询按这些名写）
  - [x] **不要**自定义 `grpc_requests_total` / `grpc_request_duration_seconds`（会与 go-grpc-prometheus 指标名对不上，Grafana 面板空转）
  - [x] AI/HTTP 业务指标用 `prometheus/client_golang` 自写（`prometheus.NewCounter` / `prometheus.NewHistogram`），注册到全局 `prometheus.DefaultRegisterer`

**交付物**：`pkg/middleware/` 下新增 2 个文件

**关键约束**：拦截器链顺序必须为 `otelgrpc -> prometheus(grpc_prometheus) -> recovery -> logging -> auth`（otel 最外层，确保 span 在 logging 求值时已进 ctx）

### 1.3 统一 gRPC 客户端 Dial 选项

- [x] `pkg/middleware/dial_options.go`
  - [x] `CommonDialOptions() []grpc.DialOption` —— **只含基座，不含 auth**（auth 由各客户端按需追加，见下）
  - [x] 内含：`grpc.WithTransportCredentials(insecure.NewCredentials())`
  - [x] 内含：`grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor)`
  - [x] 内含：`grpc.WithUnaryInterceptor(MetricsClientInterceptor())`（可选，client 侧指标）
  - [x] **auth 拦截器不进 CommonDialOptions，各客户端按场景追加**：
    - [x] 无 auth：`grpc.Dial(addr, CommonDialOptions()...)`（interview→ai/rag/code_runner）
    - [x] ServiceAuth：`grpc.Dial(addr, append(CommonDialOptions(), grpc.WithUnaryInterceptor(auth.ServiceAuthInterceptor(token)))...)`（archive/membership/companion 客户端）
    - [x] ForwardToken：`grpc.Dial(addr, append(CommonDialOptions(), grpc.WithUnaryInterceptor(auth.ForwardTokenClientInterceptor()))...)`（companion→interview、gateway 转发用户 JWT）
  - [x] 多个 `grpc.WithUnaryInterceptor` 会被 grpc-go 的 `combine()` 按顺序叠加（先加的最外层），不要误以为"后者覆盖前者"

**交付物**：`pkg/middleware/dial_options.go`

### 1.4 抽取 `pkg/server/` 统一 gRPC Server 构造

- [x] `pkg/server/grpc.go`
  - [x] `NewGRPCServer` 签名参数化（auth 用 `grpc.UnaryServerInterceptor`，nil = 无 auth；timeout==0 用默认值兜底）：
    ```go
    func NewGRPCServer(
        addr string,
        timeout time.Duration, // 0 或负值 → 用默认（如 10s），learning_archive 未配 timeout 时兜底
        authInterceptor grpc.UnaryServerInterceptor, // nil = 无 auth（rag/coderunner）
        register func(*grpc.Server),
        extraOpts ...grpc.ServerOption,
    ) *kratosgrpc.Server
    ```
  - [x] 拦截器链统一组装：`otelgrpc -> prometheus(grpc_prometheus) -> recovery -> logging -> auth`
  - [x] 内部调用 `grpc_prometheus.Register(srv.Server)`（gRPC 指标一处注册）
  - [x] 支持 `RegisterFunc func(*grpc.Server)` 回调注册各服务的 proto service
  - [x] **auth 差异由各服务构造自己的 interceptor 传入**：
    - [x] 普通服务：`auth.NewInterceptor(secret).UnaryServerInterceptor()`
    - [x] user（blacklist 变体）：`auth.NewInterceptor(secret, WithBlacklist(...)).UnaryServerInterceptor()`
    - [x] rag/coderunner：传 `nil`
    - [x] **不要**用 `extraOpts` 注入 auth 拦截器（grpc.ServerOption 只会 append 到链尾，导致双重 auth）
  - [x] timeout 从 config 传入（`cfg.GRPC.Timeout`），与现有 6 个服务的 if/else 行为一致（config 优先、缺省兜底）
- [x] 各服务的 `internal/server/grpc.go` 改为调用 `pkg/server.NewGRPCServer`（替换 14 处复制粘贴）

**交付物**：`pkg/server/grpc.go` + 14 个服务的 grpc.go 精简

### 1.5 MQ trace context 传递

- [x] 改 `pkg/mq/publisher.go`
  - [x] `Publish(ctx, ...)` 中用 `otel.GetTextMapPropagator().Inject(ctx, amqpHeaderCarrier(publishing.Headers))` 注入 traceparent（**注意是 `amqp.Publishing.Headers`，不是消费者侧的 `delivery.Headers`**）
  - [x] 不修改 `TaskMessage` 结构体
- [x] 改 `pkg/mq/consumer.go`
  - [x] `processMessages` 中用 `otel.GetTextMapPropagator().Extract(ctx, amqpHeaderCarrier(delivery.Headers))` 提取 trace context
  - [x] 为每条消息创建 consumer span（`tracer.Start(msgCtx, "mq.consume."+msg.TaskType, ...)`)
  - [x] handler 用派生的 `msgCtx` 而非共享的 `Start(ctx)`
  - [x] span 属性：`messaging.system=rabbitmq`、`messaging.destination=<queue>`、`messaging.message_id=<entity_id>`
- [x] 新增 `pkg/mq/propagator.go`
  - [x] `amqpHeaderCarrier` 实现 `propagation.TextMapCarrier` 接口（Get/Set/Keys 操作 `amqp.Table`）
- [x] Consumer 重试共享同一 consume span，用 span events 记录重试（attempt/delay/err）
- [x] handler 内部的 gRPC 子调用仍建 child span

**交付物**：`pkg/mq/` 下改 2 个文件 + 新增 1 个文件

### 1.6 结构化日志注入 trace_id

**方案：logger 初始化处用 Kratos `log.With` + `tracing.TraceID()/SpanID()` Valuer（覆盖所有 `log.Context(ctx)` 日志，包括 biz 层的 Errorw/panic 日志）**

- [x] 改 `pkg/logger/zap.go`
  - [x] `NewZapLogger(serviceName string)` 接受服务名参数
  - [x] 返回前包装：`log.With(&kratosZapLogger{logger: zl}, "service", serviceName, "trace_id", tracing.TraceID(), "span_id", tracing.SpanID())`
  - [x] `tracing.TraceID()`/`SpanID()` 自带 `HasTraceID()` 守卫，无 span 时输出空串
  - [x] **不要**手动 `trace.SpanContextFromContext(ctx).TraceID()` 提取——无 span 时它返回 32 位全零串，会污染日志
- [x] `pkg/middleware/logging.go` **无需改**：`log.Context(ctx)` 会自动带上包装后的 trace_id（前提：otelgrpc 在 Logging 之前执行，已由 1.2 顺序保证）

**交付物**：改 `pkg/middleware/logging.go` + `pkg/logger/`

---

## 阶段 2：各微服务接入

### 2.1 gRPC server 拦截器链扩展（14 个 Kratos 服务）

对每个服务执行：
- [x] gateway（不适用，用 Gin）
- [x] user
- [x] membership
- [x] question
- [x] interview
- [x] realtime（**适用**：gRPC(9005) + WS(8085) 双 transport，gRPC server 照常接 otelgrpc 链；WS 部分见 2.6/2.7）
- [x] growth
- [x] plan
- [x] companion
- [x] community
- [x] learning_archive
- [x] ai_gateway
- [x] rag
- [x] coderunner
- [x] admin

每个服务的 `internal/server/grpc.go`：
- [x] 替换为调用 `pkg/server.NewGRPCServer`（如果 1.4 已完成）
- [x] 或手动在拦截器数组最前面追加 `otelgrpc.UnaryServerInterceptor` 和 `grpc_prometheus.UnaryServerInterceptor`

### 2.2 gRPC client Dial 替换（有跨服务调用的服务）

- [x] gateway - `internal/proxy/` 下的所有 `grpc.Dial` 调用
- [x] interview - `internal/data/` 下的 ai_client/archive_client/membership_client/rag_client/code_runner_client
- [x] companion - `internal/data/` 下的 interview_client/growth_client/plan_client
- [x] growth - `internal/data/clients.go` 下的 interview/archive/plan/question client
- [x] plan - `internal/data/` 下的 learning_archive_client
- [x] admin - `internal/data/` 下的各服务 client
- [x] realtime - `internal/data/` 下的 interview/rag client
- [x] question - `internal/data/` 下的 ai_client/code_runner_client

每个客户端：
- [x] `grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))` 替换为 `grpc.Dial(addr, middleware.CommonDialOptions()...)`

### 2.3 各服务 main.go 接入 telemetry

对全部 15 个服务执行：
- [x] gateway
- [x] user
- [x] membership
- [x] question
- [x] interview
- [x] realtime
- [x] growth
- [x] plan
- [x] companion
- [x] community
- [x] learning_archive
- [x] ai_gateway
- [x] rag
- [x] coderunner
- [x] admin

每个服务的 `cmd/server/main.go`：
- [x] 调用 `telemetry.Init(cfg)` 初始化 TracerProvider + HTTP server
- [x] `defer cleanup()` 确保退出时 flush span
- [x] pprof 端口：本地 dev 用偏移（6060+服务序号），K8s 内统一 6060（通过环境变量控制）

### 2.4 Gateway 接入

- [x] 追加 `otelgin.Middleware("gateway")` 到 Gin 中间件链（最外层）
- [x] 优雅停机改造：
  - [x] `r.Run()` 替换为 `http.Server{Handler: r}`
  - [x] 添加 signal handling（`signal.Notify(SIGTERM, SIGINT)`）
  - [x] 收到信号后 `srv.Shutdown(ctx)` 等待 in-flight 请求完成
- [x] 补充业务 Prometheus 指标（在 handler 中埋点）：
  - [x] `http_requests_total`（CounterVec: method/path/code）
  - [x] `http_request_duration_seconds`（HistogramVec: method/path）
  - [x] gateway 同时有 :8082/metrics（现有）与 :6060/metrics（telemetry.Init），**复用同一个 `prometheus.DefaultRegisterer`**，两个端点数据一致，避免指标分裂
- [x] Gateway 的 gRPC 客户端 Dial 全部替换为 `CommonDialOptions()`

### 2.5 config.yaml 增加 telemetry 配置

全部 15 个服务的 `configs/config.yaml`：
- [x] 增加 `telemetry` 段（**提交的 config.yaml 里 `http_port` 写偏移值（6060+服务序号）避免本地 dev 冲突；K8s ConfigMap 才统一覆盖成 6060**）：
  ```yaml
  telemetry:
    otlp_endpoint: "localhost:4317"
    service_name: "makejob.interview"
    sample_ratio: 1.0
    http_port: 6060  # 本地 dev 各服务写 6060+序号（如 interview 在 6060+4）；K8s ConfigMap 覆盖为 6060
  ```
- [x] 增加 `conf.go` 中 `Telemetry` struct 定义

### 2.6 realtime WebSocket trace 传播（特殊处理）

- [x] WS 握手时从 HTTP header 提取 traceparent，创建 server span
- [ ] WS 消息中携带 traceparent（自定义消息头或 first frame）- 未做：需前端配合改 WS 协议；已用 session server span + 关键事件 span 替代覆盖（commit 5cd14b5）
- [x] realtime -> Interview/RAG 的 gRPC 调用正常用 `CommonDialOptions()`

### 2.7 realtime 指标

- [x] realtime 服务注册 `active_websocket_connections`（Gauge）
- [x] 在 WS 连接建立时 +1，断开时 -1

---

## 阶段 3：AI/RAG 手动埋点

### 3.1 AI Gateway LLM 调用 span

- [x] `app/ai_gateway/internal/data/ark_client.go`
  - [x] `Chat()` 前后创建 span：`tracer.Start(ctx, "ark.chat")`
  - [x] span 属性：`llm.model`、`llm.provider=volcengine`、`llm.thinking_enabled`
  - [x] span 事件：`prompt_tokens`、`completion_tokens`、`total_tokens`
  - [x] 异常时记录 `span.RecordError(err)`
- [x] `app/ai_gateway/internal/data/openai_client.go`
  - [x] 同上，`llm.provider=openai_compatible`

### 3.2 AI Gateway 调用指标

- [x] `app/ai_gateway/internal/biz/` 或 `internal/data/`
  - [x] 定义 `ai_calls_total`（CounterVec: scene/model/status）
  - [x] 定义 `ai_tokens_total`（CounterVec: scene/model/type=prompt|completion）
  - [x] 定义 `ai_call_duration_seconds`（HistogramVec: scene/model）
  - [x] 在 LLM 调用后埋点
  - [x] 验证 `context.WithoutCancel` 保留 trace context values - ai_client.go:208 不存在（调研纠正）；实际 ark_client.go:209 + biz/ai.go:21；并修复 interview_session.go 4 处 context.Background() -> WithoutCancel(ctx) 保留 trace

### 3.3 RAG 检索 span

- [x] `app/rag/internal/biz/`
  - [x] `Retrieve()` 前后创建 span：`tracer.Start(ctx, "rag.retrieve")`
  - [x] span 属性：`rag.query_length`、`rag.top_k`、`rag.results_count`
- [x] `app/rag/internal/data/milvus_client.go`
  - [x] Milvus 搜索 span（如果 Milvus 客户端自带 OTel instrument 则验证即可）
- [x] RAG MQ consumer（`rag.sync.question`）trace 提取

### 3.4 Interview 报告生成 span

- [x] `app/interview/internal/biz/usecase.go`
  - [x] `GenerateReport()` 前后创建 span：`tracer.Start(ctx, "interview.generate_report")`
  - [x] span 属性：`report.type`（standard/knowledge/job/realtime）、`interview.id`
  - [x] 异步 goroutine（如 `SubmitAnswer -> WriteEntry`）用 `otel.Start(ctx)` 显式 span，避免 `context.Background()` 断链
- [x] Interview MQ publisher（`interview.finished`）注入 traceparent（已由 1.5 覆盖）
- [x] LearningArchive MQ consumer 提取 traceparent 并创建 consumer span（已由 1.5 覆盖）

### 3.5 端到端验证脚本

- [x] 创建 `scripts/e2e-trace-check.sh`
  - [x] curl 发起一条面试链路请求（Gateway -> Interview -> AI Gateway）
  - [x] 查询 Jaeger API 断言 trace 存在且 span 链完整
  - [ ] 断言 MQ 链路 trace 从 Interview 跨到 LearningArchive - 未做：脚本当前断言 gateway->interview->user 跨服务 trace；MQ 链路(interview.finished)断言待补（需触发报告生成流程）

---

## 阶段 4：容器化 + 配置外置

### 4.1 配置外置（ConfigMap/Secret 挂载）

- [ ] K8s 用 ConfigMap 挂载 `config.yaml` 到 `-conf` flag 指向路径，加载机制零改动
- [ ] ConfigMap 内容：peer 地址用 K8s Service DNS（如 `interview:9004`）、OTLP 用 `collector:4317`
- [ ] Secret 单独挂载为文件：`DB_PASSWORD`、`JWT_SECRET`、`ARK_API_KEY`、MQ URL（含密码）
- [ ] 敏感字段注入二选一：
  - [ ] **方案 A（推荐，改动最小）**：conf.go 对这几个敏感字段加 `os.Getenv` 覆盖（`if v := os.Getenv("DB_PASSWORD"); v != "" { ... }`），K8s 用 Secret + env 注入
  - [ ] **方案 B**：config.yaml 引用 Secret 文件路径（如 `db.password: /etc/secret/db_password`），conf.go **需加一个 secret-resolver**（值以 `/` 开头则读文件内容）——别忘了这个 conf 改动，否则 password 字段拿到的是路径字符串而不是密码
- [ ] 本地用仓库里的 `config.yaml` 不变

### 4.2 统一多阶段 Dockerfile

- [ ] `Dockerfile`（根目录，通用模板）
  - [ ] Stage 1 `builder`：`golang:1.25-alpine`，拷贝 go.mod/go.sum，`go mod download`，编译指定服务的二进制
  - [ ] `CGO_ENABLED=0` 静态编译
  - [ ] Stage 2 `runtime`：`gcr.io/distroless/static-debian12` 或 `alpine:3.19`
  - [ ] 非 root 用户运行
  - [ ] `ARG SERVICE_NAME` 控制编译哪个服务
- [ ] 验证 15 个服务都能用同一个 Dockerfile 构建

### 4.3 前端 Dockerfile

- [ ] `frontend-react/Dockerfile`
  - [ ] Stage 1：`node:20-alpine`，`npm ci`，`npm run build:web`
  - [ ] Stage 2：`nginx:alpine`，拷贝 dist，配置 nginx.conf
- [ ] `frontend-react/nginx.conf`
  - [ ] 静态资源服务
  - [ ] `/api/v1` 反向代理到 gateway:8082
  - [ ] WebSocket 升级头（`/api/v1/interviews/*/ws`）
  - [ ] `/live2d-assets` 反向代理到 gateway:8082

### 4.4 Makefile 补全

- [ ] 补全缺失的 8 个服务构建目标（companion、ai_gateway、learning_archive、plan、rag、realtime、membership、coderunner）
- [ ] 增加 `docker-build SERVICE=<name>` 目标
- [ ] 增加 `docker-build-all` 目标

### 4.5 docker-compose 全栈编排（可选，本地开发用）

- [ ] 扩展 `docker-compose.yml`，增加全部 15 个服务 + 前端
- [ ] 基础设施服务（PostgreSQL、Redis、RabbitMQ）加入 compose
- [ ] 可观测性栈（OTel Collector + Jaeger + Prometheus + Grafana）加入 compose
- [ ] 健康检查 + depends_on

---

## 阶段 5：K8s 部署

### 5.1 kind 集群

- [ ] `deploy/kind-cluster.yaml`
  - [ ] kind 配置：1 control-plane + 2 worker
  - [ ] `extraPortMappings`：8080（Ingress）、16686（Jaeger UI）、3000（Grafana）、9090（Prometheus）
  - [ ] ingress-nginx controller 安装
- [ ] `deploy/kind-setup.sh` 脚本
  - [ ] 创建集群
  - [ ] 加载本地镜像（`kind load docker-image`）
  - [ ] 安装 ingress controller
  - [ ] 安装 metrics-server（需 `--kubelet-insecure-tls` 参数）
  - [ ] 基础设施容器部署（PostgreSQL/Redis/RabbitMQ/Piston 可用 Helm subchart 或外部 Service）

### 5.2 Helm Chart 骨架

- [ ] `deploy/helm/makejob/Chart.yaml`
  - [ ] name: makejob, version: 0.1.0
- [ ] `deploy/helm/makejob/values.yaml`
  - [ ] 全局：image registry/tag、imagePullPolicy、env（dev/staging/prod）
  - [ ] 服务列表（15 个 + 前端）：name、port、replicas、resources、config
  - [ ] 可观测性栈配置
  - [ ] 数据库/Redis/MQ 连接配置
- [ ] `deploy/helm/makejob/templates/_helpers.tpl`
  - [ ] `makejob.fullname`、`makejob.labels`、`makejob.selectorLabels`
  - [ ] `makejob.serviceName`、`makejob.servicePort`

### 5.3 通用 Deployment 模板

- [ ] `deploy/helm/makejob/templates/deployment.yaml`
  - [ ] `{{- range $svc, $cfg := .Values.services }}` 遍历生成
  - [ ] `values.yaml` 用 overrides map 做例外覆盖（95% 通用 + 5% overrides）
  - [ ] 容器：image、ports（gRPC + telemetry HTTP）、env（从 ConfigMap/Secret 引用）
  - [ ] livenessProbe: `/healthz` on :6060
  - [ ] readinessProbe: `/readyz` on :6060
  - [ ] resources: requests/limits（CPU/Memory）
  - [ ] `terminationGracePeriodSeconds: 30`
  - [ ] `preStop` hook（可选：`sleep 5` 让 LB 摘除）
- [ ] Gateway 特殊处理
  - [ ] HTTP port 8082 暴露
  - [ ] 优雅停机已由阶段 2.4 保证
- [ ] coderunner 例外：有 `:6060` telemetry（不例外），唯一例外是无业务 HTTP Service 端口

### 5.4 Service + Ingress

- [ ] `deploy/helm/makejob/templates/service.yaml`
  - [ ] 每个服务一个 ClusterIP Service（gRPC port + telemetry port）
  - [ ] Gateway 额外暴露 HTTP port
- [ ] `deploy/helm/makejob/templates/ingress.yaml`
  - [ ] 单 Ingress 规则：
    - `/api/v1` -> gateway:8082（含 WS upgrade 注解）
    - `/live2d-assets` -> gateway:8082
    - `/` -> frontend:80
  - [ ] `nginx.ingress.kubernetes.io/proxy-body-size: 50m`
  - [ ] WS 超时注解用 `$cfg.ingress.websocket` 条件判断

### 5.5 ConfigMap + Secret

- [ ] `deploy/helm/makejob/templates/configmap.yaml`
  - [ ] 每个服务的 config.yaml 挂载（非敏感部分）
  - [ ] config.yaml 内服务地址用 K8s Service DNS（如 `interview:9004`）
- [ ] `deploy/helm/makejob/templates/secret.yaml`
  - [ ] `DB_PASSWORD`、`REDIS_PASSWORD`、`JWT_SECRET`、`ARK_API_KEY`
  - [ ] MQ URL（含密码）

### 5.6 HPA

- [ ] `deploy/helm/makejob/templates/hpa.yaml`
  - [ ] gateway: CPU > 70% -> 2-5 replicas
  - [ ] companion: CPU > 70% -> 2-5 replicas
  - [ ] ai_gateway: CPU > 70% -> 2-3 replicas
  - [ ] interview: CPU > 70% -> 2-3 replicas
  - [ ] Deployment 必须设置 `resources.requests.cpu`（否则 HPA 无基准）

### 5.7 可观测性栈 K8s 部署

- [ ] `deploy/helm/makejob/charts/observability/` 子 Chart
  - [ ] OTel Collector（Deployment + ConfigMap）
    - [ ] 接收 OTLP gRPC :4317
    - [ ] 只导出 trace 到 Jaeger（不导出 metrics）
    - [ ] 采样过滤（可选）
  - [ ] `collector.yaml` 配置文件（本地 dev 栈已写 observability/otel-collector/collector.yaml，K8s ConfigMap 待部署时复用此配置）
    - [x] receivers: otlp (grpc :4317)
    - [x] processors: batch（sampling 配好但默认关闭，demo 用 AlwaysOn）
    - [x] exporters: otlp/jaeger + debug（本地调试）
  - [x] `observability/docker-compose.yml` 增加 OTel Collector 容器
    - [x] Jaeger :4317 改为从 Collector 收数据（服务不直连 Jaeger）
  - [ ] Jaeger（Deployment + Service）
    - [ ] `all-in-one:1.57`
    - [ ] `COLLECTOR_OTLP_ENABLED=true`
    - [ ] Service: 16686（UI）+ 4317（OTLP，仅 ClusterIP）
  - [ ] Prometheus（Deployment + ConfigMap + Service）
    - [ ] `kube-prometheus-stack` 或独立 `prom/prometheus`
    - [ ] 抓取配置：kubernetes_sd_configs 或 PodMonitor
    - [ ] 抓取所有 Pod 的 :6060/metrics
  - [ ] Grafana（Deployment + ConfigMap + Service）
    - [ ] 数据源：Prometheus + Jaeger
    - [ ] 导入 `observability/grafana/dashboards/makejob-overview.json`
    - [ ] 新增 AI 调用仪表盘（ai_calls_total / ai_tokens_total / ai_call_duration_seconds）

### 5.8 Prometheus 服务发现

- [ ] 改 Prometheus 抓取配置
  - [ ] 从 `static_configs: [host.docker.internal:8082]` 改为 `kubernetes_sd_configs`
  - [ ] 按 Pod label `makejob.io/scrape: "true"` 自动发现
  - [ ] 抓取端口 6060，path `/metrics`

---

## 验收标准

### 阶段 1 验收
- [x] `go build ./pkg/...` 编译通过
- [x] `pkg/telemetry/` 有单元测试
- [x] MQ trace 传递有集成测试（publisher 注入 -> consumer 提取 -> span 链路连通）

### 阶段 2 验收
- [x] `go build ./app/...` 全部 15 个服务编译通过
- [x] 启动 interview + gateway + user，在 Jaeger UI 看到 `HTTP Gateway -> Interview -> User` 完整跨服务 trace 链 - 已验证（gateway->interview ListInterviews 3 span，gateway->user Login 3 span；ai_gateway 链待阶段三 AI 埋点后验证）
- [x] 日志中出现 `trace_id` 字段 - 已验证（interview RPC 日志含非空 trace_id 如 15bd7f62...；gateway access log 经 GinLoggerMiddleware 改走 kratos log，带 trace_id 如 c0ee5906...，commit b7209af）
- [x] 各服务 `:6060/metrics` 返回 Prometheus 格式数据 - 已验证（interview :6064 返回 200）
- [x] 各服务 `:6060/healthz` 返回 200 - 已验证（interview :6064/healthz + /readyz 200）
- [x] 各服务 `:6060/debug/pprof/` 可访问 - 已验证（interview :6064/debug/pprof/ 200）
- [ ] 确认 8 个服务（user/membership/learning_archive/rag/realtime/growth/community/coderunner）超时从 1s 默认值变为配置值，行为变更无异常 - interview/user 启动运行正常，其余 6 个待回归
- [x] learning_archive 补充 config.yaml 中 timeout 字段

### 阶段 3 验收
- [x] 在 Jaeger 中看到 AI Gateway 的 `ark.chat` span，含 model/token 属性 - 已验证（grpcurl 调 QuizAnalyzer，ark.chat span 挂在 RPC span 下，model=minimax-m3，tokens prompt=396/completion=942）
- [x] Grafana 仪表盘 `ai_calls_total` / `ai_tokens_total` 有数据 - 已验证 ai_gateway :6071/metrics 输出指标（Grafana 完整展示待 Prometheus 配置抓取 ai_gateway:6071，当前 prometheus.yml 仅抓 gateway:8082）
- [x] MQ 链路 `interview.finished` 的 trace 从 Interview 跨到 LearningArchive 不断链 - 已验证（FinishInterview -> interview.report.generate MQ -> GenerateReport -> interview.finished MQ -> learning_archive 消费，全链路同一 traceID 93b8b064...，含 mq.consume.interview.report.generate + mq.consume.interview.finished 两个 consumer span，pkg/mq trace 注入/提取机制端到端确认）

### 阶段 4 验收
- [ ] `docker build` 能构建全部 15 个 Go 服务 + 前端镜像
- [ ] 镜像大小 < 50MB（distroless）或 < 100MB（alpine）
- [ ] ConfigMap 挂载的 config.yaml 生效（K8s 内服务读取到挂载配置）

### 阶段 5 验收
- [ ] `kind create cluster` + `helm install makejob` 一键部署
- [ ] `kubectl port-forward svc/gateway 8082:8082` 后前端可正常访问
- [ ] Jaeger UI（`localhost:16686`）有完整 trace
- [ ] Grafana（`localhost:3000`）有指标面板
- [ ] `kubectl delete pod <gateway-pod>` 后 Pod 优雅停机 + 自动重建
- [ ] HPA 生效（压测后 replicas 自动扩展）
- [ ] 端到端验证脚本通过
