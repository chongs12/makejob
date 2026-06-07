package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	aiv1 "makejob/api/makejob/ai/v1"
	"makejob/app/companion/internal/biz"
	"makejob/app/companion/internal/conf"
)

// companionAIClient 实现 biz.CompanionClient 接口，通过 gRPC 调用 AI Gateway
type companionAIClient struct {
	client aiv1.AIServiceClient
	conn   *grpc.ClientConn
}

// NewCompanionAIClient 创建 AI 服务客户端
func NewCompanionAIClient(cfg *conf.AI) (biz.CompanionClient, error) {
	conn, err := grpc.Dial(cfg.ServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial AI service at %s: %w", cfg.ServiceAddr, err)
	}
	return &companionAIClient{
		client: aiv1.NewAIServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *companionAIClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CompanionAgent 调用 AI Gateway 的 CompanionAgent RPC
func (c *companionAIClient) CompanionAgent(ctx context.Context, req *biz.CompanionAgentRequest) (*biz.CompanionAgentResponse, error) {
	resp, err := c.client.CompanionAgent(ctx, &aiv1.CompanionAgentRequest{
		UserId:       req.UserID,
		Message:      req.Message,
		ContextType:  req.ContextType,
		RecentTopics: req.RecentTopics,
	})
	if err != nil {
		return nil, fmt.Errorf("CompanionAgent gRPC call failed: %w", err)
	}
	return &biz.CompanionAgentResponse{
		Reply:       resp.GetReply(),
		Emotion:     resp.GetEmotion(),
		Suggestions: resp.GetSuggestions(),
	}, nil
}

// Live2DDirector 调用 AI Gateway 的 Live2DDirector RPC
func (c *companionAIClient) Live2DDirector(ctx context.Context, req *biz.Live2DDirectorRequest) (*biz.Live2DDirectiveResponse, error) {
	resp, err := c.client.Live2DDirector(ctx, &aiv1.Live2DDirectiveRequest{
		Context:     req.Context,
		EmotionHint: req.EmotionHint,
		ReplyText:   req.ReplyText,
	})
	if err != nil {
		return nil, fmt.Errorf("Live2DDirector gRPC call failed: %w", err)
	}
	return &biz.Live2DDirectiveResponse{
		Emotion:     resp.GetEmotion(),
		Action:      resp.GetAction(),
		Reply:       resp.GetReply(),
		MotionKey:   resp.GetMotionKey(),
		MotionGroup: resp.GetMotionGroup(),
		DurationMs:  resp.GetDurationMs(),
	}, nil
}
