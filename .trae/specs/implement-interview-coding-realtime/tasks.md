# Tasks

## P4-5: Interview Service - Implement SubmitCodingAnswer

- [x] Task 1: 修改 biz/interview.go - 添加 CodeRunnerClient 接口
- [x] Task 2: 修改 biz/usecase.go - 实现 SubmitCodingAnswer UseCase
  - [x] SubTask 2.1: 验证 interview status == "ongoing"
  - [x] SubTask 2.2: 获取题目信息
  - [x] SubTask 2.3: 调用 CodeRunner.Execute（降级处理）
  - [x] SubTask 2.4: 调用 AI Gateway.QuizAnalyzer（降级处理）
  - [x] SubTask 2.5: 保存 interview_coding_attempts 记录
  - [x] SubTask 2.6: 返回综合结果
- [x] Task 3: 修改 service/interview.go - 添加 SubmitCodingAnswer handler

## P4-6: Interview Service - Implement Realtime RPCs (5)

- [x] Task 4: 修改 biz/usecase.go - 实现 IsRealtimeInterview
- [x] Task 5: 修改 biz/usecase.go - 实现 GetRealtimeContext
- [x] Task 6: 修改 biz/usecase.go - 实现 BindRealtimeDialog
- [x] Task 7: 修改 biz/usecase.go - 实现 AppendRealtimeUserAnswer
- [x] Task 8: 修改 biz/usecase.go - 实现 AppendRealtimeAssistantReply
- [x] Task 9: 修改 service/interview.go - 添加 5 个 Realtime handler

# Task Dependencies
- Task 1-3 (P4-5) 无依赖
- Task 4-9 (P4-6) 无依赖
