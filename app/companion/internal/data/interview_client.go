package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	interviewv1 "makejob/api/makejob/interview/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	"makejob/app/companion/internal/biz"
	"makejob/pkg/auth"
	"makejob/pkg/middleware"
)

// interviewClient 实现 biz.InterviewClient 接口，通过 gRPC 调用 Interview 服务
type interviewClient struct {
	client interviewv1.InterviewServiceClient
	conn   *grpc.ClientConn
}

// NewInterviewClient 创建 Interview 服务客户端
func NewInterviewClient(serviceAddr string) (biz.InterviewClient, error) {
	opts := append(middleware.CommonDialOptions(),
		grpc.WithUnaryInterceptor(auth.ForwardTokenClientInterceptor()),
	)
	conn, err := grpc.Dial(serviceAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial interview service at %s: %w", serviceAddr, err)
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

// ListRecent 获取用户最近面试摘要
func (c *interviewClient) ListRecent(ctx context.Context, userID uint64, limit int32) ([]biz.InterviewBrief, error) {
	resp, err := c.client.ListInterviews(ctx, &interviewv1.ListInterviewsRequest{
		UserId: userID,
		Page: &sharedv1.PageParam{
			Page:     1,
			PageSize: limit,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ListInterviews gRPC call failed: %w", err)
	}

	interviews := make([]biz.InterviewBrief, 0, len(resp.GetInterviews()))
	for _, i := range resp.GetInterviews() {
		interviews = append(interviews, biz.InterviewBrief{
			ID:     i.GetInterviewId(),
			Status: i.GetStatus(),
			Score:  i.GetScore(),
		})
	}
	return interviews, nil
}
