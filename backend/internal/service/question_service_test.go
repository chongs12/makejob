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

	got := svc.resolveExamIndustryID(context.Background(), nil, &categoryID)
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
	got := svc.resolveExamIndustryID(context.Background(), nil, nil)
	if got != nil {
		t.Fatalf("expected nil industry id, got %v", *got)
	}
}

// TestResolveExamIndustryIDWithExplicitIndustry 验证未指定分类时会回退使用显式行业筛选。
func TestResolveExamIndustryIDWithExplicitIndustry(t *testing.T) {
	t.Parallel()

	svc := &questionService{}
	industryID := uint(11)
	got := svc.resolveExamIndustryID(context.Background(), &industryID, nil)
	if got == nil {
		t.Fatal("expected industry id, got nil")
	}
	if *got != 11 {
		t.Fatalf("unexpected industry id: got %d want %d", *got, 11)
	}
}

// TestResolveCategoryIndustryIDByCode 验证分类查询支持按行业编码解析真实行业ID。
func TestResolveCategoryIndustryIDByCode(t *testing.T) {
	t.Parallel()

	svc := &questionService{
		industryRepo: stubQuestionIndustryRepo{
			byCode: map[string]*model.Industry{
				"go": {
					BaseModel: model.BaseModel{ID: 7},
					Code:      "go",
					Name:      "Go",
				},
			},
		},
	}

	got, err := svc.resolveCategoryIndustryID(context.Background(), 0, "go")
	if err != nil {
		t.Fatalf("resolveCategoryIndustryID returned error: %v", err)
	}
	if got != 7 {
		t.Fatalf("expected industry id 7, got %d", got)
	}
}

// TestResolveCategoryIndustryIDWithoutRepo 验证未注入行业仓库时不会强行按行业编码失败。
func TestResolveCategoryIndustryIDWithoutRepo(t *testing.T) {
	t.Parallel()

	svc := &questionService{}
	got, err := svc.resolveCategoryIndustryID(context.Background(), 0, "go")
	if err != nil {
		t.Fatalf("resolveCategoryIndustryID returned error: %v", err)
	}
	if got != 0 {
		t.Fatalf("expected industry id 0, got %d", got)
	}
}

// TestListIndustriesOnlyActive 验证前台行业列表会过滤掉未启用行业。
func TestListIndustriesOnlyActive(t *testing.T) {
	t.Parallel()

	svc := &questionService{
		industryRepo: stubQuestionIndustryRepo{
			list: []model.Industry{
				{
					BaseModel: model.BaseModel{ID: 1},
					Code:      "go",
					Name:      "Go",
					IsActive:  true,
				},
				{
					BaseModel: model.BaseModel{ID: 2},
					Code:      "legacy",
					Name:      "Legacy",
					IsActive:  false,
				},
			},
		},
	}

	got, err := svc.ListIndustries(context.Background())
	if err != nil {
		t.Fatalf("ListIndustries returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 active industry, got %d", len(got))
	}
	if got[0].Code != "go" {
		t.Fatalf("expected active industry go, got %s", got[0].Code)
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

// stubQuestionIndustryRepo 为分类行业解析测试提供最小行业仓库实现。
type stubQuestionIndustryRepo struct {
	list   []model.Industry
	byCode map[string]*model.Industry
}

// List 满足行业仓库接口，测试中无需使用。
func (s stubQuestionIndustryRepo) List(context.Context) ([]model.Industry, error) {
	return append([]model.Industry(nil), s.list...), nil
}

// GetByID 满足行业仓库接口，测试中无需使用。
func (s stubQuestionIndustryRepo) GetByID(context.Context, uint) (*model.Industry, error) {
	return nil, nil
}

// Create 满足行业仓库接口，测试中无需使用。
func (s stubQuestionIndustryRepo) Create(context.Context, *model.Industry) error {
	return nil
}

// Update 满足行业仓库接口，测试中无需使用。
func (s stubQuestionIndustryRepo) Update(context.Context, *model.Industry) error {
	return nil
}

// GetByCode 返回测试预置行业。
func (s stubQuestionIndustryRepo) GetByCode(_ context.Context, code string) (*model.Industry, error) {
	return s.byCode[code], nil
}
