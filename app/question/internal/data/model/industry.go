package model

import "gorm.io/gorm"

// Industry 行业静态字典表模型，与 admin 侧 Industry model 对齐。
type Industry struct {
	gorm.Model
	Code        string `gorm:"size:50;not null;uniqueIndex"`
	Name        string `gorm:"size:100;not null"`
	Description string `gorm:"type:text"`
	Icon        string `gorm:"size:200"`
	IsActive    bool   `gorm:"not null;default:true"`
	SortOrder   int    `gorm:"not null;default:0"`
}

// TableName 返回行业字典表名。
func (Industry) TableName() string { return "industries" }
