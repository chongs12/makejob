package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/scraper"
)

// loadPipelineIndustryContext 加载题目流水线所需的行业与分类上下文。
func (s *adminService) loadPipelineIndustryContext(ctx context.Context, industryCode string) (*model.Industry, []model.Category, error) {
	industryCode = strings.TrimSpace(industryCode)
	industry, err := s.industryRepo.GetByCode(ctx, industryCode)
	if err != nil {
		return nil, nil, err
	}
	if industry == nil {
		return nil, nil, common.NewBusinessError(common.CodeNotFound, "industry not found")
	}

	categories, err := s.adminCategoryRepo.List(ctx)
	if err != nil {
		return nil, nil, err
	}

	filtered := make([]model.Category, 0)
	for _, category := range categories {
		if category.IndustryID == industry.ID {
			filtered = append(filtered, category)
		}
	}
	if len(filtered) == 0 {
		return nil, nil, common.NewBusinessError(common.CodeBadRequest, "the selected industry has no categories")
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].SortOrder == filtered[j].SortOrder {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].SortOrder < filtered[j].SortOrder
	})

	return industry, filtered, nil
}

// collectQuestionPipelineMaterials 采集题目流水线所需的外部面经素材。
func (s *adminService) collectQuestionPipelineMaterials(ctx context.Context, req *AdminQuestionPipelineGenerateRequest, candidateLimit int) ([]questionPipelineMaterial, int, error) {
	if s.scraperProvider == nil {
		return nil, 0, fmt.Errorf("抓取 Provider 未配置")
	}

	sources := resolveQuestionPipelineSources(s.scraperProvider, req.Sources)
	if len(sources) == 0 {
		return nil, 0, fmt.Errorf("没有可用的抓取来源")
	}

	searchPageSize := 2
	if candidateLimit > 10 {
		searchPageSize = 3
	}

	materials := make([]questionPipelineMaterial, 0)
	searchedCount := 0
	for _, source := range sources {
		if len(materials) >= maxPipelineMaterialSources {
			break
		}

		results, err := s.scraperProvider.Search(ctx, scraper.SearchRequest{
			Keyword:  strings.TrimSpace(req.Requirement),
			Source:   source,
			Page:     1,
			PageSize: searchPageSize,
		})
		if err != nil {
			return materials, searchedCount, fmt.Errorf("搜索来源 %s 失败: %w", source, err)
		}
		searchedCount += len(results)

		for _, result := range results {
			if len(materials) >= maxPipelineMaterialSources {
				break
			}

			fetched, fetchErr := s.scraperProvider.Fetch(ctx, scraper.FetchRequest{
				URL:    result.URL,
				Source: source,
			})
			if fetchErr != nil {
				continue
			}
			if strings.TrimSpace(fetched.Content) == "" {
				continue
			}

			materials = append(materials, questionPipelineMaterial{
				SourceType: "scraped",
				Source:     source,
				Title:      fetched.Title,
				URL:        fetched.URL,
				Content:    fetched.Content,
			})
		}
	}

	if len(materials) == 0 {
		return nil, searchedCount, fmt.Errorf("没有抓取到可用面经素材")
	}

	return materials, searchedCount, nil
}
