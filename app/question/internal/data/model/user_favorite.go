package model

import "time"

type UserFavorite struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	UserID    uint64    `gorm:"index;not null"`
	QuestionID uint64   `gorm:"index;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (UserFavorite) TableName() string { return "user_favorites" }
