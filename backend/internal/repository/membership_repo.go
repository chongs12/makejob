// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// MembershipRepository 会员订单数据访问接口
type MembershipRepository interface {
	CreateOrder(ctx context.Context, order *model.MembershipOrder) error
	GetOrderByID(ctx context.Context, id uint) (*model.MembershipOrder, error)
	GetOrderByOrderNo(ctx context.Context, orderNo string) (*model.MembershipOrder, error)
	UpdateOrder(ctx context.Context, order *model.MembershipOrder) error
	ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]model.MembershipOrder, int64, error)
	GetActiveOrder(ctx context.Context, userID uint) (*model.MembershipOrder, error)
}

// membershipRepository 会员订单数据访问实现
type membershipRepository struct {
	db *gorm.DB
}

// NewMembershipRepository 创建会员订单仓库实例
func NewMembershipRepository(db *gorm.DB) MembershipRepository {
	return &membershipRepository{
		db: db,
	}
}

// CreateOrder 创建订单
func (r *membershipRepository) CreateOrder(ctx context.Context, order *model.MembershipOrder) error {
	if err := r.db.WithContext(ctx).Create(order).Error; err != nil {
		return fmt.Errorf("创建订单失败: %w", err)
	}
	return nil
}

// GetOrderByID 根据ID获取订单
func (r *membershipRepository) GetOrderByID(ctx context.Context, id uint) (*model.MembershipOrder, error) {
	var order model.MembershipOrder
	if err := r.db.WithContext(ctx).First(&order, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询订单失败: %w", err)
	}
	return &order, nil
}

// GetOrderByOrderNo 根据订单号获取订单
func (r *membershipRepository) GetOrderByOrderNo(ctx context.Context, orderNo string) (*model.MembershipOrder, error) {
	var order model.MembershipOrder
	if err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("根据订单号查询订单失败: %w", err)
	}
	return &order, nil
}

// UpdateOrder 更新订单
func (r *membershipRepository) UpdateOrder(ctx context.Context, order *model.MembershipOrder) error {
	if err := r.db.WithContext(ctx).Save(order).Error; err != nil {
		return fmt.Errorf("更新订单失败: %w", err)
	}
	return nil
}

// ListByUser 获取用户的订单列表
func (r *membershipRepository) ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]model.MembershipOrder, int64, error) {
	var orders []model.MembershipOrder
	var total int64

	// 计算偏移量
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	// 查询总数
	if err := r.db.WithContext(ctx).Model(&model.MembershipOrder{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("查询订单总数失败: %w", err)
	}

	// 查询列表
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&orders).Error; err != nil {
		return nil, 0, fmt.Errorf("查询订单列表失败: %w", err)
	}

	return orders, total, nil
}

// GetActiveOrder 获取用户当前有效的会员订单
func (r *membershipRepository) GetActiveOrder(ctx context.Context, userID uint) (*model.MembershipOrder, error) {
	var order model.MembershipOrder
	now := time.Now()

	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND expires_at > ?", userID, model.OrderStatusPaid, now).
		Order("expires_at DESC").
		First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询有效订单失败: %w", err)
	}
	return &order, nil
}
