// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"makejob-backend/internal/model"
)

// StudyLogRepository 学习日志数据访问接口。
type StudyLogRepository interface {
	UpsertDaily(ctx context.Context, log *model.StudyLog) error
	ListRecentByUser(ctx context.Context, userID uint, limit int) ([]model.StudyLog, error)
	CountStudyDays(ctx context.Context, userID uint) (int64, error)
}

// studyLogRepository 学习日志数据访问实现。
type studyLogRepository struct {
	db *gorm.DB
}

// NewStudyLogRepository 创建学习日志仓库实例。
func NewStudyLogRepository(db *gorm.DB) StudyLogRepository {
	return &studyLogRepository{
		db: db,
	}
}

// UpsertDaily 按用户和日期写入学习日志，已存在则更新当日摘要。
func (r *studyLogRepository) UpsertDaily(ctx context.Context, log *model.StudyLog) error {
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "log_date"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"plan_id",
				"summary",
				"focus_task_title",
				"completed_count",
				"skipped_count",
				"completed_titles_json",
				"skipped_titles_json",
				"latest_action_text",
				"updated_at",
			}),
		}).
		Create(log).Error; err != nil {
		return fmt.Errorf("写入学习日志失败: %w", err)
	}
	return nil
}

// ListRecentByUser 按日期倒序读取最近的学习日志列表。
func (r *studyLogRepository) ListRecentByUser(ctx context.Context, userID uint, limit int) ([]model.StudyLog, error) {
	if limit <= 0 {
		limit = 7
	}

	var logs []model.StudyLog
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("log_date DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("查询学习日志失败: %w", err)
	}
	return logs, nil
}

// CountStudyDays 统计用户累计留下学习日志的天数。
func (r *studyLogRepository) CountStudyDays(ctx context.Context, userID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.StudyLog{}).
		Where("user_id = ?", userID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计学习日志天数失败: %w", err)
	}
	return count, nil
}
