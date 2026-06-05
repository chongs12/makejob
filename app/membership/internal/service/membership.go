package service

import (
	"context"

	membershipv1 "makejob/api/makejob/membership/v1"
	"makejob/app/membership/internal/biz"
)

// MembershipService 会员 gRPC 服务实现
type MembershipService struct {
	membershipv1.UnimplementedMembershipServiceServer
	uc *biz.MembershipUseCase
}

// NewMembershipService 创建会员 gRPC 服务
func NewMembershipService(uc *biz.MembershipUseCase) *MembershipService {
	return &MembershipService{uc: uc}
}

// GetMembershipStatus 查询会员状态
func (s *MembershipService) GetMembershipStatus(ctx context.Context, req *membershipv1.UserIDRequest) (*membershipv1.MembershipStatus, error) {
	m, err := s.uc.GetByUserID(ctx, req.UserId)
	if err != nil {
		return &membershipv1.MembershipStatus{Level: "free", IsActive: false}, nil
	}
	return &membershipv1.MembershipStatus{Level: m.Level, IsActive: m.Level == "pro"}, nil
}

// ListPlans 查询可用套餐列表
func (s *MembershipService) ListPlans(ctx context.Context, req *membershipv1.ListPlansRequest) (*membershipv1.ListPlansResponse, error) {
	return &membershipv1.ListPlansResponse{}, nil
}

// CreateOrder 创建订单
func (s *MembershipService) CreateOrder(ctx context.Context, req *membershipv1.CreateOrderRequest) (*membershipv1.OrderResponse, error) {
	return &membershipv1.OrderResponse{}, nil
}

// GetOrder 查询订单详情
func (s *MembershipService) GetOrder(ctx context.Context, req *membershipv1.GetOrderRequest) (*membershipv1.OrderResponse, error) {
	return &membershipv1.OrderResponse{}, nil
}

// ListOrders 查询订单列表
func (s *MembershipService) ListOrders(ctx context.Context, req *membershipv1.ListOrdersRequest) (*membershipv1.ListOrdersResponse, error) {
	return &membershipv1.ListOrdersResponse{}, nil
}

// HandlePaymentCallback 处理支付回调
func (s *MembershipService) HandlePaymentCallback(ctx context.Context, req *membershipv1.PaymentCallbackRequest) (*membershipv1.OrderResponse, error) {
	return &membershipv1.OrderResponse{}, nil
}

// CheckFeatureAccess 检查功能权限
func (s *MembershipService) CheckFeatureAccess(ctx context.Context, req *membershipv1.CheckFeatureRequest) (*membershipv1.CheckFeatureResponse, error) {
	return &membershipv1.CheckFeatureResponse{Allowed: true}, nil
}

// UpgradeMembership 升级会员
func (s *MembershipService) UpgradeMembership(ctx context.Context, req *membershipv1.UpgradeRequest) (*membershipv1.MembershipStatus, error) {
	return &membershipv1.MembershipStatus{}, nil
}
