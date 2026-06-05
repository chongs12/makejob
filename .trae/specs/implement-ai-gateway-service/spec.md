# AI Gateway 服务完整实现 Spec (P1-5~P1-10)

## Why
AI Gateway 是微服务架构中所有 AI 调用的统一入口，负责配置管理、Prompt 渲染、LLM 调用、日志记录。当前只有空壳骨架，需要实现 InterviewAgent、PlanAgent、CompanionAgent、QuizAnalyzer、ResumeParser、Live2DDirector 六个 RPC。

## What Changes
- P1-5: 实现 InterviewAgent RPC（面试出题）
- P1-6: 实现 PlanAgent RPC（学习计划生成）
- P1-7: 实现 CompanionAgent RPC（AI 陪伴聊天）
- P1-8: 实现 QuizAnalyzer RPC（答题分析评估）
- P1-9: 实现 ResumeParser RPC（简历解析）
- P1-10: 实现 Live2DDirector RPC（Live2D 角色控制指令生成）

## Impact
- Affected specs: P1-5~P1-10 AI Gateway Service
- Affected code:
  - `app/ai_gateway/internal/conf/conf.go` (修改)
  - `app/ai_gateway/internal/biz/ai.go` (新建)
  - `app/ai_gateway/internal/biz/runtime_builder.go` (新建)
  - `app/ai_gateway/internal/biz/errors.go` (新建)
  - `app/ai_gateway/internal/data/data.go` (修改)
  - `app/ai_gateway/internal/data/ai_config_repo.go` (新建)
  - `app/ai_gateway/internal/data/prompt_repo.go` (新建)
  - `app/ai_gateway/internal/data/call_log_repo.go` (新建)
  - `app/ai_gateway/internal/service/ai.go` (新建)
  - `app/ai_gateway/internal/server/grpc.go` (修改)
  - `app/ai_gateway/cmd/server/main.go` (修改)
  - `app/ai_gateway/configs/config.yaml` (修改)

## ADDED Requirements

### Requirement: InterviewAgent RPC
系统 SHALL 调用 LLM 生成结构化面试题。

#### Scenario: 成功生成面试题
- **WHEN** 调用 InterviewAgent(mode="question", industry="backend")
- **THEN** 返回包含 content, question_type, difficulty, test_cases 的响应

### Requirement: PlanAgent RPC
系统 SHALL 调用 LLM 生成结构化学习计划。

#### Scenario: 成功生成计划
- **WHEN** 调用 PlanAgent(weak_topics=["goroutine"], duration_days=30)
- **THEN** 返回包含 title, phases, tasks 的响应

### Requirement: CompanionAgent RPC
系统 SHALL 调用 LLM 生成 AI 陪伴回复。

#### Scenario: 成功生成回复
- **WHEN** 调用 CompanionAgent(messages=[...], user_emotion="frustrated")
- **THEN** 返回包含 reply, emotion, action, live2d_directive 的响应

### Requirement: QuizAnalyzer RPC
系统 SHALL 调用 LLM 分析用户答题结果。

#### Scenario: 成功分析答题
- **WHEN** 调用 QuizAnalyzer(question="...", user_answer="...", correct_answer="...")
- **THEN** 返回包含 is_correct, score, feedback, mistake_tags 的响应

### Requirement: ResumeParser RPC
系统 SHALL 调用 LLM 解析简历文本。

#### Scenario: 成功解析简历
- **WHEN** 调用 ResumeParser(resume_text="...")
- **THEN** 返回包含 skills, experience, education, projects 的响应

### Requirement: Live2DDirector RPC
系统 SHALL 调用 LLM 生成 Live2D 角色控制指令。

#### Scenario: 成功生成指令
- **WHEN** 调用 Live2DDirector(text_to_express="...", scene="companion")
- **THEN** 返回包含 expression, motion, gesture, duration_ms 的响应

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
- 数据库操作使用 GORM
