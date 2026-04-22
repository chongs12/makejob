package runtime

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

// providerPlanAgent 基于真实 Provider 生成学习计划。
type providerPlanAgent struct {
	provider ai.AIProvider
	prompts  *promptResolver
	logger   *aiCallLogRecorder
}

// learningPlanPayload 表示模型返回的学习计划结构。
type learningPlanPayload struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Duration    int               `json:"duration_days"`
	Tasks       []planTaskPayload `json:"tasks"`
}

// planTaskPayload 表示模型返回的学习任务结构。
type planTaskPayload struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	TaskType    string `json:"task_type"`
	DayNumber   int    `json:"day_number"`
	Duration    int    `json:"duration_minutes"`
	Priority    string `json:"priority"`
}

// learningPlanPayloadSchema 返回学习计划结构化输出的 JSON 合同。
func learningPlanPayloadSchema() string {
	return `{
  "title": "学习计划标题",
  "description": "学习计划说明",
  "duration_days": 30,
  "tasks": [
    {
      "title": "任务标题",
      "description": "任务说明",
      "task_type": "study|practice|interview|review",
      "day_number": 1,
      "duration_minutes": 60,
      "priority": "high|medium|low"
    }
  ]
}`
}

// newPlanAgent 创建学习计划运行时 Agent。
func newPlanAgent(provider ai.AIProvider, prompts *promptResolver, logger *aiCallLogRecorder) ai.PlanAgent {
	return &providerPlanAgent{
		provider: provider,
		prompts:  prompts,
		logger:   logger,
	}
}

// GeneratePlan 根据用户画像生成学习计划。
func (a *providerPlanAgent) GeneratePlan(ctx context.Context, profile ai.UserProfile, industryCode string) (ai.LearningPlan, error) {
	if a.shouldUseFallback() {
		return ai.LearningPlan{}, fmt.Errorf("ai provider is unavailable")
	}

	traceID := uuid.NewString()
	industryID := a.resolveIndustryID(ctx, industryCode)
	promptDetails := a.resolvePromptDetails(ctx, profile, industryCode)
	userPrompt := buildPlanGenerateUserPrompt(profile, industryCode)
	messages := []ai.Message{
		{
			Role:    "system",
			Content: buildPlanSystemPrompt(promptDetails.Prompt),
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	startedAt := time.Now()
	payload, response, err := callStructuredJSON[learningPlanPayload](ctx, a.provider, messages, learningPlanPayloadSchema())
	if err != nil {
		a.recordCall(ctx, traceID, industryID, promptDetails, userPrompt, messages, response, err, startedAt)
		return ai.LearningPlan{}, err
	}

	plan, err := normalizeLearningPlan(payload, profile, industryCode)
	a.recordCall(ctx, traceID, industryID, promptDetails, userPrompt, messages, response, err, startedAt)
	if err != nil {
		return ai.LearningPlan{}, err
	}

	return plan, nil
}

// AdjustPlan 根据执行反馈调整学习计划。
func (a *providerPlanAgent) AdjustPlan(ctx context.Context, planID string, completedTasks []string, performance map[string]float64) (ai.LearningPlan, error) {
	if a.shouldUseFallback() {
		return ai.LearningPlan{}, fmt.Errorf("ai provider is unavailable")
	}

	traceID := uuid.NewString()
	promptDetails := resolvedPromptDetails{
		Prompt: buildPlanAdjustSystemPrompt(),
		Source: "inline_builtin",
	}
	userPrompt := buildPlanAdjustUserPrompt(planID, completedTasks, performance)
	messages := []ai.Message{
		{
			Role:    "system",
			Content: promptDetails.Prompt,
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	startedAt := time.Now()
	payload, response, err := callStructuredJSON[learningPlanPayload](ctx, a.provider, messages, learningPlanPayloadSchema())
	if err != nil {
		a.recordCall(ctx, traceID, nil, promptDetails, userPrompt, messages, response, err, startedAt)
		return ai.LearningPlan{}, err
	}

	profile := ai.UserProfile{
		DurationDays: maxInt(len(completedTasks)+7, 7),
	}
	plan, err := normalizeLearningPlan(payload, profile, "")
	a.recordCall(ctx, traceID, nil, promptDetails, userPrompt, messages, response, err, startedAt)
	if err != nil {
		return ai.LearningPlan{}, err
	}

	return plan, nil
}

// GetStudySuggestion 返回简短的学习建议。
func (a *providerPlanAgent) GetStudySuggestion(ctx context.Context, profile ai.UserProfile) (string, error) {
	if a.shouldUseFallback() {
		return "", fmt.Errorf("ai provider is unavailable")
	}

	traceID := uuid.NewString()
	promptDetails := resolvedPromptDetails{
		Prompt: "你是一名学习教练，请输出 3 到 5 条简短、可执行的学习建议，使用 Markdown 列表返回。",
		Source: "inline_builtin",
	}
	userPrompt := buildStudySuggestionPrompt(profile)
	messages := []ai.Message{
		{
			Role:    "system",
			Content: promptDetails.Prompt,
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	startedAt := time.Now()
	response, err := a.provider.Chat(ctx, messages)
	if err != nil {
		a.recordCall(ctx, traceID, nil, promptDetails, userPrompt, messages, response, err, startedAt)
		return "", err
	}

	content := strings.TrimSpace(response)
	if content == "" {
		a.recordCall(ctx, traceID, nil, promptDetails, userPrompt, messages, response, fmt.Errorf("empty study suggestion response"), startedAt)
		return "", fmt.Errorf("empty study suggestion response")
	}

	a.recordCall(ctx, traceID, nil, promptDetails, userPrompt, messages, response, nil, startedAt)
	return content, nil
}

// shouldUseFallback 判断当前是否缺少可用 Provider。
func (a *providerPlanAgent) shouldUseFallback() bool {
	return a.provider == nil
}

// resolvePromptDetails 解析学习计划场景的 Prompt 明细。
func (a *providerPlanAgent) resolvePromptDetails(ctx context.Context, profile ai.UserProfile, industryCode string) resolvedPromptDetails {
	vars := map[string]string{
		"industry_code":    industryCode,
		"level":            profile.Level,
		"daily_study_time": intToString(profile.DailyStudyTime),
		"duration_days":    intToString(profile.DurationDays),
		"goal_description": profile.GoalDescription,
		"weak_topics":      strings.Join(profile.WeakTopics, ", "),
		"strong_topics":    strings.Join(profile.StrongTopics, ", "),
	}

	if a.prompts == nil {
		return resolvedPromptDetails{
			Prompt: renderPrompt(builtInScenePrompts[model.PromptScenePlan], vars),
			Source: "built_in",
		}
	}

	return a.prompts.ResolveDetailsByIndustryCode(ctx, model.PromptScenePlan, industryCode, vars)
}

// resolveIndustryID 根据行业编码解析行业 ID，供日志落库使用。
func (a *providerPlanAgent) resolveIndustryID(ctx context.Context, industryCode string) *uint {
	if a.prompts == nil {
		return nil
	}

	return a.prompts.lookupIndustryID(ctx, industryCode)
}

// recordCall 记录一次学习计划链路的运行时模型调用。
func (a *providerPlanAgent) recordCall(
	ctx context.Context,
	traceID string,
	industryID *uint,
	promptDetails resolvedPromptDetails,
	userInput string,
	messages []ai.Message,
	response string,
	err error,
	startedAt time.Time,
) {
	if a.logger == nil {
		return
	}

	a.logger.Record(ctx, runtimeCallLogEntry{
		TraceID:       traceID,
		IndustryID:    industryID,
		PromptDetails: promptDetails,
		Request:       messages,
		UserInput:     userInput,
		Model:         a.provider.GetModelName(),
		Output:        response,
		Err:           err,
		StartedAt:     startedAt,
	})
}

// buildPlanSystemPrompt 构造生成学习计划的系统提示词。
func buildPlanSystemPrompt(basePrompt string) string {
	return strings.TrimSpace(basePrompt) + `

请基于以上规则生成学习计划，并严格返回 JSON，不要输出 Markdown 或额外解释。JSON 结构如下：
{
  "title": "计划标题",
  "description": "计划说明",
  "duration_days": 30,
  "tasks": [
    {
      "title": "任务标题",
      "description": "任务说明",
      "task_type": "study|practice|interview|review",
      "day_number": 1,
      "duration_minutes": 60,
      "priority": "high|medium|low"
    }
  ]
}
要求：
1. tasks 至少返回 1 条。
2. day_number 必须位于 1 到 duration_days 之间。
3. 内容要贴合行业、用户水平、学习目标和强弱项。
4. 输出必须是合法 JSON。`
}

// buildPlanGenerateUserPrompt 构造学习计划生成请求。
func buildPlanGenerateUserPrompt(profile ai.UserProfile, industryCode string) string {
	return fmt.Sprintf(
		"请根据以下信息生成学习计划：\n行业: %s\n当前水平: %s\n每日可学习时间: %d 分钟\n计划周期: %d 天\n学习目标: %s\n薄弱主题: %s\n优势主题: %s",
		defaultString(strings.TrimSpace(industryCode), "go"),
		defaultString(strings.TrimSpace(profile.Level), "beginner"),
		maxInt(profile.DailyStudyTime, 60),
		maxInt(profile.DurationDays, 14),
		defaultString(strings.TrimSpace(profile.GoalDescription), "提升面试能力"),
		defaultString(strings.Join(profile.WeakTopics, "、"), "无"),
		defaultString(strings.Join(profile.StrongTopics, "、"), "无"),
	)
}

// buildPlanAdjustSystemPrompt 构造调整学习计划的系统提示词。
func buildPlanAdjustSystemPrompt() string {
	return `请根据已有学习进度重新生成一个更合理的学习计划，并严格返回 JSON，不要输出 Markdown 或额外解释。JSON 结构如下：
{
  "title": "调整后的计划标题",
  "description": "调整后的计划说明",
  "duration_days": 14,
  "tasks": [
    {
      "title": "任务标题",
      "description": "任务说明",
      "task_type": "study|practice|interview|review",
      "day_number": 1,
      "duration_minutes": 60,
      "priority": "high|medium|low"
    }
  ]
}
请重点体现未完成项补齐、薄弱项加练和节奏调整。`
}

// buildPlanAdjustUserPrompt 构造学习计划调整请求。
func buildPlanAdjustUserPrompt(planID string, completedTasks []string, performance map[string]float64) string {
	return fmt.Sprintf(
		"请调整学习计划：\n计划ID: %s\n已完成任务: %s\n表现数据: %s\n请输出未来 7 到 21 天内更合理的学习安排。",
		planID,
		defaultString(strings.Join(completedTasks, "、"), "无"),
		renderPerformance(performance),
	)
}

// buildStudySuggestionPrompt 构造学习建议请求。
func buildStudySuggestionPrompt(profile ai.UserProfile) string {
	return fmt.Sprintf(
		"当前水平: %s\n每日可学习时间: %d 分钟\n计划周期: %d 天\n学习目标: %s\n薄弱主题: %s\n优势主题: %s",
		defaultString(profile.Level, "beginner"),
		maxInt(profile.DailyStudyTime, 60),
		maxInt(profile.DurationDays, 14),
		defaultString(profile.GoalDescription, "提升学习效率"),
		defaultString(strings.Join(profile.WeakTopics, "、"), "无"),
		defaultString(strings.Join(profile.StrongTopics, "、"), "无"),
	)
}

// normalizeLearningPlan 规范化模型返回的学习计划。
func normalizeLearningPlan(payload learningPlanPayload, profile ai.UserProfile, industryCode string) (ai.LearningPlan, error) {
	plan := ai.LearningPlan{
		Title:       strings.TrimSpace(payload.Title),
		Description: strings.TrimSpace(payload.Description),
		Duration:    payload.Duration,
		Tasks:       make([]ai.PlanTask, 0, len(payload.Tasks)),
	}

	if plan.Duration <= 0 {
		plan.Duration = maxInt(profile.DurationDays, 14)
	}
	if plan.Title == "" {
		plan.Title = buildDefaultPlanTitle(industryCode, profile.Level, plan.Duration)
	}
	if plan.Description == "" {
		plan.Description = buildDefaultPlanDescription(profile, industryCode, plan.Duration)
	}

	for index, taskPayload := range payload.Tasks {
		task := normalizePlanTask(taskPayload, index, plan.Duration, profile.DailyStudyTime)
		if strings.TrimSpace(task.Title) == "" {
			continue
		}
		plan.Tasks = append(plan.Tasks, task)
	}

	if len(plan.Tasks) == 0 {
		return ai.LearningPlan{}, fmt.Errorf("empty learning plan tasks")
	}

	sort.SliceStable(plan.Tasks, func(i, j int) bool {
		if plan.Tasks[i].DayNumber == plan.Tasks[j].DayNumber {
			return plan.Tasks[i].Priority < plan.Tasks[j].Priority
		}
		return plan.Tasks[i].DayNumber < plan.Tasks[j].DayNumber
	})

	return plan, nil
}

// normalizePlanTask 规范化模型返回的学习任务。
func normalizePlanTask(taskPayload planTaskPayload, index int, durationDays int, dailyStudyTime int) ai.PlanTask {
	task := ai.PlanTask{
		Title:       strings.TrimSpace(taskPayload.Title),
		Description: strings.TrimSpace(taskPayload.Description),
		TaskType:    normalizeTaskType(taskPayload.TaskType),
		DayNumber:   taskPayload.DayNumber,
		Duration:    taskPayload.Duration,
		Priority:    normalizePriority(taskPayload.Priority),
	}

	if task.DayNumber <= 0 {
		task.DayNumber = minInt(index+1, maxInt(durationDays, 1))
	}
	if task.DayNumber > durationDays && durationDays > 0 {
		task.DayNumber = durationDays
	}
	if task.Duration <= 0 {
		task.Duration = defaultTaskDuration(dailyStudyTime, task.TaskType)
	}
	if task.Description == "" {
		task.Description = fmt.Sprintf("围绕 %s 安排一段聚焦练习。", task.Title)
	}

	return task
}

// normalizeTaskType 标准化任务类型。
func normalizeTaskType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "study", "practice", "interview", "review":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "study"
	}
}

// normalizePriority 标准化任务优先级。
func normalizePriority(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "medium"
	}
}

// defaultTaskDuration 根据任务类型补齐默认时长。
func defaultTaskDuration(dailyStudyTime int, taskType string) int {
	budget := maxInt(dailyStudyTime, 60)

	switch taskType {
	case "practice":
		return minInt(maxInt(budget, 60), 180)
	case "interview":
		return minInt(maxInt(budget, 45), 120)
	case "review":
		return minInt(maxInt(budget/2, 30), 90)
	default:
		return minInt(maxInt(budget, 45), 150)
	}
}

// buildDefaultPlanTitle 构造默认学习计划标题。
func buildDefaultPlanTitle(industryCode string, level string, durationDays int) string {
	return fmt.Sprintf("%d天%s%s学习计划", durationDays, strings.ToUpper(defaultString(industryCode, "go")), levelLabel(level))
}

// buildDefaultPlanDescription 构造默认学习计划说明。
func buildDefaultPlanDescription(profile ai.UserProfile, industryCode string, durationDays int) string {
	return fmt.Sprintf(
		"这是一份面向 %s %s 学习者的 %d 天学习计划，目标是帮助你逐步达成“%s”。",
		defaultString(industryCode, "go"),
		levelLabel(profile.Level),
		durationDays,
		defaultString(profile.GoalDescription, "提升面试竞争力"),
	)
}

// renderPerformance 将表现数据渲染为可读文本。
func renderPerformance(performance map[string]float64) string {
	if len(performance) == 0 {
		return "无"
	}

	keys := make([]string, 0, len(performance))
	for key := range performance {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%.0f", key, performance[key]))
	}
	return strings.Join(parts, "、")
}

// levelLabel 返回用户水平的中文标签。
func levelLabel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "advanced":
		return "高级"
	case "intermediate":
		return "中级"
	default:
		return "初级"
	}
}

// minInt 返回两个整数中的较小值。
func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
