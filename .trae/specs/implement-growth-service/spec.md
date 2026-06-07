# Growth Service - GetGrowthSummary + GetWeeklyFocus + SyncStudyLog 实现 Spec (P5-6~P5-8)

## Why
Growth 服务需要重写成长总览和周聚焦推荐，通过并发 gRPC 调用聚合多服务数据，并修复学习日志同步。

## What Changes
- P5-6: 重写 GetGrowthSummary（并发 gRPC 调用聚合多服务数据）
- P5-7: 重写 GetWeeklyFocus（结合学习档案和题目集匹配）
- P5-8: 修复 SyncStudyLog（合并单体和微服务两套字段）

## Impact
- Affected specs: P5-6, P5-7, P5-8
- Affected code:
  - `app/growth/internal/biz/growth.go` (重写)
  - `app/growth/internal/service/growth.go` (修改)
  - `app/growth/internal/conf/conf.go` (修改)

## ADDED Requirements

### Requirement: GetGrowthSummary RPC
系统 SHALL 通过并发 gRPC 调用聚合多服务数据。

#### Scenario: 正常情况
- **WHEN** 调用 GetGrowthSummary()
- **THEN** 返回所有维度数据（练习统计、计划进度、面试统计、薄弱点、成就）

#### Scenario: 单个服务不可用
- **WHEN** 某个下游服务调用失败
- **THEN** 该部分使用零值，整体不失败

### Requirement: GetWeeklyFocus RPC
系统 SHALL 根据学习档案推导本周聚焦方向。

#### Scenario: 正常情况
- **WHEN** 调用 GetWeeklyFocus()
- **THEN** 返回聚焦主题、薄弱点、推荐题目集

### Requirement: SyncStudyLog RPC
系统 SHALL 同步学习日志，支持新旧字段合并。

#### Scenario: 正常情况
- **WHEN** 调用 SyncStudyLog(date_key, plan_id, summary, action, ref_id, ref_type, duration, source)
- **THEN** 创建或更新学习日志记录

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
- 并发调用使用 errgroup
