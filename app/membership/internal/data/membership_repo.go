package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/membership/internal/biz"
)

type membershipRepo struct {
	db *gorm.DB
}

// NewMembershipRepo 创建会员仓库实现
func NewMembershipRepo(db *gorm.DB) biz.MembershipRepo {
	return &membershipRepo{db: db}
}

func (r *membershipRepo) GetByUserID(ctx context.Context, userID uint64) (*biz.Membership, error) {
	var m biz.Membership
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}
