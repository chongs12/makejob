package biz

import (
	"context"
)

// MembershipRepo 会员仓库接口，data 层必须实现
type MembershipRepo interface {
	// GetByUserID 根据用户 ID 查询会员信息
	GetByUserID(ctx context.Context, userID uint64) (*Membership, error)
}

// Membership 会员领域实体
type Membership struct {
	ID     uint64 `gorm:"primaryKey"`
	UserID uint64 `gorm:"uniqueIndex"`
	Level  string `gorm:"size:20;not null;default:'free'"`
}

// MembershipUseCase 会员业务用例
type MembershipUseCase struct {
	repo MembershipRepo
}

// NewMembershipUseCase 创建会员业务用例
func NewMembershipUseCase(repo MembershipRepo) *MembershipUseCase {
	return &MembershipUseCase{repo: repo}
}

// GetByUserID 根据用户 ID 查询会员信息
func (uc *MembershipUseCase) GetByUserID(ctx context.Context, userID uint64) (*Membership, error) {
	return uc.repo.GetByUserID(ctx, userID)
}
