# RabbitMQ 集成修改计划

## 现状与痛点分析

当前 `backend/` 内存在三类异步问题：

1. `backend/internal/service/plan_diagnosis_support.go`
   当前通过进程内 `go func()` 做学习任务反馈诊断。问题是无持久化、无重试、进程重启直接丢任务。
2. `backend/internal/service/scraper_service.go`
   当前题库导入任务先写 `scraper_tasks`，再依赖 `backend/cmd/worker/main.go` 轮询领取。问题是触发和消费链路分离，但没有统一消息总线。
3. `backend/internal/service/admin_question_pipeline_task.go`
   后台题目流水线同样依赖 `scraper_tasks + worker 轮询`，已经具备明显“穷人版消息队列”特征。

同步重链路仍然存在但当前未直接切换：

- `backend/internal/service/interview_service.go`
  - 简历驱动面试创建时的简历解析
  - 面试结束时的评分补齐、报告生成、编程诊断、学习档案沉淀
- `backend/internal/service/plan_service.go`
  - 学习计划生成

这些链路已经在 RabbitMQ 拓扑和消息结构中预留，但本轮优先落地最急迫、最不破坏现有接口的三块：

- 学习任务反馈诊断
- 爬虫导入任务
- 后台题目流水线任务

## 设计目标与原则

1. RabbitMQ 负责任务分发，数据库负责任务审计与幂等。
2. 低延迟实时链路不经过 MQ。
   `backend/internal/handler/interview_handler_realtime.go` 相关 WebSocket 会话、逐轮问答、实时语音流仍然保持直连。
3. 先兼容单体，再兼容微服务。
   交换机、路由键、消息结构都按“未来独立服务消费”设计，不依赖进程内函数调用。
4. 先迁移高价值异步任务，再扩展同步重链路。
5. MQ 不可用时允许回退：
   - `plan diagnosis` 回退进程内异步
   - `scraper/admin pipeline` 回退现有 DB 轮询 worker

## 当前实施状态

- 已完成：RabbitMQ 配置项与容器编排
- 已完成：`async_tasks` 通用任务表与仓储
- 已完成：`internal/mq` 拓扑、发布器、消费者、重试与死信封装
- 已完成：学习任务反馈诊断接入 `async_tasks + RabbitMQ`
- 已完成：爬虫导入任务接入 RabbitMQ 投递
- 已完成：后台题目流水线任务接入 RabbitMQ 投递
- 已完成：简历驱动面试创建接入 RabbitMQ 异步简历解析
- 已完成：面试结束后的报告生成、编程诊断、学习档案沉淀接入 RabbitMQ
- 已完成：`backend/cmd/worker/main.go` 增加 RabbitMQ 消费模式
- 已完成：学习计划生成接入 RabbitMQ 异步链路
- 已完成：前端面试/学习陪伴页接入“准备中 / 生成中”状态轮询与禁用态

## RabbitMQ 拓扑设计

### 交换机

- 主交换机：`makejob.tasks.topic`
  类型：`topic`
  用途：所有正式业务任务统一投递入口
- 重试交换机：`makejob.tasks.retry`
  类型：`topic`
  用途：失败消息进入带 TTL 的重试队列
- 死信交换机：`makejob.tasks.dlx`
  类型：`topic`
  用途：超过最大重试次数的消息落地排障

### 队列与路由键

| 任务类型 | 路由键 | 业务队列 | 当前状态 |
| --- | --- | --- | --- |
| 学习反馈诊断 | `plan.feedback.diagnosis` | `makejob.async.plan.feedback.diagnosis` | 已接入 |
| 爬虫导入 | `scraper.import.questions` | `makejob.async.scraper.import.questions` | 已接入 |
| 后台题目流水线 | `admin.question.pipeline.build` | `makejob.async.admin.question.pipeline.build` | 已接入 |
| 简历解析 | `interview.resume.parse` | `makejob.async.interview.resume.parse` | 已接入 |
| 面试报告生成 | `interview.report.generate` | `makejob.async.interview.report.generate` | 已接入 |
| 面试档案沉淀 | `interview.archive.persist` | `makejob.async.interview.archive.persist` | 已预留 |
| 学习计划生成 | `plan.generate` | `makejob.async.plan.generate` | 已接入 |

每个业务队列都自动派生：

- 重试队列：`<queue>.retry`
- 死信队列：`<queue>.dlq`

### 文字版架构图

```text
HTTP/API Service
  -> 写业务表 / async_tasks / scraper_tasks
  -> publish to makejob.tasks.topic

makejob.tasks.topic
  -> business queue
  -> worker consumer
     -> success: 更新业务表 / async_tasks 为 succeeded
     -> fail and retryable: publish to makejob.tasks.retry
     -> fail and exhausted: publish to makejob.tasks.dlx

makejob.tasks.retry
  -> <queue>.retry (TTL)
  -> TTL 到期后回投 makejob.tasks.topic

makejob.tasks.dlx
  -> <queue>.dlq
```

### 为什么这套拓扑适合未来微服务拆分

1. 服务拆分时只需要把消费者搬到新进程，不需要改消息结构。
2. 路由键按业务域命名，不依赖当前单体目录结构。
3. `TaskMessage` 自带 `Version / TaskType / EntityType / EntityID / IdempotencyKey`，适合跨服务演进。
4. `async_tasks` 只承接“通用审计任务”；`scraper_tasks` 继续承接后台任务台，避免前台管理页面被一次性推翻。

## 消息格式规范

统一消息结构位于 `backend/internal/mq/message.go`：

```go
type TaskMessage struct {
    Version        string
    MessageID      string
    TaskType       string
    Source         string
    TaskID         uint
    EntityType     string
    EntityID       uint
    IdempotencyKey string
    CreatedAt      time.Time
    Payload        json.RawMessage
}
```

当前已落地的 payload：

```go
type PlanFeedbackDiagnosisPayload struct {
    PlanID     uint
    TaskID     uint
    FeedbackID uint
    UserID     uint
}

type ScraperImportPayload struct {
    ScraperTaskID uint
}

type AdminQuestionPipelinePayload struct {
    ScraperTaskID uint
}
```

已落地并可消费的 payload：

- `InterviewResumeParsePayload`
- `InterviewReportPayload`
- `PlanGeneratePayload`

仍预留：

- `InterviewArchivePayload`

## 改造点与示例代码

### 1. 通用异步任务表

涉及文件：

- `backend/internal/model/async_task.go`
- `backend/internal/repository/async_task_repo.go`
- `backend/internal/model/database.go`

关键点：

- 新增 `async_tasks` 表
- 记录 `TaskType / Status / QueueName / RoutingKey / IdempotencyKey / PayloadJSON / ResultJSON / RetryCount / MaxRetries`
- `ClaimByID()` 使用数据库行锁避免重复消费

### 2. 学习任务反馈诊断

涉及文件：

- `backend/internal/service/plan_service.go`
- `backend/internal/service/plan_diagnosis_support.go`
- `backend/cmd/server/main.go`
- `backend/cmd/worker/main.go`

当前实现：

1. `SubmitTaskFeedback()` 仍然先写反馈表。
2. `enqueueTaskFeedbackDiagnosis()` 优先走 RabbitMQ：
   - 创建或复用 `async_tasks`
   - 发布 `plan.feedback.diagnosis`
3. RabbitMQ 不可用时回退到原来的 `go func()`。
4. worker 通过 `ProcessTaskFeedbackDiagnosisTask()` 消费：
   - `ClaimByID()`
   - 读取 `LearningPlan / LearningTask / LearningTaskFeedback`
   - 生成诊断
   - 持久化 `LearningTaskDiagnosis / LearningArchiveEntry`
   - 更新 `async_tasks`

示例伪代码：

```go
task := AsyncTask{Status: pending, TaskType: plan.feedback.diagnosis}
repo.Create(task)
publisher.PublishTask(...)
repo.Update(status=queued)
```

```go
asyncTask, shouldRun := repo.ClaimByID(taskID)
result := buildTaskFeedbackDiagnosis(...)
persistTaskFeedbackDiagnosis(...)
repo.Update(status=succeeded, result_json=...)
```

### 3. 爬虫导入任务

涉及文件：

- `backend/internal/service/scraper_service.go`
- `backend/internal/repository/scraper_repo.go`
- `backend/cmd/server/main.go`
- `backend/cmd/worker/main.go`

当前实现：

1. `CreateImportTask()` 仍然先写 `scraper_tasks`
2. 如果 RabbitMQ 可用，立即发布 `scraper.import.questions`
3. `RetryTask()` 在重置任务状态后也会重新发布 MQ 消息
4. worker 消费后调用 `ProcessImportTask()`

保留兼容：

- MQ 不可用时仍可由旧 `RunNextPendingTask()` 轮询执行

### 4. 后台题目流水线任务

涉及文件：

- `backend/internal/service/admin_question_pipeline_task.go`
- `backend/internal/repository/scraper_repo.go`
- `backend/cmd/server/main.go`
- `backend/cmd/worker/main.go`

当前实现：

1. `CreateQuestionPipelineTask()` 先写 `scraper_tasks`
2. RabbitMQ 发布 `admin.question.pipeline.build`
3. worker 消费后调用 `ProcessQuestionPipelineTask()`
4. 失败后进入重试交换机，再失败进入死信队列

### 5. Server 与 Worker 注入点

服务端：

- `backend/cmd/server/main.go`
  - 初始化 `AsyncTaskRepo`
  - 初始化 `mq.NewTaskPublisher(...)`
  - 通过 `service.AsyncDispatchOption` 注入 `PlanService / AdminService / ScraperService`

Worker：

- `backend/cmd/worker/main.go`
  - 默认优先启动 RabbitMQ 消费模式
  - MQ 启动失败时回退现有 DB 轮询模式
  - 已注册 6 个 handler：
    - 学习反馈诊断
    - 学习计划生成
    - 爬虫导入
    - 后台题目流水线
    - 简历解析
    - 面试报告生成

### 6. 面试核心同步链路异步化

涉及文件：

- `backend/internal/service/interview_service.go`
- `backend/internal/service/interview_async_support.go`
- `backend/internal/model/mock_interview.go`
- `backend/internal/repository/async_task_repo.go`
- `backend/cmd/server/main.go`
- `backend/cmd/worker/main.go`
- `backend/internal/ai/runtime/dynamic_client.go`

当前实现：

1. 实时简历驱动面试创建时，先创建 `mock_interviews` 记录并置为 `preparing`。
2. 服务端创建 `async_tasks`，发布 `interview.resume.parse` 到 `makejob.async.interview.resume.parse`。
3. 如果 MQ 不可用，则回退到本地同步解析简历，并把面试状态直接推进到 `ongoing`。
4. 面试结束时，文本面试链路先把状态更新为 `report_generating`，再发布 `interview.report.generate`。
5. worker 侧新增两个消费者：
   - `ProcessInterviewResumeParseTask()`
   - `ProcessInterviewReportTask()`
6. 报告消费者复用原同步逻辑，完成：
   - 作答补评分
   - `GenerateReport`
   - 编程题诊断补齐
   - 学习档案沉淀
   - 面试状态切换为 `completed`
7. `GetInterview()` 和 `GetReport()` 继续作为前端轮询入口，对外暴露：
   - `status`
   - `async_task_id`
   - `task_status`
   - `task_error`

新的面试状态约定：

- `preparing`：简历解析处理中，前端轮询 `GET /interviews/:id`
- `ongoing`：可正常开始/继续面试
- `report_generating`：报告处理中，前端轮询 `GET /interviews/:id/report`
- `completed`：报告已落库，可直接读取完整结果

示例伪代码：

```go
interview.Status = preparing
repo.Create(interview)
taskRepo.Create(asyncTask)
publisher.PublishTask("interview.resume.parse", message)
```

```go
interview.Status = report_generating
repo.Update(interview)
taskRepo.Create(asyncTask)
publisher.PublishTask("interview.report.generate", message)
```

```go
asyncTask, shouldRun := asyncTaskRepo.ClaimByID(taskID)
payload := decodeInterviewReportPayload(asyncTask.PayloadJSON)
report := generateAndPersistInterviewReport(...)
asyncTaskRepo.Update(status=succeeded, result_json=...)
```

注意事项：

1. 实时语音 WebSocket 逐轮问答链路仍然不进 MQ。
2. API 契约已改为“立即返回处理中”，不能再假设 `FinishInterview()` 同步返回完整报告。
3. 微服务拆分时，`resume-worker` 或 `report-worker` 只需要消费相同路由键，不需要变更消息格式。
4. 本轮沿用任务状态轮询，不要求前端直接查询 `async_tasks` 表。

### 7. 学习计划生成异步化

涉及文件：

- `backend/internal/service/plan_service.go`
- `backend/internal/service/plan_async_support.go`
- `backend/internal/model/learning_plan.go`
- `backend/internal/repository/plan_repo.go`
- `backend/internal/mq/message.go`
- `backend/cmd/worker/main.go`
- `frontend-react/apps/web/src/features/companion/CompanionHubPage.tsx`
- `frontend-react/apps/web/src/features/companion/CompanionWorkspacePage.tsx`

当前实现：

1. `GeneratePlan()` 优先走 RabbitMQ 异步路径，先创建 `learning_plans` 占位记录并置为 `generating`。
2. 服务端创建 `async_tasks`，发布 `plan.generate` 到 `makejob.async.plan.generate`。
3. 如果 MQ 不可用，则回退到本地同步生成，并继续复用原有持久化逻辑。
4. worker 侧新增 `ProcessPlanGenerateTask()` 消费者，复用 `generateAndPersistLearningPlan()` 完成 AI 生成和任务落库。
5. `GetCurrentPlan()` 和 `GetPlan()` 在计划状态为 `generating` 时，对外暴露：
   - `status`
   - `async_task_id`
   - `task_status`
   - `task_error`
6. 前端入口页和独立陪伴页统一按业务接口轮询，不直接访问 `async_tasks`：
   - 入口页轮询 `GET /plans/current`
   - 陪伴页轮询 `GET /plans/current`
   - 当 `status=generating` 且任务未 `failed/dead` 时每 3 秒刷新一次
7. 陪伴页在 `generating` 阶段禁用任务推进、计划调整和聊天输入，避免用户在任务尚未落库时进入空计划执行态。

新的学习计划状态约定：

- `generating`：计划生成处理中，前端轮询 `GET /plans/current`
- `active`：计划已生成，可继续推进任务
- `completed`：计划已全部完成

示例伪代码：

```go
plan.Status = generating
planRepo.Create(plan)
asyncTaskRepo.Create(asyncTask)
publisher.PublishTask("plan.generate", message)
```

```go
asyncTask, shouldRun := asyncTaskRepo.ClaimByID(taskID)
payload := decodePlanGeneratePayload(asyncTask.PayloadJSON)
plan, tasks := generateAndPersistLearningPlan(...)
asyncTaskRepo.Update(status=succeeded, result_json=...)
```

注意事项：

1. 计划生成占位记录与最终计划复用同一 `learning_plans.id`，前端只需要盯住一个业务实体。
2. 同步兜底路径仍会落正式计划，避免 RabbitMQ 短暂不可用时接口直接失败。
3. 微服务拆分后，可把当前消费者直接迁移到独立 `plan-worker` 或 `plan-service`，消息契约无需变更。

## Docker 环境搭建指南

已新增根目录 `docker-compose.yml`，使用镜像 `rabbitmq:3-management`。

启动：

```bash
docker compose up -d rabbitmq
```

停止：

```bash
docker compose stop rabbitmq
```

删除容器并保留数据卷：

```bash
docker compose down
```

管理台：

- 地址：`http://localhost:15672`
- 用户名：`makejob`
- 密码：`makejob123`
- vhost：`/makejob`

AMQP 连接地址：

```text
amqp://makejob:makejob123@localhost:5672/%2Fmakejob
```

## 微服务拆分后的演进方案

### 第一阶段：单体内分层消费者

当前已经完成：

- API 服务负责生产消息
- `worker` 负责消费消息
- 共享数据库与 RabbitMQ

### 第二阶段：按业务域拆 worker

建议拆分为：

1. `plan-worker`
   负责 `plan.feedback.diagnosis`、`plan.generate`
2. `content-worker`
   负责 `scraper.import.questions`、`admin.question.pipeline.build`
3. `interview-worker`
   负责 `interview.resume.parse`、`interview.report.generate`、未来 `interview.archive.persist`

此阶段只需要：

- 复用相同交换机 / 队列 / 路由键
- 迁移对应 handler 到新进程
- 调整服务发现与数据库连接配置

### 第三阶段：服务独立存储

当面试、学习计划、内容生产拆库时：

1. `TaskMessage` 保持不变
2. 用 `EntityType / EntityID / Payload` 跨服务传递定位信息
3. 各服务本地维护自己的任务审计表
4. 如需跨服务最终一致性，再补 Outbox / Inbox

## 本轮实施边界

### 已真正切换到 RabbitMQ 的部分

- 学习任务反馈诊断
- 学习计划生成
- 爬虫导入任务
- 后台题目流水线任务
- 简历驱动面试的简历解析
- 面试结束后的报告生成、编程诊断、学习档案沉淀
