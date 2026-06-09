package model

// AdminConfig 系统配置 GORM model。
// 配置表采用键覆盖/删除语义，默认不保留软删除字段，避免同键多版本歧义。
type AdminConfig struct {
	Key         string `gorm:"primaryKey;size:100"`
	Value       string `gorm:"type:text"`
	ConfigType  string `gorm:"size:20;not null;default:'string'"`
	Description string `gorm:"size:500"`
}

// TableName 返回系统配置表名。
func (AdminConfig) TableName() string { return "admin_configs" }
