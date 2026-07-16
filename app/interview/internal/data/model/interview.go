package model

import (
	"time"

	"gorm.io/gorm"
)

// MockInterview 面试会话 GORM model（对齐单体 mock_interviews 表结构）
type MockInterview struct {
	gorm.Model
	UserID             uint64     `gorm:"index;not null"`
	IndustryID         uint64     `gorm:"column:industry_id;index;not null"`
	Status             string     `gorm:"size:20;default:created;index"` // created, in_progress, ongoing, report_generating, report_failed, completed
	InterviewType      string     `gorm:"size:20;default:'';index"` // knowledge | job，决定出题与报告模板
	KnowledgeTopicsJSON string    `gorm:"column:knowledge_topics;type:text"` // 知识点专项面试的自定义知识点 JSON 数组
	ResumeText          string    `gorm:"column:resume_text;type:text"` // 简历原文，岗位求职报告依赖
	JobDescription      string    `gorm:"column:job_description;type:text"` // 目标岗位 JD
	ResumeParsedJSON    string    `gorm:"column:resume_parsed_json;type:text"` // 简历解析画像 JSON
	Score               float64   `gorm:"column:score"`
	TotalQuestions     int32      `gorm:"column:total_questions"`
	CurrentIndex       int32      `gorm:"column:current_index;default:0"` // 实时面试已回答题数，用户每答一题递增，用于自动结束判定
	AIFeedback         string     `gorm:"column:ai_feedback;type:text"`
	AISessionID        string     `gorm:"column:ai_session_id;type:text"`
	ReportJSON         string     `gorm:"column:report_json;type:text"`
	StartedAt          *time.Time `gorm:"column:started_at"`
	EndedAt            *time.Time `gorm:"column:ended_at;index"`
	Live2DModelKey     string     `gorm:"size:128"`
}

func (MockInterview) TableName() string {
	return "mock_interviews"
}

// InterviewMessage 面试消息 GORM model（对齐单体 interview_messages 表结构）
type InterviewMessage struct {
	ID            uint64 `gorm:"primaryKey;autoIncrement"`
	InterviewID   uint64 `gorm:"index;not null"`
	Role          string `gorm:"size:20;not null"` // user, assistant
	Content       string `gorm:"type:text;not null"`
	MessageType   string `gorm:"size:20;default:text"` // text, code, audio
	QuestionIndex int32  `gorm:"column:question_index;default:0"`
	CreatedAt     int64  `gorm:"column:created_at;autoCreateTime:milli"`
	MetadataJSON  string `gorm:"column:metadata_json;type:text"`
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
	Output          string  `gorm:"type:text"`
	ErrorMsg        string  `gorm:"type:text"`
	AIScore         float64 `gorm:"default:0"`
	AIFeedback      string  `gorm:"type:text"`
	CreatedAt       time.Time      `gorm:"autoCreateTime"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

func (InterviewCodingAttempt) TableName() string {
	return "interview_coding_attempts"
}

// InterviewReport 面试报告 GORM model
type InterviewReport struct {
	ID                    uint64  `gorm:"primaryKey;autoIncrement"`
	InterviewID           uint64  `gorm:"uniqueIndex;not null"`
	OverallScore          float64 `gorm:"not null;default:0"`
	ReportTemplate        string  `gorm:"size:20;default:''"` // knowledge | job | ""
	ReportDataJSON        string  `gorm:"type:text"` // 完整结构化报告 JSON，前端按 report_template 渲染
	DimensionScoresJSON   string  `gorm:"type:text"`
	StrengthsJSON         string  `gorm:"type:text"`
	WeaknessesJSON        string  `gorm:"type:text"`
	SuggestionsJSON       string  `gorm:"type:text"`
	Summary               string  `gorm:"type:text"`
	CodingDiagnosticsJSON string  `gorm:"type:text"`
	CreatedAt             time.Time `gorm:"autoCreateTime"`
	UpdatedAt             time.Time `gorm:"autoUpdateTime"`
	DeletedAt             gorm.DeletedAt `gorm:"index"`
}

func (InterviewReport) TableName() string {
	return "interview_reports"
}
