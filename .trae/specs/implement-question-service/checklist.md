# Checklist

## P3-1: RunCode RPC
- [x] conf/conf.go 包含 coderunner_addr 配置
- [x] biz/question.go 定义 CodeRunnerClient 接口
- [x] service/question.go 实现 RunCode handler
- [x] main.go 注入 CodeRunner gRPC 客户端
- [x] go build 编译通过

## P3-2: DeleteNote RPC
- [x] biz/question.go 定义 NoteRepo.GetByID/Delete 接口
- [x] biz/question.go 实现 DeleteNote UseCase
- [x] data/question_repo.go 实现 GetByID/Delete
- [x] service/question.go 实现 DeleteNote handler
- [x] go build 编译通过

## P3-3: GenerateTimedExam RPC
- [x] biz/question.go 定义 Exam 实体
- [x] biz/question.go 定义 ExamRepo 接口
- [x] biz/question.go 定义 QuestionRepo.RandomSelect 接口
- [x] biz/question.go 实现 GenerateTimedExam UseCase
- [x] data/question_repo.go 实现 RandomSelect
- [x] data/question_repo.go 实现 ExamRepo
- [x] service/question.go 实现 GenerateTimedExam handler
- [x] go build 编译通过

## P3-4: SubmitExam RPC
- [x] conf/conf.go 包含 ai_gateway_addr 配置
- [x] biz/question.go 定义 QuizAnalyzerClient 接口
- [x] biz/question.go 实现 SubmitExam UseCase
- [x] service/question.go 实现 SubmitExam handler
- [x] main.go 注入 AI Gateway gRPC 客户端
- [x] go build 编译通过

## P3-5: ListQuestionSets + GetQuestionSetDetail
- [x] biz/question.go 定义 QuestionSet 实体
- [x] biz/question.go 定义 QuestionSetRepo 接口
- [x] data/question_repo.go 实现 QuestionSetRepo
- [x] service/question.go 实现 ListQuestionSets handler
- [x] service/question.go 实现 GetQuestionSetDetail handler
- [x] go build 编译通过

## P3-6: ListMistakeTopics
- [x] biz/question.go 实现 ListMistakeTopics UseCase
- [x] data/question_repo.go 实现聚合查询
- [x] service/question.go 实现 ListMistakeTopics handler
- [x] go build 编译通过

## P3-7: MQ Consumers
- [x] server/mq.go 实现 pipeline.build handler
- [x] server/mq.go 实现 scraper.import handler
- [x] main.go 注册 MQ consumers
- [x] go build 编译通过

## P3-8: Enhance GetPracticeRecommendations
- [x] biz/question.go 增强推荐算法
- [x] service/question.go 添加 interview_id 参数
- [x] go build 编译通过

## 通用
- [x] go vet 通过
