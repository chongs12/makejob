// Package model 提供数据模型定义
package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 公共基础模型，包含所有表共有的字段
type BaseModel struct {
	ID        uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time      `json:"created_at" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"not null;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
