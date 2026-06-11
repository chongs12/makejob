# Plan 服务 — 字段级差异分析

## 1. GetPlan / GetCurrentPlan（计划详情）

**单体端点**: `GET /api/plans/:id` / `GET /api/plans/current` → `PlanDetailResponse`
**微服务 RPC**: `PlanService.GetPlan` / `GetCurrentPlan` → `PlanDetail`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `industry_id` | ✅ 数字 ID | ❌ 缺失 | P1 | proto 无此字段，仅有 `industry`（字符串） |
| `phase` | ✅ 当前阶段 `foundation/drill/mock` | ❌ 缺失 | P0 | 前端需要展示当前学习阶段 |
| `phase_goal` | ✅ 阶段目标 | ❌ 缺失 | P0 | 前端需要展示阶段说明 |
| `entry_phase` | ✅ 入口阶段 | ❌ 缺失 | P2 | 用于计划调整 |
| `adjustment_summaries` | ✅ 调整摘要列表 | ❌ 缺失 | P1 | 需从调整记录表查询 |
| `adjustment_reason_codes` | ✅ 调整原因代码 | ❌ 缺失 | P2 | 同上 |
| `phase_blueprint_summary` | ✅ 阶段蓝图 `[{phase, phase_goal, start_day, end_day, expected_task_types, exit_criteria}]` | ❌ 缺失 | P0 | 前端需要展示学习路线图 |
| `tasks[].source` | ✅ 任务来源 `weekly_focus/weak_topic/goal/...` | ❌ 缺失 | P1 | 需在任务创建时记录来源 |
| `tasks[].source_label` | ✅ 来源显示文本 | ❌ 缺失 | P1 | 同上 |
| `tasks[].reason` | ✅ 推荐原因 | ❌ 缺失 | P1 | 同上 |
| `tasks[].priority_explanation` | ✅ 优先级说明 | ❌ 缺失 | P2 | 同上 |
| `tasks[].source_ref` | ✅ 来源引用 | ❌ 缺失 | P2 | 同上 |
| `tasks[].collection_hint` | ✅ 集合提示 | ❌ 缺失 | P2 | 同上 |
| `tasks[].due_date` | ✅ 截止日期 | ❌ 缺失 | P1 | proto `TaskDetail` 无此字段 |
| `tasks[].phase_goal` | ✅ 阶段目标 | ❌ 缺失 | P1 | proto `TaskDetail` 无此字段 |

---

## 2. ListPlans（计划列表）

**单体端点**: `GET /api/plans` → `PageResult{list: []LearningPlan}`
**微服务 RPC**: `PlanService.ListPlans` → `ListPlansResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `description` | ✅ 有 | ❌ 缺失（`PlanSummary` 无此字段） | P2 | 列表不需要完整描述 |
| `level` | ✅ 有 | ❌ 缺失 | P2 | 列表可不展示 |
| `duration_days` | ✅ 有 | ❌ 缺失 | P1 | 列表页需要展示计划天数 |
| `daily_study_minutes` | ✅ 有 | ❌ 缺失 | P2 | 列表可不展示 |

---

## 3. CreatePlan（创建计划）

**单体端点**: `POST /api/plans` → `PlanDetailResponse`
**微服务 RPC**: `PlanService.CreatePlan` → `PlanResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| 返回内容 | 完整 `PlanDetailResponse` | 仅 `{plan_id, status, created_at}` | P1 | 异步生成，前端需轮询 `GetPlan` 获取完整数据 |
| `tasks` | ✅ 立即返回任务列表 | ❌ 缺失（异步生成中） | P1 | 前端需等待生成完成后重新获取 |

---

## 4. UpdateTaskStatus（更新任务状态）

**单体端点**: `PUT /api/plans/:id/tasks/:taskId` → `null`
**微服务 RPC**: `PlanService.UpdateTaskStatus` → `UpdateTaskStatusResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| 返回值 | `null`（无返回） | `{task_status, plan_status, completed_tasks, total_tasks, progress}` | — | 微服务返回更丰富，需前端适配 |

---

## 5. SubmitTaskFeedback（提交任务反馈）

**单体端点**: `POST /api/plans/:id/tasks/:taskId/feedback` → `null`
**微服务 RPC**: `PlanService.SubmitTaskFeedback` → `FeedbackResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| 返回值 | `null` | `{feedback_id, status, diagnosis, suggestions}` | — | 微服务返回更丰富，需前端适配 |
| `diagnosis` | — | ⚠️ 始终为空字符串（异步生成） | P1 | 前端需轮询或等待 MQ 回调 |

---

## 6. AdjustPlan（调整计划）

**单体端点**: `POST /api/plans/:id/adjust` → `PlanDetailResponse`（调整后的完整计划）
**微服务 RPC**: `PlanService.AdjustPlan` → `AdjustPlanResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| 返回结构 | 完整 `PlanDetailResponse` | `{tasks_added, tasks_removed, tasks_reordered, adjustment_summary, updated_tasks[]}` | P1 | 前端需要适配新结构，或 Gateway 重组 |

---

## 7. GetProgress（进度统计）

**单体端点**: `GET /api/plans/:id/progress` → `PlanProgressResponse`
**微服务 RPC**: `PlanService.GetProgress` → `PlanProgressResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| 基本对齐 | ✅ | ✅ | — | 结构基本一致 |

此接口差异较小，基本对齐。
