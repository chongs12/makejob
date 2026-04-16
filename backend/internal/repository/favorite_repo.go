// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// FavoriteRepository 收藏数据访问接口
type FavoriteRepository interface {
	Create(ctx context.Context, favorite *model.UserFavorite) error
	Delete(ctx context.Context, userID, questionID uint) error
	Exists(ctx context.Context, userID, questionID uint) (bool, error)
	ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]model.UserFavorite, int64, error)
}

// favoriteRepository 收藏数据访问实现
type favoriteRepository struct {
	db *gorm.DB
}

// NewFavoriteRepository 创建收藏仓库实例
func NewFavoriteRepository(db *gorm.DB) FavoriteRepository {
	return &favoriteRepository{
		db: db,
	}
}

// Create 创建收藏
func (r *favoriteRepository) Create(ctx context.Context, favorite *model.UserFavorite) error {
	if err := r.db.WithContext(ctx).Create(favorite).Error; err != nil {
		return fmt.Errorf("创建收藏失败: %w", err)
	}
	return nil
}

// Delete 删除收藏
func (r *favoriteRepository) Delete(ctx context.Context, userID, questionID uint) error {
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND question_id = ?", userID, questionID).
		Delete(&model.UserFavorite{})

	if result.Error != nil {
		return fmt.Errorf("删除收藏失败: %w", result.Error)
	}
	return nil
}

// Exists 检查收藏是否存在
func (r *favoriteRepository) Exists(ctx context.Context, userID, questionID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.UserFavorite{}).
		Where("user_id = ? AND question_id = ?", userID, questionID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("检查收藏是否存在失败: %w", err)
	}
	return count > 0, nil
}

// ListByUser 获取用户的收藏列表
func (r *favoriteRepository) ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]model.UserFavorite, int64, error) {
	var favorites []model.UserFavorite
	var total int64

	// 统计总数
	if err := r.db.WithContext(ctx).Model(&model.UserFavorite{}).
		Where("user_id = ?", userID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计收藏数量失败: %w", err)
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

	// 查询收藏列表，同时预加载题目信息
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Preload("Question").
		Preload("Question.Category").
		Order("created_at DESC").
		Limit(pageSize).Offset(offset).
		Find(&favorites).Error; err != nil {
		return nil, 0, fmt.Errorf("查询收藏列表失败: %w", err)
	}

	return favorites, total, nil
}
