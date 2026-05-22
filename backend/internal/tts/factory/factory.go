package factory

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"makejob-backend/internal/config"
	"makejob-backend/internal/model"
	"makejob-backend/internal/tts"
	ttsminimax "makejob-backend/internal/tts/minimax"
	ttsvolc "makejob-backend/internal/tts/volcengine"
	ttsmimo "makejob-backend/internal/tts/xiaomi_mimo"
)

const (
	// ProviderTypeMock 表示使用本地 Mock TTS。
	ProviderTypeMock = "mock"
	// ProviderTypeMiniMax 表示使用 MiniMax 官方 TTS。
	ProviderTypeMiniMax = "minimax"
	// ProviderTypeVolcengine 表示使用火山云真实 TTS。
	ProviderTypeVolcengine = "volcengine"
)

// NewTTSProvider 使用全局配置创建 TTS Provider。
func NewTTSProvider(providerType string) (tts.TTSProvider, error) {
	return NewTTSProviderWithConfig(providerType, config.GetConfig())
}

// NewTTSProviderWithConfig 根据显式配置创建 TTS Provider。
func NewTTSProviderWithConfig(providerType string, cfg *config.Config) (tts.TTSProvider, error) {
	switch normalizeProviderType(providerType, cfg) {
	case ProviderTypeMiniMax:
		if cfg == nil {
			return nil, fmt.Errorf("tts provider config is nil")
		}
		return ttsminimax.NewProvider(buildMiniMaxConfig(cfg))
	case ProviderTypeVolcengine:
		if cfg == nil {
			return nil, fmt.Errorf("tts provider config is nil")
		}
		return ttsvolc.NewProvider(cfg.Volcengine)
	case ProviderTypeMock:
		return nil, fmt.Errorf("tts provider mock is disabled")
	default:
		return nil, fmt.Errorf("tts provider is not configured")
	}
}

// NewTTSProviderFromConfigRecord 根据后台 TTS 配置记录创建真实 Provider。
func NewTTSProviderFromConfigRecord(record *model.TTSConfig) (tts.TTSProvider, error) {
	if record == nil {
		return nil, fmt.Errorf("tts config record is nil")
	}

	switch strings.TrimSpace(record.Engine) {
	case model.TTSEngineMinimax:
		cfg, err := buildMiniMaxConfigFromRecord(record)
		if err != nil {
			return nil, err
		}
		return ttsminimax.NewProvider(cfg)
	case model.TTSEngineVolcengine:
		cfg, err := buildVolcengineConfigFromRecord(record)
		if err != nil {
			return nil, err
		}
		return ttsvolc.NewProvider(cfg)
	case model.TTSEngineXiaomiMIMO:
		cfg, err := buildXiaomiMIMOConfigFromRecord(record)
		if err != nil {
			return nil, err
		}
		return ttsmimo.NewProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported tts engine: %s", strings.TrimSpace(record.Engine))
	}
}

// normalizeProviderType 归一化 TTS Provider 类型。
func normalizeProviderType(providerType string, cfg *config.Config) string {
	normalized := strings.ToLower(strings.TrimSpace(providerType))
	if normalized != "" {
		return normalized
	}
	if cfg != nil && cfg.MiniMax.TTS.Enabled {
		return ProviderTypeMiniMax
	}
	if cfg != nil && cfg.Volcengine.TTS.Enabled {
		return ProviderTypeVolcengine
	}
	return ProviderTypeMock
}

// buildMiniMaxConfig 组装带有 API Key 回退逻辑的 MiniMax 配置。
func buildMiniMaxConfig(cfg *config.Config) config.MiniMaxConfig {
	if cfg == nil {
		return config.MiniMaxConfig{}
	}

	minimaxCfg := cfg.MiniMax
	if strings.TrimSpace(minimaxCfg.APIKey) == "" {
		minimaxCfg.APIKey = firstNonEmpty(cfg.AI.APIKey, cfg.Volcengine.Ark.APIKey)
	}
	if strings.TrimSpace(minimaxCfg.BaseURL) == "" {
		minimaxCfg.BaseURL = firstNonEmpty(cfg.AI.BaseURL, cfg.Volcengine.Ark.BaseURL)
	}
	return minimaxCfg
}

// firstNonEmpty 返回第一个非空字符串，用于配置回退。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// buildMiniMaxConfigFromRecord 把后台 TTS 记录转换为 MiniMax 运行时配置。
func buildMiniMaxConfigFromRecord(record *model.TTSConfig) (config.MiniMaxConfig, error) {
	authMap, err := parseJSONObject(record.AuthConfigJSON)
	if err != nil {
		return config.MiniMaxConfig{}, fmt.Errorf("parse minimax auth_config_json: %w", err)
	}
	paramsMap, err := parseJSONObject(record.ParamsJSON)
	if err != nil {
		return config.MiniMaxConfig{}, fmt.Errorf("parse minimax params_json: %w", err)
	}

	return config.MiniMaxConfig{
		GroupID: getJSONString(authMap, "group_id"),
		APIKey:  getJSONString(authMap, "api_key"),
		BaseURL: getJSONString(authMap, "base_url"),
		TTS: config.MiniMaxTTSConfig{
			Enabled:        true,
			BaseURL:        firstNonEmpty(getJSONString(authMap, "base_url"), getJSONString(paramsMap, "base_url")),
			Model:          getJSONString(paramsMap, "model"),
			VoiceID:        firstNonEmpty(strings.TrimSpace(record.VoiceID), getJSONString(paramsMap, "voice_id")),
			Emotion:        getJSONString(paramsMap, "emotion"),
			Format:         getJSONString(paramsMap, "format"),
			SampleRate:     getJSONInt(paramsMap, "sample_rate"),
			Bitrate:        getJSONInt(paramsMap, "bitrate"),
			Channel:        getJSONInt(paramsMap, "channel"),
			Speed:          getJSONFloat(paramsMap, "speed"),
			Volume:         getJSONFloat(paramsMap, "volume"),
			Pitch:          getJSONInt(paramsMap, "pitch"),
			SubtitleEnable: getJSONBool(paramsMap, "subtitle_enable"),
			OutputFormat:   getJSONString(paramsMap, "output_format"),
			TimeoutSeconds: getJSONInt(paramsMap, "timeout_seconds"),
		},
	}, nil
}

// buildVolcengineConfigFromRecord 把后台 TTS 记录转换为豆包 / 火山语音运行时配置。
func buildVolcengineConfigFromRecord(record *model.TTSConfig) (config.VolcengineConfig, error) {
	authMap, err := parseJSONObject(record.AuthConfigJSON)
	if err != nil {
		return config.VolcengineConfig{}, fmt.Errorf("parse volcengine auth_config_json: %w", err)
	}
	paramsMap, err := parseJSONObject(record.ParamsJSON)
	if err != nil {
		return config.VolcengineConfig{}, fmt.Errorf("parse volcengine params_json: %w", err)
	}

	return config.VolcengineConfig{
		TTS: config.VolcTTSConfig{
			Enabled:     true,
			BaseURL:     firstNonEmpty(getJSONString(authMap, "base_url"), getJSONString(paramsMap, "base_url")),
			APIKey:      getJSONString(authMap, "api_key"),
			AppID:       getJSONString(authMap, "app_id"),
			AccessToken: getJSONString(authMap, "access_token"),
			Cluster:     getJSONString(paramsMap, "cluster"),
			ResourceID:  getJSONString(paramsMap, "resource_id"),
			VoiceType:   firstNonEmpty(strings.TrimSpace(record.VoiceID), getJSONString(paramsMap, "voice_type")),
			Encoding:    getJSONString(paramsMap, "encoding"),
			SpeedRatio:  getJSONInt(paramsMap, "speed_ratio"),
			VolumeRatio: getJSONInt(paramsMap, "volume_ratio"),
			PitchRatio:  getJSONInt(paramsMap, "pitch_ratio"),
			SampleRate:  getJSONInt(paramsMap, "sample_rate"),
		},
	}, nil
}

// buildXiaomiMIMOConfigFromRecord 把后台 TTS 记录转换为 Xiaomi MiMo 运行时配置。
func buildXiaomiMIMOConfigFromRecord(record *model.TTSConfig) (ttsmimo.Config, error) {
	authMap, err := parseJSONObject(record.AuthConfigJSON)
	if err != nil {
		return ttsmimo.Config{}, fmt.Errorf("parse xiaomi mimo auth_config_json: %w", err)
	}
	paramsMap, err := parseJSONObject(record.ParamsJSON)
	if err != nil {
		return ttsmimo.Config{}, fmt.Errorf("parse xiaomi mimo params_json: %w", err)
	}

	return ttsmimo.Config{
		APIKey:              getJSONString(authMap, "api_key"),
		BaseURL:             getJSONString(authMap, "base_url"),
		Model:               getJSONString(paramsMap, "model"),
		Voice:               firstNonEmpty(strings.TrimSpace(record.VoiceID), getJSONString(paramsMap, "voice")),
		Format:              getJSONString(paramsMap, "format"),
		Temperature:         getJSONFloat(paramsMap, "temperature"),
		MaxCompletionTokens: getJSONInt(paramsMap, "max_completion_tokens"),
		TimeoutSeconds:      getJSONInt(paramsMap, "timeout_seconds"),
	}, nil
}

// parseJSONObject 把 JSON 对象文本解析为 map，兼容空字符串。
func parseJSONObject(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return map[string]any{}, nil
	}
	return payload, nil
}

// getJSONString 安全读取 JSON 对象中的字符串字段。
func getJSONString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

// getJSONInt 安全读取 JSON 对象中的整数值。
func getJSONInt(payload map[string]any, key string) int {
	value, ok := payload[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return 0
}

// getJSONFloat 安全读取 JSON 对象中的浮点值。
func getJSONFloat(payload map[string]any, key string) float64 {
	value, ok := payload[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

// getJSONBool 安全读取 JSON 对象中的布尔值。
func getJSONBool(payload map[string]any, key string) bool {
	value, ok := payload[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		return err == nil && parsed
	default:
		return false
	}
}
