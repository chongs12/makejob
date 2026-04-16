// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// InterviewRepository 面试数据访问接口
type InterviewRepository interface {
	Create(ctx context.Context, interview *model.MockInterview) error
	GetByID(ctx context.Context, id uint) (*model.MockInterview, error)
	Update(ctx context.Context, interview *model.MockInterview) error
	ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]model.MockInterview, int64, error)
	GetUserDailyCount(ctx context.Context, userID uint, date time.Time) (int64, error)
}

// InterviewMessageRepository 面试消息数据访问接口
type InterviewMessageRepository interface {
	Create(ctx context.Context, msg *model.InterviewMessage) error
	ListByInterview(ctx context.Context, interviewID uint) ([]model.InterviewMessage, error)
	CountByInterview(ctx context.Context, interviewID uint) (int64, error)
}

// interviewRepository 面试数据访问实现
type interviewRepository struct {
	db *gorm.DB
}

// interviewMessageRepository 面试消息数据访问实现
type interviewMessageRepository struct {
	db *gorm.DB
}

// NewInterviewRepository 创建面试仓库实例
func NewInterviewRepository(db *gorm.DB) InterviewRepository {
	return &interviewRepository{
		db: db,
	}
}

// NewInterviewMessageRepository 创建面试消息仓库实例
func NewInterviewMessageRepository(db *gorm.DB) InterviewMessageRepository {
	return &interviewMessageRepository{
		db: db,
	}
}

// Create 创建面试记录
func (r *interviewRepository) Create(ctx context.Context, interview *model.MockInterview) error {
	if err := r.db.WithContext(ctx).Create(interview).Error; err != nil {
		return fmt.Errorf("创建面试记录失败: %w", err)
	}
	return nil
}

// GetByID 根据ID获取面试记录
func (r *interviewRepository) GetByID(ctx context.Context, id uint) (*model.MockInterview, error) {
	var interview model.MockInterview
	if err := r.db.WithContext(ctx).Preload("Messages").First(&interview, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询面试记录失败: %w", err)
	}
	return &interview, nil
}

// Update 更新面试记录
func (r *interviewRepository) Update(ctx context.Context, interview *model.MockInterview) error {
	if err := r.db.WithContext(ctx).Save(interview).Error; err != nil {
		return fmt.Errorf("更新面试记录失败: %w", err)
	}
	return nil
}

// ListByUser 获取用户的面试列表
func (r *interviewRepository) ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]model.MockInterview, int64, error) {
	var interviews []model.MockInterview
	var total int64

	// 查询总数
	if err := r.db.WithContext(ctx).Model(&model.MockInterview{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("查询面试总数失败: %w", err)
	}

	// 查询列表
	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&interviews).Error; err != nil {
		return nil, 0, fmt.Errorf("查询面试列表失败: %w", err)
	}

	return interviews, total, nil
}

// GetUserDailyCount 获取用户当日面试次数
func (r *interviewRepository) GetUserDailyCount(ctx context.Context, userID uint, date time.Time) (int64, error) {
	var count int64
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	if err := r.db.WithContext(ctx).Model(&model.MockInterview{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, startOfDay, endOfDay).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("查询用户当日面试次数失败: %w", err)
	}
	return count, nil
}

// Create 创建面试消息
func (r *interviewMessageRepository) Create(ctx context.Context, msg *model.InterviewMessage) error {
	if err := r.db.WithContext(ctx).Create(msg).Error; err != nil {
		return fmt.Errorf("创建面试消息失败: %w", err)
	}
	return nil
}

// ListByInterview 获取面试的消息列表
func (r *interviewMessageRepository) ListByInterview(ctx context.Context, interviewID uint) ([]model.InterviewMessage, error) {
	var messages []model.InterviewMessage
	if err := r.db.WithContext(ctx).
		Where("interview_id = ?", interviewID).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("查询面试消息列表失败: %w", err)
	}
	return messages, nil
}

// CountByInterview 获取面试的消息数量
func (r *interviewMessageRepository) CountByInterview(ctx context.Context, interviewID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.InterviewMessage{}).
		Where("interview_id = ?", interviewID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("查询面试消息数量失败: %w", err)
	}
	return count, nil
}
