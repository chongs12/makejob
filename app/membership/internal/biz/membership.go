package biz

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"gorm.io/gorm"
)

// BaseModel 所有实体公共基础字段（FIX I4: 使用 gorm.DeletedAt 符合全局规范）
type BaseModel struct {
	ID        uint           `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time      `gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"not null;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// MembershipOrder 会员订单实体
type MembershipOrder struct {
	BaseModel
	UserID        uint64  `gorm:"index;not null"`
	OrderNo       string  `gorm:"size:32;uniqueIndex;not null"`
	PlanID        string  `gorm:"size:20;not null"`
	Amount        float64 `gorm:"not null"`
	PaymentMethod string  `gorm:"size:20"`
	TransactionID string  `gorm:"size:100"`
	Status        string  `gorm:"size:20;not null;default:'pending'"`
	PaidAt        *time.Time
	ExpiresAt     time.Time
}

// TableName 返回订单表名
func (MembershipOrder) TableName() string { return "membership_orders" }

// UserMembership 用户会员信息实体
type UserMembership struct {
	BaseModel
	UserID    uint64 `gorm:"uniqueIndex;not null"`
	Level     string `gorm:"size:20;default:'free'"`
	ExpiresAt time.Time
}

// TableName 返回用户会员表名
func (UserMembership) TableName() string { return "user_memberships" }

// OrderRepo 订单仓库接口，data 层必须实现
type OrderRepo interface {
	Create(ctx context.Context, order *MembershipOrder) error
	GetByOrderNo(ctx context.Context, orderNo string) (*MembershipOrder, error)
	GetByID(ctx context.Context, id uint64) (*MembershipOrder, error)
	ListByUserID(ctx context.Context, userID uint64, page, pageSize int) ([]*MembershipOrder, int64, error)
	UpdateStatus(ctx context.Context, orderNo string, status string, paidAt *time.Time, transactionID string) error
}

// MembershipRepo 会员仓库接口，data 层必须实现
type MembershipRepo interface {
	GetByUserID(ctx context.Context, userID uint64) (*UserMembership, error)
	Upsert(ctx context.Context, membership *UserMembership) error
}

// MembershipPlan 套餐定义
type MembershipPlan struct {
	PlanType     string
	Name         string
	Price        float64
	DurationDays int32
	Features     []string
}

// 预定义业务错误
var (
	ErrOrderNotFound      = kratosErr.NotFound("ORDER_NOT_FOUND", "订单不存在")
	ErrPlanInvalid        = kratosErr.BadRequest("PLAN_INVALID", "套餐类型无效")
	ErrOrderNotPending    = kratosErr.BadRequest("ORDER_NOT_PENDING", "订单状态非待支付")
	ErrMembershipNotFound = kratosErr.NotFound("MEMBERSHIP_NOT_FOUND", "会员信息不存在")
)

// MembershipUseCase 会员业务用例（FIX I8: 套餐配置从全局变量移入 UseCase 结构体）
type MembershipUseCase struct {
	orderRepo      OrderRepo
	membershipRepo MembershipRepo
	rng            *rand.Rand       // FIX I7: 使用显式种子的随机数生成器
	plans          []MembershipPlan // FIX I8: 套餐列表
	priceMap       map[string]float64
	daysMap        map[string]int
}

// NewMembershipUseCase 创建会员业务用例
func NewMembershipUseCase(orderRepo OrderRepo, membershipRepo MembershipRepo) *MembershipUseCase {
	plans := []MembershipPlan{
		{PlanType: "monthly", Name: "月度会员", Price: 29.9, DurationDays: 30, Features: []string{"unlimited_practice", "unlimited_interview", "advanced_ai"}},
		{PlanType: "quarterly", Name: "季度会员", Price: 79.9, DurationDays: 90, Features: []string{"unlimited_practice", "unlimited_interview", "advanced_ai"}},
		{PlanType: "yearly", Name: "年度会员", Price: 299.9, DurationDays: 365, Features: []string{"unlimited_practice", "unlimited_interview", "advanced_ai"}},
	}
	priceMap := map[string]float64{
		"monthly":   29.9,
		"quarterly": 79.9,
		"yearly":    299.9,
	}
	daysMap := map[string]int{
		"monthly":   30,
		"quarterly": 90,
		"yearly":    365,
	}
	return &MembershipUseCase{
		orderRepo:      orderRepo,
		membershipRepo: membershipRepo,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())), // FIX I7
		plans:          plans,
		priceMap:       priceMap,
		daysMap:        daysMap,
	}
}

// CreateOrder 创建会员订单，生成唯一订单号
func (uc *MembershipUseCase) CreateOrder(ctx context.Context, userID uint64, planType string) (*MembershipOrder, error) {
	price, ok := uc.priceMap[planType]
	if !ok {
		return nil, ErrPlanInvalid
	}
	days := uc.daysMap[planType]
	orderNo := fmt.Sprintf("MJ%s%06d", time.Now().Format("20060102150405"), uc.rng.Intn(1000000))
	order := &MembershipOrder{
		UserID:    userID,
		OrderNo:   orderNo,
		PlanID:    planType,
		Amount:    price,
		Status:    "pending",
		ExpiresAt: time.Now().AddDate(0, 0, days),
	}
	if err := uc.orderRepo.Create(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}

// HandlePaymentCallback 处理支付回调：验证订单状态、更新为已支付、更新用户会员信息
// FIX I2: 已支付订单重复回调时返回成功（幂等）
func (uc *MembershipUseCase) HandlePaymentCallback(ctx context.Context, orderNo string, transactionID string) (*MembershipOrder, error) {
	order, err := uc.orderRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, ErrOrderNotFound
	}
	// FIX I2: 幂等处理 - 已支付的订单直接返回成功
	if order.Status == "paid" {
		return order, nil
	}
	if order.Status != "pending" {
		return nil, ErrOrderNotPending
	}
	now := time.Now()
	if err := uc.orderRepo.UpdateStatus(ctx, orderNo, "paid", &now, transactionID); err != nil {
		return nil, err
	}
	days := uc.daysMap[order.PlanID]
	expiresAt := now.AddDate(0, 0, days)
	membership := &UserMembership{
		UserID:    order.UserID,
		Level:     order.PlanID,
		ExpiresAt: expiresAt,
	}
	if err := uc.membershipRepo.Upsert(ctx, membership); err != nil {
		return nil, err
	}
	order.Status = "paid"
	order.PaidAt = &now
	order.TransactionID = transactionID
	return order, nil
}

// CheckFeatureAccess 检查用户是否具有指定功能的访问权限
func (uc *MembershipUseCase) CheckFeatureAccess(ctx context.Context, userID uint64, feature string) (bool, string, error) {
	membership, err := uc.membershipRepo.GetByUserID(ctx, userID)
	level := "free"
	if err == nil {
		level = membership.Level
		if !membership.ExpiresAt.IsZero() && membership.ExpiresAt.Before(time.Now()) {
			level = "free"
		}
	}
	allowed, reason := getFeatureAccess(level, feature)
	return allowed, reason, nil
}

// GetUserMembership 查询用户会员信息，自动检查是否过期
func (uc *MembershipUseCase) GetUserMembership(ctx context.Context, userID uint64) (*UserMembership, error) {
	membership, err := uc.membershipRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, ErrMembershipNotFound
	}
	if !membership.ExpiresAt.IsZero() && membership.ExpiresAt.Before(time.Now()) {
		membership.Level = "free"
	}
	return membership, nil
}

// ListPlans 返回所有可用的会员套餐列表
func (uc *MembershipUseCase) ListPlans() []MembershipPlan {
	return uc.plans
}

// UpgradeMembership 管理员手动升级用户会员
func (uc *MembershipUseCase) UpgradeMembership(ctx context.Context, userID uint64, level string, durationDays int32) (*UserMembership, error) {
	if _, ok := uc.priceMap[level]; !ok {
		return nil, ErrPlanInvalid
	}
	expiresAt := time.Now().AddDate(0, 0, int(durationDays))
	membership := &UserMembership{
		UserID:    userID,
		Level:     level,
		ExpiresAt: expiresAt,
	}
	if err := uc.membershipRepo.Upsert(ctx, membership); err != nil {
		return nil, err
	}
	return membership, nil
}

// GetOrder 根据用户 ID 和订单 ID 查询订单，校验用户归属
func (uc *MembershipUseCase) GetOrder(ctx context.Context, userID uint64, orderID uint64) (*MembershipOrder, error) {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}
	if order.UserID != userID {
		return nil, ErrOrderNotFound
	}
	return order, nil
}

// ListOrders 分页查询用户订单列表
func (uc *MembershipUseCase) ListOrders(ctx context.Context, userID uint64, page, pageSize int) ([]*MembershipOrder, int64, error) {
	return uc.orderRepo.ListByUserID(ctx, userID, page, pageSize)
}

// getFeatureAccess 根据用户等级和功能名称返回权限结果
func getFeatureAccess(level string, feature string) (bool, string) {
	isPaid := level != "free"
	switch feature {
	case "unlimited_practice", "daily_practice":
		if isPaid {
			return true, ""
		}
		return false, "免费用户每日仅可练习 20 次，升级会员享受无限练习"
	case "unlimited_interview", "daily_interview":
		if isPaid {
			return true, ""
		}
		return false, "免费用户每日仅可模拟面试 2 次，升级会员享受无限面试"
	case "advanced_ai":
		if isPaid {
			return true, ""
		}
		return false, "高级 AI 功能仅对会员开放，请升级会员"
	default:
		return true, ""
	}
}
