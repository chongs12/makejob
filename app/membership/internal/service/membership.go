package service

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

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
	m, err := s.uc.GetUserMembership(ctx, req.UserId)
	if err != nil {
		return &membershipv1.MembershipStatus{Level: "free", IsActive: false}, nil
	}
	isActive := m.Level != "free"
	practiceLimit, interviewLimit := biz.GetDailyLimits(m.Level)
	practiceToday, interviewToday := s.uc.GetUsage(ctx, req.UserId)
	return &membershipv1.MembershipStatus{
		Level:               m.Level,
		ExpireAt:            toProtoTimestamp(m.ExpiresAt),
		IsActive:            isActive,
		DailyPracticeLimit:  practiceLimit,
		DailyInterviewLimit: interviewLimit,
		PracticeUsedToday:   practiceToday,
		InterviewUsedToday:  interviewToday,
	}, nil
}

// ListPlans 查询可用套餐列表
func (s *MembershipService) ListPlans(ctx context.Context, req *membershipv1.ListPlansRequest) (*membershipv1.ListPlansResponse, error) {
	plans := s.uc.ListPlans()
	protoPlans := make([]*membershipv1.MembershipPlan, 0, len(plans))
	for _, p := range plans {
		protoPlans = append(protoPlans, &membershipv1.MembershipPlan{
			PlanType:     p.PlanType,
			Name:         p.Name,
			Price:        p.Price,
			DurationDays: p.DurationDays,
			Features:     p.Features,
		})
	}
	return &membershipv1.ListPlansResponse{Plans: protoPlans}, nil
}

// CreateOrder 创建订单
func (s *MembershipService) CreateOrder(ctx context.Context, req *membershipv1.CreateOrderRequest) (*membershipv1.OrderResponse, error) {
	order, err := s.uc.CreateOrder(ctx, req.UserId, req.PlanType)
	if err != nil {
		return nil, err
	}
	return toProtoOrder(order), nil
}

// GetOrder 查询订单详情
func (s *MembershipService) GetOrder(ctx context.Context, req *membershipv1.GetOrderRequest) (*membershipv1.OrderResponse, error) {
	order, err := s.uc.GetOrder(ctx, req.UserId, req.OrderId)
	if err != nil {
		return nil, err
	}
	return toProtoOrder(order), nil
}

// ListOrders 查询订单列表
func (s *MembershipService) ListOrders(ctx context.Context, req *membershipv1.ListOrdersRequest) (*membershipv1.ListOrdersResponse, error) {
	page := int(req.Page)
	pageSize := int(req.PageSize)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	orders, total, err := s.uc.ListOrders(ctx, req.UserId, page, pageSize)
	if err != nil {
		return nil, err
	}
	protoOrders := make([]*membershipv1.OrderResponse, 0, len(orders))
	for _, o := range orders {
		protoOrders = append(protoOrders, toProtoOrder(o))
	}
	return &membershipv1.ListOrdersResponse{Orders: protoOrders, Total: total}, nil
}

// HandlePaymentCallback 处理支付回调
func (s *MembershipService) HandlePaymentCallback(ctx context.Context, req *membershipv1.PaymentCallbackRequest) (*membershipv1.OrderResponse, error) {
	order, err := s.uc.HandlePaymentCallback(ctx, req.OrderNo, req.TransactionId)
	if err != nil {
		return nil, err
	}
	return toProtoOrder(order), nil
}

// CheckFeatureAccess 检查功能权限
func (s *MembershipService) CheckFeatureAccess(ctx context.Context, req *membershipv1.CheckFeatureRequest) (*membershipv1.CheckFeatureResponse, error) {
	allowed, reason, err := s.uc.CheckFeatureAccess(ctx, req.UserId, req.Feature)
	if err != nil {
		return nil, err
	}
	return &membershipv1.CheckFeatureResponse{Allowed: allowed, Reason: reason}, nil
}

// UpgradeMembership 升级会员
func (s *MembershipService) UpgradeMembership(ctx context.Context, req *membershipv1.UpgradeRequest) (*membershipv1.MembershipStatus, error) {
	m, err := s.uc.UpgradeMembership(ctx, req.UserId, req.Level, req.DurationDays)
	if err != nil {
		return nil, err
	}
	isActive := m.Level != "free"
	return &membershipv1.MembershipStatus{
		Level:    m.Level,
		ExpireAt: toProtoTimestamp(m.ExpiresAt),
		IsActive: isActive,
	}, nil
}

// --- Proto 转换辅助函数 ---

// toProtoTimestamp 将 time.Time 转为 protobuf Timestamp
func toProtoTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// toProtoOrder 将订单实体转换为 proto OrderResponse
func toProtoOrder(o *biz.MembershipOrder) *membershipv1.OrderResponse {
	resp := &membershipv1.OrderResponse{
		Id:        uint64(o.ID),
		OrderNo:   o.OrderNo,
		PlanType:  o.PlanID,
		Amount:    o.Amount,
		Status:    o.Status,
		CreatedAt: toProtoTimestamp(o.CreatedAt),
		ExpiresAt: toProtoTimestamp(o.ExpiresAt),
	}
	if o.PaidAt != nil {
		resp.PaidAt = toProtoTimestamp(*o.PaidAt)
	}
	return resp
}
