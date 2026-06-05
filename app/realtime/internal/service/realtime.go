package service

import (
	"context"

	realtimev1 "makejob/api/makejob/realtime/v1"
	"makejob/app/realtime/internal/biz"
)

// RealtimeService 实时会话 gRPC 服务实现
type RealtimeService struct {
	realtimev1.UnimplementedRealtimeServiceServer
	uc *biz.RealtimeUseCase
}

// NewRealtimeService 创建实时会话 gRPC 服务
func NewRealtimeService(uc *biz.RealtimeUseCase) *RealtimeService {
	return &RealtimeService{uc: uc}
}

// InitSession 初始化实时会话
func (s *RealtimeService) InitSession(ctx context.Context, req *realtimev1.InitSessionRequest) (*realtimev1.SessionResponse, error) {
	return &realtimev1.SessionResponse{}, nil
}

// GetSessionStatus 查询会话状态
func (s *RealtimeService) GetSessionStatus(ctx context.Context, req *realtimev1.GetSessionStatusRequest) (*realtimev1.SessionResponse, error) {
	session, err := s.uc.GetSession(ctx, req.SessionId)
	if err != nil {
		return &realtimev1.SessionResponse{}, nil
	}
	return &realtimev1.SessionResponse{
		SessionId: session.ID,
		Status:    session.Status,
	}, nil
}

// InjectRAGContext 注入 RAG 上下文
func (s *RealtimeService) InjectRAGContext(ctx context.Context, req *realtimev1.InjectRAGContextRequest) (*realtimev1.InjectRAGContextResponse, error) {
	return &realtimev1.InjectRAGContextResponse{Success: true}, nil
}

// EndSession 结束会话
func (s *RealtimeService) EndSession(ctx context.Context, req *realtimev1.EndSessionRequest) (*realtimev1.EndSessionResponse, error) {
	return &realtimev1.EndSessionResponse{Success: true}, nil
}

// HealthCheck 健康检查
func (s *RealtimeService) HealthCheck(ctx context.Context, req *realtimev1.HealthCheckRequest) (*realtimev1.HealthCheckResponse, error) {
	return &realtimev1.HealthCheckResponse{Status: "ok"}, nil
}
