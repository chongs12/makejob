# Tasks

## P3-1: Question Service - Implement RunCode
- [x] Task 1: 修改 conf/conf.go - 添加 coderunner_addr 配置
- [x] Task 2: 修改 biz/question.go - 添加 CodeRunner 客户端接口
- [x] Task 3: 修改 service/question.go - 实现 RunCode handler（调用 CodeRunner）
- [x] Task 4: 修改 main.go - 注入 CodeRunner gRPC 客户端

## P3-2: Question Service - Implement DeleteNote
- [x] Task 5: 修改 biz/question.go - 添加 NoteRepo.GetByID/Delete 方法
- [x] Task 6: 修改 biz/question.go - 实现 DeleteNote UseCase
- [x] Task 7: 修改 data/question_repo.go - 实现 GetByID/Delete
- [x] Task 8: 修改 service/question.go - 添加 DeleteNote handler

## P3-3: Question Service - Implement GenerateTimedExam
- [x] Task 9: 修改 biz/question.go - 添加 Exam 实体和 ExamRepo 接口
- [x] Task 10: 修改 biz/question.go - 添加 QuestionRepo.RandomSelect 方法
- [x] Task 11: 修改 biz/question.go - 实现 GenerateTimedExam UseCase
- [x] Task 12: 修改 data/question_repo.go - 实现 RandomSelect 和 ExamRepo
- [x] Task 13: 修改 service/question.go - 添加 GenerateTimedExam handler

## P3-4: Question Service - Implement SubmitExam
- [x] Task 14: 修改 conf/conf.go - 添加 ai_gateway_addr 配置
- [x] Task 15: 修改 biz/question.go - 添加 QuizAnalyzerClient 接口
- [x] Task 16: 修改 biz/question.go - 实现 SubmitExam UseCase
- [x] Task 17: 修改 service/question.go - 添加 SubmitExam handler
- [x] Task 18: 修改 main.go - 注入 AI Gateway gRPC 客户端

## P3-5: Question Service - Implement ListQuestionSets + GetQuestionSetDetail
- [x] Task 19: 修改 biz/question.go - 添加 QuestionSet 实体和 QuestionSetRepo 接口
- [x] Task 20: 修改 data/question_repo.go - 实现 QuestionSetRepo
- [x] Task 21: 修改 service/question.go - 添加 ListQuestionSets/GetQuestionSetDetail handler

## P3-6: Question Service - Implement ListMistakeTopics
- [x] Task 22: 修改 biz/question.go - 实现 ListMistakeTopics UseCase
- [x] Task 23: 修改 data/question_repo.go - 实现聚合查询
- [x] Task 24: 修改 service/question.go - 添加 ListMistakeTopics handler

## P3-7: Question Service - MQ Consumers
- [x] Task 25: 新建 server/mq.go - 实现 pipeline.build handler
- [x] Task 26: 新建 server/mq.go - 实现 scraper.import handler
- [x] Task 27: 修改 main.go - 注册 MQ consumers

## P3-8: Question Service - Enhance GetPracticeRecommendations
- [x] Task 28: 修改 biz/question.go - 增强推荐算法（面试驱动加权）
- [x] Task 29: 修改 service/question.go - 添加 interview_id 参数

# Task Dependencies
- Task 1-4 依赖 CodeRunner 服务
- Task 5-8 无依赖
- Task 9-13 无依赖
- Task 14-18 依赖 AI Gateway 服务
- Task 19-21 无依赖
- Task 22-24 无依赖
- Task 25-27 依赖 AI Gateway 服务
- Task 28-29 依赖 Interview 服务（可选）
