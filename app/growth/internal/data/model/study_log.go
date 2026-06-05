package model

import "gorm.io/gorm"

// StudyLog 学习记录 GORM model
type StudyLog struct {
	gorm.Model
	UserID   uint64 `gorm:"index;not null"`
	Action   string `gorm:"size:50"`
	RefID    uint64
	Duration int32
}

func (StudyLog) TableName() string { return "study_logs" }
