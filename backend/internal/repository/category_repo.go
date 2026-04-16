// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// CategoryTree 分类树结构
type CategoryTree struct {
	model.Category
	Children []CategoryTree `json:"children"`
}

// CategoryRepository 分类数据访问接口
type CategoryRepository interface {
	List(ctx context.Context, industryID uint) ([]model.Category, error)
	GetByID(ctx context.Context, id uint) (*model.Category, error)
	GetTree(ctx context.Context, industryID uint) ([]CategoryTree, error)
}

// categoryRepository 分类数据访问实现
type categoryRepository struct {
	db *gorm.DB
}

// NewCategoryRepository 创建分类仓库实例
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{
		db: db,
	}
}

// List 获取分类列表
func (r *categoryRepository) List(ctx context.Context, industryID uint) ([]model.Category, error) {
	var categories []model.Category
	query := r.db.WithContext(ctx).Model(&model.Category{})

	if industryID > 0 {
		query = query.Where("industry_id = ?", industryID)
	}

	if err := query.Order("sort_order ASC, id ASC").Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("查询分类列表失败: %w", err)
	}

	return categories, nil
}

// GetByID 根据ID获取分类
func (r *categoryRepository) GetByID(ctx context.Context, id uint) (*model.Category, error) {
	var category model.Category
	if err := r.db.WithContext(ctx).First(&category, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询分类失败: %w", err)
	}
	return &category, nil
}

// GetTree 获取分类树
func (r *categoryRepository) GetTree(ctx context.Context, industryID uint) ([]CategoryTree, error) {
	// 获取所有分类
	allCategories, err := r.List(ctx, industryID)
	if err != nil {
		return nil, err
	}

	// 构建分类树
	return buildCategoryTree(allCategories), nil
}

// buildCategoryTree 构建分类树
func buildCategoryTree(categories []model.Category) []CategoryTree {
	// 创建分类映射
	categoryMap := make(map[uint]*CategoryTree)
	for i := range categories {
		cat := &categories[i]
		categoryMap[cat.ID] = &CategoryTree{
			Category: *cat,
			Children: []CategoryTree{},
		}
	}

	// 构建树形结构
	var roots []CategoryTree
	for _, cat := range categories {
		treeNode := categoryMap[cat.ID]
		if cat.ParentID == nil {
			// 顶级分类
			roots = append(roots, *treeNode)
		} else {
			// 子分类
			if parent, exists := categoryMap[*cat.ParentID]; exists {
				parent.Children = append(parent.Children, *treeNode)
			}
		}
	}

	return roots
}
