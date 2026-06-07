# Tasks

## P5-2: Plan Service - GetPlan, GetCurrentPlan, ListPlans

- [x] Task 1: 修改 biz/plan.go - 添加 TaskRepo 扩展方法
  - [x] SubTask 1.1: 添加 GetByID 方法
  - [x] SubTask 1.2: 添加 CountByPlanID 方法
  - [x] SubTask 1.3: 添加 Update 方法
- [x] Task 2: 修改 biz/plan.go - 实现查询 UseCase
  - [x] SubTask 2.1: 实现 GetPlanWithTasks（加载任务列表）
  - [x] SubTask 2.2: 实现 GetCurrentPlanWithTasks
  - [x] SubTask 2.3: 实现 ListPlansWithProgress
- [x] Task 3: 修改 service/plan.go - 实现查询 handler
  - [x] SubTask 3.1: 实现 GetPlan handler（含任务列表）
  - [x] SubTask 3.2: 实现 GetCurrentPlan handler
  - [x] SubTask 3.3: 实现 ListPlans handler

## P5-3: Plan Service - UpdateTaskStatus

- [x] Task 4: 修改 biz/plan.go - 实现 UpdateTaskStatus UseCase
  - [x] SubTask 4.1: 验证 plan 和 task 归属
  - [x] SubTask 4.2: 状态机校验
  - [x] SubTask 4.3: 更新任务状态
  - [x] SubTask 4.4: 同步计划进度
- [x] Task 5: 修改 service/plan.go - 实现 UpdateTaskStatus handler

## P5-4: Plan Service - SubmitTaskFeedback + Diagnosis Consumer

- [x] Task 6: 修改 biz/plan.go - 添加 TaskFeedback 实体和 Repo 接口
- [x] Task 7: 修改 biz/plan.go - 实现 SubmitTaskFeedback UseCase
- [x] Task 8: 修改 service/plan.go - 实现 SubmitTaskFeedback handler
- [x] Task 9: 修改 server/mq.go - 实现 DiagnosisConsumer

# Task Dependencies
- Task 1-3 (P5-2) 无依赖
- Task 4-5 (P5-3) 依赖 Task 1
- Task 6-9 (P5-4) 依赖 Task 1
