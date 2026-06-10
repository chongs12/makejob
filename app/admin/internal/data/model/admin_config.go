package model

import (
	"time"

	"gorm.io/gorm"
)

// AdminConfig 对应后台系统配置表，映射当前线上 schema 的 config_key/config_value 列。
type AdminConfig struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	Key         string         `gorm:"column:config_key;size:100;not null;uniqueIndex"`
	Value       string         `gorm:"column:config_value;type:text;not null"`
	ConfigType  string         `gorm:"column:config_type;size:20;not null;default:'string'"`
	Description string         `gorm:"column:description;size:500"`
}

// TableName 返回系统配置表名。
func (AdminConfig) TableName() string { return "admin_configs" }
