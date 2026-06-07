package model

import (
	"time"

	"gorm.io/gorm"
)

// LearningArchiveEntry 学习档案实体，使用 gorm.Model 承载软删除字段。
// 注意：若使用 Raw SQL 聚合 mistake_tags，必须显式补 deleted_at IS NULL 过滤条件。
type LearningArchiveEntry struct {
	gorm.Model
	UserID          uint64 `gorm:"index;not null"`
	SourceType      string `gorm:"size:50;not null;index"`
	SourceRef       string `gorm:"size:100"`
	InterviewID     uint64 `gorm:"index"`
	QuestionIndex   int32
	IndustryCode    string    `gorm:"size:50;index"`
	PlanPhase       string    `gorm:"size:50"`
	PlanPhaseGoal   string    `gorm:"size:200"`
	Language        string    `gorm:"size:30"`
	MistakeTags     string    `gorm:"type:text"` // JSON array
	StrengthTags    string    `gorm:"type:text"` // JSON array
	Suggestions     string    `gorm:"type:text"` // JSON array
	EvidenceSummary string    `gorm:"type:text"`
	OccurredAt      time.Time `gorm:"index"`
}

// TableName 返回学习档案表名。
func (LearningArchiveEntry) TableName() string {
	return "learning_archive_entries"
}
