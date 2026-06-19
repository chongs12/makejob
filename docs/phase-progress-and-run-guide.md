#  微服务改造 — 进度分析与运行指南

> 生成时间：2026-06-09

---

## 一、当前进度总览

### 1.1 整体阶段判断

**当前处于：Phase 6 已基本完成，Phase 7 尚未开始。**

所有 15 个微服务的骨架（proto、main.go、config.yaml、gRPC server 注册）均已就绪，核心 RPC 实现已落地，Bridge 已从 Gateway 中移除。

### 1.2 各 Phase 完成状态

| Phase | 预期目标 | 完成状态 | 说明 |
|-------|---------|---------|------|
| **Phase 0** — 基础设施就绪 | Proto 定义 + 7 个新服务骨架 | **已完成** | 全部 18 个 proto 文件已存在，15 个服务目录已搭建 |
| **Phase 1** — 基础设施服务 | CodeRunner/RAG/AI Gateway 全部 RPC | **已完成** | CodeRunner(2 RPC)、RAG(8 RPC + MQ)、AI Gateway(6 RPC) 均已实现 |
| **Phase 2** — User + Membership + Community | 修复/新增/补齐 RPC | **已完成** | User: Logout + UpdateUserRole; Membership: 11 个函数; Community: UpdatePost/ToggleLike/ListMyPosts |
| **Phase 3** — Question 域 | 22 RPC 全实现 + MQ | **已完成** | RunCode/GenerateTimedExam/SubmitExam/ListMistakeTopics 等均已实现，MQ consumer 已就位 |
| **Phase 4** — Interview + Realtime | 14 RPC + Realtime + MQ | **已完成** | GetNextQuestion/FinishInterview/GetReport/SubmitCodingAnswer 已实现；Realtime(5 RPC) 已实现；MQ consumer 已就位（interview + learning_archive） |
| **Phase 5** — Plan + Growth + Companion | 7+3+3 RPC + MQ | **已完成** | Plan: CreatePlan/AdjustPlan 等 + MQ; Growth: 4 个函数; Companion: 6 个函数 + TTS |
| **Phase 6** — Admin BFF + Bridge 移除 | Admin 改造 + 删除 Bridge | **已完成** | Gateway handler.go 中已无 bridge 引用（4278 行纯代理逻辑），Admin 已改为委托模式 |
| **Phase 7** — 优化与运维 | 健康检查/Tracing/CI-CD | **未开始** | 各服务无独立健康检查端点，无统一 Tracing，CI/CD 未配置 |

### 1.3 各服务实现详情

| # | 服务 | gRPC 端口 | RPC 数 | MQ Consumer | 测试文件 | 状态 |
|---|------|-----------|--------|-------------|---------|------|
| 1 | Gateway | HTTP :8080 | 代理 | — | handler_test.go | 完成 |
| 2 | User | :9001 | 9 | — | — | 完成 |
| 3 | Membership | :9002 | 8 | — | — | 完成 |
| 4 | Question | :9003 | 22 | mq.go | usecase_test.go | 完成 |
| 5 | Interview | :9004 | 14 | mq.go | usecase_test.go | 完成 |
| 6 | Realtime | :9005 (+WS :8085) | 5 | — | — | 完成 |
| 7 | Growth | :9006 | 3 | — | — | 完成 |
| 8 | Plan | :9007 | 7 | mq.go | — | 完成 |
| 9 | Companion | :9008 | 3 | — | companion_test.go, tts_client_test.go | 完成 |
| 10 | Community | :9009 | 9 | — | — | 完成 |
| 11 | LearningArchive | :9010 | 5 | mq.go | archive_test.go | 完成 |
| 12 | AI Gateway | :9011 | 6 | — | ai_test.go, admin_test.go | 完成 |
| 13 | RAG | :9012 | 8 | mq.go | rag_test.go | 完成 |
| 14 | CodeRunner | :9013 | 2 | — | coderunner_test.go | 完成 |
| 15 | Admin | :9014 | BFF | — | 4 个 test 文件 | 完成 |

### 1.4 测试覆盖情况

**可运行的测试包（15 个，全部通过）**：

```
ok  makejob/app/admin/internal/biz           0.662s
ok  makejob/app/admin/internal/data           0.627s
ok  makejob/app/admin/internal/service        2.960s
ok  makejob/app/ai_gateway/internal/biz       3.397s
ok  makejob/app/coderunner/internal/biz       2.277s
ok  makejob/app/companion/internal/data       0.455s
ok  makejob/app/companion/internal/service    2.635s
ok  makejob/app/gateway/internal/proxy        1.520s
ok  makejob/app/interview/internal/biz        2.262s
ok  makejob/app/learning_archive/internal/biz 1.080s
ok  makejob/app/learning_archive/internal/data 0.446s
ok  makejob/app/question/internal/biz         2.129s
ok  makejob/app/rag/internal/biz              2.113s
ok  makejob/app/rag/internal/service          1.298s
ok  makejob/pkg/live2dassets                  1.306s
```

**无测试文件的服务层**：User、Growth、Membership、Plan、Realtime、Community、Question(data/service)、Interview(data/service)。

### 1.5 未完成项（Phase 7 范围）

| 任务 | 说明 |
|------|------|
| P7-1 健康检查 | 各服务需实现 gRPC Health Check 协议 |
| P7-2 链路追踪 | OpenTelemetry 已引入依赖，但未在各服务中初始化 TracerProvider |
| P7-3 CI/CD | 无 pipeline 配置，Makefile 仅构建 7 个服务（缺少 8 个新服务） |
| Makefile 更新 | 需补充 ai_gateway/rag/coderunner/realtime/membership/plan/companion/learning_archive 的构建目标 |

---

## 二、编译与测试运行指南

### 2.1 前置条件

| 依赖 | 版本要求 | 说明 |
|------|---------|------|
| Go | 1.22+（当前 go.mod 声明 1.25.8） | `go version` 检查 |
| PostgreSQL | 14+ | 各服务共享/独立数据库 |
| RabbitMQ | 3.x | MQ 消费者依赖 |
| Redis | 6+ | User 服务 Token 黑名单 |
| Milvus | 2.x | RAG 服务向量检索 |
| Piston | 自部署 | CodeRunner 服务代码执行引擎 |
| etcd | 3.5+ | 服务发现（可选，开发环境可直连） |

### 2.2 编译

```bash
# 编译全部服务（推荐）
go build ./...

# 编译单个服务（示例）
go build -o bin/gateway   ./app/gateway/cmd/server
go build -o bin/user      ./app/user/cmd/server
go build -o bin/question  ./app/question/cmd/server
go build -o bin/interview ./app/interview/cmd/server
go build -o bin/growth    ./app/growth/cmd/server
go build -o bin/admin     ./app/admin/cmd/server
go build -o bin/community ./app/community/cmd/server

# 新增的 8 个服务
go build -o bin/membership       ./app/membership/cmd/server
go build -o bin/plan             ./app/plan/cmd/server
go build -o bin/companion        ./app/companion/cmd/server
go build -o bin/realtime         ./app/realtime/cmd/server
go build -o bin/ai_gateway       ./app/ai_gateway/cmd/server
go build -o bin/rag              ./app/rag/cmd/server
go build -o bin/coderunner       ./app/coderunner/cmd/server
go build -o bin/learning_archive ./app/learning_archive/cmd/server
```

### 2.3 运行测试

```bash
# 运行全部测试
go test ./...

# 运行指定包测试（跳过缓存）
go test -count=1 ./app/interview/internal/biz/...

# 运行指定测试函数
go test -run TestFuncName ./app/admin/internal/service/...

# 查看测试详细输出
go test -v ./app/coderunner/internal/biz/...

# 运行带覆盖率的测试
go test -cover ./app/rag/internal/biz/...
```

### 2.4 各服务启动命令

> 所有服务均需在项目根目录 `D:\gogogo\makejob` 下执行。
> 各服务的配置文件在 `app/<service>/configs/config.yaml`，启动前请确认数据库连接、MQ 地址等配置正确。

#### 基础设施层（无业务依赖，优先启动）

```bash
# CodeRunner 服务（gRPC :9013）— 无外部依赖，可直接启动
go run ./app/coderunner/cmd/server -conf ./app/coderunner/configs/config.yaml

# AI Gateway 服务（gRPC :9011）— 依赖 LLM API Key
go run ./app/ai_gateway/cmd/server -conf ./app/ai_gateway/configs/config.yaml

# RAG 服务（gRPC :9012）— 依赖 Milvus + Embedding API
go run ./app/rag/cmd/server -conf ./app/rag/configs/config.yaml
```

#### 数据层（依赖 PostgreSQL）

```bash
# User 服务（gRPC :9001）— 依赖 PostgreSQL + Redis
go run ./app/user/cmd/server -conf ./app/user/configs/config.yaml

# Membership 服务（gRPC :9002）— 依赖 PostgreSQL
go run ./app/membership/cmd/server -conf ./app/membership/configs/config.yaml

# Question 服务（gRPC :9003）— 依赖 PostgreSQL + CodeRunner + AI Gateway
go run ./app/question/cmd/server -conf ./app/question/configs/config.yaml

# Community 服务（gRPC :9009）— 依赖 PostgreSQL
go run ./app/community/cmd/server -conf ./app/community/configs/config.yaml

# LearningArchive 服务（gRPC :9010）— 依赖 PostgreSQL + RabbitMQ
go run ./app/learning_archive/cmd/server -conf ./app/learning_archive/configs/config.yaml
```

#### 业务层（依赖多个下游服务）

```bash
# Interview 服务（gRPC :9004）— 依赖 AI Gateway + RAG + CodeRunner + LearningArchive
go run ./app/interview/cmd/server -conf ./app/interview/configs/config.yaml

# Realtime 服务（gRPC :9005 + WebSocket :8085）— 依赖 Interview + RAG + AI Gateway
go run ./app/realtime/cmd/server -conf ./app/realtime/configs/config.yaml

# Plan 服务（gRPC :9007）— 依赖 AI Gateway + LearningArchive
go run ./app/plan/cmd/server -conf ./app/plan/configs/config.yaml

# Growth 服务（gRPC :9006）— 依赖 LearningArchive + Interview + Plan + Question
go run ./app/growth/cmd/server -conf ./app/growth/configs/config.yaml

# Companion 服务（gRPC :9008）— 依赖 AI Gateway + LearningArchive + Plan
go run ./app/companion/cmd/server -conf ./app/companion/configs/config.yaml
```

#### 管理与入口层

```bash
# Admin 服务（gRPC :9014）— BFF，依赖各域服务
go run ./app/admin/cmd/server -conf ./app/admin/configs/config.yaml

# Gateway（HTTP :8080）— 最后启动，依赖所有 gRPC 服务
go run ./app/gateway/cmd/server -conf ./app/gateway/configs/config.yaml    
```

### 2.5 推荐启动顺序

```
1. 基础设施：CodeRunner → AI Gateway → RAG
2. 数据服务：User → Membership → Community → LearningArchive → Question
3. 业务服务：Interview → Realtime → Plan → Growth → Companion
4. 入口层：Admin → Gateway
```

### 2.6 开发环境最小启动（仅测试部分功能）

如果只想测试某个服务，可只启动它和它的直接依赖：

```bash
# 示例：只测试 CodeRunner（无依赖）
go run ./app/coderunner/cmd/server -conf ./app/coderunner/configs/config.yaml

# 示例：只测试 Question 的 RunCode（需要 CodeRunner）
go run ./app/coderunner/cmd/server -conf ./app/coderunner/configs/config.yaml &
go run ./app/question/cmd/server -conf ./app/question/configs/config.yaml
```

### 2.7 Proto 代码生成

```bash
# 使用 buf 生成（推荐）
buf generate

# 或手动 protoc
protoc --go_out=. --go-grpc_out=. api/makejob/*/v1/*.proto
```

### 2.8 Wire 依赖注入生成（仅 interview 和 question 使用 wire）

```bash
cd app/interview && wire ./...
cd app/question && wire ./...
```

---

## 三、配置文件模板

各服务的 `configs/config.yaml` 结构遵循 Kratos 标准。以下为通用模板：
go run ./app/gateway/cmd/server -conf ./app/gateway/configs/config.yaml
```yaml
server:
  grpc:
    addr: "0.0.0.0:<port>"
    timeout: 10s

data:
  database:
    driver: postgres
    source: "host=localhost user=postgres password=postgres dbname=makejob_<service> port=5432 sslmode=disable"
  redis:
    addr: "localhost:6379"
    db: 0

# 各服务特有配置（示例）
piston:
  endpoint: "http://localhost:2000"
  timeout_ms: 10000

milvus:
  addr: "localhost:19530"
  collection: "makejob_questions"

rabbitmq:
  url: "amqp://guest:guest@localhost:5672/"
```

---

## 四、Makefile 建议更新

当前 Makefile 仅构建 7 个服务，建议更新为：

```makefile
.PHONY: build test clean

SERVICES = gateway user membership question interview realtime growth plan companion community learning_archive ai_gateway rag coderunner admin

build:
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		go build -o bin/$$svc ./app/$$svc/cmd/server; \
	done

build-%:
	go build -o bin/$* ./app/$*/cmd/server

test:
	go test ./...

test-v:
	go test -v ./...

clean:
	rm -rf bin/
```
