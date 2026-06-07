# Plan Service - Queries + UpdateTaskStatus + Feedback 实现 Spec (P5-2~P5-4)

## Why
Plan 服务需要实现计划查询、任务状态更新和反馈提交功能，完善学习计划的完整生命周期。

## What Changes
- P5-2: 实现 GetPlan, GetCurrentPlan, ListPlans 三个查询 RPC
- P5-3: 实现 UpdateTaskStatus RPC（带状态机校验和计划进度同步）
- P5-4: 实现 SubmitTaskFeedback RPC + Diagnosis MQ Consumer

## Impact
- Affected specs: P5-2, P5-3, P5-4
- Affected code:
  - `app/plan/internal/biz/plan.go` (修改)
  - `app/plan/internal/data/plan_repo.go` (修改)
  - `app/plan/internal/service/plan.go` (修改)
  - `app/plan/internal/server/mq.go` (修改)

## ADDED Requirements

### Requirement: GetPlan RPC
系统 SHALL 查询单个计划详情（含任务列表）。

#### Scenario: 成功查询
- **WHEN** 调用 GetPlan(plan_id)
- **THEN** 返回计划详情和任务列表

### Requirement: GetCurrentPlan RPC
系统 SHALL 获取当前活跃计划。

#### Scenario: 有活跃计划
- **WHEN** 调用 GetCurrentPlan()
- **THEN** 返回最新的 active 计划

### Requirement: ListPlans RPC
系统 SHALL 分页列出所有计划。

### Requirement: UpdateTaskStatus RPC
系统 SHALL 更新任务状态并同步计划进度。

#### Scenario: 合法状态转换
- **WHEN** pending → completed
- **THEN** 更新任务状态，同步计划进度

#### Scenario: 非法状态转换
- **WHEN** completed → pending
- **THEN** 返回 BadRequest 错误

### Requirement: SubmitTaskFeedback RPC
系统 SHALL 提交任务反馈并触发 AI 诊断。

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
