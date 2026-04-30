// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// InterviewCodingAttemptRepository 定义编程题作答与过程事件的数据访问接口。
type InterviewCodingAttemptRepository interface {
	UpsertAttempt(ctx context.Context, attempt *model.InterviewCodingAttempt) error
	ReplaceEvents(ctx context.Context, attemptID uint, events []model.InterviewCodingEvent) error
	ListByInterview(ctx context.Context, interviewID uint) ([]model.InterviewCodingAttempt, error)
}

// LearningArchiveRepository 定义学习档案条目的数据访问接口。
type LearningArchiveRepository interface {
	Upsert(ctx context.Context, entry *model.LearningArchiveEntry) error
	ListRecentByUser(ctx context.Context, userID uint, limit int, interviewID *uint) ([]model.LearningArchiveEntry, error)
}

// interviewCodingAttemptRepository 实现编程题作答仓库。
type interviewCodingAttemptRepository struct {
	db *gorm.DB
}

// learningArchiveRepository 实现学习档案仓库。
type learningArchiveRepository struct {
	db *gorm.DB
}

// NewInterviewCodingAttemptRepository 创建编程题作答仓库实例。
func NewInterviewCodingAttemptRepository(db *gorm.DB) InterviewCodingAttemptRepository {
	return &interviewCodingAttemptRepository{db: db}
}

// NewLearningArchiveRepository 创建学习档案仓库实例。
func NewLearningArchiveRepository(db *gorm.DB) LearningArchiveRepository {
	return &learningArchiveRepository{db: db}
}

// UpsertAttempt 按面试与题目序号创建或更新单道编程题作答记录。
func (r *interviewCodingAttemptRepository) UpsertAttempt(ctx context.Context, attempt *model.InterviewCodingAttempt) error {
	if attempt == nil {
		return fmt.Errorf("编程题作答记录不能为空")
	}

	var existing model.InterviewCodingAttempt
	err := r.db.WithContext(ctx).
		Where("interview_id = ? AND question_index = ?", attempt.InterviewID, attempt.QuestionIndex).
		First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("查询编程题作答记录失败: %w", err)
	}

	if err == gorm.ErrRecordNotFound {
		if err := r.db.WithContext(ctx).Create(attempt).Error; err != nil {
			return fmt.Errorf("创建编程题作答记录失败: %w", err)
		}
		return nil
	}

	attempt.ID = existing.ID
	attempt.CreatedAt = existing.CreatedAt
	attempt.UpdatedAt = existing.UpdatedAt
	if err := r.db.WithContext(ctx).Save(attempt).Error; err != nil {
		return fmt.Errorf("更新编程题作答记录失败: %w", err)
	}
	return nil
}

// ReplaceEvents 用最新事件序列覆盖单道编程题的历史过程事件。
func (r *interviewCodingAttemptRepository) ReplaceEvents(ctx context.Context, attemptID uint, events []model.InterviewCodingEvent) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("attempt_id = ?", attemptID).Delete(&model.InterviewCodingEvent{}).Error; err != nil {
			return fmt.Errorf("删除旧编程题过程事件失败: %w", err)
		}
		if len(events) == 0 {
			return nil
		}
		if err := tx.Create(&events).Error; err != nil {
			return fmt.Errorf("创建编程题过程事件失败: %w", err)
		}
		return nil
	})
}

// ListByInterview 获取某场面试下的所有编程题作答及过程事件。
func (r *interviewCodingAttemptRepository) ListByInterview(ctx context.Context, interviewID uint) ([]model.InterviewCodingAttempt, error) {
	var attempts []model.InterviewCodingAttempt
	if err := r.db.WithContext(ctx).
		Preload("Events", func(db *gorm.DB) *gorm.DB {
			return db.Order("sequence ASC")
		}).
		Where("interview_id = ?", interviewID).
		Order("question_index ASC").
		Find(&attempts).Error; err != nil {
		return nil, fmt.Errorf("查询编程题作答记录失败: %w", err)
	}
	return attempts, nil
}

// Upsert 按用户与来源唯一标识创建或更新学习档案条目。
func (r *learningArchiveRepository) Upsert(ctx context.Context, entry *model.LearningArchiveEntry) error {
	if entry == nil {
		return fmt.Errorf("学习档案条目不能为空")
	}

	var existing model.LearningArchiveEntry
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND source_ref = ?", entry.UserID, entry.SourceRef).
		First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("查询学习档案条目失败: %w", err)
	}

	if err == gorm.ErrRecordNotFound {
		if err := r.db.WithContext(ctx).Create(entry).Error; err != nil {
			return fmt.Errorf("创建学习档案条目失败: %w", err)
		}
		return nil
	}

	entry.ID = existing.ID
	entry.CreatedAt = existing.CreatedAt
	entry.UpdatedAt = existing.UpdatedAt
	if err := r.db.WithContext(ctx).Save(entry).Error; err != nil {
		return fmt.Errorf("更新学习档案条目失败: %w", err)
	}
	return nil
}

// ListRecentByUser 获取用户最近的学习档案条目，可按面试过滤。
func (r *learningArchiveRepository) ListRecentByUser(ctx context.Context, userID uint, limit int, interviewID *uint) ([]model.LearningArchiveEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("COALESCE(occurred_at, updated_at) DESC").
		Limit(limit)
	if interviewID != nil && *interviewID > 0 {
		query = query.Where("interview_id = ?", *interviewID)
	}

	var entries []model.LearningArchiveEntry
	if err := query.Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("查询学习档案条目失败: %w", err)
	}
	return entries, nil
}
