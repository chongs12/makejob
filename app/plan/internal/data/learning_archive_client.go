package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	archivev1 "makejob/api/makejob/learning_archive/v1"
	"makejob/app/plan/internal/biz"
	"makejob/app/plan/internal/conf"
	"makejob/pkg/auth"
)

type learningArchiveClient struct {
	client archivev1.LearningArchiveServiceClient
	conn   *grpc.ClientConn
}

// NewLearningArchiveClient 创建 learning_archive gRPC 客户端，注入内部服务 Token。
func NewLearningArchiveClient(cfg *conf.DependentServices, serviceToken string) (biz.LearningArchiveClient, error) {
	conn, err := grpc.Dial(
		cfg.LearningArchiveAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(auth.ServiceAuthInterceptor(serviceToken)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial learning_archive service at %s: %w", cfg.LearningArchiveAddr, err)
	}
	return &learningArchiveClient{
		client: archivev1.NewLearningArchiveServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *learningArchiveClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// WritePlanFeedback 将计划反馈诊断结果写入学习档案。
func (c *learningArchiveClient) WritePlanFeedback(ctx context.Context, entry *biz.PlanFeedbackArchiveEntry) error {
	sourceRef := fmt.Sprintf("plan-feedback:%d", entry.FeedbackID)
	req := &archivev1.WriteArchiveEntryRequest{
		UserId:          entry.UserID,
		SourceType:      "plan_task_feedback",
		SourceRef:       sourceRef,
		IndustryCode:    entry.IndustryCode,
		PlanPhase:       entry.PlanPhase,
		PlanPhaseGoal:   entry.PlanPhaseGoal,
		EntryPhase:      entry.EntryPhase,
		TaskPhase:       entry.TaskPhase,
		TaskPhaseGoal:   entry.TaskPhaseGoal,
		Language:        entry.Language,
		MistakeTags:     entry.MistakeTags,
		Suggestions:     entry.Suggestions,
		EvidenceSummary: entry.EvidenceSummary,
	}
	if !entry.OccurredAt.IsZero() {
		req.OccurredAt = timestamppb.New(entry.OccurredAt)
	}
	_, err := c.client.WriteEntry(ctx, req)
	if err != nil {
		return fmt.Errorf("WriteEntry gRPC call failed: %w", err)
	}
	return nil
}

// GetWeakTopics 读取用户高频薄弱主题列表，供计划生成/调整消费画像。
func (c *learningArchiveClient) GetWeakTopics(ctx context.Context, userID uint64) ([]string, error) {
	resp, err := c.client.GetWeakTopics(ctx, &archivev1.GetWeakTopicsRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetWeakTopics gRPC call failed: %w", err)
	}
	return resp.GetTopics(), nil
}
