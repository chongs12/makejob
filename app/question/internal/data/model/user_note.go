package model

import "gorm.io/gorm"

type UserNote struct {
	gorm.Model
	UserID     uint64 `gorm:"index;not null"`
	QuestionID uint64 `gorm:"index;not null"`
	Content    string `gorm:"type:text;not null"`
}

func (UserNote) TableName() string { return "user_notes" }
