package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	planv1 "makejob/api/makejob/plan/v1"
	"makejob/app/companion/internal/biz"
	"makejob/pkg/auth"
	"makejob/pkg/middleware"
)

// planClient 实现 biz.PlanClient 接口，通过 gRPC 调用 Plan 服务
type planClient struct {
	client planv1.PlanServiceClient
	conn   *grpc.ClientConn
}

// NewPlanClient 创建 Plan 服务客户端
func NewPlanClient(serviceAddr string) (biz.PlanClient, error) {
	opts := append(middleware.CommonDialOptions(),
		grpc.WithUnaryInterceptor(auth.ForwardTokenClientInterceptor()),
	)
	conn, err := grpc.Dial(serviceAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial plan service at %s: %w", serviceAddr, err)
	}
	return &planClient{
		client: planv1.NewPlanServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *planClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetCurrentPlan 获取用户当前活跃计划
func (c *planClient) GetCurrentPlan(ctx context.Context, userID uint64) (*biz.PlanBrief, error) {
	resp, err := c.client.GetCurrentPlan(ctx, &planv1.GetCurrentPlanRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetCurrentPlan gRPC call failed: %w", err)
	}

	totalTasks := resp.GetTotalTasks()
	completedTasks := resp.GetCompletedTasks()
	progress := float64(0)
	if totalTasks > 0 {
		progress = float64(completedTasks) / float64(totalTasks)
	}

	return &biz.PlanBrief{
		ID:             resp.GetId(),
		Title:          resp.GetTitle(),
		Status:         resp.GetStatus(),
		TotalTasks:     totalTasks,
		CompletedTasks: completedTasks,
		Progress:       progress,
	}, nil
}
