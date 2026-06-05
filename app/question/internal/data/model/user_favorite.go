package model

import "gorm.io/gorm"

type UserFavorite struct {
	gorm.Model
	UserID     uint64 `gorm:"index;not null"`
	QuestionID uint64 `gorm:"index;not null"`
}

func (UserFavorite) TableName() string { return "user_favorites" }
