# Checklist

## P5-1: Plan Service - CreatePlan + MQ Consumer

### biz 层
- [x] biz/plan.go 定义 LearningPlan 实体（完整字段）
- [x] biz/plan.go 定义 LearningTask 实体
- [x] biz/plan.go 定义 PlanRepo 扩展方法
- [x] biz/plan.go 定义 TaskRepo 接口
- [x] biz/plan.go 定义 PlanAgentClient 接口
- [x] biz/plan.go 定义 MQPublisher 接口
- [x] biz/plan.go 实现 CreatePlan UseCase（参数校验）
- [x] biz/plan.go 实现 GeneratePlan UseCase（AI 生成）

### data 层
- [x] data/plan_repo.go 实现 PlanRepo 所有方法
- [x] data/plan_repo.go 实现 TaskRepo 所有方法

### service 层
- [x] service/plan.go 实现 CreatePlan handler
- [x] service/plan.go 实现 GetPlan handler
- [x] service/plan.go 实现 GetCurrentPlan handler
- [x] service/plan.go 实现 ListPlans handler

### server 层
- [x] server/mq.go 实现 PlanGenerateHandler
- [x] server/mq.go 注册 consumer

### 启动入口
- [x] main.go 注入新依赖
- [x] configs/config.yaml 包含配置
- [x] go build 编译通过
- [x] go vet 通过
