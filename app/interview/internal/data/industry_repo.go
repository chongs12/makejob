package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/interview/internal/biz"
)

// industryModel 行业表 GORM model（与 question 服务共用同一张表）
type industryModel struct {
	Code string `gorm:"primaryKey;size:50"`
	Name string `gorm:"size:200;not null"`
}

func (industryModel) TableName() string { return "industries" }

// industryRepo 实现 biz.IndustryClient 接口，直接查询本地数据库
type industryRepo struct {
	db *gorm.DB
}

// NewIndustryRepo 创建本地行业仓储
func NewIndustryRepo(db *gorm.DB) biz.IndustryClient {
	return &industryRepo{db: db}
}

// GetIndustry 按行业编码查询行业信息
func (r *industryRepo) GetIndustry(ctx context.Context, code string) (*biz.Industry, error) {
	var m industryModel
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&m).Error; err != nil {
		return nil, err
	}
	return &biz.Industry{
		Code: m.Code,
		Name: m.Name,
	}, nil
}
