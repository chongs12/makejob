package model

import (
	"time"

	"gorm.io/gorm"
)

type Question struct {
	gorm.Model
	CategoryID         uint   `gorm:"not null;index"`
	IndustryID         uint   `gorm:"not null;index"`
	Type               string `gorm:"size:20;not null"`
	Difficulty         string `gorm:"size:10;not null;default:'medium'"`
	Title              string `gorm:"size:500;not null"`
	Content            string `gorm:"type:text;not null"`
	OptionsJSON        string `gorm:"type:text"`
	Answer             string `gorm:"type:text;not null"`
	Explanation        string `gorm:"type:text"`
	SolutionJSON       string `gorm:"type:text"`
	JudgeConfigJSON    string `gorm:"type:text"`
	AnswerTemplateJSON string `gorm:"type:text"`
	Tags               string `gorm:"size:500"`
	IsActive           bool   `gorm:"not null;default:true"`
}

func (Question) TableName() string { return "questions" }

type Category struct {
	gorm.Model
	IndustryID  uint   `gorm:"not null;index"`
	Name        string `gorm:"size:100;not null"`
	ParentID    *uint  `gorm:"index"`
	SortOrder   int    `gorm:"not null;default:0"`
	Icon        string `gorm:"size:200"`
	Description string `gorm:"type:text"`
}

func (Category) TableName() string { return "categories" }

type Industry struct {
	gorm.Model
	Code        string `gorm:"size:50;not null;uniqueIndex"`
	Name        string `gorm:"size:100;not null"`
	Description string `gorm:"type:text"`
	Icon        string `gorm:"size:200"`
	IsActive    bool   `gorm:"not null;default:true"`
	SortOrder   int    `gorm:"not null;default:0"`
}

func (Industry) TableName() string { return "industries" }

type MockInterview struct {
	gorm.Model
	UserID     uint   `gorm:"not null;index"`
	IndustryID uint   `gorm:"index"`
	Status     string `gorm:"size:20;not null;default:'preparing'"`
	Score      float64
}

func (MockInterview) TableName() string { return "mock_interviews" }

type User struct {
	gorm.Model
	Username          string     `gorm:"size:100;not null;uniqueIndex"`
	Email             string     `gorm:"size:200;not null;uniqueIndex"`
	PasswordHash      string     `gorm:"size:200;not null;column:password_hash"`
	Avatar            string     `gorm:"size:500"`
	Role              string     `gorm:"size:20;not null;default:'user'"`
	MembershipLevel   string     `gorm:"size:20;not null;default:'free'"`
	MembershipType    string     `gorm:"size:20"`
	MembershipExpireAt *time.Time
	IsDisabled        bool       `gorm:"not null;default:false"`
}

func (User) TableName() string { return "users" }

type ScraperTask struct {
	gorm.Model
	TaskType      string     `gorm:"size:50;not null;default:'fetch_snapshot'"`
	SourceURL     string     `gorm:"size:500;not null"`
	SourceTitle   string     `gorm:"size:500"`
	Source        string     `gorm:"size:50;not null"`
	Status        string     `gorm:"size:20;not null;default:'pending'"`
	RawContent    string     `gorm:"type:text" json:"-"`
	PayloadJSON   string     `gorm:"type:text" json:"-"`
	ResultJSON    string     `gorm:"type:text" json:"-"`
	QuestionCount int        `gorm:"default:0"`
	ImportedCount int        `gorm:"default:0"`
	RetryCount    int        `gorm:"default:0"`
	StartedAt     *time.Time
	FinishedAt    *time.Time
	ErrorMsg      string     `gorm:"type:text"`
}

func (ScraperTask) TableName() string { return "scraper_tasks" }

type RAGDocument struct {
	gorm.Model
	Collection string `gorm:"size:100;not null;index"`
	DocType    string `gorm:"size:50;not null;index"`
	Title      string `gorm:"size:500;not null"`
	Content    string `gorm:"type:text;not null"`
	Metadata   string `gorm:"type:jsonb"`
	VectorID   string `gorm:"size:100"`
	SyncStatus string `gorm:"size:20;not null;default:'pending'"`
	IsActive   bool   `gorm:"not null;default:true"`
}

func (RAGDocument) TableName() string { return "rag_documents" }

type Live2DModel struct {
	gorm.Model
	Name         string `gorm:"size:100;not null"`
	IndustryID   *uint  `gorm:"index"`
	Scene        string `gorm:"size:20;not null"`
	ModelURL     string `gorm:"size:500;not null"`
	ThumbnailURL string `gorm:"size:500"`
	ConfigJSON   string `gorm:"type:text"`
	TTSConfigID  *uint  `gorm:"index"`
	IsActive     bool   `gorm:"not null;default:true"`
}

func (Live2DModel) TableName() string { return "live2d_models" }

type TTSConfig struct {
	gorm.Model
	Name           string `gorm:"size:100;not null"`
	Engine         string `gorm:"size:32;not null"`
	VoiceID        string `gorm:"size:100;not null"`
	AuthConfigJSON string `gorm:"type:text"`
	ParamsJSON     string `gorm:"type:text"`
	IsActive       bool   `gorm:"not null;default:true"`
	SortOrder      int    `gorm:"not null;default:0"`
}

func (TTSConfig) TableName() string { return "tts_configs" }
