package model

import (
	"time"

	"gorm.io/gorm"
)

// TODO L1: 当前使用 gorm.Model，gorm.Model 包含 DeletedAt 支持软删除。
// 需验证 GetWeakTopics 的 jsonb_array_elements_text 聚合查询是否受 GORM default scope 影响。
// GORM 默认 scope 会添加 WHERE deleted_at IS NULL，但 jsonb_array_elements_text
// 在已软删除记录上可能仍参与聚合。建议添加集成测试验证：软删除一条记录后，
// GetWeakTopics 是否仍返回该记录的 mistake_tags。
type LearningArchiveEntry struct {
	gorm.Model
	UserID          uint64    `gorm:"index;not null"`
	SourceType      string    `gorm:"size:50;not null;index"`
	SourceRef       string    `gorm:"size:100"`
	InterviewID     uint64    `gorm:"index"`
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

func (LearningArchiveEntry) TableName() string {
	return "learning_archive_entries"
}
