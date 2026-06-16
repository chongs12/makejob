package data

import (
	"context"

	"gorm.io/gorm"
)

// industryModel 行业表 GORM model（与 question/interview 服务共用同一张表）
type industryModel struct {
	ID   uint64 `gorm:"column:id;primaryKey"`
	Code string `gorm:"size:50"`
}

func (industryModel) TableName() string { return "industries" }

// IndustryRepo 行业仓储接口，用于 code→id 解析
type IndustryRepo interface {
	GetIDByCode(ctx context.Context, code string) (uint64, error)
}

type industryRepo struct {
	db *gorm.DB
}

// NewIndustryRepo 创建行业仓储实现
func NewIndustryRepo(db *gorm.DB) IndustryRepo {
	return &industryRepo{db: db}
}

// GetIDByCode 按行业编码查询行业 ID
func (r *industryRepo) GetIDByCode(ctx context.Context, code string) (uint64, error) {
	var m industryModel
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&m).Error; err != nil {
		return 0, err
	}
	return m.ID, nil
}
