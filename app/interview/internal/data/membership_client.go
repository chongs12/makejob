package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	membershipv1 "makejob/api/makejob/membership/v1"
	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/conf"
	"makejob/pkg/auth"
)

// membershipClient 实现 biz.MembershipClient 接口
// 通过 gRPC 调用 Membership 服务，注入内部服务 Token 绕过用户鉴权。
type membershipClient struct {
	client membershipv1.MembershipServiceClient
	conn   *grpc.ClientConn
}

// NewMembershipClient 创建会员客户端，注入内部服务 Token。
func NewMembershipClient(cfg *conf.Membership, serviceToken string) (biz.MembershipClient, error) {
	conn, err := grpc.Dial(
		cfg.ServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(auth.ServiceAuthInterceptor(serviceToken)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial Membership service at %s: %w", cfg.ServiceAddr, err)
	}
	return &membershipClient{
		client: membershipv1.NewMembershipServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接。
func (c *membershipClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CheckFeatureAccess 校验用户是否具备指定功能权限。
func (c *membershipClient) CheckFeatureAccess(ctx context.Context, userID uint64, feature string) (bool, string, error) {
	resp, err := c.client.CheckFeatureAccess(ctx, &membershipv1.CheckFeatureRequest{
		UserId:  userID,
		Feature: feature,
	})
	if err != nil {
		return false, "", fmt.Errorf("CheckFeatureAccess gRPC call failed: %w", err)
	}
	return resp.Allowed, resp.Reason, nil
}
