package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 是所有 GORM model 的基础结构体
// 对应 proto 中的 makejob.shared.v1.BaseModel
type BaseModel struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
