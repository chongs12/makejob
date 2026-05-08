package service

import (
	"testing"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

// TestArrangeLearningPlanByPhaseUsesCanonicalWindows 验证新计划会按标准阶段顺序重排并压实阶段窗口。
func TestArrangeLearningPlanByPhaseUsesCanonicalWindows(t *testing.T) {
	t.Parallel()

	plan := ai.LearningPlan{
		Duration: 8,
		Tasks: []ai.PlanTask{
			{Title: "复盘状态定义", TaskType: model.TaskTypeReview, DayNumber: 1},
			{Title: "理解状态转移方程", TaskType: model.TaskTypeStudy, DayNumber: 2},
			{Title: "限时模拟一轮", TaskType: model.TaskTypeInterview, DayNumber: 4},
			{Title: "做同类专项练习", TaskType: model.TaskTypePractice, DayNumber: 3},
		},
	}

	arranged := arrangeLearningPlanByPhase(normalizeLearningPlanPhases(plan), false)
	if arranged.Phase != model.LearningPhaseFoundation {
		t.Fatalf("expected plan phase foundation, got %s", arranged.Phase)
	}
	if len(arranged.Tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(arranged.Tasks))
	}

	expectedPhases := []string{
		model.LearningPhaseFoundation,
		model.LearningPhaseDrill,
		model.LearningPhaseReview,
		model.LearningPhaseMock,
	}
	expectedDays := []int{1, 3, 5, 7}
	for index, phase := range expectedPhases {
		if arranged.Tasks[index].Phase != phase {
			t.Fatalf("expected task %d phase %s, got %#v", index, phase, arranged.Tasks[index])
		}
		if arranged.Tasks[index].DayNumber != expectedDays[index] {
			t.Fatalf("expected task %d day %d, got %#v", index, expectedDays[index], arranged.Tasks[index])
		}
		if arranged.Tasks[index].PhaseGoal == "" {
			t.Fatalf("expected task %d phase goal to be filled", index)
		}
	}
}

// TestArrangeLearningPlanByPhasePreservesEntryPhaseOrder 验证调计划场景会保留当前阶段入口顺序，再收拢同阶段任务。
func TestArrangeLearningPlanByPhasePreservesEntryPhaseOrder(t *testing.T) {
	t.Parallel()

	plan := ai.LearningPlan{
		Duration: 6,
		Tasks: []ai.PlanTask{
			{Title: "先复盘错因", TaskType: model.TaskTypeReview, DayNumber: 1},
			{Title: "切换题型继续练", TaskType: model.TaskTypePractice, DayNumber: 2},
			{Title: "补充第二轮复盘", TaskType: model.TaskTypeReview, DayNumber: 3},
		},
	}

	arranged := arrangeLearningPlanByPhase(normalizeLearningPlanPhases(plan), true)
	if arranged.Phase != model.LearningPhaseReview {
		t.Fatalf("expected plan phase review, got %s", arranged.Phase)
	}
	if len(arranged.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(arranged.Tasks))
	}

	if arranged.Tasks[0].Phase != model.LearningPhaseReview || arranged.Tasks[1].Phase != model.LearningPhaseReview {
		t.Fatalf("expected review tasks to stay at the current stage entrance, got %#v", arranged.Tasks)
	}
	if arranged.Tasks[2].Phase != model.LearningPhaseDrill {
		t.Fatalf("expected drill task to be moved after review bucket, got %#v", arranged.Tasks[2])
	}
	if arranged.Tasks[0].DayNumber != 1 || arranged.Tasks[1].DayNumber != 4 || arranged.Tasks[2].DayNumber != 5 {
		t.Fatalf("expected phase windows to be compacted into review(1,4) and drill(5), got %#v", arranged.Tasks)
	}
}

// TestResolvePlanAdjustmentEntryPhase 验证 AdjustPlan 会根据诊断动作决定留段还是进入下一阶段。
func TestResolvePlanAdjustmentEntryPhase(t *testing.T) {
	t.Parallel()

	if phase := resolvePlanAdjustmentEntryPhase(model.LearningPhaseDrill, []planAdjustmentAction{
		{Action: model.LearningTaskDiagnosisActionAddReviewTask},
		{Action: model.LearningTaskDiagnosisActionRepeatSamePattern},
	}); phase != model.LearningPhaseReview {
		t.Fatalf("expected unresolved path to enter review phase, got %s", phase)
	}

	if phase := resolvePlanAdjustmentEntryPhase(model.LearningPhaseDrill, []planAdjustmentAction{
		{Action: model.LearningTaskDiagnosisActionRaiseDifficulty},
	}); phase != model.LearningPhaseMock {
		t.Fatalf("expected improved drill path to unlock mock phase, got %s", phase)
	}

	if phase := resolvePlanAdjustmentEntryPhase(model.LearningPhaseReview, []planAdjustmentAction{
		{Action: model.LearningTaskDiagnosisActionKeepProgress},
	}); phase != model.LearningPhaseDrill {
		t.Fatalf("expected review keep-progress path to return to drill phase, got %s", phase)
	}
}
