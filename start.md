# MakeJob 启动说明

本文档基于当前代码状态更新于 `2026-05-15`，用于说明仓库的真实启动方式和依赖关系。

## 1. 当前主线

- 当前前端主线是 `frontend-react`，采用 `React 19 + Vite + TanStack Router + TanStack Query + Zustand`。
- 旧 Nuxt 前端已不再作为当前开发基线，本仓库现阶段以前后端分离的 React 工作区为准。
- 后端主入口是 `backend/cmd/server/main.go`。
- 异步任务消费入口是 `backend/cmd/worker/main.go`。

## 2. 运行前依赖

启动前至少准备以下依赖：

- PostgreSQL
- Redis
- 可选的 Piston 代码执行服务
- 可选的 AI / TTS / ASR 第三方配置

当前默认开发配置来自 `backend/config.yaml`：

- HTTP 服务端口：`8082`
- PostgreSQL：`localhost:5434`
- Redis：`localhost:6384`
- Piston：`http://localhost:2000/api/v2/execute`

## 3. 启动命令

### 3.1 启动后端 API

```bash
cd d:/gogogo/makejob/backend
go run cmd/server/main.go
```

说明：

- 启动后会自动尝试连接数据库和 Redis。
- 数据库可用时会执行迁移、基础种子和管理员引导。
- 健康检查地址为 `http://localhost:8082/api/health`。

### 3.2 启动异步 Worker

```bash
cd d:/gogogo/makejob/backend
go run cmd/worker/main.go
```

说明：

- 当前 worker 负责消费异步导入任务和题目流水线生成任务。
- 如果你只验证同步接口，可暂时不启动 worker。
- 如果你要验证后台的异步题目流水线或异步导入，必须额外启动 worker。

### 3.3 启动前台 Web

```bash
cd d:/gogogo/makejob/frontend-react
npm run dev -w @makejob/web
```

或：

```bash
cd d:/gogogo/makejob/frontend-react
npm run dev:web
```

### 3.4 启动后台 Admin

```bash
cd d:/gogogo/makejob/frontend-react
npm run dev -w @makejob/admin
```

或：

```bash
cd d:/gogogo/makejob/frontend-react
npm run dev:admin
```

## 4. 建议启动顺序

建议按以下顺序启动：

1. 启动 PostgreSQL 和 Redis
2. 启动 `backend` API 服务
3. 按需启动 `worker`
4. 启动 `frontend-react` 的 `web` 或 `admin`

## 5. 当前功能入口

前台 `web` 当前已经接入这些主路径：

- `/`
- `/practice`
- `/community`
- `/interview`
- `/companion`
- `/growth`
- `/auth/login`

后台 `admin` 当前已经接入这些主路径：

- `/dashboard`
- `/runtime`
- `/ai-configs`
- `/prompts`
- `/live2d`
- `/tts`
- `/taxonomy`
- `/question-pipeline`
- `/questions`
- `/auth/login`

## 6. 当前验证结论

基于 `2026-05-15` 的本地核验：

- `backend` 执行 `go test ./...` 通过
- `frontend-react` 执行 `npm run build` 通过
- `frontend-react` 执行 `npm run test:web` 通过

## 7. 需要注意的现实约束

- AI 面试、学习计划和题目流水线虽然都已落代码，但完整效果依赖数据库数据和第三方配置。
- 配置文件当前包含开发环境信息，接手前应先检查环境变量与敏感信息治理策略。
- 后台异步任务相关能力如果只启动 API、未启动 worker，会表现为“可创建任务但不消费”。
