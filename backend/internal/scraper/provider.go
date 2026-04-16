// Package scraper 提供面经爬取与清洗导入功能
package scraper

import "context"

// ScraperProvider 爬虫提供者接口
type ScraperProvider interface {
	// Search 搜索面经
	Search(ctx context.Context, req SearchRequest) ([]SearchResult, error)
	// Fetch 爬取面经内容
	Fetch(ctx context.Context, req FetchRequest) (*FetchResult, error)
	// GetSupportedSources 获取支持的数据源列表
	GetSupportedSources() []Source
}

// QuestionCleaner 题目清洗器接口
type QuestionCleaner interface {
	// Clean 清洗面经内容，提取结构化题目
	Clean(ctx context.Context, req CleanRequest) (*CleanResult, error)
}
