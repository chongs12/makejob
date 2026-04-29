// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

// 会员方案配置
var membershipPlans = []MembershipPlanResponse{
	{
		PlanType:  model.PlanTypeMonthly,
		Name:      "月度会员",
		Price:     29.9,
		OrigPrice: 49.9,
		Duration:  30,
		Features:  []string{"无限刷题", "无限模拟面试", "AI深度解析", "专属学习计划", "优先客服支持"},
		IsPopular: false,
	},
	{
		PlanType:  model.PlanTypeQuarterly,
		Name:      "季度会员",
		Price:     69.9,
		OrigPrice: 149.7,
		Duration:  90,
		Features:  []string{"无限刷题", "无限模拟面试", "AI深度解析", "专属学习计划", "优先客服支持"},
		IsPopular: true,
	},
	{
		PlanType:  model.PlanTypeYearly,
		Name:      "年度会员",
		Price:     199.9,
		OrigPrice: 598.8,
		Duration:  365,
		Features:  []string{"无限刷题", "无限模拟面试", "AI深度解析", "专属学习计划", "优先客服支持"},
		IsPopular: false,
	},
}

// 免费用户默认限制
const (
	DefaultDailyPracticeLimit  = 20
	DefaultDailyInterviewLimit = 2
)

// MembershipService 会员服务接口
type MembershipService interface {
	GetPlans(ctx context.Context) ([]MembershipPlanResponse, error)
	CreateOrder(ctx context.Context, userID uint, req *CreateOrderRequest) (*OrderResponse, error)
	GetOrder(ctx context.Context, userID uint, orderID uint) (*OrderResponse, error)
	ListOrders(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error)
	MockPayCallback(ctx context.Context, orderNo string) (*OrderResponse, error)
	GetMembershipStatus(ctx context.Context, userID uint) (*MembershipStatusResponse, error)
}

// MembershipPlanResponse 会员方案响应DTO
type MembershipPlanResponse struct {
	PlanType  string   `json:"plan_type"` // monthly/quarterly/yearly
	Name      string   `json:"name"`
	Price     float64  `json:"price"`
	OrigPrice float64  `json:"original_price"`
	Duration  int      `json:"duration_days"`
	Features  []string `json:"features"`
	IsPopular bool     `json:"is_popular"`
}

// CreateOrderRequest 创建订单请求DTO
type CreateOrderRequest struct {
	PlanType string `json:"plan_type" binding:"required,oneof=monthly quarterly yearly"`
}

// OrderResponse 订单响应DTO
type OrderResponse struct {
	ID        uint       `json:"id"`
	OrderNo   string     `json:"order_no"`
	PlanType  string     `json:"plan_type"`
	Amount    float64    `json:"amount"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// MembershipStatusResponse 会员状态响应DTO
type MembershipStatusResponse struct {
	Level               string     `json:"level"` // free/pro
	ExpiresAt           *time.Time `json:"expires_at,omitempty"`
	IsActive            bool       `json:"is_active"`
	DailyPracticeLimit  int        `json:"daily_practice_limit"`
	DailyInterviewLimit int        `json:"daily_interview_limit"`
	PracticeUsedToday   int        `json:"practice_used_today"`
	InterviewUsedToday  int        `json:"interview_used_today"`
}

// membershipService 会员服务实现
type membershipService struct {
	membershipRepo repository.MembershipRepository
	userRepo       repository.UserRepository
}

// NewMembershipService 创建会员服务实例
func NewMembershipService(membershipRepo repository.MembershipRepository, userRepo repository.UserRepository) MembershipService {
	return &membershipService{
		membershipRepo: membershipRepo,
		userRepo:       userRepo,
	}
}

// GetPlans 获取会员方案列表
func (s *membershipService) GetPlans(ctx context.Context) ([]MembershipPlanResponse, error) {
	// 返回硬编码的会员方案
	return membershipPlans, nil
}

// CreateOrder 创建订单
func (s *membershipService) CreateOrder(ctx context.Context, userID uint, req *CreateOrderRequest) (*OrderResponse, error) {
	// 查找对应的会员方案
	var plan *MembershipPlanResponse
	for _, p := range membershipPlans {
		if p.PlanType == req.PlanType {
			plan = &p
			break
		}
	}
	if plan == nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "无效的会员方案")
	}

	// 生成订单号
	orderNo := generateOrderNo()

	// 创建订单
	order := &model.MembershipOrder{
		UserID:   userID,
		OrderNo:  orderNo,
		PlanType: req.PlanType,
		Amount:   plan.Price,
		Status:   model.OrderStatusPending,
	}

	if err := s.membershipRepo.CreateOrder(ctx, order); err != nil {
		return nil, err
	}

	return convertToOrderResponse(order), nil
}

// GetOrder 获取订单详情
func (s *membershipService) GetOrder(ctx context.Context, userID uint, orderID uint) (*OrderResponse, error) {
	order, err := s.membershipRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "订单不存在")
	}

	// 验证订单所属用户
	if order.UserID != userID {
		return nil, common.NewBusinessError(common.CodeForbidden, "无权访问该订单")
	}

	return convertToOrderResponse(order), nil
}

// ListOrders 获取订单列表
func (s *membershipService) ListOrders(ctx context.Context, userID uint, page, pageSize int) (*common.PageResult, error) {
	// 统一规范化分页参数，确保会员订单列表与其他后台列表接口保持一致。
	pageParam := common.PageParam{Page: page, PageSize: pageSize}
	pageParam.Normalize()

	orders, total, err := s.membershipRepo.ListByUser(ctx, userID, pageParam.Page, pageParam.PageSize)
	if err != nil {
		return nil, err
	}

	// 转换为响应DTO
	list := make([]OrderResponse, len(orders))
	for i, order := range orders {
		list[i] = *convertToOrderResponse(&order)
	}

	return common.NewPageResult(list, total, pageParam), nil
}

// MockPayCallback Mock支付回调
func (s *membershipService) MockPayCallback(ctx context.Context, orderNo string) (*OrderResponse, error) {
	// 查找订单
	order, err := s.membershipRepo.GetOrderByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "订单不存在")
	}

	// 检查订单状态
	if order.Status != model.OrderStatusPending {
		return nil, common.NewBusinessError(common.CodeBadRequest, "订单状态不正确")
	}

	// 查找会员方案获取时长
	var duration int
	for _, p := range membershipPlans {
		if p.PlanType == order.PlanType {
			duration = p.Duration
			break
		}
	}
	if duration == 0 {
		return nil, common.NewBusinessError(common.CodeInternalError, "会员方案配置错误")
	}

	// 计算过期时间
	now := time.Now()
	expiresAt := now.AddDate(0, 0, duration)

	// 更新订单状态
	order.Status = model.OrderStatusPaid
	order.PaidAt = &now
	order.ExpiresAt = &expiresAt

	if err := s.membershipRepo.UpdateOrder(ctx, order); err != nil {
		return nil, err
	}

	// 更新用户会员等级和过期时间
	user, err := s.userRepo.GetByID(ctx, order.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "用户不存在")
	}

	user.MembershipLevel = model.MembershipLevelPro
	user.MembershipExpireAt = &expiresAt

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return convertToOrderResponse(order), nil
}

// GetMembershipStatus 获取会员状态
func (s *membershipService) GetMembershipStatus(ctx context.Context, userID uint) (*MembershipStatusResponse, error) {
	// 获取用户信息
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "用户不存在")
	}

	// 检查是否为Pro会员且未过期
	isPro := user.IsPro()

	// 获取今日使用次数（这里简化处理，实际应该从Redis或数据库统计）
	// TODO: 后续实现真实的每日使用统计
	practiceUsedToday := 0
	interviewUsedToday := 0

	// 获取免费用户限制（后续可从admin_config读取）
	dailyPracticeLimit := DefaultDailyPracticeLimit
	dailyInterviewLimit := DefaultDailyInterviewLimit

	status := &MembershipStatusResponse{
		Level:               model.MembershipLevelFree,
		IsActive:            false,
		DailyPracticeLimit:  dailyPracticeLimit,
		DailyInterviewLimit: dailyInterviewLimit,
		PracticeUsedToday:   practiceUsedToday,
		InterviewUsedToday:  interviewUsedToday,
	}

	if isPro {
		status.Level = model.MembershipLevelPro
		status.ExpiresAt = user.MembershipExpireAt
		status.IsActive = true
		// Pro会员无限制
		status.DailyPracticeLimit = -1
		status.DailyInterviewLimit = -1
	}

	return status, nil
}

// generateOrderNo 生成订单号: MJ + yyyyMMddHHmmss + 6位随机数
func generateOrderNo() string {
	now := time.Now()
	timestamp := now.Format("20060102150405")
	randomNum := rand.Intn(900000) + 100000 // 6位随机数
	return fmt.Sprintf("MJ%s%d", timestamp, randomNum)
}

// convertToOrderResponse 将订单模型转换为响应DTO
func convertToOrderResponse(order *model.MembershipOrder) *OrderResponse {
	return &OrderResponse{
		ID:        order.ID,
		OrderNo:   order.OrderNo,
		PlanType:  order.PlanType,
		Amount:    order.Amount,
		Status:    order.Status,
		CreatedAt: order.CreatedAt,
		PaidAt:    order.PaidAt,
		ExpiresAt: order.ExpiresAt,
	}
}

// ParseMembershipLevel 解析会员等级
func ParseMembershipLevel(level string) string {
	if level == model.MembershipLevelPro {
		return model.MembershipLevelPro
	}
	return model.MembershipLevelFree
}

// IsValidPlanType 检查是否为有效的会员方案类型
func IsValidPlanType(planType string) bool {
	for _, p := range membershipPlans {
		if p.PlanType == planType {
			return true
		}
	}
	return false
}

// GetPlanByType 根据类型获取会员方案
func GetPlanByType(planType string) *MembershipPlanResponse {
	for _, p := range membershipPlans {
		if p.PlanType == planType {
			return &p
		}
	}
	return nil
}

// GetPlanDuration 获取方案时长
func GetPlanDuration(planType string) int {
	for _, p := range membershipPlans {
		if p.PlanType == planType {
			return p.Duration
		}
	}
	return 0
}

// GetPlanPrice 获取方案价格
func GetPlanPrice(planType string) float64 {
	for _, p := range membershipPlans {
		if p.PlanType == planType {
			return p.Price
		}
	}
	return 0
}

// GetPlanOriginalPrice 获取方案原价
func GetPlanOriginalPrice(planType string) float64 {
	for _, p := range membershipPlans {
		if p.PlanType == planType {
			return p.OrigPrice
		}
	}
	return 0
}

// GetPlanName 获取方案名称
func GetPlanName(planType string) string {
	for _, p := range membershipPlans {
		if p.PlanType == planType {
			return p.Name
		}
	}
	return ""
}

// GetPlanFeatures 获取方案特性
func GetPlanFeatures(planType string) []string {
	for _, p := range membershipPlans {
		if p.PlanType == planType {
			return p.Features
		}
	}
	return nil
}

// IsPopularPlan 检查是否为热门方案
func IsPopularPlan(planType string) bool {
	for _, p := range membershipPlans {
		if p.PlanType == planType {
			return p.IsPopular
		}
	}
	return false
}

// GetDailyPracticeLimit 获取每日刷题限制
func GetDailyPracticeLimit() int {
	return DefaultDailyPracticeLimit
}

// GetDailyInterviewLimit 获取每日面试限制
func GetDailyInterviewLimit() int {
	return DefaultDailyInterviewLimit
}

// CheckPracticeLimit 检查刷题限制
func CheckPracticeLimit(usedToday int) bool {
	return usedToday < DefaultDailyPracticeLimit
}

// CheckInterviewLimit 检查面试限制
func CheckInterviewLimit(usedToday int) bool {
	return usedToday < DefaultDailyInterviewLimit
}

// GetRemainingPracticeCount 获取剩余刷题次数
func GetRemainingPracticeCount(usedToday int) int {
	remaining := DefaultDailyPracticeLimit - usedToday
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetRemainingInterviewCount 获取剩余面试次数
func GetRemainingInterviewCount(usedToday int) int {
	remaining := DefaultDailyInterviewLimit - usedToday
	if remaining < 0 {
		return 0
	}
	return remaining
}

// FormatPrice 格式化价格
func FormatPrice(price float64) string {
	return strconv.FormatFloat(price, 'f', 2, 64)
}

// CalculateDiscount 计算折扣
func CalculateDiscount(price, originalPrice float64) float64 {
	if originalPrice <= 0 {
		return 0
	}
	discount := (1 - price/originalPrice) * 100
	return discount
}
