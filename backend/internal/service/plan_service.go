// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

// PlanService 学习计划服务接口
type PlanService interface {
	GeneratePlan(ctx context.Context, userID uint, req *GeneratePlanRequest) (*PlanDetailResponse, error)
	GetCurrentPlan(ctx context.Context, userID uint) (*PlanDetailResponse, error)
	GetPlan(ctx context.Context, userID, planID uint) (*PlanDetailResponse, error)
	ListPlans(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error)
	UpdateTaskStatus(ctx context.Context, userID, planID, taskID uint, req *UpdateTaskStatusRequest) error
	AdjustPlan(ctx context.Context, userID, planID uint) (*PlanDetailResponse, error)
	GetProgress(ctx context.Context, userID, planID uint) (*PlanProgressResponse, error)
}

// GeneratePlanRequest 生成学习计划请求DTO
type GeneratePlanRequest struct {
	Level           string   `json:"level" binding:"required,oneof=beginner intermediate advanced"`
	DailyStudyTime  int      `json:"daily_study_time" binding:"required,min=15,max=480"` // 分钟
	WeakTopics      []string `json:"weak_topics"`
	GoalDescription string   `json:"goal_description"`
	DurationDays    int      `json:"duration_days" binding:"required,min=7,max=90"`
	IndustryID      uint     `json:"industry_id"`
	IndustryCode    string   `json:"industry_code"`
}

// UpdateTaskStatusRequest 更新任务状态请求DTO
type UpdateTaskStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=pending in_progress completed skipped"`
}

// PlanDetailResponse 学习计划详情响应DTO
type PlanDetailResponse struct {
	ID             uint           `json:"id"`
	IndustryID     uint           `json:"industry_id"`
	IndustryCode   string         `json:"industry_code"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Status         string         `json:"status"`
	TotalTasks     int            `json:"total_tasks"`
	CompletedTasks int            `json:"completed_tasks"`
	Progress       float64        `json:"progress"` // 0-100
	StartDate      *time.Time     `json:"start_date"`
	EndDate        *time.Time     `json:"end_date"`
	Tasks          []TaskResponse `json:"tasks"`
	CreatedAt      time.Time      `json:"created_at"`
}

// TaskResponse 任务响应DTO
type TaskResponse struct {
	ID                  uint       `json:"id"`
	Title               string     `json:"title"`
	Description         string     `json:"description"`
	TaskType            string     `json:"task_type"`
	Status              string     `json:"status"`
	DueDate             *time.Time `json:"due_date"`
	CompletedAt         *time.Time `json:"completed_at"`
	DayNumber           int        `json:"day_number"`
	SortOrder           int        `json:"sort_order"`
	Source              string     `json:"source"`
	SourceLabel         string     `json:"source_label"`
	Reason              string     `json:"reason"`
	PriorityExplanation string     `json:"priority_explanation"`
	SourceRef           string     `json:"source_ref,omitempty"`
	CollectionHint      string     `json:"collection_hint,omitempty"`
}

// PlanProgressResponse 学习进度统计响应DTO
type PlanProgressResponse struct {
	PlanID          uint            `json:"plan_id"`
	TotalTasks      int             `json:"total_tasks"`
	CompletedTasks  int             `json:"completed_tasks"`
	SkippedTasks    int             `json:"skipped_tasks"`
	InProgressTasks int             `json:"in_progress_tasks"`
	PendingTasks    int             `json:"pending_tasks"`
	Progress        float64         `json:"progress"`
	DailyProgress   []DailyProgress `json:"daily_progress"`
	TaskTypeStats   []TaskTypeStat  `json:"task_type_stats"`
}

// DailyProgress 每日进度
type DailyProgress struct {
	DayNumber int `json:"day_number"`
	Total     int `json:"total"`
	Completed int `json:"completed"`
}

// TaskTypeStat 任务类型统计
type TaskTypeStat struct {
	TaskType  string `json:"task_type"`
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
}

// planService 学习计划服务实现
type planService struct {
	planRepo            repository.PlanRepository
	taskRepo            repository.PlanTaskRepository
	planAgent           ai.PlanAgent
	learningArchiveRepo repository.LearningArchiveRepository
	interviewRepo       repository.InterviewRepository
	industryRepo        repository.IndustryRepository
}

// NewPlanService 创建学习计划服务实例
func NewPlanService(
	planRepo repository.PlanRepository,
	taskRepo repository.PlanTaskRepository,
	planAgent ai.PlanAgent,
	learningArchiveRepo repository.LearningArchiveRepository,
	interviewRepo repository.InterviewRepository,
	industryRepo ...repository.IndustryRepository,
) PlanService {
	s := &planService{
		planRepo:            planRepo,
		taskRepo:            taskRepo,
		planAgent:           planAgent,
		learningArchiveRepo: learningArchiveRepo,
		interviewRepo:       interviewRepo,
	}
	if len(industryRepo) > 0 {
		s.industryRepo = industryRepo[0]
	}
	return s
}

// GeneratePlan 生成学习计划
func (s *planService) GeneratePlan(ctx context.Context, userID uint, req *GeneratePlanRequest) (*PlanDetailResponse, error) {
	industryID, industryCode, err := s.resolvePlanIndustry(ctx, req)
	if err != nil {
		return nil, err
	}

	// 构建用户画像
	focusSignals, err := s.buildLatestPlanFocusSignals(ctx, userID, industryID, industryCode)
	if err != nil {
		return nil, err
	}
	profile := ai.UserProfile{
		Level:           req.Level,
		WeakTopics:      mergePlanProfileWeakTopics(req.WeakTopics, focusSignals),
		DailyStudyTime:  req.DailyStudyTime,
		DurationDays:    req.DurationDays,
		GoalDescription: req.GoalDescription,
	}

	// 调用AI生成学习计划
	aiPlan, err := s.planAgent.GeneratePlan(ctx, profile, industryCode)
	if err != nil {
		return nil, fmt.Errorf("AI生成学习计划失败: %w", err)
	}

	// 暂停用户的其他活跃计划
	if err := s.planRepo.PauseActivePlans(ctx, userID); err != nil {
		return nil, err
	}

	// 计算开始和结束日期
	now := time.Now()
	startDate := now
	endDate := now.AddDate(0, 0, req.DurationDays)

	// 序列化计划JSON
	storedContext := buildPlanStoredContext(req, industryCode)
	storedContext.FocusSignals = normalizeTrainingFocusSignals(focusSignals)
	planJSON, err := buildPlanStoredPayload(aiPlan, storedContext)
	if err != nil {
		return nil, fmt.Errorf("序列化计划失败: %w", err)
	}

	// 创建学习计划记录
	plan := &model.LearningPlan{
		UserID:         userID,
		IndustryID:     industryID,
		Title:          aiPlan.Title,
		Description:    aiPlan.Description,
		PlanJSON:       string(planJSON),
		Status:         model.PlanStatusActive,
		TotalTasks:     len(aiPlan.Tasks),
		CompletedTasks: 0,
		StartDate:      &startDate,
		EndDate:        &endDate,
	}

	if err := s.planRepo.Create(ctx, plan); err != nil {
		return nil, err
	}

	// 创建任务记录
	tasks := make([]model.LearningTask, 0, len(aiPlan.Tasks))
	for i, t := range aiPlan.Tasks {
		dueDate := startDate.AddDate(0, 0, t.DayNumber-1)
		task := model.LearningTask{
			PlanID:      plan.ID,
			Title:       t.Title,
			Description: t.Description,
			TaskType:    t.TaskType,
			Status:      model.TaskStatusPending,
			DueDate:     &dueDate,
			SortOrder:   i,
		}
		tasks = append(tasks, task)
	}

	if err := s.taskRepo.BatchCreate(ctx, tasks); err != nil {
		return nil, err
	}

	// 返回计划详情
	return s.buildPlanDetailResponse(ctx, plan, tasks)
}

// resolvePlanIndustry 解析学习计划使用的行业信息，优先按行业编码查真实主键并兼容旧版行业ID请求。
func (s *planService) resolvePlanIndustry(ctx context.Context, req *GeneratePlanRequest) (uint, string, error) {
	if req == nil {
		return 0, "", common.NewBusinessError(common.CodeBadRequest, "学习计划参数不能为空")
	}

	requestedCode := strings.TrimSpace(req.IndustryCode)
	if requestedCode != "" && s.industryRepo != nil {
		industry, err := s.industryRepo.GetByCode(ctx, requestedCode)
		if err != nil {
			return 0, "", fmt.Errorf("查询行业失败: %w", err)
		}
		if industry != nil {
			return industry.ID, industry.Code, nil
		}
	}

	if req.IndustryID > 0 {
		if s.industryRepo == nil {
			if requestedCode == "" {
				return req.IndustryID, "go", nil
			}
			return req.IndustryID, requestedCode, nil
		}

		industry, err := s.industryRepo.GetByID(ctx, req.IndustryID)
		if err != nil {
			return 0, "", fmt.Errorf("查询行业失败: %w", err)
		}
		if industry != nil {
			return industry.ID, industry.Code, nil
		}
	}

	if requestedCode != "" || req.IndustryID > 0 {
		return 0, "", common.NewBusinessError(common.CodeBadRequest, "所选学习方向不存在")
	}

	if s.industryRepo == nil {
		return 0, "", common.NewBusinessError(common.CodeBadRequest, "缺少有效的学习方向")
	}

	industry, err := s.industryRepo.GetByCode(ctx, "go")
	if err != nil {
		return 0, "", fmt.Errorf("查询默认行业失败: %w", err)
	}
	if industry == nil {
		return 0, "", common.NewBusinessError(common.CodeBadRequest, "默认学习方向不存在，请先初始化行业数据")
	}

	return industry.ID, industry.Code, nil
}

// GetCurrentPlan 获取当前学习计划
func (s *planService) GetCurrentPlan(ctx context.Context, userID uint) (*PlanDetailResponse, error) {
	plan, err := s.planRepo.GetCurrentByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "当前没有进行中的学习计划")
	}

	tasks, err := s.taskRepo.ListByPlan(ctx, plan.ID)
	if err != nil {
		return nil, err
	}

	return s.buildPlanDetailResponse(ctx, plan, tasks)
}

// GetPlan 获取指定学习计划
func (s *planService) GetPlan(ctx context.Context, userID, planID uint) (*PlanDetailResponse, error) {
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "学习计划不存在")
	}

	// 验证计划归属
	if plan.UserID != userID {
		return nil, common.NewBusinessError(common.CodeForbidden, "无权访问该学习计划")
	}

	tasks, err := s.taskRepo.ListByPlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	return s.buildPlanDetailResponse(ctx, plan, tasks)
}

// ListPlans 获取学习计划列表
func (s *planService) ListPlans(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error) {
	pageParam := common.PageParam{Page: page, PageSize: pageSize}
	pageParam.Normalize()

	plans, total, err := s.planRepo.ListByUser(ctx, userID, pageParam.Page, pageParam.PageSize)
	if err != nil {
		return nil, err
	}

	return common.NewPageResult(plans, total, pageParam), nil
}

// UpdateTaskStatus 更新任务状态
func (s *planService) UpdateTaskStatus(ctx context.Context, userID, planID, taskID uint, req *UpdateTaskStatusRequest) error {
	// 验证计划归属
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return err
	}
	if plan == nil {
		return common.NewBusinessError(common.CodeNotFound, "学习计划不存在")
	}
	if plan.UserID != userID {
		return common.NewBusinessError(common.CodeForbidden, "无权访问该学习计划")
	}

	// 获取任务
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return common.NewBusinessError(common.CodeNotFound, "学习任务不存在")
	}

	// 验证任务归属
	if task.PlanID != planID {
		return common.NewBusinessError(common.CodeBadRequest, "任务不属于该计划")
	}

	// 更新任务状态
	task.Status = req.Status
	if req.Status == model.TaskStatusCompleted {
		now := time.Now()
		task.CompletedAt = &now
	} else {
		task.CompletedAt = nil
	}

	if err := s.taskRepo.Update(ctx, task); err != nil {
		return err
	}

	// 更新计划的已完成任务数
	tasks, err := s.taskRepo.ListByPlan(ctx, planID)
	if err != nil {
		return err
	}

	completedCount := 0
	for _, t := range tasks {
		if t.Status == model.TaskStatusCompleted {
			completedCount++
		}
	}
	plan.CompletedTasks = completedCount

	// 如果所有任务都完成了，更新计划状态为completed
	if completedCount >= plan.TotalTasks {
		plan.Status = model.PlanStatusCompleted
	} else {
		plan.Status = model.PlanStatusActive
	}

	return s.planRepo.Update(ctx, plan)
}

// AdjustPlan 动态调整学习计划
func (s *planService) AdjustPlan(ctx context.Context, userID, planID uint) (*PlanDetailResponse, error) {
	// 验证计划归属
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "学习计划不存在")
	}
	if plan.UserID != userID {
		return nil, common.NewBusinessError(common.CodeForbidden, "无权访问该学习计划")
	}

	// 获取所有任务
	tasks, err := s.taskRepo.ListByPlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	// 收集已完成任务和计算表现
	completedTasks := make([]string, 0)
	completedTaskModels := make([]model.LearningTask, 0)
	performance := make(map[string]float64)
	taskTypeScores := make(map[string][]float64)

	for _, t := range tasks {
		if t.Status == model.TaskStatusCompleted {
			completedTaskModels = append(completedTaskModels, t)
			completedTasks = append(completedTasks, t.Title)
			// 简化：假设每种类型任务完成得分为80
			taskTypeScores[t.TaskType] = append(taskTypeScores[t.TaskType], 80)
		}
	}

	// 计算各类型平均分
	for taskType, scores := range taskTypeScores {
		if len(scores) > 0 {
			sum := 0.0
			for _, s := range scores {
				sum += s
			}
			performance[taskType] = sum / float64(len(scores))
		}
	}

	// 调用AI调整计划
	adjustedPlan, err := s.planAgent.AdjustPlan(ctx, fmt.Sprintf("%d", planID), completedTasks, performance)
	if err != nil {
		return nil, fmt.Errorf("AI调整学习计划失败: %w", err)
	}

	// 删除未完成的任务，保留已完成历史记录。
	if err := s.taskRepo.DeleteIncompleteByPlan(ctx, planID); err != nil {
		return nil, err
	}

	// 创建调整后的任务
	newTasks := make([]model.LearningTask, 0, len(adjustedPlan.Tasks))
	nextSortOrder := 0
	if len(completedTaskModels) > 0 {
		nextSortOrder = completedTaskModels[len(completedTaskModels)-1].SortOrder + 1
	}
	for i, t := range adjustedPlan.Tasks {
		dueDate := time.Now().AddDate(0, 0, t.DayNumber-1)
		task := model.LearningTask{
			PlanID:      planID,
			Title:       t.Title,
			Description: t.Description,
			TaskType:    t.TaskType,
			Status:      model.TaskStatusPending,
			DueDate:     &dueDate,
			SortOrder:   nextSortOrder + i,
		}
		newTasks = append(newTasks, task)
	}

	if err := s.taskRepo.BatchCreate(ctx, newTasks); err != nil {
		return nil, err
	}

	// 更新计划信息
	plan.Title = adjustedPlan.Title
	plan.Description = adjustedPlan.Description
	plan.CompletedTasks = len(completedTaskModels)
	plan.TotalTasks = len(completedTaskModels) + len(adjustedPlan.Tasks)
	endDate := time.Now().AddDate(0, 0, adjustedPlan.Duration)
	plan.EndDate = &endDate
	if plan.CompletedTasks >= plan.TotalTasks && plan.TotalTasks > 0 {
		plan.Status = model.PlanStatusCompleted
	} else {
		plan.Status = model.PlanStatusActive
	}

	// 更新计划JSON
	storedContext := readPlanStoredContext(plan.PlanJSON)
	if strings.TrimSpace(storedContext.IndustryCode) == "" {
		storedContext.IndustryCode = s.resolvePlanIndustryCode(ctx, plan.IndustryID)
	}
	focusSignals, err := s.buildLatestPlanFocusSignals(ctx, userID, plan.IndustryID, storedContext.IndustryCode)
	if err != nil {
		return nil, err
	}
	storedContext.FocusSignals = normalizeTrainingFocusSignals(focusSignals)
	planJSON, _ := buildPlanStoredPayload(adjustedPlan, storedContext)
	plan.PlanJSON = string(planJSON)

	if err := s.planRepo.Update(ctx, plan); err != nil {
		return nil, err
	}

	allTasks := make([]model.LearningTask, 0, len(completedTaskModels)+len(newTasks))
	allTasks = append(allTasks, completedTaskModels...)
	allTasks = append(allTasks, newTasks...)

	return s.buildPlanDetailResponse(ctx, plan, allTasks)
}

// GetProgress 获取学习进度统计
func (s *planService) GetProgress(ctx context.Context, userID, planID uint) (*PlanProgressResponse, error) {
	// 验证计划归属
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "学习计划不存在")
	}
	if plan.UserID != userID {
		return nil, common.NewBusinessError(common.CodeForbidden, "无权访问该学习计划")
	}

	// 获取所有任务
	tasks, err := s.taskRepo.ListByPlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	// 统计各状态任务数
	var completedCount, skippedCount, inProgressCount, pendingCount int
	dailyStats := make(map[int]*dailyStat)
	taskTypeStats := make(map[string]*taskTypeStatData)

	for _, t := range tasks {
		switch t.Status {
		case model.TaskStatusCompleted:
			completedCount++
		case model.TaskStatusSkipped:
			skippedCount++
		case model.TaskStatusInProgress:
			inProgressCount++
		case model.TaskStatusPending:
			pendingCount++
		}

		// 计算DayNumber（从DueDate反推）
		dayNumber := 1
		if t.DueDate != nil && plan.StartDate != nil {
			dayNumber = int(t.DueDate.Sub(*plan.StartDate).Hours()/24) + 1
		}

		// 更新每日统计
		if ds, ok := dailyStats[dayNumber]; ok {
			ds.total++
			if t.Status == model.TaskStatusCompleted {
				ds.completed++
			}
		} else {
			dailyStats[dayNumber] = &dailyStat{
				total:     1,
				completed: boolToInt(t.Status == model.TaskStatusCompleted),
			}
		}

		// 更新任务类型统计
		if ts, ok := taskTypeStats[t.TaskType]; ok {
			ts.total++
			if t.Status == model.TaskStatusCompleted {
				ts.completed++
			}
		} else {
			taskTypeStats[t.TaskType] = &taskTypeStatData{
				total:     1,
				completed: boolToInt(t.Status == model.TaskStatusCompleted),
			}
		}
	}

	// 计算进度
	totalTasks := len(tasks)
	var progress float64
	if totalTasks > 0 {
		progress = float64(completedCount) / float64(totalTasks) * 100
	}

	// 构建每日进度
	dailyProgress := make([]DailyProgress, 0)
	for day, stats := range dailyStats {
		dailyProgress = append(dailyProgress, DailyProgress{
			DayNumber: day,
			Total:     stats.total,
			Completed: stats.completed,
		})
	}

	// 构建任务类型统计
	taskTypeStatList := make([]TaskTypeStat, 0)
	for taskType, stats := range taskTypeStats {
		taskTypeStatList = append(taskTypeStatList, TaskTypeStat{
			TaskType:  taskType,
			Total:     stats.total,
			Completed: stats.completed,
		})
	}

	return &PlanProgressResponse{
		PlanID:          planID,
		TotalTasks:      totalTasks,
		CompletedTasks:  completedCount,
		SkippedTasks:    skippedCount,
		InProgressTasks: inProgressCount,
		PendingTasks:    pendingCount,
		Progress:        progress,
		DailyProgress:   dailyProgress,
		TaskTypeStats:   taskTypeStatList,
	}, nil
}

// buildPlanDetailResponse 构建计划详情响应。
func (s *planService) buildPlanDetailResponse(ctx context.Context, plan *model.LearningPlan, tasks []model.LearningTask) (*PlanDetailResponse, error) {
	storedPlan := readPlanStoredPayload(plan.PlanJSON)
	taskPriorityQueue := buildPlanTaskPriorityQueue(storedPlan.Plan)
	taskResponses := make([]TaskResponse, 0, len(tasks))
	for _, t := range tasks {
		// 计算DayNumber
		dayNumber := 1
		if t.DueDate != nil && plan.StartDate != nil {
			dayNumber = int(t.DueDate.Sub(*plan.StartDate).Hours()/24) + 1
		}
		if dayNumber < 1 {
			dayNumber = 1
		}

		priority := popPlanTaskPriority(taskPriorityQueue, buildPlanTaskLookupKey(t.Title, t.Description, t.TaskType))
		taskContext := buildPlanTaskResponseContext(t, priority, storedPlan.Context)

		taskResponses = append(taskResponses, TaskResponse{
			ID:                  t.ID,
			Title:               t.Title,
			Description:         t.Description,
			TaskType:            t.TaskType,
			Status:              t.Status,
			DueDate:             t.DueDate,
			CompletedAt:         t.CompletedAt,
			DayNumber:           dayNumber,
			SortOrder:           t.SortOrder,
			Source:              taskContext.Source,
			SourceLabel:         taskContext.SourceLabel,
			Reason:              taskContext.Reason,
			PriorityExplanation: taskContext.PriorityExplanation,
			SourceRef:           taskContext.SourceRef,
			CollectionHint:      taskContext.CollectionHint,
		})
	}

	// 计算进度
	var progress float64
	if plan.TotalTasks > 0 {
		progress = float64(plan.CompletedTasks) / float64(plan.TotalTasks) * 100
	}

	return &PlanDetailResponse{
		ID:             plan.ID,
		IndustryID:     plan.IndustryID,
		IndustryCode:   s.resolvePlanIndustryCode(ctx, plan.IndustryID),
		Title:          plan.Title,
		Description:    plan.Description,
		Status:         plan.Status,
		TotalTasks:     plan.TotalTasks,
		CompletedTasks: plan.CompletedTasks,
		Progress:       progress,
		StartDate:      plan.StartDate,
		EndDate:        plan.EndDate,
		Tasks:          taskResponses,
		CreatedAt:      plan.CreatedAt,
	}, nil
}

// resolvePlanIndustryCode 根据计划记录中的行业ID反查行业编码，失败时返回空字符串。
func (s *planService) resolvePlanIndustryCode(ctx context.Context, industryID uint) string {
	if industryID == 0 || s.industryRepo == nil {
		return ""
	}

	industry, err := s.industryRepo.GetByID(ctx, industryID)
	if err != nil || industry == nil {
		return ""
	}

	return industry.Code
}

// dailyStat 每日统计数据
type dailyStat struct {
	total     int
	completed int
}

// taskTypeStatData 任务类型统计数据
type taskTypeStatData struct {
	total     int
	completed int
}

// planStoredPayload 表示持久化到 learning_plans.plan_json 的增强计划结构。
type planStoredPayload struct {
	Plan    ai.LearningPlan   `json:"plan"`
	Context planStoredContext `json:"context"`
}

// planStoredContext 表示用于回填任务解释信息的最小计划上下文。
type planStoredContext struct {
	IndustryCode    string                `json:"industry_code"`
	Level           string                `json:"level"`
	WeakTopics      []string              `json:"weak_topics"`
	GoalDescription string                `json:"goal_description"`
	DailyStudyTime  int                   `json:"daily_study_time"`
	DurationDays    int                   `json:"duration_days"`
	FocusSignals    []trainingFocusSignal `json:"focus_signals,omitempty"`
}

// planTaskResponseContext 表示单个任务返回给前端时附带的解释信息。
type planTaskResponseContext struct {
	Source              string
	SourceLabel         string
	Reason              string
	PriorityExplanation string
	SourceRef           string
	CollectionHint      string
}

// buildPlanStoredContext 根据当前请求构造可持久化的计划上下文，供后续详情接口稳定回填解释字段。
func buildPlanStoredContext(req *GeneratePlanRequest, industryCode string) planStoredContext {
	if req == nil {
		return planStoredContext{}
	}

	return planStoredContext{
		IndustryCode:    strings.TrimSpace(industryCode),
		Level:           strings.TrimSpace(req.Level),
		WeakTopics:      sanitizePlanContextTopics(req.WeakTopics),
		GoalDescription: strings.TrimSpace(req.GoalDescription),
		DailyStudyTime:  req.DailyStudyTime,
		DurationDays:    req.DurationDays,
	}
}

// buildPlanStoredPayload 将学习计划和解释上下文序列化为增强版 plan_json 持久化载荷。
func buildPlanStoredPayload(plan ai.LearningPlan, context planStoredContext) ([]byte, error) {
	return json.Marshal(planStoredPayload{
		Plan:    plan,
		Context: normalizePlanStoredContext(context),
	})
}

// readPlanStoredPayload 兼容解析新旧两种 plan_json 结构，旧数据会自动回退为仅含计划主体的结构。
func readPlanStoredPayload(raw string) planStoredPayload {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return planStoredPayload{}
	}

	var stored planStoredPayload
	if err := json.Unmarshal([]byte(trimmed), &stored); err == nil && !isEmptyLearningPlan(stored.Plan) {
		stored.Context = normalizePlanStoredContext(stored.Context)
		return stored
	}

	var legacyPlan ai.LearningPlan
	if err := json.Unmarshal([]byte(trimmed), &legacyPlan); err == nil {
		return planStoredPayload{
			Plan:    legacyPlan,
			Context: planStoredContext{},
		}
	}

	return planStoredPayload{}
}

// readPlanStoredContext 从 plan_json 中提取解释上下文，供计划调整后延续原有解释依据。
func readPlanStoredContext(raw string) planStoredContext {
	return readPlanStoredPayload(raw).Context
}

// normalizePlanStoredContext 统一清理持久化上下文中的可读字段，避免空白和重复值污染解释结果。
func normalizePlanStoredContext(context planStoredContext) planStoredContext {
	context.IndustryCode = strings.TrimSpace(context.IndustryCode)
	context.Level = strings.TrimSpace(context.Level)
	context.GoalDescription = strings.TrimSpace(context.GoalDescription)
	context.WeakTopics = sanitizePlanContextTopics(context.WeakTopics)
	context.FocusSignals = normalizeTrainingFocusSignals(context.FocusSignals)
	if context.DailyStudyTime < 0 {
		context.DailyStudyTime = 0
	}
	if context.DurationDays < 0 {
		context.DurationDays = 0
	}
	return context
}

// sanitizePlanContextTopics 清理计划上下文中的弱项标签列表，避免重复值和空白项进入解释规则。
func sanitizePlanContextTopics(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// isEmptyLearningPlan 判断学习计划主体是否为空，避免把新结构解析失败误判为合法结果。
func isEmptyLearningPlan(plan ai.LearningPlan) bool {
	return strings.TrimSpace(plan.Title) == "" &&
		strings.TrimSpace(plan.Description) == "" &&
		plan.Duration == 0 &&
		len(plan.Tasks) == 0
}

// buildPlanTaskPriorityQueue 将持久化计划任务整理为按任务签名索引的优先级队列，便于响应阶段回填解释信息。
func buildPlanTaskPriorityQueue(plan ai.LearningPlan) map[string][]string {
	queue := make(map[string][]string, len(plan.Tasks))
	for _, task := range plan.Tasks {
		key := buildPlanTaskLookupKey(task.Title, task.Description, task.TaskType)
		queue[key] = append(queue[key], strings.TrimSpace(task.Priority))
	}
	return queue
}

// buildPlanTaskLookupKey 为计划任务生成稳定签名，便于在模型任务和持久化计划任务之间建立对应关系。
func buildPlanTaskLookupKey(title string, description string, taskType string) string {
	return strings.Join([]string{
		strings.TrimSpace(title),
		strings.TrimSpace(description),
		strings.TrimSpace(taskType),
	}, "||")
}

// popPlanTaskPriority 按任务签名弹出一条优先级记录，兼容同名任务重复出现时的顺序消费。
func popPlanTaskPriority(queue map[string][]string, key string) string {
	if len(queue) == 0 {
		return ""
	}

	items := queue[key]
	if len(items) == 0 {
		return ""
	}

	priority := items[0]
	if len(items) == 1 {
		delete(queue, key)
	} else {
		queue[key] = items[1:]
	}
	return priority
}

// buildPlanTaskResponseContext 根据任务内容、持久化上下文和优先级生成前端可直接消费的解释字段。
func buildPlanTaskResponseContext(task model.LearningTask, priority string, context planStoredContext) planTaskResponseContext {
	if signal := matchTrainingFocusSignal(task, context.FocusSignals); signal != nil {
		source := "weekly_focus"
		sourceLabel := "本周重点补强"
		reason := fmt.Sprintf("该任务围绕当前高频问题“%s”展开，用于先补方法和思路，再进入后续训练。", signal.Tag)
		if task.TaskType == model.TaskTypePractice || task.TaskType == model.TaskTypeInterview {
			source = "practice_recommendation"
			sourceLabel = "专项练习推荐"
			reason = fmt.Sprintf("该任务直接对应当前高频问题“%s”，建议通过专项练习或模拟面试尽快补齐。", signal.Tag)
		}
		return planTaskResponseContext{
			Source:              source,
			SourceLabel:         sourceLabel,
			Reason:              reason,
			PriorityExplanation: buildPlanTaskPriorityExplanation(priority, source),
			SourceRef:           signal.SourceRef,
			CollectionHint:      signal.CollectionHint,
		}
	}

	matchedWeakTopic := matchPlanWeakTopic(task, context.WeakTopics)
	source := "default"
	sourceLabel := "基础默认任务"
	reason := "该任务用于维持当前学习计划的基础推进节奏，先补齐必要铺垫再进入后续训练。"

	if matchedWeakTopic != "" {
		source = "weak_topic"
		sourceLabel = "弱项补强"
		reason = fmt.Sprintf("该任务直接围绕当前弱项“%s”安排，用于优先补齐这一块的短板。", matchedWeakTopic)
	} else if strings.TrimSpace(context.GoalDescription) != "" {
		source = "goal"
		sourceLabel = "目标拆解"
		reason = fmt.Sprintf("该任务服务于当前学习目标“%s”，用于把大目标拆成可执行的当下动作。", strings.TrimSpace(context.GoalDescription))
	}

	return planTaskResponseContext{
		Source:              source,
		SourceLabel:         sourceLabel,
		Reason:              reason,
		PriorityExplanation: buildPlanTaskPriorityExplanation(priority, source),
		SourceRef:           "",
		CollectionHint:      "",
	}
}

// matchPlanWeakTopic 从任务标题和描述中匹配最贴近的弱项标签，命中后用于标记任务来源。
func matchPlanWeakTopic(task model.LearningTask, weakTopics []string) string {
	searchText := strings.ToLower(strings.Join([]string{
		strings.TrimSpace(task.Title),
		strings.TrimSpace(task.Description),
	}, "\n"))
	for _, topic := range weakTopics {
		normalizedTopic := strings.ToLower(strings.TrimSpace(topic))
		if normalizedTopic == "" {
			continue
		}
		if strings.Contains(searchText, normalizedTopic) {
			return topic
		}
	}
	return ""
}

// buildPlanTaskPriorityExplanation 根据计划优先级和任务来源生成更可解释的排序说明。
func buildPlanTaskPriorityExplanation(priority string, source string) string {
	switch strings.TrimSpace(priority) {
	case "high":
		if source == "practice_recommendation" {
			return "该任务在计划中被标记为高优先级，且直接对应当前高频问题，建议优先完成这一轮专项补练。"
		}
		if source == "weekly_focus" {
			return "该任务在计划中被标记为高优先级，适合作为本周重点补强的起点。"
		}
		if source == "weak_topic" {
			return "该任务在计划中被标记为高优先级，建议优先完成，先集中处理当前最明显的弱项。"
		}
		return "该任务在计划中被标记为高优先级，建议优先完成，先推进当前阶段最关键的学习动作。"
	case "medium":
		if source == "practice_recommendation" {
			return "该任务被安排为中优先级，适合在主线事项后立刻补上，避免高频问题继续反复出现。"
		}
		if source == "weekly_focus" {
			return "该任务被安排为中优先级，建议在本周主线推进中持续跟进这个重点补强方向。"
		}
		if source == "goal" {
			return "该任务被安排为中优先级，用于持续推进当前学习目标，同时不给主线节奏造成中断。"
		}
		return "该任务被安排为中优先级，适合在完成最高优先级事项后紧接着推进。"
	case "low":
		return "该任务被安排为低优先级，更适合作为当前阶段的补充巩固或后续延伸。"
	default:
		if source == "practice_recommendation" {
			return "该任务对应当前高频问题，建议尽量不要长期后移，避免同类错误继续积累。"
		}
		if source == "weekly_focus" {
			return "该任务围绕当前阶段最值得补强的重点主题安排，建议按本周节奏持续推进。"
		}
		if source == "goal" {
			return "该任务用于承接当前学习目标，建议在本阶段按计划持续推进。"
		}
		if source == "weak_topic" {
			return "该任务用于继续补齐当前弱项，建议尽量不要长期后移。"
		}
		return "该任务用于补齐计划中的基础环节，适合作为后续稳步推进的一部分。"
	}
}

// buildLatestPlanFocusSignals 拉取当前行业下的学习档案和近期面试报告，生成计划使用的训练重点信号。
func (s *planService) buildLatestPlanFocusSignals(
	ctx context.Context,
	userID uint,
	industryID uint,
	industryCode string,
) ([]trainingFocusSignal, error) {
	entries := make([]model.LearningArchiveEntry, 0)
	if s.learningArchiveRepo != nil {
		items, err := s.learningArchiveRepo.ListRecentByUser(ctx, userID, growthWeeklyFocusArchiveLimit, nil)
		if err != nil {
			return nil, err
		}
		entries = filterPlanFocusEntriesByIndustry(items, industryCode)
	}

	interviews := make([]model.MockInterview, 0)
	if s.interviewRepo != nil {
		items, _, err := s.interviewRepo.ListByUser(ctx, userID, 1, growthRecentInterviewLimit)
		if err != nil {
			return nil, err
		}
		interviews = filterPlanFocusInterviewsByIndustry(items, industryID)
	}

	return buildTrainingFocusSignals(entries, interviews, defaultTrainingFocusSignalLimit), nil
}

// filterPlanFocusEntriesByIndustry 过滤学习档案，只保留当前计划所属行业的条目，避免多赛道信号互相污染。
func filterPlanFocusEntriesByIndustry(entries []model.LearningArchiveEntry, industryCode string) []model.LearningArchiveEntry {
	trimmedIndustryCode := strings.TrimSpace(industryCode)
	if trimmedIndustryCode == "" {
		return append([]model.LearningArchiveEntry(nil), entries...)
	}

	result := make([]model.LearningArchiveEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.IndustryCode) != trimmedIndustryCode {
			continue
		}
		result = append(result, entry)
	}
	return result
}

// filterPlanFocusInterviewsByIndustry 过滤面试记录，只保留当前计划所属行业的最近面试数据。
func filterPlanFocusInterviewsByIndustry(interviews []model.MockInterview, industryID uint) []model.MockInterview {
	if industryID == 0 {
		return append([]model.MockInterview(nil), interviews...)
	}

	result := make([]model.MockInterview, 0, len(interviews))
	for _, interview := range interviews {
		if interview.IndustryID != industryID {
			continue
		}
		result = append(result, interview)
	}
	return result
}

// mergePlanProfileWeakTopics 将用户显式弱项与系统识别出的训练重点合并，供生成计划时补充画像。
func mergePlanProfileWeakTopics(weakTopics []string, focusSignals []trainingFocusSignal) []string {
	result := sanitizePlanContextTopics(weakTopics)
	for _, signal := range focusSignals {
		result = sanitizePlanContextTopics(append(result, signal.Tag))
	}
	return result
}

// boolToInt 布尔转整数
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
