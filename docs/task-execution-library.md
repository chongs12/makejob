# MakeJob 微服务改造 — 执行任务指令库

> **文档结构说明**
>
> 本任务库由两个文件组成：
> - 📄 `task-execution-library.md`（本文件）：全局规范 + Phase 0 任务 + P1-1 范例
> - 📄 `tasks-phase1-7.md`：Phase 1~7 全部任务的完整执行提示词
>
> **使用方法**：将"全局约定"部分作为 System Prompt 前置，然后按 Phase 顺序逐个将 PROMPT 块复制给执行模型。

## 一、全局约定与规范

执行模型在写任何代码时，都必须严格遵守以下规范。违反任何一条，视为任务未完成。

### 1.1 技术栈

- 语言：Go 1.22+
- 微服务框架：`github.com/go-kratos/kratos/v2`
- ORM：`gorm.io/gorm` + `gorm.io/driver/postgres`
- RPC：`google.golang.org/grpc` + `google.golang.org/protobuf`
- 消息队列：RabbitMQ（`github.com/rabbitmq/amqp091-go`）
- 模块路径：`makejob`（根 go.mod）

### 1.2 服务内部分层

每个服务遵循以下目录结构：

```
app/<service>/
├── cmd/server/main.go          # 启动入口 + 手动 DI 装配
├── configs/config.yaml         # 配置文件
├── internal/
│   ├── conf/conf.go            # 配置结构体 + Load()
│   ├── biz/                    # 领域层：实体 + Repo 接口 + UseCase
│   │   ├── <domain>.go         # 实体定义 + UseCase
│   │   └── errors.go           # 领域错误
│   ├── data/                   # 数据层：Repo 实现
│   │   ├── data.go             # DB 连接初始化
│   │   └── <domain>_repo.go    # GORM 实现
│   ├── service/                # 服务层：gRPC handler
│   │   └── <service>.go        # 实现 UnimplementedXxxServer
│   └── server/                 # 传输层：gRPC server + MQ consumer
│       ├── grpc.go
│       └── mq.go               # 如有 MQ 消费
```

### 1.3 编码规范

| 规则 | 说明 |
|------|------|
| 错误处理 | **必须**使用 `github.com/go-kratos/kratos/v2/errors` 构造，如 `errors.NotFound("REASON", "message")`。**禁止** `errors.New()` 或 `fmt.Errorf` 返回给 gRPC |
| 日志 | **必须**使用 `github.com/go-kratos/kratos/v2/log`。通过 DI 注入 `log.Logger` |
| gRPC 客户端 | **必须**通过构造函数注入（DI），**禁止**在 service/biz 层直接 `grpc.Dial` |
| MQ 发布 | **必须**使用 `makejob/pkg/mq` 中的 `Publisher.Publish(ctx, routingKey, TaskMessage)` |
| MQ 消费 | **必须**实现 `pkg/mq.TaskHandler` 接口，通过 `Consumer.Register(queueName, handler)` 注册 |
| DB 查询 | **必须**使用 `.WithContext(ctx)` 传播 context |
| 实体定义 | 定义在 `biz/` 包，携带 GORM tag，有 `TableName()` 方法 |
| Repo 接口 | 定义在 `biz/` 包，实现在 `data/` 包 |
| Proto 转换 | 使用私有 `toProtoXxx()` 函数，放在 `service/` 层 |
| 构造函数 | 使用 `NewXxx(deps...) *Xxx` 模式，不使用全局变量 |
| Context 中的用户 | 通过 `auth.GetUserIDFromContext(ctx)` 获取，**禁止**从 request body 取 user_id（除内部 RPC） |
| 时间字段 | Proto 中使用 `google.protobuf.Timestamp`，Go 中使用 `timestamppb.New(t)` 转换 |
| 软删除 | 所有实体包含 `gorm.DeletedAt`，通过 `gorm:"index"` 标记 |
| ID 类型 | 数据库用 `uint`，Proto 用 `uint64`，转换时直接 `uint64(entity.ID)` |

### 1.4 BaseModel 模板

所有实体必须嵌入：
```go
type BaseModel struct {
    ID        uint           `gorm:"primaryKey;autoIncrement"`
    CreatedAt time.Time      `gorm:"not null;autoCreateTime"`
    UpdatedAt time.Time      `gorm:"not null;autoUpdateTime"`
    DeletedAt gorm.DeletedAt `gorm:"index"`
}
```

### 1.5 MQ TaskMessage 格式

```go
type TaskMessage struct {
    TaskType   string          `json:"task_type"`
    EntityType string          `json:"entity_type"`
    EntityID   uint64          `json:"entity_id"`
    Payload    json.RawMessage `json:"payload"`
    RetryCount int             `json:"retry_count"`
    CreatedAt  time.Time       `json:"created_at"`
}
```

### 1.6 禁止事项（全局）

- 禁止使用 `init()` 函数
- 禁止使用全局变量存储运行时状态
- 禁止在 service 层直接操作 `*gorm.DB`（必须通过 repo）
- 禁止 `panic`，必须返回 error
- 禁止硬编码连接字符串，必须从 config 读取
- 禁止跨服务直接访问其他服务的数据库
- 禁止在 proto 转换中忽略 nil 检查

---

## 二、Phase 0 — 基础设施就绪

### 任务 P0-1：创建 Membership Proto

**目标**：定义 `membership.proto`，从 `user.proto` 中拆出会员相关 RPC。

**需要新建的文件**：
- `api/makejob/membership/v1/membership.proto`

**执行提示词**：

---

#### PROMPT P0-1

```
你是一个 Go 微服务开发者。请按以下要求创建 Proto 文件。

【任务】创建 api/makejob/membership/v1/membership.proto

【文件路径】api/makejob/membership/v1/membership.proto

【要求】

syntax = "proto3";
package makejob.membership.v1;
option go_package = "makejob/api/makejob/membership/v1;membershipv1";

import "google/protobuf/timestamp.proto";

service MembershipService {
  rpc GetMembershipStatus(UserIDRequest) returns (MembershipStatus);
  rpc ListPlans(ListPlansRequest) returns (ListPlansResponse);
  rpc CreateOrder(CreateOrderRequest) returns (OrderResponse);
  rpc GetOrder(GetOrderRequest) returns (OrderResponse);
  rpc ListOrders(ListOrdersRequest) returns (ListOrdersResponse);
  rpc HandlePaymentCallback(PaymentCallbackRequest) returns (OrderResponse);
  rpc CheckFeatureAccess(CheckFeatureRequest) returns (CheckFeatureResponse);
  rpc UpgradeMembership(UpgradeRequest) returns (MembershipStatus);
}

message UserIDRequest {
  uint64 user_id = 1;
}

message MembershipStatus {
  string level = 1;           // "free" or "pro"
  google.protobuf.Timestamp expire_at = 2;
  bool is_active = 3;
}

message MembershipPlan {
  string plan_type = 1;       // monthly/quarterly/yearly
  string name = 2;
  double price = 3;
  int32 duration_days = 4;
  repeated string features = 5;
}

message ListPlansRequest {}
message ListPlansResponse {
  repeated MembershipPlan plans = 1;
}

message CreateOrderRequest {
  uint64 user_id = 1;
  string plan_type = 2;       // monthly/quarterly/yearly
}

message OrderResponse {
  uint64 id = 1;
  string order_no = 2;
  string plan_type = 3;
  double amount = 4;
  string status = 5;          // pending/paid/cancelled/refunded
  google.protobuf.Timestamp created_at = 6;
  google.protobuf.Timestamp paid_at = 7;
  google.protobuf.Timestamp expires_at = 8;
}

message GetOrderRequest {
  uint64 user_id = 1;
  uint64 order_id = 2;
}

message ListOrdersRequest {
  uint64 user_id = 1;
  int32 page = 2;
  int32 page_size = 3;
}

message ListOrdersResponse {
  repeated OrderResponse orders = 1;
  int64 total = 2;
}

message PaymentCallbackRequest {
  string order_no = 1;
  string channel = 2;
  string transaction_id = 3;
}

message CheckFeatureRequest {
  uint64 user_id = 1;
  string feature = 2;         // "unlimited_practice", "unlimited_interview"
}

message CheckFeatureResponse {
  bool allowed = 1;
  string reason = 2;
}

message UpgradeRequest {
  uint64 user_id = 1;
  string level = 2;
  int32 duration_days = 3;
}

【验证标准】
1. protoc 编译通过（无语法错误）
2. 8 个 RPC 全部定义
3. go_package 路径正确

【禁止事项】
- 不要使用 google.api.http 注解（Gateway 手动路由）
- 不要 import 其他服务的 proto
```

---

### 任务 P0-2：创建 Plan Proto

**文件**：`api/makejob/plan/v1/plan.proto`

**PROMPT P0-2**：见架构方案 2.8 节。package=makejob.plan.v1, go_package="makejob/api/makejob/plan/v1;planv1"。7个RPC：CreatePlan, GetPlan, GetCurrentPlan, ListPlans, UpdateTaskStatus, SubmitTaskFeedback, AdjustPlan。详细 Message 定义见架构方案。

---

### 任务 P0-3 ~ P0-6：创建其余 Proto

| 任务 | 文件 | RPC数 | 关键点 |
|------|------|-------|--------|
| P0-3 | api/makejob/companion/v1/companion.proto | 3 | Chat, GetCompanionState, SynthesizeSpeech |
| P0-4 | api/makejob/realtime/v1/realtime.proto | 5 | InitSession, GetSessionStatus, InjectRAGContext, EndSession, HealthCheck |
| P0-5 | api/makejob/coderunner/v1/coderunner.proto | 2 | Execute, ListLanguages |
| P0-6 | api/makejob/rag/v1/rag.proto | 8 | Retrieve, IndexQuestions, IndexDocuments, DeleteIndex, GetConfig, UpdateConfig, TestConnection, GetDocumentStats |

---

### 任务 P0-7：修改 Growth Proto

**文件**：`api/makejob/growth/v1/growth.proto`（修改）

删除 CreatePlan/GetPlan/UpdateTaskStatus/SubmitTaskFeedback/Chat 五个 RPC（已移到 plan.proto 和 companion.proto），仅保留 GetGrowthSummary/GetWeeklyFocus/SyncStudyLog 三个。

---

### 任务 P0-8：搭建 7 个新服务骨架

为 membership/plan/companion/realtime/coderunner/rag/ai_gateway 各创建 Kratos 标准目录。详细 Prompt 见下方 Phase 1 第一个完整实现任务作为模板参考。

---

## 三、Phase 1 — 基础设施服务实现

### 任务 P1-1：CodeRunner 服务完整实现

这是最简单的基础设施服务，作为所有服务实现的执行模板范例。

#### PROMPT P1-1

```
你是一个 Go 微服务开发者，正在为 MakeJob 项目实现 CodeRunner 微服务。

【任务】实现 CodeRunner 服务的 Execute 和 ListLanguages 两个 gRPC RPC。

【目标】提供沙箱代码执行能力，封装 Piston API，供 Question 和 Interview 服务调用。

【需要创建的文件】
1. app/coderunner/internal/conf/conf.go（配置结构）
2. app/coderunner/internal/biz/coderunner.go（领域逻辑 + 接口定义）
3. app/coderunner/internal/biz/errors.go（错误定义）
4. app/coderunner/internal/data/data.go（Piston HTTP client）
5. app/coderunner/internal/data/piston_client.go（Piston 调用实现）
6. app/coderunner/internal/service/coderunner.go（gRPC handler）
7. app/coderunner/internal/server/grpc.go（gRPC server 注册）
8. app/coderunner/cmd/server/main.go（启动入口）
9. app/coderunner/configs/config.yaml

【Proto 接口】（已定义在 api/makejob/coderunner/v1/coderunner.proto）

service CodeRunnerService {
  rpc Execute(ExecuteRequest) returns (ExecuteResponse);
  rpc ListLanguages(ListLanguagesRequest) returns (ListLanguagesResponse);
}

ExecuteRequest: language(string), code(string), stdin(string), test_cases(repeated TestCase), timeout_ms(int32)
TestCase: input(string), expected_output(string)
ExecuteResponse: success(bool), stdout(string), stderr(string), exit_code(int32), execution_time_ms(int64), test_results(repeated TestResult), passed_count(int32), total_count(int32), error(string)

【实现步骤】

步骤1: conf/conf.go
- 定义 Bootstrap 结构体，包含 Server.GRPC.Addr 和 Piston.Endpoint(string) 和 Piston.TimeoutMs(int, 默认10000)

步骤2: biz/coderunner.go
- 定义 PistonExecutor 接口:
  Execute(ctx context.Context, req *ExecuteInput) (*ExecuteOutput, error)
- 定义 ExecuteInput 结构体: Language, Code, Stdin string; TestCases []TestCaseInput; TimeoutMs int32
- 定义 TestCaseInput: Input, ExpectedOutput string
- 定义 ExecuteOutput: Success bool; Stdout, Stderr, Error string; ExitCode int; ExecutionTimeMs int64; TestResults []TestResultOutput; PassedCount, TotalCount int32
- 定义 TestResultOutput: Input, Expected, Actual string; Passed bool
- 定义 CodeRunnerUseCase 结构体，持有 PistonExecutor 接口
- 实现 NewCodeRunnerUseCase(executor PistonExecutor) *CodeRunnerUseCase
- 实现 (uc *CodeRunnerUseCase) Execute(ctx, input) (*ExecuteOutput, error) 方法:
  - 如果 test_cases 为空，直接执行代码返回 stdout/stderr
  - 如果 test_cases 非空，逐个执行（stdin=test_case.input），对比 stdout 与 expected_output
  - 汇总 passed_count / total_count

步骤3: biz/errors.go
- var ErrUnsupportedLanguage = errors.BadRequest("UNSUPPORTED_LANGUAGE", "不支持的编程语言")
- var ErrExecutionTimeout = errors.GatewayTimeout("EXECUTION_TIMEOUT", "代码执行超时")
- var ErrPistonUnavailable = errors.ServiceUnavailable("PISTON_UNAVAILABLE", "代码执行引擎不可用")

步骤4: data/piston_client.go
- 实现 pistonClient 结构体，持有 httpClient *http.Client 和 endpoint string
- 实现 NewPistonClient(endpoint string, timeoutMs int) biz.PistonExecutor
- Piston API 调用:
  POST {endpoint}/api/v2/execute
  Body: {"language": "go", "version": "*", "files": [{"content": code}], "stdin": stdin, "run_timeout": timeoutMs}
  Response: {"run": {"stdout": "...", "stderr": "...", "code": 0, "signal": null, "output": "..."}}
- 语言版本映射: go→"1.21.0", python→"3.11.0", javascript→"18.15.0", java→"17.0.0", cpp→"17.0.0"(语言名用"c++")
- 如果语言不在映射中，返回 ErrUnsupportedLanguage
- 如果 HTTP 调用失败，返回 ErrPistonUnavailable
- 如果响应中 signal 为 "SIGKILL"，视为超时，返回 ErrExecutionTimeout

步骤5: service/coderunner.go
- 定义 CodeRunnerService 结构体:
  type CodeRunnerService struct {
      coderunnerv1.UnimplementedCodeRunnerServiceServer
      uc *biz.CodeRunnerUseCase
  }
- func NewCodeRunnerService(uc *biz.CodeRunnerUseCase) *CodeRunnerService
- 实现 Execute(ctx, req) (*coderunnerv1.ExecuteResponse, error):
  - 构造 biz.ExecuteInput
  - 调用 uc.Execute
  - 转换为 proto response
- 实现 ListLanguages(ctx, req):
  - 返回硬编码列表: [{name:"go",version:"1.21"},{name:"python",version:"3.11"},{name:"javascript",version:"18"},{name:"java",version:"17"},{name:"cpp",version:"17"}]

步骤6: server/grpc.go
- func NewGRPCServer(cfg *conf.Server, svc *service.CodeRunnerService, logger log.Logger) *kratosgrpc.Server
- 注册 CodeRunnerServiceServer
- 添加 Recovery + Logging middleware
- 注意：CodeRunner 不需要 JWT 认证（内部服务间调用）

步骤7: cmd/server/main.go
- 装配顺序: conf.Load → NewPistonClient → NewCodeRunnerUseCase → NewCodeRunnerService → NewGRPCServer → kratos.New

步骤8: configs/config.yaml
server:
  grpc:
    addr: "0.0.0.0:9013"
piston:
  endpoint: "http://localhost:2000"
  timeout_ms: 10000

【错误处理】
- 语言不支持 → UNSUPPORTED_LANGUAGE (400)
- Piston HTTP 调用失败 → PISTON_UNAVAILABLE (503)
- 代码执行被 SIGKILL → EXECUTION_TIMEOUT (408)

【验证标准】
1. go build ./app/coderunner/cmd/server/ 编译通过
2. 启动后 gRPC 端口 9013 可连接
3. 调用 ListLanguages 返回 5 种语言
4. 调用 Execute(language="go", code="package main\nimport \"fmt\"\nfunc main(){fmt.Println(\"hello\")}") 返回 stdout="hello\n"

【单体参考代码路径】backend/internal/executor/piston.go

【禁止事项】
- 禁止使用全局变量
- 禁止在 service 层直接调用 http.Post（必须通过 biz 接口 + data 实现）
- 禁止硬编码 Piston 地址（必须从 config 读取）
- 禁止添加 JWT 认证（此为内部服务）
- 禁止使用 errors.New 或 fmt.Errorf 返回错误（必须用 kratos errors）
```

---

**Phase 1 其余任务（P1-2 ~ P1-10）以及 Phase 2-7 全部任务的完整 Prompt 详见**：

📄 **`docs/tasks-phase1-7.md`**（4878 行，49 个任务）

该文件包含所有任务的完整执行提示词，按 Phase 组织：
- **Phase 1**（10 任务）: CodeRunner, RAG (Retrieve/Index/MQ), AI Gateway (6个Agent RPC)
- **Phase 2**（6 任务）: User RefreshToken 修复 + Logout, Membership 全实现, Community 补齐
- **Phase 3**（8 任务）: Question RunCode/DeleteNote/TimedExam/SubmitExam/QuestionSets/MistakeTopics/MQ/推荐增强
- **Phase 4**（9 任务）: Interview 核心流程(出题/结束/报告/编程/Realtime), LearningArchive MQ, Realtime 全实现
- **Phase 5**（9 任务）: Plan(生成/查询/状态/反馈/调整), Growth(重写3个RPC), Companion 全实现
- **Phase 6**（4 任务）: Admin gRPC 委托, SSE 流, Gateway 删除 Bridge, 路由注册
- **Phase 7**（3 任务）: 健康检查, 链路追踪, CI/CD
