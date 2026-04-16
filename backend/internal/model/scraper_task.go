// Package model 提供数据模型定义
package model

// ScraperTask 爬取任务记录
type ScraperTask struct {
	BaseModel
	SourceURL     string `json:"source_url" gorm:"size:500;not null;comment:来源URL"`
	SourceTitle   string `json:"source_title" gorm:"size:500;comment:来源标题"`
	Source        string `json:"source" gorm:"size:50;not null;comment:数据源(niuke/leetcode/juejin)"`
	Status        string `json:"status" gorm:"size:20;not null;default:'pending';comment:状态(pending/fetched/cleaned/imported/failed)"`
	RawContent    string `json:"-" gorm:"type:text;comment:原始内容"` // 不返回给前端
	QuestionCount int    `json:"question_count" gorm:"default:0;comment:提取题目数"`
	ImportedCount int    `json:"imported_count" gorm:"default:0;comment:已导入题目数"`
	ErrorMsg      string `json:"error_msg,omitempty" gorm:"type:text;comment:错误信息"`
}

// TableName 指定表名
func (ScraperTask) TableName() string {
	return "scraper_tasks"
}
