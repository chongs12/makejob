package model

// UserFavorite 用户收藏表（对齐单体 user_favorites 表结构）
type UserFavorite struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement"`
	UserID     uint64 `gorm:"uniqueIndex:idx_user_favorites_user_question;not null"`
	QuestionID uint64 `gorm:"uniqueIndex:idx_user_favorites_user_question;not null"`
	CreatedAt  int64  `gorm:"column:created_at"`
}

func (UserFavorite) TableName() string { return "user_favorites" }
