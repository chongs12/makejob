package model

import "gorm.io/gorm"

type UserQuestionRecord struct {
	gorm.Model
	UserID     uint64  `gorm:"index;not null"`
	QuestionID uint64  `gorm:"index;not null"`
	IsCorrect  bool    `gorm:"not null"`
	Answer     string  `gorm:"type:text"`
	Language   string  `gorm:"size:30"`
	Score      float64 `gorm:"default:0"`
}

func (UserQuestionRecord) TableName() string { return "user_question_records" }
