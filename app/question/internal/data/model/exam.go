package model

import (
	"gorm.io/gorm"
)

// Exam 考试记录模型
type Exam struct {
	gorm.Model
	UserID       uint64 `gorm:"index;not null"`
	IndustryCode string `gorm:"size:50;index"`
	QuestionIDs  string `gorm:"type:text"` // JSON 序列化的题目 ID 列表
	TimeLimitMin int32  `gorm:"not null"`
	Status       string `gorm:"size:20;not null;default:pending"`
	TotalScore   float64
}

func (Exam) TableName() string { return "exams" }
