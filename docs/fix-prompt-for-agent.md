# 微服务 Gateway 响应格式修复 — 智能体执行提示词

## 一、任务背景

MakeJob 项目刚完成从单体架构到微服务架构的迁移（Phase 0-6）。迁移后，前端页面大面积报错，根因是 **Gateway 返回的 gRPC 响应结构与前端期望的 JSON 结构不匹配**。

### 架构数据流

```
前端(React) → Vite代理 → Gateway(Gin HTTP) → gRPC → 各微服务 → PostgreSQL
```

- 前端所有 API 调用通过 `requestJson<ApiEnvelope<T>>()` 发送
- 前端期望响应格式：`{ code: number, message: string, data: T }`
- Gateway 已有 `WrapResponseMiddleware` 自动包装 envelope + camelCase→snake_case + 单字段解包
- **但数据结构本身不匹配**：proto message 的字段、类型、嵌套结构与前端 TypeScript 接口不同

### 为什么单体正常

单体架构中，handler 直接从 DB 读数据并组装成前端需要的 JSON 结构。微服务改造后，Gateway handler 只是把 gRPC protobuf 对象原样序列化返回。proto 定义是按新架构设计的，没有对齐前端已有的数据结构。

## 二、核心问题

**不是格式问题（snake_case vs camelCase），是数据模型不匹配。**

中间件已处理：envelope 包装、camelCase→snake_case、单字段对象解包。

但以下问题需要在 handler 层解决：
1. proto 缺少前端需要的字段
2. proto 字段名与前端期望不同
3. proto 嵌套结构与前端期望不同
4. proto 某些字段是 string 但前端期望是 object

## 三、详细字段不匹配清单

**完整清单见：`docs/proto-frontend-mismatch-report.md`（1048 行，覆盖全部 8 个服务）**

以下是最严重的 10 个问题：

### 3.1 Growth 服务（最严重 — 几乎所有字段都不匹配）

- Proto `GrowthSummary` 只有 7 个字段（`total_study_days`, `total_questions`, `total_interviews`, `current_streak`, `avg_score`, `weekly_stats`, `weak_topics`）
- 前端 `GrowthSummaryResponse` 期望 12 个顶层字段 + 60+ 子字段（`practice_stats`, `current_plan`, `focus_signals`, `trend_summary`, `recent_study_logs`, `recent_interviews`, `recent_plans` 等）
- 前端期望 `study_days`，proto 是 `total_study_days`
- 前端期望 `interview_count`，proto 是 `total_interviews`
- 前端期望 `average_interview_score`，proto 是 `avg_score`

### 3.2 Interview 服务

- `InterviewDetail` 缺少 `score`, `total_questions`, `current_question`, `started_at`, `ended_at`, `async_task_id`, `task_status`, `task_error`
- `CodingDiagnosis.evidence`：proto 是 `string`（`evidence_summary`），前端期望 `string[]`

### 3.3 Companion 服务

- `CompanionPlanDetail`（来自 plan.proto）缺 12 个字段：`phase`, `industry_id`, `target_role`, `start_date`, `end_date`, `async_task_id`, `task_status`, `task_error`, `adjustment_count`, `last_adjustment_at`, `next_task_source`, `next_task_reason`

### 3.4 Question 服务

- `QuestionDetail` 缺少 `solution`（结构化对象）、`judge_config`（结构化对象）、`answer_template`（结构化对象）
- 推荐接口 `RecommendedQuestion` 只有 5 字段，前端期望 17 字段
- `QuestionTagTaxonomyGroup`：proto 用 `category`，前端期望 `group`

### 3.5 Community 服务

- `PostSummary` 缺少 `post_type`, `summary`, `tags`, `view_count`, `is_pinned`, `is_recommended`, `updated_at`, `is_author`, `author` 对象

### 3.6 Admin 服务

- `AdminConfigItem`：proto 用 `key`/`value`，前端期望 `config_key`/`config_value`
- `AICallLog` 列表消息缺少 `trace_id`, `source`, `scene`, `provider`, `model_error`, `is_success`

## 四、关键文件位置

### Proto 定义
- `api/makejob/growth/v1/growth.proto`
- `api/makejob/companion/v1/companion.proto`
- `api/makejob/plan/v1/plan.proto`
- `api/makejob/interview/v1/interview.proto`
- `api/makejob/question/v1/question.proto`
- `api/makejob/community/v1/community.proto`
- `api/makejob/admin/v1/admin.proto`
- `api/makejob/membership/v1/membership.proto`
- `api/makejob/user/v1/user.proto`
- `api/makejob/shared/v1/base.proto`
- `api/makejob/shared/v1/pagination.proto`

### Proto 生成代码
- `api/makejob/*/v1/*.pb.go`
- `api/makejob/*/v1/*_grpc.pb.go`

### Gateway Handler
- `app/gateway/internal/proxy/handler.go`（约 4000 行，所有 HTTP→gRPC 转换逻辑）
- `app/gateway/internal/proxy/handler_test.go`
- `app/gateway/cmd/server/main.go`
- `app/gateway/internal/conf/conf.go`

### Gateway 中间件
- `WrapResponseMiddleware` 在 `handler.go` 中（自动 envelope 包装 + camelCase→snake_case + 单字段解包）
- `envelopeWriter` 结构体（捕获 handler 响应 body）
- `unwrapGRPCResponse` 函数（解包 + 键名转换）
- `camelToSnake` 函数

### 微服务 Handler（Gateway 调用的 gRPC 服务端）
- `app/growth/internal/service/growth.go`
- `app/interview/internal/service/interview.go`
- `app/companion/internal/service/companion.go`
- `app/plan/internal/service/plan.go`
- `app/question/internal/service/question.go`
- `app/community/internal/service/community.go`
- `app/admin/internal/service/admin.go`
- `app/user/internal/service/user.go`
- `app/membership/internal/service/membership.go`

### 微服务 Data 层（数据库访问）
- `app/*/internal/data/*_repo.go`
- `app/*/internal/data/data.go`

### 前端类型定义
- `frontend-react/packages/shared-types/src/index.ts`（ApiEnvelope, LoginPayload 等）
- `frontend-react/packages/api-client/src/index.ts`（requestJson, getApiBaseUrl）

### 前端页面（消费 API 的地方）
- `frontend-react/apps/web/src/features/growth/GrowthPage.tsx`
- `frontend-react/apps/web/src/features/interview/interviewApi.ts`
- `frontend-react/apps/web/src/features/companion/companionApi.ts`
- `frontend-react/apps/web/src/features/companion/companionTypes.ts`
- `frontend-react/apps/web/src/features/practice/PracticePage.tsx`
- `frontend-react/apps/web/src/features/practice/PracticeDetailPages.tsx`
- `frontend-react/apps/web/src/features/community/CommunityPages.tsx`
- `frontend-react/apps/web/src/shared/practiceRecommendations.ts`
- `frontend-react/apps/web/src/shared/mistakeTopics.ts`
- `frontend-react/apps/web/src/shared/weeklyFocus.ts`
- `frontend-react/apps/admin/src/features/*/`（admin 各页面）

## 五、修复策略

### 推荐方案：修改 proto + Gateway handler 层做响应转换

**不要修改前端。** 前端已经按单体的 API 约定写好了，改动面太大。

**修改步骤：**

1. **修改 proto 定义**：在 `api/makejob/*/v1/*.proto` 中添加缺失的 message 和字段，使 proto 响应结构与前端期望一致
2. **重新生成 proto 代码**：`buf generate` 或 `protoc` 生成 `*.pb.go` 和 `*_grpc.pb.go`
3. **修改微服务 service 层**：在 `app/*/internal/service/*.go` 中填充新增的字段（从 DB 或其他服务获取数据）
4. **修改 Gateway handler**：在 `app/gateway/internal/proxy/handler.go` 中做字段重命名和结构转换（如 `total_study_days` → `study_days`）
5. **运行测试**：`go test ./...` 确保不破坏已有功能
6. **运行前端验证**：启动 Gateway + 前端 dev server，逐页验证

### 关键原则

- Proto 字段用 snake_case（protobuf 规范）
- 前端期望 snake_case（已由中间件处理 camelCase→snake_case 转换）
- 如果 proto 字段名与前端期望不同，在 Gateway handler 中做映射
- 如果 proto 缺少字段，添加到 proto 并在 service 层填充
- 如果前端期望嵌套对象但 proto 是 flat 字段，添加子 message

## 六、验证方法

```bash
# 1. 编译
go build ./app/gateway/cmd/server

# 2. 运行测试
go test ./app/gateway/... -count=1

# 3. 测试 API 响应
TOKEN=$(curl -s http://localhost:8082/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"email":"admin@makejob.com","password":"admin123456"}' | 提取token)
curl -s http://localhost:8082/api/v1/growth/summary -H "Authorization: Bearer $TOKEN"
# 应返回包含 practice_stats, current_plan, focus_signals 等字段的完整 JSON

# 4. 前端验证
# 访问 http://localhost:3001 逐页检查是否还有 "Something went wrong" 错误
```

## 七、优先级

按影响范围排序：

1. **P0 — Growth 服务**：成长档案页、首页都依赖此接口，几乎所有字段都不匹配
2. **P0 — Interview 服务**：面试详情页核心字段缺失
3. **P1 — Question 服务**：题库页、练习页依赖，推荐接口字段严重不足
4. **P1 — Companion 服务**：陪伴页依赖，Plan 子结构缺失
5. **P2 — Community 服务**：社区页字段缺失
6. **P2 — Admin 服务**：管理后台部分字段不匹配
7. **P3 — 其他**：Membership、User 等已基本正常

## 八、约束

- **不要修改前端代码**（前端已按单体 API 约定实现，改动面太大）
- **不要修改 `WrapResponseMiddleware`**（它已正确处理 envelope 包装、camelCase→snake_case、单字段解包）
- **不要修改已有的 proto 字段编号**（会破坏 wire format 兼容性）
- **新增 proto 字段使用新的编号**（不能与已有编号冲突）
- **所有修改必须通过** `go build ./...` **和** `go test ./...`
- **proto 修改后必须重新生成 Go 代码**
