// Package model 提供数据模型定义
package model

// ConfigType 配置类型枚举
const (
	ConfigTypeString  = "string"  // 字符串
	ConfigTypeJSON    = "json"    // JSON
	ConfigTypeNumber  = "number"  // 数字
	ConfigTypeBoolean = "boolean" // 布尔值
)

// AdminConfig 后台配置表
type AdminConfig struct {
	BaseModel
	ConfigKey   string `json:"config_key" gorm:"size:100;not null;uniqueIndex;comment:配置键"`
	ConfigValue string `json:"config_value" gorm:"type:text;not null;comment:配置值"`
	ConfigType  string `json:"config_type" gorm:"size:20;not null;default:'string';comment:配置类型(string/json/number/boolean)"`
	Description string `json:"description" gorm:"size:500;comment:配置描述"`
}

// TableName 指定表名
func (AdminConfig) TableName() string {
	return "admin_configs"
}
