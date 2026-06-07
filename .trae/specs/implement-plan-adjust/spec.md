# Plan Service - AdjustPlan 实现 Spec (P5-5)

## Why
Plan 服务需要实现 AI 驱动的计划调整功能，根据用户反馈和诊断结果动态调整学习计划。

## What Changes
- P5-5: 实现 AdjustPlan RPC
  - 加载计划 + 所有反馈 + 诊断结果
  - 调用 AI Gateway.PlanAgent(mode=adjust)
  - 应用调整（增删改任务）
  - 记录调整历史

## Impact
- Affected specs: P5-5
- Affected code:
  - `app/plan/internal/biz/plan.go` (修改)
  - `app/plan/internal/data/plan_repo.go` (修改)
  - `app/plan/internal/service/plan.go` (修改)

## ADDED Requirements

### Requirement: AdjustPlan RPC
系统 SHALL 根据用户反馈和诊断结果，调用 AI 调整学习计划。

#### Scenario: 成功调整计划
- **WHEN** 调用 AdjustPlan(plan_id, reason)
- **THEN** 调用 AI 生成调整方案，应用增删改任务，返回调整统计

#### Scenario: 计划已完成
- **WHEN** 调用 AdjustPlan(plan_id) 且计划 status == "completed"
- **THEN** 返回 BadRequest 错误

#### Scenario: AI 调用失败
- **WHEN** AI Gateway 调用失败
- **THEN** 返回 502 错误

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
- 多表操作必须使用事务
