package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/ai_gateway/internal/biz"
)

type promptRepo struct {
	db *gorm.DB
}

// NewPromptRepo 创建 Prompt 模板仓库实现
func NewPromptRepo(db *gorm.DB) biz.PromptRepo {
	return &promptRepo{db: db}
}

// GetActiveTemplate 查询指定场景下最新版本的生效 Prompt 模板
func (r *promptRepo) GetActiveTemplate(ctx context.Context, scene string) (*biz.PromptTemplate, error) {
	var tpl biz.PromptTemplate
	err := r.db.WithContext(ctx).
		Where("scene = ? AND is_active = ?", scene, true).
		Order("version DESC").
		First(&tpl).Error
	if err != nil {
		return nil, err
	}
	return &tpl, nil
}
