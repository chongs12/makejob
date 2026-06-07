# Plan Service - CreatePlan + MQ Consumer 实现 Spec (P5-1)

## Why
Plan 服务需要实现学习计划创建和异步生成功能，这是学习体系闭环的核心服务。当前只有空壳骨架，需要完整实现。

## What Changes
- P5-1: 实现 CreatePlan RPC + MQ Consumer
  - CreatePlan 同步创建计划骨架（status=generating）
  - 发布 MQ 消息触发异步生成
  - MQ Consumer 调用 AI 生成计划详情
  - 创建任务列表
  - 更新状态为 active

## Impact
- Affected specs: P5-1
- Affected code:
  - `app/plan/internal/biz/plan.go` (修改)
  - `app/plan/internal/data/plan_repo.go` (修改)
  - `app/plan/internal/service/plan.go` (修改)
  - `app/plan/internal/server/mq.go` (新建)

## ADDED Requirements

### Requirement: CreatePlan RPC
系统 SHALL 创建学习计划并触发异步生成。

#### Scenario: 成功创建计划
- **WHEN** 调用 CreatePlan(weak_topics, level, duration_days, industry, daily_study_minutes, goal_description)
- **THEN** 创建计划骨架（status=generating），发布 MQ 消息，返回 plan_id 和 status

#### Scenario: 参数校验失败
- **WHEN** 参数不合法（level 非法、duration_days 超出范围等）
- **THEN** 返回 BadRequest 错误

### Requirement: Plan Generate MQ Consumer
系统 SHALL 异步生成学习计划详情。

#### Scenario: 消费计划生成消息
- **WHEN** 收到 plan.generate 消息
- **THEN** 调用 AI Gateway.PlanAgent 生成计划，创建任务列表，更新状态为 active

#### Scenario: AI 调用失败
- **WHEN** AI Gateway 调用失败
- **THEN** 更新计划状态为 failed

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
- MQ 发布使用 pkg/mq
