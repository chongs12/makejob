package model

import "gorm.io/gorm"

// UserNote 用户笔记表（对齐单体：增加 Title 字段）
type UserNote struct {
	gorm.Model
	UserID     uint64 `gorm:"index;not null"`
	QuestionID *uint64 `gorm:"index"` // 对齐单体：可空（全局笔记）
	Title      string `gorm:"size:200;not null"`
	Content    string `gorm:"type:text;not null"`
}

func (UserNote) TableName() string { return "user_notes" }
