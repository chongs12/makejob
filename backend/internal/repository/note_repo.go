// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// NoteRepository 笔记数据访问接口
type NoteRepository interface {
	Create(ctx context.Context, note *model.UserNote) error
	GetByID(ctx context.Context, id uint) (*model.UserNote, error)
	Update(ctx context.Context, note *model.UserNote) error
	Delete(ctx context.Context, id, userID uint) error
	ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]model.UserNote, int64, error)
	GetByUserAndQuestion(ctx context.Context, userID, questionID uint) (*model.UserNote, error)
}

// noteRepository 笔记数据访问实现
type noteRepository struct {
	db *gorm.DB
}

// NewNoteRepository 创建笔记仓库实例
func NewNoteRepository(db *gorm.DB) NoteRepository {
	return &noteRepository{
		db: db,
	}
}

// Create 创建笔记
func (r *noteRepository) Create(ctx context.Context, note *model.UserNote) error {
	if err := r.db.WithContext(ctx).Create(note).Error; err != nil {
		return fmt.Errorf("创建笔记失败: %w", err)
	}
	return nil
}

// GetByID 根据ID获取笔记
func (r *noteRepository) GetByID(ctx context.Context, id uint) (*model.UserNote, error) {
	var note model.UserNote
	if err := r.db.WithContext(ctx).First(&note, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询笔记失败: %w", err)
	}
	return &note, nil
}

// Update 更新笔记
func (r *noteRepository) Update(ctx context.Context, note *model.UserNote) error {
	if err := r.db.WithContext(ctx).Save(note).Error; err != nil {
		return fmt.Errorf("更新笔记失败: %w", err)
	}
	return nil
}

// Delete 删除笔记
func (r *noteRepository) Delete(ctx context.Context, id, userID uint) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.UserNote{})

	if result.Error != nil {
		return fmt.Errorf("删除笔记失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("笔记不存在或无权限删除")
	}
	return nil
}

// ListByUser 获取用户的笔记列表
func (r *noteRepository) ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]model.UserNote, int64, error) {
	var notes []model.UserNote
	var total int64

	// 统计总数
	if err := r.db.WithContext(ctx).Model(&model.UserNote{}).
		Where("user_id = ?", userID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计笔记数量失败: %w", err)
	}

	// 分页查询
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// 查询笔记列表，同时预加载题目信息
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Preload("Question").
		Order("created_at DESC").
		Limit(pageSize).Offset(offset).
		Find(&notes).Error; err != nil {
		return nil, 0, fmt.Errorf("查询笔记列表失败: %w", err)
	}

	return notes, total, nil
}

// GetByUserAndQuestion 获取用户对某题的笔记
func (r *noteRepository) GetByUserAndQuestion(ctx context.Context, userID, questionID uint) (*model.UserNote, error) {
	var note model.UserNote
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND question_id = ?", userID, questionID).
		First(&note).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询笔记失败: %w", err)
	}
	return &note, nil
}
