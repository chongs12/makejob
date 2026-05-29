// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"makejob-backend/internal/model"
)

// AsyncTaskRepository 定义通用异步任务的数据访问接口。
type AsyncTaskRepository interface {
	Create(ctx context.Context, task *model.AsyncTask) error
	GetByID(ctx context.Context, id uint) (*model.AsyncTask, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*model.AsyncTask, error)
	GetLatestByEntity(ctx context.Context, entityType string, entityID uint, taskTypes ...string) (*model.AsyncTask, error)
	Update(ctx context.Context, task *model.AsyncTask) error
	ClaimByID(ctx context.Context, id uint) (*model.AsyncTask, bool, error)
}

// asyncTaskRepository 提供通用异步任务仓储实现。
type asyncTaskRepository struct {
	db *gorm.DB
}

// NewAsyncTaskRepository 创建通用异步任务仓库实例。
func NewAsyncTaskRepository(db *gorm.DB) AsyncTaskRepository {
	return &asyncTaskRepository{db: db}
}

// Create 创建异步任务记录。
func (r *asyncTaskRepository) Create(ctx context.Context, task *model.AsyncTask) error {
	if err := r.db.WithContext(ctx).Create(task).Error; err != nil {
		return fmt.Errorf("创建异步任务失败: %w", err)
	}
	return nil
}

// GetByID 根据任务 ID 查询异步任务详情。
func (r *asyncTaskRepository) GetByID(ctx context.Context, id uint) (*model.AsyncTask, error) {
	var task model.AsyncTask
	if err := r.db.WithContext(ctx).First(&task, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询异步任务失败: %w", err)
	}
	return &task, nil
}

// GetByIdempotencyKey 根据幂等键查询异步任务，避免重复创建。
func (r *asyncTaskRepository) GetByIdempotencyKey(ctx context.Context, key string) (*model.AsyncTask, error) {
	var task model.AsyncTask
	if err := r.db.WithContext(ctx).Where("idempotency_key = ?", key).First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("按幂等键查询异步任务失败: %w", err)
	}
	return &task, nil
}

// GetLatestByEntity 查询某个业务实体最近一次异步任务，供接口轮询展示处理进度。
func (r *asyncTaskRepository) GetLatestByEntity(ctx context.Context, entityType string, entityID uint, taskTypes ...string) (*model.AsyncTask, error) {
	query := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at DESC, id DESC")
	if len(taskTypes) > 0 {
		query = query.Where("task_type IN ?", taskTypes)
	}

	var task model.AsyncTask
	if err := query.First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询实体最新异步任务失败: %w", err)
	}
	return &task, nil
}

// Update 更新异步任务状态与执行结果。
func (r *asyncTaskRepository) Update(ctx context.Context, task *model.AsyncTask) error {
	if err := r.db.WithContext(ctx).Save(task).Error; err != nil {
		return fmt.Errorf("更新异步任务失败: %w", err)
	}
	return nil
}

// ClaimByID 通过行锁领取指定任务，避免同一消息被多个消费者并发执行。
func (r *asyncTaskRepository) ClaimByID(ctx context.Context, id uint) (*model.AsyncTask, bool, error) {
	var claimed *model.AsyncTask
	var shouldRun bool

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.AsyncTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return fmt.Errorf("领取异步任务失败: %w", err)
		}

		switch task.Status {
		case model.AsyncTaskStatusSucceeded, model.AsyncTaskStatusDead, model.AsyncTaskStatusRunning:
			claimed = &task
			shouldRun = false
			return nil
		}

		now := time.Now()
		task.Status = model.AsyncTaskStatusRunning
		task.StartedAt = &now
		task.FinishedAt = nil
		task.RetryCount++
		if err := tx.Save(&task).Error; err != nil {
			return fmt.Errorf("更新异步任务运行状态失败: %w", err)
		}
		claimed = &task
		shouldRun = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return claimed, shouldRun, nil
}
