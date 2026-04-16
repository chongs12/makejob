package ai

import "context"

// PlanAgent 学习规划Agent接口
// 提供个性化学习计划的生成和调整能力
type PlanAgent interface {
	// GeneratePlan 根据用户画像生成学习计划
	// ctx: 上下文
	// profile: 用户画像（水平、强弱项、目标等）
	// industryCode: 行业代码
	// 返回: 学习计划、错误
	GeneratePlan(ctx context.Context, profile UserProfile, industryCode string) (LearningPlan, error)

	// AdjustPlan 根据学习进度调整计划
	// ctx: 上下文
	// planID: 计划ID
	// completedTasks: 已完成的任务列表
	// performance: 各维度表现评分
	// 返回: 调整后的学习计划、错误
	AdjustPlan(ctx context.Context, planID string, completedTasks []string, performance map[string]float64) (LearningPlan, error)

	// GetStudySuggestion 获取学习建议
	// ctx: 上下文
	// profile: 用户画像
	// 返回: 学习建议文本、错误
	GetStudySuggestion(ctx context.Context, profile UserProfile) (string, error)
}
