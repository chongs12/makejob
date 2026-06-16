# 微服务 GORM 结构体与既有数据库表对齐计划（Interview / Plan）

> 执行方案：**方案 A —— 改微服务代码去对齐既有表，不动数据库表结构、不迁移既有数据，与单体共库一致。**
>
> 本文档供执行智能体逐项落地。每个改动点都给出了：**文件 → 当前错误字段 → 目标列 → 修改方法**。
>
> 生成日期：2026-06-14

---

## 0. 根因（先读这段，理解为什么要改）

单体（`docs/backend/`）先建好了数据库表并写入数据。微服务在后续重写时，**GORM struct 重新发明了一套列名**，与既有表不一致。GORM 的 `AutoMigrate` 不会删除/重命名既有列，只会尝试新增缺失列，因此：

- 查询/写入时 GORM 按 struct 字段推导列名（如 `IndustryCode` → `industry_code`），而表里根本没有该列 → `ERROR: column "xxx" does not exist`。
- 这就是 POST 类接口 500、而部分 GET 能work（恰好命中存在的列）的原因之一。

修复原则：**struct 字段名/标签必须映射到表里真实存在的列**。凡是表里没有的列，要么删字段、要么改名对齐、要么用 `gorm:"-"` 标记为非持久化。

---

## 1. 真实表结构（psql 已核实，作为唯一权威）

### 1.1 `mock_interviews`
```
id, created_at, updated_at, deleted_at,
user_id,
industry_id    (bigint, 外键 → industries.id),
status,
score          (numeric 5,2),
ai_feedback    (text),
total_questions(bigint),
started_at, ended_at,
ai_session_id  (text),
report_json    (text),
live2_d_model_key (varchar 128)
```

### 1.2 `interview_messages`
```
id, interview_id, role, content, message_type,
created_at  (bigint!  注意是 bigint 时间戳，不是 timestamptz),
metadata_json (text)
```
> 该表**没有** `updated_at`、`deleted_at`、`question_index`。

### 1.3 `learning_plans`
```
id, created_at, updated_at, deleted_at,
user_id,
industry_id (bigint, 外键 → industries.id),
title, description,
plan_json (text),
status,
total_tasks, completed_tasks,
start_date, end_date,
phase, phase_goal
```

### 1.4 `learning_tasks`
```
id, created_at, updated_at, deleted_at,
plan_id, title, description,
task_type, target_id,
status, due_date, completed_at,
sort_order, phase, phase_goal
```

### 1.5 `industries`（用于 code → id 解析）
```
id (bigint PK), created_at, updated_at, deleted_at,
code (varchar 50, UNIQUE),
name, description, icon, is_active, sort_order
```
> 关键：业务层一路传的是 **industry code 字符串**（如 `"java"`/`"frontend"`/`"1"`），但 `mock_interviews` 与 `learning_plans` 存的是 **`industry_id` bigint 外键**。必须在写库前做 code→id 解析。详见第 4 节。

---

## 2. Interview 服务对齐

### 2.1 文件：`app/interview/internal/data/model/interview.go`

#### MockInterview struct 字段映射表

| 当前字段（错误） | 推导出的列 | 表里真实列 | 处理方法 |
|---|---|---|---|
| `IndustryCode string` `gorm:"size:50"` | `industry_code` ❌ | `industry_id bigint` | **改为** `IndustryID uint64 gorm:"column:industry_id;index;not null"`，删除 IndustryCode |
| `Difficulty string` | `difficulty` ❌ | 无 | **删除**（表无此列；难度仅用于运行期 AI 出题，不落库）。若需保留运行期值，放到 biz 层而非 model |
| `InterviewMode string` | `interview_mode` ❌ | 无 | **删除**（表无此列；同上，运行期字段） |
| `QuestionCount int32` | `question_count` ❌ | `total_questions bigint` | **改为** `TotalQuestions int32 gorm:"column:total_questions"` |
| `CurrentIndex int32` | `current_index` ❌ | 无 | **删除**。当前 index 由 `interview_messages` 计数推导，不再单独存列（见 2.3 的 AppendMessageAndBumpIndex） |
| `OverallScore float64` | `overall_score` ❌ | `score numeric(5,2)` | **改为** `Score float64 gorm:"column:score"` |
| `ResumeText string` | `resume_text` ❌ | 无 | **删除**（表无此列；简历原文不落 mock_interviews） |
| `ResumeParsedJSON string` | `resume_parsed_json` ❌ | 无 | **删除** |
| `JobDescription string` | `job_description` ❌ | 无 | **删除** |
| `RealtimeDialogID string` | `realtime_dialog_id` ❌ | 无 | **删除**；如需实时对话 ID，可复用 `ai_session_id` |
| `FinishedAt *time.Time` | `finished_at` ❌ | `ended_at` | **改为** `EndedAt *time.Time gorm:"column:ended_at;index"` |
| `Live2DModelKey string` `gorm:"size:100"` | `live2_d_model_key` ✅ | `live2_d_model_key varchar(128)` | 保留，size 改 128 |
| （缺失） | — | `ai_feedback text` | **新增** `AIFeedback string gorm:"column:ai_feedback;type:text"` |
| （缺失） | — | `ai_session_id text` | **新增** `AISessionID string gorm:"column:ai_session_id;type:text"` |
| （缺失） | — | `report_json text` | **新增** `ReportJSON string gorm:"column:report_json;type:text"` |
| （缺失） | — | `started_at` | **新增** `StartedAt *time.Time gorm:"column:started_at"` |

> 参考单体权威实现：`docs/backend/internal/model/mock_interview.go`。对齐后字段集合应与单体一致：`UserID, IndustryID, Status, Score, AISessionID, Live2DModelKey, AIFeedback, ReportJSON, TotalQuestions, StartedAt, EndedAt`。

#### InterviewMessage struct 字段映射表

| 当前字段（错误） | 表里真实列 | 处理方法 |
|---|---|---|
| `QuestionIndex int32` → `question_index` ❌ | 无 | **删除** |
| `CreatedAt time.Time gorm:"autoCreateTime"` | `created_at` 是 **bigint**，不是 timestamptz | **改为** `CreatedAt int64 gorm:"column:created_at;autoCreateTime:milli"`（用 int64 毫秒时间戳，匹配 bigint 列） |
| `UpdatedAt time.Time gorm:"autoUpdateTime"` → `updated_at` ❌ | 无 | **删除** |
| `DeletedAt gorm.DeletedAt` → `deleted_at` ❌ | 无 | **删除** |
| （缺失） | `metadata_json text` | **新增** `MetadataJSON string gorm:"column:metadata_json;type:text"` |

> 保留：`ID, InterviewID, Role, Content, MessageType`。

#### InterviewCodingAttempt / InterviewReport
- 这两张表（`interview_coding_attempts`、`interview_reports`）是微服务新建的，单体没有。**先用 psql 核实它们是否真实存在及列结构**：
  ```
  docker exec my-postgres psql -U postgres -d makejob -c "\d interview_coding_attempts"
  docker exec my-postgres psql -U postgres -d makejob -c "\d interview_reports"
  ```
  若由微服务 AutoMigrate 自建且 struct 与表一致，则**无需改动**；若不一致按同样方法对齐。

### 2.2 文件：`app/interview/internal/data/interview_repo.go`（映射层联动改动）

struct 改名后，`toModel` / `toBiz` 以及多处显式列引用必须同步：

| 位置 | 当前代码 | 改为 |
|---|---|---|
| `toModel`（约 242-260 行） | `IndustryCode: iv.IndustryCode` | `IndustryID: iv.IndustryID`（biz.Interview 也要加 IndustryID，见 2.4） |
| `toModel` | `Difficulty/InterviewMode/QuestionCount/CurrentIndex/OverallScore/ResumeText/ResumeParsedJSON/JobDescription/RealtimeDialogID/FinishedAt` | 删除已删字段；`QuestionCount→TotalQuestions`、`OverallScore→Score`、`FinishedAt→EndedAt` |
| `toBiz`（约 262-282 行） | 同上反向 | 同步修改 |
| `CreateMessage`（90-98 行） | `QuestionIndex: msg.QuestionIndex` | 删除该行（列不存在） |
| `ListMessages` / `ListMessagesLimited` | 构造 biz 时含 `QuestionIndex: m.QuestionIndex` | 删除该字段赋值 |
| `BindRealtimeDialog`（213-217 行） | `Update("realtime_dialog_id", dialogID)` | 改为 `Update("ai_session_id", dialogID)` 或整体移除该方法（取决于 biz 是否仍调用） |
| `AppendMessageAndBumpIndex`（220-238 行） | `Update("current_index", gorm.Expr("current_index + 1"))` | **删除 bump 逻辑**（列不存在）。只保留 message 插入；当前题号改由 `COUNT(interview_messages)` 推导 |
| `GetStats`（285-304 行） | `SUM(current_index)`、`AVG(overall_score)`、`DATE(finished_at)` | 改为 `SUM(total_questions)`、`AVG(score)`、`DATE(ended_at)` |

> **特别注意 `created_at` 排序**：`interview_messages.created_at` 是 bigint，`ListMessages` 用 `Order("created_at ASC")` 仍有效（bigint 升序即时间序），无需改。

### 2.3 当前题号（current index）的替代方案

表里没有 `current_index` 列，原代码靠它递增追踪进度。改为运行期推导：
- 在需要 index 处，用 `SELECT COUNT(*) FROM interview_messages WHERE interview_id = ? AND role='assistant'`（或按业务定义）得到已出题数。
- `AppendMessageAndBumpIndex` 退化为单纯 `CreateMessage`，删掉事务里第二条 Update。
- 若 biz 层 `SubmitAnswer` 等依赖 `interview.CurrentIndex`，改为传入消息计数或调用方算好的 index。

### 2.4 文件：`app/interview/internal/biz/interview.go`（biz 实体联动）

`biz.Interview`（83-101 行）目前也用 `IndustryCode/Difficulty/...`。biz 是领域模型，可保留运行期字段（Difficulty/InterviewMode/QuestionCount/ResumeText 等仅在内存流转），但**必须新增 `IndustryID uint64`** 用于落库映射：
- 新增字段：`IndustryID uint64`
- `OverallScore→` 可保留 biz 名，但 toModel 映射到 `Score`
- `FinishedAt` biz 名可保留，toModel 映射到 `EndedAt`
- biz 字段不强制改名，**只要 toModel/toBiz 正确桥接 biz 名 ↔ 真实列**即可。关键是 model 层（持久化）必须用真实列。

### 2.5 文件：`app/interview/internal/biz/usecase.go`（CreateInterview 解析 industry）

`CreateInterview`（67-96 行）已调用 `uc.industry.GetIndustry(ctx, req.IndustryCode)` 但丢弃了结果（`_ = ind`）。改为：
1. `biz.Industry` 需要带上 `ID`（见第 4 节 industry_repo 改动）。
2. 取到 `ind.ID` 后赋值：`interview.IndustryID = ind.ID`，不再只存 code。
3. 运行期仍可保留 `IndustryCode` 在 biz 内存里传给 AI（`InterviewAgentRequest.IndustryCode`），但落库用 `IndustryID`。

---

## 3. Plan 服务对齐

> Plan 服务把 GORM 实体直接定义在 **biz 层**（`app/plan/internal/biz/plan.go`），repo 直接 `Create(plan)` 用 biz 实体，**没有独立的 data/model 包**。所以改动集中在 `biz/plan.go`（struct + 标签）和 `data/plan_repo.go`（Update map 的列名）。

### 3.1 文件：`app/plan/internal/biz/plan.go` — LearningPlan struct（37-58 行）

#### 字段映射表

| 当前字段（错误） | 推导出的列 | 表里真实列 | 处理方法 |
|---|---|---|---|
| `Level string` | `level` ❌ | 无 | **删除**（运行期值移到 CreatePlanRequest，不落库）；或 `gorm:"-"` |
| `DurationDays int32` | `duration_days` ❌ | 无 | **删除** / `gorm:"-"` |
| `DailyStudyMinutes int32` | `daily_study_minutes` ❌ | 无 | **删除** / `gorm:"-"` |
| `Industry string` `gorm:"size:50"` | `industry` ❌ | `industry_id bigint` | **改为** `IndustryID uint64 gorm:"column:industry_id;index;not null"` |
| `EntryPhase string` | `entry_phase` ❌ | 无 | **删除** / `gorm:"-"` |
| `AdjustmentSummariesJSON string` `type:jsonb` | `adjustment_summaries_json` ❌ | 无 | **删除** / `gorm:"-"` |
| `AdjustmentReasonCodesJSON string` | `adjustment_reason_codes_json` ❌ | 无 | **删除** / `gorm:"-"` |
| `PhaseBlueprintSummaryJSON string` | `phase_blueprint_summary_json` ❌ | 无 | **删除** / `gorm:"-"` |
| （缺失） | — | `plan_json text` | **新增** `PlanJSON string gorm:"column:plan_json;type:text"` |
| （缺失） | — | `start_date` | **新增** `StartDate *time.Time gorm:"column:start_date"` |
| （缺失） | — | `end_date` | **新增** `EndDate *time.Time gorm:"column:end_date"` |
| `Title/Description/Status/CompletedTasks/TotalTasks/Phase/PhaseGoal` | 对应列均存在 ✅ | 同名 | 保留 |

> 对齐后字段集合应与单体 `docs/backend/internal/model/learning_plan.go` 一致：`UserID, IndustryID, Title, Description, Phase, PhaseGoal, PlanJSON, Status, TotalTasks, CompletedTasks, StartDate, EndDate`。

#### 注意：被删字段在 biz 逻辑里的引用
`CreatePlan`（404-413 行）当前写：
```go
Title:             fmt.Sprintf("%s 学习计划", req.IndustryCode),
Level:             req.Level,         // 字段将删除
DurationDays:      req.DurationDays,  // 字段将删除
DailyStudyMinutes: req.DailyStudyMinutes, // 字段将删除
Industry:          req.IndustryCode,  // 改为 IndustryID
```
改为：
```go
Title:      fmt.Sprintf("%s 学习计划", req.IndustryCode),
IndustryID: industryID,   // 见第 4 节解析
Description: req.Goal,
Status:     "generating",
```
`Level/DurationDays/DailyStudyMinutes` 仍可从 `req`（CreatePlanRequest）继续传给 AI 生成（`PlanAgentRequest` 里已有这些字段），**只是不再落 learning_plans 表**。

### 3.2 文件：`app/plan/internal/biz/plan.go` — LearningTask struct（125-147 行）

#### 字段映射表

| 当前字段（错误） | 推导出的列 | 表里真实列 | 处理方法 |
|---|---|---|---|
| `DayNumber int32` | `day_number` ❌ | 无 | **删除** / `gorm:"-"` |
| `DurationMinutes int32` | `duration_minutes` ❌ | 无 | **删除** / `gorm:"-"` |
| `Priority string` | `priority` ❌ | 无 | **删除** / `gorm:"-"` |
| `Source string` | `source` ❌ | 无 | **删除** / `gorm:"-"` |
| `SourceLabel string` | `source_label` ❌ | 无 | **删除** / `gorm:"-"` |
| `Reason string` | `reason` ❌ | 无 | **删除** / `gorm:"-"` |
| `PriorityExplanation string` | `priority_explanation` ❌ | 无 | **删除** / `gorm:"-"` |
| `SourceRef string` | `source_ref` ❌ | 无 | **删除** / `gorm:"-"` |
| `CollectionHint string` | `collection_hint` ❌ | 无 | **删除** / `gorm:"-"` |
| （缺失） | — | `target_id` | **新增** `TargetID *uint64 gorm:"column:target_id"` |
| （缺失） | — | `due_date` | **新增** `DueDate *time.Time gorm:"column:due_date"` |
| `Title/Description/TaskType/Phase/PhaseGoal/Status/CompletedAt/SortOrder/PlanID` | 均存在 ✅ | 同名 | 保留（注意表有 `phase_goal`，struct 当前缺，建议补 `PhaseGoal string gorm:"column:phase_goal"`） |

> 对齐后应与单体 `docs/backend/internal/model/learning_task.go` 一致：`PlanID, Title, Description, TaskType, Phase, PhaseGoal, TargetID, Status, DueDate, CompletedAt, SortOrder`。

> **建议优先用 `gorm:"-"` 而非物理删除字段**：DayNumber/DurationMinutes/Priority 等在 `GeneratePlan`（489 行起）、`AddTasks`（797 行起）和 AI 响应里都有赋值。打 `gorm:"-"` 可让字段在内存里继续存在、参与逻辑，仅不落库，改动面最小、最安全。

### 3.3 文件：`app/plan/internal/data/plan_repo.go` — Update map 列名

#### planRepo.Update（59-71 行）
当前 map 含不存在的列，写库必报错：
```go
Updates(map[string]any{
    "title":               plan.Title,
    "description":         plan.Description,
    "level":               plan.Level,                // ❌ 删
    "duration_days":       plan.DurationDays,         // ❌ 删
    "daily_study_minutes": plan.DailyStudyMinutes,    // ❌ 删
    "industry":            plan.Industry,             // ❌ 改 industry_id
    "status":              plan.Status,
    "completed_tasks":     plan.CompletedTasks,
    "total_tasks":         plan.TotalTasks,
})
```
改为只保留真实列：
```go
Updates(map[string]any{
    "title":           plan.Title,
    "description":     plan.Description,
    "industry_id":     plan.IndustryID,
    "status":          plan.Status,
    "completed_tasks": plan.CompletedTasks,
    "total_tasks":     plan.TotalTasks,
    "phase":           plan.Phase,
    "phase_goal":      plan.PhaseGoal,
})
```

#### taskRepo.Update（148-161 行）
当前含 `day_number/duration_minutes/priority`（不存在列）：
```go
Updates(map[string]any{
    "title":            task.Title,
    "description":      task.Description,
    "task_type":        task.TaskType,
    "phase":            task.Phase,
    "day_number":       task.DayNumber,        // ❌ 删
    "duration_minutes": task.DurationMinutes,  // ❌ 删
    "priority":         task.Priority,         // ❌ 删
    "status":           task.Status,
    "completed_at":     task.CompletedAt,
    "sort_order":       task.SortOrder,
})
```
改为：
```go
Updates(map[string]any{
    "title":        task.Title,
    "description":  task.Description,
    "task_type":    task.TaskType,
    "phase":        task.Phase,
    "phase_goal":   task.PhaseGoal,
    "target_id":    task.TargetID,
    "due_date":     task.DueDate,
    "status":       task.Status,
    "completed_at": task.CompletedAt,
    "sort_order":   task.SortOrder,
})
```

> `planRepo.Create`（33-35 行）和 `taskRepo.BatchCreate`（122-127 行）用 `Create(plan)`/`CreateInBatches`，由 GORM 按 struct 标签推导列——只要 3.1/3.2 的 struct 标签对齐了，这两处**无需改**（前提是被删列用 `gorm:"-"` 而非残留默认推导）。

---

## 4. industry code → industry_id 解析（两服务共用，关键）

业务层一路传 industry **code 字符串**，但表存 **industry_id bigint 外键**。落库前必须把 code 换成 id。

### 4.1 Interview 服务
- `industry_repo.go` 的 `industryModel`（12-15 行）当前只有 `Code/Name`，**缺 ID**。
  - **新增** `ID uint64 gorm:"column:id;primaryKey"` 字段。
  - `industries` 表真实列：`id, code, name, description, icon, is_active, sort_order`（已 psql 核实）。
- `biz.Industry`（241 行起）**新增** `ID uint64`。
- `GetIndustry`（30-39 行）返回时填充 `ID: m.ID`。
- `CreateInterview`（usecase.go 67 行）已调用 `GetIndustry`，取 `ind.ID` 赋给 `interview.IndustryID`。

### 4.2 Plan 服务
- Plan 服务**当前没有 industry 仓储/查询**（CreatePlan 直接把 code 当字符串存）。需要新增 code→id 解析：
  - 方案 1（推荐，与 interview 一致）：在 plan/data 下加一个 `industryModel{ID, Code}` + 查询方法，CreatePlan 里解析 `req.IndustryCode → industryID`。
  - 方案 2：plan 服务通过 gRPC 调 question/其它服务拿 industry id（成本高，不推荐）。
- 解析失败（code 不存在）应返回 `ErrIndustryRequired` 或明确 400，不要写入 NULL/0 触发外键约束错误。

### 4.3 code 形态确认（已 psql 核实 2026-06-14）

`SELECT id, code, name FROM industries ORDER BY id;` 实际返回：

| id | code | name |
|---|---|---|
| 46 | `1` | go语言面试 |
| 47 | `java` | Java面试 |
| 48 | `frontend` | 前端面试 |

**结论（已确定，无需再假设）：**
- code 是 slug 字符串（`1` / `java` / `frontend`），**不是 industry_id**。注意 `code="1"` 看着像数字但对应的 id 是 46——**绝不能把 code 当 id 直接写入 `industry_id`**。
- id 非连续（从 46 起），与 code 无任何算术对应关系。
- **解析逻辑必须是「按 code 查 industries 表得到 id」**（`WHERE code = ?`），不能按 id 直查。
- 解析失败（code 不存在）返回 400，不要写 0/NULL（会触发外键约束 `fk_*_industry`）。

---

## 5. 执行顺序与验证

### 5.1 建议执行顺序
1. **先 Interview**：改 `data/model/interview.go`（struct）→ `data/interview_repo.go`（toModel/toBiz/各 Update）→ `data/industry_repo.go`（加 ID）→ `biz/interview.go`（加 IndustryID）→ `biz/usecase.go`（赋 IndustryID）。
2. **再 Plan**：改 `biz/plan.go`（两个 struct 标签）→ `data/plan_repo.go`（两个 Update map）→ 新增 industry 解析 → `biz/plan.go CreatePlan` 用 IndustryID。
3. 每改完一个服务先 `go build ./...` 通过，再起服务实测。

### 5.2 编译验证
```
cd D:/gogogo/makejob
go build ./app/interview/...
go build ./app/plan/...
```
删字段后会暴露所有引用点编译错误，**逐个按映射表修正**（这是用编译器帮你找全引用的好处；若用 `gorm:"-"` 保留字段则无编译错误，但要人工确认无残留落库）。

### 5.3 运行验证（改完后）
- 起 interview + ai_gateway，POST 创建面试，确认无 `column does not exist`，记录成功落 `mock_interviews`（industry_id 正确）。
- 起 plan + ai_gateway，POST 创建计划，确认落 `learning_plans`（industry_id、plan_json 正确），异步生成的 task 落 `learning_tasks`（target_id/due_date/phase_goal 正确）。
- 用 psql 抽查新写入行的列值是否符合预期。
- **测试完关闭临时起的调试服务。**

### 5.4 不要做的事
- ❌ 不要改数据库表结构 / 不要迁移既有数据（方案 A 的前提）。
- ❌ 不要给微服务表加 AutoMigrate 去「补列」——会污染单体共用的表。
- ❌ 不要假设 code 形态、不要假设 coding_attempts/reports 表结构，**psql 核实优先**。

---

## 6. 改动文件清单（速查）

| # | 文件 | 改动类型 |
|---|---|---|
| 1 | `app/interview/internal/data/model/interview.go` | MockInterview + InterviewMessage struct 字段/标签对齐 |
| 2 | `app/interview/internal/data/interview_repo.go` | toModel/toBiz、CreateMessage、ListMessages(Limited)、BindRealtimeDialog、AppendMessageAndBumpIndex、GetStats |
| 3 | `app/interview/internal/data/industry_repo.go` | industryModel + biz.Industry 增加 ID，GetIndustry 回填 |
| 4 | `app/interview/internal/biz/interview.go` | biz.Interview 增加 IndustryID；biz.Industry 增加 ID |
| 5 | `app/interview/internal/biz/usecase.go` | CreateInterview 用 ind.ID 赋 IndustryID |
| 6 | `app/plan/internal/biz/plan.go` | LearningPlan + LearningTask struct 对齐；CreatePlan 用 IndustryID |
| 7 | `app/plan/internal/data/plan_repo.go` | planRepo.Update + taskRepo.Update 的 map 列名对齐 |
| 8 | `app/plan/internal/data/`（新增） | plan 侧 industry code→id 解析（仓储或方法） |

---

## 7. 权威参考（对齐目标）
- `docs/backend/internal/model/mock_interview.go`
- `docs/backend/internal/model/learning_plan.go`
- `docs/backend/internal/model/learning_task.go`

以上单体 model 是表结构的「设计原稿」，对齐后微服务的持久化字段应与之一致。
