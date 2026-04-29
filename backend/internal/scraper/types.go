// Package scraper 提供面经爬取与清洗导入功能
// 定义了爬虫接口、清洗接口及相关数据类型
package scraper

import "time"

// Source 数据源定义
type Source struct {
	Name     string `json:"name"`  // niuke/leetcode/juejin
	Label    string `json:"label"` // 牛客/LeetCode/掘金
	BaseURL  string `json:"base_url"`
	IsActive bool   `json:"is_active"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Keyword  string `json:"keyword" binding:"required"`
	Source   string `json:"source" binding:"required"` // niuke/leetcode/juejin
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Title     string `json:"title"`
	URL       string `json:"url"`
	Author    string `json:"author"`
	Date      string `json:"date"`
	Summary   string `json:"summary"`
	Source    string `json:"source"`
	ViewCount int    `json:"view_count"`
}

// FetchRequest 爬取请求
type FetchRequest struct {
	URL    string `json:"url" binding:"required"`
	Source string `json:"source" binding:"required"`
}

// FetchResult 爬取结果
type FetchResult struct {
	Title     string    `json:"title"`
	Content   string    `json:"content"` // 原始面经内容
	Author    string    `json:"author"`
	URL       string    `json:"url"`
	Source    string    `json:"source"`
	FetchedAt time.Time `json:"fetched_at"`
}

// CleanRequest 清洗请求
type CleanRequest struct {
	Content      string `json:"content" binding:"required"`
	IndustryCode string `json:"industry_code"`
	Source       string `json:"source"`
	SourceURL    string `json:"source_url"`
}

// CleanedQuestion 清洗后的题目
type CleanedQuestion struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Type        string   `json:"type"`       // choice/multi/code/subjective
	Difficulty  string   `json:"difficulty"` // easy/medium/hard
	Category    string   `json:"category"`   // 推荐分类名
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
	Tags        []string `json:"tags"`
	Confidence  float64  `json:"confidence"` // AI置信度 0-1
}

// CleanResult 清洗结果
type CleanResult struct {
	Questions   []CleanedQuestion `json:"questions"`
	TotalFound  int               `json:"total_found"`
	SourceTitle string            `json:"source_title"`
	SourceURL   string            `json:"source_url"`
}

// ImportRequest 导入请求
type ImportRequest struct {
	IndustryCode string            `json:"industry_code" binding:"required"`
	Questions    []CleanedQuestion `json:"questions" binding:"required,min=1"`
	SourceURL    string            `json:"source_url"`
	SourceTitle  string            `json:"source_title"`
}

// ImportResult 导入结果
type ImportResult struct {
	TotalCount   int      `json:"total_count"`
	SuccessCount int      `json:"success_count"`
	FailCount    int      `json:"fail_count"`
	Errors       []string `json:"errors,omitempty"`
}

// TaskListFilter 抓取任务列表筛选条件，供后台运行态页面按状态与任务类型缩小范围。
type TaskListFilter struct {
	Status   string `json:"status,omitempty" form:"status"`
	TaskType string `json:"task_type,omitempty" form:"task_type"`
}

// TaskDetail 抓取任务详情结构，显式暴露异步任务载荷与执行结果，方便后台排查。
type TaskDetail struct {
	ID            uint       `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	TaskType      string     `json:"task_type"`
	SourceURL     string     `json:"source_url"`
	SourceTitle   string     `json:"source_title"`
	Source        string     `json:"source"`
	Status        string     `json:"status"`
	QuestionCount int        `json:"question_count"`
	ImportedCount int        `json:"imported_count"`
	RetryCount    int        `json:"retry_count"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	ErrorMsg      string     `json:"error_msg,omitempty"`
	PayloadJSON   string     `json:"payload_json,omitempty"`
	ResultJSON    string     `json:"result_json,omitempty"`
}

// TaskStatus 任务状态常量
const (
	TaskStatusPending   = "pending"   // 待处理
	TaskStatusRunning   = "running"   // 执行中
	TaskStatusFetched   = "fetched"   // 已爬取
	TaskStatusCleaned   = "cleaned"   // 已清洗
	TaskStatusImported  = "imported"  // 已导入
	TaskStatusSucceeded = "succeeded" // 已完成
	TaskStatusFailed    = "failed"    // 失败
)

// TaskType 任务类型常量。
const (
	TaskTypeFetchSnapshot         = "fetch_snapshot"          // 同步抓取后留痕的快照任务
	TaskTypeImportQuestions       = "import_questions"        // 异步导入题目任务
	TaskTypeQuestionPipelineBuild = "question_pipeline_build" // 异步生成题目流水线候选题卡任务
)

// 支持的数据源常量
const (
	SourceNiuke    = "niuke"
	SourceLeetCode = "leetcode"
	SourceJuejin   = "juejin"
)
