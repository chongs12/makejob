# Checklist

## P4-5: SubmitCodingAnswer RPC
- [x] biz/interview.go 定义 CodeRunnerClient 接口
- [x] biz/usecase.go 实现 SubmitCodingAnswer UseCase
- [x] SubmitCodingAnswer 验证 interview status == "ongoing"
- [x] SubmitCodingAnswer 调用 CodeRunner.Execute（降级处理）
- [x] SubmitCodingAnswer 调用 AI Gateway.QuizAnalyzer（降级处理）
- [x] SubmitCodingAnswer 保存 interview_coding_attempts 记录
- [x] service/interview.go 实现 SubmitCodingAnswer handler
- [x] go build 编译通过

## P4-6: Realtime RPCs
- [x] biz/usecase.go 实现 IsRealtimeInterview
- [x] biz/usecase.go 实现 GetRealtimeContext
- [x] biz/usecase.go 实现 BindRealtimeDialog
- [x] biz/usecase.go 实现 AppendRealtimeUserAnswer
- [x] biz/usecase.go 实现 AppendRealtimeAssistantReply
- [x] service/interview.go 实现 5 个 Realtime handler
- [x] go build 编译通过

## 通用
- [x] go vet 通过
