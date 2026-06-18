package data

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	archivev1 "makejob/api/makejob/learning_archive/v1"
	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/conf"
	"makejob/pkg/auth"
)

// learningArchiveClient 实现 biz.LearningArchiveClient 接口
// 通过 gRPC 调用 LearningArchive 服务。
type learningArchiveClient struct {
	client archivev1.LearningArchiveServiceClient
	conn   *grpc.ClientConn
}

// NewLearningArchiveClient 创建学习档案客户端，注入内部服务 Token 绕过用户鉴权。
func NewLearningArchiveClient(cfg *conf.Archive, serviceToken string) (biz.LearningArchiveClient, error) {
	conn, err := grpc.Dial(
		cfg.ServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(auth.ServiceAuthInterceptor(serviceToken)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial LearningArchive service at %s: %w", cfg.ServiceAddr, err)
	}
	return &learningArchiveClient{
		client: archivev1.NewLearningArchiveServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接。
func (c *learningArchiveClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *learningArchiveClient) WriteEntry(ctx context.Context, entry *biz.ArchiveEntry) error {
	req := &archivev1.WriteArchiveEntryRequest{
		UserId:          entry.UserID,
		SourceType:      entry.SourceType,
		SourceRef:       entry.SourceRef,
		InterviewId:     entry.InterviewID,
		QuestionIndex:   entry.QuestionIndex,
		IndustryCode:    entry.IndustryCode,
		Language:        entry.Language,
		MistakeTags:     entry.MistakeTags,
		StrengthTags:    entry.StrengthTags,
		Suggestions:     entry.Suggestions,
		EvidenceSummary: entry.EvidenceSummary,
	}
	if !entry.OccurredAt.IsZero() {
		req.OccurredAt = timestamppb.New(entry.OccurredAt)
	} else {
		req.OccurredAt = timestamppb.New(time.Now())
	}

	_, err := c.client.WriteEntry(ctx, req)
	if err != nil {
		return fmt.Errorf("WriteEntry gRPC call failed: %w", err)
	}
	return nil
}

func (c *learningArchiveClient) ListByUser(ctx context.Context, userID uint64, limit int32) ([]*biz.ArchiveEntry, error) {
	resp, err := c.client.ListByUser(ctx, &archivev1.ListByUserRequest{
		UserId: userID,
		Limit:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("ListByUser gRPC call failed: %w", err)
	}

	entries := make([]*biz.ArchiveEntry, len(resp.Entries))
	for i, e := range resp.Entries {
		entry := &biz.ArchiveEntry{
			ID:              e.Id,
			UserID:          e.UserId,
			SourceType:      e.SourceType,
			SourceRef:       e.SourceRef,
			InterviewID:     e.InterviewId,
			QuestionIndex:   e.QuestionIndex,
			IndustryCode:    e.IndustryCode,
			Language:        e.Language,
			MistakeTags:     e.MistakeTags,
			StrengthTags:    e.StrengthTags,
			Suggestions:     e.Suggestions,
			EvidenceSummary: e.EvidenceSummary,
		}
		if e.OccurredAt != nil {
			entry.OccurredAt = e.OccurredAt.AsTime()
		}
		if e.CreatedAt != nil {
			entry.CreatedAt = e.CreatedAt.AsTime()
		}
		entries[i] = entry
	}
	return entries, nil
}
