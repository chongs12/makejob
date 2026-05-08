// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// PlanTaskFeedbackRepository 定义学习任务反馈的数据访问接口。
type PlanTaskFeedbackRepository interface {
	Create(ctx context.Context, feedback *model.LearningTaskFeedback) error
}

// planTaskFeedbackRepository 提供学习任务反馈仓储实现。
type planTaskFeedbackRepository struct {
	db *gorm.DB
}

// NewPlanTaskFeedbackRepository 创建学习任务反馈仓库实例。
func NewPlanTaskFeedbackRepository(db *gorm.DB) PlanTaskFeedbackRepository {
	return &planTaskFeedbackRepository{db: db}
}

// Create 创建学习任务反馈记录。
func (r *planTaskFeedbackRepository) Create(ctx context.Context, feedback *model.LearningTaskFeedback) error {
	if err := r.db.WithContext(ctx).Create(feedback).Error; err != nil {
		return fmt.Errorf("创建学习任务反馈失败: %w", err)
	}
	return nil
}
