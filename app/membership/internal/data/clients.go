package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	interviewv1 "makejob/api/makejob/interview/v1"
	questionv1 "makejob/api/makejob/question/v1"
	"makejob/app/membership/internal/biz"
	"makejob/app/membership/internal/conf"
	"makejob/pkg/middleware"
)

// --- QuestionClient 实现 ---

type questionClient struct {
	client questionv1.QuestionServiceClient
	conn   *grpc.ClientConn
}

func NewQuestionClient(cfg *conf.DependentServices) (biz.QuestionClient, error) {
	if cfg == nil || cfg.QuestionAddr == "" {
		return nil, nil
	}
	conn, err := grpc.Dial(cfg.QuestionAddr, middleware.CommonDialOptions()...)
	if err != nil {
		return nil, fmt.Errorf("连接题目服务失败(%s): %w", cfg.QuestionAddr, err)
	}
	return &questionClient{
		client: questionv1.NewQuestionServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *questionClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *questionClient) GetUserPracticeStats(ctx context.Context, userID uint64) (*biz.PracticeStats, error) {
	resp, err := c.client.GetUserPracticeStats(ctx, &questionv1.UserIDRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetUserPracticeStats 调用失败: %w", err)
	}
	return &biz.PracticeStats{
		TodayCount: resp.GetTodayCount(),
	}, nil
}

// --- InterviewClient 实现 ---

type interviewClient struct {
	client interviewv1.InterviewServiceClient
	conn   *grpc.ClientConn
}

func NewInterviewClient(cfg *conf.DependentServices) (biz.InterviewClient, error) {
	if cfg == nil || cfg.InterviewAddr == "" {
		return nil, nil
	}
	conn, err := grpc.Dial(cfg.InterviewAddr, middleware.CommonDialOptions()...)
	if err != nil {
		return nil, fmt.Errorf("连接面试服务失败(%s): %w", cfg.InterviewAddr, err)
	}
	return &interviewClient{
		client: interviewv1.NewInterviewServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *interviewClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *interviewClient) GetUserInterviewStats(ctx context.Context, userID uint64) (*biz.InterviewUsage, error) {
	resp, err := c.client.GetInterviewStats(ctx, &interviewv1.UserIDRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetInterviewStats 调用失败: %w", err)
	}
	return &biz.InterviewUsage{
		TodayCount: resp.GetTodayCount(),
	}, nil
}
