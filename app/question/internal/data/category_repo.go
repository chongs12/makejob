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

func (r *categoryRepo) ListByIndustry(ctx context.Context, industryID uint64) ([]*biz.Category, error) {
	query := r.db.WithContext(ctx).Model(&model.Category{})
	if industryID > 0 {
		query = query.Where("industry_id = ?", industryID)
	}

	var models []model.Category
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	categories := make([]*biz.Category, len(models))
	for i, m := range models {
		var parentID uint64
		if m.ParentID != nil {
			parentID = uint64(*m.ParentID)
		}
		categories[i] = &biz.Category{
			ID:         uint64(m.ID),
			Name:       m.Name,
			ParentID:   parentID,
			IndustryID: uint64(m.IndustryID),
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
	var parentID uint64
	if category.ParentID != nil {
		parentID = uint64(*category.ParentID)
	}
	return &biz.Category{
		ID:         uint64(category.ID),
		Name:       category.Name,
		ParentID:   parentID,
		IndustryID: uint64(category.IndustryID),
	}, nil
}
