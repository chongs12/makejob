package service

import (
	"context"
	"testing"

	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

// TestResolveExamIndustryID 验证组卷时会按分类归属推导行业 ID。
func TestResolveExamIndustryID(t *testing.T) {
	t.Parallel()

	categoryID := uint(12)
	svc := &questionService{
		categoryRepo: stubExamCategoryRepo{
			category: &model.Category{
				IndustryID: 9,
			},
		},
	}

	got := svc.resolveExamIndustryID(context.Background(), &categoryID)
	if got == nil {
		t.Fatal("expected industry id, got nil")
	}
	if *got != 9 {
		t.Fatalf("unexpected industry id: got %d want %d", *got, 9)
	}
}

// TestResolveExamIndustryIDWithoutCategory 验证未指定分类时不会强制筛选行业。
func TestResolveExamIndustryIDWithoutCategory(t *testing.T) {
	t.Parallel()

	svc := &questionService{}
	got := svc.resolveExamIndustryID(context.Background(), nil)
	if got != nil {
		t.Fatalf("expected nil industry id, got %v", *got)
	}
}

// stubExamCategoryRepo 为组卷行业推导测试提供最小分类仓库实现。
type stubExamCategoryRepo struct {
	category *model.Category
}

// List 满足分类仓库接口，测试中无需使用。
func (s stubExamCategoryRepo) List(context.Context, uint) ([]model.Category, error) {
	return nil, nil
}

// GetByID 返回测试预置分类。
func (s stubExamCategoryRepo) GetByID(context.Context, uint) (*model.Category, error) {
	return s.category, nil
}

// GetTree 满足分类仓库接口，测试中无需使用。
func (s stubExamCategoryRepo) GetTree(context.Context, uint) ([]repository.CategoryTree, error) {
	return nil, nil
}
