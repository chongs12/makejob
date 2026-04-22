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
	ID          uint       `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	TaskType    string     `json:"task_type"`
	Status      string     `json:"status"`
	DueDate     *time.Time `json:"due_date"`
	CompletedAt *time.Time `json:"completed_at"`
	DayNumber   int        `json:"day_number"`
	SortOrder   int        `json:"sort_order"`
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
	planRepo     repository.PlanRepository
	taskRepo     repository.PlanTaskRepository
	planAgent    ai.PlanAgent
	industryRepo repository.IndustryRepository
}

// NewPlanService 创建学习计划服务实例
func NewPlanService(
	planRepo repository.PlanRepository,
	taskRepo repository.PlanTaskRepository,
	planAgent ai.PlanAgent,
	industryRepo ...repository.IndustryRepository,
) PlanService {
	s := &planService{
		planRepo:  planRepo,
		taskRepo:  taskRepo,
		planAgent: planAgent,
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
	profile := ai.UserProfile{
		Level:           req.Level,
		WeakTopics:      req.WeakTopics,
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
	planJSON, err := json.Marshal(aiPlan)
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
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	plans, total, err := s.planRepo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	return &common.PageResult{
		List:     plans,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
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
	planJSON, _ := json.Marshal(adjustedPlan)
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

		taskResponses = append(taskResponses, TaskResponse{
			ID:          t.ID,
			Title:       t.Title,
			Description: t.Description,
			TaskType:    t.TaskType,
			Status:      t.Status,
			DueDate:     t.DueDate,
			CompletedAt: t.CompletedAt,
			DayNumber:   dayNumber,
			SortOrder:   t.SortOrder,
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

// boolToInt 布尔转整数
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
