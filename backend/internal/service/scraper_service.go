// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"fmt"
	"strings"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
	"makejob-backend/internal/scraper"
)

// ScraperService 爬虫服务接口
type ScraperService interface {
	Search(ctx context.Context, req scraper.SearchRequest) ([]scraper.SearchResult, error)
	Fetch(ctx context.Context, req scraper.FetchRequest) (*scraper.FetchResult, error)
	Clean(ctx context.Context, req scraper.CleanRequest) (*scraper.CleanResult, error)
	Import(ctx context.Context, req scraper.ImportRequest) (*scraper.ImportResult, error)
	ListTasks(ctx context.Context, page, pageSize int) (*common.PageResult, error)
	GetSources(ctx context.Context) ([]scraper.Source, error)
}

// scraperService 爬虫服务实现
type scraperService struct {
	provider     scraper.ScraperProvider
	cleaner      scraper.QuestionCleaner
	scraperRepo  repository.ScraperTaskRepository
	industryRepo repository.IndustryRepository
	categoryRepo repository.AdminCategoryRepository
	questionRepo repository.AdminQuestionRepository
}

// NewScraperService 创建爬虫服务实例
func NewScraperService(
	provider scraper.ScraperProvider,
	cleaner scraper.QuestionCleaner,
	scraperRepo repository.ScraperTaskRepository,
	industryRepo repository.IndustryRepository,
	categoryRepo repository.AdminCategoryRepository,
	questionRepo repository.AdminQuestionRepository,
) ScraperService {
	return &scraperService{
		provider:     provider,
		cleaner:      cleaner,
		scraperRepo:  scraperRepo,
		industryRepo: industryRepo,
		categoryRepo: categoryRepo,
		questionRepo: questionRepo,
	}
}

// Search 搜索面经
func (s *scraperService) Search(ctx context.Context, req scraper.SearchRequest) ([]scraper.SearchResult, error) {
	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	return s.provider.Search(ctx, req)
}

// Fetch 爬取面经内容
func (s *scraperService) Fetch(ctx context.Context, req scraper.FetchRequest) (*scraper.FetchResult, error) {
	result, err := s.provider.Fetch(ctx, req)
	if err != nil {
		return nil, err
	}

	// 创建爬取任务记录
	task := &model.ScraperTask{
		SourceURL:   req.URL,
		SourceTitle: result.Title,
		Source:      req.Source,
		Status:      scraper.TaskStatusFetched,
		RawContent:  result.Content,
	}

	if err := s.scraperRepo.Create(ctx, task); err != nil {
		return nil, err
	}

	return result, nil
}

// Clean 清洗面经内容
func (s *scraperService) Clean(ctx context.Context, req scraper.CleanRequest) (*scraper.CleanResult, error) {
	return s.cleaner.Clean(ctx, req)
}

// Import 导入题目到题库
func (s *scraperService) Import(ctx context.Context, req scraper.ImportRequest) (*scraper.ImportResult, error) {
	// 查找行业
	industry, err := s.industryRepo.GetByCode(ctx, req.IndustryCode)
	if err != nil {
		return nil, err
	}
	if industry == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "行业不存在")
	}

	// 获取所有分类
	categories, err := s.categoryRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	// 构建分类名称到ID的映射
	categoryMap := make(map[string]uint)
	for _, cat := range categories {
		categoryMap[cat.Name] = cat.ID
		// 也支持小写匹配
		categoryMap[strings.ToLower(cat.Name)] = cat.ID
	}

	result := &scraper.ImportResult{
		TotalCount: len(req.Questions),
		Errors:     make([]string, 0),
	}

	var questionsToImport []model.Question

	for i, q := range req.Questions {
		// 查找分类ID
		categoryID, exists := categoryMap[q.Category]
		if !exists {
			// 尝试模糊匹配
			found := false
			for catName, id := range categoryMap {
				if strings.Contains(strings.ToLower(catName), strings.ToLower(q.Category)) ||
					strings.Contains(strings.ToLower(q.Category), strings.ToLower(catName)) {
					categoryID = id
					found = true
					break
				}
			}
			if !found {
				result.FailCount++
				result.Errors = append(result.Errors, fmt.Sprintf("第%d题: 分类'%s'不存在", i+1, q.Category))
				continue
			}
		}

		// 验证题目类型
		validTypes := map[string]bool{
			model.QuestionTypeChoice:     true,
			model.QuestionTypeMulti:      true,
			model.QuestionTypeCode:       true,
			model.QuestionTypeSubjective: true,
		}
		if !validTypes[q.Type] {
			q.Type = model.QuestionTypeSubjective // 默认主观题
		}

		// 验证难度
		validDifficulties := map[string]bool{
			model.QuestionDifficultyEasy:   true,
			model.QuestionDifficultyMedium: true,
			model.QuestionDifficultyHard:   true,
		}
		if !validDifficulties[q.Difficulty] {
			q.Difficulty = model.QuestionDifficultyMedium // 默认中等
		}

		question := model.Question{
			CategoryID:  categoryID,
			IndustryID:  industry.ID,
			Type:        q.Type,
			Difficulty:  q.Difficulty,
			Title:       q.Title,
			Content:     q.Content,
			Answer:      q.Answer,
			Explanation: q.Explanation,
			Tags:        strings.Join(q.Tags, ","),
			IsActive:    true,
		}

		questionsToImport = append(questionsToImport, question)
	}

	// 批量创建题目
	if len(questionsToImport) > 0 {
		if err := s.questionRepo.BatchCreate(ctx, questionsToImport); err != nil {
			return nil, err
		}
		result.SuccessCount = len(questionsToImport)
	}

	return result, nil
}

// ListTasks 获取爬取任务列表
func (s *scraperService) ListTasks(ctx context.Context, page, pageSize int) (*common.PageResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	tasks, total, err := s.scraperRepo.List(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}

	return &common.PageResult{
		List:     tasks,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetSources 获取支持的数据源列表
func (s *scraperService) GetSources(ctx context.Context) ([]scraper.Source, error) {
	return s.provider.GetSupportedSources(), nil
}
