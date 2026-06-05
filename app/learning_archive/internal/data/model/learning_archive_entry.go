package model

import (
	"time"

	"gorm.io/gorm"
)

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
