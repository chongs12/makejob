package model

import (
	"time"

	"gorm.io/gorm"
)

// MockInterview 面试会话 GORM model
type MockInterview struct {
	gorm.Model
	UserID         uint64  `gorm:"index;not null"`
	IndustryCode   string  `gorm:"size:50;not null;index"`
	Difficulty     string  `gorm:"size:20"`
	Status         string  `gorm:"size:20;default:created;index"` // created, in_progress, completed
	InterviewMode  string  `gorm:"size:30;default:standard"`      // standard, realtime_voice, coding
	QuestionCount  int32
	CurrentIndex   int32
	OverallScore   float64
	ResumeText     string `gorm:"type:text"`
	JobDescription string `gorm:"type:text"`
	Live2DModelKey string `gorm:"size:100"`
}

func (MockInterview) TableName() string {
	return "mock_interviews"
}

// InterviewMessage 面试消息 GORM model
type InterviewMessage struct {
	ID            uint64         `gorm:"primaryKey;autoIncrement"`
	InterviewID   uint64         `gorm:"index;not null"`
	Role          string         `gorm:"size:20;not null"` // user, assistant
	Content       string         `gorm:"type:text;not null"`
	MessageType   string         `gorm:"size:20;default:text"` // text, code, audio
	QuestionIndex int32
	CreatedAt     time.Time      `gorm:"autoCreateTime"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (InterviewMessage) TableName() string {
	return "interview_messages"
}

// InterviewCodingAttempt 编程答题记录 GORM model
type InterviewCodingAttempt struct {
	ID              uint64         `gorm:"primaryKey;autoIncrement"`
	InterviewID     uint64         `gorm:"index;not null"`
	QuestionIndex   int32          `gorm:"not null"`
	Language        string         `gorm:"size:30"`
	Code            string         `gorm:"type:text"`
	Passed          bool
	TestCasesPassed int32
	TotalTestCases  int32
	Output          string `gorm:"type:text"`
	ErrorMsg        string `gorm:"type:text"`
	CreatedAt       time.Time      `gorm:"autoCreateTime"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

func (InterviewCodingAttempt) TableName() string {
	return "interview_coding_attempts"
}
