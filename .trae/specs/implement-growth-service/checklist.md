# Checklist

## P5-6: GetGrowthSummary
- [x] conf/conf.go 添加下游服务地址配置
- [x] biz/growth.go 添加 QuestionClient 接口
- [x] biz/growth.go 添加 PlanClient 接口
- [x] biz/growth.go 添加 LearningArchiveClient 接口
- [x] biz/growth.go 添加 InterviewClient 接口
- [x] biz/growth.go 重写 GetGrowthSummary UseCase（errgroup 并发）
- [x] GetGrowthSummary 设置 5 秒超时
- [x] GetGrowthSummary 失败降级处理
- [x] service/growth.go 更新 GetGrowthSummary handler
- [x] data/clients.go 实现下游服务客户端
- [x] go build 编译通过

## P5-7: GetWeeklyFocus
- [x] biz/growth.go 重写 GetWeeklyFocus UseCase
- [x] GetWeeklyFocus 调用 LearningArchive.GetFocusSignals
- [x] GetWeeklyFocus 调用 LearningArchive.GetWeakTopics
- [x] GetWeeklyFocus 调用 Plan.GetCurrentPlan
- [x] GetWeeklyFocus 调用 Question.ListQuestionSets
- [x] service/growth.go 更新 GetWeeklyFocus handler
- [x] go build 编译通过

## P5-8: SyncStudyLog
- [x] biz/growth.go 更新 StudyLog 实体（合并新旧字段）
- [x] biz/growth.go 重写 SyncStudyLog UseCase（Upsert）
- [x] data/growth_repo.go 实现 Upsert
- [x] service/growth.go 更新 SyncStudyLog handler
- [x] go build 编译通过

## 通用
- [x] go vet 通过
