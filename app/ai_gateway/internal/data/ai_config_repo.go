package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/ai_gateway/internal/biz"
)

type aiConfigRepo struct {
	db *gorm.DB
}

// NewAIConfigRepo 创建 AI 配置仓库实现
func NewAIConfigRepo(db *gorm.DB) biz.AIConfigRepo {
	return &aiConfigRepo{db: db}
}

// GetActiveConfig 查询指定场景下当前生效的 AI 配置
func (r *aiConfigRepo) GetActiveConfig(ctx context.Context, scene string) (*biz.AIConfig, error) {
	var cfg biz.AIConfig
	err := r.db.WithContext(ctx).
		Where("scene = ? AND is_active = ?", scene, true).
		First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
