package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/plan/internal/biz"
)

type planRepo struct {
	db *gorm.DB
}

// NewPlanRepo 创建学习计划仓库实现
func NewPlanRepo(db *gorm.DB) biz.PlanRepo {
	return &planRepo{db: db}
}

func (r *planRepo) Create(ctx context.Context, plan *biz.Plan) error {
	return r.db.WithContext(ctx).Create(plan).Error
}

func (r *planRepo) GetByID(ctx context.Context, id uint64) (*biz.Plan, error) {
	var plan biz.Plan
	if err := r.db.WithContext(ctx).First(&plan, id).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *planRepo) GetByUserID(ctx context.Context, userID uint64) (*biz.Plan, error) {
	var plan biz.Plan
	if err := r.db.WithContext(ctx).Where("user_id = ? AND status = ?", userID, "active").First(&plan).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}
