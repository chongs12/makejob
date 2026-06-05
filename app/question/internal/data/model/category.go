package model

import "gorm.io/gorm"

type Category struct {
	gorm.Model
	Name         string `gorm:"size:200;not null"`
	IndustryCode string `gorm:"size:50;index"`
	ParentID     uint64 `gorm:"index;default:0"`
}

func (Category) TableName() string { return "categories" }
