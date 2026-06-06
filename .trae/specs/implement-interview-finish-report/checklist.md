# Checklist

## P4-2: FinishInterview RPC
- [x] biz/interview.go 定义 InterviewReport 实体
- [x] biz/interview.go 定义 ReportRepo 接口
- [x] biz/usecase.go 实现 FinishInterview UseCase
- [x] FinishInterview 验证 interview 属于当前用户
- [x] FinishInterview 验证 status == "ongoing"
- [x] FinishInterview 更新 status = "report_generating"
- [x] FinishInterview 设置 finished_at
- [x] FinishInterview 发布 MQ 消息
- [x] service/interview.go 实现 FinishInterview handler
- [x] go build 编译通过

## P4-3: Report Generation MQ Consumer
- [x] server/mq.go 实现 ReportGenerationHandler
- [x] ReportGenerationHandler 加载 interview + 所有消息
- [x] ReportGenerationHandler 调用 AI Gateway.InterviewAgent
- [x] ReportGenerationHandler 保存 interview_reports 记录
- [x] ReportGenerationHandler 更新 interview.status = "completed"
- [x] ReportGenerationHandler 发布 interview.finished 事件
- [x] main.go 注册 MQ consumer
- [x] go build 编译通过

## P4-4: GetReport RPC
- [x] biz/usecase.go 实现 GetReport UseCase
- [x] GetReport 验证 interview 属于当前用户
- [x] GetReport 根据 status 返回不同响应
- [x] GetReport 加载 interview_reports
- [x] GetReport 解析 JSON 字段
- [x] service/interview.go 实现 GetReport handler
- [x] go build 编译通过

## 通用
- [x] go vet 通过
