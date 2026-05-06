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

// TestGetPracticeRecommendationsEnrichesQuestionSetHint 验证练习推荐会补齐专题、题单和推荐模式信息。
func TestGetPracticeRecommendationsEnrichesQuestionSetHint(t *testing.T) {
	t.Parallel()

	svc := &questionService{
		questionRepo: stubPracticeRecommendationQuestionRepo{
			questions: []model.Question{
				{
					BaseModel: model.BaseModel{ID: 101},
					Title:     "状态机动态规划专项",
					Content:   "请实现一个动态规划状态转移题",
					Type:      model.QuestionTypeCode,
					IsActive:  true,
				},
			},
		},
		learningArchiveRepo: stubPracticeRecommendationArchiveRepo{
			entries: []model.LearningArchiveEntry{
				{
					SourceRef:       "practice:7:101",
					MistakeTagsJSON: `["状态定义不清","状态定义不清"]`,
				},
				{
					SourceRef:       "practice:7:102",
					MistakeTagsJSON: `["状态定义不清"]`,
				},
			},
		},
	}

	resp, err := svc.GetPracticeRecommendations(context.Background(), 7, nil, 3)
	if err != nil {
		t.Fatalf("GetPracticeRecommendations returned error: %v", err)
	}
	if len(resp.FocusTags) == 0 || resp.FocusTags[0] != "状态定义不清" {
		t.Fatalf("expected first focus tag 状态定义不清, got %#v", resp.FocusTags)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 recommendation item, got %d", len(resp.Items))
	}
	item := resp.Items[0]
	if item.RecommendationMode != "question_set" {
		t.Fatalf("expected recommendation mode question_set, got %s", item.RecommendationMode)
	}
	if item.TopicCode != "state-definition" || item.TopicTitle == "" {
		t.Fatalf("expected mapped topic info, got %#v", item)
	}
	if item.PrimaryQuestionSet == "" || len(item.RelatedQuestionSets) == 0 {
		t.Fatalf("expected related question set hints, got %#v", item)
	}
	if item.PriorityExplanation == "" || len(item.RecommendedActions) == 0 {
		t.Fatalf("expected enriched explanation and actions, got %#v", item)
	}
}

// TestGetPracticeRecommendationsFallsBackToKeywordMode 验证未知错因标签仍会回退为关键词推荐模式。
func TestGetPracticeRecommendationsFallsBackToKeywordMode(t *testing.T) {
	t.Parallel()

	interviewID := uint(9)
	svc := &questionService{
		questionRepo: stubPracticeRecommendationQuestionRepo{
			questions: []model.Question{
				{
					BaseModel: model.BaseModel{ID: 202},
					Title:     "自定义标签补练题",
					Content:   "围绕一次陌生错因做补练",
					Type:      model.QuestionTypeCode,
					IsActive:  true,
				},
			},
		},
		learningArchiveRepo: stubPracticeRecommendationArchiveRepo{
			entries: []model.LearningArchiveEntry{
				{
					InterviewID:     interviewID,
					SourceRef:       "interview:9",
					MistakeTagsJSON: `["陌生自定义标签"]`,
				},
			},
		},
	}

	resp, err := svc.GetPracticeRecommendations(context.Background(), 7, &interviewID, 2)
	if err != nil {
		t.Fatalf("GetPracticeRecommendations returned error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 recommendation item, got %d", len(resp.Items))
	}
	item := resp.Items[0]
	if item.RecommendationMode != "keyword" {
		t.Fatalf("expected keyword recommendation mode, got %s", item.RecommendationMode)
	}
	if item.SourceType != "interview_archive" {
		t.Fatalf("expected source type interview_archive, got %s", item.SourceType)
	}
	if item.TopicCode != "" || item.TopicTitle != "" || item.PrimaryQuestionSet != "" {
		t.Fatalf("expected no mapped topic info, got %#v", item)
	}
	if item.PriorityExplanation == "" {
		t.Fatalf("expected fallback priority explanation, got %#v", item)
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

// stubPracticeRecommendationQuestionRepo 为练习推荐测试提供最小题库仓库实现。
type stubPracticeRecommendationQuestionRepo struct {
	questions []model.Question
}

// List 返回预置题目列表，供推荐接口按关键词检索时复用。
func (s stubPracticeRecommendationQuestionRepo) List(context.Context, repository.QuestionListParams) ([]model.Question, int64, error) {
	return append([]model.Question(nil), s.questions...), int64(len(s.questions)), nil
}

// GetByID 满足题库仓库接口，当前测试不依赖该行为。
func (s stubPracticeRecommendationQuestionRepo) GetByID(context.Context, uint) (*model.Question, error) {
	return nil, nil
}

// GetByIDs 满足题库仓库接口，当前测试不依赖该行为。
func (s stubPracticeRecommendationQuestionRepo) GetByIDs(context.Context, []uint) ([]model.Question, error) {
	return nil, nil
}

// GetRandomByParams 满足题库仓库接口，当前测试不依赖该行为。
func (s stubPracticeRecommendationQuestionRepo) GetRandomByParams(context.Context, repository.RandomQuestionParams) ([]model.Question, error) {
	return nil, nil
}

// stubPracticeRecommendationArchiveRepo 为练习推荐测试提供学习档案数据。
type stubPracticeRecommendationArchiveRepo struct {
	entries []model.LearningArchiveEntry
}

// Upsert 满足学习档案仓库接口，当前测试不依赖写入行为。
func (s stubPracticeRecommendationArchiveRepo) Upsert(context.Context, *model.LearningArchiveEntry) error {
	return nil
}

// ListRecentByUser 返回预置学习档案列表。
func (s stubPracticeRecommendationArchiveRepo) ListRecentByUser(_ context.Context, _ uint, _ int, interviewID *uint) ([]model.LearningArchiveEntry, error) {
	if interviewID == nil || *interviewID == 0 {
		return append([]model.LearningArchiveEntry(nil), s.entries...), nil
	}

	filtered := make([]model.LearningArchiveEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.InterviewID == *interviewID {
			filtered = append(filtered, entry)
		}
	}
	return filtered, nil
}
