// Package service 提供业务逻辑层实现
package service

import (
	"sort"
	"strings"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

const (
	// PhaseBlueprintVersion 当前阶段蓝图规则版本。
	PhaseBlueprintVersion = "v1"

	// PhaseBlueprintSourceDuration 表示蓝图由周期规则自动生成。
	PhaseBlueprintSourceDuration = "duration_blueprint"
	// PhaseBlueprintSourceFocusOverride 表示蓝图由训练重点信号覆盖生成。
	PhaseBlueprintSourceFocusOverride = "focus_signal_override"
	// PhaseBlueprintSourceDiagnosisAdjustment 表示蓝图由诊断调整驱动生成。
	PhaseBlueprintSourceDiagnosisAdjustment = "diagnosis_adjustment"
)

// phaseBlueprintEntry 表示阶段蓝图中的单个阶段窗口。
type phaseBlueprintEntry struct {
	Phase             string   `json:"phase"`
	PhaseGoal         string   `json:"phase_goal"`
	StartDay          int      `json:"start_day"`
	EndDay            int      `json:"end_day"`
	ExpectedTaskTypes []string `json:"expected_task_types"`
	ExitCriteria      []string `json:"exit_criteria"`
	Source            string   `json:"source"`
}

// phaseBlueprintSummaryEntry 表示对前端轻展示友好的阶段窗口摘要。
type phaseBlueprintSummaryEntry struct {
	Phase     string `json:"phase"`
	PhaseGoal string `json:"phase_goal"`
	StartDay  int    `json:"start_day"`
	EndDay    int    `json:"end_day"`
}

// phaseTransitionEntry 表示阶段过渡历史中的单条记录。
type phaseTransitionEntry struct {
	At            string   `json:"at"`
	FromPhase     string   `json:"from_phase"`
	EntryPhase    string   `json:"entry_phase"`
	ReasonCodes   []string `json:"reason_codes"`
	Summaries     []string `json:"summaries"`
	DiagnosisRefs []string `json:"diagnosis_refs"`
}

// planAdjustmentActionFlags 表示诊断动作的聚合特征，用于按阶段流转决定原因码。
type planAdjustmentActionFlags struct {
	HasAddReviewTask        bool
	HasRepeatSamePattern    bool
	HasSwitchVariantPattern bool
	HasRaiseDifficulty      bool
	HasKeepProgress         bool
}

// buildPhaseBlueprint 根据计划周期天数生成阶段蓝图模板。
// 蓝图规则：
//   - 7-13 天：foundation -> drill -> review（不强制 mock）
//   - 14-20 天：foundation -> drill -> review -> 轻量 mock
//   - 21+ 天：完整 foundation -> drill -> review -> mock
func buildPhaseBlueprint(durationDays int, source string) []phaseBlueprintEntry {
	durationDays = max(durationDays, 7)
	if source == "" {
		source = PhaseBlueprintSourceDuration
	}

	if durationDays < 14 {
		return buildShortDurationBlueprint(durationDays, source)
	}
	if durationDays < 21 {
		return buildMediumDurationBlueprint(durationDays, source)
	}
	return buildLongDurationBlueprint(durationDays, source)
}

// buildPhaseBlueprintTemplateIndex 将蓝图模板整理为 phase -> entry 映射，供真实窗口重建时复用 exit_criteria。
func buildPhaseBlueprintTemplateIndex(durationDays int, source string) map[string]phaseBlueprintEntry {
	template := buildPhaseBlueprint(durationDays, source)
	index := make(map[string]phaseBlueprintEntry, len(template))
	for _, entry := range template {
		index[entry.Phase] = entry
	}
	return index
}

// buildPhaseBlueprintFromPlanTasks 根据已经过 normalize + arrange 的任务列表重建真实阶段蓝图。
func buildPhaseBlueprintFromPlanTasks(tasks []ai.PlanTask, durationDays int, source string) []phaseBlueprintEntry {
	if len(tasks) == 0 {
		return buildPhaseBlueprint(durationDays, source)
	}

	templateIndex := buildPhaseBlueprintTemplateIndex(durationDays, source)
	buckets := buildRealPhaseBuckets(tasks)
	if len(buckets) == 0 {
		return buildPhaseBlueprint(durationDays, source)
	}

	blueprint := make([]phaseBlueprintEntry, 0, len(buckets))
	for _, bucket := range buckets {
		entry := phaseBlueprintEntry{
			Phase:             bucket.Phase,
			PhaseGoal:         resolvePhaseBucketGoal(bucket),
			StartDay:          bucket.StartDay,
			EndDay:            bucket.EndDay,
			ExpectedTaskTypes: resolvePhaseBucketTaskTypes(bucket),
			Source:            source,
		}
		if template, ok := templateIndex[bucket.Phase]; ok {
			entry.ExitCriteria = template.ExitCriteria
		} else {
			entry.ExitCriteria = []string{}
		}
		blueprint = append(blueprint, entry)
	}
	return blueprint
}

// phaseBucket 表示按连续阶段聚合的任务桶。
type phaseBucket struct {
	Phase     string
	StartDay  int
	EndDay    int
	Tasks     []ai.PlanTask
	FirstGoal string
}

// buildRealPhaseBuckets 按任务当前顺序聚合连续阶段桶。
func buildRealPhaseBuckets(tasks []ai.PlanTask) []phaseBucket {
	if len(tasks) == 0 {
		return nil
	}

	var buckets []phaseBucket
	var current *phaseBucket

	for _, task := range tasks {
		phase := resolveOptionalLearningPhase(task.Phase, task.TaskType)

		if current == nil || current.Phase != phase {
			if current != nil {
				buckets = append(buckets, *current)
			}
			current = &phaseBucket{
				Phase:     phase,
				StartDay:  task.DayNumber,
				EndDay:    task.DayNumber,
				FirstGoal: strings.TrimSpace(task.PhaseGoal),
			}
			current.Tasks = append(current.Tasks, task)
		} else {
			current.Tasks = append(current.Tasks, task)
			if task.DayNumber < current.StartDay {
				current.StartDay = task.DayNumber
			}
			if task.DayNumber > current.EndDay {
				current.EndDay = task.DayNumber
			}
			if current.FirstGoal == "" {
				current.FirstGoal = strings.TrimSpace(task.PhaseGoal)
			}
		}
	}
	if current != nil {
		buckets = append(buckets, *current)
	}
	return buckets
}

// resolvePhaseBucketGoal 从桶中提取阶段目标，优先取首个非空任务 phase_goal。
func resolvePhaseBucketGoal(bucket phaseBucket) string {
	if bucket.FirstGoal != "" {
		return bucket.FirstGoal
	}
	return model.BuildLearningPhaseGoal(bucket.Phase)
}

// resolvePhaseBucketTaskTypes 从桶中提取去重后的任务类型列表。
func resolvePhaseBucketTaskTypes(bucket phaseBucket) []string {
	seen := make(map[string]struct{}, len(bucket.Tasks))
	var types []string
	for _, task := range bucket.Tasks {
		tt := strings.TrimSpace(task.TaskType)
		if tt == "" {
			continue
		}
		if _, exists := seen[tt]; exists {
			continue
		}
		seen[tt] = struct{}{}
		types = append(types, tt)
	}
	if len(types) == 0 {
		switch bucket.Phase {
		case model.LearningPhaseDrill:
			return []string{model.TaskTypePractice}
		case model.LearningPhaseReview:
			return []string{model.TaskTypeReview, model.TaskTypeStudy}
		case model.LearningPhaseMock:
			return []string{model.TaskTypeInterview, model.TaskTypePractice}
		default:
			return []string{model.TaskTypeStudy, model.TaskTypePractice}
		}
	}
	return types
}

// buildShortDurationBlueprint 生成 7-13 天短周期蓝图。
func buildShortDurationBlueprint(durationDays int, source string) []phaseBlueprintEntry {
	foundationEnd := max(durationDays*3/10, 2)
	drillEnd := max(durationDays*7/10, foundationEnd+1)
	reviewEnd := durationDays

	return []phaseBlueprintEntry{
		{
			Phase:             model.LearningPhaseFoundation,
			PhaseGoal:         model.BuildLearningPhaseGoal(model.LearningPhaseFoundation),
			StartDay:          1,
			EndDay:            foundationEnd,
			ExpectedTaskTypes: []string{model.TaskTypeStudy, model.TaskTypePractice},
			ExitCriteria:      []string{"能说清核心解法步骤", "完成至少一题开题型练习"},
			Source:            source,
		},
		{
			Phase:             model.LearningPhaseDrill,
			PhaseGoal:         model.BuildLearningPhaseGoal(model.LearningPhaseDrill),
			StartDay:          foundationEnd + 1,
			EndDay:            drillEnd,
			ExpectedTaskTypes: []string{model.TaskTypePractice},
			ExitCriteria:      []string{"同类题型正确率明显提升", "弱项标签覆盖次数达标"},
			Source:            source,
		},
		{
			Phase:             model.LearningPhaseReview,
			PhaseGoal:         model.BuildLearningPhaseGoal(model.LearningPhaseReview),
			StartDay:          drillEnd + 1,
			EndDay:            reviewEnd,
			ExpectedTaskTypes: []string{model.TaskTypeReview, model.TaskTypeStudy},
			ExitCriteria:      []string{"近期高频错误已整理成固定检查点"},
			Source:            source,
		},
	}
}

// buildMediumDurationBlueprint 生成 14-20 天中周期蓝图。
func buildMediumDurationBlueprint(durationDays int, source string) []phaseBlueprintEntry {
	foundationEnd := max(durationDays*2/10, 3)
	drillEnd := max(durationDays*5/10, foundationEnd+2)
	reviewEnd := max(durationDays*8/10, drillEnd+2)
	mockEnd := durationDays

	return []phaseBlueprintEntry{
		{
			Phase:             model.LearningPhaseFoundation,
			PhaseGoal:         model.BuildLearningPhaseGoal(model.LearningPhaseFoundation),
			StartDay:          1,
			EndDay:            foundationEnd,
			ExpectedTaskTypes: []string{model.TaskTypeStudy, model.TaskTypePractice},
			ExitCriteria:      []string{"能说清核心解法步骤", "完成至少一题开题型练习"},
			Source:            source,
		},
		{
			Phase:             model.LearningPhaseDrill,
			PhaseGoal:         model.BuildLearningPhaseGoal(model.LearningPhaseDrill),
			StartDay:          foundationEnd + 1,
			EndDay:            drillEnd,
			ExpectedTaskTypes: []string{model.TaskTypePractice},
			ExitCriteria:      []string{"同类题型正确率明显提升", "弱项标签覆盖次数达标"},
			Source:            source,
		},
		{
			Phase:             model.LearningPhaseReview,
			PhaseGoal:         model.BuildLearningPhaseGoal(model.LearningPhaseReview),
			StartDay:          drillEnd + 1,
			EndDay:            reviewEnd,
			ExpectedTaskTypes: []string{model.TaskTypeReview, model.TaskTypeStudy},
			ExitCriteria:      []string{"近期高频错误已整理成固定检查点"},
			Source:            source,
		},
		{
			Phase:             model.LearningPhaseMock,
			PhaseGoal:         model.BuildLearningPhaseGoal(model.LearningPhaseMock),
			StartDay:          reviewEnd + 1,
			EndDay:            mockEnd,
			ExpectedTaskTypes: []string{model.TaskTypeInterview, model.TaskTypePractice},
			ExitCriteria:      []string{"限时场景下能稳定输出正确解法"},
			Source:            source,
		},
	}
}

// buildLongDurationBlueprint 生成 21+ 天长周期蓝图。
func buildLongDurationBlueprint(durationDays int, source string) []phaseBlueprintEntry {
	foundationEnd := max(durationDays/5, 4)
	drillEnd := max(durationDays*2/5, foundationEnd+3)
	reviewEnd := max(durationDays*4/5, drillEnd+3)
	mockEnd := durationDays

	return []phaseBlueprintEntry{
		{
			Phase:             model.LearningPhaseFoundation,
			PhaseGoal:         model.BuildLearningPhaseGoal(model.LearningPhaseFoundation),
			StartDay:          1,
			EndDay:            foundationEnd,
			ExpectedTaskTypes: []string{model.TaskTypeStudy, model.TaskTypePractice},
			ExitCriteria:      []string{"能说清核心解法步骤", "完成至少一题开题型练习"},
			Source:            source,
		},
		{
			Phase:             model.LearningPhaseDrill,
			PhaseGoal:         model.BuildLearningPhaseGoal(model.LearningPhaseDrill),
			StartDay:          foundationEnd + 1,
			EndDay:            drillEnd,
			ExpectedTaskTypes: []string{model.TaskTypePractice},
			ExitCriteria:      []string{"同类题型正确率明显提升", "弱项标签覆盖次数达标"},
			Source:            source,
		},
		{
			Phase:             model.LearningPhaseReview,
			PhaseGoal:         model.BuildLearningPhaseGoal(model.LearningPhaseReview),
			StartDay:          drillEnd + 1,
			EndDay:            reviewEnd,
			ExpectedTaskTypes: []string{model.TaskTypeReview, model.TaskTypeStudy},
			ExitCriteria:      []string{"近期高频错误已整理成固定检查点"},
			Source:            source,
		},
		{
			Phase:             model.LearningPhaseMock,
			PhaseGoal:         model.BuildLearningPhaseGoal(model.LearningPhaseMock),
			StartDay:          reviewEnd + 1,
			EndDay:            mockEnd,
			ExpectedTaskTypes: []string{model.TaskTypeInterview, model.TaskTypePractice},
			ExitCriteria:      []string{"限时场景下能稳定输出正确解法"},
			Source:            source,
		},
	}
}

// buildPhaseBlueprintSummary 从完整蓝图中提取对前端轻展示友好的摘要。
func buildPhaseBlueprintSummary(blueprint []phaseBlueprintEntry) []phaseBlueprintSummaryEntry {
	if len(blueprint) == 0 {
		return nil
	}

	summary := make([]phaseBlueprintSummaryEntry, 0, len(blueprint))
	for _, entry := range blueprint {
		summary = append(summary, phaseBlueprintSummaryEntry{
			Phase:     entry.Phase,
			PhaseGoal: entry.PhaseGoal,
			StartDay:  entry.StartDay,
			EndDay:    entry.EndDay,
		})
	}
	return summary
}

// normalizePhaseBlueprint 清理蓝图中的无效条目，确保阶段枚举合法。
func normalizePhaseBlueprint(blueprint []phaseBlueprintEntry) []phaseBlueprintEntry {
	if len(blueprint) == 0 {
		return nil
	}

	result := make([]phaseBlueprintEntry, 0, len(blueprint))
	for _, entry := range blueprint {
		normalized := entry
		normalized.Phase = model.NormalizeLearningPhase(entry.Phase)
		normalized.PhaseGoal = strings.TrimSpace(entry.PhaseGoal)
		if normalized.PhaseGoal == "" {
			normalized.PhaseGoal = model.BuildLearningPhaseGoal(normalized.Phase)
		}
		normalized.Source = strings.TrimSpace(normalized.Source)
		if normalized.Source == "" {
			normalized.Source = PhaseBlueprintSourceDuration
		}
		normalized.StartDay = max(normalized.StartDay, 1)
		if normalized.EndDay < normalized.StartDay {
			normalized.EndDay = normalized.StartDay
		}
		result = append(result, normalized)
	}
	return result
}

// resolvePhaseBlueprintSource 根据是否有诊断调整决定蓝图来源标记。
func resolvePhaseBlueprintSource(hasDiagnosisAdjustment bool, hasFocusOverride bool) string {
	if hasDiagnosisAdjustment {
		return PhaseBlueprintSourceDiagnosisAdjustment
	}
	if hasFocusOverride {
		return PhaseBlueprintSourceFocusOverride
	}
	return PhaseBlueprintSourceDuration
}

// buildAdjustmentReasonCodes 根据诊断动作和阶段流转生成机器可读的原因码。
func buildAdjustmentReasonCodes(actions []planAdjustmentAction, currentPhase string, entryPhase string) []string {
	if len(actions) == 0 {
		return nil
	}

	flags := collectPlanAdjustmentActionFlags(actions)
	currentPhase = model.NormalizeLearningPhase(currentPhase)
	entryPhase = model.NormalizeLearningPhase(entryPhase)

	codes := make([]string, 0, 3)

	// mock -> review：模拟验证不稳定
	if currentPhase == model.LearningPhaseMock && entryPhase == model.LearningPhaseReview {
		if flags.HasAddReviewTask || flags.HasRepeatSamePattern || flags.HasSwitchVariantPattern {
			codes = append(codes, "mock_not_stable")
		}
	}

	// drill -> review 或 mock -> review 中的 unresolved 动作
	if entryPhase == model.LearningPhaseReview {
		if flags.HasAddReviewTask || flags.HasRepeatSamePattern {
			codes = append(codes, "weakness_unresolved")
		}
	}

	// drill -> drill：部分掌握，用变式题确认
	if currentPhase == model.LearningPhaseDrill && entryPhase == model.LearningPhaseDrill {
		if flags.HasSwitchVariantPattern {
			codes = append(codes, "partial_mastery")
		}
	}

	// review -> drill：复盘完成，回到训练
	if currentPhase == model.LearningPhaseReview && entryPhase == model.LearningPhaseDrill {
		if flags.HasKeepProgress || flags.HasRaiseDifficulty {
			codes = append(codes, "review_completed")
		}
	}

	// drill -> mock：专项验证通过
	if currentPhase == model.LearningPhaseDrill && entryPhase == model.LearningPhaseMock {
		if flags.HasRaiseDifficulty || flags.HasKeepProgress {
			codes = append(codes, "progress_verified")
		}
	}

	// fallback：如果上面规则都没命中，按动作兜底
	if len(codes) == 0 {
		if flags.HasAddReviewTask || flags.HasRepeatSamePattern {
			codes = append(codes, "weakness_unresolved")
		} else if flags.HasSwitchVariantPattern {
			codes = append(codes, "partial_mastery")
		} else if flags.HasRaiseDifficulty || flags.HasKeepProgress {
			codes = append(codes, "progress_verified")
		}
	}

	// 去重并按固定顺序输出
	return deduplicateReasonCodes(codes)
}

// reasonCodeOrder 定义原因码的固定输出顺序。
var reasonCodeOrder = map[string]int{
	"mock_not_stable":     0,
	"weakness_unresolved": 1,
	"partial_mastery":     2,
	"review_completed":    3,
	"progress_verified":   4,
}

// deduplicateReasonCodes 去重并按固定顺序排列原因码。
func deduplicateReasonCodes(codes []string) []string {
	if len(codes) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(codes))
	unique := make([]string, 0, len(codes))
	for _, code := range codes {
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		unique = append(unique, code)
	}

	sort.SliceStable(unique, func(i, j int) bool {
		oi, okI := reasonCodeOrder[unique[i]]
		oj, okJ := reasonCodeOrder[unique[j]]
		if okI && okJ {
			return oi < oj
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}
		return unique[i] < unique[j]
	})

	if len(unique) == 0 {
		return nil
	}
	return unique
}

// collectPlanAdjustmentActionFlags 从诊断动作列表中聚合动作特征。
func collectPlanAdjustmentActionFlags(actions []planAdjustmentAction) planAdjustmentActionFlags {
	var flags planAdjustmentActionFlags
	for _, action := range actions {
		switch action.Action {
		case model.LearningTaskDiagnosisActionAddReviewTask:
			flags.HasAddReviewTask = true
		case model.LearningTaskDiagnosisActionRepeatSamePattern:
			flags.HasRepeatSamePattern = true
		case model.LearningTaskDiagnosisActionSwitchVariantPattern:
			flags.HasSwitchVariantPattern = true
		case model.LearningTaskDiagnosisActionRaiseDifficulty:
			flags.HasRaiseDifficulty = true
		case model.LearningTaskDiagnosisActionKeepProgress:
			flags.HasKeepProgress = true
		}
	}
	return flags
}

// buildPhaseTransitionEntry 构造一条阶段过渡历史记录。
func buildPhaseTransitionEntry(
	fromPhase string,
	entryPhase string,
	reasonCodes []string,
	summaries []string,
	diagnosisRefs []string,
) phaseTransitionEntry {
	return phaseTransitionEntry{
		At:            time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		FromPhase:     model.NormalizeLearningPhase(fromPhase),
		EntryPhase:    model.NormalizeLearningPhase(entryPhase),
		ReasonCodes:   append([]string(nil), reasonCodes...),
		Summaries:     append([]string(nil), summaries...),
		DiagnosisRefs: append([]string(nil), diagnosisRefs...),
	}
}

// buildDiagnosisSourceRefs 从诊断动作中提取唯一的来源引用，供阶段过渡历史记录使用。
func buildDiagnosisSourceRefs(actions []planAdjustmentAction) []string {
	if len(actions) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(actions))
	refs := make([]string, 0, len(actions))
	for _, action := range actions {
		ref := strings.TrimSpace(action.SourceRef)
		if ref == "" {
			continue
		}
		if _, exists := seen[ref]; exists {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return nil
	}
	return refs
}
