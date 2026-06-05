package model

import "gorm.io/gorm"

// PromptTemplate Prompt 模板 GORM model
type PromptTemplate struct {
	gorm.Model
	IndustryID      *uint  `gorm:"index"`
	Name            string `gorm:"size:100;not null"`
	IndustryCode    string `gorm:"size:50;index"`
	TemplateType    string `gorm:"size:50"`
	Scene           string `gorm:"size:20;not null"`
	TemplateContent string `gorm:"type:text;not null"`
	Content         string `gorm:"type:text"` // legacy alias
	Variables       string `gorm:"type:text"`
	IsActive        bool   `gorm:"not null;default:true"`
}

func (PromptTemplate) TableName() string { return "prompt_templates" }
