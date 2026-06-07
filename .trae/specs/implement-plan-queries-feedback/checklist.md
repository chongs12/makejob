# Checklist

## P5-2: GetPlan, GetCurrentPlan, ListPlans
- [x] biz/plan.go 添加 TaskRepo 扩展方法
- [x] biz/plan.go 实现 GetPlanWithTasks UseCase
- [x] biz/plan.go 实现 GetCurrentPlanWithTasks UseCase
- [x] biz/plan.go 实现 ListPlansWithProgress UseCase
- [x] service/plan.go 实现 GetPlan handler（含任务列表）
- [x] service/plan.go 实现 GetCurrentPlan handler
- [x] service/plan.go 实现 ListPlans handler
- [x] go build 编译通过

## P5-3: UpdateTaskStatus
- [x] biz/plan.go 实现 UpdateTaskStatus UseCase
- [x] UpdateTaskStatus 验证 plan 和 task 归属
- [x] UpdateTaskStatus 状态机校验
- [x] UpdateTaskStatus 更新任务状态
- [x] UpdateTaskStatus 同步计划进度
- [x] service/plan.go 实现 UpdateTaskStatus handler
- [x] go build 编译通过

## P5-4: SubmitTaskFeedback + Diagnosis Consumer
- [x] biz/plan.go 添加 TaskFeedback 实体
- [x] biz/plan.go 添加 TaskFeedbackRepo 接口
- [x] biz/plan.go 实现 SubmitTaskFeedback UseCase
- [x] service/plan.go 实现 SubmitTaskFeedback handler
- [x] server/mq.go 实现 DiagnosisConsumer
- [x] go build 编译通过

## 通用
- [x] go vet 通过
