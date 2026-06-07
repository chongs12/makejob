# Checklist

## P5-5: AdjustPlan RPC

### biz 层
- [x] biz/plan.go 添加 PlanAdjustment 实体
- [x] biz/plan.go 添加 PlanAdjustmentRepo 接口
- [x] biz/plan.go 添加 TaskFeedbackRepo.ListByPlanID 方法
- [x] biz/plan.go 添加 TaskRepo.BatchDelete 方法
- [x] biz/plan.go 添加 TaskRepo.BatchUpdateSortOrder 方法
- [x] biz/plan.go 实现 AdjustPlan UseCase
- [x] AdjustPlan 验证计划归属和状态
- [x] AdjustPlan 加载计划 + 任务 + 反馈
- [x] AdjustPlan 调用 AI Gateway.PlanAgent(mode=adjust)
- [x] AdjustPlan 应用调整（增删改任务）
- [x] AdjustPlan 记录调整历史

### data 层
- [x] data/plan_repo.go 实现 PlanAdjustmentRepo
- [x] data/plan_repo.go 实现 TaskFeedbackRepo.ListByPlanID
- [x] data/plan_repo.go 实现 TaskRepo.BatchDelete
- [x] data/plan_repo.go 实现 TaskRepo.BatchUpdateSortOrder

### service 层
- [x] service/plan.go 实现 AdjustPlan handler

### 通用
- [x] go build 编译通过
- [x] go vet 通过
