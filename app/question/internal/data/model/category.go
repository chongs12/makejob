package model

import "gorm.io/gorm"

// Category 分类模型，与 admin 侧 Category model 对齐。
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
