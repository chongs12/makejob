package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/data/model"
)

type categoryRepo struct {
	db *gorm.DB
}

func NewCategoryRepo(db *gorm.DB) biz.CategoryRepo {
	return &categoryRepo{db: db}
}

func (r *categoryRepo) ListByIndustry(ctx context.Context, industryCode string) ([]*biz.Category, error) {
	query := r.db.WithContext(ctx).Model(&model.Category{})
	if industryCode != "" {
		query = query.Where("industry_code = ?", industryCode)
	}

	var models []model.Category
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	categories := make([]*biz.Category, len(models))
	for i, m := range models {
		categories[i] = &biz.Category{
			ID:           uint64(m.ID),
			Name:         m.Name,
			ParentID:     m.ParentID,
			IndustryCode: m.IndustryCode,
		}
	}
	return categories, nil
}

// GetByID 按分类 ID 读取分类信息，供管理后台题目写入时补齐行业字段。
func (r *categoryRepo) GetByID(ctx context.Context, id uint64) (*biz.Category, error) {
	var category model.Category
	if err := r.db.WithContext(ctx).First(&category, id).Error; err != nil {
		return nil, err
	}
	return &biz.Category{
		ID:           uint64(category.ID),
		Name:         category.Name,
		ParentID:     category.ParentID,
		IndustryCode: category.IndustryCode,
	}, nil
}
