// Package model 提供数据模型定义
package model

// TTSEngine TTS引擎枚举
const (
	TTSEngineVolcengine = "volcengine"  // 豆包语音 / 火山语音
	TTSEngineMinimax    = "minimax"     // MiniMax
	TTSEngineXiaomiMIMO = "xiaomi_mimo" // Xiaomi MiMo
)

// TTSConfig TTS音色配置表
type TTSConfig struct {
	BaseModel
	Name           string `json:"name" gorm:"size:100;not null;comment:音色名称"`
	Engine         string `json:"engine" gorm:"size:32;not null;comment:TTS供应商引擎(volcengine/minimax/xiaomi_mimo等)"`
	VoiceID        string `json:"voice_id" gorm:"size:100;not null;comment:音色ID或说话人ID"`
	Scene          string `json:"scene" gorm:"size:20;comment:遗留场景字段，不再用于运行时绑定"`
	AuthConfigJSON string `json:"auth_config_json" gorm:"type:text;comment:鉴权配置JSON"`
	ParamsJSON     string `json:"params_json" gorm:"type:text;comment:额外参数JSON"`
	IsActive       bool   `json:"is_active" gorm:"not null;default:true;comment:是否启用"`
	SortOrder      int    `json:"sort_order" gorm:"not null;default:0;comment:排序顺序"`
}

// TableName 指定表名
func (TTSConfig) TableName() string {
	return "tts_configs"
}
