package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/companion/internal/biz"
)

type companionRepo struct {
	db *gorm.DB
}

// NewCompanionRepo 创建陪伴助手仓库实现
func NewCompanionRepo(db *gorm.DB) biz.CompanionRepo {
	return &companionRepo{db: db}
}

func (r *companionRepo) GetState(ctx context.Context, userID uint64) (*biz.CompanionState, error) {
	var state biz.CompanionState
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}
