# Growth 服务 — 字段级差异分析

## 1. GetGrowthSummary（成长总览）

**单体端点**: `GET /api/user/growth-summary` → `GrowthSummaryResponse`
**微服务 RPC**: `GrowthService.GetGrowthSummary` → `GrowthSummary`

这是差异最集中的接口，影响成长主页的核心展示。

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `weekly_stats` | ✅ 周统计数据 `[{week, questions_answered, interviews_taken, avg_score}]` | ❌ 始终为空数组 `[]` | P0 | 需要从 study_log 和 interview 表按周聚合 |
| `recent_study_logs` | ✅ 最近学习记录 `[{id, date_key, summary, focus_task_title, completed_count, ...}]` | ❌ 始终为空数组 `[]` | P0 | 需查询 `study_logs` 表最近 N 条 |
| `recent_interviews` | ✅ 最近面试 `[{id, status, score, total_questions, created_at, ended_at}]` | ❌ 始终为空数组 `[]` | P0 | 需调用 Interview 服务 `ListInterviews` |
| `recent_plans` | ✅ 最近计划 `[{id, title, status, total_tasks, completed_tasks, progress, start_date, end_date}]` | ❌ 始终为空数组 `[]` | P0 | 需调用 Plan 服务 `ListPlans` |
| `practice_stats.today_count` | ✅ 今日练习数 | ❌ 始终为 0 | P1 | 需查询当天 `user_question_records` |
| `practice_stats.category_stats` | ✅ 分类统计 `[{category_id, category_name, total, correct, accuracy_rate}]` | ❌ 始终为空数组 `[]` | P0 | 需从 Question 服务 `GetUserPracticeStats` 获取 |
| `completed_interview_count` | ✅ 已完成面试数 | ⚠️ 等于 `total_interviews`（未区分完成/总数） | P1 | 需 SQL 区分 `status='completed'` |
| `current_plan.id` | ✅ 计划 ID | ❌ 始终为 0 | P1 | Plan 服务返回了 ID 但 Growth 未映射 |
| `current_plan.status` | ✅ 计划状态 | ❌ 始终为空 | P1 | 同上 |
| `current_plan.next_task_title` | ✅ 下一个任务标题 | ❌ 始终为空 | P1 | 需查询 Plan 服务获取下一个 pending 任务 |
| `current_plan.next_task_source` | ✅ 任务来源 | ❌ 始终为空 | P2 | 需从任务元数据获取 |
| `current_plan.next_task_reason` | ✅ 推荐原因 | ❌ 始终为空 | P2 | 同上 |
| `focus_signals[].topic_code` | ✅ 有 | ❌ 始终为空 | P2 | LearningArchive 未返回此字段 |
| `focus_signals[].topic_problem_pattern` | ✅ 有 | ❌ 始终为空 | P2 | 同上 |
| `focus_signals[].related_question_sets` | ✅ 有 | ❌ 始终为空数组 | P2 | 同上 |

**说明**: 成长主页 6 个核心区块中有 4 个（周统计、最近学习、最近面试、最近计划）完全为空，页面大面积白屏。

---

## 2. GetWeeklyFocus（每周聚焦）

**单体端点**: `GET /api/user/weekly-focus` → `WeeklyFocusResponse`
**微服务 RPC**: `GrowthService.GetWeeklyFocus` → `WeeklyFocus`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `themes[].topic_codes` | ✅ 有 | ❌ 始终为空数组 | P1 | 需从 LearningArchive 获取关联 topic codes |
| `themes[].related_question_sets` | ✅ 有 | ❌ 始终为空数组 | P1 | 需从 Question 服务获取关联题集 |
| `themes[].suggestions` | ✅ 有 | ❌ 始终为空数组 | P1 | 需 AI 生成或从 FocusSignal 提取 |
| `themes[].interview_occurrence_count` | ✅ 有 | ❌ 始终为 0 | P1 | 需从 Interview 统计中获取 |
| `themes[].dominant_archive_phase` | ✅ 有 | ❌ 始终为空 | P2 | 需从 LearningArchive 聚合 |
| `themes[].dominant_archive_phase_label` | ✅ 有 | ❌ 始终为空 | P2 | 同上 |

---

## 3. SyncStudyLog（同步学习日志）

**单体端点**: `PUT /api/user/study-logs/daily` → `StudyLogResponse`
**微服务 RPC**: `GrowthService.SyncStudyLog` → `StudyLog`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| 结构完全不同 | `{id, date_key, summary, focus_task_title, completed_count, skipped_count, completed_titles, skipped_titles, latest_action_text, updated_at}` | `{id, user_id, action, ref_id, created_at}` | P0 | Proto 定义与单体完全不同，需重新设计 `StudyLog` message |
| `date_key` | ✅ 日期键 | ❌ 缺失 | P0 | 前端需要此字段展示日历 |
| `summary` | ✅ 学习摘要 | ❌ 缺失 | P1 | 需 AI 生成或从任务数据聚合 |
| `completed_count` | ✅ 完成任务数 | ❌ 缺失 | P1 | 需从 Plan 服务获取 |
| `skipped_count` | ✅ 跳过任务数 | ❌ 缺失 | P1 | 同上 |
| `completed_titles` | ✅ 完成任务标题列表 | ❌ 缺失 | P1 | 同上 |
| `latest_action_text` | ✅ 最近操作文本 | ❌ 缺失 | P2 | 需从学习记录聚合 |

**说明**: 这是结构性差异 — 单体的 `StudyLogResponse` 是一个丰富的日聚合对象，微服务的 `StudyLog` 只是一个原始事件记录。
