# Question 服务 — 字段级差异分析

## 1. ListQuestions

**单体端点**: `GET /api/questions` → `PageResult{list: []Question}`
**微服务 RPC**: `QuestionService.ListQuestions` → `ListQuestionsResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `category_id` | ✅ 有 | ❌ 缺失 | P1 | 在 `QuestionSummary` proto 增加 `category_id` 字段 |
| `industry_id` | ✅ 有 | ❌ 缺失 | P1 | 在 `QuestionSummary` proto 增加 `industry_id` 字段 |
| `is_favorited` | ✅ 有 | ❌ 缺失 | P0 | 需要查询 `user_favorites` 表，按当前用户填充 |
| `content` | ✅ 有（列表也返回） | ❌ 缺失 | P2 | 列表不需要完整 content，可接受 |
| `tags` | ✅ 有 | ❌ 缺失 | P1 | 在 `QuestionSummary` 增加 `tags` 字段 |
| `is_active` | ✅ 有 | ❌ 缺失 | P2 | 列表默认只返回 active，可接受 |
| `created_at` | ✅ 有 | ❌ 缺失 | P2 | 在 `QuestionSummary` 增加 `created_at` |

---

## 2. GetQuestion（题目详情）

**单体端点**: `GET /api/questions/:id` → `QuestionDetail`
**微服务 RPC**: `QuestionService.GetQuestion` → `QuestionDetail`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `is_favorited` | ✅ 有 | ❌ 缺失（proto 有字段但未填充） | P0 | service 层查询 `user_favorites` 表填充 |
| `user_note` | ✅ `{id, title, content}` | ❌ 缺失 | P0 | service 层查询 `user_notes` 表填充 |
| `tag_list` | ✅ `[]string` | ❌ 缺失 | P1 | Gateway `normalizeQuestionDetailPayload` 已添加 `tag_list`，但仅限详情端点 |
| `industry_id` | ✅ 有 | ❌ 缺失 | P1 | proto 有 `industry_code` 但无 `industry_id` |
| `industry_name` | ✅ 有 | ❌ 缺失 | P1 | 需关联查询 `industries` 表 |
| `judge_summary` | ✅ `{evaluation_mode, test_cases}` | ❌ 缺失 | P1 | proto 中 `judge_config_json` 是字符串，前端需自行解析 |
| `solution` | ✅ 嵌套对象 `{approach, steps, complexity}` | ⚠️ `solution_json` 字符串 | P1 | Gateway `normalizeQuestionDetailPayload` 会解析 JSON 字符串为对象 |
| `answer_template` | ✅ 嵌套对象 | ⚠️ `answer_template_json` 字符串 | P1 | 同上，Gateway 已处理 |
| `options` | ✅ `[]string` 数组 | ⚠️ `options_json` 字符串 | P1 | Gateway `normalizeQuestionDetailPayload` 会解析 |

**说明**: `is_favorited` 和 `user_note` 是最严重的问题 — 题目详情页的核心交互状态完全缺失。

---

## 3. SubmitAnswer（提交答案）

**单体端点**: `POST /api/questions/:id/submit` → `SubmitAnswerResponse`
**微服务 RPC**: `QuestionService.SubmitAnswer` → `SubmitAnswerResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `ai_analysis` | ✅ AI 分析 JSON 字符串 | ❌ 缺失 | P1 | 需 AI Gateway 返回完整分析结果 |
| `evaluation_mode` | ✅ 有 | ❌ 缺失 | P0 | 前端根据此字段决定渲染方式（选择题/编程题/主观题） |
| `judge_summary` | ✅ `{all_passed, total_cases, passed_cases, results[]}` | ❌ 缺失 | P0 | 编程题答案提交后无法显示测试用例结果 |
| `score` | ❌ 单体无此字段 | ✅ 有（float64） | — | 新增字段，需前端适配 |
| `key_points` | ❌ 单体无此字段 | ✅ 有（[]string） | — | 新增字段，需前端适配 |
| `feedback` | ❌ 单体无此字段 | ✅ 有（string） | — | 新增字段，需前端适配 |

**说明**: 单体的 `judge_summary` 包含每个测试用例的 input/expected/actual/passed 详情，微服务完全缺失。

---

## 4. RunCode（运行代码）

**单体端点**: `POST /api/questions/:id/run` → `RunCodeResponse`
**微服务 RPC**: `QuestionService.RunCode` → `RunCodeResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `evaluation_mode` | ✅ 有 | ❌ 缺失 | P1 | proto 增加字段或从题目信息获取 |
| `judge_summary` | ✅ `{all_passed, total_cases, passed_cases, results[]}` | ❌ 缺失 | P0 | 前端无法显示逐个测试用例的通过/失败状态 |
| `passed` | ✅ 布尔值 | ⚠️ `success` 字段名不同 | P1 | 确认前端读取的是 `success` 还是 `passed` |

**说明**: `judge_summary.results[]` 包含每个测试用例的 input/expected/actual/passed，是代码题核心交互。

---

## 5. ToggleFavorite（收藏切换）

**单体端点**: `POST /api/questions/:id/favorite` → `{is_favorited: bool}`
**微服务 RPC**: `CreateFavorite` / `DeleteFavorite` → `Empty`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `is_favorited` | ✅ 返回切换后的状态 | ❌ 无返回值 | P0 | 两个 RPC 都返回 Empty，前端无法知道当前状态 |

**说明**: 单体是一个 toggle 端点返回新状态，微服务拆成 create/delete 两个端点且无返回值。

---

## 6. ListFavorites（收藏列表）

**单体端点**: `GET /api/user/favorites` → `PageResult{list: []UserFavorite}`
**微服务 RPC**: `QuestionService.ListFavorites` → `FavoriteListResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `category_name` | ✅ 有 | ❌ 缺失 | P1 | `ListFavorites` 的 `QuestionSummary` 未填充 `category_name` |
| `content` | ✅ 有 | ❌ 缺失 | P2 | 列表不需要完整 content |

---

## 7. GetPracticeStats（练习统计）

**单体端点**: `GET /api/user/practice-stats` → `UserPracticeStats`
**微服务 RPC**: `QuestionService.GetUserPracticeStats` → `UserPracticeStats`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `streak_days` | ✅ 有 | ❌ 缺失 | P1 | proto 有字段但 service 层未填充 |
| `today_count` | ✅ 有 | ❌ 缺失（始终为 0） | P1 | 需查询当天的 `user_question_records` |
| `category_stats[].category_id` | ✅ 有 | ❌ 缺失 | P2 | proto `CategoryStat` 无 `category_id` 字段 |

---

## 8. GetPracticeRecommendations（练习推荐）

**单体端点**: `GET /api/user/practice-recommendations` → `PracticeRecommendationResponse`
**微服务 RPC**: `QuestionService.GetPracticeRecommendations` → `PracticeRecommendationResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| 结构差异 | `items[]` 每项包含完整 `QuestionDetail` | `questions[]` 每项仅 `QuestionSummary` 级别 | P0 | 推荐页无法展示题目详情，需嵌入完整题目对象 |
| `items[].question.solution` | ✅ 嵌套对象 | ❌ 缺失 | P1 | 推荐的题目需要展示解题思路 |
| `items[].question.judge_config` | ✅ 嵌套对象 | ❌ 缺失 | P1 | 编程题需要 judge 配置 |
| `items[].occurrence_count` | ✅ 有 | ❌ 始终为 0 | P1 | 需从 LearningArchive 聚合 |
| `items[].primary_question_set` | ✅ 有 | ❌ 空字符串 | P2 | 需关联 question_sets |
| `items[].related_question_sets` | ✅ 有 | ❌ 空数组 | P2 | 同上 |
| `items[].recommended_actions` | ✅ 有 | ❌ 空数组 | P2 | 同上 |

---

## 9. GetWrongQuestions（错题列表）

**单体端点**: `GET /api/user/wrong-questions` → `PageResult{list: []WrongQuestion}`
**微服务 RPC**: `QuestionService.GetWrongQuestions` → `WrongQuestionListResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `last_answer` | ✅ 用户上次作答内容 | ❌ 始终为空字符串（SQL 硬编码 `''`） | P1 | 修改 SQL 查询，JOIN `user_question_records` 获取实际答案 |

---

## 10. ExamResponse / SubmitExamResponse

**单体端点**: `POST /api/exams/submit` → `ExamResult`
**微服务 RPC**: `QuestionService.SubmitExam` → `SubmitExamResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `details[].user_answer` | ✅ 用户作答 | ❌ 缺失 | P1 | proto `QuestionResult` 无此字段 |
| `details[].correct_answer` | ✅ 正确答案 | ❌ 缺失（biz 有但未映射） | P0 | service 层需映射 `CorrectAnswer` 到 proto |
| `details[].explanation` | ✅ 解析 | ❌ 缺失 | P1 | proto `QuestionResult` 无此字段，仅有 `feedback` |

---

## 11. GetRandomExam / GenerateTimedExam

**单体端点**: `POST /api/exams/random` → `ExamResponse`
**微服务 RPC**: `QuestionService.GetRandomExam` → `ExamResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `exam_id` | ✅ string 类型 | ⚠️ `uint64` 类型 | P2 | 确认前端类型处理 |
| `time_limit` | ✅ 字段名 `time_limit` | ⚠️ 字段名 `time_limit_minutes` | P1 | 确认前端读取的字段名 |
| `questions[].options` | ✅ 数组 | ❌ 缺失（RandomSelect 不加载） | P0 | `RandomSelect` 方法需加载 `OptionsJSON` |
| `questions[].answer` | ✅ 有 | ❌ 缺失 | P1 | 考试模式可能不需要显示答案 |
| `questions[].tags` | ✅ 有 | ❌ 缺失 | P2 | 题目列表不需要 tags |

---

## 12. ListQuestionSets（题集列表）

**单体端点**: `GET /api/question-sets` → `[]QuestionSetSummary`
**微服务 RPC**: `QuestionService.ListQuestionSets` → `ListQuestionSetsResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `difficulty` | ✅ 有 | ❌ 缺失 | P2 | proto `QuestionSetSummary` 无此字段 |
| `cover_image` | ✅ 有 | ❌ 缺失 | P2 | proto 无此字段 |
| 返回结构 | 直接返回数组 | `{items: [], page_result: {}}` | P1 | Gateway `normalizeQuestionSetsPayload` 处理了 `focus_tags` 和 `questions` 数组 |

---

## 13. CategoryNode（分类树）

**单体端点**: `GET /api/categories` → `[]CategoryTree`
**微服务 RPC**: `QuestionService.ListCategories` → `CategoryTreeResponse`

| 项目 | 单体返回 | 微服务返回 | 优先级 | 修复建议 |
|------|---------|-----------|--------|---------|
| `question_count` | ✅ 每个分类的题目数 | ❌ 缺失 | P1 | 需要聚合查询或在 category 表中维护计数 |
| `icon` | ✅ 有 | ❌ 缺失 | P2 | proto `CategoryNode` 无此字段 |
| `description` | ✅ 有 | ❌ 缺失 | P2 | proto 无此字段 |
