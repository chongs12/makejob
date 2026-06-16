package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/data/model"
)

type industryRepo struct {
	db *gorm.DB
}

func NewIndustryRepo(db *gorm.DB) biz.IndustryRepo {
	return &industryRepo{db: db}
}

func (r *industryRepo) List(ctx context.Context) ([]*biz.Industry, error) {
	var models []model.Industry
	// 对齐单体：只返回启用的行业
	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&models).Error; err != nil {
		return nil, err
	}

	industries := make([]*biz.Industry, len(models))
	for i, m := range models {
		industries[i] = &biz.Industry{
			ID:   uint64(m.ID),
			Code: m.Code,
			Name: m.Name,
			Icon: m.Icon,
		}
	}
	return industries, nil
}

func (r *industryRepo) GetByCode(ctx context.Context, code string) (*biz.Industry, error) {
	var m model.Industry
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&m).Error; err != nil {
		return nil, err
	}
	return toBizIndustry(&m), nil
}

func (r *industryRepo) GetByID(ctx context.Context, id uint64) (*biz.Industry, error) {
	var m model.Industry
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toBizIndustry(&m), nil
}

func toBizIndustry(m *model.Industry) *biz.Industry {
	return &biz.Industry{
		ID:   uint64(m.ID),
		Code: m.Code,
		Name: m.Name,
		Icon: m.Icon,
	}
}
