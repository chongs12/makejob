package model

import "gorm.io/gorm"

// QuestionSet 题集模型
type QuestionSet struct {
	gorm.Model
	Name         string `gorm:"size:300;not null"`
	Description  string `gorm:"type:text"`
	IndustryCode string `gorm:"size:50;index"`
	CoverImage   string `gorm:"size:500"`
	QuestionCount int32 `gorm:"default:0"`
}

func (QuestionSet) TableName() string { return "question_sets" }

// QuestionSetItem 题集与题目关联模型
type QuestionSetItem struct {
	gorm.Model
	SetID      uint64 `gorm:"index;not null"`
	QuestionID uint64 `gorm:"index;not null"`
	SortOrder  int32  `gorm:"default:0"`
}

func (QuestionSetItem) TableName() string { return "question_set_items" }
