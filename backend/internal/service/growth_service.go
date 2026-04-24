// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"encoding/json"
	"fmt"
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
	ID             uint    `json:"id"`
	Title          string  `json:"title"`
	Status         string  `json:"status"`
	TotalTasks     int     `json:"total_tasks"`
	CompletedTasks int     `json:"completed_tasks"`
	Progress       float64 `json:"progress"`
	NextTaskTitle  string  `json:"next_task_title"`
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
}

// GrowthService 定义成长档案相关的聚合能力。
type GrowthService interface {
	SyncStudyLog(ctx context.Context, userID uint, req *SyncStudyLogRequest) (*StudyLogResponse, error)
	GetGrowthSummary(ctx context.Context, userID uint) (*GrowthSummaryResponse, error)
}

// growthService 实现成长档案所需的学习日志同步与聚合查询逻辑。
type growthService struct {
	studyLogRepo  repository.StudyLogRepository
	recordRepo    repository.QuestionRecordRepository
	interviewRepo repository.InterviewRepository
	planRepo      repository.PlanRepository
	taskRepo      repository.PlanTaskRepository
}

// NewGrowthService 创建成长档案服务实例。
func NewGrowthService(
	studyLogRepo repository.StudyLogRepository,
	recordRepo repository.QuestionRecordRepository,
	interviewRepo repository.InterviewRepository,
	planRepo repository.PlanRepository,
	taskRepo repository.PlanTaskRepository,
) GrowthService {
	return &growthService{
		studyLogRepo:  studyLogRepo,
		recordRepo:    recordRepo,
		interviewRepo: interviewRepo,
		planRepo:      planRepo,
		taskRepo:      taskRepo,
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
	}

	var completedInterviewCount int64
	var completedInterviewScore float64
	for index, interview := range metricInterviews {
		if interview.Status == model.InterviewStatusCompleted {
			completedInterviewCount++
			completedInterviewScore += interview.Score
		}

		if index < growthRecentInterviewLimit {
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

	if currentPlan != nil {
		tasks, err := s.taskRepo.ListByPlan(ctx, currentPlan.ID)
		if err != nil {
			return nil, err
		}

		response.CurrentPlan = &GrowthCurrentPlan{
			ID:             currentPlan.ID,
			Title:          currentPlan.Title,
			Status:         currentPlan.Status,
			TotalTasks:     currentPlan.TotalTasks,
			CompletedTasks: currentPlan.CompletedTasks,
			Progress:       calculatePlanProgress(currentPlan.CompletedTasks, currentPlan.TotalTasks),
			NextTaskTitle:  resolveNextGrowthTaskTitle(tasks),
		}
	}

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
	for _, task := range tasks {
		if task.Status != model.TaskStatusCompleted && task.Status != model.TaskStatusSkipped {
			return task.Title
		}
	}
	return ""
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
