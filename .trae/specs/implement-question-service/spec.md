# Question 服务完整实现 Spec (P3-1~P3-8)

## Why
Question 服务当前只有基础的题目 CRUD 和答题功能，缺少 RunCode（调用 CodeRunner）、DeleteNote、GenerateTimedExam、SubmitExam、ListQuestionSets、ListMistakeTopics、MQ Consumers、增强推荐等功能。

## What Changes
- P3-1: 实现 RunCode RPC（调用 CodeRunner 服务）
- P3-2: 实现 DeleteNote RPC（软删除笔记）
- P3-3: 实现 GenerateTimedExam RPC（限时考试生成）
- P3-4: 实现 SubmitExam RPC（考试提交和 AI 批改）
- P3-5: 实现 ListQuestionSets + GetQuestionSetDetail RPC
- P3-6: 实现 ListMistakeTopics RPC（错题主题聚合）
- P3-7: 实现 MQ Consumers（pipeline.build + scraper.import）
- P3-8: 增强 GetPracticeRecommendations（面试驱动加权推荐）

## Impact
- Affected specs: P3-1~P3-8
- Affected code:
  - `app/question/internal/biz/question.go` (修改)
  - `app/question/internal/data/question_repo.go` (修改)
  - `app/question/internal/service/question.go` (修改)
  - `app/question/internal/conf/conf.go` (修改)
  - `app/question/internal/server/mq.go` (新建)
  - `app/question/cmd/server/main.go` (修改)

## 全局规范遵循
- 错误处理：使用 kratos errors 包
- 构造函数：NewXxx(deps...) 模式
- 禁止全局变量和 init() 函数
- 使用 context 传播
- 使用中文注释
