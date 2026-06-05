package model

// AdminConfig 系统配置 GORM model
type AdminConfig struct {
	Key         string `gorm:"primaryKey;size:100"`
	Value       string `gorm:"type:text"`
	ConfigType  string `gorm:"size:20;not null;default:'string'"`
	Description string `gorm:"size:500"`
}

func (AdminConfig) TableName() string { return "admin_configs" }
