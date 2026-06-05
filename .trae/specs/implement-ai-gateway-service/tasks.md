# Tasks

- [x] Task 1: 修改配置结构 conf.go - 添加 AI Gateway 相关配置
  - [x] SubTask 1.1: 添加 ARK 结构体（APIKey, BaseURL）
  - [x] SubTask 1.2: 更新 Bootstrap 结构体包含 ARK 字段
  - [x] SubTask 1.3: 设置默认值

- [x] Task 2: 创建领域错误定义 biz/errors.go
  - [x] SubTask 2.1: 定义 ErrAIConfigNotFound (NotFound)
  - [x] SubTask 2.2: 定义 ErrLLMCallFailed (BadGateway)
  - [x] SubTask 2.3: 定义 ErrPromptRenderFailed (InternalServerError)
  - [x] SubTask 2.4: 定义 ErrParseFailed (InternalServerError)

- [x] Task 3: 创建数据库实体和 Repo 接口 biz/ai.go
  - [x] SubTask 3.1: 定义 AIConfig 实体
  - [x] SubTask 3.2: 定义 PromptTemplate 实体
  - [x] SubTask 3.3: 定义 AICallLog 实体
  - [x] SubTask 3.4: 定义 AIConfigRepo 接口
  - [x] SubTask 3.5: 定义 PromptRepo 接口
  - [x] SubTask 3.6: 定义 CallLogRepo 接口

- [x] Task 4: 创建 LLM 客户端 biz/runtime_builder.go
  - [x] SubTask 4.1: 定义 LLMClient 接口
  - [x] SubTask 4.2: 定义 LLMResponse 结构体
  - [x] SubTask 4.3: 实现 RenderPrompt 函数

- [x] Task 5: 实现 InterviewAgent UseCase biz/ai.go
  - [x] SubTask 5.1: 定义 InterviewAgentUseCase
  - [x] SubTask 5.2: 实现 GenerateQuestion 方法

- [x] Task 6: 实现 PlanAgent UseCase biz/ai.go
  - [x] SubTask 6.1: 定义 PlanAgentUseCase
  - [x] SubTask 6.2: 实现 GeneratePlan 方法

- [x] Task 7: 实现 CompanionAgent UseCase biz/ai.go
  - [x] SubTask 7.1: 定义 CompanionAgentUseCase
  - [x] SubTask 7.2: 实现 Chat 方法

- [x] Task 8: 实现 QuizAnalyzer UseCase biz/ai.go
  - [x] SubTask 8.1: 定义 QuizAnalyzerUseCase
  - [x] SubTask 8.2: 实现 Analyze 方法

- [x] Task 9: 实现 ResumeParser UseCase biz/ai.go
  - [x] SubTask 9.1: 定义 ResumeParserUseCase
  - [x] SubTask 9.2: 实现 Parse 方法

- [x] Task 10: 实现 Live2DDirector UseCase biz/ai.go
  - [x] SubTask 10.1: 定义 Live2DDirectorUseCase
  - [x] SubTask 10.2: 实现 GenerateDirective 方法

- [x] Task 11: 创建 Repo 实现 data/ai_config_repo.go
- [x] Task 12: 创建 Repo 实现 data/prompt_repo.go
- [x] Task 13: 创建 Repo 实现 data/call_log_repo.go
- [x] Task 14: 更新 data 层 data.go - 实现 GORM 连接和 AutoMigrate

- [x] Task 15: 实现 gRPC handler service/ai.go
  - [x] SubTask 15.1: 实现 InterviewAgent handler
  - [x] SubTask 15.2: 实现 PlanAgent handler
  - [x] SubTask 15.3: 实现 CompanionAgent handler
  - [x] SubTask 15.4: 实现 QuizAnalyzer handler
  - [x] SubTask 15.5: 实现 ResumeParser handler
  - [x] SubTask 15.6: 实现 Live2DDirector handler

- [x] Task 16: 更新 server 层 server/grpc.go
- [x] Task 17: 更新 main.go 启动入口
- [x] Task 18: 更新配置文件 configs/config.yaml

# Task Dependencies
- Task 1-2 无依赖
- Task 3 依赖 Task 1
- Task 4 依赖 Task 1
- Task 5-10 依赖 Task 3, 4
- Task 11-14 依赖 Task 1
- Task 15 依赖 Task 5-10
- Task 16-18 依赖 Task 1
