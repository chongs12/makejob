package model

import "gorm.io/gorm"

// AIPreset 对应 AI 预设表，保持与当前线上 schema 一致。
type AIPreset struct {
	gorm.Model
	Name       string `gorm:"size:100;not null;uniqueIndex"`
	ConfigJSON string `gorm:"column:config_json;type:text;not null;default:''"`
	IsActive   bool   `gorm:"column:is_active;not null;default:false"`
}

// TableName 返回 AI 预设表名。
func (AIPreset) TableName() string { return "ai_presets" }
