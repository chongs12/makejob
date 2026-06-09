# MakeJob 微服务改造 Phase 0-6 复核审查报告

> **复核日期**: 2026-06-08
> **审查范围**: Phase 0-6 代码、Gateway 路由、前端调用对齐情况、补丁阶段 1-3
> **审查依据**: `docs/开发准则.md`、`docs/microservice-redesign.md`、`docs/task-execution-library.md`
> **参考基准**: `docs/backend/` 单体架构代码
> **本次复核动作**: 重新核对代码、前端调用、Proto、Gateway 注册情况，并重新执行 `go test ./...`、`go build ./...`

---

## 一、总体评估

| 维度 | 当前状态 | 说明 |
|------|----------|------|
| Proto 定义 | 15 个服务、184 个 RPC | 数量完整 |
| Service 实现 | 165/184 RPC 为真实实现 | 核心改造已基本完成 |
| Gateway 路由 | **4 条已实现 RPC 路由缺失** | 仅 Membership 域确认缺失 |
| 前台能力对齐 | **3 组能力未对齐** | PlanProgress、MistakeTopic、Live2D 前台查询 |
| 编译状态 | 通过 | `go build ./...` 已复核通过 |
| 测试状态 | 通过 | `go test ./...` 已复核通过 |

**结论**: 旧版审查报告的大方向并非失实，绝大多数实现类问题是真实存在的；但部分条目把“产品欠账”“前后端接口尚未对齐”“设计是否必须如此”直接写成了“确定性缺陷”。本次复核后，已将这几类问题拆开表述。

---

## 二、对旧版报告的修正结论

| # | 旧版表述 | 复核结论 | 说明 |
|---|----------|----------|------|
| 1 | `ChangePassword` 属于“单体功能未迁移” | **误判** | 单体本身就是 `501 not implemented` 桩方法，不能算迁移遗漏 |
| 2 | “8 条 Gateway 路由缺失” | **表述不准确** | 确认缺失的只有 Membership 4 条；其余更准确地说是“前台能力未对齐” |
| 3 | `GetMistakeTopic` 只缺“单个详情 RPC” | **表述不完整** | 实际还包括公开路由命名和响应结构与前端不一致 |
| 4 | Live2D 前台查询接口“全缺”且必须新增 Companion RPC | **问题属实，修复方案不唯一** | 当前前台查询能力确实缺失，但不一定只能通过 Companion 新增 RPC 修复 |
| 5 | BaseModel / 软删除共 4 处“违规” | **1 处确认 + 3 处待确认设计** | `pkg/model/base.go` 可确认为偏差；其余 3 处是否必须软删除要看领域设计 |
| 6 | `IndustryService` 只是“Proto 孤儿服务” | **问题比原结论更严重** | 除了没有服务端实现外，Interview 默认配置还把 `industry.service_addr` 指到了 AI Gateway gRPC 端口，存在运行时失败风险 |

---

## 三、确认属实的未解决问题

### 3.1 未实现桩方法（2 处）

#### 3.1.1 Admin: ImportLive2DPackage

| 项目 | 内容 |
|------|------|
| 文件 | `app/admin/internal/service/admin.go:958` |
| 现状 | 返回 `ServiceUnavailable("UNIMPLEMENTED", "Live2D 模型包导入待 Companion 微服务实现后接入")` |
| Gateway 路由 | 已注册 `POST /api/admin/live2d-models/import` |
| 影响 | 管理后台导入入口可见，但调用必失败 |

#### 3.1.2 Admin: ImportLive2DBackground

| 项目 | 内容 |
|------|------|
| 文件 | `app/admin/internal/service/admin.go:963` |
| 现状 | 返回 `ServiceUnavailable("UNIMPLEMENTED", "Live2D 背景资源导入待 Companion 微服务实现后接入")` |
| Gateway 路由 | 已注册 `POST /api/admin/live2d-models/backgrounds/import` |
| 影响 | 管理后台背景导入入口可见，但调用必失败 |

---

### 3.2 已确认的功能 / 接口缺口（7 组）

#### 3.2.1 PlanProgress 专用统计接口缺失

| 项目 | 内容 |
|------|------|
| 单体位置 | `docs/backend/internal/handler/plan_handler.go:327` |
| 微服务现状 | `plan.proto` 无 `GetPlanProgress` 类 RPC；`PlanDetail` 只有基础 `progress/completed_tasks/total_tasks` 字段 |
| 前端现状 | Web 前端已调用 `/plans/:id/progress` |
| 影响 | 依赖该专用统计结构的页面无法正常获取数据 |
| 说明 | 这是明确的能力缺口，不是旧报告所述的“仅仅缺一个 float progress 字段” |

#### 3.2.2 Membership.ListPlans 路由缺失

| 项目 | 内容 |
|------|------|
| 微服务现状 | `membership.proto` 有 `ListPlans`，service 已实现 |
| Gateway 现状 | 未注册 `GET /api/v1/membership/plans` |
| 影响 | 前台无法获取可购买套餐列表 |

#### 3.2.3 Membership.ListOrders 路由缺失

| 项目 | 内容 |
|------|------|
| 微服务现状 | `membership.proto` 有 `ListOrders`，service 已实现 |
| Gateway 现状 | 未注册 `GET /api/v1/membership/orders` |
| 影响 | 用户无法查看订单历史 |

#### 3.2.4 Membership.GetOrder 路由缺失

| 项目 | 内容 |
|------|------|
| 微服务现状 | `membership.proto` 有 `GetOrder`，service 已实现 |
| Gateway 现状 | 未注册 `GET /api/v1/membership/orders/:id` |
| 影响 | 用户无法查看订单详情 |

#### 3.2.5 Membership.HandlePaymentCallback 路由缺失

| 项目 | 内容 |
|------|------|
| 微服务现状 | `membership.proto` 有 `HandlePaymentCallback`，service 已实现 |
| Gateway 现状 | 未注册 `POST /api/v1/membership/callback` |
| 影响 | 支付回调无法经由 Gateway 进入服务 |

#### 3.2.6 Live2D 前台查询能力未对齐

| 项目 | 内容 |
|------|------|
| 单体位置 | `docs/backend/internal/handler/live2d_handler.go` 中 `ListSelectableModels`、`GetCurrentModel` |
| 前端现状 | 当前前端实际请求 `/live2d/models`，按 `scene + industry_code` 获取可选模型 |
| 微服务现状 | Admin 仅有管理向 `ListLive2DModels`；Gateway 无公开 `/live2d/models`、`/live2d/current` 路由 |
| 影响 | 陪伴页、面试页的 Live2D 选模链路无法依赖后端真实数据完成 |
| 说明 | 问题属实，但修复不一定只能走 Companion 新增 RPC，也可以由 Gateway 聚合现有管理数据并输出前台专用 DTO |

#### 3.2.7 MistakeTopic 公开接口未对齐

| 项目 | 内容 |
|------|------|
| 单体位置 | `docs/backend/internal/handler/question_handler.go:273` |
| 前端现状 | 前端当前请求 `/mistake-topics` 与 `/mistake-topics/:code`，并期望 `MistakeTopicCard` 结构 |
| 微服务现状 | `question.proto` 仅提供聚合统计型 `ListMistakeTopics`；Gateway 当前仅暴露 `/api/v1/mistakes/topics` |
| 影响 | 错因专题列表与详情页均无法按前端当前契约工作 |
| 说明 | 这不是“只缺一个详情 RPC”，而是公开路由和响应结构整体未对齐 |

---

### 3.3 已确认的工程规范问题（10 处）

#### 3.3.1 多表写入缺少事务（4 处）

| # | 文件 | 方法 | 行号 | 问题 |
|---|------|------|------|------|
| 1 | `app/interview/internal/biz/usecase.go` | `CreateInterview` | 88-119 | 创建面试记录 + 写首题消息，不在同一事务 |
| 2 | `app/interview/internal/biz/usecase.go` | `SubmitAnswer` | 125-218 | 写用户答案 + 写 AI 回复 + 写下一题 + 更新进度，不在同一事务 |
| 3 | `app/interview/internal/biz/usecase.go` | `SubmitCodingAnswer` | 551-652 | 写代码消息 + 写答题记录 + 写 AI 评审消息，不在同一事务 |
| 4 | `app/learning_archive/internal/biz/archive.go` | `HandleInterviewFinished` | 98-149 | 批量建档与事件发布缺乏明确原子边界 |

#### 3.3.2 MQ 消费者缺乏幂等保护（3 处）

| # | 文件 | 方法 | 行号 | 问题 |
|---|------|------|------|------|
| 1 | `app/interview/internal/server/mq.go` | 简历解析消费者 | 32-38 | 重复消费会重复触发 AI 简历解析 |
| 2 | `app/interview/internal/server/mq.go` | 编程归档消费者 | 48-54 | 重复消费会重复写学习档案 |
| 3 | `app/learning_archive/internal/server/mq.go` | `handleInterviewFinished` | 52-61 | 重复消费会重复创建学习档案条目 |

> 说明：`app/question/internal/server/mq.go` 中 `handleScraperImport` 已通过 task terminal status 检查修复，不再计入问题。

#### 3.3.3 gRPC 错误处理不规范（2 处）

| # | 文件 | 行号 | 问题 |
|---|------|------|------|
| 1 | `app/admin/internal/service/admin.go` | 740 | `fmt.Errorf("ai preset %d not found")` 应改为 kratos `NotFound` |
| 2 | `app/admin/internal/service/bridge_helpers.go` | 28 | helper 返回的 `fmt.Errorf` 会直接沿 service 层返回，不符合 gRPC 错误约定 |

#### 3.3.4 全局 BaseModel 缺少软删除字段（1 处）

| 文件 | 问题 |
|------|------|
| `pkg/model/base.go` | 当前仅有 `ID/CreatedAt/UpdatedAt`，缺少 `DeletedAt gorm.DeletedAt`；会影响直接嵌入此基类的实体 |

---

### 3.4 运行时配置 / 契约风险（1 处）

#### IndustryService 实现与默认配置不一致

| 项目 | 内容 |
|------|------|
| Proto | `api/makejob/industry/v1/industry.proto` 定义了 `IndustryService` |
| 服务端现状 | 仓库中无 `app/industry/` 服务实现 |
| 调用方现状 | Interview 在 `CreateInterview` 中会通过 gRPC 调用 `IndustryService.GetIndustry` 做行业校验 |
| 默认配置 | `app/interview/configs/config.yaml` 的 `industry.service_addr` 指向 `localhost:9011` |
| 实际端口 | `9011` 当前是 AI Gateway gRPC 端口，而非 IndustryService |
| 风险 | 按默认配置运行时，Interview 的行业校验链路存在直接调用失败的高风险 |

---

## 四、不宜直接定性为“缺陷”的条目

### 4.1 ChangePassword 不属于迁移遗漏

| 项目 | 内容 |
|------|------|
| 单体现状 | `docs/backend/internal/handler/auth_handler.go:179` 直接返回 `501 not implemented` |
| 微服务现状 | `user.proto` 无该 RPC，Gateway 也无该路由 |
| 复核结论 | 这是产品待实现能力，不是“单体已有功能但微服务漏迁” |

### 4.2 CommunityLike 是否必须软删除，需看领域设计

| 项目 | 内容 |
|------|------|
| 现状 | `community_likes` 仅保留 `post_id + user_id` 唯一约束与创建时间 |
| 复核结论 | 对“点赞开关表”而言，硬删除可能是有意设计，不能仅凭无 `DeletedAt` 直接定性违规 |

### 4.3 Industry 是否必须软删除，需看是否被定义为静态字典表

| 项目 | 内容 |
|------|------|
| 现状 | `app/question/internal/data/model/industry.go` 使用 `code` 作为主键，无审计字段 |
| 复核结论 | 若行业表被定义为静态字典表，不一定需要软删除；应先确认数据治理策略 |

### 4.4 AdminConfig 是否必须软删除，需看配置表语义

| 项目 | 内容 |
|------|------|
| 现状 | `app/admin/internal/data/model/admin_config.go` 为键值配置表 |
| 复核结论 | 配置表可能天然采用覆盖/删除语义，是否必须保留软删除审计需由后台配置策略决定 |

---

## 五、Gateway 路由缺失与前台能力差异汇总

### 5.1 已实现 RPC 但 Gateway 未注册的路由（4 条）

| # | 路由 | 对应 RPC | 状态 |
|---|------|----------|------|
| 1 | `GET /api/v1/membership/plans` | `Membership.ListPlans` | 未修复 |
| 2 | `GET /api/v1/membership/orders` | `Membership.ListOrders` | 未修复 |
| 3 | `GET /api/v1/membership/orders/:id` | `Membership.GetOrder` | 未修复 |
| 4 | `POST /api/v1/membership/callback` | `Membership.HandlePaymentCallback` | 未修复 |

### 5.2 前台能力未对齐（3 组）

| # | 能力 | 当前问题 |
|---|------|----------|
| 1 | PlanProgress | 前端已调用 `/plans/:id/progress`，微服务未提供对应入口 |
| 2 | MistakeTopic | 前端使用 `/mistake-topics` 与 `/mistake-topics/:code`，Gateway 与 QuestionService 当前均未对齐 |
| 3 | Live2D 前台查询 | 前端使用 `/live2d/models`，Gateway 当前无公开查询路由，后端也无前台专用 DTO 输出 |

---

## 六、RAG 服务复核结果

| RPC / 能力 | 复核结论 | 说明 |
|------------|----------|------|
| `UpdateConfig` | 已实现 | 已具备运行时配置更新能力，并同步检索 / 索引 / MQ 使用链路 |
| `GetDocumentStats` | 已实现 | 统计值当前等同题目集合规模，语义可接受 |
| `LastIndexedAt` | 返回 `nil` | 未伪造时间戳，符合“不伪成功”原则 |

---

## 七、修复优先级建议

### P0（直接影响运行或用户主流程）— 已全部修复
1. ✅ 修复 Interview 对 `IndustryService` 的默认配置指向问题 — 改为本地 DB 查询
2. ✅ Gateway 补齐 Membership 5 条已实现 RPC 路由（ListPlans, ListOrders, GetOrder, HandlePaymentCallback, UpgradeMembership）
3. ✅ 补齐 `/plans/:id/progress` 能力 — 新增 GetProgress RPC + Gateway 路由
4. ✅ 统一 MistakeTopic 的公开路由与响应结构 — 新增 GetMistakeTopic RPC + 静态知识库 + Gateway 路由

> 2026-06-08 复修补记：Gateway 已补下游 gRPC 鉴权透传；`/api/v1/membership/callback` 调整为公开入口并改为内部服务令牌调用；补充 `/api/v1/mistake-topics` 兼容路由；修正 `UpgradeMembership` 按 `membership.proto` 契约转发。

### P1（用户可感知能力缺口）
5. 补齐 Live2D 前台查询链路：公开路由 + 前台专用 DTO + 场景过滤能力
6. 实现 `ImportLive2DPackage` / `ImportLive2DBackground`，或在前端隐藏未实现入口

### P2（数据一致性与稳定性）
7. Interview 的 `CreateInterview`、`SubmitAnswer`、`SubmitCodingAnswer` 补齐事务
8. LearningArchive 的 `HandleInterviewFinished` 明确事务边界与失败策略
9. Interview / LearningArchive 的 3 个 MQ 消费者补齐幂等保护

### P3（工程规范）
10. 修复 Admin service 中 2 处 gRPC 错误类型不规范问题
11. 为 `pkg/model/base.go` 补齐 `DeletedAt`
12. 对 `CommunityLike`、`Industry`、`AdminConfig` 三处补充“是否需要软删除”的明确设计结论

---

## 八、统计总览（按本次复核口径）

| 指标 | 当前结论 |
|------|----------|
| 总服务数 | 15 |
| 总 RPC 数 | 184 |
| 已实现 RPC | 165 |
| 未实现桩方法 | 2 |
| 已确认的功能 / 接口缺口 | 7 组 |
| 已确认的工程规范问题 | 10 处 |
| 待确认的设计项 | 3 处 |
| 已确认缺失的 Gateway 路由 | 4 条 |
| 已确认前台能力未对齐 | 3 组 |
| 运行时配置 / 契约风险 | 1 处 |

### 2026-06-08 P1 修复回写

- P1-1 `Live2D 前台查询能力未对齐` 已修复：
  - 前台查询能力归属落在 `Admin`，未新增 `Companion` 第二份真相源
  - `AdminService` 已新增前台专用 RPC：`ListSelectableLive2DModels`、`GetCurrentLive2DModel`
  - `Gateway` 已补齐公开路由：`/api/v1/live2d/models`、`/api/v1/live2d/current`
  - 为兼容现有 Web 前端调用，已同步保留 `/api/live2d/models`、`/api/live2d/current`
- P1-2 `ImportLive2DPackage / ImportLive2DBackground 未实现` 已修复：
  - `AdminService` 已本地实现 ZIP 模型包导入与背景图导入
  - 已补齐正式 `pkg/live2dassets` 资产处理能力，包括自动发现、路径校验、资源删除与背景图命名去重
  - `Gateway` 已挂载 `/live2d-assets/*` 静态资源目录，导入结果可被前台直接访问

### 2026-06-09 P2/P3 修复回写

- P2-1 `Interview 三个方法补齐事务` 已修复：
  - `CreateInterview` 已将“创建面试 + 首题消息”包裹到 `uc.repo.Transaction(...)`
  - `SubmitAnswer` 已改为“先算 AI，再事务写入用户答案 / AI 反馈 / 下一题 / 进度”
  - `SubmitCodingAnswer` 已改为“先执行代码与 AI 评分，再事务写入代码消息 / attempt / AI 评审消息”
  - `interviewRepo` 相关写路径已统一改为优先使用事务上下文中的 DB 连接
- P2-2 `LearningArchive HandleInterviewFinished 事务边界` 已修复：
  - `BatchCreate` 已放入仓储事务，MQ 发布改为事务提交后执行
  - `archive.written` 发布失败仅记录日志，不回滚已提交的档案数据
- P2-3 `3 个 MQ 消费者补齐幂等保护` 已修复：
  - Interview 简历解析消费者已通过 `ResumeParsedJSON` 终态检查跳过重复消费
  - Interview 编程归档链路已增加“消费前检查 + 学习档案写入前去重”双层保护
  - LearningArchive `interview.finished` 消费者已通过已落档条目检查跳过重复建档
- P3-1 `Admin gRPC 错误类型不规范` 已修复：
  - `UpdateAIPreset` 缺失场景已改为 `kratoserr.NotFound("AI_PRESET_NOT_FOUND", ...)`
  - `metadataJSONFromStringMap` 序列化失败已改为 `kratoserr.InternalServer("METADATA_MARSHAL_FAILED", ...)`
- P3-2 `pkg/model/base.go` 缺少软删除字段 已修复：
  - `BaseModel` 已补齐 `DeletedAt gorm.DeletedAt`
- P3-3 `CommunityLike / Industry / AdminConfig 软删除设计结论` 已补充：
  - `CommunityLike` 维持硬删除，明确为点赞开关表语义
  - `Industry` 维持无软删除，明确为静态字典表
  - `AdminConfig` 维持无软删除，明确为键覆盖型配置表

### 2026-06-09 Review Follow-up

- 已修复复核中的 3 个残留问题：
  - `LearningArchive.WriteEntry` 命中幂等重复时，改为返回已有记录，不再返回零值空实体
  - `interview.finished` 在 weak/strength 为空时，改为仅写内部 marker、不发布 `archive.written`，并通过 marker 闭合幂等
  - `Interview.CreateInterview` 改为先调用 AI 生成首题，再用短事务原子写入 `interview + first_message`，移除外部依赖包裹在本地事务内的长事务风险
