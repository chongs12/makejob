// Package repository 提供数据访问层实现。
package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// AIPresetRepository AI 预设管理仓库接口。
type AIPresetRepository interface {
	List(ctx context.Context) ([]model.AIPreset, error)
	GetByID(ctx context.Context, id uint) (*model.AIPreset, error)
	GetByName(ctx context.Context, name string) (*model.AIPreset, error)
	GetActive(ctx context.Context) (*model.AIPreset, error)
	Create(ctx context.Context, preset *model.AIPreset) error
	Update(ctx context.Context, preset *model.AIPreset) error
	Delete(ctx context.Context, id uint) error
	SetActive(ctx context.Context, id uint) error
	ClearActive(ctx context.Context) error
}

// aiPresetRepository AI 预设管理仓库实现。
type aiPresetRepository struct {
	db *gorm.DB
}

// NewAIPresetRepository 创建 AI 预设管理仓库实例。
func NewAIPresetRepository(db *gorm.DB) AIPresetRepository {
	return &aiPresetRepository{db: db}
}

// List 获取 AI 预设列表。
func (r *aiPresetRepository) List(ctx context.Context) ([]model.AIPreset, error) {
	var presets []model.AIPreset
	if err := r.db.WithContext(ctx).Order("is_active DESC, updated_at DESC, id DESC").Find(&presets).Error; err != nil {
		return nil, fmt.Errorf("查询 AI 预设列表失败: %w", err)
	}
	return presets, nil
}

// GetByID 根据 ID 获取 AI 预设。
func (r *aiPresetRepository) GetByID(ctx context.Context, id uint) (*model.AIPreset, error) {
	var preset model.AIPreset
	if err := r.db.WithContext(ctx).First(&preset, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询 AI 预设失败: %w", err)
	}
	return &preset, nil
}

// GetByName 根据名称获取 AI 预设。
func (r *aiPresetRepository) GetByName(ctx context.Context, name string) (*model.AIPreset, error) {
	var preset model.AIPreset
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&preset).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("根据名称查询 AI 预设失败: %w", err)
	}
	return &preset, nil
}

// GetActive 获取当前生效的 AI 预设。
func (r *aiPresetRepository) GetActive(ctx context.Context) (*model.AIPreset, error) {
	var preset model.AIPreset
	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Order("updated_at DESC, id DESC").First(&preset).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询当前生效 AI 预设失败: %w", err)
	}
	return &preset, nil
}

// Create 创建 AI 预设。
func (r *aiPresetRepository) Create(ctx context.Context, preset *model.AIPreset) error {
	if err := r.db.WithContext(ctx).Create(preset).Error; err != nil {
		return fmt.Errorf("创建 AI 预设失败: %w", err)
	}
	return nil
}

// Update 更新 AI 预设。
func (r *aiPresetRepository) Update(ctx context.Context, preset *model.AIPreset) error {
	if err := r.db.WithContext(ctx).Save(preset).Error; err != nil {
		return fmt.Errorf("更新 AI 预设失败: %w", err)
	}
	return nil
}

// Delete 删除 AI 预设。
func (r *aiPresetRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&model.AIPreset{}, id)
	if result.Error != nil {
		return fmt.Errorf("删除 AI 预设失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("AI 预设不存在")
	}
	return nil
}

// SetActive 将指定预设标记为当前唯一生效预设。
func (r *aiPresetRepository) SetActive(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.AIPreset{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return fmt.Errorf("清空当前 AI 生效预设失败: %w", err)
		}

		result := tx.Model(&model.AIPreset{}).Where("id = ?", id).Update("is_active", true)
		if result.Error != nil {
			return fmt.Errorf("设置 AI 生效预设失败: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("AI 预设不存在")
		}
		return nil
	})
}

// ClearActive 清空当前生效的 AI 预设标记。
func (r *aiPresetRepository) ClearActive(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Model(&model.AIPreset{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
		return fmt.Errorf("清空 AI 生效预设失败: %w", err)
	}
	return nil
}
