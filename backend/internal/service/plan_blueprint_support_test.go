package service

import (
	"testing"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

// TestBuildPhaseBlueprintShortDuration 验证 7-13 天短周期蓝图只包含 foundation/drill/review 三个阶段。
func TestBuildPhaseBlueprintShortDuration(t *testing.T) {
	t.Parallel()

	blueprint := buildPhaseBlueprint(10, PhaseBlueprintSourceDuration)
	if len(blueprint) != 3 {
		t.Fatalf("expected 3 phases for short duration, got %d", len(blueprint))
	}

	expectedPhases := []string{
		model.LearningPhaseFoundation,
		model.LearningPhaseDrill,
		model.LearningPhaseReview,
	}
	for i, phase := range expectedPhases {
		if blueprint[i].Phase != phase {
			t.Fatalf("expected phase %d to be %s, got %s", i, phase, blueprint[i].Phase)
		}
		if blueprint[i].PhaseGoal == "" {
			t.Fatalf("expected phase %d to have phase_goal", i)
		}
		if blueprint[i].StartDay < 1 {
			t.Fatalf("expected phase %d start_day >= 1, got %d", i, blueprint[i].StartDay)
		}
		if blueprint[i].EndDay < blueprint[i].StartDay {
			t.Fatalf("expected phase %d end_day >= start_day, got %d < %d", i, blueprint[i].EndDay, blueprint[i].StartDay)
		}
		if blueprint[i].Source != PhaseBlueprintSourceDuration {
			t.Fatalf("expected phase %d source to be duration_blueprint, got %s", i, blueprint[i].Source)
		}
		if len(blueprint[i].ExpectedTaskTypes) == 0 {
			t.Fatalf("expected phase %d to have expected_task_types", i)
		}
		if len(blueprint[i].ExitCriteria) == 0 {
			t.Fatalf("expected phase %d to have exit_criteria", i)
		}
	}

	if blueprint[2].EndDay != 10 {
		t.Fatalf("expected short cycle to end at day 10, got %d", blueprint[2].EndDay)
	}
}

// TestBuildPhaseBlueprintMediumDuration 验证 14-20 天中周期蓝图包含四个阶段。
func TestBuildPhaseBlueprintMediumDuration(t *testing.T) {
	t.Parallel()

	blueprint := buildPhaseBlueprint(18, PhaseBlueprintSourceDuration)
	if len(blueprint) != 4 {
		t.Fatalf("expected 4 phases for medium duration, got %d", len(blueprint))
	}

	expectedPhases := []string{
		model.LearningPhaseFoundation,
		model.LearningPhaseDrill,
		model.LearningPhaseReview,
		model.LearningPhaseMock,
	}
	for i, phase := range expectedPhases {
		if blueprint[i].Phase != phase {
			t.Fatalf("expected phase %d to be %s, got %s", i, phase, blueprint[i].Phase)
		}
	}

	if blueprint[3].EndDay != 18 {
		t.Fatalf("expected medium cycle to end at day 18, got %d", blueprint[3].EndDay)
	}
}

// TestBuildPhaseBlueprintLongDuration 验证 21+ 天长周期蓝图包含四个阶段且时间窗口更宽。
func TestBuildPhaseBlueprintLongDuration(t *testing.T) {
	t.Parallel()

	blueprint := buildPhaseBlueprint(30, PhaseBlueprintSourceDuration)
	if len(blueprint) != 4 {
		t.Fatalf("expected 4 phases for long duration, got %d", len(blueprint))
	}

	if blueprint[3].EndDay != 30 {
		t.Fatalf("expected long cycle to end at day 30, got %d", blueprint[3].EndDay)
	}

	drillSpan := blueprint[1].EndDay - blueprint[1].StartDay + 1
	if drillSpan < 5 {
		t.Fatalf("expected long cycle drill span >= 5 days, got %d", drillSpan)
	}
}

// TestBuildPhaseBlueprintMinimumDuration 验证低于 7 天的周期会被强制提升到 7 天。
func TestBuildPhaseBlueprintMinimumDuration(t *testing.T) {
	t.Parallel()

	blueprint := buildPhaseBlueprint(3, PhaseBlueprintSourceDuration)
	if len(blueprint) != 3 {
		t.Fatalf("expected 3 phases for very short duration, got %d", len(blueprint))
	}
	if blueprint[2].EndDay != 7 {
		t.Fatalf("expected minimum duration blueprint to end at day 7, got %d", blueprint[2].EndDay)
	}
}

// TestBuildPhaseBlueprintSummary 验证摘要提取只包含前端展示所需的四个字段。
func TestBuildPhaseBlueprintSummary(t *testing.T) {
	t.Parallel()

	blueprint := buildPhaseBlueprint(14, PhaseBlueprintSourceDuration)
	summary := buildPhaseBlueprintSummary(blueprint)
	if len(summary) != len(blueprint) {
		t.Fatalf("expected summary length %d, got %d", len(blueprint), len(summary))
	}

	for i, entry := range summary {
		if entry.Phase != blueprint[i].Phase {
			t.Fatalf("expected summary phase %d to match blueprint, got %s vs %s", i, entry.Phase, blueprint[i].Phase)
		}
		if entry.StartDay != blueprint[i].StartDay || entry.EndDay != blueprint[i].EndDay {
			t.Fatalf("expected summary day range %d to match blueprint", i)
		}
	}
}

// TestNormalizePhaseBlueprint 验证蓝图规范化会修复非法阶段枚举和缺失的阶段目标。
func TestNormalizePhaseBlueprint(t *testing.T) {
	t.Parallel()

	raw := []phaseBlueprintEntry{
		{Phase: "INVALID", PhaseGoal: "", StartDay: 1, EndDay: 5, Source: ""},
		{Phase: "drill", PhaseGoal: "  ", StartDay: 6, EndDay: 3, Source: "custom"},
	}

	normalized := normalizePhaseBlueprint(raw)
	if len(normalized) != 2 {
		t.Fatalf("expected 2 normalized entries, got %d", len(normalized))
	}
	if normalized[0].Phase != model.LearningPhaseFoundation {
		t.Fatalf("expected invalid phase to normalize to foundation, got %s", normalized[0].Phase)
	}
	if normalized[0].PhaseGoal == "" {
		t.Fatalf("expected empty phase goal to be filled")
	}
	if normalized[0].Source != PhaseBlueprintSourceDuration {
		t.Fatalf("expected empty source to default to duration_blueprint, got %s", normalized[0].Source)
	}
	if normalized[1].EndDay != normalized[1].StartDay {
		t.Fatalf("expected end_day to be adjusted to start_day when reversed, got %d < %d", normalized[1].EndDay, normalized[1].StartDay)
	}
}

// TestBuildAdjustmentReasonCodesUnresolved 验证未解决弱点会生成 weakness_unresolved 原因码。
func TestBuildAdjustmentReasonCodesUnresolved(t *testing.T) {
	t.Parallel()

	actions := []planAdjustmentAction{
		{Action: model.LearningTaskDiagnosisActionAddReviewTask},
		{Action: model.LearningTaskDiagnosisActionRepeatSamePattern},
	}

	codes := buildAdjustmentReasonCodes(actions, model.LearningPhaseDrill, model.LearningPhaseReview)
	if len(codes) != 1 {
		t.Fatalf("expected 1 deduplicated reason code, got %d: %v", len(codes), codes)
	}
	if codes[0] != "weakness_unresolved" {
		t.Fatalf("expected weakness_unresolved, got %s", codes[0])
	}
}

// TestBuildAdjustmentReasonCodesProgressVerified 验证已改善路径会生成 progress_verified 原因码。
func TestBuildAdjustmentReasonCodesProgressVerified(t *testing.T) {
	t.Parallel()

	actions := []planAdjustmentAction{
		{Action: model.LearningTaskDiagnosisActionRaiseDifficulty},
		{Action: model.LearningTaskDiagnosisActionKeepProgress},
	}

	codes := buildAdjustmentReasonCodes(actions, model.LearningPhaseDrill, model.LearningPhaseMock)
	if len(codes) != 1 {
		t.Fatalf("expected 1 deduplicated reason code, got %d: %v", len(codes), codes)
	}
	if codes[0] != "progress_verified" {
		t.Fatalf("expected progress_verified, got %s", codes[0])
	}
}

// TestBuildAdjustmentReasonCodesPartialMastery 验证变式确认路径会生成 partial_mastery 原因码。
func TestBuildAdjustmentReasonCodesPartialMastery(t *testing.T) {
	t.Parallel()

	actions := []planAdjustmentAction{
		{Action: model.LearningTaskDiagnosisActionSwitchVariantPattern},
	}

	codes := buildAdjustmentReasonCodes(actions, model.LearningPhaseDrill, model.LearningPhaseDrill)
	if len(codes) != 1 {
		t.Fatalf("expected 1 reason code, got %d: %v", len(codes), codes)
	}
	if codes[0] != "partial_mastery" {
		t.Fatalf("expected partial_mastery, got %s", codes[0])
	}
}

// TestBuildAdjustmentReasonCodesEmpty 验证空动作列表不会生成原因码。
func TestBuildAdjustmentReasonCodesEmpty(t *testing.T) {
	t.Parallel()

	codes := buildAdjustmentReasonCodes(nil, model.LearningPhaseDrill, model.LearningPhaseDrill)
	if codes != nil {
		t.Fatalf("expected nil codes for empty actions, got %v", codes)
	}
}

// TestBuildAdjustmentReasonCodesMockNotStable 验证 mock -> review 路径会生成 mock_not_stable 原因码。
func TestBuildAdjustmentReasonCodesMockNotStable(t *testing.T) {
	t.Parallel()

	actions := []planAdjustmentAction{
		{Action: model.LearningTaskDiagnosisActionAddReviewTask},
		{Action: model.LearningTaskDiagnosisActionRepeatSamePattern},
	}

	codes := buildAdjustmentReasonCodes(actions, model.LearningPhaseMock, model.LearningPhaseReview)
	if len(codes) < 1 {
		t.Fatalf("expected at least 1 reason code, got %d: %v", len(codes), codes)
	}
	if codes[0] != "mock_not_stable" {
		t.Fatalf("expected mock_not_stable as first code, got %s", codes[0])
	}
}

// TestBuildAdjustmentReasonCodesReviewCompleted 验证 review -> drill 路径会生成 review_completed 原因码。
func TestBuildAdjustmentReasonCodesReviewCompleted(t *testing.T) {
	t.Parallel()

	actions := []planAdjustmentAction{
		{Action: model.LearningTaskDiagnosisActionKeepProgress},
		{Action: model.LearningTaskDiagnosisActionRaiseDifficulty},
	}

	codes := buildAdjustmentReasonCodes(actions, model.LearningPhaseReview, model.LearningPhaseDrill)
	if len(codes) != 1 {
		t.Fatalf("expected 1 reason code, got %d: %v", len(codes), codes)
	}
	if codes[0] != "review_completed" {
		t.Fatalf("expected review_completed, got %s", codes[0])
	}
}

// TestBuildPhaseBlueprintFromPlanTasks 验证从任务列表重建蓝图会生成与真实任务阶段顺序一致的窗口。
func TestBuildPhaseBlueprintFromPlanTasks(t *testing.T) {
	t.Parallel()

	tasks := []ai.PlanTask{
		{Title: "t1", TaskType: model.TaskTypeStudy, Phase: model.LearningPhaseFoundation, DayNumber: 1},
		{Title: "t2", TaskType: model.TaskTypeStudy, Phase: model.LearningPhaseFoundation, DayNumber: 2},
		{Title: "t3", TaskType: model.TaskTypePractice, Phase: model.LearningPhaseDrill, DayNumber: 3},
		{Title: "t4", TaskType: model.TaskTypePractice, Phase: model.LearningPhaseDrill, DayNumber: 4},
		{Title: "t5", TaskType: model.TaskTypeReview, Phase: model.LearningPhaseReview, DayNumber: 5},
	}

	blueprint := buildPhaseBlueprintFromPlanTasks(tasks, 14, PhaseBlueprintSourceDuration)
	if len(blueprint) != 3 {
		t.Fatalf("expected 3 phase buckets, got %d", len(blueprint))
	}
	if blueprint[0].Phase != model.LearningPhaseFoundation {
		t.Fatalf("expected first phase to be foundation, got %s", blueprint[0].Phase)
	}
	if blueprint[0].StartDay != 1 || blueprint[0].EndDay != 2 {
		t.Fatalf("expected foundation window 1-2, got %d-%d", blueprint[0].StartDay, blueprint[0].EndDay)
	}
	if blueprint[1].Phase != model.LearningPhaseDrill {
		t.Fatalf("expected second phase to be drill, got %s", blueprint[1].Phase)
	}
	if blueprint[1].StartDay != 3 || blueprint[1].EndDay != 4 {
		t.Fatalf("expected drill window 3-4, got %d-%d", blueprint[1].StartDay, blueprint[1].EndDay)
	}
	if blueprint[2].Phase != model.LearningPhaseReview {
		t.Fatalf("expected third phase to be review, got %s", blueprint[2].Phase)
	}
}

// TestBuildPhaseBlueprintFromPlanTasksEmpty 验证空任务列表会回退到标准模板蓝图。
func TestBuildPhaseBlueprintFromPlanTasksEmpty(t *testing.T) {
	t.Parallel()

	blueprint := buildPhaseBlueprintFromPlanTasks(nil, 14, PhaseBlueprintSourceDuration)
	if len(blueprint) == 0 {
		t.Fatal("expected fallback blueprint for empty tasks")
	}
	// 14 天中周期模板应有 4 个阶段。
	if len(blueprint) != 4 {
		t.Fatalf("expected 4 phases in fallback blueprint, got %d", len(blueprint))
	}
}

// TestBuildPhaseTransitionEntry 验证阶段过渡历史记录的字段构造正确。
func TestBuildPhaseTransitionEntry(t *testing.T) {
	t.Parallel()

	entry := buildPhaseTransitionEntry(
		model.LearningPhaseDrill,
		model.LearningPhaseReview,
		[]string{"weakness_unresolved"},
		[]string{"先复盘再继续"},
		[]string{"plan-feedback:601"},
	)

	if entry.At == "" {
		t.Fatal("expected transition entry to have timestamp")
	}
	if entry.FromPhase != model.LearningPhaseDrill {
		t.Fatalf("expected from_phase drill, got %s", entry.FromPhase)
	}
	if entry.EntryPhase != model.LearningPhaseReview {
		t.Fatalf("expected entry_phase review, got %s", entry.EntryPhase)
	}
	if len(entry.ReasonCodes) != 1 || entry.ReasonCodes[0] != "weakness_unresolved" {
		t.Fatalf("expected reason codes [weakness_unresolved], got %v", entry.ReasonCodes)
	}
	if len(entry.Summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(entry.Summaries))
	}
	if len(entry.DiagnosisRefs) != 1 || entry.DiagnosisRefs[0] != "plan-feedback:601" {
		t.Fatalf("expected diagnosis refs [plan-feedback:601], got %v", entry.DiagnosisRefs)
	}
}

// TestBuildDiagnosisSourceRefs 验证从诊断动作中提取唯一来源引用。
func TestBuildDiagnosisSourceRefs(t *testing.T) {
	t.Parallel()

	actions := []planAdjustmentAction{
		{SourceRef: "plan-feedback:601"},
		{SourceRef: "plan-feedback:601"},
		{SourceRef: "plan-feedback:602"},
		{SourceRef: ""},
	}

	refs := buildDiagnosisSourceRefs(actions)
	if len(refs) != 2 {
		t.Fatalf("expected 2 unique refs, got %d: %v", len(refs), refs)
	}
	if refs[0] != "plan-feedback:601" || refs[1] != "plan-feedback:602" {
		t.Fatalf("unexpected refs: %v", refs)
	}
}

// TestBuildDiagnosisSourceRefsEmpty 验证空动作列表返回 nil。
func TestBuildDiagnosisSourceRefsEmpty(t *testing.T) {
	t.Parallel()

	refs := buildDiagnosisSourceRefs(nil)
	if refs != nil {
		t.Fatalf("expected nil refs for empty actions, got %v", refs)
	}
}

// TestResolvePhaseBlueprintSource 验证蓝图来源标记的优先级。
func TestResolvePhaseBlueprintSource(t *testing.T) {
	t.Parallel()

	if s := resolvePhaseBlueprintSource(true, false); s != PhaseBlueprintSourceDiagnosisAdjustment {
		t.Fatalf("expected diagnosis_adjustment, got %s", s)
	}
	if s := resolvePhaseBlueprintSource(false, true); s != PhaseBlueprintSourceFocusOverride {
		t.Fatalf("expected focus_signal_override, got %s", s)
	}
	if s := resolvePhaseBlueprintSource(false, false); s != PhaseBlueprintSourceDuration {
		t.Fatalf("expected duration_blueprint, got %s", s)
	}
	if s := resolvePhaseBlueprintSource(true, true); s != PhaseBlueprintSourceDiagnosisAdjustment {
		t.Fatalf("expected diagnosis_adjustment to take priority, got %s", s)
	}
}
