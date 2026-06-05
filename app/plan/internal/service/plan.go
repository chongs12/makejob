package service

import (
	"context"

	planv1 "makejob/api/makejob/plan/v1"
	"makejob/app/plan/internal/biz"
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
	return &planv1.PlanResponse{}, nil
}

// GetPlan 查询计划详情
func (s *PlanService) GetPlan(ctx context.Context, req *planv1.GetPlanRequest) (*planv1.PlanDetail, error) {
	return &planv1.PlanDetail{}, nil
}

// GetCurrentPlan 查询用户当前计划
func (s *PlanService) GetCurrentPlan(ctx context.Context, req *planv1.GetCurrentPlanRequest) (*planv1.PlanDetail, error) {
	return &planv1.PlanDetail{}, nil
}

// ListPlans 查询计划列表
func (s *PlanService) ListPlans(ctx context.Context, req *planv1.ListPlansRequest) (*planv1.ListPlansResponse, error) {
	return &planv1.ListPlansResponse{}, nil
}

// UpdateTaskStatus 更新任务状态
func (s *PlanService) UpdateTaskStatus(ctx context.Context, req *planv1.UpdateTaskStatusRequest) (*planv1.TaskDetail, error) {
	return &planv1.TaskDetail{}, nil
}

// SubmitTaskFeedback 提交任务反馈
func (s *PlanService) SubmitTaskFeedback(ctx context.Context, req *planv1.SubmitFeedbackRequest) (*planv1.FeedbackResponse, error) {
	return &planv1.FeedbackResponse{}, nil
}

// AdjustPlan 调整学习计划
func (s *PlanService) AdjustPlan(ctx context.Context, req *planv1.AdjustPlanRequest) (*planv1.AdjustPlanResponse, error) {
	return &planv1.AdjustPlanResponse{}, nil
}
