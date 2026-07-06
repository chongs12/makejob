package data

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"makejob/app/companion/internal/biz"
)

// adminConfigRepo 实现 biz.AdminConfigRepo 接口
type adminConfigRepo struct {
	db *gorm.DB
}

// NewAdminConfigRepo 创建管理后台配置仓库
func NewAdminConfigRepo(db *gorm.DB) biz.AdminConfigRepo {
	return &adminConfigRepo{db: db}
}

func (r *adminConfigRepo) GetByKey(ctx context.Context, key string) (*biz.AdminConfig, error) {
	var config biz.AdminConfig
	if err := r.db.WithContext(ctx).Where("config_key = ?", key).First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get admin config by key: %w", err)
	}
	return &config, nil
}

// Upsert 创建或更新管理后台配置。
func (r *adminConfigRepo) Upsert(ctx context.Context, config *biz.AdminConfig) error {
	if config == nil || config.ConfigKey == "" {
		return fmt.Errorf("invalid admin config")
	}
	return r.db.WithContext(ctx).
		Where("config_key = ?", config.ConfigKey).
		Assign(map[string]interface{}{"config_value": config.ConfigValue}).
		FirstOrCreate(config).Error
}
