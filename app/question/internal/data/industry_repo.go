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
	if err := r.db.WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}

	industries := make([]*biz.Industry, len(models))
	for i, m := range models {
		industries[i] = &biz.Industry{
			Code: m.Code,
			Name: m.Name,
			Icon: m.Icon,
		}
	}
	return industries, nil
}
