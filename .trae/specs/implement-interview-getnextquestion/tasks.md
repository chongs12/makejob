# Tasks

## P4-1: Interview Service - Implement GetNextQuestion

- [x] Task 1: 修改 conf/conf.go - 添加 RAG 服务地址配置
  - [x] SubTask 1.1: 添加 RAG 结构体（ServiceAddr）
  - [x] SubTask 1.2: 更新 Bootstrap 结构体

- [x] Task 2: 修改 biz/interview.go - 添加 RAG 客户端接口
  - [x] SubTask 2.1: 定义 RAGClient 接口
  - [x] SubTask 2.2: 添加 GetNextQuestion UseCase 方法

- [x] Task 3: 实现 GetNextQuestion UseCase
  - [x] SubTask 3.1: 获取 interview 记录，验证 status
  - [x] SubTask 3.2: 保存 user_answer（如果非空）
  - [x] SubTask 3.3: 加载面试上下文（最近 20 条消息）
  - [x] SubTask 3.4: 调用 RAG.Retrieve（降级处理）
  - [x] SubTask 3.5: 调用 AI Gateway.InterviewAgent
  - [x] SubTask 3.6: 保存 AI 回复消息
  - [x] SubTask 3.7: 更新 current_question_index

- [x] Task 4: 修改 service/interview.go - 添加 GetNextQuestion handler
  - [x] SubTask 4.1: 实现 GetNextQuestion 方法
  - [x] SubTask 4.2: 添加 toProtoQuestion 转换函数

- [x] Task 5: 修改 main.go - 注入 RAG 客户端

# Task Dependencies
- Task 1 无依赖
- Task 2 依赖 Task 1
- Task 3 依赖 Task 2
- Task 4 依赖 Task 3
- Task 5 依赖 Task 2
