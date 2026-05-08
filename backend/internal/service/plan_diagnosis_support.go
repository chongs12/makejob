// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"makejob-backend/internal/model"
	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
)

const taskFeedbackDiagnosisTimeout = 15 * time.Second

// taskFeedbackDiagnosisAction 表示供后续计划调整消费的一条结构化建议动作。
type taskFeedbackDiagnosisAction struct {
	Action         string `json:"action"`
	TargetFocusTag string `json:"target_focus_tag,omitempty"`
	CollectionHint string `json:"collection_hint,omitempty"`
	Reason         string `json:"reason"`
}

// taskFeedbackDiagnosisResult 表示训练反馈诊断后的聚合结果。
type taskFeedbackDiagnosisResult struct {
	WeaknessStatus string                        `json:"weakness_status"`
	MistakeTags    []string                      `json:"mistake_tags"`
	StrengthTags   []string                      `json:"strength_tags"`
	Suggestions    []string                      `json:"suggestions"`
	Evidence       []string                      `json:"evidence"`
	Summary        string                        `json:"summary"`
	Actions        []taskFeedbackDiagnosisAction `json:"actions"`
}

// taskFeedbackWrongAnswerPayload 表示训练反馈中可选的错误答案扩展载荷。
type taskFeedbackWrongAnswerPayload struct {
	Code            string   `json:"code"`
	Language        string   `json:"language"`
	Answer          string   `json:"answer"`
	WrongOptions    []string `json:"wrong_options"`
	SelectedOptions []string `json:"selected_options"`
	Excerpt         string   `json:"excerpt"`
	Note            string   `json:"note"`
}

// taskFeedbackDiagnosisContext 表示本次训练反馈诊断时可消费的计划阶段上下文。
type taskFeedbackDiagnosisContext struct {
	PlanPhase     string
	PlanPhaseGoal string
	EntryPhase    string
	TaskPhase     string
	TaskPhaseGoal string
}

// diagnosisAsyncResult 用于在异步诊断协程内部传递结果与错误。
type diagnosisAsyncResult struct {
	result *taskFeedbackDiagnosisResult
	err    error
}

// enqueueTaskFeedbackDiagnosis 异步生成训练反馈诊断，避免阻塞提交接口返回。
func (s *planService) enqueueTaskFeedbackDiagnosis(plan *model.LearningPlan, task *model.LearningTask, feedback *model.LearningTaskFeedback) {
	if plan == nil || task == nil || feedback == nil {
		return
	}
	if s.learningArchiveRepo == nil && s.diagnosisRepo == nil {
		return
	}

	planSnapshot := *plan
	taskSnapshot := *task
	feedbackSnapshot := *feedback

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), taskFeedbackDiagnosisTimeout)
		defer cancel()

		resultCh := make(chan diagnosisAsyncResult, 1)
		go func() {
			result, err := s.buildTaskFeedbackDiagnosis(ctx, &planSnapshot, &taskSnapshot, &feedbackSnapshot)
			resultCh <- diagnosisAsyncResult{result: result, err: err}
		}()

		select {
		case outcome := <-resultCh:
			if outcome.err != nil {
				applogger.Warn(
					"学习任务反馈异步诊断失败",
					zap.Uint("plan_id", planSnapshot.ID),
					zap.Uint("task_id", taskSnapshot.ID),
					zap.Uint("feedback_id", feedbackSnapshot.ID),
					zap.Error(outcome.err),
				)
				return
			}
			if err := s.persistTaskFeedbackDiagnosis(ctx, &planSnapshot, &taskSnapshot, &feedbackSnapshot, outcome.result); err != nil {
				applogger.Warn(
					"学习任务反馈诊断结果持久化失败",
					zap.Uint("plan_id", planSnapshot.ID),
					zap.Uint("task_id", taskSnapshot.ID),
					zap.Uint("feedback_id", feedbackSnapshot.ID),
					zap.Error(err),
				)
			}
		case <-ctx.Done():
			applogger.Warn(
				"学习任务反馈异步诊断超时",
				zap.Uint("plan_id", planSnapshot.ID),
				zap.Uint("task_id", taskSnapshot.ID),
				zap.Uint("feedback_id", feedbackSnapshot.ID),
			)
		}
	}()
}

// buildTaskFeedbackDiagnosis 基于训练反馈、任务信息和可选AI分析构建结构化诊断结果。
func (s *planService) buildTaskFeedbackDiagnosis(
	ctx context.Context,
	plan *model.LearningPlan,
	task *model.LearningTask,
	feedback *model.LearningTaskFeedback,
) (*taskFeedbackDiagnosisResult, error) {
	if plan == nil || task == nil || feedback == nil {
		return nil, fmt.Errorf("训练反馈诊断输入不能为空")
	}

	diagnosisContext := buildTaskFeedbackDiagnosisContext(plan, task)
	result := buildLocalTaskFeedbackDiagnosis(task, feedback, diagnosisContext)
	if err := s.enrichTaskFeedbackDiagnosisWithAnalyzer(ctx, task, feedback, diagnosisContext, result); err != nil {
		applogger.Warn(
			"学习任务反馈AI诊断补充失败，已回退本地诊断",
			zap.Uint("plan_id", plan.ID),
			zap.Uint("task_id", task.ID),
			zap.Uint("feedback_id", feedback.ID),
			zap.Error(err),
		)
	}

	result.MistakeTags = sanitizeWeeklyFocusTextList(result.MistakeTags)
	result.StrengthTags = sanitizeWeeklyFocusTextList(result.StrengthTags)
	result.Suggestions = sanitizeWeeklyFocusTextList(result.Suggestions)
	result.Evidence = sanitizeWeeklyFocusTextList(result.Evidence)
	result.Actions = normalizeTaskFeedbackDiagnosisActions(result.Actions)
	result.Summary = strings.TrimSpace(result.Summary)
	if result.Summary == "" {
		result.Summary = buildTaskFeedbackDiagnosisSummary(task, feedback, diagnosisContext, result)
	}
	return result, nil
}

// buildLocalTaskFeedbackDiagnosis 根据反馈中的显式信号生成第一版本地诊断结果。
func buildLocalTaskFeedbackDiagnosis(
	task *model.LearningTask,
	feedback *model.LearningTaskFeedback,
	diagnosisContext taskFeedbackDiagnosisContext,
) *taskFeedbackDiagnosisResult {
	result := &taskFeedbackDiagnosisResult{
		WeaknessStatus: model.LearningTaskDiagnosisWeaknessPartial,
		MistakeTags:    decodeWeeklyFocusTextList(feedback.MistakeTagsJSON),
		StrengthTags:   []string{},
		Suggestions:    []string{},
		Evidence:       []string{},
		Actions:        []taskFeedbackDiagnosisAction{},
	}

	targetFocusTag := firstDiagnosisFocusTag(result.MistakeTags)
	collectionHint := resolveMistakeTopicCodeByTag(targetFocusTag)
	if targetFocusTag == "" {
		targetFocusTag = strings.TrimSpace(task.Title)
	}

	if feedback.AttemptCount <= 1 && feedback.DifficultySelfAssessment == model.DifficultyTooEasy {
		result.WeaknessStatus = model.LearningTaskDiagnosisWeaknessImproved
		result.StrengthTags = appendUniqueStrings(result.StrengthTags, "同类任务通过较快")
		result.Suggestions = appendUniqueStrings(result.Suggestions, "下一道同主题任务可以提升一档复杂度，直接验证能力是否真正稳定。")
		result.Actions = append(result.Actions, taskFeedbackDiagnosisAction{
			Action:         model.LearningTaskDiagnosisActionRaiseDifficulty,
			TargetFocusTag: targetFocusTag,
			CollectionHint: collectionHint,
			Reason:         "本次任务几乎无反复且主观难度偏低，可以直接推进下一档训练。",
		})
	}

	if feedback.AttemptCount >= 3 || feedback.DifficultySelfAssessment == model.DifficultyTooHard {
		result.WeaknessStatus = model.LearningTaskDiagnosisWeaknessUnresolved
		result.Suggestions = appendUniqueStrings(result.Suggestions,
			"下一轮先保留同主题训练，不要直接升难度。",
			"先做一次简短复盘，把错误步骤和正确解法写成固定模板后再做下一题。",
		)
		result.Actions = append(result.Actions,
			taskFeedbackDiagnosisAction{
				Action:         model.LearningTaskDiagnosisActionRepeatSamePattern,
				TargetFocusTag: targetFocusTag,
				CollectionHint: collectionHint,
				Reason:         "当前任务中仍有明显反复，说明同类问题尚未真正收敛。",
			},
			taskFeedbackDiagnosisAction{
				Action:         model.LearningTaskDiagnosisActionAddReviewTask,
				TargetFocusTag: targetFocusTag,
				CollectionHint: collectionHint,
				Reason:         "需要先补一轮复盘，再进入下一次同类训练。",
			},
		)
	}

	if result.WeaknessStatus == model.LearningTaskDiagnosisWeaknessPartial {
		result.Suggestions = appendUniqueStrings(result.Suggestions, "下一轮可换一道同主题变形题，确认方法是否能迁移。")
		result.Actions = append(result.Actions, taskFeedbackDiagnosisAction{
			Action:         model.LearningTaskDiagnosisActionSwitchVariantPattern,
			TargetFocusTag: targetFocusTag,
			CollectionHint: collectionHint,
			Reason:         "当前任务已完成，但还需要用变式题确认方法是否真正内化。",
		})
	}

	if feedback.TimeSpentSeconds >= 30*60 {
		result.Evidence = appendUniqueStrings(result.Evidence, fmt.Sprintf("本次任务耗时 %d 分钟，明显高于轻松完成区间。", feedback.TimeSpentSeconds/60))
		result.Suggestions = appendUniqueStrings(result.Suggestions, "建议先把解题步骤压缩成 3 到 5 个固定检查点，下次先按检查点推进。")
		if result.WeaknessStatus != model.LearningTaskDiagnosisWeaknessImproved {
			result.Actions = append(result.Actions, taskFeedbackDiagnosisAction{
				Action:         model.LearningTaskDiagnosisActionAddReviewTask,
				TargetFocusTag: targetFocusTag,
				CollectionHint: collectionHint,
				Reason:         "耗时偏长，说明方法链路还不够稳定，适合插入一轮短复盘。",
			})
		}
	}

	result.Evidence = appendUniqueStrings(result.Evidence, buildTaskFeedbackEvidenceLines(feedback, diagnosisContext)...)
	result.Summary = buildTaskFeedbackDiagnosisSummary(task, feedback, diagnosisContext, result)
	return result
}

// enrichTaskFeedbackDiagnosisWithAnalyzer 在可用时调用题目分析器补齐错因、建议与证据。
func (s *planService) enrichTaskFeedbackDiagnosisWithAnalyzer(
	ctx context.Context,
	task *model.LearningTask,
	feedback *model.LearningTaskFeedback,
	diagnosisContext taskFeedbackDiagnosisContext,
	result *taskFeedbackDiagnosisResult,
) error {
	if s.quizAnalyzer == nil || task == nil || feedback == nil || result == nil {
		return nil
	}
	if feedback.TrainingType != model.TrainingTypeCoding {
		return nil
	}

	payload := decodeTaskFeedbackWrongAnswerPayload(feedback.WrongAnswerJSON)
	code := strings.TrimSpace(payload.Code)
	if code == "" {
		return nil
	}

	analysis, err := s.quizAnalyzer.AnalyzeCode(ctx, code, defaultInterviewCodeLanguage(payload.Language), buildTaskFeedbackAnalyzerQuestion(task, diagnosisContext))
	if err != nil {
		return err
	}

	result.MistakeTags = appendUniqueStrings(result.MistakeTags, analysis.MistakeTags...)
	result.StrengthTags = appendUniqueStrings(result.StrengthTags, analysis.StrengthTags...)
	result.Suggestions = appendUniqueStrings(result.Suggestions, analysis.Improvements...)
	result.Evidence = appendUniqueStrings(result.Evidence, analysis.Issues...)
	if analysis.IsCorrect && result.WeaknessStatus == model.LearningTaskDiagnosisWeaknessPartial && feedback.AttemptCount <= 1 {
		result.WeaknessStatus = model.LearningTaskDiagnosisWeaknessImproved
		result.Actions = append(result.Actions, taskFeedbackDiagnosisAction{
			Action:         model.LearningTaskDiagnosisActionKeepProgress,
			TargetFocusTag: firstDiagnosisFocusTag(result.MistakeTags),
			CollectionHint: resolveMistakeTopicCodeByTag(firstDiagnosisFocusTag(result.MistakeTags)),
			Reason:         "代码分析显示当前任务实现已经基本稳定，可以保持当前推进节奏。",
		})
	}
	if strings.TrimSpace(analysis.Feedback) != "" {
		result.Evidence = appendUniqueStrings(result.Evidence, strings.TrimSpace(analysis.Feedback))
	}
	return nil
}

// persistTaskFeedbackDiagnosis 持久化诊断结果，并同步写入学习档案供成长页和计划聚焦复用。
func (s *planService) persistTaskFeedbackDiagnosis(
	ctx context.Context,
	plan *model.LearningPlan,
	task *model.LearningTask,
	feedback *model.LearningTaskFeedback,
	result *taskFeedbackDiagnosisResult,
) error {
	if plan == nil || task == nil || feedback == nil || result == nil {
		return fmt.Errorf("训练反馈诊断持久化输入不能为空")
	}

	industryCode := strings.TrimSpace(readPlanStoredContext(plan.PlanJSON).IndustryCode)
	if industryCode == "" {
		industryCode = s.resolvePlanIndustryCode(ctx, plan.IndustryID)
	}
	diagnosisContext := buildTaskFeedbackDiagnosisContext(plan, task)

	mistakeTagsJSON, err := marshalStringSlice(result.MistakeTags)
	if err != nil {
		return fmt.Errorf("序列化诊断错因标签失败: %w", err)
	}
	strengthTagsJSON, err := marshalStringSlice(result.StrengthTags)
	if err != nil {
		return fmt.Errorf("序列化诊断优势标签失败: %w", err)
	}
	suggestionsJSON, err := marshalStringSlice(result.Suggestions)
	if err != nil {
		return fmt.Errorf("序列化诊断建议失败: %w", err)
	}
	evidenceJSON, err := marshalStringSlice(result.Evidence)
	if err != nil {
		return fmt.Errorf("序列化诊断证据失败: %w", err)
	}
	actionJSON, err := json.Marshal(result.Actions)
	if err != nil {
		return fmt.Errorf("序列化诊断动作失败: %w", err)
	}

	occurredAt := buildTaskFeedbackOccurredAt(feedback.CreatedAt)
	if s.diagnosisRepo != nil {
		diagnosis := &model.LearningTaskDiagnosis{
			FeedbackID:       feedback.ID,
			PlanID:           plan.ID,
			TaskID:           task.ID,
			UserID:           feedback.UserID,
			IndustryCode:     industryCode,
			PlanPhase:        diagnosisContext.PlanPhase,
			PlanPhaseGoal:    diagnosisContext.PlanPhaseGoal,
			EntryPhase:       diagnosisContext.EntryPhase,
			TaskPhase:        diagnosisContext.TaskPhase,
			TaskPhaseGoal:    diagnosisContext.TaskPhaseGoal,
			WeaknessStatus:   result.WeaknessStatus,
			ActionJSON:       string(actionJSON),
			MistakeTagsJSON:  mistakeTagsJSON,
			StrengthTagsJSON: strengthTagsJSON,
			SuggestionsJSON:  suggestionsJSON,
			EvidenceJSON:     evidenceJSON,
			Summary:          strings.TrimSpace(result.Summary),
			OccurredAt:       occurredAt,
		}
		if err := s.diagnosisRepo.Upsert(ctx, diagnosis); err != nil {
			return err
		}
	}

	if s.learningArchiveRepo != nil && len(result.MistakeTags) > 0 {
		archiveEntry := &model.LearningArchiveEntry{
			UserID:           feedback.UserID,
			SourceType:       model.LearningArchiveSourcePlanTaskFeedback,
			SourceRef:        buildPlanTaskFeedbackSourceRef(feedback.ID),
			QuestionIndex:    0,
			IndustryCode:     industryCode,
			PlanPhase:        diagnosisContext.PlanPhase,
			PlanPhaseGoal:    diagnosisContext.PlanPhaseGoal,
			EntryPhase:       diagnosisContext.EntryPhase,
			TaskPhase:        diagnosisContext.TaskPhase,
			TaskPhaseGoal:    diagnosisContext.TaskPhaseGoal,
			Language:         detectTaskFeedbackLanguage(feedback),
			MistakeTagsJSON:  mistakeTagsJSON,
			StrengthTagsJSON: strengthTagsJSON,
			SuggestionsJSON:  suggestionsJSON,
			EvidenceSummary:  strings.Join(result.Evidence, "；"),
			OccurredAt:       occurredAt,
		}
		if err := s.learningArchiveRepo.Upsert(ctx, archiveEntry); err != nil {
			return err
		}
	}

	return nil
}

// buildTaskFeedbackOccurredAt 将反馈创建时间转换为可持久化的发生时间指针。
func buildTaskFeedbackOccurredAt(createdAt time.Time) *time.Time {
	if createdAt.IsZero() {
		now := time.Now()
		return &now
	}
	occurredAt := createdAt
	return &occurredAt
}

// buildPlanTaskFeedbackSourceRef 为训练反馈诊断生成稳定的学习档案来源标识。
func buildPlanTaskFeedbackSourceRef(feedbackID uint) string {
	return fmt.Sprintf("plan-feedback:%d", feedbackID)
}

// buildTaskFeedbackDiagnosisContext 归一化训练反馈诊断时需要使用的计划阶段上下文。
func buildTaskFeedbackDiagnosisContext(plan *model.LearningPlan, task *model.LearningTask) taskFeedbackDiagnosisContext {
	context := taskFeedbackDiagnosisContext{}
	storedContext := planStoredContext{}
	if plan != nil {
		storedContext = readPlanStoredContext(plan.PlanJSON)
	}

	planFallbackPhase := ""
	if task != nil {
		if trimmedTaskPhase := strings.TrimSpace(task.Phase); trimmedTaskPhase != "" {
			planFallbackPhase = trimmedTaskPhase
		} else {
			planFallbackPhase = model.ResolveLearningPhaseFromTaskType(task.TaskType)
		}
	}
	context.PlanPhase = resolveTaskFeedbackDiagnosisPhaseValue(strings.TrimSpace(planPhaseValue(plan)), planFallbackPhase, nil, nil)
	context.PlanPhaseGoal = resolveTaskFeedbackDiagnosisPhaseGoal(context.PlanPhase, "", plan, task, false)
	context.EntryPhase = resolveTaskFeedbackDiagnosisEntryPhase(storedContext.EntryPhase, context.PlanPhase)
	taskFallbackPhase := strings.TrimSpace(planPhaseValue(plan))
	context.TaskPhase = resolveTaskFeedbackDiagnosisPhaseValue(taskPhaseValue(task), taskFallbackPhase, nil, nil)
	context.TaskPhaseGoal = resolveTaskFeedbackDiagnosisPhaseGoal(context.TaskPhase, "", nil, task, true)
	if strings.TrimSpace(context.TaskPhase) == "" {
		context.TaskPhase = context.PlanPhase
		context.TaskPhaseGoal = context.PlanPhaseGoal
	}
	return context
}

// planPhaseValue 安全读取计划阶段字段，避免空指针判断散落在诊断上下文组装逻辑中。
func planPhaseValue(plan *model.LearningPlan) string {
	if plan == nil {
		return ""
	}
	return plan.Phase
}

// taskPhaseValue 安全读取任务阶段字段，避免空指针判断散落在诊断上下文组装逻辑中。
func taskPhaseValue(task *model.LearningTask) string {
	if task == nil {
		return ""
	}
	return task.Phase
}

// resolveTaskFeedbackDiagnosisPhaseValue 归一化计划或任务上的学习阶段字段，缺失时按任务类型兜底。
func resolveTaskFeedbackDiagnosisPhaseValue(
	explicitPhase string,
	fallbackPhase string,
	plan *model.LearningPlan,
	task *model.LearningTask,
) string {
	candidates := []string{explicitPhase}
	if task != nil {
		candidates = append(candidates, task.Phase)
	}
	if plan != nil {
		candidates = append(candidates, plan.Phase)
	}
	candidates = append(candidates, fallbackPhase)
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		return model.NormalizeLearningPhase(trimmed)
	}
	if task != nil {
		return model.ResolveLearningPhaseFromTaskType(task.TaskType)
	}
	return model.LearningPhaseFoundation
}

// resolveTaskFeedbackDiagnosisPhaseGoal 为阶段字段补齐稳定的阶段目标文案。
func resolveTaskFeedbackDiagnosisPhaseGoal(
	phase string,
	explicitGoal string,
	plan *model.LearningPlan,
	task *model.LearningTask,
	preferTask bool,
) string {
	goalCandidates := []string{explicitGoal}
	if preferTask && task != nil {
		goalCandidates = append(goalCandidates, task.PhaseGoal)
	}
	if !preferTask && plan != nil {
		goalCandidates = append(goalCandidates, plan.PhaseGoal)
	}
	if !preferTask && task != nil {
		goalCandidates = append(goalCandidates, task.PhaseGoal)
	}
	if preferTask && plan != nil {
		goalCandidates = append(goalCandidates, plan.PhaseGoal)
	}
	for _, goal := range goalCandidates {
		trimmed := strings.TrimSpace(goal)
		if trimmed != "" {
			return trimmed
		}
	}
	return model.BuildLearningPhaseGoal(phase)
}

// resolveTaskFeedbackDiagnosisEntryPhase 归一化本轮计划入口阶段，缺失时回退到当前计划阶段。
func resolveTaskFeedbackDiagnosisEntryPhase(entryPhase string, planPhase string) string {
	if trimmed := strings.TrimSpace(entryPhase); trimmed != "" {
		return model.NormalizeLearningPhase(trimmed)
	}
	if trimmed := strings.TrimSpace(planPhase); trimmed != "" {
		return model.NormalizeLearningPhase(trimmed)
	}
	return model.LearningPhaseFoundation
}

// buildTaskFeedbackAnalyzerQuestion 构造供代码分析器使用的简要题面描述。
func buildTaskFeedbackAnalyzerQuestion(task *model.LearningTask, diagnosisContext taskFeedbackDiagnosisContext) string {
	if task == nil {
		return ""
	}
	parts := []string{
		strings.TrimSpace(task.Title),
		strings.TrimSpace(task.Description),
	}
	if phaseLabel := formatTaskFeedbackDiagnosisPhaseLabel(diagnosisContext.TaskPhase); phaseLabel != "" {
		parts = append(parts, "当前训练阶段："+phaseLabel)
	}
	if goal := strings.TrimSpace(diagnosisContext.TaskPhaseGoal); goal != "" {
		parts = append(parts, "阶段目标："+goal)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// decodeTaskFeedbackWrongAnswerPayload 解析训练反馈中的错误答案扩展载荷。
func decodeTaskFeedbackWrongAnswerPayload(raw string) taskFeedbackWrongAnswerPayload {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return taskFeedbackWrongAnswerPayload{}
	}

	var payload taskFeedbackWrongAnswerPayload
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		payload.Excerpt = trimmed
	}
	return payload
}

// detectTaskFeedbackLanguage 推断训练反馈对应的语言字段，供学习档案与后续检索使用。
func detectTaskFeedbackLanguage(feedback *model.LearningTaskFeedback) string {
	if feedback == nil {
		return ""
	}
	payload := decodeTaskFeedbackWrongAnswerPayload(feedback.WrongAnswerJSON)
	return strings.TrimSpace(payload.Language)
}

// buildTaskFeedbackEvidenceLines 把显式反馈字段和阶段上下文转换为更易读的诊断证据。
func buildTaskFeedbackEvidenceLines(
	feedback *model.LearningTaskFeedback,
	diagnosisContext taskFeedbackDiagnosisContext,
) []string {
	if feedback == nil {
		return nil
	}

	lines := make([]string, 0, 6)
	if phaseLabel := formatTaskFeedbackDiagnosisPhaseLabel(diagnosisContext.TaskPhase); phaseLabel != "" {
		lines = append(lines, fmt.Sprintf("本次反馈对应%s阶段任务。", phaseLabel))
	}
	if entryPhaseLabel := formatTaskFeedbackDiagnosisPhaseLabel(diagnosisContext.EntryPhase); entryPhaseLabel != "" {
		lines = append(lines, fmt.Sprintf("本轮计划入口阶段为%s。", entryPhaseLabel))
	}
	if feedback.AttemptCount > 0 {
		lines = append(lines, fmt.Sprintf("本次任务共尝试 %d 次。", feedback.AttemptCount))
	}
	if feedback.TimeSpentSeconds > 0 {
		lines = append(lines, fmt.Sprintf("本次任务耗时 %d 分钟。", feedback.TimeSpentSeconds/60))
	}
	if difficulty := strings.TrimSpace(feedback.DifficultySelfAssessment); difficulty != "" {
		lines = append(lines, fmt.Sprintf("用户自评难度为 %s。", difficulty))
	}
	if summary := strings.TrimSpace(feedback.Summary); summary != "" {
		lines = append(lines, "用户补充说明："+summary)
	}
	return lines
}

// buildTaskFeedbackDiagnosisSummary 生成单条任务诊断的简洁摘要。
func buildTaskFeedbackDiagnosisSummary(
	task *model.LearningTask,
	feedback *model.LearningTaskFeedback,
	diagnosisContext taskFeedbackDiagnosisContext,
	result *taskFeedbackDiagnosisResult,
) string {
	title := ""
	if task != nil {
		title = strings.TrimSpace(task.Title)
	}
	if title == "" {
		title = "当前任务"
	}
	tag := firstDiagnosisFocusTag(result.MistakeTags)
	phaseLabel := formatTaskFeedbackDiagnosisPhaseLabel(diagnosisContext.TaskPhase)

	switch result.WeaknessStatus {
	case model.LearningTaskDiagnosisWeaknessImproved:
		if phaseLabel != "" && diagnosisContext.TaskPhase == model.LearningPhaseReview {
			if tag != "" {
				return fmt.Sprintf("%s 在%s对“%s”已经明显收敛，可以回到后续训练继续验证。", title, phaseLabel, tag)
			}
			return fmt.Sprintf("%s 在%s已经明显收敛，可以回到后续训练继续验证。", title, phaseLabel)
		}
		if phaseLabel != "" && diagnosisContext.TaskPhase == model.LearningPhaseMock {
			if tag != "" {
				return fmt.Sprintf("%s 在%s对“%s”已经较稳定，后续可以继续保持验证强度。", title, phaseLabel, tag)
			}
			return fmt.Sprintf("%s 在%s已经较稳定，后续可以继续保持验证强度。", title, phaseLabel)
		}
		if phaseLabel != "" && diagnosisContext.TaskPhase == model.LearningPhaseFoundation {
			if tag != "" {
				return fmt.Sprintf("%s 在%s对“%s”已经较稳定，可以继续推进到下一段训练。", title, phaseLabel, tag)
			}
			return fmt.Sprintf("%s 在%s已经较稳定，可以继续推进到下一段训练。", title, phaseLabel)
		}
		if tag != "" {
			return fmt.Sprintf("%s 在“%s”上的表现已较稳定，后续可以推进更高一档的训练。", title, tag)
		}
		return fmt.Sprintf("%s 当前表现较稳定，后续可以继续向下一档推进。", title)
	case model.LearningTaskDiagnosisWeaknessUnresolved:
		if phaseLabel != "" && diagnosisContext.TaskPhase == model.LearningPhaseMock {
			if tag != "" {
				return fmt.Sprintf("%s 在%s的“%s”仍不稳定，建议先回到复盘纠偏和同类巩固。", title, phaseLabel, tag)
			}
			return fmt.Sprintf("%s 在%s仍不稳定，建议先回到复盘纠偏和同类巩固。", title, phaseLabel)
		}
		if phaseLabel != "" && diagnosisContext.TaskPhase == model.LearningPhaseReview {
			if tag != "" {
				return fmt.Sprintf("%s 在%s对“%s”仍有明显反复，说明这轮复盘还需要继续收口。", title, phaseLabel, tag)
			}
			return fmt.Sprintf("%s 在%s仍有明显反复，说明这轮复盘还需要继续收口。", title, phaseLabel)
		}
		if phaseLabel != "" && diagnosisContext.TaskPhase == model.LearningPhaseFoundation {
			if tag != "" {
				return fmt.Sprintf("%s 在%s对“%s”仍不稳定，建议先补齐基础方法再继续推进。", title, phaseLabel, tag)
			}
			return fmt.Sprintf("%s 在%s仍不稳定，建议先补齐基础方法再继续推进。", title, phaseLabel)
		}
		if tag != "" {
			return fmt.Sprintf("%s 在“%s”上仍有明显反复，建议先做同类巩固与短复盘。", title, tag)
		}
		return fmt.Sprintf("%s 当前仍有明显反复，建议先做同类巩固与短复盘。", title)
	default:
		if phaseLabel != "" && diagnosisContext.TaskPhase == model.LearningPhaseReview {
			if tag != "" {
				return fmt.Sprintf("%s 在%s已基本完成，但“%s”还需要再用一题确认是否真正收口。", title, phaseLabel, tag)
			}
			return fmt.Sprintf("%s 在%s已基本完成，但还需要再用一题确认是否真正收口。", title, phaseLabel)
		}
		if phaseLabel != "" && diagnosisContext.TaskPhase == model.LearningPhaseMock {
			if tag != "" {
				return fmt.Sprintf("%s 在%s已通过当前验证，但“%s”还需要继续观察迁移稳定性。", title, phaseLabel, tag)
			}
			return fmt.Sprintf("%s 在%s已通过当前验证，但还需要继续观察迁移稳定性。", title, phaseLabel)
		}
		if tag != "" {
			return fmt.Sprintf("%s 已完成，但“%s”还需要通过一道变式题继续确认。", title, tag)
		}
		if feedback != nil && feedback.AttemptCount > 1 {
			return fmt.Sprintf("%s 已完成，但过程仍有反复，建议用一题变式继续确认。", title)
		}
		return fmt.Sprintf("%s 已完成，建议保持当前节奏并用一道变式题继续确认。", title)
	}
}

// formatTaskFeedbackDiagnosisPhaseLabel 将阶段枚举转换为更适合诊断文案使用的中文名称。
func formatTaskFeedbackDiagnosisPhaseLabel(phase string) string {
	switch model.NormalizeLearningPhase(phase) {
	case model.LearningPhaseDrill:
		return "专项突破阶段"
	case model.LearningPhaseReview:
		return "复盘纠偏阶段"
	case model.LearningPhaseMock:
		return "模拟验证阶段"
	default:
		return "打基础阶段"
	}
}

// normalizeTaskFeedbackDiagnosisActions 清理动作列表中的空值和重复项，保证后续消费稳定。
func normalizeTaskFeedbackDiagnosisActions(actions []taskFeedbackDiagnosisAction) []taskFeedbackDiagnosisAction {
	result := make([]taskFeedbackDiagnosisAction, 0, len(actions))
	seen := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		action.Action = strings.TrimSpace(action.Action)
		action.TargetFocusTag = strings.TrimSpace(action.TargetFocusTag)
		action.CollectionHint = strings.TrimSpace(action.CollectionHint)
		action.Reason = strings.TrimSpace(action.Reason)
		if action.Action == "" || action.Reason == "" {
			continue
		}
		key := strings.Join([]string{action.Action, action.TargetFocusTag, action.Reason}, "||")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, action)
	}
	return result
}

// firstDiagnosisFocusTag 取第一条可用错因标签，便于生成诊断动作锚点。
func firstDiagnosisFocusTag(tags []string) string {
	for _, tag := range tags {
		if trimmed := strings.TrimSpace(tag); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
