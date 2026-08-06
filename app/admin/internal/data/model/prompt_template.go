package model

import (
	"time"

	"gorm.io/gorm"
)

// PromptTemplate Prompt 模板 GORM model（对齐 AI Gateway 数据库表结构）
type PromptTemplate struct {
	ID              uint           `gorm:"primaryKey;autoIncrement"`
	IndustryID      *uint          `gorm:"index"`
	Name            string         `gorm:"size:100;not null"`
	Scene           string         `gorm:"size:64;not null;index"`
	TemplateContent string         `gorm:"type:text;not null"`
	Variables       string         `gorm:"type:text"`
	IsActive        bool           `gorm:"not null;default:true"`
	CreatedAt       time.Time      `gorm:"autoCreateTime"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

func (PromptTemplate) TableName() string { return "prompt_templates" }
