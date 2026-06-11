# Gateway 转换层 — 字段级差异分析

## 概述
Gateway 是 REST→gRPC 的转换层，所有前端请求经过 Gateway 的 `normalizeJSONValue()` + `adaptLegacyResponseByPath()` 处理。以下分析 Gateway 转换可能导致的字段丢失或结构变化。

---

## 1. unwrapSingleListField — 数组外层键被丢弃

**影响**: 所有返回单一数组字段的 gRPC 响应

| 单体返回 | Gateway 转换后 | 影响端点 |
|---------|---------------|---------|
| `{industries: [...]}` | `[...]` | `ListIndustries` |
| `{categories: [...]}` | `[...]` | `ListCategories` |
| `{templates: [...]}` | `[...]` | `ListPromptTemplates` |
| `{models: [...]}` | `[...]` | `ListLive2DModels` |
| `{sources: [...]}` | `[...]` | `GetScraperSources` |

**优先级**: P1
**修复建议**: 确认前端是否直接读取数组还是读取包装键。如果前端读取 `response.data.industries`，则会得到 `undefined`。

---

## 2. flattenPageResult — 分页数组键被替换为 `list`

**影响**: 所有分页列表端点

| 单体返回 | Gateway 转换后 | 影响端点 |
|---------|---------------|---------|
| `{questions: [...], page_result: {total, page, page_size}}` | `{list: [...], total: 10, page: 1, page_size: 20}` | `ListQuestions`, `ListFavorites`, `AdminListQuestions` |
| `{interviews: [...], page_result: {...}}` | `{list: [...], total, page, page_size}` | `ListInterviews` |
| `{posts: [...], page_result: {...}}` | `{list: [...], total, page, page_size}` | `ListPosts`, `ListMyPosts` |
| `{users: [...], page_result: {...}}` | `{list: [...], total, page, page_size}` | `ListUsers` |
| `{logs: [...], page_result: {...}}` | `{list: [...], total, page, page_size}` | `ListAICallLogs` |

**优先级**: P0
**修复建议**: 前端所有分页列表读取路径从 `response.data.questions` 改为 `response.data.list`，或在 Gateway 中保留原始键名。

---

## 3. normalizeInterviewReportPayload — 报告结构重组

**影响**: `FinishInterview` 和 `GetReport` 的 `/report` 和 `/finish` 端点

**单体返回**:
```json
{
  "interview_id": 1,
  "status": "completed",
  "report": { "overall_score": 85, "total_questions": 10, ... },
  "duration_seconds": 1800,
  "completed_at": "..."
}
```

**微服务 gRPC 返回** (扁平):
```json
{
  "interview_id": 1,
  "status": "completed",
  "overall_score": 85,
  "total_questions": 10,
  ...
}
```

**Gateway 转换后**:
```json
{
  "interview_id": 1,
  "status": "completed",
  "report": { "interview_id": 1, "status": "completed", "overall_score": 85, ... },
  "duration_seconds": 0,
  "completed_at": ""
}
```

**优先级**: P0
**问题**: Gateway 会将扁平结构重组为嵌套结构，但 `duration_seconds` 和 `completed_at` 始终为零值/空值（微服务未填充），导致 `report` 内部也包含重复的顶层字段。

---

## 4. normalizeInterviewAnswerPayload — 答案反馈重组

**影响**: `SubmitAnswer` 的 `/answer` 端点

**单体返回**:
```json
{
  "feedback": { "score": 85, "is_correct": true, "feedback": "...", "key_points": [...] },
  "next_question": { "question": "...", ... },
  "is_finished": false
}
```

**微服务 gRPC 返回** (扁平):
```json
{
  "score": 85, "is_correct": true, "feedback": "...", "key_points": [...],
  "suggestions": "...", "follow_up": "...", "next_question": { ... }
}
```

**Gateway 转换后**:
```json
{
  "feedback": { "score": 85, "is_correct": true, "feedback": "...", ... },
  "next_question": { ... },
  "is_finished": false
}
```

**优先级**: P0
**问题**: `is_finished` 被硬编码为 `false`，前端无法知道面试是否结束。

---

## 5. normalizePracticeRecommendationPayload — 推荐结构重组

**影响**: `GetPracticeRecommendations` 的 `/recommendations` 端点

**单体返回**:
```json
{
  "focus_tags": ["tag1"],
  "items": [
    {
      "question": { "id": 1, "title": "...", "content": "...", ... },
      "focus_tag": "tag1",
      "topic_code": "...",
      ...
    }
  ]
}
```

**微服务 gRPC 返回**:
```json
{
  "questions": [
    { "question_id": 1, "title": "...", "difficulty": "easy", "type": "choice", ... }
  ],
  "reason": "...",
  "focus_tags": ["tag1"]
}
```

**Gateway 转换后**: Gateway 会尝试重组，但 `question` 对象只是 `QuestionSummary` 级别，不是完整的 `QuestionDetail`。

**优先级**: P0
**问题**: 推荐的题目缺少 `content`、`options`、`answer`、`explanation` 等详情字段，前端无法展示完整题目。

---

## 6. normalizeQuestionSetsPayload — 题集数组保证

**影响**: `ListQuestionSets` 和 `GetQuestionSetDetail`

Gateway 确保 `focus_tags` 和 `questions` 始终为数组（即使后端返回 null）。

**优先级**: P2 — 已正确处理

---

## 7. normalizeGrowthSummaryPayload — 成长数据数组保证

**影响**: `GetGrowthSummary`

Gateway 确保以下 4 个数组字段始终存在（即使后端返回空）:
- `recent_study_logs`
- `recent_interviews`
- `recent_plans`
- `weekly_stats`

**优先级**: P2 — Gateway 已补偿，但数据始终为空

---

## 8. omitempty 零值字段丢失

**影响**: 所有没有专用 normalizer 的端点

protobuf 生成的 Go 结构体使用 `json:"...,omitempty"`，导致:
- 空字符串字段被省略
- 零值整数字段被省略
- `false` 布尔字段被省略
- 空数组字段被省略
- nil 嵌套消息被省略

**受影响的端点** (无专用 normalizer):
- `GetQuestion` — `is_favorited: false` 被省略
- `GetPost` — `is_liked: false`, `is_author: false` 被省略
- `ListComments` — `is_author: false` 被省略
- `GetMembershipStatus` — `is_active: false` 被省略

**优先级**: P1
**修复建议**: 为这些端点添加 Gateway normalizer，使用 `ensureBoolField()` 补偿零值布尔字段。
