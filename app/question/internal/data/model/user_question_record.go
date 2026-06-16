package model

import "gorm.io/gorm"

// UserQuestionRecord 用户答题记录（对齐单体 user_question_records 表结构）
type UserQuestionRecord struct {
	gorm.Model
	UserID       uint64  `gorm:"index;not null"`
	QuestionID   uint64  `gorm:"index;not null"`
	UserAnswer   string  `gorm:"column:user_answer;type:text"`
	IsCorrect    bool    `gorm:"not null"`
	TimeSpent    int64   `gorm:"column:time_spent"`
	AnalysisJSON string  `gorm:"column:analysis_json;type:text"`
	Language     string  `gorm:"column:language;size:30"`
	Score        float64 `gorm:"column:score;default:0"`
}

func (UserQuestionRecord) TableName() string { return "user_question_records" }
