package data

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"makejob/app/membership/internal/biz"
)

// --- 订单仓库实现 ---

type orderRepo struct {
	db *gorm.DB
}

// NewOrderRepo 创建订单仓库实现
func NewOrderRepo(db *gorm.DB) biz.OrderRepo {
	return &orderRepo{db: db}
}

// Create 创建订单记录
func (r *orderRepo) Create(ctx context.Context, order *biz.MembershipOrder) error {
	return r.db.WithContext(ctx).Create(order).Error
}

// GetByOrderNo 根据订单号查询订单
func (r *orderRepo) GetByOrderNo(ctx context.Context, orderNo string) (*biz.MembershipOrder, error) {
	var order biz.MembershipOrder
	if err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// GetByID 根据订单 ID 查询订单
func (r *orderRepo) GetByID(ctx context.Context, id uint64) (*biz.MembershipOrder, error) {
	var order biz.MembershipOrder
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// ListByUserID 分页查询用户订单列表，返回订单列表和总数
func (r *orderRepo) ListByUserID(ctx context.Context, userID uint64, page, pageSize int) ([]*biz.MembershipOrder, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&biz.MembershipOrder{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var orders []*biz.MembershipOrder
	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// UpdateStatus 更新订单状态、支付时间和交易 ID
func (r *orderRepo) UpdateStatus(ctx context.Context, orderNo string, status string, paidAt *time.Time, transactionID string) error {
	return r.db.WithContext(ctx).Model(&biz.MembershipOrder{}).Where("order_no = ?", orderNo).Updates(map[string]interface{}{
		"status":         status,
		"paid_at":        paidAt,
		"transaction_id": transactionID,
	}).Error
}

// --- 会员仓库实现 ---

type membershipRepo struct {
	db *gorm.DB
}

// NewMembershipRepo 创建会员仓库实现
func NewMembershipRepo(db *gorm.DB) biz.MembershipRepo {
	return &membershipRepo{db: db}
}

// GetByUserID 根据用户 ID 查询会员信息
func (r *membershipRepo) GetByUserID(ctx context.Context, userID uint64) (*biz.UserMembership, error) {
	var m biz.UserMembership
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Upsert 创建或更新用户会员信息（基于 user_id 冲突更新）
func (r *membershipRepo) Upsert(ctx context.Context, membership *biz.UserMembership) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"level", "expires_at", "updated_at"}),
	}).Create(membership).Error
}
