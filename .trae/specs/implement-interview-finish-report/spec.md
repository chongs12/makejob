# Interview Service - FinishInterview + Report 实现 Spec (P4-2~P4-4)

## Why
Interview 服务需要实现面试结束、报告生成和报告查询功能，完成面试流程闭环。

## What Changes
- P4-2: 实现 FinishInterview RPC（结束面试，触发异步报告生成）
- P4-3: 实现 Report Generation MQ Consumer（异步生成面试报告）
- P4-4: 实现 GetReport RPC（查询面试报告）

## Impact
- Affected specs: P4-2, P4-3, P4-4
- Affected code:
  - `app/interview/internal/biz/interview.go` (修改)
  - `app/interview/internal/biz/usecase.go` (修改)
  - `app/interview/internal/service/interview.go` (修改)
  - `app/interview/internal/server/mq.go` (新建)
  - `app/interview/internal/data/interview_repo.go` (修改)

## ADDED Requirements

### Requirement: FinishInterview RPC
系统 SHALL 结束面试并触发异步报告生成。

#### Scenario: 成功结束面试
- **WHEN** 调用 FinishInterview(interview_id)
- **THEN** 面试状态变为 report_generating，MQ 消息被发布

#### Scenario: 面试已结束
- **WHEN** 调用 FinishInterview(interview_id) 且面试 status != "ongoing"
- **THEN** 返回 INTERVIEW_FINISHED 错误

### Requirement: Report Generation MQ Consumer
系统 SHALL 异步生成面试报告。

#### Scenario: 消费报告生成消息
- **WHEN** 收到 interview.report.generate 消息
- **THEN** 调用 AI 生成报告，保存到数据库，发布 interview.finished 事件

### Requirement: GetReport RPC
系统 SHALL 查询面试报告。

#### Scenario: 报告正在生成
- **WHEN** 调用 GetReport(interview_id) 且 status == "report_generating"
- **THEN** 返回 status="generating"

#### Scenario: 报告已完成
- **WHEN** 调用 GetReport(interview_id) 且 status == "completed"
- **THEN** 返回完整报告数据

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
- MQ 发布使用 pkg/mq
