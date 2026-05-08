// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

const defaultPlanDiagnosisLoadLimit = 8

// planAdjustmentAction 表示本轮计划调整时可直接消费的一条结构化动作建议。
type planAdjustmentAction struct {
	Action         string
	TargetFocusTag string
	CollectionHint string
	SourceRef      string
	Reason         string
}

// planAdjustmentDecision 表示本轮 AdjustPlan 聚合得到的内部调整决策。
type planAdjustmentDecision struct {
	PerformanceByTaskType map[string]float64
	Actions               []planAdjustmentAction
	ActionSummaries       []string
	CurrentPhase          string
	EntryPhase            string
}

// planAdjustmentApplication 表示本轮诊断动作应用后的计划结果，以及需要持久化的任务解释覆盖信息。
type planAdjustmentApplication struct {
	Plan             ai.LearningPlan
	TaskExplanations map[string]planTaskResponseContext
}

// loadPlanAdjustmentDecision 读取最近诊断结果，构建可供 AdjustPlan 消费的性能分数和动作建议。
func (s *planService) loadPlanAdjustmentDecision(
	ctx context.Context,
	planID uint,
	tasks []model.LearningTask,
) (*planAdjustmentDecision, error) {
	decision := &planAdjustmentDecision{
		PerformanceByTaskType: buildDefaultPlanPerformance(tasks),
		Actions:               []planAdjustmentAction{},
		ActionSummaries:       []string{},
	}
	decision.CurrentPhase, _ = resolveCurrentPlanPhase(tasks)
	decision.EntryPhase = decision.CurrentPhase
	if s.diagnosisRepo == nil {
		return decision, nil
	}

	diagnoses, err := s.diagnosisRepo.ListRecentByPlan(ctx, planID, defaultPlanDiagnosisLoadLimit)
	if err != nil {
		return nil, err
	}
	if len(diagnoses) == 0 {
		return decision, nil
	}

	taskByID := make(map[uint]model.LearningTask, len(tasks))
	for _, task := range tasks {
		taskByID[task.ID] = task
	}

	typeScoreBuckets := make(map[string][]float64)
	for _, diagnosis := range diagnoses {
		task, exists := taskByID[diagnosis.TaskID]
		if !exists || task.Status != model.TaskStatusCompleted {
			continue
		}
		typeScoreBuckets[task.TaskType] = append(typeScoreBuckets[task.TaskType], buildDiagnosisPerformanceScore(diagnosis))
		decision.Actions = append(decision.Actions, decodePlanAdjustmentActions(diagnosis.ActionJSON, buildPlanTaskFeedbackSourceRef(diagnosis.FeedbackID))...)
		if summary := strings.TrimSpace(diagnosis.Summary); summary != "" {
			decision.ActionSummaries = append(decision.ActionSummaries, summary)
		}
	}

	for taskType, scores := range typeScoreBuckets {
		if len(scores) == 0 {
			continue
		}
		total := 0.0
		for _, score := range scores {
			total += score
		}
		decision.PerformanceByTaskType[taskType] = total / float64(len(scores))
	}

	decision.Actions = normalizePlanAdjustmentActions(decision.Actions)
	decision.ActionSummaries = sanitizeWeeklyFocusTextList(decision.ActionSummaries)
	decision.EntryPhase = resolvePlanAdjustmentEntryPhase(decision.CurrentPhase, decision.Actions)
	return decision, nil
}

// buildDefaultPlanPerformance 为已完成任务构造默认的类型表现分。
func buildDefaultPlanPerformance(tasks []model.LearningTask) map[string]float64 {
	taskTypeScores := make(map[string][]float64)
	for _, task := range tasks {
		if task.Status != model.TaskStatusCompleted {
			continue
		}
		taskTypeScores[task.TaskType] = append(taskTypeScores[task.TaskType], 80)
	}

	result := make(map[string]float64, len(taskTypeScores))
	for taskType, scores := range taskTypeScores {
		total := 0.0
		for _, score := range scores {
			total += score
		}
		result[taskType] = total / float64(len(scores))
	}
	return result
}

// buildDiagnosisPerformanceScore 将诊断状态映射为更有区分度的任务表现分。
func buildDiagnosisPerformanceScore(diagnosis model.LearningTaskDiagnosis) float64 {
	switch strings.TrimSpace(diagnosis.WeaknessStatus) {
	case model.LearningTaskDiagnosisWeaknessImproved:
		return 92
	case model.LearningTaskDiagnosisWeaknessUnresolved:
		return 58
	default:
		return 76
	}
}

// decodePlanAdjustmentActions 解析单条任务诊断中记录的动作列表。
func decodePlanAdjustmentActions(raw string, sourceRef string) []planAdjustmentAction {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	var payload []taskFeedbackDiagnosisAction
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil
	}

	actions := make([]planAdjustmentAction, 0, len(payload))
	for _, item := range payload {
		actions = append(actions, planAdjustmentAction{
			Action:         strings.TrimSpace(item.Action),
			TargetFocusTag: strings.TrimSpace(item.TargetFocusTag),
			CollectionHint: strings.TrimSpace(item.CollectionHint),
			SourceRef:      strings.TrimSpace(sourceRef),
			Reason:         strings.TrimSpace(item.Reason),
		})
	}
	return actions
}

// normalizePlanAdjustmentActions 清理空动作并做稳定去重。
func normalizePlanAdjustmentActions(actions []planAdjustmentAction) []planAdjustmentAction {
	result := make([]planAdjustmentAction, 0, len(actions))
	seen := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		action.Action = strings.TrimSpace(action.Action)
		action.TargetFocusTag = strings.TrimSpace(action.TargetFocusTag)
		action.CollectionHint = strings.TrimSpace(action.CollectionHint)
		action.SourceRef = strings.TrimSpace(action.SourceRef)
		action.Reason = strings.TrimSpace(action.Reason)
		if action.Action == "" {
			continue
		}
		key := strings.Join([]string{action.Action, action.TargetFocusTag, action.Reason}, "||")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, action)
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].TargetFocusTag == result[j].TargetFocusTag {
			return result[i].Action < result[j].Action
		}
		return result[i].TargetFocusTag < result[j].TargetFocusTag
	})
	return result
}

// applyPlanAdjustmentDecision 将诊断动作真正应用到 AI 返回的后续任务列表中。
func applyPlanAdjustmentDecision(plan ai.LearningPlan, decision *planAdjustmentDecision) planAdjustmentApplication {
	application := planAdjustmentApplication{
		Plan:             plan,
		TaskExplanations: make(map[string]planTaskResponseContext),
	}
	if decision == nil {
		return application
	}

	tasks := make([]ai.PlanTask, 0, len(plan.Tasks))
	tasks = append(tasks, plan.Tasks...)
	insertedReviews := make(map[string]struct{}, len(decision.Actions))

	for _, action := range decision.Actions {
		switch action.Action {
		case model.LearningTaskDiagnosisActionAddReviewTask:
			focusTag := normalizePlanAdjustmentFocusTag(action.TargetFocusTag)
			if focusTag == "" {
				focusTag = "本轮重点"
			}
			if _, exists := insertedReviews[focusTag]; exists {
				continue
			}
			reviewTask := newReviewPlanTask(focusTag, action.CollectionHint, action.Reason)
			tasks = append([]ai.PlanTask{reviewTask}, tasks...)
			application.TaskExplanations[buildPlanTaskLookupKey(reviewTask.Title, reviewTask.Description, reviewTask.TaskType)] =
				buildPlanAdjustmentTaskResponseContext(action, reviewTask)
			insertedReviews[focusTag] = struct{}{}
		case model.LearningTaskDiagnosisActionRepeatSamePattern:
			if updatedTask, ok := applyDiagnosisActionToFirstTrainTask(&tasks, action, "同类巩固", "本轮继续围绕“%s”做同类模式训练，优先把同类错误收敛。"); ok {
				application.TaskExplanations[buildPlanTaskLookupKey(updatedTask.Title, updatedTask.Description, updatedTask.TaskType)] =
					buildPlanAdjustmentTaskResponseContext(action, updatedTask)
			}
		case model.LearningTaskDiagnosisActionSwitchVariantPattern:
			if updatedTask, ok := applyDiagnosisActionToFirstTrainTask(&tasks, action, "变式确认", "本轮改做“%s”的变式题，确认方法是否能迁移到新题面。"); ok {
				application.TaskExplanations[buildPlanTaskLookupKey(updatedTask.Title, updatedTask.Description, updatedTask.TaskType)] =
					buildPlanAdjustmentTaskResponseContext(action, updatedTask)
			}
		case model.LearningTaskDiagnosisActionRaiseDifficulty:
			if updatedTask, ok := applyDiagnosisActionToFirstTrainTask(&tasks, action, "进阶推进", "该任务相对上一轮复杂度提升一档，重点验证“%s”是否已经稳定。"); ok {
				application.TaskExplanations[buildPlanTaskLookupKey(updatedTask.Title, updatedTask.Description, updatedTask.TaskType)] =
					buildPlanAdjustmentTaskResponseContext(action, updatedTask)
			}
		case model.LearningTaskDiagnosisActionKeepProgress:
			if updatedTask, ok := applyDiagnosisActionToFirstTrainTask(&tasks, action, "节奏保持", "该任务保持当前推进节奏，继续验证“%s”是否稳定。"); ok {
				application.TaskExplanations[buildPlanTaskLookupKey(updatedTask.Title, updatedTask.Description, updatedTask.TaskType)] =
					buildPlanAdjustmentTaskResponseContext(action, updatedTask)
			}
		}
	}

	applyPlanAdjustmentEntryPhase(tasks, decision)
	renumberPlanTasks(tasks)
	application.Plan.Tasks = tasks
	if len(decision.ActionSummaries) > 0 {
		application.Plan.Description = mergePlanDescriptionWithAdjustmentSummary(plan.Description, decision.ActionSummaries)
	}
	return application
}

// resolvePlanAdjustmentEntryPhase 根据当前阶段和诊断动作决定本轮调计划应从哪个阶段重新切入。
func resolvePlanAdjustmentEntryPhase(currentPhase string, actions []planAdjustmentAction) string {
	currentPhase = model.NormalizeLearningPhase(currentPhase)
	if currentPhase == "" {
		currentPhase = model.LearningPhaseFoundation
	}

	for _, action := range actions {
		if action.Action == model.LearningTaskDiagnosisActionAddReviewTask {
			return model.LearningPhaseReview
		}
	}
	for _, action := range actions {
		if action.Action == model.LearningTaskDiagnosisActionRepeatSamePattern || action.Action == model.LearningTaskDiagnosisActionSwitchVariantPattern {
			if currentPhase == model.LearningPhaseFoundation {
				return model.LearningPhaseFoundation
			}
			return model.LearningPhaseDrill
		}
	}
	for _, action := range actions {
		if action.Action == model.LearningTaskDiagnosisActionRaiseDifficulty || action.Action == model.LearningTaskDiagnosisActionKeepProgress {
			return resolveNextLearningPhase(currentPhase)
		}
	}
	return currentPhase
}

// resolveNextLearningPhase 返回当前阶段在轻量计划主线中的下一个阶段。
func resolveNextLearningPhase(currentPhase string) string {
	switch model.NormalizeLearningPhase(currentPhase) {
	case model.LearningPhaseFoundation:
		return model.LearningPhaseDrill
	case model.LearningPhaseReview:
		return model.LearningPhaseDrill
	case model.LearningPhaseDrill:
		return model.LearningPhaseMock
	default:
		return model.LearningPhaseMock
	}
}

// mergePlanDescriptionWithAdjustmentSummary 将诊断调整摘要并入计划描述，方便后续查看本轮收口点。
func mergePlanDescriptionWithAdjustmentSummary(description string, summaries []string) string {
	description = strings.TrimSpace(description)
	summaries = sanitizeWeeklyFocusTextList(summaries)
	if len(summaries) == 0 {
		return description
	}

	summaryText := "本轮调整重点：" + strings.Join(summaries, "；")
	if description == "" {
		return summaryText
	}
	if strings.Contains(description, summaryText) {
		return description
	}
	return description + " " + summaryText
}

// applyDiagnosisActionToFirstTrainTask 将诊断动作文案应用到第一条适合继续训练的任务上。
func applyDiagnosisActionToFirstTrainTask(tasks *[]ai.PlanTask, action planAdjustmentAction, titlePrefix string, template string) (ai.PlanTask, bool) {
	if tasks == nil || len(*tasks) == 0 {
		return ai.PlanTask{}, false
	}

	focusTag := normalizePlanAdjustmentFocusTag(action.TargetFocusTag)
	for index, task := range *tasks {
		if strings.TrimSpace(task.TaskType) != model.TaskTypePractice && strings.TrimSpace(task.TaskType) != model.TaskTypeInterview && strings.TrimSpace(task.TaskType) != model.TaskTypeStudy {
			continue
		}
		task.Title = strings.TrimSpace(task.Title)
		task.Description = strings.TrimSpace(task.Description)
		if titlePrefix != "" && !strings.Contains(task.Title, titlePrefix) {
			task.Title = titlePrefix + "：" + task.Title
		}

		extra := strings.TrimSpace(action.Reason)
		if focusTag != "" {
			extra = fmt.Sprintf(template, focusTag)
			if strings.TrimSpace(action.Reason) != "" {
				extra = extra + " " + strings.TrimSpace(action.Reason)
			}
		}
		task.Description = mergePlanTaskDescription(task.Description, extra)
		task.Priority = "high"
		(*tasks)[index] = task
		return task, true
	}
	return ai.PlanTask{}, false
}

// applyPlanAdjustmentEntryPhase 将本轮入口阶段落到首个后续任务上，确保 AdjustPlan 真正体现留段或晋段决策。
func applyPlanAdjustmentEntryPhase(tasks []ai.PlanTask, decision *planAdjustmentDecision) {
	if len(tasks) == 0 || decision == nil {
		return
	}

	entryPhase := model.NormalizeLearningPhase(decision.EntryPhase)
	if entryPhase == "" {
		return
	}

	for index := range tasks {
		if !isPlanAdjustmentEntryCandidate(tasks[index], entryPhase) {
			continue
		}
		tasks[index].Phase = entryPhase
		tasks[index].PhaseGoal = model.BuildLearningPhaseGoal(entryPhase)
		return
	}
}

// isPlanAdjustmentEntryCandidate 判断一条任务是否适合作为本轮阶段入口任务。
func isPlanAdjustmentEntryCandidate(task ai.PlanTask, entryPhase string) bool {
	taskType := strings.TrimSpace(task.TaskType)
	if entryPhase == model.LearningPhaseReview {
		return taskType == model.TaskTypeReview
	}
	return taskType == model.TaskTypeStudy || taskType == model.TaskTypePractice || taskType == model.TaskTypeInterview
}

// mergePlanTaskDescription 将诊断动作说明附加到任务描述末尾。
func mergePlanTaskDescription(description string, extra string) string {
	description = strings.TrimSpace(description)
	extra = strings.TrimSpace(extra)
	if extra == "" {
		return description
	}
	if description == "" {
		return extra
	}
	if strings.Contains(description, extra) {
		return description
	}
	return description + " " + extra
}

// normalizePlanAdjustmentFocusTag 清理诊断动作中的焦点标签。
func normalizePlanAdjustmentFocusTag(tag string) string {
	return strings.TrimSpace(tag)
}

// buildPlanAdjustmentTaskResponseContext 为诊断驱动的新任务生成稳定的解释字段，便于前端直接承接动作来源。
func buildPlanAdjustmentTaskResponseContext(action planAdjustmentAction, task ai.PlanTask) planTaskResponseContext {
	source := "plan_feedback_diagnosis"
	return planTaskResponseContext{
		Source:              source,
		SourceLabel:         "训练反馈诊断",
		Reason:              buildPlanAdjustmentTaskReason(action, task),
		PriorityExplanation: buildPlanTaskPriorityExplanation(task.Priority, source),
		SourceRef:           strings.TrimSpace(action.SourceRef),
		CollectionHint:      strings.TrimSpace(action.CollectionHint),
	}
}

// buildPlanAdjustmentTaskReason 根据诊断动作和任务形态生成更贴近用户语义的任务来源说明。
func buildPlanAdjustmentTaskReason(action planAdjustmentAction, task ai.PlanTask) string {
	focusTag := normalizePlanAdjustmentFocusTag(action.TargetFocusTag)
	reason := strings.TrimSpace(action.Reason)
	baseReason := "该任务来自最近一次训练反馈诊断，用于把上轮暴露出的真实问题继续收口。"

	switch action.Action {
	case model.LearningTaskDiagnosisActionAddReviewTask:
		if focusTag != "" {
			baseReason = fmt.Sprintf("该任务来自最近一次训练反馈诊断，建议先围绕“%s”做短复盘，再进入后续训练。", focusTag)
		}
	case model.LearningTaskDiagnosisActionRepeatSamePattern:
		if focusTag != "" {
			baseReason = fmt.Sprintf("该任务来自最近一次训练反馈诊断，当前“%s”仍未稳定，需要继续做同类巩固。", focusTag)
		}
	case model.LearningTaskDiagnosisActionSwitchVariantPattern:
		if focusTag != "" {
			baseReason = fmt.Sprintf("该任务来自最近一次训练反馈诊断，建议围绕“%s”切到变式题继续确认迁移能力。", focusTag)
		}
	case model.LearningTaskDiagnosisActionRaiseDifficulty:
		if focusTag != "" {
			baseReason = fmt.Sprintf("该任务来自最近一次训练反馈诊断，说明“%s”已相对稳定，可以继续升一档推进。", focusTag)
		}
	case model.LearningTaskDiagnosisActionKeepProgress:
		if focusTag != "" {
			baseReason = fmt.Sprintf("该任务来自最近一次训练反馈诊断，建议围绕“%s”保持当前推进节奏继续验证。", focusTag)
		}
	}

	if strings.TrimSpace(task.TaskType) == model.TaskTypeReview && focusTag != "" && action.Action == model.LearningTaskDiagnosisActionAddReviewTask {
		baseReason = fmt.Sprintf("该任务来自最近一次训练反馈诊断，先复盘“%s”的错误步骤和正确做法，再进入后续训练。", focusTag)
	}
	if reason == "" || strings.Contains(baseReason, reason) {
		return baseReason
	}
	return baseReason + " " + reason
}

// renumberPlanTasks 在插入复盘任务后重新整理 day_number，保持计划顺序稳定。
func renumberPlanTasks(tasks []ai.PlanTask) {
	for index := range tasks {
		tasks[index].DayNumber = index + 1
	}
}

// newReviewPlanTask 构造一条诊断驱动的复盘任务。
func newReviewPlanTask(focusTag string, collectionHint string, reason string) ai.PlanTask {
	description := fmt.Sprintf("先复盘最近在“%s”上的错误步骤和正确做法，再进入后续训练。", focusTag)
	if strings.TrimSpace(collectionHint) != "" {
		description = description + fmt.Sprintf(" 可优先围绕“%s”相关题单或专题回看。", strings.TrimSpace(collectionHint))
	}
	if strings.TrimSpace(reason) != "" {
		description = description + " " + strings.TrimSpace(reason)
	}
	return ai.PlanTask{
		Title:       "复盘：" + focusTag,
		Description: description,
		TaskType:    model.TaskTypeReview,
		DayNumber:   1,
		Duration:    20,
		Priority:    "high",
	}
}
