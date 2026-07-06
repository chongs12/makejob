package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/companion/internal/biz"
)

// asrConfigRepo ASR 配置仓库 GORM 实现。
type asrConfigRepo struct {
	db *gorm.DB
}

// NewASRConfigRepo 创建 ASR 配置仓库。
func NewASRConfigRepo(db *gorm.DB) biz.ASRConfigRepo {
	return &asrConfigRepo{db: db}
}

func (r *asrConfigRepo) GetByID(ctx context.Context, id uint) (*biz.ASRConfig, error) {
	var config biz.ASRConfig
	if err := r.db.WithContext(ctx).First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

func (r *asrConfigRepo) List(ctx context.Context) ([]biz.ASRConfig, error) {
	var configs []biz.ASRConfig
	if err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("sort_order ASC, id ASC").
		Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

func (r *asrConfigRepo) Create(ctx context.Context, config *biz.ASRConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *asrConfigRepo) Update(ctx context.Context, config *biz.ASRConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

func (r *asrConfigRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&biz.ASRConfig{}, id).Error
}
