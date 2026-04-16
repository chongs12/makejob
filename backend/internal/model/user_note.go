// Package model 提供数据模型定义
package model

// UserNote 用户学习笔记表
type UserNote struct {
	BaseModel
	UserID     uint   `json:"user_id" gorm:"not null;index;comment:用户ID"`
	QuestionID *uint  `json:"question_id" gorm:"index;comment:关联题目ID，可为空"`
	Title      string `json:"title" gorm:"size:200;not null;comment:笔记标题"`
	Content    string `json:"content" gorm:"type:text;not null;comment:笔记内容"`

	// 关联关系
	User     User     `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Question Question `json:"question,omitempty" gorm:"foreignKey:QuestionID"`
}

// TableName 指定表名
func (UserNote) TableName() string {
	return "user_notes"
}
