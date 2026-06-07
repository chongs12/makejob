package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 所有实体公共基础字段（FIX G3: 符合全局规范 1.4）
type BaseModel struct {
	ID        uint           `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time      `gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"not null;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
