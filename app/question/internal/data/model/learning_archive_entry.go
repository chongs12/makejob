package model

import "time"

// LearningArchiveEntry 学习档案条目 GORM model（对齐单体 learning_archive_entries 表）
type LearningArchiveEntry struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement"`
	UserID           uint64     `gorm:"not null;index;uniqueIndex:idx_learning_archive_user_source"`
	SourceType       string     `gorm:"size:40;not null;uniqueIndex:idx_learning_archive_user_source"`
	SourceRef        string     `gorm:"size:120;not null;uniqueIndex:idx_learning_archive_user_source"`
	InterviewID      uint64     `gorm:"index"`
	QuestionIndex    int        `gorm:"column:question_index"`
	IndustryCode     string     `gorm:"size:50"`
	TaskPhase        string     `gorm:"size:20"`
	TaskPhaseGoal    string     `gorm:"type:text"`
	Language         string     `gorm:"size:30"`
	MistakeTagsJSON  string     `gorm:"type:text;column:mistake_tags_json"`
	StrengthTagsJSON string     `gorm:"type:text;column:strength_tags_json"`
	SuggestionsJSON  string     `gorm:"type:text;column:suggestions_json"`
	EvidenceSummary  string     `gorm:"type:text;column:evidence_summary"`
	OccurredAt       *time.Time `gorm:"column:occurred_at"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (LearningArchiveEntry) TableName() string {
	return "learning_archive_entries"
}
