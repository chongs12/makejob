package model

import "gorm.io/gorm"

// AIPreset AI 预设配置 GORM model
type AIPreset struct {
	gorm.Model
	Name       string `gorm:"size:100;not null;uniqueIndex"`
	Provider   string `gorm:"size:50"`
	ModelName  string `gorm:"column:model;size:100"`
	Params     string `gorm:"type:text"` // JSON-encoded map[string]string (legacy)
	ConfigJSON string `gorm:"type:text"` // JSON-encoded map[string]string (full config)
	IsDefault  bool   `gorm:"default:false"`
	IsActive   bool   `gorm:"not null;default:false"`
}

func (AIPreset) TableName() string { return "ai_presets" }
