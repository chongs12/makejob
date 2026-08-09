package data

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	archivev1 "makejob/api/makejob/learning_archive/v1"
	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/conf"
	"makejob/pkg/auth"
	"makejob/pkg/middleware"
)

// learningArchiveClient 实现 biz.LearningArchiveClient 接口
// 通过 gRPC 调用 learning_archive 服务。
type learningArchiveClient struct {
	client archivev1.LearningArchiveServiceClient
	conn   *grpc.ClientConn
}

// NewLearningArchiveClient 创建 learning_archive gRPC 客户端，注入内部服务 Token 绕过用户鉴权。
func NewLearningArchiveClient(cfg *conf.DependentServices, serviceToken string) (biz.LearningArchiveClient, error) {
	opts := append(middleware.CommonDialOptions(),
		grpc.WithUnaryInterceptor(auth.ServiceAuthInterceptor(serviceToken)),
	)
	conn, err := grpc.Dial(cfg.LearningArchiveAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial learning_archive service at %s: %w", cfg.LearningArchiveAddr, err)
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

// WriteEntry 写入单条学习档案。
func (c *learningArchiveClient) WriteEntry(ctx context.Context, entry *biz.LearningArchiveEntry) error {
	var mistakeTags, strengthTags, suggestions []string
	_ = json.Unmarshal([]byte(entry.MistakeTagsJSON), &mistakeTags)
	_ = json.Unmarshal([]byte(entry.StrengthTagsJSON), &strengthTags)
	_ = json.Unmarshal([]byte(entry.SuggestionsJSON), &suggestions)

	req := &archivev1.WriteArchiveEntryRequest{
		UserId:          entry.UserID,
		SourceType:      entry.SourceType,
		SourceRef:       entry.SourceRef,
		InterviewId:     entry.InterviewID,
		QuestionIndex:   int32(entry.QuestionIndex),
		IndustryCode:    entry.IndustryCode,
		TaskPhase:       entry.TaskPhase,
		TaskPhaseGoal:   entry.TaskPhaseGoal,
		Language:        entry.Language,
		MistakeTags:     mistakeTags,
		StrengthTags:    strengthTags,
		Suggestions:     suggestions,
		EvidenceSummary: entry.EvidenceSummary,
	}
	if entry.OccurredAt != nil {
		req.OccurredAt = timestamppb.New(*entry.OccurredAt)
	}
	_, err := c.client.WriteEntry(ctx, req)
	if err != nil {
		return fmt.Errorf("WriteEntry gRPC call failed: %w", err)
	}
	return nil
}

// GetFocusSignals 获取用户训练重点信号。
func (c *learningArchiveClient) GetFocusSignals(ctx context.Context, userID uint64, limit int32) ([]biz.FocusSignalData, error) {
	resp, err := c.client.GetFocusSignals(ctx, &archivev1.GetFocusSignalsRequest{
		UserId: userID,
		Limit:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("GetFocusSignals gRPC call failed: %w", err)
	}

	signals := make([]biz.FocusSignalData, len(resp.Signals))
	for i, s := range resp.Signals {
		signals[i] = biz.FocusSignalData{
			Tag:                       s.Tag,
			TopicCode:                 s.TopicCode,
			TopicTitle:                s.TopicTitle,
			TopicProblemPattern:       s.TopicProblemPattern,
			RelatedQuestionSets:       s.RelatedQuestionSets,
			RecommendedActions:        s.RecommendedActions,
			PrimaryQuestionSet:        s.PrimaryQuestionSet,
			OccurrenceCount:           int(s.OccurrenceCount),
			ArchiveOccurrenceCount:    int(s.ArchiveOccurrenceCount),
			InterviewOccurrenceCount:  int(s.InterviewOccurrenceCount),
			DominantArchivePhase:      s.DominantArchivePhase,
			DominantArchivePhaseLabel: s.DominantArchivePhaseLabel,
			Source:                    s.Source,
			SourceLabel:               s.SourceLabel,
			Reason:                    s.Reason,
		}
	}
	return signals, nil
}

// GetMistakeTopic 按编码查询错因专题详情。
func (c *learningArchiveClient) GetMistakeTopic(ctx context.Context, code string) (*biz.MistakeTopicCard, bool) {
	resp, err := c.client.GetMistakeTopic(ctx, &archivev1.GetMistakeTopicRequest{Code: code})
	if err != nil || resp == nil {
		return nil, false
	}
	return &biz.MistakeTopicCard{
		Code:                resp.Code,
		Tag:                 resp.Tag,
		Title:               resp.Title,
		ProblemPattern:      resp.ProblemPattern,
		RootCauses:          resp.RootCauses,
		SelfCheckList:       resp.SelfCheckList,
		PracticeDirections:  resp.PracticeDirections,
		RecommendedActions:  resp.RecommendedActions,
		RelatedQuestionSets: resp.RelatedQuestionSets,
	}, true
}
