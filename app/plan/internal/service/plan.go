package service

import (
	"context"
	"math"

	"google.golang.org/protobuf/types/known/timestamppb"

	planv1 "makejob/api/makejob/plan/v1"
	"makejob/app/plan/internal/biz"
	"makejob/pkg/auth"
)

// PlanService 学习计划 gRPC 服务实现
type PlanService struct {
	planv1.UnimplementedPlanServiceServer
	uc *biz.PlanUseCase
}

// NewPlanService 创建学习计划 gRPC 服务
func NewPlanService(uc *biz.PlanUseCase) *PlanService {
	return &PlanService{uc: uc}
}

// CreatePlan 创建学习计划
func (s *PlanService) CreatePlan(ctx context.Context, req *planv1.CreatePlanRequest) (*planv1.PlanResponse, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}

	plan, err := s.uc.CreatePlan(ctx, &biz.CreatePlanRequest{
		UserID:            userID,
		IndustryCode:      req.GetIndustry(),
		Goal:              req.GetGoalDescription(),
		DailyHours:        int32(math.Ceil(float64(req.GetDailyStudyMinutes()) / 60)),
		Level:             req.GetLevel(),
		DurationDays:      req.GetDurationDays(),
		DailyStudyMinutes: req.GetDailyStudyMinutes(),
		WeakTopics:        req.GetWeakTopics(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &planv1.PlanResponse{
		PlanId:    plan.ID,
		Status:    plan.Status,
		CreatedAt: timestamppb.New(plan.CreatedAt),
	}, nil
}

// GetPlan 查询计划详情（含任务列表）
func (s *PlanService) GetPlan(ctx context.Context, req *planv1.GetPlanRequest) (*planv1.PlanDetail, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}

	plan, tasks, err := s.uc.GetPlanWithTasks(ctx, userID, req.GetPlanId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoPlanDetailWithTasks(plan, tasks), nil
}

// GetCurrentPlan 查询用户当前活跃计划（含任务列表）
func (s *PlanService) GetCurrentPlan(ctx context.Context, req *planv1.GetCurrentPlanRequest) (*planv1.PlanDetail, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}

	plan, tasks, err := s.uc.GetCurrentPlanWithTasks(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoPlanDetailWithTasks(plan, tasks), nil
}

// ListPlans 查询计划列表
func (s *PlanService) ListPlans(ctx context.Context, req *planv1.ListPlansRequest) (*planv1.ListPlansResponse, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}

	plans, total, err := s.uc.ListPlansWithProgress(ctx, userID, req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, toGRPCError(err)
	}

	items := make([]*planv1.PlanSummary, 0, len(plans))
	for _, plan := range plans {
		items = append(items, toProtoPlanSummary(plan))
	}
	return &planv1.ListPlansResponse{
		Items: items,
		Total: total,
	}, nil
}

// UpdateTaskStatus 更新任务状态
func (s *PlanService) UpdateTaskStatus(ctx context.Context, req *planv1.UpdateTaskStatusRequest) (*planv1.UpdateTaskStatusResponse, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}

	task, plan, err := s.uc.UpdateTaskStatus(ctx, userID, req.GetPlanId(), req.GetTaskId(), req.GetStatus())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &planv1.UpdateTaskStatusResponse{
		TaskStatus:     task.Status,
		PlanStatus:     plan.Status,
		CompletedTasks: plan.CompletedTasks,
		TotalTasks:     plan.TotalTasks,
		Progress:       biz.CalculatePlanProgress(plan.CompletedTasks, plan.TotalTasks),
	}, nil
}

// SubmitTaskFeedback 提交任务反馈
func (s *PlanService) SubmitTaskFeedback(ctx context.Context, req *planv1.SubmitFeedbackRequest) (*planv1.FeedbackResponse, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}

	feedback, err := s.uc.SubmitTaskFeedback(ctx, userID, req.GetPlanId(), req.GetTaskId(), &biz.SubmitFeedbackBizRequest{
		DifficultyFeeling:     req.GetDifficultyFeeling(),
		FeedbackText:          req.GetFeedbackText(),
		ActualDurationMinutes: req.GetActualDurationMinutes(),
		ProblemAreas:          req.GetProblemAreas(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &planv1.FeedbackResponse{
		FeedbackId: feedback.ID,
		Status:     "diagnosis_pending",
		Diagnosis:  "",
	}, nil
}

// AdjustPlan 调整学习计划
func (s *PlanService) AdjustPlan(ctx context.Context, req *planv1.AdjustPlanRequest) (*planv1.AdjustPlanResponse, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}

	adjustment, err := s.uc.AdjustPlan(ctx, userID, req.GetPlanId(), req.GetReason())
	if err != nil {
		return nil, toGRPCError(err)
	}

	_, tasks, err := s.uc.GetPlanWithTasks(ctx, userID, req.GetPlanId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	updatedTasks := make([]*planv1.TaskDetail, 0, len(tasks))
	for _, task := range tasks {
		updatedTasks = append(updatedTasks, toProtoTaskDetail(task))
	}
	return &planv1.AdjustPlanResponse{
		TasksAdded:        adjustment.AddedCount,
		TasksRemoved:      adjustment.RemovedCount,
		TasksReordered:    adjustment.ReorderedCount,
		AdjustmentSummary: adjustment.Summary,
		UpdatedTasks:      updatedTasks,
	}, nil
}

// toProtoPlanDetailWithTasks 将 LearningPlan 和任务列表转换为 PlanDetail。
func toProtoPlanDetailWithTasks(plan *biz.LearningPlan, tasks []*biz.LearningTask) *planv1.PlanDetail {
	taskDetails := make([]*planv1.TaskDetail, 0, len(tasks))
	for _, task := range tasks {
		taskDetails = append(taskDetails, toProtoTaskDetail(task))
	}

	detail := &planv1.PlanDetail{
		Id:                plan.ID,
		Title:             plan.Title,
		Description:       plan.Description,
		Status:            plan.Status,
		DurationDays:      plan.DurationDays,
		CompletedTasks:    plan.CompletedTasks,
		TotalTasks:        plan.TotalTasks,
		Progress:          biz.CalculatePlanProgress(plan.CompletedTasks, plan.TotalTasks),
		Tasks:             taskDetails,
		CreatedAt:         timestamppb.New(plan.CreatedAt),
		Industry:          plan.Industry,
		IndustryCode:      plan.Industry,
		Level:             plan.Level,
		DailyStudyMinutes: plan.DailyStudyMinutes,
		GoalDescription:   plan.Description,
		TaskStatus:        plan.Status,
		StartDate:         plan.CreatedAt.Format("2006-01-02"),
		Phase:             plan.Phase,
		PhaseGoal:         plan.PhaseGoal,
	}
	if plan.DurationDays > 0 {
		detail.EndDate = plan.CreatedAt.AddDate(0, 0, int(plan.DurationDays)).Format("2006-01-02")
	}
	return detail
}

// toProtoPlanSummary 将 LearningPlan 转换为 PlanSummary。
func toProtoPlanSummary(plan *biz.LearningPlan) *planv1.PlanSummary {
	return &planv1.PlanSummary{
		Id:           plan.ID,
		Title:        plan.Title,
		Status:       plan.Status,
		Progress:     biz.CalculatePlanProgress(plan.CompletedTasks, plan.TotalTasks),
		CreatedAt:    timestamppb.New(plan.CreatedAt),
		Industry:     plan.Industry,
		DurationDays: plan.DurationDays,
	}
}

// toProtoTaskDetail 将 LearningTask 转换为 TaskDetail。
func toProtoTaskDetail(task *biz.LearningTask) *planv1.TaskDetail {
	detail := &planv1.TaskDetail{
		Id:              task.ID,
		PlanId:          task.PlanID,
		Title:           task.Title,
		Description:     task.Description,
		TaskType:        task.TaskType,
		Phase:           task.Phase,
		DayNumber:       task.DayNumber,
		DurationMinutes: task.DurationMinutes,
		Priority:        task.Priority,
		Status:          task.Status,
		OrderIndex:      task.SortOrder,
		SortOrder:       task.SortOrder,
		PhaseGoal:       biz.PhaseGoalMap[task.Phase],
		Source:          task.Source,
		SourceLabel:     task.SourceLabel,
		Reason:          task.Reason,
	}
	if task.CompletedAt != nil {
		detail.CompletedAt = timestamppb.New(*task.CompletedAt)
	}
	return detail
}

// GetProgress 获取学习计划进度统计
func (s *PlanService) GetProgress(ctx context.Context, req *planv1.GetProgressRequest) (*planv1.PlanProgressResponse, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}

	progress, err := s.uc.GetProgress(ctx, userID, req.GetPlanId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	dailyProgress := make([]*planv1.DailyProgress, 0, len(progress.DailyProgress))
	for _, dp := range progress.DailyProgress {
		dailyProgress = append(dailyProgress, &planv1.DailyProgress{
			DayNumber: int32(dp.DayNumber),
			Completed: int32(dp.Completed),
			Total:     int32(dp.Total),
		})
	}

	taskTypeStats := make([]*planv1.TaskTypeStat, 0, len(progress.TaskTypeStats))
	for _, ts := range progress.TaskTypeStats {
		taskTypeStats = append(taskTypeStats, &planv1.TaskTypeStat{
			TaskType:  ts.TaskType,
			Completed: int32(ts.Completed),
			Total:     int32(ts.Total),
		})
	}

	return &planv1.PlanProgressResponse{
		PlanId:          progress.PlanID,
		TotalTasks:      int32(progress.TotalTasks),
		CompletedTasks:  int32(progress.CompletedTasks),
		SkippedTasks:    int32(progress.SkippedTasks),
		InProgressTasks: int32(progress.InProgressTasks),
		PendingTasks:    int32(progress.PendingTasks),
		Progress:        float32(progress.Progress),
		DailyProgress:   dailyProgress,
		TaskTypeStats:   taskTypeStats,
	}, nil
}

// toGRPCError 将业务错误转换为 gRPC 错误
func toGRPCError(err error) error {
	if err == nil {
		return nil
	}
	return err
}
