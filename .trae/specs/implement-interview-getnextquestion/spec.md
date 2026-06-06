# Interview Service - GetNextQuestion 实现 Spec (P4-1)

## Why
Interview 服务需要实现 GetNextQuestion RPC，根据面试上下文动态生成下一道面试题。这是面试流程的核心功能，需要结合 RAG 检索和 AI 生成。

## What Changes
- P4-1: 实现 GetNextQuestion RPC
  - 加载面试上下文（历史消息、简历、JD）
  - 调用 RAG 获取相关知识
  - 调用 AI Gateway 生成下一题
  - 保存消息记录
  - 返回 InterviewQuestion

## Impact
- Affected specs: P4-1
- Affected code:
  - `app/interview/internal/biz/interview.go` (修改)
  - `app/interview/internal/service/interview.go` (修改)
  - `app/interview/internal/conf/conf.go` (修改，添加 RAG 地址)

## ADDED Requirements

### Requirement: GetNextQuestion RPC
系统 SHALL 根据面试上下文动态生成下一道面试题。

#### Scenario: 首次调用（user_answer 为空）
- **WHEN** 调用 GetNextQuestion(interview_id, user_answer="")
- **THEN** 返回第一题

#### Scenario: 带 user_answer 调用
- **WHEN** 调用 GetNextQuestion(interview_id, user_answer="...")
- **THEN** 保存答案并返回下一题

#### Scenario: 面试已结束
- **WHEN** 调用 GetNextQuestion(interview_id, ...) 且面试 status != "ongoing"
- **THEN** 返回 INTERVIEW_FINISHED 错误

#### Scenario: RAG 不可用
- **WHEN** RAG 服务调用失败
- **THEN** 降级处理，仍能生成题目

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
