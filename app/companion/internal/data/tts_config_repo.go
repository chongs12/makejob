package data

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"makejob/app/companion/internal/biz"
)

// ttsConfigRepo 实现 biz.TTSConfigRepo 接口
type ttsConfigRepo struct {
	db *gorm.DB
}

// NewTTSConfigRepo 创建 TTS 配置仓库
func NewTTSConfigRepo(db *gorm.DB) biz.TTSConfigRepo {
	return &ttsConfigRepo{db: db}
}

func (r *ttsConfigRepo) GetByID(ctx context.Context, id uint) (*biz.TTSConfig, error) {
	var config biz.TTSConfig
	if err := r.db.WithContext(ctx).First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get tts config by id: %w", err)
	}
	return &config, nil
}

func (r *ttsConfigRepo) List(ctx context.Context) ([]biz.TTSConfig, error) {
	var configs []biz.TTSConfig
	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Order("sort_order ASC").Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("list tts configs: %w", err)
	}
	return configs, nil
}
