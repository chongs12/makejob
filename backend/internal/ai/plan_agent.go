package ai

import "context"

// PhaseBlueprintEntry 表示阶段蓝图中的单个阶段窗口，用于传入 AI prompt。
type PhaseBlueprintEntry struct {
	Phase             string   `json:"phase"`
	PhaseGoal         string   `json:"phase_goal"`
	StartDay          int      `json:"start_day"`
	EndDay            int      `json:"end_day"`
	ExpectedTaskTypes []string `json:"expected_task_types"`
	ExitCriteria      []string `json:"exit_criteria"`
}

// PlanAdjustmentInput 表示学习计划调优时传给 AI 的结构化上下文。
type PlanAdjustmentInput struct {
	PlanID          string
	CompletedTasks  []string
	Performance     map[string]float64
	CurrentPhase    string
	EntryPhase      string
	ActionSummaries []string
	ReasonCodes     []string
	PhaseBlueprint  []PhaseBlueprintEntry
	WeakTopics      []string
	GoalDescription string
}

// PlanAgent 定义学习计划相关的 AI 能力。
type PlanAgent interface {
	// GeneratePlan 根据用户画像生成学习计划。
	GeneratePlan(ctx context.Context, profile UserProfile, industryCode string) (LearningPlan, error)

	// AdjustPlan 根据已完成任务、表现和阶段上下文调整学习计划。
	AdjustPlan(ctx context.Context, input PlanAdjustmentInput) (LearningPlan, error)

	// GetStudySuggestion 根据用户画像生成学习建议。
	GetStudySuggestion(ctx context.Context, profile UserProfile) (string, error)
}
