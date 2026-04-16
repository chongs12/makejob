// Package model 提供数据模型定义
package model

// UserFavorite 用户收藏表
type UserFavorite struct {
	ID         uint  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID     uint  `json:"user_id" gorm:"not null;uniqueIndex:idx_user_question;index;comment:用户ID"`
	QuestionID uint  `json:"question_id" gorm:"not null;uniqueIndex:idx_user_question;index;comment:题目ID"`
	CreatedAt  int64 `json:"created_at" gorm:"autoCreateTime;comment:收藏时间"`

	// 关联关系
	User     User     `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Question Question `json:"question,omitempty" gorm:"foreignKey:QuestionID"`
}

// TableName 指定表名
func (UserFavorite) TableName() string {
	return "user_favorites"
}
