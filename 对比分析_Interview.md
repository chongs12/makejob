# Interview 服务 — 字段级差异分析

## 1. CreateInterview

**单体端点**: `POST /api/interviews` → `InterviewResponse`
**微服务 RPC**: `InterviewService.CreateInterview` → `InterviewResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `first_question.live2d_directive` | ✅ `{emotion, action, source, ...}` | ❌ 缺失（proto 有字段但 `toProtoQuestion` 未映射） | P0 | 修改 `toProtoQuestion` 函数映射 `Live2DDirective` |
| `async_task_id` | ✅ 有 | ❌ 始终为 0 | P2 | 异步任务 ID 未填充 |
| `task_status` | ✅ 有 | ❌ 始终为空 | P2 | 同上 |
| `task_error` | ✅ 有 | ❌ 始终为空 | P2 | 同上 |

**说明**: `live2d_directive` 缺失导致 Live2D 虚拟形象无法做出正确表情和动作。

---

## 2. GetInterview（面试详情）

**单体端点**: `GET /api/interviews/:id` → `InterviewDetailResponse`
**微服务 RPC**: `InterviewService.GetInterview` → `InterviewDetail`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `live2d_model_key` | ✅ 有 | ❌ 缺失 | P1 | proto `InterviewDetail` 无此字段 |
| `report`（嵌套） | ✅ 有（详情页内嵌报告） | ❌ 始终为 nil（需单独调用 `GetReport`） | P1 | 需前端改为两步调用，或在 `GetInterview` 中填充摘要 |
| `score` 类型 | ✅ float64 | ⚠️ int32（截断小数） | P2 | proto 定义为 `int32`，丢失精度 |

---

## 3. ListInterviews（面试列表）

**单体端点**: `GET /api/interviews` → `PageResult{list: []MockInterview}`
**微服务 RPC**: `InterviewService.ListInterviews` → `ListInterviewsResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `industry_code` | ✅ 有 | ❌ 缺失 | P1 | 列表页需要显示行业信息 |
| `first_question` | ✅ 有 | ❌ 缺失 | P2 | 列表不需要完整题目 |
| `interview_mode` | ✅ 有 | ❌ 缺失 | P1 | 列表页需要区分标准/实时/编码模式 |

---

## 4. SubmitAnswer（提交面试答案）

**单体端点**: `POST /api/interviews/:id/answer` → `InterviewAnswerResponse`
**微服务 RPC**: `InterviewService.SubmitAnswer` → `AnswerFeedback`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| 结构差异 | `{feedback: {...}, next_question, is_finished}` | 扁平 `{score, is_correct, feedback, key_points, suggestions, follow_up, next_question}` | P0 | Gateway `normalizeInterviewAnswerPayload` 已重构为单体格式，但 `is_finished` 始终为 false |
| `is_finished` | ✅ 标记面试是否结束 | ❌ 缺失（Gateway 设为 false） | P0 | 需在 service 层计算 `is_finished` 状态 |

**说明**: Gateway 的 `normalizeInterviewAnswerPayload` 会将扁平结构重组为嵌套结构，但 `is_finished` 字段被硬编码为 `false`。

---

## 5. FinishInterview（结束面试）

**单体端点**: `POST /api/interviews/:id/finish` → `InterviewReportResponse`
**微服务 RPC**: `InterviewService.FinishInterview` → `InterviewReport`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `report` | ✅ 完整报告对象 | ❌ 缺失（仅返回 `interview_id` + `status`） | P0 | 异步生成报告，前端需轮询 `GetReport` |
| `duration_seconds` | ✅ 有 | ❌ 缺失 | P1 | 需计算 `finished_at - created_at` |
| `completed_at` | ✅ 有 | ❌ 缺失 | P1 | 需返回 `finished_at` 时间戳 |
| `async_task_id` | ✅ 有 | ❌ 缺失 | P2 | 异步任务追踪 |

**说明**: 这是 P0 问题 — 前端调用 `finish` 后期望立即拿到报告数据，但微服务只返回一个 "generating" 状态。Gateway 的 `normalizeInterviewReportPayload` 会重组结构，但数据本身就是空的。

---

## 6. GetReport（获取报告）

**单体端点**: `GET /api/interviews/:id/report` → `InterviewReportResponse`
**微服务 RPC**: `InterviewService.GetReport` → `InterviewReport`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `duration_seconds` | ✅ 有 | ❌ 始终为 0 | P1 | 需在 report 生成时计算并存储 |
| `completed_at` | ✅ 有 | ❌ 始终为空 | P1 | 需返回 `interview.finished_at` |
| `async_task_id` | ✅ 有 | ❌ 始终为 0 | P2 | 异步任务追踪 |

---

## 7. GetInterviewStats（面试统计）

**单体端点**: 无直接对应（Growth 服务内部调用）
**微服务 RPC**: `InterviewService.GetInterviewStats` → `InterviewStats`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `total_questions_answered` | ✅ 有 | ❌ 始终为 0（proto 有字段但 SQL 未查询） | P1 | 需增加 SQL 聚合查询 |
| `avg_accuracy` | ✅ 有 | ❌ 始终为 0（proto 有字段但 SQL 未查询） | P1 | 需增加 SQL 聚合查询 |

**说明**: 这两个字段影响 Growth 服务的 `GrowthSummary` 聚合。

---

## 8. InterviewMessage（消息结构）

**单体**: 消息包含完整的 `question` 子对象（含 `live2d_directive`）
**微服务**: `InterviewMessage.question` 子对象不含 `live2d_directive`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `question.live2d_directive` | ✅ 有 | ❌ 缺失 | P0 | 修改 `toProtoQuestion` 映射 |
| `content` 处理 | 原始 JSON 内容 | ⚠️ 经过 `NormalizeMessageContent` 只返回 question 文本 | P1 | 确认前端是否需要原始 JSON |

---

## 9. CodingDiagnosis（编程诊断）

**单体**: `coding_diagnostics[].evidence` 是字符串数组
**微服务**: `evidence` 由 `splitEvidenceSummary` 从 `evidence_summary` 拆分生成

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `process_summary` | ✅ 独立字段 | ⚠️ 与 `evidence_summary` 相同值 | P2 | 语义相同但来源不同 |
| `evidence` | ✅ 原始证据数组 | ⚠️ 从 `evidence_summary` 拆分 | P2 | 拆分逻辑可能不准确 |
