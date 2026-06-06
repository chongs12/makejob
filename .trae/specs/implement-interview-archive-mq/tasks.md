# Tasks

## P4-7: Interview Service - Resume Parse MQ Consumer

- [x] Task 1: 修改 biz/interview.go - 添加 ResumeParsedResult 实体
- [x] Task 2: 修改 biz/usecase.go - 实现 ParseResume UseCase
  - [x] SubTask 2.1: 验证 interview 存在
  - [x] SubTask 2.2: 验证 resume_text 非空
  - [x] SubTask 2.3: 调用 AI Gateway.ResumeParser
  - [x] SubTask 2.4: 将结果序列化为 JSON
  - [x] SubTask 2.5: UPDATE interviews SET resume_parsed_json
- [x] Task 3: 修改 server/mq.go - 注册 ResumeParseHandler

## P4-8: LearningArchive - MQ Consumer for interview.finished

- [x] Task 4: 修改 biz/archive.go - 添加 MQPublisher 接口
- [x] Task 5: 修改 biz/archive.go - 实现 HandleInterviewFinished UseCase
  - [x] SubTask 5.1: 对每个 weak_topic 创建/更新学习档案条目
  - [x] SubTask 5.2: 对每个 strength_topic 创建学习档案条目
  - [x] SubTask 5.3: 发布 archive.written 事件
- [x] Task 6: 新建 server/mq.go - 实现 InterviewFinishedHandler
- [x] Task 7: 修改 main.go - 注册 MQ consumer

# Task Dependencies
- Task 1-3 (P4-7) 无依赖
- Task 4-7 (P4-8) 无依赖
