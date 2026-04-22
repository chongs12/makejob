// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// AICallLogListParams AI 调用日志查询参数。
type AICallLogListParams struct {
	Page     int
	PageSize int
	Scene    string
	Source   string
	Status   string
	TraceID  string
}

// AICallLogRepository AI 调用日志仓库接口。
type AICallLogRepository interface {
	Create(ctx context.Context, log *model.AICallLog) error
	List(ctx context.Context, params AICallLogListParams) ([]model.AICallLog, int64, error)
	GetLatestByTraceID(ctx context.Context, traceID string) (*model.AICallLog, error)
}

// aiCallLogRepository AI 调用日志仓库实现。
type aiCallLogRepository struct {
	db *gorm.DB
}

// NewAICallLogRepository 创建 AI 调用日志仓库。
func NewAICallLogRepository(db *gorm.DB) AICallLogRepository {
	return &aiCallLogRepository{db: db}
}

// Create 创建一条 AI 调用日志。
func (r *aiCallLogRepository) Create(ctx context.Context, log *model.AICallLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("创建 AI 调用日志失败: %w", err)
	}
	return nil
}

// List 按条件分页查询 AI 调用日志。
func (r *aiCallLogRepository) List(ctx context.Context, params AICallLogListParams) ([]model.AICallLog, int64, error) {
	logs := make([]model.AICallLog, 0)
	var total int64

	query := r.db.WithContext(ctx).Model(&model.AICallLog{})
	if params.Scene != "" {
		query = query.Where("scene = ?", params.Scene)
	}
	if params.Source != "" {
		query = query.Where("source = ?", params.Source)
	}
	switch params.Status {
	case "success":
		query = query.Where("is_success = ?", true)
	case "failed":
		query = query.Where("is_success = ?", false)
	}
	if params.TraceID != "" {
		query = query.Where("trace_id LIKE ?", "%"+params.TraceID+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计 AI 调用日志失败: %w", err)
	}

	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}

	offset := (params.Page - 1) * params.PageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(params.PageSize).Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("查询 AI 调用日志失败: %w", err)
	}

	return logs, total, nil
}

// GetLatestByTraceID 按 trace_id 获取最近一条 AI 调用日志，便于运行时补齐原始输出。
func (r *aiCallLogRepository) GetLatestByTraceID(ctx context.Context, traceID string) (*model.AICallLog, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return nil, nil
	}

	var log model.AICallLog
	err := r.db.WithContext(ctx).
		Where("trace_id = ?", traceID).
		Order("created_at DESC").
		First(&log).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("按 trace_id 查询 AI 调用日志失败: %w", err)
	}

	return &log, nil
}
