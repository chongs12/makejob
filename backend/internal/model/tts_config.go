// Package model 提供数据模型定义
package model

// TTSEngine TTS引擎枚举
const (
	TTSEngineElevenlabs = "elevenlabs" // ElevenLabs
	TTSEngineMinimax    = "minimax"    // MiniMax
	TTSEngineAliyun     = "aliyun"     // 阿里云
	TTSEngineXunfei     = "xunfei"     // 讯飞
)

// TTSScene 使用场景枚举
const (
	TTSSceneInterview = "interview" // 面试场景
	TTSSceneCompanion = "companion" // 陪伴场景
)

// TTSConfig TTS音色配置表
type TTSConfig struct {
	BaseModel
	Name       string `json:"name" gorm:"size:100;not null;comment:音色名称"`
	Engine     string `json:"engine" gorm:"size:20;not null;comment:TTS引擎(elevenlabs/minimax/aliyun/xunfei)"`
	VoiceID    string `json:"voice_id" gorm:"size:100;not null;comment:音色ID"`
	Scene      string `json:"scene" gorm:"size:20;not null;comment:使用场景(interview/companion)"`
	ParamsJSON string `json:"params_json" gorm:"type:text;comment:额外参数JSON"`
	IsActive   bool   `json:"is_active" gorm:"not null;default:true;comment:是否启用"`
	SortOrder  int    `json:"sort_order" gorm:"not null;default:0;comment:排序顺序"`
}

// TableName 指定表名
func (TTSConfig) TableName() string {
	return "tts_configs"
}
