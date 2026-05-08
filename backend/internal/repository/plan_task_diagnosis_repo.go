// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// PlanTaskDiagnosisRepository 定义学习任务诊断的数据访问接口。
type PlanTaskDiagnosisRepository interface {
	Upsert(ctx context.Context, diagnosis *model.LearningTaskDiagnosis) error
	ListRecentByPlan(ctx context.Context, planID uint, limit int) ([]model.LearningTaskDiagnosis, error)
}

// planTaskDiagnosisRepository 提供学习任务诊断仓储实现。
type planTaskDiagnosisRepository struct {
	db *gorm.DB
}

// NewPlanTaskDiagnosisRepository 创建学习任务诊断仓库实例。
func NewPlanTaskDiagnosisRepository(db *gorm.DB) PlanTaskDiagnosisRepository {
	return &planTaskDiagnosisRepository{db: db}
}

// Upsert 按反馈ID创建或更新学习任务诊断。
func (r *planTaskDiagnosisRepository) Upsert(ctx context.Context, diagnosis *model.LearningTaskDiagnosis) error {
	if diagnosis == nil {
		return fmt.Errorf("学习任务诊断不能为空")
	}

	var existing model.LearningTaskDiagnosis
	err := r.db.WithContext(ctx).
		Where("feedback_id = ?", diagnosis.FeedbackID).
		First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("查询学习任务诊断失败: %w", err)
	}

	if err == gorm.ErrRecordNotFound {
		if err := r.db.WithContext(ctx).Create(diagnosis).Error; err != nil {
			return fmt.Errorf("创建学习任务诊断失败: %w", err)
		}
		return nil
	}

	diagnosis.ID = existing.ID
	diagnosis.CreatedAt = existing.CreatedAt
	diagnosis.UpdatedAt = existing.UpdatedAt
	if err := r.db.WithContext(ctx).Save(diagnosis).Error; err != nil {
		return fmt.Errorf("更新学习任务诊断失败: %w", err)
	}
	return nil
}

// ListRecentByPlan 获取指定学习计划最近的任务诊断结果。
func (r *planTaskDiagnosisRepository) ListRecentByPlan(ctx context.Context, planID uint, limit int) ([]model.LearningTaskDiagnosis, error) {
	if limit <= 0 {
		limit = 10
	}

	var diagnoses []model.LearningTaskDiagnosis
	if err := r.db.WithContext(ctx).
		Where("plan_id = ?", planID).
		Order("COALESCE(occurred_at, updated_at) DESC").
		Limit(limit).
		Find(&diagnoses).Error; err != nil {
		return nil, fmt.Errorf("查询学习任务诊断失败: %w", err)
	}
	return diagnoses, nil
}
