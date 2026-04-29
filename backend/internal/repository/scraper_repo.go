// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"makejob-backend/internal/model"
	"makejob-backend/internal/scraper"
)

// ScraperTaskRepository 爬取任务仓库接口
type ScraperTaskRepository interface {
	Create(ctx context.Context, task *model.ScraperTask) error
	Update(ctx context.Context, task *model.ScraperTask) error
	List(ctx context.Context, page, pageSize int, filter scraper.TaskListFilter) ([]model.ScraperTask, int64, error)
	GetByID(ctx context.Context, id uint) (*model.ScraperTask, error)
	ClaimNextPending(ctx context.Context, taskType string) (*model.ScraperTask, error)
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
func (r *scraperTaskRepository) List(ctx context.Context, page, pageSize int, filter scraper.TaskListFilter) ([]model.ScraperTask, int64, error) {
	var tasks []model.ScraperTask
	var total int64

	query := r.db.WithContext(ctx).Model(&model.ScraperTask{})
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if taskType := strings.TrimSpace(filter.TaskType); taskType != "" {
		query = query.Where("task_type = ?", taskType)
	}

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

// ClaimNextPending 以行锁方式领取下一条待执行任务，并立即标记为 running，避免多 worker 重复消费。
func (r *scraperTaskRepository) ClaimNextPending(ctx context.Context, taskType string) (*model.ScraperTask, error) {
	var claimed *model.ScraperTask

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.ScraperTask
		query := tx.Model(&model.ScraperTask{}).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ?", scraper.TaskStatusPending).
			Order("created_at ASC")
		if taskType != "" {
			query = query.Where("task_type = ?", taskType)
		}
		if err := query.First(&task).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return fmt.Errorf("领取待执行任务失败: %w", err)
		}

		now := time.Now()
		task.Status = scraper.TaskStatusRunning
		task.StartedAt = &now
		task.RetryCount++
		if err := tx.Save(&task).Error; err != nil {
			return fmt.Errorf("标记任务为运行中失败: %w", err)
		}
		claimed = &task
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}
