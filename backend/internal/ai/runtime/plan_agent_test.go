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

// TestBuildPlanAdjustUserPromptWithContextContainsPhase 验证调计划提示词会带出阶段上下文。
func TestBuildPlanAdjustUserPromptWithContextContainsPhase(t *testing.T) {
	prompt := buildPlanAdjustUserPromptWithContext(ai.PlanAdjustmentInput{
		PlanID:          "12",
		CompletedTasks:  []string{"数组基础", "链表练习"},
		Performance:     map[string]float64{"practice": 45},
		CurrentPhase:    "drill",
		EntryPhase:      "review",
		ActionSummaries: []string{"最近诊断：链表指针处理仍不稳定"},
	})

	if !strings.Contains(prompt, "当前阶段: drill") {
		t.Fatalf("expected prompt to contain current phase, got %q", prompt)
	}
	if !strings.Contains(prompt, "本轮入口阶段: review") {
		t.Fatalf("expected prompt to contain entry phase, got %q", prompt)
	}
	if !strings.Contains(prompt, "最近诊断摘要: 最近诊断：链表指针处理仍不稳定") {
		t.Fatalf("expected prompt to contain diagnosis summary, got %q", prompt)
	}
}

// TestBuildPlanGenerateUserPromptWithPhasesContainsBlueprint 验证生成计划提示词会带出阶段蓝图。
func TestBuildPlanGenerateUserPromptWithPhasesContainsBlueprint(t *testing.T) {
	prompt := buildPlanGenerateUserPromptWithPhases(ai.UserProfile{
		Level:           "intermediate",
		DailyStudyTime:  90,
		DurationDays:    21,
		GoalDescription: "准备后端面试",
		WeakTopics:      []string{"链表", "动态规划"},
	}, "go")

	if !strings.Contains(prompt, "阶段蓝图:") {
		t.Fatalf("expected prompt to contain phase blueprint, got %q", prompt)
	}
	if !strings.Contains(prompt, "打基础") || !strings.Contains(prompt, "专项突破") || !strings.Contains(prompt, "复盘纠偏") || !strings.Contains(prompt, "模拟验证") {
		t.Fatalf("expected prompt to mention full phase pipeline, got %q", prompt)
	}
}

// TestBuildGeneratePhaseBlueprintShortDurationSkipsMock 验证短周期蓝图不会强制 mock。
func TestBuildGeneratePhaseBlueprintShortDurationSkipsMock(t *testing.T) {
	blueprint := buildGeneratePhaseBlueprint(10)

	if !strings.Contains(blueprint, "foundation") || !strings.Contains(blueprint, "drill") || !strings.Contains(blueprint, "review") {
		t.Fatalf("expected short blueprint to include foundation/drill/review, got %q", blueprint)
	}
	if !strings.Contains(blueprint, "不强制安排 mock") {
		t.Fatalf("expected short blueprint to mention mock is optional, got %q", blueprint)
	}
}

// TestBuildGeneratePhaseBlueprintMediumDurationAddsLightMock 验证中过渡周期会在 review 后加入少量 mock。
func TestBuildGeneratePhaseBlueprintMediumDurationAddsLightMock(t *testing.T) {
	blueprint := buildGeneratePhaseBlueprint(18)

	if !strings.Contains(blueprint, "后段先做 review，再安排少量 mock") {
		t.Fatalf("expected medium blueprint to mention review then light mock, got %q", blueprint)
	}
	if !strings.Contains(blueprint, "interview") || !strings.Contains(blueprint, "practice") {
		t.Fatalf("expected medium blueprint to constrain mock task type, got %q", blueprint)
	}
}

// TestBuildGeneratePhaseBlueprintLongDurationUsesFullPipeline 验证长周期会显式覆盖完整四阶段主线。
func TestBuildGeneratePhaseBlueprintLongDurationUsesFullPipeline(t *testing.T) {
	blueprint := buildGeneratePhaseBlueprint(28)

	if !strings.Contains(blueprint, "第一阶段使用 foundation") {
		t.Fatalf("expected long blueprint to mention foundation stage, got %q", blueprint)
	}
	if !strings.Contains(blueprint, "第二阶段进入 drill") || !strings.Contains(blueprint, "第三阶段进入 review") || !strings.Contains(blueprint, "最后一阶段进入 mock") {
		t.Fatalf("expected long blueprint to mention full four-stage pipeline, got %q", blueprint)
	}
}

// TestBuildPlanSystemPromptWithPhasesContainsRules 验证生成系统提示词会显式约束阶段推进规则。
func TestBuildPlanSystemPromptWithPhasesContainsRules(t *testing.T) {
	prompt := buildPlanSystemPromptWithPhases("base prompt")

	if !strings.Contains(prompt, "foundation -> drill -> review -> mock") {
		t.Fatalf("expected prompt to contain phase pipeline, got %q", prompt)
	}
	if !strings.Contains(prompt, "计划和每个任务都必须输出 phase 与 phase_goal") {
		t.Fatalf("expected prompt to require phase fields, got %q", prompt)
	}
	if !strings.Contains(prompt, "mock 阶段任务类型必须以 interview 或限时综合 practice 为主") {
		t.Fatalf("expected prompt to contain mock-stage rule, got %q", prompt)
	}
}
