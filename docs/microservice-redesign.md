# MakeJob 微服务架构重新设计方案

## 一、最终服务拓扑

### 1.1 服务清单

| # | 服务名 | 核心职责 | 决策理由 |
|---|--------|----------|----------|
| 1 | **Gateway** | HTTP→gRPC 协议转换、JWT 鉴权、限流、WebSocket 路由 | 保留，剥离全部业务逻辑 |
| 2 | **User** | 用户注册/登录/Profile、RBAC 角色 | 保留，去除 Membership 职责 |
| 3 | **Membership** | 会员等级、订单、支付回调、权益校验 | 从 User 拆出，独立变更频率高 |
| 4 | **Question** | 题库 CRUD、题集、错题专题、收藏/笔记、考试、刷题统计 | 保留，补齐缺失功能 |
| 5 | **Interview** | 面试生命周期（创建/答题/结束/报告）、简历驱动模式 | 保留，聚焦面试流程编排 |
| 6 | **Realtime** | 实时语音面试 WebSocket 长连接、ASR/TTS 流式处理 | 从 Interview 拆出：独立资源密集型、有状态连接 |
| 7 | **Growth** | 成长总览聚合、周重点、学习日志 | 保留，定位为只读聚合服务 |
| 8 | **Plan** | 学习计划生成/调整、任务管理、反馈诊断 | 从 Growth 拆出：有独立写入状态和 AI 编排 |
| 9 | **Companion** | AI 伴侣对话、情绪感知、学习激励 | 从 Growth 拆出：独立交互模式，依赖 AI+TTS+Live2D |
| 10 | **Community** | 帖子 CRUD、评论、点赞 | 保留，补齐缺失功能 |
| 11 | **LearningArchive** | 学习归档写入/查询、薄弱项/聚焦信号提取 | 保留，作为跨域学习数据枢纽 |
| 12 | **AI Gateway** | LLM 调用路由、Prompt 模板渲染、结构化输出、调用日志 | 新增：统一 AI 调用入口，异构可替换 |
| 13 | **RAG** | 向量索引管理、语义检索、文档管理 | 新增：独立基础设施服务，有自己的 Milvus |
| 14 | **CodeRunner** | 沙箱代码执行、测试用例评判 | 新增：无状态、可独立扩缩、安全隔离 |
| 15 | **Admin** | 管理后台聚合 BFF（调用各域服务完成管理操作） | 保留，重构为纯编排层 |
| 16 | **Worker** | MQ 消费者进程（可按队列独立部署） | 保留概念，拆分为各服务内嵌 consumer |

**服务数量：15 个独立部署单元**（不含 Worker，因为 consumer 嵌入各自的服务进程）

决策说明：
- **拆分 Growth → Growth + Plan + Companion**：原 growth.proto 混杂了只读聚合（Growth）、有状态编排（Plan）和交互对话（Companion）三种完全不同的变更理由和资源特征。
- **拆分 Interview → Interview + Realtime**：实时语音是有状态长连接 + 外部 ASR/TTS 流，与面试 CRUD 的变更频率和资源模型截然不同。
- **新增 AI Gateway**：所有 AI 调用收敛到一个服务，便于统一日志/限流/模型切换，且未来可用 Python/Rust 重写而不影响业务服务。
- **新增 RAG**：有独立存储（Milvus）、独立外部依赖（Embedding API）、独立扩缩需求。
- **新增 CodeRunner**：安全隔离要求高，资源消耗不可控，必须独立于业务服务。
- **新增 Membership**：支付回调、订单状态机、权益校验是独立业务域，与用户认证的变更节奏不同。

### 1.2 架构拓扑图

```
                              ┌─────────────────────────────────────────────┐
                              │              Frontend (React)               │
                              └──────────────────────┬──────────────────────┘
                                                     │ HTTP / WebSocket
                              ┌──────────────────────▼──────────────────────┐
                              │                  Gateway                     │
                              │  (JWT鉴权 + 限流 + HTTP→gRPC + WS路由)      │
                              └──┬───┬───┬───┬───┬───┬───┬───┬───┬───┬─────┘
                                 │   │   │   │   │   │   │   │   │   │
          ┌──────────────────────┘   │   │   │   │   │   │   │   │   └──────────┐
          │                          │   │   │   │   │   │   │   │              │
          ▼                          ▼   │   ▼   │   ▼   │   ▼   │              ▼
    ┌──────────┐  ┌────────────┐    │ ┌─────┐   │ ┌─────────┐  │     ┌───────────┐
    │   User   │  │ Membership │    │ │Quest│   │ │Community│  │     │   Admin   │
    │          │  │            │    │ │ ion  │   │ │         │  │     │   (BFF)   │
    └──────────┘  └────────────┘    │ └──┬──┘   │ └─────────┘  │     └─────┬─────┘
                                    │    │      │              │           │
          ┌─────────────────────────┘    │      │              │           │
          ▼                              │      ▼              ▼           │
    ┌───────────┐                        │  ┌────────┐  ┌──────────┐      │
    │ Interview │◄───────────────────────┘  │  Plan  │  │Companion │      │
    │           │                           └───┬────┘  └────┬─────┘      │
    └─────┬─────┘                               │            │            │
          │                                     │            │            │
          ▼                                     ▼            ▼            ▼
    ┌───────────┐                         ┌──────────┐  ┌─────────────────────┐
    │ Realtime  │                         │  Growth  │  │  (调用各域服务)      │
    │ (WS+ASR  │                         └──────────┘  └─────────────────────┘
    │  +TTS)   │
    └───────────┘

    ═══════════════════════ 基础设施层 ═══════════════════════════════════

    ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────────┐
    │  AI Gateway  │  │     RAG      │  │  CodeRunner  │  │LearningArchive │
    │  (LLM路由)   │  │  (Milvus)    │  │  (Piston)    │  │  (归档枢纽)    │
    └──────────────┘  └──────────────┘  └──────────────┘  └────────────────┘

    ═══════════════════════ 异步事件流 (RabbitMQ) ═════════════════════════

    Interview ──publish──▶ [interview.finished] ──▶ LearningArchive (consumer)
    Interview ──publish──▶ [resume.parse.requested] ──▶ Interview (self-consumer)
    Plan      ──publish──▶ [plan.generate.requested] ──▶ Plan (self-consumer)
    Plan      ──publish──▶ [feedback.diagnosis.requested] ──▶ Plan (self-consumer)
    Admin     ──publish──▶ [question.pipeline.requested] ──▶ Question (consumer)
    Admin     ──publish──▶ [scraper.import.requested] ──▶ Question (consumer)
    Question  ──publish──▶ [question.changed] ──▶ RAG (consumer)
    LearningArchive ──publish──▶ [archive.written] ──▶ Growth (consumer, 刷新缓存)
```

### 1.3 gRPC 同步调用方向

```
Gateway ──▶ User, Membership, Question, Interview, Growth, Plan, Companion, Community, Admin
Admin  ──▶ User, Question, Interview, RAG, AI Gateway, CodeRunner
Interview ──▶ AI Gateway, RAG, LearningArchive, User (获取 Profile)
Realtime ──▶ AI Gateway, RAG (实时注入), Interview (持久化 Q&A)
Plan ──▶ AI Gateway, LearningArchive, Interview (读取历史)
Companion ──▶ AI Gateway, LearningArchive, Plan (读取当前任务)
Growth ──▶ LearningArchive, Interview (统计), Plan (当前计划), Question (练习统计)
Question ──▶ AI Gateway (答题评估), CodeRunner (RunCode)
RAG ──▶ (无出站 gRPC，纯被动)
CodeRunner ──▶ (无出站 gRPC，纯被动)
AI Gateway ──▶ (无出站 gRPC，调用外部 LLM API)
LearningArchive ──▶ (无出站 gRPC，纯被动)
```

---

## 二、各服务详细设计

### 2.1 Gateway

**职责**：纯反向代理，零业务逻辑。

**功能清单**：
- HTTP ↔ gRPC 协议转换（基于 proto 注解或手动路由表）
- JWT Token 验证（调用 `pkg/auth` 解析，不做用户查询）
- 基础限流（令牌桶，按 IP / UserID）
- WebSocket 连接升级并透传到 Realtime 服务
- SSE 流式端点代理（Admin 的 Pipeline Stream）
- 请求 ID 注入与 Tracing 传播

**与单体对照**：删除全部 `bridge.Runtime` 相关代码，删除 `backend/bridge/` 包。Gateway 不再持有数据库连接。

---

### 2.2 User 服务

**Proto RPC 清单（9 个）**：

| RPC | 来源 | 说明 |
|-----|------|------|
| Register | 现有微服务 | 修复：填充 refresh_token |
| Login | 现有微服务 | 修复：填充 refresh_token |
| RefreshToken | 现有微服务 | 修复：正确生成新 refresh_token |
| Logout | 新增 | Token 黑名单（Redis TTL = token 剩余有效期） |
| GetProfile | 现有微服务 | 不变 |
| UpdateProfile | 现有微服务 | 不变 |
| GetUserByID | 现有微服务 | 内部 RPC |
| BatchGetUsers | 现有微服务 | 内部 RPC |
| UpdateUserRole | 新增 | 供 Admin 调用，原 Admin 直接改 DB |

**数据模型**：
- `users` (id, phone, email, password_hash, nickname, avatar, role, created_at, updated_at)
- `token_blacklist` (Redis SET，key=jti，TTL=token剩余秒数)

**与单体对照**：
- Membership 相关字段和 RPC 移出到 Membership 服务
- 新增 Logout（Redis 黑名单方案）
- 修复 RefreshToken Bug

---

### 2.3 Membership 服务

**Proto RPC 清单（8 个，全新设计）**：

| RPC | 来源 | 说明 |
|-----|------|------|
| GetMembershipStatus | 从 User 迁移 | 查询用户会员等级和到期时间 |
| ListPlans | 单体迁移 | 可购买的会员套餐列表 |
| CreateOrder | 单体迁移 | 创建支付订单 |
| GetOrder | 单体迁移 | 查询订单详情 |
| ListOrders | 单体迁移 | 用户订单历史 |
| HandlePaymentCallback | 单体迁移 | 支付平台回调处理 |
| CheckFeatureAccess | 新增 | 权益校验（其他服务调用判断功能是否可用） |
| UpgradeMembership | 从 User 迁移 | 管理员手动升级 |

**数据模型**：
- `membership_plans` (id, name, level, duration_days, price, features_json)
- `membership_orders` (id, user_id, plan_id, amount, status, payment_channel, paid_at, expired_at)
- `user_membership` (user_id, level, expired_at, auto_renew)

**MQ 职责**：
- 生产者：`membership.upgraded` → Growth（触发计划可用功能刷新）
- 消费者：无

---

### 2.4 Question 服务

**Proto RPC 清单（22 个）**：

| RPC | 来源 | 说明 |
|-----|------|------|
| ListQuestions | 现有微服务 | 不变 |
| GetQuestion | 现有微服务 | 补齐富字段 |
| ListCategories | 现有微服务 | 不变 |
| ListIndustries | 现有微服务 | 不变 |
| SubmitAnswer | 现有微服务 | 不变 |
| RunCode | 现有微服务（桩） | 重实现：调用 CodeRunner |
| CreateFavorite | 现有微服务 | 不变 |
| DeleteFavorite | 现有微服务 | 不变 |
| ListFavorites | 现有微服务 | 不变 |
| CreateNote | 现有微服务 | 不变 |
| UpdateNote | 现有微服务 | 不变 |
| DeleteNote | 新增 | 从单体迁移 |
| ListNotes | 现有微服务 | 不变 |
| GetPracticeRecommendations | 现有微服务 | 补齐 interview_id + limit |
| GetWrongQuestions | 现有微服务 | 不变 |
| GetUserPracticeStats | 现有微服务 | 不变 |
| GetRandomExam | 现有微服务 | 不变 |
| GenerateTimedExam | 新增 | 从单体迁移 |
| SubmitExam | 新增 | 从单体迁移，AI 批改 |
| ListQuestionSets | 新增 | 从单体迁移 |
| GetQuestionSetDetail | 新增 | 从单体迁移 |
| ListMistakeTopics | 新增 | 从单体迁移 |

**数据模型**：
- `questions` (id, title, content, answer, analysis, difficulty, category_id, industry_id, tags, evaluation_mode, test_cases_json, starter_code)
- `categories` (id, name, parent_id, sort_order)
- `industries` (id, code, name, icon)
- `question_sets` (id, title, slug, description, question_ids)
- `user_answers` (id, user_id, question_id, answer, is_correct, ai_feedback, duration)
- `user_favorites` (user_id, question_id)
- `user_notes` (id, user_id, question_id, content)
- `exams` (id, user_id, type, time_limit, question_ids, started_at, finished_at, score)

**MQ 职责**：
- 生产者：`question.changed`（CRUD 后触发 RAG 索引同步）
- 消费者：
  - `question.pipeline.build`（AI 题目生成流水线）
  - `scraper.import.questions`（爬虫导入结构化题目）

**关键实现**：
- `RunCode`：调用 `CodeRunner.Execute` gRPC，附带 test_cases 和 stdin
- `SubmitExam`：调用 `AI Gateway.QuizAnalyzer` 批改
- `ListMistakeTopics`：按 category 聚合错题，计算薄弱项分布

**与单体对照**：补齐 6 个缺失功能 + 修复 RunCode 桩实现。推荐接口补齐参数。

---

### 2.5 Interview 服务（核心复杂服务）

**Proto RPC 清单（14 个）**：

| RPC | 来源 | 说明 |
|-----|------|------|
| CreateInterview | 现有 | 增强：简历驱动发 MQ 解析 |
| GetInterview | 现有 | 不变 |
| ListInterviews | 现有 | 不变 |
| SubmitAnswer | 现有 | 增强：调用 AI Gateway 评估 |
| GetNextQuestion | 待实现 | AI 动态出题 + RAG |
| FinishInterview | 待实现 | 结束 → 发 MQ 生成报告 |
| GetReport | 待实现 | 查询已生成报告 |
| SubmitCodingAnswer | 待实现 | CodeRunner 执行 + AI 评估 |
| IsRealtimeInterview | 待实现 | 查询是否实时语音类型 |
| GetRealtimeContext | 待实现 | 供 Realtime 服务拉取上下文 |
| BindRealtimeDialog | 待实现 | 绑定 Dialog ID |
| AppendRealtimeUserAnswer | 待实现 | 存储 ASR 文本 |
| AppendRealtimeAssistantReply | 待实现 | 存储 AI 回复 |
| GetInterviewStats | 现有 | 不变 |

**数据模型**：
- `interviews` (id, user_id, type, mode, industry_id, status, resume_text, jd_text, config_json, started_at, finished_at)
- `interview_messages` (id, interview_id, role, content, question_index, coding_metadata)
- `interview_reports` (id, interview_id, overall_score, dimension_scores_json, strengths, weaknesses, suggestions, coding_diagnostics)
- `interview_coding_attempts` (id, interview_id, question_index, code, language, result_json)

**MQ 职责**：
- 生产者：
  - `interview.resume.parse.requested`（创建简历驱动面试时）
  - `interview.report.generate.requested`（FinishInterview 时）
  - `interview.finished`（报告生成完毕，通知归档）
- 消费者：
  - `interview.resume.parse.requested`（自消费：AI 解析简历）
  - `interview.report.generate.requested`（自消费：AI 生成报告）

**关键业务流程 — 面试出题**：
```
GetNextQuestion
  → 读取面试上下文 (历史问答、简历、JD)
  → gRPC: RAG.Retrieve (获取相关题目)
  → gRPC: AI Gateway.InterviewAgent (生成下一题)
  → 如果是编程题，附带 test_cases + starter_code
  → 存储 interview_message (role=assistant)
  → 返回 NextQuestionResponse
```

**关键业务流程 — 报告生成**：
```
FinishInterview → 标记 status=finished → 发布 MQ
[Consumer]
  → AI Gateway.InterviewAgent (综合评估)
  → 如有编程题: AI Gateway.QuizAnalyzer (编程诊断)
  → 写入 interview_reports
  → 发布 MQ: interview.finished
```

**与单体对照**：
- 单体直接调用 `ai.RuntimeBuilder` → 改为 gRPC 调用 AI Gateway
- 单体直接写 `learningArchiveRepo` → 改为 MQ 事件驱动
- 实时语音逻辑全部移到 Realtime 服务

---

### 2.6 Realtime 服务（实时语音面试）

**Proto RPC 清单（5 个，全新设计）**：

| RPC | 说明 |
|-----|------|
| InitSession | 创建 Volcengine Dialog 会话，返回 WS 参数 |
| GetSessionStatus | 查询语音会话状态 |
| InjectRAGContext | 运行时注入 RAG 检索结果 |
| EndSession | 终止语音会话 |
| HealthCheck | 探活 |

**主要通信方式**：WebSocket 长连接（非 gRPC）

**WebSocket 协议**：
```
浏览器 ←WS→ Gateway(透传) ←WS→ Realtime ←WS→ Volcengine
```

**数据模型**：仅 Redis 维护会话状态
- `realtime:session:{id}` → interview_id, dialog_id, status

**关键实现**：
- 双端 WebSocket：客户端音频流 ↔ Volcengine 二进制协议
- ASR 文本回调 → `Interview.AppendRealtimeUserAnswer`
- Chat 回复回调 → `Interview.AppendRealtimeAssistantReply`
- 定期 `RAG.Retrieve` → Volcengine event 502 注入外部知识
- TTS 音频从 Volcengine 直接透传客户端

**与单体对照**：`backend/internal/realtime/volcengine/` 整体迁入，去除直接 DB 访问。

---

### 2.7 Growth 服务（只读聚合）

**Proto RPC 清单（3 个）**：

| RPC | 来源 | 说明 |
|-----|------|------|
| GetGrowthSummary | 现有（重写） | 并发调用多服务实时聚合 |
| GetWeeklyFocus | 现有（重写） | 并发调用 LearningArchive + Question |
| SyncStudyLog | 现有（重写） | 修复数据模型兼容性 |

**数据模型**：
- `study_logs` (id, user_id, date_key, plan_id, action, ref_type, ref_id, duration_seconds, summary, completed_titles, skipped_titles)

注意：study_logs 模型合并单体和微服务的字段，保持向后兼容。

**关键实现 — GetGrowthSummary 聚合**：
```
并发 gRPC 调用：
  - Question.GetUserPracticeStats → 练习统计
  - Plan.GetCurrentPlan → 当前计划概要
  - LearningArchive.GetWeakTopics → 薄弱项
  - LearningArchive.GetFocusSignals → 聚焦信号
  - Interview.GetInterviewStats → 面试统计
组装完整 GrowthSummary 返回
```

**MQ 职责**：
- 消费者：`archive.written`（可选：刷新缓存或预计算）
- 生产者：无

**与单体对照**：
- 单体的 `growthService` 直接读所有 repo → 改为 gRPC 并发聚合
- 返回结构补齐到单体水平（PracticeStats + Plans + Archives + Trends）

---

### 2.8 Plan 服务

**Proto RPC 清单（7 个）**：

| RPC | 来源 | 说明 |
|-----|------|------|
| CreatePlan | growth.proto 迁出 | AI 生成学习计划 |
| GetPlan | growth.proto 迁出 | 查询计划详情 |
| GetCurrentPlan | 新增 | 获取用户当前活跃计划 |
| ListPlans | 新增 | 用户历史计划列表 |
| UpdateTaskStatus | growth.proto 迁出 | 标记任务完成/跳过 |
| SubmitTaskFeedback | growth.proto 迁出 | 提交反馈触发 AI 诊断 |
| AdjustPlan | 新增 | 基于诊断结果动态调整计划 |

**数据模型**：
- `plans` (id, user_id, title, phase, status, industry_id, target_role, created_at)
- `plan_tasks` (id, plan_id, title, description, type, status, sort_order, due_date, ref_question_ids)
- `plan_adjustments` (id, plan_id, trigger, diagnosis_json, adjustments_json, applied_at)

**MQ 职责**：
- 生产者：
  - `plan.generate.requested`（CreatePlan 异步 AI 生成）
  - `plan.feedback.diagnosis.requested`（SubmitTaskFeedback 异步诊断）
- 消费者：
  - `plan.generate.requested`（自消费：调用 AI Gateway 生成计划）
  - `plan.feedback.diagnosis.requested`（自消费：AI 诊断 + 自动调整）

**关键实现 — 计划生成**：
```
CreatePlan
  → 读取 LearningArchive.GetWeakTopics + GetFocusSignals
  → 读取 Interview.GetInterviewStats (了解面试表现)
  → 发布 MQ: plan.generate.requested

[Consumer]
  → AI Gateway.PlanAgent (生成结构化计划)
  → 写入 plans + plan_tasks
  → 更新 plan.status = active
```

**与单体对照**：从 Growth 拆出，单体的 `planService` 整体迁入。

---

### 2.9 Companion 服务

**Proto RPC 清单（3 个）**：

| RPC | 来源 | 说明 |
|-----|------|------|
| Chat | growth.proto 迁出 | AI 伴侣对话 |
| GetCompanionState | 新增 | 获取伴侣情绪/状态上下文 |
| SynthesizeSpeech | 新增 | TTS 合成（返回音频 URL） |

**数据模型**：
- `companion_sessions` (id, user_id, messages_json, emotion, last_active_at)

**关键实现 — Chat**：
```
Chat
  → 读取 LearningArchive (最近学习状态)
  → 读取 Plan.GetCurrentPlan (当前任务)
  → AI Gateway.CompanionAgent (生成回复 + 情绪 + Live2D 指令)
  → AI Gateway.Live2DDirector (生成表情动作指令)
  → 调用 TTS 合成（通过 Companion 内置 TTS client，直接调外部 API）
  → 返回 text + emotion + live2d_directives + audio_url
```

**TTS 集成说明**：Companion 直接持有 TTS Provider client（Volcengine/MiMo），因为 TTS 是纯无状态 HTTP 调用，不值得独立为服务。作为 Companion 的内部实现细节存在。

**与单体对照**：从 `companionService` + `live2dDirectiveService` + `sceneTTSService` 整合。

---

### 2.10 Community 服务

**Proto RPC 清单（9 个）**：

| RPC | 来源 | 说明 |
|-----|------|------|
| ListPosts | 现有（增强） | 补齐过滤 + 富字段 |
| GetPost | 现有（增强） | 补齐 is_liked/counts/浏览量 |
| CreatePost | 现有（增强） | 补齐验证 + 标签规范化 |
| UpdatePost | 待实现 | 从单体迁移 |
| DeletePost | 现有（增强） | 补齐级联删除 |
| ListComments | 现有（增强） | 补齐 is_author |
| CreateComment | 现有（增强） | 补齐内容校验 |
| ToggleLike | 待实现 | 从单体迁移 |
| ListMyPosts | 新增 | 从单体迁移 |

**数据模型**：
- `posts` (id, user_id, title, content, summary, post_type, tags, view_count, like_count, comment_count, created_at)
- `comments` (id, post_id, user_id, content, created_at)
- `likes` (user_id, post_id, created_at)

**MQ 职责**：无（Community 是独立域，无跨域写入需求）

**与单体对照**：所有已实现方法补齐到单体水平，新增 ListMyPosts。

---

### 2.11 LearningArchive 服务

**Proto RPC 清单（5 个，已完整实现）**：

| RPC | 来源 | 说明 |
|-----|------|------|
| WriteEntry | 现有 | 不变 |
| BatchWriteEntries | 现有 | 不变 |
| ListByUser | 现有 | 不变 |
| GetWeakTopics | 现有 | 不变 |
| GetFocusSignals | 现有 | 不变 |

**数据模型**：
- `archive_entries` (id, user_id, source_type, source_id, plan_phase, tags, mistake_tags, strength_tags, evidence_summary, created_at)

**MQ 职责**：
- 消费者：`interview.finished`（接收面试完成事件，写入归档条目）
- 生产者：`archive.written`（通知 Growth 有新归档数据）

**角色定位**：跨域学习数据枢纽。Interview 产生的学习信号通过此服务沉淀，Plan/Growth 读取此服务获取用户画像。

**与单体对照**：唯一已完整的微服务，保持不变。新增 MQ 消费能力替代单体直接调用。

---

### 2.12 AI Gateway 服务

**Proto RPC 清单（6 个）**：

| RPC | 来源 | 说明 |
|-----|------|------|
| InterviewAgent | ai.proto | 面试出题/评估/报告生成 |
| PlanAgent | ai.proto | 计划生成/调整建议 |
| CompanionAgent | ai.proto | 伴侣对话生成 |
| QuizAnalyzer | ai.proto | 答题评估/考试批改/编程诊断 |
| ResumeParser | ai.proto | 简历结构化解析 |
| Live2DDirector | ai.proto | Live2D 表情/动作指令生成 |

**数据模型**：
- `ai_configs` (id, scene, provider, model, temperature, max_tokens, extra_params)
- `ai_presets` (id, name, scene, config_snapshot)
- `prompt_templates` (id, scene, version, template_text, variables)
- `ai_call_logs` (id, scene, model, input_tokens, output_tokens, latency_ms, status, created_at)

**关键实现**：
- 动态 RuntimeBuilder：根据 scene 从 DB 加载 AI 配置，构建 LLM client
- Prompt 模板渲染：从 DB 加载模板 → 变量注入 → 发送给 LLM
- 结构化输出：JSON Schema 约束 LLM 输出格式
- 调用日志：每次 AI 调用自动记录 tokens/latency/status
- 管理接口：Admin 通过 gRPC 直接操作此服务的配置表

**异构预留**：
- 此服务是 AI 调用的唯一入口
- 未来可用 Python (LangChain/LlamaIndex) 重写而不影响任何业务服务
- 接口契约（proto）保持稳定，内部实现语言可替换

**与单体对照**：从 `backend/internal/ai/` + `admin_ai_config` + `admin_ai_preset` + `admin_ai_debug` 整合迁入。

---

### 2.13 RAG 服务

**Proto RPC 清单（8 个）**：

| RPC | 来源 | 说明 |
|-----|------|------|
| IndexQuestions | admin.proto 迁出 | 批量索引题目向量 |
| IndexDocuments | admin.proto 迁出 | 索引 RAG 文档 |
| DeleteIndex | admin.proto 迁出 | 删除指定向量 |
| Retrieve | 新增 | 语义检索（核心能力） |
| GetConfig | admin.proto 迁出 | RAG 配置查询 |
| UpdateConfig | admin.proto 迁出 | RAG 配置更新 |
| TestConnection | admin.proto 迁出 | Milvus 连通性测试 |
| GetDocumentStats | admin.proto 迁出 | 索引统计 |

**数据模型**：
- Milvus Collection: `makejob_questions` (id, content, vector[1024], metadata_json)
- PostgreSQL: `rag_documents` (id, title, content, source, status, indexed_at)

**MQ 职责**：
- 消费者：`question.changed`（增量同步题目到向量库）

**关键实现**：
- Embedding：Volcengine Ark API (doubao-embedding-large-text)
- 检索：embed query → Milvus ANN search → 返回 top-K 结果
- 批量索引：分批 embed + upsert

**与单体对照**：从 `backend/internal/rag/` 整体迁出为独立服务。

---

### 2.14 CodeRunner 服务

**Proto RPC 清单（2 个）**：

| RPC | 来源 | 说明 |
|-----|------|------|
| Execute | 新增 | 执行代码 + 测试用例评判 |
| ListLanguages | 新增 | 支持的编程语言列表 |

**数据模型**：无（完全无状态）

**关键实现**：
- 封装 Piston API 调用
- 支持语言：Go, Python, JavaScript, Java, C++
- 超时控制：每次执行最多 10s
- 资源隔离：Piston 容器自带沙箱

**与单体对照**：从 `backend/internal/executor/piston.go` 提升为独立服务。理由：安全隔离 + 资源不可控 + 独立扩缩。

---

### 2.15 Admin 服务（BFF 编排层）

**设计哲学变更**：Admin 不再是一个拥有 75 个 RPC 实现的"上帝服务"。重新定位为 **管理后台 BFF（Backend For Frontend）**——它仍然对外暴露管理 API，但内部通过调用各域服务完成实际操作。

**RPC 分类与归属**：

| 类别 | 保留在 Admin | 委托给 | 说明 |
|------|-------------|--------|------|
| Dashboard | 是 | 聚合调用各服务 | 纯聚合，无自有逻辑 |
| 用户管理 | 是 | User.UpdateUserRole 等 | 编排层 |
| 题库 CRUD | 是 | Question 服务 | 编排层 |
| Question Pipeline | 是 | 发 MQ → Question 消费 | 异步编排 |
| 分类/行业管理 | 是 | Question 服务 | 编排层 |
| Prompt 模板管理 | 是 | AI Gateway | 委托 |
| AI 配置/预设/日志 | 是 | AI Gateway | 委托 |
| Live2D 管理 | 是 | 自有实现 | Admin 独有管理能力 |
| TTS 配置 | 是 | 自有实现 | Admin 独有管理能力 |
| RAG 配置/索引/文档 | 是 | RAG 服务 | 委托 |
| Scraper | 是 | 自有实现 + MQ | Admin 独有 |
| System Config | 是 | 自有实现 | KV 配置 |
| Pipeline Stream (SSE) | 新增 | 自有实现 | 补齐缺失 |

**Admin 自有数据模型**：
- `live2d_models` (id, name, path, config_json)
- `tts_configs` (id, provider, voice, scene, is_default)
- `scraper_sources` (id, platform, config_json)
- `scraper_tasks` (id, source_id, status, result_json, created_at)
- `system_configs` (key, value, updated_at)

**MQ 职责**：
- 生产者：
  - `question.pipeline.build`（触发题目生成）
  - `scraper.import.questions`（触发爬虫导入）

**新增 SSE 端点**：`GenerateQuestionPipelineStream` — Gateway 透传 SSE 连接到 Admin，Admin 监听 pipeline 任务进度并推送事件。

**与单体对照**：
- 单体 Admin 直接操作所有 DB 表 → 改为通过 gRPC 委托给各域服务
- 彻底删除 bridge 委托，Admin 不再持有单体的 service 引用

---

## 三、共享基础设施（pkg/）设计

### 3.1 保留为共享库的模块

| Package | 职责 | 为何是库而非服务 |
|---------|------|-----------------|
| `pkg/auth` | JWT 解析、Claims 结构、Context 注入 | 纯计算，无状态，无 I/O，每个服务启动时引入即可。不存在独立的"认证服务"运行时——token 签发在 User 服务，验证在各服务本地完成。 |
| `pkg/logger` | Zap 封装、文件轮转配置 | 日志写入是进程本地行为，不需要远程调用。每个服务引入相同配置即可。 |
| `pkg/errors` | 统一错误码定义、gRPC Status 映射 | 纯类型定义 + 工具函数，零状态。 |
| `pkg/pagination` | PageParam/PageResult 转换工具 | 纯数据映射函数，无状态。 |
| `pkg/middleware` | gRPC Interceptor（Tracing 传播、Recovery、Logging） | 中间件是进程内 hook，不是远程服务。 |
| `pkg/discovery` | etcd 服务发现 client 封装 | 连接配置工具，各服务本地使用。 |
| `pkg/mq` | RabbitMQ Publisher/Consumer 抽象接口 + TaskMessage 定义 | 连接管理库，不是消息路由服务本身。 |
| `pkg/model` | 共享 Protobuf Go 生成代码的别名/辅助 | 纯类型，零逻辑。 |

### 3.2 不放入 pkg/ 的模块（明确归属）

| 模块 | 归属决策 | 理由 |
|------|----------|------|
| AI Runtime (LLM 调用) | → AI Gateway 服务 | 有独立配置状态（DB 中的 ai_configs）、有运行时资源消耗（HTTP 调用 LLM）、有独立变更理由（换模型/换 provider）、有独立可观测需求（token 计量）。放入 pkg/ 会导致每个服务都依赖 LLM SDK + 配置 DB。 |
| RAG (向量检索) | → RAG 服务 | 有独立存储（Milvus）、有独立外部依赖（Embedding API）、有独立扩缩需求。 |
| TTS Provider | → Companion 服务内部 | 虽然是无状态 HTTP 调用，但 TTS 只被 Companion 使用。如果未来有多个消费者，再提取为服务。当前放在 Companion 内部避免过度拆分。 |
| Code Executor | → CodeRunner 服务 | 安全隔离要求 + 资源不可控。 |
| Scraper | → Admin 服务内部 | 只被管理后台使用的爬虫能力，不需要独立服务。 |

### 3.3 关于 AI 相关逻辑的特别论证

单体中 `backend/internal/ai/` 包含：RuntimeBuilder、Prompt 模板引擎、结构化输出解析、调用日志。

**为什么不做成 `pkg/ai` 共享库？**

1. **有状态**：AI 配置存储在 DB 中，可通过 Admin 热更新。如果做成库，每个服务都要连接配置 DB 并监听变更。
2. **资源密集**：LLM 调用耗时长、消耗大。独立服务便于限流、排队、监控。
3. **异构可能**：AI 领域 Python 生态远强于 Go。独立服务意味着未来可以用 Python 重写（LangChain、LlamaIndex），不影响 Go 业务服务。
4. **单一变更理由**：换 LLM provider、调整 prompt、加 guardrails，都只需修改 AI Gateway。

**结论**：AI 相关逻辑必须是独立服务（AI Gateway），通过 gRPC 被其他服务调用。`pkg/` 中不放任何 AI 相关代码。

---

## 四、跨服务通信与事件流设计

### 4.1 gRPC 同步调用矩阵

| 调用方 → 被调方 | 场景 |
|----------------|------|
| Gateway → User | 所有用户相关 HTTP 请求 |
| Gateway → Membership | 会员/订单 HTTP 请求 |
| Gateway → Question | 题库/练习 HTTP 请求 |
| Gateway → Interview | 面试 CRUD HTTP 请求 |
| Gateway → Growth | 成长数据 HTTP 请求 |
| Gateway → Plan | 计划管理 HTTP 请求 |
| Gateway → Companion | 伴侣对话 HTTP 请求 |
| Gateway → Community | 社区 HTTP 请求 |
| Gateway → Admin | 管理后台 HTTP 请求 |
| Interview → AI Gateway | 出题、答题评估、报告生成 |
| Interview → RAG | 面试中检索相关题目 |
| Interview → User | 获取用户 Profile（简历驱动模式） |
| Interview → CodeRunner | 编程题代码执行 |
| Realtime → Interview | 持久化 ASR/Chat 记录 |
| Realtime → RAG | 实时注入检索结果 |
| Realtime → AI Gateway | Live2D 指令生成 |
| Plan → AI Gateway | 计划生成、反馈诊断 |
| Plan → LearningArchive | 读取薄弱项/聚焦信号 |
| Plan → Interview | 读取面试统计 |
| Companion → AI Gateway | 对话生成、Live2D 指令 |
| Companion → LearningArchive | 读取学习状态 |
| Companion → Plan | 读取当前任务 |
| Growth → LearningArchive | 薄弱项、聚焦信号 |
| Growth → Interview | 面试统计 |
| Growth → Plan | 当前计划概要 |
| Growth → Question | 练习统计 |
| Question → AI Gateway | 考试批改、答题评估 |
| Question → CodeRunner | RunCode 执行 |
| Admin → AI Gateway | AI 配置/预设/调试/日志管理 |
| Admin → RAG | RAG 配置/索引/文档管理 |
| Admin → Question | 题库 CRUD |
| Admin → User | 用户管理 |
| Admin → Interview | 面试数据管理 |

### 4.2 MQ 拓扑详细设计

**Exchange**：

| Exchange | Type | 用途 |
|----------|------|------|
| `makejob.events` | topic | 业务域事件（跨域通知） |
| `makejob.tasks` | topic | 异步任务派发 |
| `makejob.tasks.retry` | topic | 重试延迟 |
| `makejob.tasks.dlx` | topic | 死信 |

**业务事件（makejob.events）**：

| 事件 | Routing Key | 生产者 | 消费者 | Payload 要点 |
|------|-------------|--------|--------|-------------|
| 面试完成 | `interview.finished` | Interview | LearningArchive | {user_id, interview_id, report_summary, dimension_scores} |
| 归档写入 | `archive.written` | LearningArchive | Growth | {user_id, entry_id, source_type, tags} |
| 题目变更 | `question.changed` | Question | RAG | {question_id, action: create/update/delete} |
| 会员升级 | `membership.upgraded` | Membership | Growth, Plan | {user_id, new_level, features} |

**异步任务（makejob.tasks）**：

| 任务 | Routing Key | 生产者 | 消费者 | Retry | Payload |
|------|-------------|--------|--------|-------|---------|
| 简历解析 | `interview.resume.parse` | Interview | Interview | 3次/30s | {interview_id, resume_text} |
| 报告生成 | `interview.report.generate` | Interview | Interview | 3次/45s | {interview_id} |
| 计划生成 | `plan.generate` | Plan | Plan | 3次/30s | {user_id, plan_config} |
| 反馈诊断 | `plan.feedback.diagnosis` | Plan | Plan | 3次/30s | {plan_id, task_id, feedback} |
| 题目流水线 | `question.pipeline.build` | Admin | Question | 5次/30s | {pipeline_config} |
| 爬虫导入 | `scraper.import.questions` | Admin | Question | 5次/20s | {task_id, questions[]} |
| RAG 同步 | `rag.sync.question` | Question | RAG | 3次/30s | {question_id, action} |

**Queue 命名规范**：`makejob.{type}.{domain}.{action}`
- 例：`makejob.tasks.interview.resume.parse`
- 每个 Queue 伴随 `.retry` 和 `.dlq`

### 4.3 WebSocket 路由设计

```
前端请求：ws://api.makejob.com/interviews/{id}/ws

Gateway 处理：
  1. 验证 JWT
  2. 调用 Interview.IsRealtimeInterview 确认面试类型
  3. 升级为 WebSocket
  4. 透传到 Realtime 服务（基于 interview_id hash 路由到具体实例）

连接维持：
  - Gateway 作为 L7 proxy，不解析 WebSocket 帧
  - 心跳由 Realtime 服务与客户端直接维护
  - 连接断开时 Realtime 自动调用 Interview.FinishInterview
```

### 4.4 SSE 流式端点

```
前端请求：GET /admin/question-pipeline/generate/stream

Gateway 处理：
  1. 验证 JWT + Admin 角色
  2. 透传 SSE 连接到 Admin 服务
  3. Admin 服务监听 pipeline 任务状态变化，推送 event stream
  4. 任务完成后关闭连接
```

---

## 五、实施路线图

### 设计原则
- 每个阶段结束后系统可正常运行
- Bridge 作为安全网，按域逐步摘除
- 每摘除一个域的 Bridge，该域必须通过完整集成测试

---

### Phase 0：基础设施就绪（约 2 人周）

**目标**：建立新架构骨架，不影响现有功能。

**任务**：
- 重新划分 Proto（新增 plan.proto, companion.proto, membership.proto, realtime.proto, coderunner.proto, rag.proto）
- 从 growth.proto 拆出 plan/companion RPC
- 重构 pkg/ 包结构
- 搭建 AI Gateway / RAG / CodeRunner / Realtime / Membership / Plan / Companion 服务骨架
- 确保 Bridge 模式回归通过

**完成指标**：新服务目录已创建，proto 代码已生成，服务可启动（RPC 返回 Unimplemented）。Bridge 不受影响。

**工作量**：2 人周

---

### Phase 1：基础设施服务实现（约 3 人周）

**目标**：实现三个纯基础设施服务——业务服务的依赖底座。

**任务**：
- AI Gateway 6/6 RPC（从单体迁移 RuntimeBuilder + Prompt 引擎 + 调用日志）
- RAG 8/8 RPC（从单体迁移 Milvus 集成 + 接入 MQ 消费 question.changed）
- CodeRunner 2/2 RPC（从单体迁移 Piston 封装 + 超时控制）

**完成指标**：3 个服务完整可用，gRPC 直连验证通过。

**工作量**：3 人周

---

### Phase 2：User + Membership + Community（约 2.5 人周）

**目标**：完成三个简单业务域，摘除对应 Bridge。

**任务**：
- User 9/9 RPC（修复 RefreshToken + 新增 Logout/UpdateUserRole）
- Membership 8/8 RPC（全新实现订单/支付/权益）
- Community 9/9 RPC（补齐 UpdatePost/ToggleLike/ListMyPosts + 增强）
- Gateway 摘除三个域的 Bridge 路由
- 集成测试

**完成指标**：前端 User/Membership/Community 页面 100% 走微服务。

**工作量**：2.5 人周

---

### Phase 3：Question 域（约 3 人周）

**目标**：补齐 Question 所有缺失功能。

**任务**：
- Question 22/22 RPC 全实现
- RunCode 接入 CodeRunner
- SubmitExam 接入 AI Gateway
- 接入 MQ 生产/消费
- Gateway 摘除 Question Bridge

**完成指标**：刷题、考试、题集、错题专题全部走微服务。

**工作量**：3 人周

---

### Phase 4：Interview + Realtime（约 4 人周，最复杂）

**目标**：完整面试流程 + 实时语音。

**任务**：
- Interview 14/14 RPC（出题/结束/报告/编程/Realtime系列）
- Realtime 5/5 RPC + WebSocket（Volcengine 迁移 + 双端中继）
- LearningArchive 接入 MQ 消费 interview.finished
- Gateway 摘除 Interview Bridge + WebSocket 路由
- 端到端面试测试

**完成指标**：创建面试→AI出题→答题→编程→结束→报告→语音面试 全链路。

**工作量**：4 人周

---

### Phase 5：Plan + Growth + Companion（约 3 人周）

**目标**：完成学习闭环。

**任务**：
- Plan 7/7 RPC（计划生成/调整/诊断）
- Growth 3/3 RPC 重写（并发聚合 + 模型兼容）
- Companion 3/3 RPC（AI对话 + TTS + Live2D）
- Gateway 摘除对应 Bridge

**完成指标**：学习计划→任务→反馈→诊断→调整 + 伴侣对话全链路。

**工作量**：3 人周

---

### Phase 6：Admin BFF + 删除 Bridge（约 2.5 人周）

**目标**：Admin 改为纯编排层，彻底删除 Bridge。

**任务**：
- Admin 内部改为委托各域服务
- 新增 GenerateQuestionPipelineStream SSE
- 删除 `backend/bridge/` 目录
- 删除 Gateway 所有 bridge 代码和数据库配置
- 全功能回归测试

**完成指标**：`backend/bridge/` 不存在，Gateway 无 DB 连接，100% gRPC。

**工作量**：2.5 人周

---

### Phase 7：优化清理（约 1.5 人周）

**任务**：清理 backend 残留、健康检查、Tracing 串联、CI/CD、压测、文档。

**工作量**：1.5 人周

---

### 总工作量

| Phase | 人周 | 累计 | 完成的独立服务 |
|-------|------|------|---------------|
| 0 | 2 | 2 | 骨架 ×7 |
| 1 | 3 | 5 | AI Gateway, RAG, CodeRunner |
| 2 | 2.5 | 7.5 | User, Membership, Community |
| 3 | 3 | 10.5 | Question |
| 4 | 4 | 14.5 | Interview, Realtime, LearningArchive(增强) |
| 5 | 3 | 17.5 | Plan, Growth, Companion |
| 6 | 2.5 | 20 | Admin(改造) |
| 7 | 1.5 | 21.5 | — |

**总计：约 21-22 人周**

---

## 六、架构收益分析

### 6.1 与当前状态对比

| 维度 | 当前状态 | 新方案 |
|------|----------|--------|
| **功能覆盖** | ~35%，大量桩实现和简化版 | 100%，每个 RPC 都有完整实现 |
| **Bridge 依赖** | 默认走 Bridge，微服务形同虚设 | Bridge 彻底删除，Gateway 纯代理 |
| **服务边界** | 8 服务，边界模糊（Growth 混杂三种职责） | 15 服务，每个服务单一职责 |
| **AI 耦合** | AI 逻辑散布在各 handler 中，通过 bridge 调用 | AI Gateway 统一收口，清晰接口 |
| **故障隔离** | Bridge 模式 = 单体，一处崩全崩 | 服务独立进程，故障不蔓延 |
| **独立部署** | 形式上独立，实际全走 bridge | 真正独立部署、独立扩缩 |
| **技术栈锁定** | 全 Go，AI 能力被 Go SDK 限制 | AI Gateway 可异构替换 |
| **数据主权** | 所有服务共享一个 DB 连接（通过 bridge） | 每服务私有 DB/表空间 |

### 6.2 化解审查报告核心问题

**问题 1：Bridge 模式使微服务形同虚设**
→ Phase 2-6 逐域摘除 Bridge，Phase 6 彻底删除。Gateway 回归纯代理职责。

**问题 2：大量 Proto RPC 有定义无实现**
→ 本方案要求每个服务的每个 RPC 都有完整实现。Phase 1-5 按域逐一补齐。

**问题 3：已实现方法功能大幅缩水**
→ 每个服务详细设计中明确标注"增强"的接口，要求返回结构补齐到单体水平。Growth 重写为并发聚合。

**问题 4：核心业务能力完全缺失**
→ 新增 AI Gateway（统一 AI 入口）、RAG（向量检索）、CodeRunner（代码执行）、Realtime（实时语音）四个基础设施服务，不再依赖 bridge 委托。

**问题 5：实现 Bug**
→ Phase 2 中 User 服务明确列出 RefreshToken 修复项：正确生成新 refresh_token + Login/Register 填充该字段。

### 6.3 核心架构优势

**可维护性**：
- 15 个服务各自只关注一个业务域，代码量可控（除 Admin 外，每个服务 < 2000 行核心逻辑）
- 新开发者只需理解一个服务即可开始贡献
- Proto 作为契约，变更影响可追踪

**可扩展性**：
- 面试高峰期只需扩容 Interview + Realtime + AI Gateway
- RAG 索引量增长只需扩容 RAG 服务
- 新增业务域只需新增一个服务，不影响已有服务

**故障隔离**：
- AI Gateway 宕机 → 面试/计划/伴侣不可用，但题库浏览/社区/用户管理不受影响
- Realtime 宕机 → 实时语音不可用，但文本面试正常
- CodeRunner 宕机 → RunCode 不可用，但答题评估正常

**技术栈异构**：
- AI Gateway：未来可用 Python 重写（LangChain 生态）
- RAG：未来可换用专业向量数据库服务
- CodeRunner：未来可换用 Docker-based 沙箱或云函数
- 所有替换对业务服务透明，只要 Proto 契约不变

**事件驱动解耦**：
- Interview 不需要知道 LearningArchive 的存在——只发事件
- Question 不需要知道 RAG 的索引逻辑——只发 changed 事件
- 新增消费者（如未来的"推荐引擎"）只需订阅已有事件，零修改

---

## 附录 A：Proto 文件重组对照

| 当前 Proto | 新方案 | 变更说明 |
|-----------|--------|---------|
| user.proto | user.proto | 去除 Membership RPC，新增 Logout/UpdateUserRole |
| — | membership.proto | 新文件，8 RPC |
| question.proto | question.proto | 新增 6 RPC |
| interview.proto | interview.proto | 不变（14 RPC 全部实现） |
| — | realtime.proto | 新文件，5 RPC |
| growth.proto | growth.proto | 缩减为 3 RPC（纯聚合） |
| — | plan.proto | 从 growth 拆出，7 RPC |
| — | companion.proto | 从 growth 拆出，3 RPC |
| community.proto | community.proto | 新增 ListMyPosts |
| ai.proto | ai.proto | 不变（6 RPC 全部实现） |
| — | rag.proto | 新文件，8 RPC |
| — | coderunner.proto | 新文件，2 RPC |
| admin.proto | admin.proto | 减少自有实现，增加委托逻辑 |
| learning_archive.proto | learning_archive.proto | 不变 |
| industry.proto | 合并入 question.proto | Industry 是 Question 域的子实体 |

---

## 附录 B：服务端口规划（开发环境）

| 服务 | gRPC 端口 | HTTP 端口 | 说明 |
|------|-----------|-----------|------|
| Gateway | — | 8080 | 对外唯一 HTTP 入口 |
| User | 9001 | — | |
| Membership | 9002 | — | |
| Question | 9003 | — | |
| Interview | 9004 | — | |
| Realtime | 9005 | 8085 (WS) | WebSocket 需 HTTP 升级 |
| Growth | 9006 | — | |
| Plan | 9007 | — | |
| Companion | 9008 | — | |
| Community | 9009 | — | |
| LearningArchive | 9010 | — | |
| AI Gateway | 9011 | — | |
| RAG | 9012 | — | |
| CodeRunner | 9013 | — | |
| Admin | 9014 | — | |