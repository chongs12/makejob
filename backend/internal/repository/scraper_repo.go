// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// ScraperTaskRepository 爬取任务仓库接口
type ScraperTaskRepository interface {
	Create(ctx context.Context, task *model.ScraperTask) error
	Update(ctx context.Context, task *model.ScraperTask) error
	List(ctx context.Context, page, pageSize int) ([]model.ScraperTask, int64, error)
	GetByID(ctx context.Context, id uint) (*model.ScraperTask, error)
}

// scraperTaskRepository 爬取任务仓库实现
type scraperTaskRepository struct {
	db *gorm.DB
}

// NewScraperTaskRepository 创建爬取任务仓库实例
func NewScraperTaskRepository(db *gorm.DB) ScraperTaskRepository {
	return &scraperTaskRepository{db: db}
}

// Create 创建爬取任务
func (r *scraperTaskRepository) Create(ctx context.Context, task *model.ScraperTask) error {
	if err := r.db.WithContext(ctx).Create(task).Error; err != nil {
		return fmt.Errorf("创建爬取任务失败: %w", err)
	}
	return nil
}

// Update 更新爬取任务
func (r *scraperTaskRepository) Update(ctx context.Context, task *model.ScraperTask) error {
	if err := r.db.WithContext(ctx).Save(task).Error; err != nil {
		return fmt.Errorf("更新爬取任务失败: %w", err)
	}
	return nil
}

// List 获取爬取任务列表
func (r *scraperTaskRepository) List(ctx context.Context, page, pageSize int) ([]model.ScraperTask, int64, error) {
	var tasks []model.ScraperTask
	var total int64

	query := r.db.WithContext(ctx).Model(&model.ScraperTask{})

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计爬取任务数量失败: %w", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("查询爬取任务列表失败: %w", err)
	}

	return tasks, total, nil
}

// GetByID 根据ID获取爬取任务
func (r *scraperTaskRepository) GetByID(ctx context.Context, id uint) (*model.ScraperTask, error) {
	var task model.ScraperTask
	if err := r.db.WithContext(ctx).First(&task, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询爬取任务失败: %w", err)
	}
	return &task, nil
}
