package data

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"google.golang.org/grpc"

	aiv1 "makejob/api/makejob/ai/v1"
	"makejob/app/plan/internal/biz"
	"makejob/app/plan/internal/conf"
	"makejob/pkg/middleware"
)

// planAgentClient 实现 biz.PlanAgentClient 接口，通过 gRPC 调用 AI Gateway
type planAgentClient struct {
	client aiv1.AIServiceClient
	conn   *grpc.ClientConn
}

// NewPlanAgentClient 创建 AI 服务客户端
func NewPlanAgentClient(cfg *conf.AI) (biz.PlanAgentClient, error) {
	conn, err := grpc.Dial(cfg.ServiceAddr, middleware.CommonDialOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial AI service at %s: %w", cfg.ServiceAddr, err)
	}
	return &planAgentClient{
		client: aiv1.NewAIServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *planAgentClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GeneratePlan 调用 AI Gateway 的 PlanAgent RPC 生成计划
func (c *planAgentClient) GeneratePlan(ctx context.Context, req *biz.PlanAgentRequest) (*biz.PlanAgentResponse, error) {
	dailyHours := req.DailyHours
	if dailyHours <= 0 && req.DailyStudyMinutes > 0 {
		dailyHours = int32(math.Ceil(float64(req.DailyStudyMinutes) / 60))
	}

	resp, err := c.client.PlanAgent(ctx, &aiv1.PlanAgentRequest{
		UserId:           req.UserID,
		IndustryCode:     req.IndustryCode,
		Goal:             buildPlanGoal(req),
		DailyHours:       dailyHours,
		WeakTopics:       req.WeakTopics,
		RecentActivities: req.RecentActivities,
	})
	if err != nil {
		return nil, fmt.Errorf("PlanAgent gRPC call failed: %w", err)
	}
	return toBizPlanAgentResponse(resp), nil
}

// AdjustPlan 调用专用 AdjustPlan RPC 生成最新建议，并在客户端本地计算增删改。
func (c *planAgentClient) AdjustPlan(ctx context.Context, req *biz.PlanAgentAdjustRequest) (*biz.PlanAgentAdjustResponse, error) {
	if req.Plan == nil {
		return nil, fmt.Errorf("plan is required for adjustment")
	}

	resp, err := c.client.AdjustPlan(ctx, &aiv1.AdjustPlanRequest{
		UserId:          req.Plan.UserID,
		PlanId:          fmt.Sprintf("%d", req.Plan.ID),
		CompletedTasks:  buildCompletedTaskTitles(req.CurrentTasks),
		Performance:     buildAdjustPerformance(req.Feedbacks),
		CurrentPhase:    req.Plan.Phase,
		WeakTopics:      mergeWeakTopics(collectWeakTopics(req.Feedbacks), req.ExtraWeakTopics),
		GoalDescription: buildAdjustGoal(req),
		IndustryCode:    strings.TrimSpace(req.Plan.Industry),
		DailyHours:      deriveAdjustDailyHours(req.Plan, req.CurrentTasks),
	})
	if err != nil {
		return nil, fmt.Errorf("AdjustPlan RPC call failed: %w", err)
	}

	generatedTasks := make([]*biz.PlanAgentTask, 0, len(resp.GetTasks()))
	for _, task := range resp.GetTasks() {
		durationMinutes := task.GetDurationMinutes()
		if durationMinutes <= 0 {
			durationMinutes = task.GetEstimatedHours() * 60
		}
		if durationMinutes <= 0 {
			durationMinutes = 30
		}
		taskType := task.GetTaskType()
		if taskType == "" {
			taskType = inferTaskType(task.GetPhase())
		}
		priority := task.GetPriority()
		if priority == "" {
			priority = inferPriority(task.GetEstimatedHours())
		}
		generatedTasks = append(generatedTasks, &biz.PlanAgentTask{
			Title:           task.GetTitle(),
			Description:     task.GetDescription(),
			TaskType:        taskType,
			Phase:           task.GetPhase(),
			PhaseGoal:       task.GetPhaseGoal(),
			DayNumber:       firstPositive(task.GetDayNumber(), task.GetOrderIndex()),
			DurationMinutes: durationMinutes,
			Priority:        priority,
			SortOrder:       firstPositive(task.GetOrderIndex(), task.GetDayNumber()),
		})
	}

	return buildAdjustResponse(req.CurrentTasks, generatedTasks, resp.GetSummary()), nil
}

// toBizPlanAgentResponse 将 AI proto 响应转换为 biz 响应。
func toBizPlanAgentResponse(resp *aiv1.PlanAgentResponse) *biz.PlanAgentResponse {
	tasks := make([]*biz.PlanAgentTask, 0, len(resp.Tasks))
	for _, task := range resp.Tasks {
		durationMinutes := task.EstimatedHours * 60
		if durationMinutes <= 0 {
			durationMinutes = 30
		}
		taskType := inferTaskType(task.Phase)
		priority := inferPriority(task.EstimatedHours)
		tasks = append(tasks, &biz.PlanAgentTask{
			Title:           task.Title,
			Description:     task.Description,
			TaskType:        taskType,
			Phase:           task.Phase,
			PhaseGoal:       task.PhaseGoal,
			DayNumber:       task.OrderIndex,
			DurationMinutes: durationMinutes,
			Priority:        priority,
			SortOrder:       task.OrderIndex,
		})
	}
	return &biz.PlanAgentResponse{
		PlanTitle: resp.PlanTitle,
		Tasks:     tasks,
		Summary:   resp.Summary,
	}
}

// buildPlanGoal 组合计划生成上下文，补足当前 AI proto 缺失的字段。
func buildPlanGoal(req *biz.PlanAgentRequest) string {
	return fmt.Sprintf("%s。学习级别：%s。计划周期：%d 天。每日学习时长：%d 分钟。",
		req.Goal, req.Level, req.DurationDays, req.DailyStudyMinutes)
}

// buildAdjustGoal 组合计划调整上下文，让 AI 基于当前任务与反馈重新规划。
func buildAdjustGoal(req *biz.PlanAgentAdjustRequest) string {
	return fmt.Sprintf(
		"请基于当前学习计划做调整。行业：%s；级别：%s；周期：%d 天；每日学习：%d 分钟；用户原因：%s。当前任务：%s。反馈摘要：%s。",
		req.Plan.Industry,
		req.Plan.Level,
		req.Plan.DurationDays,
		req.Plan.DailyStudyMinutes,
		req.Reason,
		summarizeTasks(req.CurrentTasks),
		summarizeFeedbacks(req.Feedbacks),
	)
}

// summarizeTasks 压缩任务列表为 AI 可消费的文本摘要。
func summarizeTasks(tasks []*biz.LearningTask) string {
	if len(tasks) == 0 {
		return "无任务"
	}
	parts := make([]string, 0, len(tasks))
	for _, task := range tasks {
		parts = append(parts, fmt.Sprintf("[%d]%s/%s/%s/%d分钟", task.SortOrder, task.Title, task.Status, task.Phase, task.DurationMinutes))
	}
	return strings.Join(parts, "；")
}

// summarizeFeedbacks 压缩反馈和诊断状态，避免直接丢失调整依据。
func summarizeFeedbacks(feedbacks []*biz.TaskFeedback) string {
	if len(feedbacks) == 0 {
		return "暂无反馈"
	}
	parts := make([]string, 0, len(feedbacks))
	for _, feedback := range feedbacks {
		parts = append(parts, fmt.Sprintf("task=%d,难度=%s,反馈=%s,诊断状态=%s", feedback.TaskID, feedback.DifficultyFeeling, feedback.FeedbackText, feedback.DiagnosisStatus))
	}
	return strings.Join(parts, "；")
}

// collectWeakTopics 从反馈中的 problem_areas_json 聚合薄弱点列表。
func collectWeakTopics(feedbacks []*biz.TaskFeedback) []string {
	seen := make(map[string]struct{})
	topics := make([]string, 0)
	for _, feedback := range feedbacks {
		var areas []string
		if err := json.Unmarshal([]byte(feedback.ProblemAreasJSON), &areas); err != nil {
			continue
		}
		for _, area := range areas {
			trimmed := strings.TrimSpace(area)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			topics = append(topics, trimmed)
		}
	}
	return topics
}

// buildRecentActivities 将反馈转成 recent_activities，帮助 AI 理解近况。
func buildRecentActivities(feedbacks []*biz.TaskFeedback) []string {
	activities := make([]string, 0, len(feedbacks))
	for _, feedback := range feedbacks {
		activities = append(activities, fmt.Sprintf("task=%d difficulty=%s feedback=%s", feedback.TaskID, feedback.DifficultyFeeling, feedback.FeedbackText))
	}
	return activities
}

// deriveAdjustDailyHours 推断调整计划时可用的每日学习时长，缺失时给出保守默认值。
func deriveAdjustDailyHours(plan *biz.LearningPlan, tasks []*biz.LearningTask) int32 {
	if plan != nil && plan.DailyStudyMinutes > 0 {
		return int32(math.Ceil(float64(plan.DailyStudyMinutes) / 60))
	}

	var maxDuration int32
	for _, task := range tasks {
		if task != nil && task.DurationMinutes > maxDuration {
			maxDuration = task.DurationMinutes
		}
	}
	if maxDuration > 0 {
		return int32(math.Ceil(float64(maxDuration) / 60))
	}
	return 1
}

// buildCompletedTaskTitles 提取已完成或已跳过任务标题，供调整 prompt 识别当前推进情况。
func buildCompletedTaskTitles(tasks []*biz.LearningTask) []string {
	titles := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if task.Status != "completed" && task.Status != "skipped" {
			continue
		}
		title := strings.TrimSpace(task.Title)
		if title == "" {
			continue
		}
		titles = append(titles, title)
	}
	return titles
}

// buildAdjustPerformance 将反馈分布压缩为简要指标，供调整 prompt 判断节奏和难度。
func buildAdjustPerformance(feedbacks []*biz.TaskFeedback) map[string]float64 {
	performance := map[string]float64{
		"feedback_count": float64(len(feedbacks)),
	}
	for _, feedback := range feedbacks {
		if feedback == nil {
			continue
		}
		if feedback.DiagnosisStatus == "completed" {
			performance["diagnosed_feedback_count"]++
		}
		switch feedback.DifficultyFeeling {
		case "too_hard":
			performance["too_hard_count"]++
		case "just_right":
			performance["just_right_count"]++
		case "too_easy":
			performance["too_easy_count"]++
		}
	}
	return performance
}

// mergeWeakTopics 合并反馈薄弱点与画像薄弱点，并保持去重顺序稳定。
func mergeWeakTopics(groups ...[]string) []string {
	seen := make(map[string]struct{})
	topics := make([]string, 0)
	for _, group := range groups {
		for _, topic := range group {
			trimmed := strings.TrimSpace(topic)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			topics = append(topics, trimmed)
		}
	}
	return topics
}

// firstPositive 返回首个大于 0 的值，用于兼容 order/day_number 任一字段缺失的 AI 输出。
func firstPositive(values ...int32) int32 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 1
}

// inferTaskType 根据阶段推断任务类型。
func inferTaskType(phase string) string {
	lower := strings.ToLower(phase)
	switch {
	case strings.Contains(lower, "interview"):
		return "interview"
	case strings.Contains(lower, "review"):
		return "review"
	case strings.Contains(lower, "practice"):
		return "practice"
	default:
		return "study"
	}
}

// inferPriority 根据预计时长给出默认优先级。
func inferPriority(estimatedHours int32) string {
	switch {
	case estimatedHours >= 3:
		return "high"
	case estimatedHours == 2:
		return "medium"
	default:
		return "low"
	}
}

// buildAdjustResponse 基于新旧任务列表计算可落库的增删改集合。
func buildAdjustResponse(currentTasks []*biz.LearningTask, generatedTasks []*biz.PlanAgentTask, summary string) *biz.PlanAgentAdjustResponse {
	baseSortOrder := int32(0)
	existingPending := make(map[string][]*biz.LearningTask)
	for _, task := range currentTasks {
		if task.Status != "pending" {
			if task.SortOrder > baseSortOrder {
				baseSortOrder = task.SortOrder
			}
			continue
		}
		key := normalizeTaskKey(task.Title, task.Phase)
		existingPending[key] = append(existingPending[key], task)
	}

	reorder := make(map[uint64]int32)
	added := make([]*biz.PlanAgentTask, 0)
	for idx, task := range generatedTasks {
		task.SortOrder = baseSortOrder + int32(idx) + 1
		if task.DayNumber <= 0 {
			task.DayNumber = int32(idx) + 1
		}
		key := normalizeTaskKey(task.Title, task.Phase)
		if matches := existingPending[key]; len(matches) > 0 {
			matched := matches[0]
			existingPending[key] = matches[1:]
			if matched.SortOrder != task.SortOrder {
				reorder[matched.ID] = task.SortOrder
			}
			continue
		}
		added = append(added, task)
	}

	remove := make([]uint64, 0)
	for _, matches := range existingPending {
		for _, task := range matches {
			remove = append(remove, task.ID)
		}
	}

	return &biz.PlanAgentAdjustResponse{
		Add:     added,
		Remove:  remove,
		Reorder: reorder,
		Summary: summary,
	}
}

// normalizeTaskKey 生成任务匹配键，避免调整时重复创建同名同阶段任务。
func normalizeTaskKey(title, phase string) string {
	return strings.ToLower(strings.TrimSpace(title)) + "|" + strings.ToLower(strings.TrimSpace(phase))
}

// diagnosisClient 实现 biz.DiagnosisClient 接口，通过 QuizAnalyzer RPC 进行诊断分析
type diagnosisClient struct {
	client aiv1.AIServiceClient
	conn   *grpc.ClientConn
}

// NewDiagnosisClient 创建诊断分析客户端
func NewDiagnosisClient(cfg *conf.AI) (biz.DiagnosisClient, error) {
	conn, err := grpc.Dial(cfg.ServiceAddr, middleware.CommonDialOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial AI service for diagnosis at %s: %w", cfg.ServiceAddr, err)
	}
	return &diagnosisClient{
		client: aiv1.NewAIServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *diagnosisClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Analyze 调用 AI Gateway 的 QuizAnalyzer 执行诊断分析，返回 JSON 字符串。
func (c *diagnosisClient) Analyze(ctx context.Context, task *biz.LearningTask, feedbackText, difficultyFeeling string, problemAreas []string) (string, error) {
	question := fmt.Sprintf("学习任务：%s；阶段：%s；任务描述：%s", task.Title, task.Phase, task.Description)
	answer := feedbackText
	if difficultyFeeling != "" {
		answer = fmt.Sprintf("%s；自评难度：%s", answer, difficultyFeeling)
	}
	topic := task.Phase
	if len(problemAreas) > 0 {
		topic = strings.Join(problemAreas, ", ")
	}

	resp, err := c.client.QuizAnalyzer(ctx, &aiv1.QuizAnalyzerRequest{
		Question:   question,
		Answer:     answer,
		Topic:      topic,
		Difficulty: difficultyFeeling,
	})
	if err != nil {
		return "", fmt.Errorf("QuizAnalyzer gRPC call failed: %w", err)
	}

	result := map[string]any{
		"score":          resp.GetScore(),
		"is_correct":     resp.GetIsCorrect(),
		"feedback":       resp.GetFeedback(),
		"key_points":     resp.GetKeyPoints(),
		"suggestions":    resp.GetSuggestions(),
		"correct_answer": resp.GetCorrectAnswer(),
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("序列化诊断结果失败: %w", err)
	}
	return string(jsonBytes), nil
}
