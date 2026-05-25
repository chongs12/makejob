// Package config 提供应用程序配置管理功能
// 使用Viper库从YAML配置文件加载配置，支持环境变量覆盖
package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

// Config 应用程序全局配置结构体
type Config struct {
	Server         ServerConfig         `mapstructure:"server"`
	Database       DatabaseConfig       `mapstructure:"database"`
	Redis          RedisConfig          `mapstructure:"redis"`
	JWT            JWTConfig            `mapstructure:"jwt"`
	Casbin         CasbinConfig         `mapstructure:"casbin"`
	AI             AIConfig             `mapstructure:"ai"`
	MiniMax        MiniMaxConfig        `mapstructure:"minimax"`
	Volcengine     VolcengineConfig     `mapstructure:"volcengine"`
	AdminBootstrap AdminBootstrapConfig `mapstructure:"admin_bootstrap"`
	Piston         PistonConfig         `mapstructure:"piston"`
	Logging        LoggingConfig        `mapstructure:"logging"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level      string `mapstructure:"level"`       // debug, info, warn, error
	Format     string `mapstructure:"format"`      // console, json
	Output     string `mapstructure:"output"`      // stdout, file
	FilePath   string `mapstructure:"file_path"`   // 日志文件路径（output=file 时生效）
	MaxSizeMB  int    `mapstructure:"max_size_mb"` // 单个文件最大 MB，默认 100
	MaxBackups int    `mapstructure:"max_backups"` // 最多保留旧文件数，默认 5
	MaxDays    int    `mapstructure:"max_days"`    // 最多保留天数，默认 30
}

// PistonConfig 代码执行引擎配置
type PistonConfig struct {
	Endpoint string `mapstructure:"endpoint"` // Piston API 地址，如 http://localhost:2000/api/v2/piston/execute
	Timeout  int    `mapstructure:"timeout"`  // 超时秒数，默认 30
}

// ServerConfig 服务器相关配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug 或 release
}

// DatabaseConfig 数据库连接配置
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

// DSN 生成PostgreSQL连接字符串
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
}

// RedisConfig Redis连接配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Addr 返回Redis服务器地址
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// JWTConfig JWT令牌配置
type JWTConfig struct {
	Secret        string `mapstructure:"secret"`
	Expire        int    `mapstructure:"expire"`         // 访问令牌过期时间（小时）
	RefreshExpire int    `mapstructure:"refresh_expire"` // 刷新令牌过期时间（小时）
}

// CasbinConfig Casbin RBAC权限配置
type CasbinConfig struct {
	ModelPath string `mapstructure:"model_path"`
}

// AIConfig AI 运行时默认配置
type AIConfig struct {
	Provider         string  `mapstructure:"provider"`
	FallbackProvider string  `mapstructure:"fallback_provider"`
	Model            string  `mapstructure:"model"`
	APIKey           string  `mapstructure:"api_key"`
	BaseURL          string  `mapstructure:"base_url"`
	Temperature      float64 `mapstructure:"temperature"`
	TopP             float64 `mapstructure:"top_p"`
	MaxTokens        int     `mapstructure:"max_tokens"`
	TimeoutSeconds   int     `mapstructure:"timeout_seconds"`
	EnableStream     bool    `mapstructure:"enable_stream"`
	InterviewModel   string  `mapstructure:"interview_model"`
	PlanModel        string  `mapstructure:"plan_model"`
	CompanionModel   string  `mapstructure:"companion_model"`
	QuizModel        string  `mapstructure:"quiz_model"`
}

// MiniMaxConfig MiniMax 公共配置。
type MiniMaxConfig struct {
	GroupID string           `mapstructure:"group_id"`
	APIKey  string           `mapstructure:"api_key"`
	BaseURL string           `mapstructure:"base_url"`
	TTS     MiniMaxTTSConfig `mapstructure:"tts"`
}

// MiniMaxTTSConfig MiniMax TTS 配置。
type MiniMaxTTSConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	BaseURL        string  `mapstructure:"base_url"`
	Model          string  `mapstructure:"model"`
	VoiceID        string  `mapstructure:"voice_id"`
	Emotion        string  `mapstructure:"emotion"`
	Format         string  `mapstructure:"format"`
	SampleRate     int     `mapstructure:"sample_rate"`
	Bitrate        int     `mapstructure:"bitrate"`
	Channel        int     `mapstructure:"channel"`
	Speed          float64 `mapstructure:"speed"`
	Volume         float64 `mapstructure:"volume"`
	Pitch          int     `mapstructure:"pitch"`
	SubtitleEnable bool    `mapstructure:"subtitle_enable"`
	OutputFormat   string  `mapstructure:"output_format"`
	TimeoutSeconds int     `mapstructure:"timeout_seconds"`
}

// VolcengineConfig 火山云公共配置
type VolcengineConfig struct {
	Region    string                   `mapstructure:"region"`
	AccessKey string                   `mapstructure:"access_key"`
	SecretKey string                   `mapstructure:"secret_key"`
	Ark       VolcArkConfig            `mapstructure:"ark"`
	ASR       VolcASRConfig            `mapstructure:"asr"`
	TTS       VolcTTSConfig            `mapstructure:"tts"`
	Realtime  VolcRealtimeDialogConfig `mapstructure:"realtime"`
}

// VolcArkConfig 火山云 Ark 大模型配置
type VolcArkConfig struct {
	APIKey         string `mapstructure:"api_key"`
	BaseURL        string `mapstructure:"base_url"`
	ChatModel      string `mapstructure:"chat_model"`
	InterviewModel string `mapstructure:"interview_model"`
	PlanModel      string `mapstructure:"plan_model"`
	CompanionModel string `mapstructure:"companion_model"`
	QuizModel      string `mapstructure:"quiz_model"`
}

// VolcASRConfig 火山云 ASR 配置
type VolcASRConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	BaseURL     string `mapstructure:"base_url"`
	AppID       string `mapstructure:"app_id"`
	AccessToken string `mapstructure:"access_token"`
	Cluster     string `mapstructure:"cluster"`
	ResourceID  string `mapstructure:"resource_id"`
	Workflow    string `mapstructure:"workflow"`
	AudioFormat string `mapstructure:"audio_format"`
	SampleRate  int    `mapstructure:"sample_rate"`
	Language    string `mapstructure:"language"`
}

// VolcTTSConfig 火山云 TTS 配置
type VolcTTSConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	BaseURL     string `mapstructure:"base_url"`
	APIKey      string `mapstructure:"api_key"`
	AppID       string `mapstructure:"app_id"`
	AccessToken string `mapstructure:"access_token"`
	Cluster     string `mapstructure:"cluster"`
	ResourceID  string `mapstructure:"resource_id"`
	VoiceType   string `mapstructure:"voice_type"`
	Encoding    string `mapstructure:"encoding"`
	SpeedRatio  int    `mapstructure:"speed_ratio"`
	VolumeRatio int    `mapstructure:"volume_ratio"`
	PitchRatio  int    `mapstructure:"pitch_ratio"`
	SampleRate  int    `mapstructure:"sample_rate"`
}

// VolcRealtimeDialogConfig 火山云端到端实时语音大模型配置。
type VolcRealtimeDialogConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	BaseURL         string `mapstructure:"base_url"`
	AppID           string `mapstructure:"app_id"`
	AccessToken     string `mapstructure:"access_token"`
	AppKey          string `mapstructure:"app_key"`
	ResourceID      string `mapstructure:"resource_id"`
	Speaker         string `mapstructure:"speaker"`
	InputMode       string `mapstructure:"input_mode"`
	AudioFormat     string `mapstructure:"audio_format"`
	SampleRate      int    `mapstructure:"sample_rate"`
	TTSFormat       string `mapstructure:"tts_format"`
	TTSSampleRate   int    `mapstructure:"tts_sample_rate"`
	BotName         string `mapstructure:"bot_name"`
	SystemRole      string `mapstructure:"system_role"`
	SpeakingStyle   string `mapstructure:"speaking_style"`
	CharacterPrompt string `mapstructure:"character_prompt"`
	LocationCity    string `mapstructure:"location_city"`
	RecvTimeout     int    `mapstructure:"recv_timeout"`
}

// AdminBootstrapConfig 启动时管理员自检与补齐配置
type AdminBootstrapConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	Username        string `mapstructure:"username"`
	Email           string `mapstructure:"email"`
	Password        string `mapstructure:"password"`
	MembershipLevel string `mapstructure:"membership_level"`
}

var (
	instance *Config
	once     sync.Once
)

// Load 从配置文件加载配置
// configPath 为配置文件路径，如果为空则使用默认路径 "config.yaml"
func Load(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "config.yaml"
	}

	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 支持环境变量覆盖配置
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}

// AIRuntimeDefaults 构建 AI runtime 的默认配置。
// 优先使用 ai 段配置，缺失时回退到 volcengine.ark 段。
func (c *Config) AIRuntimeDefaults() map[string]string {
	runtimeConfig := map[string]string{
		"ai_provider":              firstNonEmpty(c.AI.Provider, "eino"),
		"ai_fallback_provider":     firstNonEmpty(c.AI.FallbackProvider, ""),
		"ai_model":                 firstNonEmpty(c.AI.Model, c.Volcengine.Ark.ChatModel),
		"ai_api_key":               firstNonEmpty(c.AI.APIKey, c.Volcengine.Ark.APIKey),
		"ai_base_url":              firstNonEmpty(c.AI.BaseURL, c.Volcengine.Ark.BaseURL),
		"ai_temperature":           formatFloat(c.AI.Temperature, 0.7),
		"ai_top_p":                 formatFloat(c.AI.TopP, 0.9),
		"ai_max_tokens":            formatInt(c.AI.MaxTokens, 2048),
		"ai_timeout_seconds":       formatInt(c.AI.TimeoutSeconds, 30),
		"ai_enable_stream":         formatBool(c.AI.EnableStream),
		"ai_scene_interview_model": firstNonEmpty(c.AI.InterviewModel, c.Volcengine.Ark.InterviewModel),
		"ai_scene_plan_model":      firstNonEmpty(c.AI.PlanModel, c.Volcengine.Ark.PlanModel),
		"ai_scene_companion_model": firstNonEmpty(c.AI.CompanionModel, c.Volcengine.Ark.CompanionModel),
		"ai_scene_quiz_model":      firstNonEmpty(c.AI.QuizModel, c.Volcengine.Ark.QuizModel),
	}

	return runtimeConfig
}

// GetConfig 获取全局配置实例（单例模式）
// 首次调用时会从默认路径加载配置
func GetConfig() *Config {
	once.Do(func() {
		cfg, err := Load("")
		if err != nil {
			// 如果加载失败，使用默认配置
			cfg = &Config{
				Server: ServerConfig{
					Port: 8082,
					Mode: "debug",
				},
				Database: DatabaseConfig{
					Host:    "localhost",
					Port:    5432,
					User:    "postgres",
					DBName:  "makejob",
					SSLMode: "disable",
				},
				Redis: RedisConfig{
					Host: "localhost",
					Port: 6379,
					DB:   0,
				},
				JWT: JWTConfig{
					Secret:        "makejob-secret-key",
					Expire:        24,
					RefreshExpire: 168,
				},
				Casbin: CasbinConfig{
					ModelPath: "config/rbac_model.conf",
				},
				AI: AIConfig{
					Provider:         "eino",
					FallbackProvider: "mock",
					Temperature:      0.7,
					TopP:             0.9,
					MaxTokens:        2048,
					TimeoutSeconds:   30,
					EnableStream:     false,
				},
				MiniMax: MiniMaxConfig{
					TTS: MiniMaxTTSConfig{
						Model:          "speech-2.8-turbo",
						VoiceID:        "male-qn-jingying",
						Format:         "mp3",
						SampleRate:     32000,
						Bitrate:        128000,
						Channel:        1,
						Speed:          1,
						Volume:         1,
						Pitch:          0,
						SubtitleEnable: false,
						OutputFormat:   "hex",
						TimeoutSeconds: 45,
					},
				},
				Volcengine: VolcengineConfig{
					Ark: VolcArkConfig{},
					ASR: VolcASRConfig{
						AudioFormat: "wav",
						SampleRate:  16000,
						Language:    "zh-CN",
					},
					TTS: VolcTTSConfig{
						Encoding:    "mp3",
						SampleRate:  24000,
						SpeedRatio:  100,
						VolumeRatio: 100,
						PitchRatio:  100,
					},
					Realtime: VolcRealtimeDialogConfig{
						BaseURL:       "wss://openspeech.bytedance.com/api/v3/realtime/dialogue",
						AppKey:        "PlgvMymc7f3tQnJ6",
						ResourceID:    "volc.speech.dialog",
						Speaker:       "zh_female_vv_jupiter_bigtts",
						InputMode:     "push_to_talk",
						AudioFormat:   "pcm",
						SampleRate:    16000,
						TTSFormat:     "pcm_s16le",
						TTSSampleRate: 24000,
						BotName:       "Ariu",
						SystemRole:    "你是一位专业、直接、耐心的中文技术面试官。",
						SpeakingStyle: "请保持口语化、简洁、明确，每次只说一段自然的中文，不要输出 Markdown。",
						LocationCity:  "北京",
						RecvTimeout:   120,
					},
				},
				AdminBootstrap: AdminBootstrapConfig{
					Enabled:         true,
					Username:        "Admin",
					Email:           "admin@makejob.com",
					Password:        "admin123456",
					MembershipLevel: "pro",
				},
				Piston: PistonConfig{
					Endpoint: "http://localhost:2000/api/v2/execute",
					Timeout:  30,
				},
			}
		}
		instance = cfg
	})
	return instance
}

// SetConfig 设置全局配置实例（主要用于测试）
func SetConfig(cfg *Config) {
	instance = cfg
}

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// formatFloat 将浮点配置格式化为字符串，并在未配置时返回默认值。
func formatFloat(value float64, fallback float64) string {
	if value == 0 {
		value = fallback
	}
	return fmt.Sprintf("%.2f", value)
}

// formatInt 将整型配置格式化为字符串，并在未配置时返回默认值。
func formatInt(value int, fallback int) string {
	if value <= 0 {
		value = fallback
	}
	return fmt.Sprintf("%d", value)
}

// formatBool 将布尔配置格式化为字符串。
func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
