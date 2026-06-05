# MakeJob 微服务架构改造文档

## 一、项目概述

MakeJob 是一个 AI 驱动的面试准备与学习平台，原为 Go 单体应用（Gin + GORM），本次改造为基于 **Kratos + gRPC** 的微服务架构。

### 改造目标
- 体现 Go 语言优雅设计：清晰分层、接口隔离、依赖注入、Proto-first API
- 支持服务独立部署、独立扩缩容
- 保留现有 RabbitMQ 异步能力，引入 gRPC 同步通信
- 前端 API 无感切换（BFF 网关保持 REST 接口）

---

## 二、架构总览

```
                    ┌─────────────────┐
                    │   BFF Gateway   │  :8082 (HTTP REST → gRPC)
                    └────────┬────────┘
                             │ gRPC
     ┌──────────┬────────────┼────────────┬──────────┬──────────┐
     ▼          ▼            ▼            ▼          ▼          ▼
┌─────────┐┌─────────┐┌──────────┐┌──────────┐┌─────────┐┌──────────┐
│  user   ││question ││interview ││  growth  ││  admin  ││community │
│ :9004   ││ :9002   ││  :9003   ││  :9005   ││ :9006   ││ :9007    │
└────┬────┘└────┬────┘└────┬─────┘└────┬─────┘└────┬────┘└─────┬────┘
     │          │          │           │           │           │
     └──────────┴──────────┴───────────┴───────────┴───────────┘
                              │ gRPC
                    ┌─────────┴─────────┐
                    │learning_archive   │
                    │      :9008        │
                    ├───────────────────┤
                    │  AI Service :9014 │
                    │  Industry  :9011  │
                    └───────────────────┘
                              │
     ┌────────────┬───────────┼───────────┬────────────┐
     ▼            ▼           ▼           ▼            ▼
 PostgreSQL    Redis     RabbitMQ       etcd       Milvus
  :5434       :6384      :5672        :2379       :19530
```

### 服务职责

| 服务 | 职责 | 拥有的表 | gRPC 方法数 |
|---|---|---|---|
| **user** | 注册登录、JWT 签发、会员管理 | `users`, `membership_orders` | 9 |
| **question** | 题库管理、刷题、编程判题、智能推荐 | `questions`, `categories`, `industries`, `user_question_records`, `user_favorites`, `user_notes`, `rag_documents` | 16 |
| **interview** | AI 面试（含实时语音）、报告生成 | `mock_interviews`, `interview_messages`, `interview_coding_attempts` | 14 |
| **growth** | 学习计划、AI 陪伴、成长记录 | `learning_plans`, `learning_tasks`, `learning_task_feedbacks`, `learning_task_diagnoses`, `study_logs` | 8 |
| **admin** | 后台管理、AI 配置、Prompt 模板 | `admin_configs`, `ai_presets`, `ai_call_logs`, `prompt_templates`, `scraper_tasks` | 10 |
| **community** | 社区帖子、评论、点赞 | `community_posts`, `community_comments`, `community_post_likes` | 8 |
| **learning_archive** | 学习档案（共享内核） | `learning_archive_entries` | 5 |
| **gateway** | BFF 网关，REST → gRPC 代理 | 无 | HTTP 代理 |

---

## 三、技术栈

| 层次 | 技术 |
|---|---|
| 微服务框架 | Kratos v2.8 |
| 通信协议 | gRPC（服务间）+ HTTP REST（对外） |
| Proto 代码生成 | protoc + protoc-gen-go + protoc-gen-go-grpc |
| 依赖注入 | Google Wire（手动组装模式） |
| 数据库 | PostgreSQL + GORM |
| 消息队列 | RabbitMQ（amqp091-go） |
| 缓存 | Redis |
| 服务发现 | etcd |
| 向量数据库 | Milvus（RAG） |
| 认证 | JWT（golang-jwt/v5）+ bcrypt |
| 日志 | Zap + Kratos log 适配器 |
| 可观测性 | Prometheus + Jaeger + Grafana（OTLP） |

---

## 四、目录结构

```
makejob/
├── api/makejob/                    # Proto 定义 + 生成代码
│   ├── shared/v1/                  # 共享类型（base, pagination, error）
│   ├── user/v1/                    # UserService 定义
│   ├── question/v1/                # QuestionService 定义
│   ├── interview/v1/               # InterviewService 定义
│   ├── growth/v1/                  # GrowthService 定义
│   ├── admin/v1/                   # AdminService 定义
│   ├── community/v1/               # CommunityService 定义
│   ├── learning_archive/v1/        # LearningArchiveService 定义
│   ├── industry/v1/                # IndustryService 定义
│   └── ai/v1/                      # AIService 定义
│
├── app/                            # 微服务实现
│   ├── gateway/                    # BFF 网关
│   │   ├── cmd/server/main.go      # 入口
│   │   ├── configs/config.yaml     # 配置
│   │   └── internal/
│   │       ├── conf/conf.go        # 配置结构
│   │       └── proxy/handler.go    # HTTP→gRPC 代理
│   │
│   ├── interview/                  # ★ 面试服务（核心）
│   │   ├── cmd/server/main.go
│   │   ├── configs/config.yaml
│   │   └── internal/
│   │       ├── biz/                # 业务逻辑层
│   │       │   ├── interview.go    # 领域实体 + 接口定义
│   │       │   └── usecase.go      # 业务用例实现
│   │       ├── data/               # 数据访问层
│   │       │   ├── interview_repo.go
│   │       │   ├── ai_client.go    # AI 服务 gRPC 客户端
│   │       │   ├── archive_client.go
│   │       │   ├── industry_client.go
│   │       │   └── model/          # GORM model（服务私有）
│   │       ├── service/            # gRPC 服务实现
│   │       │   └── interview.go    # implements pb.InterviewServiceServer
│   │       ├── server/             # Transport 层
│   │       │   ├── grpc.go         # gRPC server
│   │       │   └── mq.go           # MQ consumer
│   │       └── conf/conf.go        # 配置结构
│   │
│   ├── question/                   # ★ 题目服务（核心）
│   │   └── internal/
│   │       ├── biz/                # 10 个业务方法
│   │       ├── data/               # 6 个 repo + AI 客户端
│   │       ├── service/            # 16 个 gRPC 方法
│   │       └── server/
│   │
│   ├── user/                       # 用户服务
│   ├── growth/                     # 成长服务
│   ├── admin/                      # 管理服务
│   ├── community/                  # 社区服务
│   └── learning_archive/           # 学习档案服务
│
├── pkg/                            # 共享库
│   ├── auth/                       # JWT + gRPC 拦截器
│   ├── errors/                     # 统一错误码
│   ├── middleware/                  # Recovery + Logging
│   ├── mq/                         # RabbitMQ consumer/publisher
│   ├── logger/                     # Zap 适配器
│   ├── discovery/                  # etcd 服务注册
│   ├── model/                      # BaseModel
│   └── pagination/                 # 分页工具
│
├── deploy/                         # 部署配置
│   └── k8s/                        # K8s 清单
│
├── Makefile                        # 构建命令
├── buf.yaml                        # Buf lint 配置
├── buf.gen.yaml                    # Buf 代码生成配置
└── go.mod                          # 单 Go module: makejob
```

---

## 五、Kratos 分层架构

每个服务严格遵循 Kratos 标准四层架构：

```
cmd/server/main.go      → 入口，组装依赖
    ↓
internal/service/       → gRPC 服务实现（proto → biz 转换）
    ↓
internal/biz/           → 业务逻辑层（领域实体 + 用例）
    ↓
internal/data/          → 数据访问层（GORM repo + gRPC 客户端）
    ↓
internal/server/        → Transport 层（gRPC/HTTP/MQ server）
```

### 依赖方向

```
service → biz → data（单向依赖）
service → biz ← data（通过接口解耦）
```

- **biz 层**定义接口（如 `InterviewRepo`, `AIServiceClient`），不依赖任何外部实现
- **data 层**实现 biz 层接口，持有 `*gorm.DB` 或 gRPC 客户端
- **service 层**实现 proto 生成的 gRPC 接口，调用 biz 层方法，负责 DTO 转换
- **server 层**注册 transport（gRPC/HTTP/MQ），注入中间件

### 接口隔离示例

```go
// biz/interview.go — 只定义接口，不依赖实现
type InterviewRepo interface {
    Create(ctx context.Context, interview *Interview) error
    GetByID(ctx context.Context, id uint64) (*Interview, error)
    ListByUser(ctx context.Context, userID uint64, page, pageSize int32) ([]*Interview, int64, error)
    Update(ctx context.Context, interview *Interview) error
}

type AIServiceClient interface {
    InterviewAgent(ctx context.Context, req *InterviewAgentRequest) (*InterviewAgentResponse, error)
}

// data/interview_repo.go — 实现接口
func NewInterviewRepo(db *gorm.DB) biz.InterviewRepo {
    return &interviewRepo{db: db}
}
```

---

## 六、关键设计模式

### 6.1 Proto-first API 设计

所有 gRPC 接口通过 `.proto` 文件定义，共享类型抽取到 `shared/v1/`：

```protobuf
// api/makejob/shared/v1/base.proto
message BaseModel {
  uint64 id = 1;
  google.protobuf.Timestamp created_at = 2;
  google.protobuf.Timestamp updated_at = 3;
}

// api/makejob/interview/v1/interview.proto
service InterviewService {
  rpc CreateInterview(CreateInterviewRequest) returns (InterviewResponse);
  rpc SubmitAnswer(SubmitAnswerRequest) returns (AnswerFeedback);
  rpc GetInterviewStats(UserIDRequest) returns (InterviewStats);
  // ... 共 14 个 RPC 方法
}
```

### 6.2 统一错误处理

使用 Kratos errors 包定义业务错误码，gRPC 拦截器自动转换：

```go
// pkg/errors/codes.go
var (
    ErrUserNotFound    = kratosErr.NotFound("USER_NOT_FOUND", "用户不存在")
    ErrEmailExists     = kratosErr.Conflict("EMAIL_EXISTS", "邮箱已注册")
    ErrInterviewNotFound = kratosErr.NotFound("INTERVIEW_NOT_FOUND", "面试不存在")
)

// biz/usecase.go — 业务层抛出结构化错误
func (uc *InterviewUseCase) GetInterview(ctx context.Context, id, userID uint64) (*Interview, error) {
    interview, err := uc.repo.GetByID(ctx, id)
    if err != nil {
        return nil, ErrInterviewNotFound  // 自动转为 gRPC NOT_FOUND
    }
    if interview.UserID != userID {
        return nil, ErrUnauthorized       // 自动转为 gRPC UNAUTHENTICATED
    }
    return interview, nil
}

// gateway/internal/proxy/handler.go — 网关层映射 gRPC 状态码到 HTTP
func grpcErrorToHTTP(err error) (int, string) {
    st, _ := status.FromError(err)
    switch st.Code() {
    case codes.NotFound:     return 404, st.Message()
    case codes.Unauthenticated: return 401, st.Message()
    case codes.PermissionDenied: return 403, st.Message()
    default:                 return 500, "internal error"
    }
}
```

### 6.3 依赖注入（Wire 模式）

每个服务的 `wire.go` 定义 ProviderSet，`main.go` 手动组装：

```go
// app/interview/wire.go（build tag: wireinject）
var providerSet = wire.NewSet(
    wire.FieldsOf(new(*conf.Bootstrap), "AI", "Industry", "Archive"),
    data.NewData,
    data.NewInterviewRepo,
    data.NewAIServiceClient,
    data.NewLearningArchiveClient,
    data.NewIndustryClient,
    biz.NewInterviewUseCase,
    service.NewInterviewService,
    server.NewGRPCServer,
    server.NewMQConsumer,
)

// app/interview/cmd/server/main.go — 实际组装
func wireApp(bc *conf.Bootstrap, logger log.Logger) (*kratos.App, func(), error) {
    db, _ := data.NewData(bc.Data)
    interviewRepo := data.NewInterviewRepo(db)
    aiClient, _ := data.NewAIServiceClient(bc.AI)
    archiveClient, _ := data.NewLearningArchiveClient(bc.Archive)
    industryClient, _ := data.NewIndustryClient(bc.Industry)

    uc := biz.NewInterviewUseCase(interviewRepo, aiClient, archiveClient, industryClient)
    svc := service.NewInterviewService(uc)

    gs := server.NewGRPCServer(bc.Server, svc, authInterceptor, logger)
    mqConsumer, _ := server.NewMQConsumer(bc.MQ, uc)

    app := kratos.New(
        kratos.Name("makejob.interview"),
        kratos.Server(gs, mqConsumer),
    )
    return app, cleanup, nil
}
```

### 6.4 gRPC 中间件链

```go
// 统一中间件链：Recovery → Logging → Auth
grpc.ChainUnaryInterceptor(
    middleware.Recovery(),                      // panic 恢复
    middleware.Logging(),                       // 请求日志
    authInterceptor.UnaryServerInterceptor(),  // JWT 认证
)
```

### 6.5 MQ 消费者作为 Kratos Transport

```go
// app/interview/internal/server/mq.go
type MQConsumer struct {
    consumer *mq.Consumer
    uc       *biz.InterviewUseCase
}

func (c *MQConsumer) Start(ctx context.Context) error {
    c.consumer.Register(mq.QueueInterviewResumeParse, mq.TaskHandlerFunc(func(ctx context.Context, msg mq.TaskMessage) error {
        var payload mq.InterviewResumeParsePayload
        json.Unmarshal(msg.Payload, &payload)
        return c.uc.ProcessResumeParse(ctx, payload.InterviewID, payload.UserID, payload.ResumeText)
    }))
    return c.consumer.Start(ctx)
}

func (c *MQConsumer) Stop(ctx context.Context) error {
    return c.consumer.Stop(ctx)
}

// 注册到 Kratos app（与 gRPC/HTTP 并列）
app := kratos.New(kratos.Server(gs, mqConsumer))
```

### 6.6 跨服务调用模式

```go
// Before（单体）：直接查其他域的表
user, err := s.userRepo.GetByID(ctx, userID)  // 跨域直接查 users 表

// After（微服务）：通过 gRPC 客户端接口调用
type IndustryClient interface {
    GetIndustry(ctx context.Context, code string) (*Industry, error)
}

// data/industry_client.go — 真实 gRPC 实现 + 本地缓存
func (c *industryClient) GetIndustry(ctx context.Context, code string) (*biz.Industry, error) {
    // 先查缓存（30 分钟 TTL）
    if v, ok := c.cache.Load(code); ok {
        ci := v.(*cachedIndustry)
        if time.Since(ci.cachedAt) < industryCacheTTL {
            return ci.industry, nil
        }
    }
    // gRPC 调用 Industry 服务
    resp, _ := c.client.GetIndustry(ctx, &industryv1.GetIndustryRequest{Code: code})
    industry := &biz.Industry{Code: resp.Code, Name: resp.Name}
    c.cache.Store(code, &cachedIndustry{industry: industry, cachedAt: time.Now()})
    return industry, nil
}
```

---

## 七、统计指标

| 指标 | 数量 |
|---|---|
| 微服务数 | 8 个（含 gateway） |
| Proto 定义文件 | 12 个 |
| gRPC 服务 | 9 个 |
| gRPC 方法总数 | 80 个 |
| Go 源文件（app/） | 82 个 |
| Go 源文件（pkg/） | 12 个 |
| Proto 生成代码（api/） | 21 个 |
| GORM Model 文件 | 13 个 |
| 代码总行数 | ~24,900 行 |
| 基础设施容器 | 7 个 |

---

## 八、启动与测试指南

### 8.1 前置依赖

```bash
# 必需工具
go version          # >= 1.23
protoc --version    # >= 3.20
protoc-gen-go --version
protoc-gen-go-grpc --version

# Go 工具
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### 8.2 启动基础设施

```bash
# 1. 启动核心基础设施（PostgreSQL + RabbitMQ + etcd + MinIO + Milvus）
cd D:/gogogo/makejob
docker-compose up -d

# 2. 启动可观测性栈（可选）
cd D:/gogogo/makejob/observability
docker-compose up -d

# 验证服务状态
docker ps
# 应看到：postgres:5434, rabbitmq:5672, etcd:2379, minio:9000, milvus:19530
```

### 8.3 数据库初始化

```bash
# PostgreSQL 会自动创建数据库 makejob
# 各服务启动时 GORM AutoMigrate 会自动建表
# 如需手动连接：
psql -h localhost -p 5434 -U postgres -d makejob
```

### 8.4 Proto 代码生成

```bash
cd D:/gogogo/makejob

# 方式一：使用 Makefile
make api

# 方式二：手动 protoc（如果 buf 未安装）
protoc \
  --proto_path=./api \
  --proto_path=/path/to/google/protobuf/include \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  api/makejob/**/*.proto
```

### 8.5 编译

```bash
cd D:/gogogo/makejob

# 编译全部服务
go build ./...

# 编译单个服务
go build ./app/interview/cmd/server
go build ./app/question/cmd/server
go build ./app/user/cmd/server

# 使用 Makefile
make build           # 全部
make build-interview # 仅面试服务
```

### 8.6 启动服务

**逐个启动（推荐开发环境）：**

```bash
# 终端 1：启动 User 服务
cd app/user
go run cmd/server/main.go -conf configs/config.yaml

# 终端 2：启动 Question 服务
cd app/question
go run cmd/server/main.go -conf configs/config.yaml

# 终端 3：启动 Interview 服务
cd app/interview
go run cmd/server/main.go -conf configs/config.yaml

# 终端 4：启动 Growth 服务
cd app/growth
go run cmd/server/main.go -conf configs/config.yaml

# 终端 5：启动 Admin 服务
cd app/admin
go run cmd/server/main.go -conf configs/config.yaml

# 终端 6：启动 Community 服务
cd app/community
go run cmd/server/main.go -conf configs/config.yaml

# 终端 7：启动 LearningArchive 服务
cd app/learning_archive
go run cmd/server/main.go -conf configs/config.yaml

# 终端 8：启动 Gateway
cd app/gateway
go run cmd/server/main.go -conf configs/config.yaml
```

**端口分配：**

| 服务 | gRPC 端口 | HTTP 端口 |
|---|---|---|
| user | 9004 | 8004 |
| question | 9002 | 8002 |
| interview | 9003 | 8003 |
| growth | 9005 | 8005 |
| admin | 9006 | 8006 |
| community | 9007 | 8007 |
| learning_archive | 9008 | 8008 |
| **gateway** | - | **8082** |

### 8.7 测试接口

**通过 Gateway HTTP 访问（推荐）：**

```bash
# 健康检查
curl http://localhost:8082/api/health

# 用户注册
curl -X POST http://localhost:8082/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"test@example.com","password":"123456"}'

# 用户登录
curl -X POST http://localhost:8082/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"123456"}'
# 返回：{"token":"eyJhbG...","user":{...}}

# 使用 token 访问受保护接口
TOKEN="eyJhbG..."

# 获取用户资料
curl http://localhost:8082/api/auth/profile \
  -H "Authorization: Bearer $TOKEN"

# 获取题目列表
curl "http://localhost:8082/api/questions?page=1&page_size=10" \
  -H "Authorization: Bearer $TOKEN"

# 创建面试
curl -X POST http://localhost:8082/api/interview \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"industry_code":"go","difficulty":"medium","question_count":5,"interview_mode":"standard"}'

# 获取成长摘要
curl http://localhost:8082/api/growth/summary \
  -H "Authorization: Bearer $TOKEN"

# 获取社区帖子
curl http://localhost:8082/api/community/posts \
  -H "Authorization: Bearer $TOKEN"
```

**直接 gRPC 访问（使用 grpcurl）：**

```bash
# 安装 grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# 列出 Interview 服务的方法
grpcurl -plaintext localhost:9003 list makejob.interview.v1.InterviewService

# 调用 CreateInterview
grpcurl -plaintext -d '{"user_id":1,"industry_code":"go","difficulty":"medium","question_count":5}' \
  localhost:9003 makejob.interview.v1.InterviewService/CreateInterview

# 调用 Question 服务
grpcurl -plaintext -d '{"industry_code":"go","page":{"page":1,"page_size":10}}' \
  localhost:9002 makejob.question.v1.QuestionService/ListQuestions
```

### 8.8 日志配置

通过环境变量控制日志行为：

```bash
# 设置日志级别（debug/info/warn/error）
export LOG_LEVEL=debug

# 设置日志输出（stdout/stderr）
export LOG_OUTPUT=stderr

# 启动服务
go run cmd/server/main.go -conf configs/config.yaml
```

### 8.9 可观测性访问

```bash
# Prometheus 指标
open http://localhost:9090

# Grafana 仪表盘
open http://localhost:3000  # 默认账号 admin/admin

# Jaeger 链路追踪
open http://localhost:16686

# RabbitMQ 管理面板
open http://localhost:15672  # 默认账号 guest/guest
```

---

## 九、开发工作流

### 9.1 新增 gRPC 接口

```bash
# 1. 编辑 proto 文件
vim api/makejob/interview/v1/interview.proto

# 2. 生成代码
make api  # 或 protoc ...

# 3. 实现 biz 层方法
vim app/interview/internal/biz/usecase.go

# 4. 实现 service 层方法
vim app/interview/internal/service/interview.go

# 5. 编译验证
go build ./app/interview/...
```

### 9.2 新增跨服务调用

```bash
# 1. 在 biz 层定义客户端接口
vim app/interview/internal/biz/interview.go  # type XxxClient interface {...}

# 2. 在 data 层实现 gRPC 客户端
vim app/interview/internal/data/xxx_client.go

# 3. 在 main.go 注入
vim app/interview/cmd/server/main.go

# 4. 更新 conf.go 添加服务地址配置
vim app/interview/internal/conf/conf.go
```

### 9.3 运行测试

```bash
# 全量测试
go test ./...

# 单个服务测试
go test ./app/interview/...

# 带覆盖率
go test -cover ./app/interview/internal/biz/...
```

---

## 十、生产环境注意事项

以下为生产部署时需要补充的事项（当前阶段为开发/展示模式）：

1. **数据库物理分库**：当前所有服务共享同一个 PostgreSQL，通过逻辑 schema 隔离。生产环境应按服务拆分独立数据库。

2. **分布式事务**：跨服务数据一致性使用 Saga + Outbox 模式，通过 RabbitMQ 实现最终一致性。

3. **服务发现**：当前使用 etcd 做服务注册，gRPC 客户端直连配置地址。生产环境应启用 etcd resolver 做动态服务发现。

4. **TLS 加密**：当前 gRPC 通信使用 insecure 模式。生产环境应启用 mTLS。

5. **熔断限流**：集成 gobreaker 做熔断，使用 Redis 做分布式限流。

6. **K8s 部署**：每个服务编写 Deployment + Service + HPA，配置健康检查和优雅关闭。

7. **配置中心**：将 config.yaml 迁移到 etcd 或 Nacos，支持动态配置更新。

8. **API 版本管理**：Proto 文件使用 `v1/v2` 版本目录，通过 gRPC 多版本服务共存。

---

## 附录 A：Proto 服务方法清单

### UserService（9 个方法）
| 方法 | 说明 |
|---|---|
| Register | 用户注册（bcrypt 哈希） |
| Login | 用户登录（返回 JWT） |
| RefreshToken | 刷新 token |
| GetProfile | 获取用户资料 |
| UpdateProfile | 更新用户资料 |
| GetUserByID | 内部调用：按 ID 查询 |
| BatchGetUsers | 内部调用：批量查询 |
| GetMembershipStatus | 获取会员状态 |
| UpgradeMembership | 升级会员 |

### QuestionService（16 个方法）
| 方法 | 说明 |
|---|---|
| ListQuestions | 题目列表（分页+筛选） |
| GetQuestion | 题目详情 |
| ListCategories | 分类树 |
| ListIndustries | 行业列表 |
| SubmitAnswer | 提交答案（AI 分析） |
| RunCode | 运行代码（Piston 判题） |
| CreateFavorite / DeleteFavorite | 收藏管理 |
| ListFavorites | 收藏列表 |
| CreateNote / UpdateNote / ListNotes | 笔记管理 |
| GetPracticeRecommendations | 智能推荐（基于错题） |
| GetWrongQuestions | 错题列表 |
| GetUserPracticeStats | 练习统计 |
| GetRandomExam | 随机模拟考试 |

### InterviewService（14 个方法）
| 方法 | 说明 |
|---|---|
| CreateInterview | 创建面试会话 |
| GetInterview | 面试详情 |
| ListInterviews | 面试列表 |
| SubmitAnswer | 提交答案（AI 反馈） |
| FinishInterview | 结束面试 |
| GetNextQuestion | 获取下一题 |
| SubmitCodingAnswer | 提交编程题 |
| GetReport | 获取面试报告 |
| GetInterviewStats | 面试统计（供 growth 调用） |
| IsRealtimeInterview | 判断是否实时语音 |
| GetRealtimeContext | 获取实时面试上下文 |
| BindRealtimeDialog | 绑定实时对话 |
| AppendRealtimeUserAnswer | 追加用户语音答案 |
| AppendRealtimeAssistantReply | 追加 AI 回复 |

### GrowthService（8 个方法）
| 方法 | 说明 |
|---|---|
| GetGrowthSummary | 成长摘要（聚合统计） |
| GetWeeklyFocus | 每周聚焦建议 |
| SyncStudyLog | 同步学习日志 |
| CreatePlan | 创建学习计划 |
| GetPlan | 获取学习计划 |
| UpdateTaskStatus | 更新任务状态 |
| SubmitTaskFeedback | 提交任务反馈 |
| Chat | AI 陪伴对话 |

### AdminService（10 个方法）
| 方法 | 说明 |
|---|---|
| ListUsers | 用户管理列表 |
| UpdateUserRole | 修改用户角色 |
| ListAIPresets / SaveAIPreset | AI 模型预设管理 |
| ListPromptTemplates / SavePromptTemplate | Prompt 模板管理 |
| GetAdminConfig / SetAdminConfig | 系统配置管理 |
| DebugAI | AI 调试接口 |
| ListAICallLogs | AI 调用日志 |

### CommunityService（8 个方法）
| 方法 | 说明 |
|---|---|
| ListPosts / GetPost | 帖子浏览 |
| CreatePost / UpdatePost / DeletePost | 帖子管理 |
| ListComments | 评论列表 |
| CreateComment | 发表评论 |
| ToggleLike | 点赞/取消 |

### LearningArchiveService（5 个方法）
| 方法 | 说明 |
|---|---|
| WriteEntry | 写入学习档案条目 |
| BatchWriteEntries | 批量写入 |
| ListByUser | 查询用户档案 |
| GetWeakTopics | 获取薄弱知识点 |
| GetFocusSignals | 获取聚焦信号 |

### AIService（6 个方法）
| 方法 | 说明 |
|---|---|
| InterviewAgent | 面试 AI 代理 |
| PlanAgent | 学习计划 AI |
| CompanionAgent | AI 陪伴 |
| QuizAnalyzer | 答题分析 |
| ResumeParser | 简历解析 |
| Live2DDirector | Live2D 指令生成 |

### IndustryService（4 个方法）
| 方法 | 说明 |
|---|---|
| ListIndustries | 行业列表 |
| GetIndustry | 行业详情 |
| CreateIndustry | 创建行业 |
| UpdateIndustry | 更新行业 |

---

## 附录 B：配置文件示例

### Interview 服务配置（app/interview/configs/config.yaml）

```yaml
server:
  http:
    addr: 0.0.0.0:8003
    timeout: 10s
  grpc:
    addr: 0.0.0.0:9003
    timeout: 10s

data:
  database:
    driver: postgres
    source: "host=localhost port=5434 user=postgres password=password dbname=makejob sslmode=disable"
  redis:
    addr: localhost:6384
    password: ""
    db: 1

ai:
  service_addr: "localhost:9014"

industry:
  service_addr: "localhost:9011"

archive:
  service_addr: "localhost:9005"

mq:
  url: "amqp://guest:guest@localhost:5672/"
  exchange: "makejob.async"

jwt:
  secret: "makejob-secret-key-change-in-production"
  expire_hours: 168
  service_secret: "makejob-internal-service-secret"
```

### Gateway 配置（app/gateway/configs/config.yaml）

```yaml
server:
  http:
    addr: 0.0.0.0:8082
    timeout: 10s

services:
  user:
    addr: "localhost:9004"
  question:
    addr: "localhost:9002"
  interview:
    addr: "localhost:9003"
  growth:
    addr: "localhost:9005"
  admin:
    addr: "localhost:9006"
  community:
    addr: "localhost:9007"

jwt:
  secret: "makejob-secret-key-change-in-production"
```

---

## 附录 C：Makefile 命令

```bash
make init              # go mod tidy
make api               # buf generate（Proto 代码生成）
make build             # 编译全部服务到 bin/
make build-interview   # 仅编译面试服务
make build-question    # 仅编译题目服务
make wire              # 运行 Wire 依赖注入生成
make test              # go test ./...
make clean             # 清理 bin/ 目录
```
---

## 2026-06-02 Bridge 接通记录

### 本次完成

- 在 `backend/bridge/` 新增可复用 bridge 运行时，由 root 模块通过 `makejob-backend/bridge` 直接复用单体内部真实能力。
- `app/admin` 已将以下 gRPC 占位实现替换为 bridge 真实调用：
  - 题目流水线同步生成与异步任务
  - AI 调试与提示词渲染
  - Live2D 模型包与背景导入
  - RAG 连接测试、索引、检索、文档同步
  - 爬虫源、搜索、抓取、清洗、导入、异步导入
- `app/gateway` 已直接挂载 backend HTTP handler，不再伪造以下能力：
  - `/api/interviews/:id/ws`
  - `/api/membership/*`
  - `/api/admin/question-pipeline/generate/stream`
  - `/api/live2d/models`
  - `/api/live2d/current`
  - `/live2d-assets/*`
- `backend/internal/scraper` 已新增真实 HTTP Provider，并在单体 `cmd/server`、`cmd/worker` 中替换掉 mock provider。

### Bridge 设计

- root 模块不能直接导入 `backend/internal/...`，因此采用 `backend/bridge` 作为公开复用层。
- bridge 只暴露两类能力：
  - 供 `admin` gRPC 使用的纯函数式服务包装
  - 供 `gateway` 直接挂载的 `gin.HandlerFunc`
- WebSocket 与 SSE 不经过 gRPC 转发，直接在 gateway 中挂载 bridge handler。
- gateway 在检测到 bridge 可用时，会停止注册重叠的 gRPC REST 代理路由，避免前端同一路径落到两套不同实现。
- gateway 会通过 bridge 模式专用鉴权中间件复用自身 JWT 配置，同时保持单体一致的错误响应结构，并把 JWT 中的 `uint64 user_id` 适配为 legacy handler 需要的 `uint`。
- gateway 会补齐单体基础端点语义，包括 `/` → `/api/health` 重定向、`/api/health`、`/api/health/ready` 与 `/metrics`。
- bridge 现已补齐原单体遗漏的受保护路由挂载，包括：
  - `/api/companion/chat`
  - `/api/user/study-logs/daily`
  - `/api/user/growth-summary`
  - `/api/user/weekly-focus`
- `app/admin` 的 AI 配置写入现在会先调用 `bridge.NormalizeAIConfigs`，复用单体的 `NormalizeRuntimeConfig + ValidateRuntimeConfig` 规则，非法 Provider、超时或数值范围错误会明确返回校验错误。
- `app/admin` 与无 bridge 直挂时的 `gateway` 已补齐 AI 调用日志筛选字段透传，`scene`、`source`、`status`、`trace_id`、`task_id` 与原单体后台保持一致。

### 配置要求

- `backend` bridge 初始化时默认读取以下配置文件之一：
  - `backend/config.yaml`
  - `config.yaml`
  - `../backend/config.yaml`
- 也可通过环境变量 `MAKEJOB_BACKEND_CONFIG` 显式指定 backend 配置文件路径。
- `app/gateway/configs/config.yaml` 已新增：

```yaml
data:
  database:
    driver: postgres
    source: "host=localhost port=5434 user=postgres password=password dbname=makejob sslmode=disable"
```

- 外部依赖未就绪时不再返回空数据，而是返回明确错误：
  - Milvus 或 Embedding 配置异常会在 RAG 接口中直接报错
  - 模型 API 未配置或调用失败会在 AI 调试接口中直接报错
  - Live2D 资源目录未就绪时，`/live2d-assets/*` 返回 `500`

### 构建与测试

- root：
  - `go build ./...`
  - `go test ./app/admin/... ./app/gateway/...`
- backend：
  - `cd backend && go build ./...`
  - `cd backend && go test ./...`

### 快速验证

1. 启动 `backend/config.yaml` 中依赖的 PostgreSQL、RabbitMQ、Milvus 以及模型服务配置。
2. 启动 `app/admin`、`app/gateway` 与 `backend`。
3. 验证 SSE：
   - `curl -N -X POST http://localhost:8082/api/admin/question-pipeline/generate/stream -H "Authorization: Bearer <admin-token>" -H "Content-Type: application/json" -d "{\"industry_code\":\"go\",\"requirement\":\"并发开发\",\"candidate_count\":3}"`
4. 验证爬虫：
   - `curl -X POST http://localhost:8082/api/admin/scraper/search -H "Authorization: Bearer <admin-token>" -H "Content-Type: application/json" -d "{\"source\":\"niuke\",\"keyword\":\"golang 面试题\",\"page\":1,\"page_size\":5}"`
5. 验证会员：
   - `curl http://localhost:8082/api/membership/plans`
   - `curl http://localhost:8082/api/membership/status -H "Authorization: Bearer <user-token>"`
6. 验证 Live2D：
   - `curl http://localhost:8082/api/live2d/models`
   - 浏览器访问 `http://localhost:8082/live2d-assets/...`
