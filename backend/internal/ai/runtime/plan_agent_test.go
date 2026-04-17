package runtime

import (
	"strings"
	"testing"

	"makejob-backend/internal/ai"
)

// TestNormalizeLearningPlanFillsMissingFields 验证计划归一化会补齐关键字段。
func TestNormalizeLearningPlanFillsMissingFields(t *testing.T) {
	plan, err := normalizeLearningPlan(learningPlanPayload{
		Tasks: []planTaskPayload{
			{
				Title: "Go 并发基础",
			},
		},
	}, ai.UserProfile{
		Level:           "beginner",
		DailyStudyTime:  90,
		DurationDays:    14,
		GoalDescription: "准备 Go 后端面试",
	}, "go")
	if err != nil {
		t.Fatalf("normalizeLearningPlan returned error: %v", err)
	}

	if plan.Duration != 14 {
		t.Fatalf("expected duration 14, got %d", plan.Duration)
	}
	if plan.Title == "" || plan.Description == "" {
		t.Fatalf("expected title and description to be filled")
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("expected one task, got %d", len(plan.Tasks))
	}
	if plan.Tasks[0].TaskType != "study" {
		t.Fatalf("expected default task type study, got %q", plan.Tasks[0].TaskType)
	}
	if plan.Tasks[0].DayNumber != 1 {
		t.Fatalf("expected default day number 1, got %d", plan.Tasks[0].DayNumber)
	}
	if plan.Tasks[0].Duration <= 0 {
		t.Fatalf("expected default duration to be filled, got %d", plan.Tasks[0].Duration)
	}
}

// TestBuildPlanGenerateUserPromptContainsDuration 验证生成提示词携带周期信息。
func TestBuildPlanGenerateUserPromptContainsDuration(t *testing.T) {
	prompt := buildPlanGenerateUserPrompt(ai.UserProfile{
		Level:           "intermediate",
		DailyStudyTime:  120,
		DurationDays:    21,
		GoalDescription: "准备 Java 面试",
	}, "java")

	if !strings.Contains(prompt, "计划周期: 21 天") {
		t.Fatalf("expected prompt to contain duration days, got %q", prompt)
	}
}

// TestNormalizePlanTaskClampsDayNumber 验证任务天数会被限制在计划周期内。
func TestNormalizePlanTaskClampsDayNumber(t *testing.T) {
	task := normalizePlanTask(planTaskPayload{
		Title:     "刷题训练",
		TaskType:  "practice",
		DayNumber: 30,
	}, 0, 14, 60)

	if task.DayNumber != 14 {
		t.Fatalf("expected day number to be clamped to 14, got %d", task.DayNumber)
	}
	if task.Priority != "medium" {
		t.Fatalf("expected default priority medium, got %q", task.Priority)
	}
}
