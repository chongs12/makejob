package bridge

// QuestionPipelineGenerateRequest 表示题目流水线生成请求。
type QuestionPipelineGenerateRequest struct {
	IndustryCode     string
	Requirement      string
	AgentPrompt      string
	GenerationMode   string
	CandidateCount   int32
	IncludeScraped   bool
	IncludeGenerated bool
	Sources          []string
}

// QuestionPipelineGenerateResponse 表示题目流水线生成结果。
type QuestionPipelineGenerateResponse struct {
	IndustryCode   string
	Requirement    string
	GenerationMode string
	Warnings       []string
	Stats          map[string]any
	Cards          []QuestionPipelineCard
}

// QuestionPipelineCard 表示流水线返回的候选题卡。
type QuestionPipelineCard struct {
	ID          string
	Title       string
	Content     string
	Type        string
	Difficulty  string
	Category    string
	Answer      string
	Solution    string
	Explanation string
	Tags        []string
	JudgeConfig string
	Confidence  float64
	SourceType  string
	SourceLabel string
	SourceTitle string
	SourceURL   string
}

// TaskInfo 表示 bridge 暴露的异步任务信息。
type TaskInfo struct {
	TaskID uint64
	Status string
}

// AIDebugRequest 表示 AI 调试请求。
type AIDebugRequest struct {
	AgentType string
	Prompt    string
	Params    map[string]string
	RunModel  bool
}

// AIDebugResponse 表示 AI 调试结果。
type AIDebugResponse struct {
	Response       string
	RenderedPrompt string
	Model          string
	TokensUsed     int32
	LatencyMS      int64
}

// ImportLive2DPackageResult 表示 Live2D 模型导入结果。
type ImportLive2DPackageResult struct {
	Name         string
	AssetDir     string
	ModelURL     string
	ThumbnailURL string
	ModelID      uint64
	Created      bool
	IsActive     bool
}

// ImportLive2DBackgroundResult 表示 Live2D 背景导入结果。
type ImportLive2DBackgroundResult struct {
	FileName string
	AssetURL string
}

// RAGConnectionResult 表示 RAG 依赖检测结果。
type RAGConnectionResult struct {
	MilvusOK    bool
	EmbeddingOK bool
	Error       string
}

// RAGIndexResult 表示 RAG 索引操作结果。
type RAGIndexResult struct {
	Indexed int32
	Deleted int32
}

// RAGSearchResult 表示单条 RAG 检索结果。
type RAGSearchResult struct {
	DocID    string
	Title    string
	Content  string
	Score    float64
	Metadata map[string]any
}

// RAGSearchResponse 表示 RAG 检索响应。
type RAGSearchResponse struct {
	Query   string
	Results []RAGSearchResult
}

// ScraperSource 表示可用爬虫源。
type ScraperSource struct {
	Name     string
	Label    string
	BaseURL  string
	IsActive bool
}

// ScraperSearchRequest 表示爬虫搜索请求。
type ScraperSearchRequest struct {
	Keyword  string
	Source   string
	Page     int32
	PageSize int32
}

// ScraperSearchResult 表示爬虫搜索结果。
type ScraperSearchResult struct {
	Title   string
	URL     string
	Source  string
	Snippet string
}

// ScraperFetchRequest 表示爬虫正文抓取请求。
type ScraperFetchRequest struct {
	URL    string
	Source string
}

// ScraperFetchResult 表示爬虫正文抓取结果。
type ScraperFetchResult struct {
	Title   string
	Content string
	Source  string
	URL     string
}

// ScraperCleanRequest 表示题目清洗请求。
type ScraperCleanRequest struct {
	Content      string
	IndustryCode string
	Source       string
	SourceURL    string
}

// ScraperCleanedQuestion 表示清洗后的题目。
type ScraperCleanedQuestion struct {
	CategoryName string
	Type         string
	Difficulty   string
	Title        string
	Content      string
	OptionsJSON  string
	Answer       string
	Explanation  string
	Tags         string
}

// ScraperCleanResult 表示题目清洗结果。
type ScraperCleanResult struct {
	Questions      []ScraperCleanedQuestion
	TotalExtracted int32
}

// ScraperImportRequest 表示题目导入请求。
type ScraperImportRequest struct {
	IndustryCode string
	SourceURL    string
	SourceTitle  string
	Questions    []ScraperCleanedQuestion
}

// ScraperImportResult 表示题目导入结果。
type ScraperImportResult struct {
	TotalCount   int32
	SuccessCount int32
	FailCount    int32
	Errors       []string
}
