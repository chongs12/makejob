package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	growthv1 "makejob/api/makejob/growth/v1"
	"makejob/app/companion/internal/biz"
)

// growthClient 实现 biz.GrowthClient 接口，通过 gRPC 调用 Growth 服务
type growthClient struct {
	client growthv1.GrowthServiceClient
	conn   *grpc.ClientConn
}

// NewGrowthClient 创建 Growth 服务客户端
func NewGrowthClient(serviceAddr string) (biz.GrowthClient, error) {
	conn, err := grpc.Dial(serviceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial growth service at %s: %w", serviceAddr, err)
	}
	return &growthClient{
		client: growthv1.NewGrowthServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *growthClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetFocusSignals 获取用户高频薄弱点
func (c *growthClient) GetFocusSignals(ctx context.Context, userID uint64) ([]biz.FocusSignal, error) {
	resp, err := c.client.GetGrowthSummary(ctx, &growthv1.UserIDRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetGrowthSummary gRPC call failed: %w", err)
	}

	signals := make([]biz.FocusSignal, 0, len(resp.GetFocusSignals()))
	for _, s := range resp.GetFocusSignals() {
		signals = append(signals, biz.FocusSignal{
			Tag:             s.GetFocusTag(),
			TopicTitle:      s.GetTopicTitle(),
			OccurrenceCount: s.GetOccurrenceCount(),
		})
	}
	return signals, nil
}
