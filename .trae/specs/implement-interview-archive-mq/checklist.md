# Checklist

## P4-7: Interview Service - Resume Parse MQ Consumer
- [x] biz/interview.go 添加 ResumeParsedResult 实体
- [x] biz/usecase.go 实现 ParseResume UseCase
- [x] ParseResume 验证 interview 存在
- [x] ParseResume 验证 resume_text 非空
- [x] ParseResume 调用 AI Gateway.ResumeParser
- [x] ParseResume 将结果序列化为 JSON
- [x] ParseResume UPDATE interviews SET resume_parsed_json
- [x] server/mq.go 注册 ResumeParseHandler
- [x] go build 编译通过

## P4-8: LearningArchive - MQ Consumer
- [x] biz/archive.go 添加 MQPublisher 接口
- [x] biz/archive.go 实现 HandleInterviewFinished UseCase
- [x] HandleInterviewFinished 对 weak_topic 创建/更新学习档案条目
- [x] HandleInterviewFinished 对 strength_topic 创建学习档案条目
- [x] HandleInterviewFinished 发布 archive.written 事件
- [x] server/mq.go 实现 InterviewFinishedHandler
- [x] main.go 注册 MQ consumer
- [x] go build 编译通过

## 通用
- [x] go vet 通过
