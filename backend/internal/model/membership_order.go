// Package model 提供数据模型定义
package model

import (
	"time"
)

// PlanType 会员计划类型枚举
const (
	PlanTypeMonthly   = "monthly"   // 月卡
	PlanTypeQuarterly = "quarterly" // 季卡
	PlanTypeYearly    = "yearly"    // 年卡
)

// OrderStatus 订单状态枚举
const (
	OrderStatusPending   = "pending"   // 待支付
	OrderStatusPaid      = "paid"      // 已支付
	OrderStatusCancelled = "cancelled" // 已取消
	OrderStatusRefunded  = "refunded"  // 已退款
)

// MembershipOrder 会员订单表
type MembershipOrder struct {
	BaseModel
	UserID    uint       `json:"user_id" gorm:"not null;index;comment:用户ID"`
	OrderNo   string     `json:"order_no" gorm:"size:64;not null;uniqueIndex;comment:订单编号"`
	PlanType  string     `json:"plan_type" gorm:"size:20;not null;comment:计划类型(monthly/quarterly/yearly)"`
	Amount    float64    `json:"amount" gorm:"type:decimal(10,2);not null;comment:订单金额"`
	Status    string     `json:"status" gorm:"size:20;not null;default:'pending';comment:订单状态(pending/paid/cancelled/refunded)"`
	PaidAt    *time.Time `json:"paid_at" gorm:"comment:支付时间"`
	ExpiresAt *time.Time `json:"expires_at" gorm:"comment:会员过期时间"`

	// 关联关系
	User User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName 指定表名
func (MembershipOrder) TableName() string {
	return "membership_orders"
}

// IsPaid 判断订单是否已支付
func (o *MembershipOrder) IsPaid() bool {
	return o.Status == OrderStatusPaid
}

// IsPending 判断订单是否待支付
func (o *MembershipOrder) IsPending() bool {
	return o.Status == OrderStatusPending
}
