package model

// UserQuestionRecord 用户答题记录（对齐单体 user_question_records 表结构）
type UserQuestionRecord struct {
	ID           uint64  `gorm:"primaryKey;autoIncrement"`
	UserID       uint64  `gorm:"index;not null"`
	QuestionID   uint64  `gorm:"index;not null"`
	UserAnswer   string  `gorm:"column:user_answer;type:text"`
	IsCorrect    bool    `gorm:"not null"`
	TimeSpent    int64   `gorm:"column:time_spent"`
	AnalysisJSON string  `gorm:"column:analysis_json;type:text"`
	Language     string  `gorm:"column:language;size:30"`
	Score        float64 `gorm:"column:score;default:0"`
	CreatedAt    int64   `gorm:"column:created_at"`
}

func (UserQuestionRecord) TableName() string { return "user_question_records" }
