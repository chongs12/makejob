package biz

import (
	"context"
)

// PlanRepo 学习计划仓库接口，data 层必须实现
type PlanRepo interface {
	// Create 创建学习计划
	Create(ctx context.Context, plan *Plan) error
	// GetByID 根据 ID 查询计划
	GetByID(ctx context.Context, id uint64) (*Plan, error)
	// GetByUserID 查询用户当前计划
	GetByUserID(ctx context.Context, userID uint64) (*Plan, error)
}

// Plan 学习计划领域实体
type Plan struct {
	ID           uint64 `gorm:"primaryKey"`
	UserID       uint64 `gorm:"index"`
	IndustryCode string `gorm:"size:50"`
	Goal         string `gorm:"size:200"`
	Status       string `gorm:"size:20;not null;default:'active'"`
}

// PlanUseCase 学习计划业务用例
type PlanUseCase struct {
	repo PlanRepo
}

// NewPlanUseCase 创建学习计划业务用例
func NewPlanUseCase(repo PlanRepo) *PlanUseCase {
	return &PlanUseCase{repo: repo}
}

// Create 创建学习计划
func (uc *PlanUseCase) Create(ctx context.Context, plan *Plan) error {
	return uc.repo.Create(ctx, plan)
}

// GetByID 根据 ID 查询计划
func (uc *PlanUseCase) GetByID(ctx context.Context, id uint64) (*Plan, error) {
	return uc.repo.GetByID(ctx, id)
}

// GetByUserID 查询用户当前计划
func (uc *PlanUseCase) GetByUserID(ctx context.Context, userID uint64) (*Plan, error) {
	return uc.repo.GetByUserID(ctx, userID)
}
