# MakeJob 微服务改造 Phase 0-6 全面审查报告

> **审查日期**: 2026-06-08
> **审查范围**: Phase 0-6 所有代码 + 补丁阶段 1-3
> **审查依据**: docs/开发准则.md、docs/microservice-redesign.md、docs/task-execution-library.md
> **参考基准**: docs/backend/ 单体架构代码

---

## 一、总体评估

| 维度 | 状态 | 说明 |
|------|------|------|
| Proto 定义 | 15 个服务、184 个 RPC | 覆盖完整 |
| Service 实现 | 163/184 RPC 真实实现 | 87.6% 完成率 |
| Gateway 路由 | 基本覆盖 | 缺 5 条路由 |
| 编译状态 | 全部通过 | `go build ./...` OK |
| 测试状态 | 全部通过 | `go test ./...` OK |

**结论**: Phase 0-6 的核心功能改造基本完成，14 个服务中 12 个达到 100% 实现。但存在功能遗漏、开发准则违规、以及基础设施层缺失等问题需要后续修复。

---

## 二、未实现的桩方法（2 处）

### 2.1 Admin: ImportLive2DPackage

| 项目 | 内容 |
|------|------|
| 文件 | `app/admin/internal/service/admin.go:954-957` |
| 现状 | 返回 `ServiceUnavailable("UNIMPLEMENTED", "Live2D 模型包导入待 Companion 微服务实现后接入")` |
| Gateway 路由 | 已注册 `POST /api/admin/live2d-models/import`，前端调用必失败 |
| 根因 | Companion 服务无 Live2D 包导入能力 |

### 2.2 Admin: ImportLive2DBackground

| 项目 | 内容 |
|------|------|
| 文件 | `app/admin/internal/service/admin.go:959-962` |
| 现状 | 返回 `ServiceUnavailable("UNIMPLEMENTED", "Live2D 背景资源导入待 Companion 微服务实现后接入")` |
| Gateway 路由 | 已注册 `POST /api/admin/live2d-models/backgrounds/import`，前端调用必失败 |
| 根因 | 同上 |

---

## 三、单体功能未迁移（9 处）

### 3.1 ChangePassword（全缺）

| 项目 | 内容 |
|------|------|
| 单体位置 | `docs/backend/internal/handler/auth_handler.go:179`（单体也是 501 桩） |
| 微服务现状 | user.proto 无此 RPC，service 无此方法，gateway 无此路由 |
| 影响 | 用户无法修改密码 |
| 修复建议 | user.proto 新增 `ChangePassword` RPC → user service 实现 → gateway 注册路由 |

### 3.2 GetProgress（Plan 专用进度统计，全缺）

| 项目 | 内容 |
|------|------|
| 单体位置 | `docs/backend/internal/handler/plan_handler.go:327` — 返回 `PlanProgressResponse` |
| 微服务现状 | plan.proto 无此 RPC，PlanDetail 只有 `progress` float 字段 |
| 影响 | 用户无法查看计划的详细进度统计 |
| 修复建议 | plan.proto 新增 `GetPlanProgress` RPC → plan service 实现 |

### 3.3 ListOrders（Membership，Gateway 路由缺失）

| 项目 | 内容 |
|------|------|
| 单体位置 | `docs/backend/internal/handler/membership_handler.go:195` |
| 微服务现状 | membership.proto 有 `ListOrders` RPC，service 已实现，**但 gateway 未注册路由** |
| 影响 | 用户无法查看订单历史 |
| 修复建议 | Gateway 添加 `GET /api/v1/membership/orders` 路由 |

### 3.4 GetOrder（Membership 订单详情，Gateway 路由缺失）

| 项目 | 内容 |
|------|------|
| 单体位置 | `docs/backend/internal/handler/membership_handler.go` |
| 微服务现状 | membership.proto 有 `GetOrder` RPC，**但 gateway 未注册路由** |
| 影响 | 用户无法查看单个订单详情 |
| 修复建议 | Gateway 添加 `GET /api/v1/membership/orders/:id` 路由 |

### 3.5 ListPlans（Membership 可购买套餐，Gateway 路由缺失）

| 项目 | 内容 |
|------|------|
| 单体位置 | `docs/backend/internal/handler/membership_handler.go:31` |
| 微服务现状 | membership.proto 有 `ListPlans` RPC，**但 gateway 未注册路由** |
| 影响 | 用户无法浏览可购买的会员套餐 |
| 修复建议 | Gateway 添加 `GET /api/v1/membership/plans` 路由 |

### 3.6 HandlePaymentCallback（Membership 支付回调，Gateway 路由缺失）

| 项目 | 内容 |
|------|------|
| 单体位置 | `docs/backend/internal/handler/membership_handler.go:229` |
| 微服务现状 | membership.proto 有 `HandlePaymentCallback` RPC，**但 gateway 未注册路由** |
| 影响 | 支付回调无法到达服务 |
| 修复建议 | Gateway 添加 `POST /api/v1/membership/callback` 路由 |

### 3.7 Live2D 前端查询接口（全缺）

| 项目 | 内容 |
|------|------|
| 单体位置 | `docs/backend/internal/handler/live2d_handler.go` — `ListSelectableModels`、`GetCurrentModel` |
| 微服务现状 | Admin 只有 CRUD 管理接口，Companion 无 Live2D 查询 RPC，gateway 无前端路由 |
| 影响 | 前端无法查询可用的 Live2D 模型，伴侣/面试的视觉体验缺失 |
| 修复建议 | companion.proto 新增 `ListLive2DModels` / `GetCurrentModel` RPC |

### 3.8 GetMistakeTopic（单个错题主题详情，全缺）

| 项目 | 内容 |
|------|------|
| 单体位置 | `docs/backend/internal/handler/question_handler.go:273` |
| 微服务现状 | question.proto 只有 `ListMistakeTopics`，无单个详情 RPC |
| 影响 | 用户无法查看单个错题主题的详细内容 |
| 修复建议 | question.proto 新增 `GetMistakeTopic` RPC |

### 3.9 Casbin RBAC + 分布式限流（基础设施层缺失）

| 项目 | 内容 |
|------|------|
| 单体实现 | Casbin RBAC 角色鉴权 + Redis 滑动窗口分布式限流 |
| 微服务现状 | Gateway 仅有简单 `AdminMiddleware` 角色检查，无 Casbin，无限流 |
| 影响 | 鉴权粒度不足，无 API 限流保护 |
| 修复建议 | Phase 7 运维阶段处理 |

---

## 四、开发准则违规（14 处）

### 4.1 HIGH: 多表写入缺少事务（4 处）

| # | 文件 | 方法 | 行号 | 问题 |
|---|------|------|------|------|
| 1 | `app/interview/internal/biz/usecase.go` | `CreateInterview` | 88-119 | 创建面试记录 + 创建首条消息，无事务包裹 |
| 2 | `app/interview/internal/biz/usecase.go` | `SubmitAnswer` | 125-218 | 写用户答案 + 写 AI 反馈 + 写下一题 + 更新进度，4 次写入无事务 |
| 3 | `app/interview/internal/biz/usecase.go` | `SubmitCodingAnswer` | 551-652 | 写代码消息 + 写编码尝试 + 写 AI 反馈，无事务 |
| 4 | `app/learning_archive/internal/biz/archive.go` | `HandleInterviewFinished` | 98-149 | 批量创建归档条目，部分失败会导致数据不一致 |

**违反准则**: 2.2 — 多表写入必须使用事务

**修复方案**: 在 data 层 repo 新增事务方法，biz 层调用事务包裹多表写入。参考 plan service 的 `CreatePlan` 实现（已正确使用事务）。

### 4.2 HIGH: MQ 消费者缺乏幂等保护（4 处）

| # | 文件 | 方法 | 行号 | 问题 |
|---|------|------|------|------|
| 1 | `app/interview/internal/server/mq.go` | `handleResumeParse` | 32-38 | 重复消费浪费 AI 调用 |
| 2 | `app/interview/internal/server/mq.go` | `handleArchivePersist` | 48-54 | 重复消费创建重复归档条目 |
| 3 | `app/question/internal/server/mq.go` | `handleScraperImport` | 266-303 | 重复消费创建重复题目 |
| 4 | `app/learning_archive/internal/server/mq.go` | `handleInterviewFinished` | 52-61 | 重复消费创建重复归档条目 |

**违反准则**: 3.1 — MQ Consumer 必须是幂等的

**修复方案**: 每个 consumer 开头增加幂等检查（查 DB 是否已处理），参考 plan service 的 `GeneratePlan` 实现（已正确检查 `existingCount > 0`）。

### 4.3 MEDIUM: 错误处理不当（2 处）

| # | 文件 | 行号 | 问题 |
|---|------|------|------|
| 1 | `app/admin/internal/service/admin.go` | 737 | `fmt.Errorf("ai preset %d not found")` 应改为 `errors.NotFound(...)` |
| 2 | `app/admin/internal/service/bridge_helpers.go` | 28 | `fmt.Errorf("marshal metadata failed")` 应改为 `errors.InternalServer(...)` |

**违反准则**: 4.1 — 禁止 `fmt.Errorf` 返回给 gRPC，必须用 kratos errors

### 4.4 MEDIUM: BaseModel 缺少 DeletedAt（4 处）

| # | 文件 | 问题 |
|---|------|------|
| 1 | `pkg/model/base.go` | 全局 BaseModel 无 `DeletedAt gorm.DeletedAt`，所有使用此 BaseModel 的服务均不支持软删除 |
| 2 | `app/community/internal/data/model/community_like.go` | CommunityLike 无软删除，`Delete` 执行硬删除 |
| 3 | `app/question/internal/data/model/industry.go` | Industry 无 BaseModel 嵌入，无软删除 |
| 4 | `app/admin/internal/data/model/admin_config.go` | AdminConfig 无软删除（可能是有意为之） |

**违反准则**: 5.1 — 软删除是默认选项

---

## 五、Proto 孤儿服务（1 处）

### IndustryService

| 项目 | 内容 |
|------|------|
| Proto | `api/makejob/industry/v1/industry.proto` 定义了 4 个 RPC（ListIndustries, GetIndustry, CreateIndustry, UpdateIndustry） |
| 实现 | 无 `app/industry/` 目录，无任何 service 实现 |
| 实际路由 | 行业读取走 QuestionService，行业写入走 AdminService |
| 建议 | 删除孤儿 proto 或标记为废弃，避免混淆 |

---

## 六、Gateway 路由缺失汇总

| # | 缺失路由 | 对应 RPC | 优先级 |
|---|----------|----------|--------|
| 1 | `GET /api/v1/membership/plans` | Membership.ListPlans | HIGH |
| 2 | `GET /api/v1/membership/orders` | Membership.ListOrders | HIGH |
| 3 | `GET /api/v1/membership/orders/:id` | Membership.GetOrder | MEDIUM |
| 4 | `POST /api/v1/membership/callback` | Membership.HandlePaymentCallback | LOW |
| 5 | `DELETE /api/v1/community/posts/:id` | Community.DeletePost | MEDIUM |

---

## 七、RAG 服务特殊说明

| RPC | 状态 | 说明 |
|-----|------|------|
| UpdateConfig | 返回 501 | 明确拒绝运行时修改，admin_configs 表写入代替 |
| GetDocumentStats | TotalQuestions=TotalDocuments | 当前集合仅存题目，语义正确但需注意 |
| LastIndexedAt | 返回 nil | 不伪造时间戳，符合准则 4（禁止伪成功） |

---

## 八、补丁阶段审查结论

补丁阶段 1-3 已完成并通过审查：

| 补丁 | 状态 | 审查结论 |
|------|------|----------|
| 阶段一: Admin Scraper 自有实现 | ✅ | 6 个 Scraper RPC 全部实现，MQ 链路打通 |
| 阶段二: RAG 管理能力补全 | ✅ | 5 个 RAG RPC 真实实现，Admin 委托恢复 |
| 阶段三: AI Gateway Admin 调试对接 | ✅ | 3 个 admin RPC 实现，字段全链路透传 |

---

## 九、修复优先级建议

### P0（阻塞用户功能，建议立即修复）
1. Gateway 补齐 Membership 3 条路由（ListPlans, ListOrders, GetOrder）
2. Gateway 补齐 Community DeletePost 路由

### P1（数据一致性风险，建议尽快修复）
3. Interview service 3 个方法加事务
4. LearningArchive HandleInterviewFinished 加事务
5. 4 个 MQ 消费者加幂等保护

### P2（功能完整性，建议后续迭代）
6. 新增 ChangePassword RPC
7. 新增 GetPlanProgress RPC
8. 新增 GetMistakeTopic RPC
9. Live2D 前端查询接口

### P3（代码质量）
10. 修复 2 处 fmt.Errorf → kratos errors
11. BaseModel 补齐 DeletedAt
12. 清理孤儿 IndustryService proto

### P4（基础设施，Phase 7）
13. Casbin RBAC
14. 分布式限流
15. ImportLive2DPackage/ImportLive2DBackground 实现

---

## 十、统计总览

| 指标 | 数值 |
|------|------|
| 总服务数 | 15 |
| 总 RPC 数 | 184 |
| 已实现 RPC | 163 (87.6%) |
| 桩方法 | 2 |
| 功能遗漏 | 9 处 |
| 准则违规 | 14 处 |
| Gateway 路由缺失 | 5 条 |
| Proto 孤儿服务 | 1 个 |
