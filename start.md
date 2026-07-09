# MakeJob 启动说明

> 更新于 `2026-07-09`。后端已从单体 `backend/` 迁移为 15 个 Kratos 微服务（单体归档于 `docs/backend/`）。完整的进度、端口表与运行指南见 [`docs/phase-progress-and-run-guide.md`](docs/phase-progress-and-run-guide.md)，本文仅给快速启动。

## 1. 当前主线

- **后端**：`app/` 下 15 个微服务（Kratos DDD 四层）+ 1 个 BFF Gateway。Gateway 是唯一 HTTP 入口（`:8080`），对外暴露 `/api/v1/*`，内部通过 gRPC 调用各服务。
- **前端**：`frontend-react/` 工作区，React 19 + Vite 7 + TanStack Router/Query + Zustand，含 `apps/web`（用户前台）和 `apps/admin`（管理后台）。
- 单体 `backend/` 已废弃并归档至 `docs/backend/`，**不要再启动它**。

## 2. 运行前依赖

| 依赖 | 用途 | 默认地址 |
|------|------|---------|
| PostgreSQL | 各服务数据存储 | `localhost:5432`（各服务 config 可独立配置） |
| Redis | User 服务 Token 黑名单 / 缓存 | `localhost:6379` |
| RabbitMQ | 异步任务（面试报告、RAG 同步、计划生成等） | `amqp://localhost:5672` |
| Milvus | RAG 向量检索 | `localhost:19530` |
| Piston | CodeRunner 代码执行沙箱 | `http://localhost:2000` |
| etcd | 服务发现（开发环境可直连，可选） | `localhost:2379` |

各服务的连接配置在 `app/<service>/configs/config.yaml`，启动前请确认数据库 / MQ / 第三方 API Key 正确。

## 3. 启动命令

所有命令在项目根目录 `D:\gogogo\makejob` 下执行。

### 3.1 编译

```bash
go build ./...          # 编译全部服务
make build              # 或用 Makefile（当前仅构建 7 个核心服务）
```

### 3.2 启动后端微服务

每个服务独立启动，指定各自配置：

```bash
go run ./app/<service>/cmd/server -conf ./app/<service>/configs/config.yaml
```

15 个服务及其端口：

| 服务 | 端口 | 服务 | 端口 |
|------|------|------|------|
| Gateway（HTTP 入口） | :8080 | Companion | :9008 |
| User | :9001 | Community | :9009 |
| Membership | :9002 | LearningArchive | :9010 |
| Question | :9003 | AI Gateway | :9011 |
| Interview | :9004 | RAG | :9012 |
| Realtime（+WS :8085） | :9005 | CodeRunner | :9013 |
| Growth | :9006 | Admin（BFF） | :9014 |
| Plan | :9007 | | |

### 3.3 推荐启动顺序

1. 基础设施：CodeRunner → AI Gateway → RAG
2. 数据服务：User → Membership → Community → LearningArchive → Question
3. 业务服务：Interview → Realtime → Plan → Growth → Companion
4. 入口层：Admin → **Gateway（最后启动，依赖所有 gRPC 服务）**

> 最小验证：只想跑某个服务，启动它和它的直接依赖即可（如只测 CodeRunner 无需任何依赖）。

### 3.4 启动前端

```bash
cd frontend-react
npm run dev:web      # 用户前台
npm run dev:admin    # 管理后台
```

前端 dev 服务将 `/api/v1` 代理到 Gateway（`:8080`）。

## 4. 测试与生成

```bash
go test ./...                    # 全部后端测试
buf generate                     # Proto 代码生成
cd app/interview && wire ./...   # Wire 依赖注入（仅 interview / question 用 wire）
```

## 5. 注意事项

- 异步任务（面试报告生成、RAG 索引同步、学习计划生成）依赖 RabbitMQ；MQ 未启动时相关流程会降级或失败。
- AI 面试 / 学习计划 / RAG 的完整效果依赖数据库种子数据与第三方（火山引擎 / MiniMax 等）API 配置。
- 健康检查端点、OpenTelemetry 链路追踪、CI/CD 属 Phase 7，**尚未实现**。
- 更多细节（各服务 RPC 清单、测试覆盖、Makefile 更新建议）见 `docs/phase-progress-and-run-guide.md`。
