// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// PlanRepository 学习计划数据访问接口
type PlanRepository interface {
	Create(ctx context.Context, plan *model.LearningPlan) error
	GetByID(ctx context.Context, id uint) (*model.LearningPlan, error)
	GetCurrentByUser(ctx context.Context, userID uint) (*model.LearningPlan, error)
	Update(ctx context.Context, plan *model.LearningPlan) error
	ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]model.LearningPlan, int64, error)
	PauseActivePlans(ctx context.Context, userID uint) error
}

// PlanTaskRepository 学习任务数据访问接口
type PlanTaskRepository interface {
	Create(ctx context.Context, task *model.LearningTask) error
	BatchCreate(ctx context.Context, tasks []model.LearningTask) error
	GetByID(ctx context.Context, id uint) (*model.LearningTask, error)
	Update(ctx context.Context, task *model.LearningTask) error
	ListByPlan(ctx context.Context, planID uint) ([]model.LearningTask, error)
	CountByPlanAndStatus(ctx context.Context, planID uint, status string) (int64, error)
	DeleteByPlan(ctx context.Context, planID uint) error
}

// planRepository 学习计划数据访问实现
type planRepository struct {
	db *gorm.DB
}

// planTaskRepository 学习任务数据访问实现
type planTaskRepository struct {
	db *gorm.DB
}

// NewPlanRepository 创建学习计划仓库实例
func NewPlanRepository(db *gorm.DB) PlanRepository {
	return &planRepository{db: db}
}

// NewPlanTaskRepository 创建学习任务仓库实例
func NewPlanTaskRepository(db *gorm.DB) PlanTaskRepository {
	return &planTaskRepository{db: db}
}

// Create 创建学习计划
func (r *planRepository) Create(ctx context.Context, plan *model.LearningPlan) error {
	if err := r.db.WithContext(ctx).Create(plan).Error; err != nil {
		return fmt.Errorf("创建学习计划失败: %w", err)
	}
	return nil
}

// GetByID 根据ID获取学习计划
func (r *planRepository) GetByID(ctx context.Context, id uint) (*model.LearningPlan, error) {
	var plan model.LearningPlan
	if err := r.db.WithContext(ctx).Preload("Tasks").First(&plan, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询学习计划失败: %w", err)
	}
	return &plan, nil
}

// GetCurrentByUser 获取用户当前进行中的计划
func (r *planRepository) GetCurrentByUser(ctx context.Context, userID uint) (*model.LearningPlan, error) {
	var plan model.LearningPlan
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, model.PlanStatusActive).
		Preload("Tasks", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		First(&plan).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询当前学习计划失败: %w", err)
	}
	return &plan, nil
}

// Update 更新学习计划
func (r *planRepository) Update(ctx context.Context, plan *model.LearningPlan) error {
	if err := r.db.WithContext(ctx).Save(plan).Error; err != nil {
		return fmt.Errorf("更新学习计划失败: %w", err)
	}
	return nil
}

// ListByUser 获取用户的学习计划列表
func (r *planRepository) ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]model.LearningPlan, int64, error) {
	var plans []model.LearningPlan
	var total int64

	// 查询总数
	if err := r.db.WithContext(ctx).Model(&model.LearningPlan{}).
		Where("user_id = ?", userID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("查询学习计划总数失败: %w", err)
	}

	// 查询列表
	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&plans).Error; err != nil {
		return nil, 0, fmt.Errorf("查询学习计划列表失败: %w", err)
	}

	return plans, total, nil
}

// PauseActivePlans 暂停用户所有活跃的计划
func (r *planRepository) PauseActivePlans(ctx context.Context, userID uint) error {
	if err := r.db.WithContext(ctx).
		Model(&model.LearningPlan{}).
		Where("user_id = ? AND status = ?", userID, model.PlanStatusActive).
		Update("status", model.PlanStatusPaused).Error; err != nil {
		return fmt.Errorf("暂停活跃计划失败: %w", err)
	}
	return nil
}

// Create 创建学习任务
func (r *planTaskRepository) Create(ctx context.Context, task *model.LearningTask) error {
	if err := r.db.WithContext(ctx).Create(task).Error; err != nil {
		return fmt.Errorf("创建学习任务失败: %w", err)
	}
	return nil
}

// BatchCreate 批量创建学习任务
func (r *planTaskRepository) BatchCreate(ctx context.Context, tasks []model.LearningTask) error {
	if len(tasks) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&tasks).Error; err != nil {
		return fmt.Errorf("批量创建学习任务失败: %w", err)
	}
	return nil
}

// GetByID 根据ID获取学习任务
func (r *planTaskRepository) GetByID(ctx context.Context, id uint) (*model.LearningTask, error) {
	var task model.LearningTask
	if err := r.db.WithContext(ctx).First(&task, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询学习任务失败: %w", err)
	}
	return &task, nil
}

// Update 更新学习任务
func (r *planTaskRepository) Update(ctx context.Context, task *model.LearningTask) error {
	if err := r.db.WithContext(ctx).Save(task).Error; err != nil {
		return fmt.Errorf("更新学习任务失败: %w", err)
	}
	return nil
}

// ListByPlan 获取计划的所有任务
func (r *planTaskRepository) ListByPlan(ctx context.Context, planID uint) ([]model.LearningTask, error) {
	var tasks []model.LearningTask
	if err := r.db.WithContext(ctx).
		Where("plan_id = ?", planID).
		Order("sort_order ASC").
		Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("查询学习任务列表失败: %w", err)
	}
	return tasks, nil
}

// CountByPlanAndStatus 统计计划下指定状态的任务数量
func (r *planTaskRepository) CountByPlanAndStatus(ctx context.Context, planID uint, status string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.LearningTask{}).
		Where("plan_id = ? AND status = ?", planID, status).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计任务数量失败: %w", err)
	}
	return count, nil
}

// DeleteByPlan 删除计划的所有任务
func (r *planTaskRepository) DeleteByPlan(ctx context.Context, planID uint) error {
	if err := r.db.WithContext(ctx).
		Where("plan_id = ?", planID).
		Delete(&model.LearningTask{}).Error; err != nil {
		return fmt.Errorf("删除学习任务失败: %w", err)
	}
	return nil
}
