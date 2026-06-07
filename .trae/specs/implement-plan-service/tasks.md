# Tasks

## P5-1: Plan Service - CreatePlan + MQ Consumer

- [x] Task 1: 修改 biz/plan.go - 扩展实体和接口
  - [x] SubTask 1.1: 添加 LearningPlan 实体（完整字段）
  - [x] SubTask 1.2: 添加 LearningTask 实体
  - [x] SubTask 1.3: 添加 PlanRepo 扩展方法
  - [x] SubTask 1.4: 添加 TaskRepo 接口
  - [x] SubTask 1.5: 添加 PlanAgentClient 接口
  - [x] SubTask 1.6: 添加 MQPublisher 接口
  - [x] SubTask 1.7: 实现 CreatePlan UseCase（参数校验 + 创建骨架 + 发布 MQ）
  - [x] SubTask 1.8: 实现 GeneratePlan UseCase（AI 生成 + 创建任务）

- [x] Task 2: 修改 data/plan_repo.go - 实现 GORM
  - [x] SubTask 2.1: 实现 PlanRepo 所有方法
  - [x] SubTask 2.2: 实现 TaskRepo 所有方法

- [x] Task 3: 修改 service/plan.go - 实现 gRPC handler
  - [x] SubTask 3.1: 实现 CreatePlan handler
  - [x] SubTask 3.2: 实现其他 handler（GetPlan, GetCurrentPlan 等）

- [x] Task 4: 新建 server/mq.go - 实现 MQ Consumer
  - [x] SubTask 4.1: 实现 PlanGenerateHandler
  - [x] SubTask 4.2: 注册 consumer

- [x] Task 5: 修改 main.go - 注入新依赖
- [x] Task 6: 更新 configs/config.yaml

# Task Dependencies
- Task 1 无依赖
- Task 2 依赖 Task 1
- Task 3 依赖 Task 1
- Task 4 依赖 Task 1
- Task 5 依赖 Task 1-4
- Task 6 依赖 Task 1
