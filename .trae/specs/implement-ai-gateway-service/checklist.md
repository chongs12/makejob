# Checklist

## P1-5: InterviewAgent RPC
- [x] conf.go 包含 ARK 配置字段
- [x] biz/errors.go 定义四个领域错误
- [x] biz/ai.go 定义 AIConfig/PromptTemplate/AICallLog 实体
- [x] biz/ai.go 定义 AIConfigRepo/PromptRepo/CallLogRepo 接口
- [x] biz/runtime_builder.go 定义 LLMClient 接口
- [x] biz/ai.go 实现 InterviewAgentUseCase.GenerateQuestion 方法
- [x] data/ai_config_repo.go 实现 GetActiveConfig 方法
- [x] data/prompt_repo.go 实现 GetActiveTemplate 方法
- [x] data/call_log_repo.go 实现 Create 方法
- [x] data/data.go 实现 GORM 连接和 AutoMigrate
- [x] service/ai.go 实现 InterviewAgent handler
- [x] go build 编译通过

## P1-6: PlanAgent RPC
- [x] biz/ai.go 实现 PlanAgentUseCase.GeneratePlan 方法
- [x] service/ai.go 实现 PlanAgent handler
- [x] go build 编译通过

## P1-7: CompanionAgent RPC
- [x] biz/ai.go 实现 CompanionAgentUseCase.Chat 方法
- [x] service/ai.go 实现 CompanionAgent handler
- [x] go build 编译通过

## P1-8: QuizAnalyzer RPC
- [x] biz/ai.go 实现 QuizAnalyzerUseCase.Analyze 方法
- [x] service/ai.go 实现 QuizAnalyzer handler
- [x] go build 编译通过

## P1-9: ResumeParser RPC
- [x] biz/ai.go 实现 ResumeParserUseCase.Parse 方法
- [x] service/ai.go 实现 ResumeParser handler
- [x] go build 编译通过

## P1-10: Live2DDirector RPC
- [x] biz/ai.go 实现 Live2DDirectorUseCase.GenerateDirective 方法
- [x] service/ai.go 实现 Live2DDirector handler
- [x] go build 编译通过

## 通用
- [x] server/grpc.go 正确注册服务
- [x] main.go 正确装配依赖
- [x] config.yaml 包含 ark 配置段
- [x] go vet 通过
