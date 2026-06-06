# Interview Service - SubmitCodingAnswer + Realtime RPCs 实现 Spec (P4-5~P4-6)

## Why
Interview 服务需要实现编程题提交和实时面试相关的 5 个 RPC，完成面试流程的编程题和实时语音场景支持。

## What Changes
- P4-5: 实现 SubmitCodingAnswer RPC（编程题代码提交、执行和 AI 评审）
- P4-6: 实现 5 个 Realtime RPC（IsRealtimeInterview, GetRealtimeContext, BindRealtimeDialog, AppendRealtimeUserAnswer, AppendRealtimeAssistantReply）

## Impact
- Affected specs: P4-5, P4-6
- Affected code:
  - `app/interview/internal/biz/interview.go` (修改)
  - `app/interview/internal/biz/usecase.go` (修改)
  - `app/interview/internal/service/interview.go` (修改)

## ADDED Requirements

### Requirement: SubmitCodingAnswer RPC
系统 SHALL 执行编程题代码并进行 AI 评审。

#### Scenario: 成功提交编程题
- **WHEN** 调用 SubmitCodingAnswer(interview_id, question_index, language, code)
- **THEN** 调用 CodeRunner 执行代码，调用 AI 评审，返回综合结果

#### Scenario: CodeRunner 不可用
- **WHEN** CodeRunner 服务调用失败
- **THEN** 返回 execution_success=false，仍返回 AI 评审结果

### Requirement: Realtime RPCs
系统 SHALL 提供实时面试场景所需的数据读写接口。

#### Scenario: IsRealtimeInterview
- **WHEN** 调用 IsRealtimeInterview(interview_id)
- **THEN** 返回该面试是否为实时模式

#### Scenario: GetRealtimeContext
- **WHEN** 调用 GetRealtimeContext(interview_id)
- **THEN** 返回面试上下文（最近 10 条消息）

#### Scenario: BindRealtimeDialog
- **WHEN** 调用 BindRealtimeDialog(interview_id, dialog_id)
- **THEN** 绑定实时会话 ID

#### Scenario: AppendRealtimeUserAnswer
- **WHEN** 调用 AppendRealtimeUserAnswer(interview_id, content)
- **THEN** 保存用户回答消息

#### Scenario: AppendRealtimeAssistantReply
- **WHEN** 调用 AppendRealtimeAssistantReply(interview_id, content)
- **THEN** 保存 AI 回复消息，递增 question_index

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
