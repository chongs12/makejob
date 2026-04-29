// Package model 提供数据模型定义
package model

import "time"

// ScraperTask 爬取任务记录
type ScraperTask struct {
	BaseModel
	TaskType      string     `json:"task_type" gorm:"size:50;not null;default:'fetch_snapshot';comment:任务类型(fetch_snapshot/import_questions/question_pipeline_build)"`
	SourceURL     string     `json:"source_url" gorm:"size:500;not null;comment:来源URL"`
	SourceTitle   string     `json:"source_title" gorm:"size:500;comment:来源标题"`
	Source        string     `json:"source" gorm:"size:50;not null;comment:数据源(niuke/leetcode/juejin/manual)"`
	Status        string     `json:"status" gorm:"size:20;not null;default:'pending';comment:状态(pending/running/fetched/cleaned/imported/succeeded/failed)"`
	RawContent    string     `json:"-" gorm:"type:text;comment:原始内容"`   // 不返回给前端
	PayloadJSON   string     `json:"-" gorm:"type:text;comment:异步任务载荷"` // 不返回给前端
	ResultJSON    string     `json:"-" gorm:"type:text;comment:异步任务结果"` // 不返回给前端
	QuestionCount int        `json:"question_count" gorm:"default:0;comment:提取题目数"`
	ImportedCount int        `json:"imported_count" gorm:"default:0;comment:已导入题目数"`
	RetryCount    int        `json:"retry_count" gorm:"default:0;comment:执行尝试次数"`
	StartedAt     *time.Time `json:"started_at,omitempty" gorm:"comment:任务开始执行时间"`
	FinishedAt    *time.Time `json:"finished_at,omitempty" gorm:"comment:任务结束时间"`
	ErrorMsg      string     `json:"error_msg,omitempty" gorm:"type:text;comment:错误信息"`
}

// TableName 指定表名
func (ScraperTask) TableName() string {
	return "scraper_tasks"
}
