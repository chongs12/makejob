package model

import "gorm.io/gorm"

// AICallLog AI 调用日志 GORM model
type AICallLog struct {
	gorm.Model
	TraceID         string `gorm:"size:64;not null;index"`
	TaskID          *uint  `gorm:"index"`
	Source          string `gorm:"size:32;not null;index"`
	Scene           string `gorm:"size:32;not null;index"`
	IndustryID      *uint  `gorm:"index"`
	PromptSource    string `gorm:"size:64"`
	SelectedPromptID *uint `gorm:"index"`
	SelectedPromptName string `gorm:"size:255"`
	RenderedPrompt  string `gorm:"type:text"`
	RequestMessages string `gorm:"type:text"`
	RuntimeConfig   string `gorm:"type:text"`
	SceneConfig     string `gorm:"type:text"`
	Provider        string `gorm:"size:64"`
	ModelName       string `gorm:"column:model;size:128"`
	UserInput       string `gorm:"type:text"`
	ModelOutput     string `gorm:"type:text"`
	ModelError      string `gorm:"type:text"`
	LatencyMs       int64  `gorm:"not null;default:0"`
	IsSuccess       bool   `gorm:"not null;default:false;index"`
	InputTokens     int    `gorm:"not null;default:0"`
	OutputTokens    int    `gorm:"not null;default:0"`
	AgentType       string `gorm:"size:50;index"`
	TokensUsed      int32
	Status          string `gorm:"size:20"`
}

func (AICallLog) TableName() string { return "ai_call_logs" }
