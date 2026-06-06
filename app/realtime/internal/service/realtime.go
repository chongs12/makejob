package service

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	realtimev1 "makejob/api/makejob/realtime/v1"
	"makejob/app/realtime/internal/biz"
	"makejob/pkg/auth"
)

// RealtimeService 实时会话 gRPC 服务实现
type RealtimeService struct {
	realtimev1.UnimplementedRealtimeServiceServer
	uc       *biz.RealtimeUseCase
	wsScheme string
	wsHost   string
}

// NewRealtimeService 创建实时会话 gRPC 服务
func NewRealtimeService(uc *biz.RealtimeUseCase, wsScheme string, wsHost string) *RealtimeService {
	return &RealtimeService{
		uc:       uc,
		wsScheme: wsScheme,
		wsHost:   wsHost,
	}
}

// InitSession 初始化实时会话，创建会话记录并返回 WebSocket 连接 URL
func (s *RealtimeService) InitSession(ctx context.Context, req *realtimev1.InitSessionRequest) (*realtimev1.SessionResponse, error) {
	userID := req.UserId
	if authenticatedUserID := auth.GetUserIDFromContext(ctx); authenticatedUserID != 0 {
		userID = authenticatedUserID
	}
	session, err := s.uc.InitSession(ctx, req.InterviewId, userID)
	if err != nil {
		return nil, err
	}

	// 构造 WebSocket URL
	wsURL := s.buildWSURL(ctx, session.InterviewID, session.ID)

	return &realtimev1.SessionResponse{
		SessionId: session.ID,
		Status:    session.Status,
		WsUrl:     wsURL,
		CreatedAt: timestamppb.New(time.Now()),
	}, nil
}

// GetSessionStatus 查询会话状态
func (s *RealtimeService) GetSessionStatus(ctx context.Context, req *realtimev1.GetSessionStatusRequest) (*realtimev1.SessionResponse, error) {
	session, err := s.uc.GetSession(ctx, req.SessionId)
	if err != nil {
		return nil, err
	}
	return &realtimev1.SessionResponse{
		SessionId: session.ID,
		Status:    session.Status,
	}, nil
}

// InjectRAGContext 注入 RAG 上下文到活跃会话
func (s *RealtimeService) InjectRAGContext(ctx context.Context, req *realtimev1.InjectRAGContextRequest) (*realtimev1.InjectRAGContextResponse, error) {
	if err := s.uc.InjectRAGContext(ctx, req.SessionId, req.Context); err != nil {
		return nil, err
	}
	return &realtimev1.InjectRAGContextResponse{Success: true}, nil
}

// EndSession 结束指定会话
func (s *RealtimeService) EndSession(ctx context.Context, req *realtimev1.EndSessionRequest) (*realtimev1.EndSessionResponse, error) {
	if err := s.uc.EndSession(ctx, req.SessionId); err != nil {
		return nil, err
	}
	return &realtimev1.EndSessionResponse{Success: true}, nil
}

// HealthCheck 健康检查
func (s *RealtimeService) HealthCheck(_ context.Context, _ *realtimev1.HealthCheckRequest) (*realtimev1.HealthCheckResponse, error) {
	return &realtimev1.HealthCheckResponse{Status: "ok"}, nil
}

// buildWSURL 构建 WebSocket 连接地址
func (s *RealtimeService) buildWSURL(ctx context.Context, interviewID uint64, sessionID string) string {
	scheme := s.wsScheme
	if scheme == "" {
		scheme = "ws"
	}
	host := s.wsHost
	if host == "" {
		host = "localhost:8008"
	}
	values := url.Values{}
	if sessionID != "" {
		values.Set("session_id", sessionID)
	}
	token := auth.GetAccessTokenFromContext(ctx)
	if token == "" {
		token = auth.GetAccessTokenFromMetadata(ctx)
	}
	if token != "" {
		values.Set("token", token)
	}
	wsURL := scheme + "://" + host + "/ws/interview/" + fmt.Sprintf("%d", interviewID)
	if encoded := values.Encode(); encoded != "" {
		wsURL += "?" + encoded
	}
	return wsURL
}
