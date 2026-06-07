# Tasks

## P5-6: Growth Service - Rewrite GetGrowthSummary

- [x] Task 1: 修改 conf/conf.go - 添加下游服务地址配置
- [x] Task 2: 修改 biz/growth.go - 添加下游服务客户端接口
  - [x] SubTask 2.1: 添加 QuestionClient 接口
  - [x] SubTask 2.2: 添加 PlanClient 接口
  - [x] SubTask 2.3: 添加 LearningArchiveClient 接口
  - [x] SubTask 2.4: 添加 InterviewClient 接口
- [x] Task 3: 修改 biz/growth.go - 重写 GetGrowthSummary UseCase
  - [x] SubTask 3.1: 使用 errgroup 并发调用
  - [x] SubTask 3.2: 设置 5 秒超时
  - [x] SubTask 3.3: 失败降级处理
- [x] Task 4: 修改 service/growth.go - 更新 GetGrowthSummary handler
- [x] Task 5: 新建 data/clients.go - 实现下游服务客户端

## P5-7: Growth Service - Rewrite GetWeeklyFocus

- [x] Task 6: 修改 biz/growth.go - 重写 GetWeeklyFocus UseCase
  - [x] SubTask 6.1: 调用 LearningArchive.GetFocusSignals
  - [x] SubTask 6.2: 调用 LearningArchive.GetWeakTopics
  - [x] SubTask 6.3: 调用 Plan.GetCurrentPlan
  - [x] SubTask 6.4: 调用 Question.ListQuestionSets
- [x] Task 7: 修改 service/growth.go - 更新 GetWeeklyFocus handler

## P5-8: Growth Service - Fix SyncStudyLog

- [x] Task 8: 修改 biz/growth.go - 更新 StudyLog 实体
- [x] Task 9: 修改 biz/growth.go - 重写 SyncStudyLog UseCase（Upsert）
- [x] Task 10: 修改 data/growth_repo.go - 实现 Upsert
- [x] Task 11: 修改 service/growth.go - 更新 SyncStudyLog handler

# Task Dependencies
- Task 1-5 (P5-6) 无依赖
- Task 6-7 (P5-7) 依赖 Task 2
- Task 8-11 (P5-8) 无依赖
