# Tasks

## P4-2: Interview Service - Implement FinishInterview

- [x] Task 1: 修改 biz/interview.go - 添加 InterviewReport 实体和 ReportRepo 接口
- [x] Task 2: 修改 biz/usecase.go - 实现 FinishInterview UseCase
  - [x] SubTask 2.1: 验证 interview 属于当前用户
  - [x] SubTask 2.2: 验证 status == "ongoing"
  - [x] SubTask 2.3: 更新 status = "report_generating"
  - [x] SubTask 2.4: 设置 finished_at
  - [x] SubTask 2.5: 发布 MQ 消息
- [x] Task 3: 修改 service/interview.go - 添加 FinishInterview handler

## P4-3: Interview Service - Report Generation MQ Consumer

- [x] Task 4: 新建 server/mq.go - 实现 ReportGenerationHandler
  - [x] SubTask 4.1: 加载 interview + 所有消息
  - [x] SubTask 4.2: 调用 AI Gateway.InterviewAgent(mode="report")
  - [x] SubTask 4.3: 保存 interview_reports 记录
  - [x] SubTask 4.4: 更新 interview.status = "completed"
  - [x] SubTask 4.5: 发布 interview.finished 事件
- [x] Task 5: 修改 main.go - 注册 MQ consumer

## P4-4: Interview Service - Implement GetReport

- [x] Task 6: 修改 biz/usecase.go - 实现 GetReport UseCase
  - [x] SubTask 6.1: 获取 interview，验证属于当前用户
  - [x] SubTask 6.2: 根据 status 返回不同响应
  - [x] SubTask 6.3: 加载 interview_reports
  - [x] SubTask 6.4: 解析 JSON 字段
- [x] Task 7: 修改 service/interview.go - 添加 GetReport handler

# Task Dependencies
- Task 1-3 (P4-2) 无依赖
- Task 4-5 (P4-3) 依赖 Task 1-3
- Task 6-7 (P4-4) 依赖 Task 1-3
