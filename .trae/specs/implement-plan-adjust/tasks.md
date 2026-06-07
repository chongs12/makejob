# Tasks

## P5-5: Plan Service - AdjustPlan

- [x] Task 1: 修改 biz/plan.go - 添加 PlanAdjustment 实体和 Repo 接口
  - [x] SubTask 1.1: 添加 PlanAdjustment 实体
  - [x] SubTask 1.2: 添加 PlanAdjustmentRepo 接口
  - [x] SubTask 1.3: 添加 TaskFeedbackRepo.ListByPlanID 方法
  - [x] SubTask 1.4: 添加 TaskRepo.BatchDelete 方法
  - [x] SubTask 1.5: 添加 TaskRepo.BatchUpdateSortOrder 方法

- [x] Task 2: 修改 biz/plan.go - 实现 AdjustPlan UseCase
  - [x] SubTask 2.1: 验证计划归属和状态
  - [x] SubTask 2.2: 加载计划 + 任务 + 反馈
  - [x] SubTask 2.3: 调用 AI Gateway.PlanAgent(mode=adjust)
  - [x] SubTask 2.4: 应用调整（增删改任务）
  - [x] SubTask 2.5: 记录调整历史

- [x] Task 3: 修改 data/plan_repo.go - 实现新方法
- [x] Task 4: 修改 service/plan.go - 实现 AdjustPlan handler

# Task Dependencies
- Task 1 无依赖
- Task 2 依赖 Task 1
- Task 3 依赖 Task 1
- Task 4 依赖 Task 2
