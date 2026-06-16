package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	interviewv1 "makejob/api/makejob/interview/v1"
	"makejob/app/realtime/internal/biz"
	"makejob/app/realtime/internal/conf"
	"makejob/pkg/auth"
)

// interviewClient 通过 gRPC 调用 Interview 服务
type interviewClient struct {
	client interviewv1.InterviewServiceClient
	conn   *grpc.ClientConn
}

// NewInterviewClient 创建 Interview 服务客户端，返回接口实现和可选的关闭函数
func NewInterviewClient(cfg *conf.DependentServices) (biz.InterviewClient, *interviewClient, error) {
	conn, err := grpc.NewClient(cfg.InterviewAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("连接 Interview 服务失败 (%s): %w", cfg.InterviewAddr, err)
	}
	c := &interviewClient{
		client: interviewv1.NewInterviewServiceClient(conn),
		conn:   conn,
	}
	return c, c, nil
}

// Close 关闭 gRPC 连接
func (c *interviewClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsRealtimeInterview 查询面试是否为实时模式
func (c *interviewClient) IsRealtimeInterview(ctx context.Context, interviewID uint64) (bool, error) {
	resp, err := c.client.IsRealtimeInterview(withAuthContext(ctx), &interviewv1.IsRealtimeRequest{
		InterviewId: interviewID,
	})
	if err != nil {
		return false, fmt.Errorf("IsRealtimeInterview RPC 失败: %w", err)
	}
	return resp.IsRealtime, nil
}

// GetRealtimeContext 获取实时面试上下文（对齐单体：解析简历画像、面试模式等完整信息）
func (c *interviewClient) GetRealtimeContext(ctx context.Context, interviewID uint64) (*biz.RealtimeContext, error) {
	resp, err := c.client.GetRealtimeContext(withAuthContext(ctx), &interviewv1.GetRealtimeRequest{
		InterviewId: interviewID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetRealtimeContext RPC 失败: %w", err)
	}

	rtCtx := &biz.RealtimeContext{
		InterviewID:    resp.InterviewId,
		IndustryCode:   resp.IndustryCode,
		Live2DModelKey: resp.Live2DModelKey,
		TotalQuestions: int(resp.TotalQuestions),
		Difficulty:     resp.Difficulty,
		InterviewMode:  resp.InterviewMode,
		Topics:         resp.Topics,
		WeakTopics:     resp.WeakTopics,
		DialogID:       resp.DialogId,
		HasStarted:     resp.HasStarted,
		CurrentTopic:   resp.CurrentTopic,
	}

	// 解析简历画像
	if resp.ResumeProfile != nil {
		rtCtx.ResumeProfile = &biz.ResumeProfile{
			Summary:     resp.ResumeProfile.Summary,
			Skills:      resp.ResumeProfile.Skills,
			Projects:    resp.ResumeProfile.Projects,
			Strengths:   resp.ResumeProfile.Strengths,
			WeakSignals: resp.ResumeProfile.WeakSignals,
		}
	}

	return rtCtx, nil
}

// BindRealtimeDialog 绑定实时对话 ID
func (c *interviewClient) BindRealtimeDialog(ctx context.Context, interviewID uint64, dialogID string) error {
	_, err := c.client.BindRealtimeDialog(withAuthContext(ctx), &interviewv1.BindDialogRequest{
		InterviewId: interviewID,
		DialogId:    dialogID,
	})
	if err != nil {
		return fmt.Errorf("BindRealtimeDialog RPC 失败: %w", err)
	}
	return nil
}

// AppendRealtimeUserAnswer 追加用户回答
func (c *interviewClient) AppendRealtimeUserAnswer(ctx context.Context, interviewID uint64, content string) error {
	_, err := c.client.AppendRealtimeUserAnswer(withAuthContext(ctx), &interviewv1.AppendAnswerRequest{
		InterviewId: interviewID,
		AnswerText:  content,
	})
	if err != nil {
		return fmt.Errorf("AppendRealtimeUserAnswer RPC 失败: %w", err)
	}
	return nil
}

// AppendRealtimeAssistantReply 追加助手回复，返回下一题元数据
func (c *interviewClient) AppendRealtimeAssistantReply(ctx context.Context, interviewID uint64, content string) (*biz.NextQuestionMeta, error) {
	resp, err := c.client.AppendRealtimeAssistantReply(withAuthContext(ctx), &interviewv1.AppendReplyRequest{
		InterviewId: interviewID,
		ReplyText:   content,
	})
	if err != nil {
		return nil, fmt.Errorf("AppendRealtimeAssistantReply RPC 失败: %w", err)
	}
	return &biz.NextQuestionMeta{
		IsLastQuestion: resp.ShouldEnd,
	}, nil
}

// FinishInterview 请求 Interview 服务结束实时面试并触发报告生成。
func (c *interviewClient) FinishInterview(ctx context.Context, interviewID uint64) error {
	_, err := c.client.FinishInterview(withAuthContext(ctx), &interviewv1.FinishInterviewRequest{
		InterviewId: interviewID,
	})
	if err != nil {
		return fmt.Errorf("FinishInterview RPC 失败: %w", err)
	}
	return nil
}

// withAuthContext 将当前请求的访问令牌透传给下游 Interview 服务。
func withAuthContext(ctx context.Context) context.Context {
	token := auth.GetAccessTokenFromContext(ctx)
	if token == "" {
		token = auth.GetAccessTokenFromMetadata(ctx)
	}
	if token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}
