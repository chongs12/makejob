// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

const growthInterviewMetricsPageSize = 1000
const growthRecentInterviewLimit = 5
const growthRecentPlanLimit = 5
const growthRecentStudyLogLimit = 7
const growthWeeklyFocusArchiveLimit = 24
const growthWeeklyFocusThemeLimit = 3
const growthWeeklyFocusSuggestionLimit = 2

// SyncStudyLogRequest 表示前端同步每日学习摘要时提交的请求体。
type SyncStudyLogRequest struct {
	DateKey          string   `json:"date_key" binding:"required"`
	PlanID           *uint    `json:"plan_id"`
	Summary          string   `json:"summary"`
	FocusTaskTitle   string   `json:"focus_task_title"`
	CompletedCount   int      `json:"completed_count"`
	SkippedCount     int      `json:"skipped_count"`
	CompletedTitles  []string `json:"completed_titles"`
	SkippedTitles    []string `json:"skipped_titles"`
	LatestActionText string   `json:"latest_action_text"`
}

// StudyLogResponse 表示返回给前端的学习日志结构。
type StudyLogResponse struct {
	ID               uint      `json:"id"`
	DateKey          string    `json:"date_key"`
	Summary          string    `json:"summary"`
	FocusTaskTitle   string    `json:"focus_task_title"`
	CompletedCount   int       `json:"completed_count"`
	SkippedCount     int       `json:"skipped_count"`
	CompletedTitles  []string  `json:"completed_titles"`
	SkippedTitles    []string  `json:"skipped_titles"`
	LatestActionText string    `json:"latest_action_text"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// GrowthInterviewSnapshot 表示成长档案页所需的最近面试摘要。
type GrowthInterviewSnapshot struct {
	ID             uint       `json:"id"`
	Status         string     `json:"status"`
	Score          float64    `json:"score"`
	TotalQuestions int        `json:"total_questions"`
	CreatedAt      *time.Time `json:"created_at"`
	EndedAt        *time.Time `json:"ended_at"`
}

// GrowthPlanSnapshot 表示成长档案页所需的最近学习计划摘要。
type GrowthPlanSnapshot struct {
	ID             uint       `json:"id"`
	Title          string     `json:"title"`
	Status         string     `json:"status"`
	TotalTasks     int        `json:"total_tasks"`
	CompletedTasks int        `json:"completed_tasks"`
	Progress       float64    `json:"progress"`
	StartDate      *time.Time `json:"start_date"`
	EndDate        *time.Time `json:"end_date"`
}

// GrowthCurrentPlan 表示成长档案页当前主计划的精简摘要。
type GrowthCurrentPlan struct {
	ID                     uint    `json:"id"`
	Title                  string  `json:"title"`
	Status                 string  `json:"status"`
	TotalTasks             int     `json:"total_tasks"`
	CompletedTasks         int     `json:"completed_tasks"`
	Progress               float64 `json:"progress"`
	NextTaskTitle          string  `json:"next_task_title"`
	NextTaskSource         string  `json:"next_task_source,omitempty"`
	NextTaskReason         string  `json:"next_task_reason,omitempty"`
	NextTaskSourceRef      string  `json:"next_task_source_ref,omitempty"`
	NextTaskCollectionHint string  `json:"next_task_collection_hint,omitempty"`
}

// GrowthFocusSignal 表示成长档案首页可直接展示的一条结构化训练重点信号。
type GrowthFocusSignal struct {
	FocusTag                  string   `json:"focus_tag"`
	TopicCode                 string   `json:"topic_code,omitempty"`
	TopicTitle                string   `json:"topic_title,omitempty"`
	TopicProblemPattern       string   `json:"topic_problem_pattern,omitempty"`
	RelatedQuestionSets       []string `json:"related_question_sets"`
	RecommendedActions        []string `json:"recommended_actions"`
	PrimaryQuestionSet        string   `json:"primary_question_set,omitempty"`
	DominantArchivePhase      string   `json:"dominant_archive_phase,omitempty"`
	DominantArchivePhaseLabel string   `json:"dominant_archive_phase_label,omitempty"`
	OccurrenceCount           int      `json:"occurrence_count"`
	ArchiveOccurrenceCount    int      `json:"archive_occurrence_count"`
	InterviewOccurrenceCount  int      `json:"interview_occurrence_count"`
	Source                    string   `json:"source"`
	SourceLabel               string   `json:"source_label"`
	Reason                    string   `json:"reason"`
}

// GrowthTrendSummary 表示成长档案首页用于概括当前训练趋势的摘要块。
type GrowthTrendSummary struct {
	DominantSource      string `json:"dominant_source"`
	DominantSourceLabel string `json:"dominant_source_label"`
	TopFocusTag         string `json:"top_focus_tag,omitempty"`
	TopTopicCode        string `json:"top_topic_code,omitempty"`
	TopTopicTitle       string `json:"top_topic_title,omitempty"`
	Summary             string `json:"summary"`
}

// GrowthSummaryResponse 表示成长档案页首页所需的聚合数据。
type GrowthSummaryResponse struct {
	PracticeStats           *repository.UserPracticeStats `json:"practice_stats"`
	StudyDays               int64                         `json:"study_days"`
	InterviewCount          int64                         `json:"interview_count"`
	CompletedInterviewCount int64                         `json:"completed_interview_count"`
	AverageInterviewScore   float64                       `json:"average_interview_score"`
	PlanCount               int64                         `json:"plan_count"`
	CurrentPlan             *GrowthCurrentPlan            `json:"current_plan"`
	RecentStudyLogs         []StudyLogResponse            `json:"recent_study_logs"`
	RecentInterviews        []GrowthInterviewSnapshot     `json:"recent_interviews"`
	RecentPlans             []GrowthPlanSnapshot          `json:"recent_plans"`
	FocusSignals            []GrowthFocusSignal           `json:"focus_signals"`
	TrendSummary            *GrowthTrendSummary           `json:"trend_summary"`
}

// WeeklyFocusTheme 表示本周最值得集中补强的一项主题。
type WeeklyFocusTheme struct {
	Title                     string   `json:"title"`
	Reason                    string   `json:"reason"`
	Source                    string   `json:"source"`
	SourceLabel               string   `json:"source_label"`
	FocusTags                 []string `json:"focus_tags"`
	TopicCodes                []string `json:"topic_codes"`
	RelatedQuestionSets       []string `json:"related_question_sets"`
	Suggestions               []string `json:"suggestions"`
	DominantArchivePhase      string   `json:"dominant_archive_phase,omitempty"`
	DominantArchivePhaseLabel string   `json:"dominant_archive_phase_label,omitempty"`
	OccurrenceCount           int      `json:"occurrence_count"`
	InterviewOccurrenceCount  int      `json:"interview_occurrence_count"`
}

// WeeklyFocusResponse 表示成长页和学习陪伴页共用的本周重点补强摘要。
type WeeklyFocusResponse struct {
	Themes []WeeklyFocusTheme `json:"themes"`
}

// GrowthService 定义成长档案相关的聚合能力。
type GrowthService interface {
	SyncStudyLog(ctx context.Context, userID uint, req *SyncStudyLogRequest) (*StudyLogResponse, error)
	GetGrowthSummary(ctx context.Context, userID uint) (*GrowthSummaryResponse, error)
	GetWeeklyFocus(ctx context.Context, userID uint) (*WeeklyFocusResponse, error)
}

// growthService 实现成长档案所需的学习日志同步与聚合查询逻辑。
type growthService struct {
	studyLogRepo        repository.StudyLogRepository
	recordRepo          repository.QuestionRecordRepository
	interviewRepo       repository.InterviewRepository
	planRepo            repository.PlanRepository
	taskRepo            repository.PlanTaskRepository
	learningArchiveRepo repository.LearningArchiveRepository
}

// NewGrowthService 创建成长档案服务实例。
func NewGrowthService(
	studyLogRepo repository.StudyLogRepository,
	recordRepo repository.QuestionRecordRepository,
	interviewRepo repository.InterviewRepository,
	planRepo repository.PlanRepository,
	taskRepo repository.PlanTaskRepository,
	learningArchiveRepo repository.LearningArchiveRepository,
) GrowthService {
	return &growthService{
		studyLogRepo:        studyLogRepo,
		recordRepo:          recordRepo,
		interviewRepo:       interviewRepo,
		planRepo:            planRepo,
		taskRepo:            taskRepo,
		learningArchiveRepo: learningArchiveRepo,
	}
}

// SyncStudyLog 将前端当日学习摘要写入服务端，便于跨设备查看成长轨迹。
func (s *growthService) SyncStudyLog(ctx context.Context, userID uint, req *SyncStudyLogRequest) (*StudyLogResponse, error) {
	if req == nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "学习日志参数不能为空")
	}

	logDate, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(req.DateKey), time.Local)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "学习日志日期格式错误")
	}

	completedTitles := sanitizeStudyLogTitles(req.CompletedTitles)
	skippedTitles := sanitizeStudyLogTitles(req.SkippedTitles)
	completedTitlesJSON, err := json.Marshal(completedTitles)
	if err != nil {
		return nil, fmt.Errorf("序列化完成任务标题失败: %w", err)
	}
	skippedTitlesJSON, err := json.Marshal(skippedTitles)
	if err != nil {
		return nil, fmt.Errorf("序列化跳过任务标题失败: %w", err)
	}

	log := &model.StudyLog{
		UserID:              userID,
		PlanID:              req.PlanID,
		LogDate:             logDate,
		Summary:             strings.TrimSpace(req.Summary),
		FocusTaskTitle:      strings.TrimSpace(req.FocusTaskTitle),
		CompletedCount:      maxStudyLogCount(req.CompletedCount, len(completedTitles)),
		SkippedCount:        maxStudyLogCount(req.SkippedCount, len(skippedTitles)),
		CompletedTitlesJSON: string(completedTitlesJSON),
		SkippedTitlesJSON:   string(skippedTitlesJSON),
		LatestActionText:    strings.TrimSpace(req.LatestActionText),
	}

	if err := s.studyLogRepo.UpsertDaily(ctx, log); err != nil {
		return nil, err
	}

	return buildStudyLogResponse(log), nil
}

// GetGrowthSummary 汇总练习、学习计划、面试和学习日志数据，供成长档案页直接渲染。
func (s *growthService) GetGrowthSummary(ctx context.Context, userID uint) (*GrowthSummaryResponse, error) {
	practiceStats, err := s.recordRepo.GetUserStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	studyDays, err := s.studyLogRepo.CountStudyDays(ctx, userID)
	if err != nil {
		return nil, err
	}

	recentLogs, err := s.studyLogRepo.ListRecentByUser(ctx, userID, growthRecentStudyLogLimit)
	if err != nil {
		return nil, err
	}

	metricInterviews, interviewCount, err := s.interviewRepo.ListByUser(ctx, userID, 1, growthInterviewMetricsPageSize)
	if err != nil {
		return nil, err
	}

	recentPlans, planCount, err := s.planRepo.ListByUser(ctx, userID, 1, growthRecentPlanLimit)
	if err != nil {
		return nil, err
	}

	currentPlan, err := s.planRepo.GetCurrentByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	response := &GrowthSummaryResponse{
		PracticeStats:    practiceStats,
		StudyDays:        studyDays,
		InterviewCount:   interviewCount,
		PlanCount:        planCount,
		RecentStudyLogs:  make([]StudyLogResponse, 0, len(recentLogs)),
		RecentInterviews: make([]GrowthInterviewSnapshot, 0, minGrowthCount(len(metricInterviews), growthRecentInterviewLimit)),
		RecentPlans:      make([]GrowthPlanSnapshot, 0, len(recentPlans)),
		FocusSignals:     []GrowthFocusSignal{},
	}

	recentTrendInterviews := make([]model.MockInterview, 0, growthRecentInterviewLimit)
	var completedInterviewCount int64
	var completedInterviewScore float64
	for index, interview := range metricInterviews {
		if interview.Status == model.InterviewStatusCompleted {
			completedInterviewCount++
			completedInterviewScore += interview.Score
		}

		if index < growthRecentInterviewLimit {
			recentTrendInterviews = append(recentTrendInterviews, interview)
			response.RecentInterviews = append(response.RecentInterviews, GrowthInterviewSnapshot{
				ID:             interview.ID,
				Status:         interview.Status,
				Score:          interview.Score,
				TotalQuestions: interview.TotalQuestions,
				CreatedAt:      buildGrowthTimePointer(interview.CreatedAt),
				EndedAt:        interview.EndedAt,
			})
		}
	}

	response.CompletedInterviewCount = completedInterviewCount
	if completedInterviewCount > 0 {
		response.AverageInterviewScore = completedInterviewScore / float64(completedInterviewCount)
	}

	for _, plan := range recentPlans {
		response.RecentPlans = append(response.RecentPlans, GrowthPlanSnapshot{
			ID:             plan.ID,
			Title:          plan.Title,
			Status:         plan.Status,
			TotalTasks:     plan.TotalTasks,
			CompletedTasks: plan.CompletedTasks,
			Progress:       calculatePlanProgress(plan.CompletedTasks, plan.TotalTasks),
			StartDate:      plan.StartDate,
			EndDate:        plan.EndDate,
		})
	}

	for _, log := range recentLogs {
		response.RecentStudyLogs = append(response.RecentStudyLogs, *buildStudyLogResponse(&log))
	}

	archiveEntries := make([]model.LearningArchiveEntry, 0)
	if s.learningArchiveRepo != nil {
		entries, err := s.learningArchiveRepo.ListRecentByUser(ctx, userID, growthWeeklyFocusArchiveLimit, nil)
		if err != nil {
			return nil, err
		}
		archiveEntries = entries
	}
	focusSignals := buildTrainingFocusSignals(archiveEntries, recentTrendInterviews, growthWeeklyFocusThemeLimit)
	response.FocusSignals = buildGrowthFocusSignals(focusSignals)
	response.TrendSummary = buildGrowthTrendSummary(focusSignals)

	if currentPlan != nil {
		tasks, err := s.taskRepo.ListByPlan(ctx, currentPlan.ID)
		if err != nil {
			return nil, err
		}
		nextTask := resolveNextGrowthTask(tasks)
		nextTaskContext := planTaskResponseContext{}
		if nextTask != nil {
			storedContext := readPlanStoredContext(currentPlan.PlanJSON)
			nextTaskContext = buildPlanTaskResponseContext(*nextTask, "", storedContext)
		}

		response.CurrentPlan = &GrowthCurrentPlan{
			ID:                     currentPlan.ID,
			Title:                  currentPlan.Title,
			Status:                 currentPlan.Status,
			TotalTasks:             currentPlan.TotalTasks,
			CompletedTasks:         currentPlan.CompletedTasks,
			Progress:               calculatePlanProgress(currentPlan.CompletedTasks, currentPlan.TotalTasks),
			NextTaskTitle:          resolveNextGrowthTaskTitle(tasks),
			NextTaskSource:         nextTaskContext.Source,
			NextTaskReason:         nextTaskContext.Reason,
			NextTaskSourceRef:      nextTaskContext.SourceRef,
			NextTaskCollectionHint: nextTaskContext.CollectionHint,
		}
	}

	return response, nil
}

// GetWeeklyFocus 聚合最近练习归档和面试报告，给出本周最值得优先补强的主题。
func (s *growthService) GetWeeklyFocus(ctx context.Context, userID uint) (*WeeklyFocusResponse, error) {
	response := &WeeklyFocusResponse{
		Themes: []WeeklyFocusTheme{},
	}

	archiveEntries := make([]model.LearningArchiveEntry, 0)
	if s.learningArchiveRepo != nil {
		entries, err := s.learningArchiveRepo.ListRecentByUser(ctx, userID, growthWeeklyFocusArchiveLimit, nil)
		if err != nil {
			return nil, err
		}
		archiveEntries = entries
	}

	recentInterviews, _, err := s.interviewRepo.ListByUser(ctx, userID, 1, growthRecentInterviewLimit)
	if err != nil {
		return nil, err
	}
	focusSignals := buildTrainingFocusSignals(archiveEntries, recentInterviews, growthWeeklyFocusThemeLimit)
	response.Themes = buildWeeklyFocusThemesFromSignals(focusSignals)
	return response, nil
}

// sanitizeStudyLogTitles 统一清理学习日志中的任务标题列表，避免写入重复和空值。
func sanitizeStudyLogTitles(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// maxStudyLogCount 取请求显式数值和标题列表长度中的较大值，避免前端漏填时统计偏小。
func maxStudyLogCount(explicitCount int, titleCount int) int {
	if explicitCount > titleCount {
		return explicitCount
	}
	return titleCount
}

// calculatePlanProgress 统一计算学习计划进度百分比。
func calculatePlanProgress(completedTasks int, totalTasks int) float64 {
	if totalTasks <= 0 {
		return 0
	}
	return float64(completedTasks) / float64(totalTasks) * 100
}

// resolveNextGrowthTaskTitle 从计划任务列表里找出当前最值得继续推进的下一项任务标题。
func resolveNextGrowthTaskTitle(tasks []model.LearningTask) string {
	if task := resolveNextGrowthTask(tasks); task != nil {
		return task.Title
	}
	return ""
}

// resolveNextGrowthTask 从计划任务列表里找出当前最值得继续推进的下一项任务。
func resolveNextGrowthTask(tasks []model.LearningTask) *model.LearningTask {
	for _, task := range tasks {
		if task.Status != model.TaskStatusCompleted && task.Status != model.TaskStatusSkipped {
			copy := task
			return &copy
		}
	}
	return nil
}

// buildGrowthFocusSignals 将统一训练重点信号映射为成长页首页可直接消费的结构。
func buildGrowthFocusSignals(signals []trainingFocusSignal) []GrowthFocusSignal {
	items := make([]GrowthFocusSignal, 0, len(signals))
	for _, signal := range signals {
		items = append(items, GrowthFocusSignal{
			FocusTag:                  signal.Tag,
			TopicCode:                 signal.TopicCode,
			TopicTitle:                signal.TopicTitle,
			TopicProblemPattern:       signal.TopicProblemPattern,
			RelatedQuestionSets:       append([]string(nil), signal.RelatedQuestionSets...),
			RecommendedActions:        append([]string(nil), signal.RecommendedActions...),
			PrimaryQuestionSet:        signal.PrimaryQuestionSet,
			DominantArchivePhase:      signal.DominantArchivePhase,
			DominantArchivePhaseLabel: signal.DominantArchivePhaseLabel,
			OccurrenceCount:           signal.OccurrenceCount,
			ArchiveOccurrenceCount:    signal.ArchiveOccurrenceCount,
			InterviewOccurrenceCount:  signal.InterviewOccurrenceCount,
			Source:                    signal.Source,
			SourceLabel:               signal.SourceLabel,
			Reason:                    signal.Reason,
		})
	}
	return items
}

// buildGrowthTrendSummary 根据当前最强的训练重点信号生成成长页趋势摘要。
func buildGrowthTrendSummary(signals []trainingFocusSignal) *GrowthTrendSummary {
	if len(signals) == 0 {
		return nil
	}

	top := signals[0]
	topicTitle := top.TopicTitle
	if topicTitle == "" {
		topicTitle = top.Tag
	}

	return &GrowthTrendSummary{
		DominantSource:      top.Source,
		DominantSourceLabel: top.SourceLabel,
		TopFocusTag:         top.Tag,
		TopTopicCode:        top.TopicCode,
		TopTopicTitle:       top.TopicTitle,
		Summary:             fmt.Sprintf("当前最值得优先处理的是“%s”，它主要来自%s，并且已经连续影响最近一轮训练表现。", topicTitle, top.SourceLabel),
	}
}

// buildWeeklyFocusThemesFromSignals 将统一训练重点信号转成本周补强主题。
func buildWeeklyFocusThemesFromSignals(signals []trainingFocusSignal) []WeeklyFocusTheme {
	themes := make([]WeeklyFocusTheme, 0, len(signals))
	for _, signal := range signals {
		title := signal.TopicTitle
		if title == "" {
			title = fmt.Sprintf("补强「%s」", signal.Tag)
		}
		themes = append(themes, WeeklyFocusTheme{
			Title:                     title,
			Reason:                    signal.Reason,
			Source:                    signal.Source,
			SourceLabel:               signal.SourceLabel,
			FocusTags:                 []string{signal.Tag},
			TopicCodes:                sanitizeWeeklyFocusTopicCodes([]string{signal.TopicCode}),
			RelatedQuestionSets:       append([]string(nil), signal.RelatedQuestionSets...),
			Suggestions:               append([]string(nil), signal.RecommendedActions...),
			DominantArchivePhase:      signal.DominantArchivePhase,
			DominantArchivePhaseLabel: signal.DominantArchivePhaseLabel,
			OccurrenceCount:           signal.OccurrenceCount,
			InterviewOccurrenceCount:  signal.InterviewOccurrenceCount,
		})
	}
	return themes
}

// buildStudyLogResponse 将学习日志模型转换为前端可直接消费的响应结构。
func buildStudyLogResponse(log *model.StudyLog) *StudyLogResponse {
	if log == nil {
		return nil
	}

	return &StudyLogResponse{
		ID:               log.ID,
		DateKey:          log.LogDate.Format("2006-01-02"),
		Summary:          strings.TrimSpace(log.Summary),
		FocusTaskTitle:   strings.TrimSpace(log.FocusTaskTitle),
		CompletedCount:   log.CompletedCount,
		SkippedCount:     log.SkippedCount,
		CompletedTitles:  decodeStudyLogTitles(log.CompletedTitlesJSON),
		SkippedTitles:    decodeStudyLogTitles(log.SkippedTitlesJSON),
		LatestActionText: strings.TrimSpace(log.LatestActionText),
		UpdatedAt:        log.UpdatedAt,
	}
}

// decodeStudyLogTitles 解析学习日志中的标题 JSON 字段，失败时回退为空数组。
func decodeStudyLogTitles(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	var titles []string
	if err := json.Unmarshal([]byte(raw), &titles); err != nil {
		return []string{}
	}
	return sanitizeStudyLogTitles(titles)
}

// buildGrowthTimePointer 为面试创建时间生成稳定的时间指针副本。
func buildGrowthTimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}

	clone := value
	return &clone
}

// minGrowthCount 返回两个整数中的较小值，避免切片容量高估。
func minGrowthCount(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

// weeklyInterviewWeaknessStat 表示最近面试报告里聚合出的一个高频薄弱项。
type weeklyInterviewWeaknessStat struct {
	Weakness    string
	Count       int
	TopicCode   string
	Suggestions []string
}

// buildWeeklyFocusThemes 将练习错因和面试薄弱项合并为最多三条可执行的本周补强主题。
func buildWeeklyFocusThemes(
	focusStats []practiceFocusTagStat,
	entries []model.LearningArchiveEntry,
	interviewWeaknessStats []weeklyInterviewWeaknessStat,
) []WeeklyFocusTheme {
	themes := make([]WeeklyFocusTheme, 0, growthWeeklyFocusThemeLimit)
	usedKeys := make(map[string]struct{}, growthWeeklyFocusThemeLimit*2)
	interviewWeaknessMap := make(map[string]weeklyInterviewWeaknessStat, len(interviewWeaknessStats))
	for _, stat := range interviewWeaknessStats {
		interviewWeaknessMap[normalizeWeeklyFocusKey(stat.Weakness)] = stat
	}

	for _, focusStat := range focusStats {
		if len(themes) >= growthWeeklyFocusThemeLimit {
			break
		}
		if isWeeklyFocusThemeUsed(usedKeys, focusStat.Tag, focusStat.TopicCode) {
			continue
		}

		themes = append(themes, buildWeeklyArchiveFocusTheme(
			focusStat,
			entries,
			interviewWeaknessMap[normalizeWeeklyFocusKey(focusStat.Tag)],
		))
		markWeeklyFocusThemeUsed(usedKeys, focusStat.Tag, focusStat.TopicCode)
	}

	for _, weaknessStat := range interviewWeaknessStats {
		if len(themes) >= growthWeeklyFocusThemeLimit {
			break
		}
		if isWeeklyFocusThemeUsed(usedKeys, weaknessStat.Weakness, weaknessStat.TopicCode) {
			continue
		}

		themes = append(themes, buildWeeklyInterviewFocusTheme(weaknessStat))
		markWeeklyFocusThemeUsed(usedKeys, weaknessStat.Weakness, weaknessStat.TopicCode)
	}

	return themes
}

// buildWeeklyArchiveFocusTheme 将高频错因标签整理成前端可直接展示的补强主题。
func buildWeeklyArchiveFocusTheme(
	focusStat practiceFocusTagStat,
	entries []model.LearningArchiveEntry,
	interviewWeaknessStat weeklyInterviewWeaknessStat,
) WeeklyFocusTheme {
	title := fmt.Sprintf("补强「%s」", focusStat.Tag)
	suggestions := collectWeeklyArchiveSuggestions(entries, focusStat.Tag)
	source := "learning_archive"
	sourceLabel := "练习归档"

	if topic, ok := resolveMistakeTopicByTag(focusStat.Tag); ok {
		title = topic.Title
		if len(suggestions) == 0 {
			suggestions = appendUniqueStrings(suggestions, topic.RecommendedActions...)
		}
	}

	reason := fmt.Sprintf("最近学习档案里“%s”累计出现 %d 次，说明这个问题还在反复影响你的输出。", focusStat.Tag, focusStat.Count)
	if interviewWeaknessStat.Count > 0 {
		source = "mixed"
		sourceLabel = "练习 + 面试"
		reason = fmt.Sprintf("最近学习档案里“%s”累计出现 %d 次，而且最近 %d 场面试报告也反复提到它，适合本周优先补强。", focusStat.Tag, focusStat.Count, interviewWeaknessStat.Count)
		suggestions = appendUniqueStrings(suggestions, interviewWeaknessStat.Suggestions...)
	}

	return WeeklyFocusTheme{
		Title:       title,
		Reason:      reason,
		Source:      source,
		SourceLabel: sourceLabel,
		FocusTags:   []string{focusStat.Tag},
		TopicCodes:  sanitizeWeeklyFocusTopicCodes([]string{focusStat.TopicCode}),
		Suggestions: trimWeeklyFocusSuggestions(suggestions),
	}
}

// buildWeeklyInterviewFocusTheme 将面试报告中的薄弱项整理为单独的补强主题。
func buildWeeklyInterviewFocusTheme(stat weeklyInterviewWeaknessStat) WeeklyFocusTheme {
	title := stat.Weakness
	suggestions := trimWeeklyFocusSuggestions(stat.Suggestions)
	if topic, ok := resolveMistakeTopicByTag(stat.Weakness); ok {
		title = topic.Title
		if len(suggestions) == 0 {
			suggestions = trimWeeklyFocusSuggestions(topic.RecommendedActions)
		}
	}

	return WeeklyFocusTheme{
		Title:       title,
		Reason:      fmt.Sprintf("最近 %d 场已完成面试的报告都提到“%s”，建议本周围绕这个薄弱点做一轮专项复盘。", stat.Count, stat.Weakness),
		Source:      "interview_report",
		SourceLabel: "面试报告",
		FocusTags:   []string{stat.Weakness},
		TopicCodes:  sanitizeWeeklyFocusTopicCodes([]string{stat.TopicCode}),
		Suggestions: suggestions,
	}
}

// rankWeeklyInterviewWeaknesses 统计最近已完成面试报告中的高频薄弱项。
func rankWeeklyInterviewWeaknesses(interviews []model.MockInterview) []weeklyInterviewWeaknessStat {
	counts := make(map[string]int)
	suggestionsByWeakness := make(map[string][]string)
	for _, interview := range interviews {
		if interview.Status != model.InterviewStatusCompleted {
			continue
		}

		report, err := parseStoredInterviewReport(interview.ReportJSON)
		if err != nil || report == nil {
			continue
		}

		weaknesses := sanitizeWeeklyFocusTextList(report.Weaknesses)
		reportSuggestions := sanitizeWeeklyFocusTextList(report.Suggestions)
		for _, weakness := range weaknesses {
			counts[weakness]++
			suggestionsByWeakness[weakness] = appendUniqueStrings(suggestionsByWeakness[weakness], reportSuggestions...)
		}
	}

	items := make([]weeklyInterviewWeaknessStat, 0, len(counts))
	for weakness, count := range counts {
		items = append(items, weeklyInterviewWeaknessStat{
			Weakness:    weakness,
			Count:       count,
			TopicCode:   resolveMistakeTopicCodeByTag(weakness),
			Suggestions: trimWeeklyFocusSuggestions(suggestionsByWeakness[weakness]),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Weakness < items[j].Weakness
		}
		return items[i].Count > items[j].Count
	})

	if len(items) > growthWeeklyFocusThemeLimit {
		return items[:growthWeeklyFocusThemeLimit]
	}
	return items
}

// collectWeeklyArchiveSuggestions 提取某个错因标签对应的最近补强建议，便于前端直接展示。
func collectWeeklyArchiveSuggestions(entries []model.LearningArchiveEntry, focusTag string) []string {
	result := make([]string, 0, growthWeeklyFocusSuggestionLimit)
	for _, entry := range entries {
		if !weeklyArchiveEntryContainsTag(entry, focusTag) {
			continue
		}
		result = appendUniqueStrings(result, decodeWeeklyFocusTextList(entry.SuggestionsJSON)...)
		if len(result) >= growthWeeklyFocusSuggestionLimit {
			break
		}
	}
	return trimWeeklyFocusSuggestions(result)
}

// weeklyArchiveEntryContainsTag 判断某条学习档案是否命中了指定错因标签。
func weeklyArchiveEntryContainsTag(entry model.LearningArchiveEntry, focusTag string) bool {
	target := normalizeWeeklyFocusKey(focusTag)
	if target == "" {
		return false
	}

	for _, tag := range decodeWeeklyFocusTextList(entry.MistakeTagsJSON) {
		if normalizeWeeklyFocusKey(tag) == target {
			return true
		}
	}
	return false
}

// decodeWeeklyFocusTextList 解析 JSON 字符串数组，并统一清理空值与重复项。
func decodeWeeklyFocusTextList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []string{}
	}
	return sanitizeWeeklyFocusTextList(values)
}

// sanitizeWeeklyFocusTextList 清理文本列表中的空值和重复项，保证补强主题文案稳定。
func sanitizeWeeklyFocusTextList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := normalizeWeeklyFocusKey(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// trimWeeklyFocusSuggestions 控制补强建议数量，避免单张卡片信息过载。
func trimWeeklyFocusSuggestions(values []string) []string {
	suggestions := sanitizeWeeklyFocusTextList(values)
	if len(suggestions) > growthWeeklyFocusSuggestionLimit {
		return suggestions[:growthWeeklyFocusSuggestionLimit]
	}
	return suggestions
}

// sanitizeWeeklyFocusTopicCodes 过滤空专题编码，保证前端跳转参数稳定。
func sanitizeWeeklyFocusTopicCodes(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// isWeeklyFocusThemeUsed 判断某个主题是否已经被本轮补强结果占用。
func isWeeklyFocusThemeUsed(usedKeys map[string]struct{}, label string, topicCode string) bool {
	labelKey := normalizeWeeklyFocusKey(label)
	if labelKey != "" {
		if _, exists := usedKeys[labelKey]; exists {
			return true
		}
	}

	topicKey := normalizeWeeklyFocusKey(topicCode)
	if topicKey != "" {
		if _, exists := usedKeys[topicKey]; exists {
			return true
		}
	}
	return false
}

// markWeeklyFocusThemeUsed 记录已经输出过的主题，避免错因标签和面试薄弱项重复展示。
func markWeeklyFocusThemeUsed(usedKeys map[string]struct{}, label string, topicCode string) {
	if labelKey := normalizeWeeklyFocusKey(label); labelKey != "" {
		usedKeys[labelKey] = struct{}{}
	}
	if topicKey := normalizeWeeklyFocusKey(topicCode); topicKey != "" {
		usedKeys[topicKey] = struct{}{}
	}
}

// normalizeWeeklyFocusKey 统一本周补强主题的去重键。
func normalizeWeeklyFocusKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
