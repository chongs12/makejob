package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	aiv1 "makejob/api/makejob/ai/v1"
	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/conf"
	"makejob/pkg/middleware"
)

// quizAnalyzerClient 实现 biz.QuizAnalyzerClient 接口
// 通过 gRPC 调用 AI 服务
type quizAnalyzerClient struct {
	client aiv1.AIServiceClient
	conn   *grpc.ClientConn
}

// NewQuizAnalyzerClient 创建 QuizAnalyzer 客户端
func NewQuizAnalyzerClient(cfg *conf.AI) (biz.QuizAnalyzerClient, error) {
	conn, err := grpc.Dial(cfg.ServiceAddr, middleware.CommonDialOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial AI service at %s: %w", cfg.ServiceAddr, err)
	}
	return &quizAnalyzerClient{
		client: aiv1.NewAIServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *quizAnalyzerClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *quizAnalyzerClient) Analyze(ctx context.Context, req *biz.QuizAnalyzerRequest) (*biz.QuizAnalyzerResponse, error) {
	resp, err := c.client.QuizAnalyzer(ctx, &aiv1.QuizAnalyzerRequest{
		Question:   req.Question,
		Answer:     req.Answer,
		Topic:      req.Topic,
		Difficulty: req.Difficulty,
	})
	if err != nil {
		return nil, fmt.Errorf("QuizAnalyzer gRPC call failed: %w", err)
	}

	return &biz.QuizAnalyzerResponse{
		Score:         resp.Score,
		IsCorrect:     resp.IsCorrect,
		Feedback:      resp.Feedback,
		KeyPoints:     resp.KeyPoints,
		Suggestions:   resp.Suggestions,
		CorrectAnswer: resp.CorrectAnswer,
	}, nil
}
