# Interview + LearningArchive MQ Consumer 实现 Spec (P4-7~P4-8)

## Why
Interview 服务需要实现简历解析 MQ 消费者，LearningArchive 服务需要实现面试完成事件消费者，完成面试流程的异步处理闭环。

## What Changes
- P4-7: Interview Service - Resume Parse MQ Consumer（简历解析）
- P4-8: LearningArchive - MQ Consumer for interview.finished（面试完成事件归档）

## Impact
- Affected specs: P4-7, P4-8
- Affected code:
  - `app/interview/internal/server/mq.go` (修改)
  - `app/interview/internal/biz/interview.go` (修改)
  - `app/interview/internal/biz/usecase.go` (修改)
  - `app/learning_archive/internal/server/mq.go` (新建)
  - `app/learning_archive/internal/biz/archive.go` (修改)

## ADDED Requirements

### Requirement: Resume Parse MQ Consumer
系统 SHALL 异步解析简历并存储结果。

#### Scenario: 消费简历解析消息
- **WHEN** 收到 interview.resume.parse 消息
- **THEN** 调用 AI Gateway.ResumeParser，将解析结果存入 interview 记录

### Requirement: Interview Finished MQ Consumer
系统 SHALL 监听面试完成事件并写入学习档案。

#### Scenario: 消费面试完成事件
- **WHEN** 收到 interview.finished 消息
- **THEN** 提取薄弱点和优势，创建学习档案条目，发布 archive.written 事件

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
- MQ 发布使用 pkg/mq
