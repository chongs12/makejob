// Package model 提供数据模型定义
package model

// UserQuestionRecord 用户答题记录表
type UserQuestionRecord struct {
	ID         uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID     uint   `json:"user_id" gorm:"not null;index:idx_user_question,unique;comment:用户ID"`
	QuestionID uint   `json:"question_id" gorm:"not null;index:idx_user_question,unique;index;comment:题目ID"`
	UserAnswer string `json:"user_answer" gorm:"type:text;comment:用户答案"`
	IsCorrect  bool   `json:"is_correct" gorm:"not null;default:false;comment:是否正确"`
	TimeSpent  int    `json:"time_spent" gorm:"comment:答题用时(秒)"`
	CreatedAt  int64  `json:"created_at" gorm:"autoCreateTime;comment:答题时间"`

	// 关联关系
	User     User     `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Question Question `json:"question,omitempty" gorm:"foreignKey:QuestionID"`
}

// TableName 指定表名
func (UserQuestionRecord) TableName() string {
	return "user_question_records"
}
