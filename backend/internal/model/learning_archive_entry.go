// Package model 提供数据模型定义
package model

import "time"

// LearningArchiveSourceType 定义学习档案条目来源类型。
const (
	LearningArchiveSourceInterviewCoding = "interview_coding"
	LearningArchiveSourcePracticeQuestion = "practice_question"
)

// LearningArchiveEntry 表示写入长期学习档案的一条结构化诊断记录。
type LearningArchiveEntry struct {
	BaseModel
	UserID           uint       `json:"user_id" gorm:"not null;index;uniqueIndex:idx_learning_archive_user_source;comment:用户ID"`
	SourceType       string     `json:"source_type" gorm:"size:40;not null;uniqueIndex:idx_learning_archive_user_source;comment:来源类型"`
	SourceRef        string     `json:"source_ref" gorm:"size:120;not null;uniqueIndex:idx_learning_archive_user_source;comment:来源唯一标识"`
	InterviewID      uint       `json:"interview_id" gorm:"index;comment:关联面试ID"`
	QuestionIndex    int        `json:"question_index" gorm:"comment:题目序号(从0开始)"`
	IndustryCode     string     `json:"industry_code" gorm:"size:50;comment:行业编码"`
	Language         string     `json:"language" gorm:"size:30;comment:编程语言"`
	MistakeTagsJSON  string     `json:"mistake_tags_json" gorm:"type:text;comment:错因标签JSON"`
	StrengthTagsJSON string     `json:"strength_tags_json" gorm:"type:text;comment:优势标签JSON"`
	SuggestionsJSON  string     `json:"suggestions_json" gorm:"type:text;comment:建议列表JSON"`
	EvidenceSummary  string     `json:"evidence_summary" gorm:"type:text;comment:证据摘要"`
	OccurredAt       *time.Time `json:"occurred_at" gorm:"comment:问题发生时间"`

	// 关联关系
	User User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName 指定学习档案条目表名。
func (LearningArchiveEntry) TableName() string {
	return "learning_archive_entries"
}
