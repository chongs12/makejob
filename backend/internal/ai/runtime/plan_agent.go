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
	Phase       string            `json:"phase"`
	PhaseGoal   string            `json:"phase_goal"`
	Duration    int               `json:"duration_days"`
	Tasks       []planTaskPayload `json:"tasks"`
}

// planTaskPayload 表示模型返回的学习任务结构。
type planTaskPayload struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	TaskType    string `json:"task_type"`
	Phase       string `json:"phase"`
	PhaseGoal   string `json:"phase_goal"`
	DayNumber   int    `json:"day_number"`
	Duration    int    `json:"duration_minutes"`
	Priority    string `json:"priority"`
}

// learningPlanPayloadSchema 返回学习计划结构化输出的 JSON 合同。
func learningPlanPayloadSchema() string {
	return `{
  "title": "学习计划标题",
  "description": "学习计划说明",
  "phase": "foundation|drill|review|mock",
  "phase_goal": "phase goal",
  "duration_days": 30,
  "tasks": [
    {
      "title": "任务标题",
      "description": "任务说明",
      "task_type": "study|practice|interview|review",
      "phase": "foundation|drill|review|mock",
      "phase_goal": "phase goal",
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
	userPrompt := buildPlanGenerateUserPromptWithPhases(profile, industryCode)
	messages := []ai.Message{
		{
			Role:    "system",
			Content: buildPlanSystemPromptWithPhases(promptDetails.Prompt),
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
func (a *providerPlanAgent) AdjustPlan(ctx context.Context, input ai.PlanAdjustmentInput) (ai.LearningPlan, error) {
	if a.shouldUseFallback() {
		return ai.LearningPlan{}, fmt.Errorf("ai provider is unavailable")
	}

	traceID := uuid.NewString()
	promptDetails := resolvedPromptDetails{
		Prompt: buildPlanAdjustSystemPrompt(),
		Source: "inline_builtin",
	}
	userPrompt := buildPlanAdjustUserPromptWithContext(input)
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
		DurationDays: maxInt(len(input.CompletedTasks)+7, 7),
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
		defaultString(truncateString(strings.TrimSpace(profile.GoalDescription), 200), "提升面试能力"),
		defaultString(truncateString(strings.Join(profile.WeakTopics, "、"), 200), "无"),
		defaultString(truncateString(strings.Join(profile.StrongTopics, "、"), 200), "无"),
	)
}

// buildPlanAdjustSystemPrompt 构造调整学习计划的系统提示词。
func buildPlanAdjustSystemPrompt() string {
	return `请根据已有学习进度和调整上下文重新生成一个更合理的学习计划，并严格返回 JSON，不要输出 Markdown 或额外解释。JSON 结构如下：
{
  "title": "调整后的计划标题",
  "description": "调整后的计划说明",
  "phase": "foundation|drill|review|mock",
  "phase_goal": "phase goal",
  "duration_days": 14,
  "tasks": [
    {
      "title": "任务标题",
      "description": "任务说明",
      "task_type": "study|practice|interview|review",
      "phase": "foundation|drill|review|mock",
      "phase_goal": "phase goal",
      "day_number": 1,
      "duration_minutes": 60,
      "priority": "high|medium|low"
    }
  ]
}
阶段约束规则：
1. 计划和每个任务都必须输出 phase 与 phase_goal。
2. 阶段推进必须遵循 foundation -> drill -> review -> mock 的顺序，严禁跨阶段或乱序安排。
3. 每个任务的 day_number 必须落在该阶段蓝图规定的 start_day 到 end_day 范围内。
4. foundation 阶段任务类型必须以 study 为主，仅可少量加入用于开题的 practice，严禁安排 interview。
5. drill 阶段任务类型必须以 practice 为主，围绕弱项做密集巩固，严禁安排 interview。
6. review 阶段任务类型必须以 review 或短 study 为主，用于复盘、纠偏和总结。
7. mock 阶段任务类型必须以 interview 或限时综合 practice 为主，用于验证真实掌握度，严禁安排 study。
8. 各阶段的任务数量和天数分配必须严格遵循阶段蓝图中的 day range 约束。
9. 调整原因码是本轮调整的核心依据，必须严格按照原因码约束指令安排阶段和任务。`
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
		Phase:       model.NormalizeLearningPhase(payload.Phase),
		PhaseGoal:   strings.TrimSpace(payload.PhaseGoal),
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
	if plan.Phase == "" && len(plan.Tasks) > 0 {
		plan.Phase = plan.Tasks[0].Phase
	}
	if plan.PhaseGoal == "" && plan.Phase != "" {
		plan.PhaseGoal = model.BuildLearningPhaseGoal(plan.Phase)
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
		Phase:       model.NormalizeLearningPhase(taskPayload.Phase),
		PhaseGoal:   strings.TrimSpace(taskPayload.PhaseGoal),
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
	if task.PhaseGoal == "" && task.Phase != "" {
		task.PhaseGoal = model.BuildLearningPhaseGoal(task.Phase)
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

// truncateString 截断字符串到指定最大长度，超出部分用省略号替代。
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// buildPlanAdjustUserPromptWithContext 构造带阶段上下文的学习计划调优提示词。
func buildPlanAdjustUserPromptWithContext(input ai.PlanAdjustmentInput) string {
	parts := []string{
		"请基于下面的计划调整上下文，生成一份后续 7 到 21 天的学习计划。",
		fmt.Sprintf("计划ID: %s", input.PlanID),
		fmt.Sprintf("已完成任务: %s", defaultString(truncateString(strings.Join(input.CompletedTasks, "；"), 200), "无")),
		fmt.Sprintf("任务表现: %s", renderPerformance(input.Performance)),
		fmt.Sprintf("当前阶段: %s", defaultString(strings.TrimSpace(input.CurrentPhase), "foundation")),
		fmt.Sprintf("本轮入口阶段: %s", defaultString(strings.TrimSpace(input.EntryPhase), defaultString(strings.TrimSpace(input.CurrentPhase), "foundation"))),
		fmt.Sprintf("最近诊断摘要: %s", defaultString(truncateString(strings.Join(input.ActionSummaries, "；"), 200), "无")),
	}

	if input.GoalDescription != "" {
		parts = append(parts, fmt.Sprintf("学习目标: %s", truncateString(input.GoalDescription, 200)))
	}
	if len(input.WeakTopics) > 0 {
		parts = append(parts, fmt.Sprintf("薄弱主题: %s", truncateString(strings.Join(input.WeakTopics, "、"), 200)))
	}

	if len(input.PhaseBlueprint) > 0 {
		parts = append(parts, fmt.Sprintf("阶段蓝图:\n%s", formatPhaseBlueprintForPrompt(input.PhaseBlueprint)))
	}

	if len(input.ReasonCodes) > 0 {
		parts = append(parts, fmt.Sprintf("调整原因码: %s", strings.Join(input.ReasonCodes, "、")))
		parts = append(parts, fmt.Sprintf("原因码约束:\n%s", buildReasonCodeConstraint(input.ReasonCodes)))
	}

	parts = append(parts, "要求：优先让任务与入口阶段保持一致，严格按照阶段蓝图的 day range 安排任务，并在任务和计划中都输出 phase 与 phase_goal 字段。")
	return strings.Join(parts, "\n")
}

// formatPhaseBlueprintForPrompt 将结构化蓝图格式化为 AI 可读的文本。
func formatPhaseBlueprintForPrompt(blueprint []ai.PhaseBlueprintEntry) string {
	var parts []string
	for i, entry := range blueprint {
		part := fmt.Sprintf("%d. %s (Day %d-%d): 目标「%s」；预期任务类型: %s；退出标准: %s",
			i+1,
			phaseDisplayName(entry.Phase),
			entry.StartDay,
			entry.EndDay,
			entry.PhaseGoal,
			strings.Join(entry.ExpectedTaskTypes, "、"),
			strings.Join(entry.ExitCriteria, "；"),
		)
		parts = append(parts, part)
	}
	return strings.Join(parts, "\n")
}

// buildReasonCodeConstraint 根据原因码生成对应的生成约束指令。
func buildReasonCodeConstraint(codes []string) string {
	constraintMap := map[string]string{
		"mock_not_stable":     "mock_not_stable: 当前模拟验证不稳定，必须回退到 review 阶段，安排复盘和查漏补缺任务，暂时不要安排 mock 类型任务。",
		"weakness_unresolved": "weakness_unresolved: 弱项尚未解决，必须继续 drill 阶段，安排专项 practice 巩固薄弱点，暂时不要推进到 review 或 mock。",
		"partial_mastery":     "partial_mastery: 部分掌握但不够扎实，继续 drill 阶段，安排变式题和同类练习确认掌握度。",
		"review_completed":    "review_completed: 复盘已完成，可以回到 drill 阶段提升难度，或推进到 mock 阶段做验证。",
		"progress_verified":   "progress_verified: 进度已验证通过，可以进入 mock 阶段，安排 interview 或限时综合 practice 做真实验证。",
	}

	var parts []string
	for _, code := range codes {
		if constraint, ok := constraintMap[code]; ok {
			parts = append(parts, constraint)
		}
	}
	if len(parts) == 0 {
		return "无特定约束。"
	}
	return strings.Join(parts, "\n")
}

// buildPlanSystemPromptWithPhases 构造带阶段约束的生成系统提示词。
func buildPlanSystemPromptWithPhases(basePrompt string) string {
	return buildPlanSystemPrompt(basePrompt) + `

阶段生成补充要求：
1. 计划和每个任务都必须输出 phase 与 phase_goal。
2. 阶段推进必须遵循 foundation -> drill -> review -> mock 的顺序，严禁跨阶段或乱序安排。
3. 每个任务的 day_number 必须落在该阶段蓝图规定的 start_day 到 end_day 范围内。
4. foundation 阶段任务类型必须以 study 为主，仅可少量加入用于开题的 practice，严禁安排 interview。
5. drill 阶段任务类型必须以 practice 为主，围绕弱项做密集巩固，严禁安排 interview。
6. review 阶段任务类型必须以 review 或短 study 为主，用于复盘、纠偏和总结。
7. mock 阶段任务类型必须以 interview 或限时综合 practice 为主，用于验证真实掌握度，严禁安排 study。
8. 各阶段的任务数量和天数分配必须严格遵循阶段蓝图中的 day range 约束。`
}

// buildPlanGenerateUserPromptWithPhases 构造带阶段蓝图的生成提示词。
func buildPlanGenerateUserPromptWithPhases(profile ai.UserProfile, industryCode string) string {
	return fmt.Sprintf(
		"%s\n阶段蓝图:\n%s\n\n要求：每个任务的 day_number 必须落在对应阶段的 day range 内，任务类型必须符合阶段预期。",
		buildPlanGenerateUserPrompt(profile, industryCode),
		buildStructuredPhaseBlueprintText(profile.DurationDays),
	)
}

// buildGeneratePhaseBlueprint 根据计划周期给出阶段化编排蓝图。
func buildGeneratePhaseBlueprint(durationDays int) string {
	durationDays = maxInt(durationDays, 7)
	if durationDays < 14 {
		return "1. 前段先放 foundation，帮助补齐概念、方法和题型框架。\n2. 中段进入 drill，集中安排 practice 巩固弱项。\n3. 收尾进入 review，用 review 或短 study 做复盘总结。\n4. 短周期可以不强制安排 mock，但不要跳过 review。"
	}
	if durationDays < 21 {
		return "1. 前段从 foundation 起步，先补齐核心概念和通用解题方法。\n2. 中段进入 drill，围绕弱项连续安排 practice。\n3. 后段先做 review，再安排少量 mock 验证掌握度。\n4. mock 阶段优先使用 interview 或限时综合 practice。"
	}
	return "1. 第一阶段使用 foundation 建立概念、方法和知识框架。\n2. 第二阶段进入 drill，持续做专项 practice 提升熟练度。\n3. 第三阶段进入 review，系统复盘近期错误与薄弱点。\n4. 最后一阶段进入 mock，用 interview 或限时综合 practice 做真实验证。"
}

// buildStructuredPhaseBlueprintText 根据计划周期生成结构化蓝图文本，包含 day range、预期任务类型和退出标准。
func buildStructuredPhaseBlueprintText(durationDays int) string {
	entries := buildPhaseBlueprintEntries(durationDays)
	var parts []string
	for i, entry := range entries {
		part := fmt.Sprintf("%d. %s (Day %d-%d): 目标「%s」；预期任务类型: %s；退出标准: %s",
			i+1,
			phaseDisplayName(entry.Phase),
			entry.StartDay,
			entry.EndDay,
			entry.PhaseGoal,
			strings.Join(entry.ExpectedTaskTypes, "、"),
			strings.Join(entry.ExitCriteria, "；"),
		)
		parts = append(parts, part)
	}
	return strings.Join(parts, "\n")
}

// buildPhaseBlueprintEntries 根据计划周期生成阶段蓝图条目列表。
func buildPhaseBlueprintEntries(durationDays int) []struct {
	Phase             string
	PhaseGoal         string
	StartDay          int
	EndDay            int
	ExpectedTaskTypes []string
	ExitCriteria      []string
} {
	durationDays = maxInt(durationDays, 7)
	if durationDays < 14 {
		foundationEnd := maxInt(durationDays*3/10, 2)
		drillEnd := foundationEnd + maxInt(durationDays*4/10, 3)
		return []struct {
			Phase             string
			PhaseGoal         string
			StartDay          int
			EndDay            int
			ExpectedTaskTypes []string
			ExitCriteria      []string
		}{
			{"foundation", "先补齐核心概念、基础方法和通用解题框架。", 1, foundationEnd, []string{"study", "practice"}, []string{"能说清核心解法步骤", "完成至少一题开题型练习"}},
			{"drill", "围绕当前高频薄弱点做专项强化训练。", foundationEnd + 1, drillEnd, []string{"practice"}, []string{"同类题型正确率明显提升", "弱项标签覆盖次数达标"}},
			{"review", "回看近期训练表现，修正易错点并巩固方法。", drillEnd + 1, durationDays, []string{"review", "study"}, []string{"近期高频错误已整理成固定检查点"}},
		}
	}
	if durationDays < 21 {
		foundationEnd := maxInt(durationDays*2/10, 2)
		drillEnd := foundationEnd + maxInt(durationDays*3/10, 3)
		reviewEnd := drillEnd + maxInt(durationDays*3/10, 3)
		return []struct {
			Phase             string
			PhaseGoal         string
			StartDay          int
			EndDay            int
			ExpectedTaskTypes []string
			ExitCriteria      []string
		}{
			{"foundation", "先补齐核心概念、基础方法和通用解题框架。", 1, foundationEnd, []string{"study", "practice"}, []string{"能说清核心解法步骤", "完成至少一题开题型练习"}},
			{"drill", "围绕当前高频薄弱点做专项强化训练。", foundationEnd + 1, drillEnd, []string{"practice"}, []string{"同类题型正确率明显提升", "弱项标签覆盖次数达标"}},
			{"review", "回看近期训练表现，修正易错点并巩固方法。", drillEnd + 1, reviewEnd, []string{"review", "study"}, []string{"近期高频错误已整理成固定检查点"}},
			{"mock", "用模拟或限时任务验证当前阶段的真实掌握度。", reviewEnd + 1, durationDays, []string{"interview", "practice"}, []string{"限时场景下能稳定输出正确解法"}},
		}
	}
	foundationEnd := maxInt(durationDays*2/10, 3)
	drillEnd := foundationEnd + maxInt(durationDays*2/10, 3)
	reviewEnd := drillEnd + maxInt(durationDays*4/10, 5)
	return []struct {
		Phase             string
		PhaseGoal         string
		StartDay          int
		EndDay            int
		ExpectedTaskTypes []string
		ExitCriteria      []string
	}{
		{"foundation", "先补齐核心概念、基础方法和通用解题框架。", 1, foundationEnd, []string{"study", "practice"}, []string{"能说清核心解法步骤", "完成至少一题开题型练习"}},
		{"drill", "围绕当前高频薄弱点做专项强化训练。", foundationEnd + 1, drillEnd, []string{"practice"}, []string{"同类题型正确率明显提升", "弱项标签覆盖次数达标"}},
		{"review", "回看近期训练表现，修正易错点并巩固方法。", drillEnd + 1, reviewEnd, []string{"review", "study"}, []string{"近期高频错误已整理成固定检查点"}},
		{"mock", "用模拟或限时任务验证当前阶段的真实掌握度。", reviewEnd + 1, durationDays, []string{"interview", "practice"}, []string{"限时场景下能稳定输出正确解法"}},
	}
}

// phaseDisplayName 返回阶段的中文显示名称。
func phaseDisplayName(phase string) string {
	switch phase {
	case "foundation":
		return "打基础"
	case "drill":
		return "专项突破"
	case "review":
		return "复盘纠偏"
	case "mock":
		return "模拟验证"
	default:
		return phase
	}
}
