package biz

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ProviderMeta TTS 供应商元数据
type ProviderMeta struct {
	Key            string
	Label          string
	SupportStatus  string // "supported", "unsupported", "unknown"
	SupportMessage string
	AuthTemplate   string
	ParamsTemplate string
	AuthFields     []string
	ParamFields    []string
}

// ProviderCatalog TTS 供应商目录（对齐单体 buildLegacyTTSProviders）
type ProviderCatalog struct {
	providers map[string]*ProviderMeta
}

// NewProviderCatalog 创建 TTS 供应商目录
func NewProviderCatalog() *ProviderCatalog {
	c := &ProviderCatalog{providers: make(map[string]*ProviderMeta)}
	c.registerDefaults()
	return c
}

// registerDefaults 注册默认供应商（对齐单体硬编码列表）
func (c *ProviderCatalog) registerDefaults() {
	c.Register(&ProviderMeta{
		Key:            "volcengine",
		Label:          "火山引擎",
		SupportStatus:  "supported",
		SupportMessage: "完全支持，推荐使用",
		AuthTemplate:   `{"api_key":"xxx","app_id":"xxx","access_token":"xxx"}`,
		ParamsTemplate: `{"voice_type":"BV001_streaming","encoding":"mp3","sample_rate":24000}`,
		AuthFields:     []string{"api_key", "app_id", "access_token"},
		ParamFields:    []string{"voice_type", "encoding", "sample_rate", "speed_ratio", "volume_ratio", "pitch_ratio"},
	})
	c.Register(&ProviderMeta{
		Key:            "minimax",
		Label:          "MiniMax",
		SupportStatus:  "supported",
		SupportMessage: "完全支持",
		AuthTemplate:   `{"api_key":"xxx","group_id":"xxx"}`,
		ParamsTemplate: `{"model":"speech-2.8-turbo","voice_id":"male-qn-jingying","format":"mp3"}`,
		AuthFields:     []string{"api_key", "group_id"},
		ParamFields:    []string{"model", "voice_id", "format", "sample_rate", "emotion"},
	})
	c.Register(&ProviderMeta{
		Key:            "xiaomi_mimo",
		Label:          "小米 MiMo",
		SupportStatus:  "supported",
		SupportMessage: "完全支持",
		AuthTemplate:   `{"api_key":"xxx"}`,
		ParamsTemplate: `{"model":"mimo-v2-tts","voice":"mimo_default","format":"wav"}`,
		AuthFields:     []string{"api_key"},
		ParamFields:    []string{"model", "voice", "format"},
	})
}

// Register 注册供应商元数据
func (c *ProviderCatalog) Register(meta *ProviderMeta) {
	if meta == nil || strings.TrimSpace(meta.Key) == "" {
		return
	}
	c.providers[strings.ToLower(strings.TrimSpace(meta.Key))] = meta
}

// Lookup 查询供应商元数据
func (c *ProviderCatalog) Lookup(engine string) *ProviderMeta {
	if c == nil {
		return nil
	}
	return c.providers[strings.ToLower(strings.TrimSpace(engine))]
}

// ListAll 列出所有供应商元数据
func (c *ProviderCatalog) ListAll() []*ProviderMeta {
	if c == nil {
		return nil
	}
	result := make([]*ProviderMeta, 0, len(c.providers))
	for _, meta := range c.providers {
		result = append(result, meta)
	}
	return result
}

// GetSupportStatus 获取供应商支持状态
func (c *ProviderCatalog) GetSupportStatus(engine string) (status, message string) {
	meta := c.Lookup(engine)
	if meta == nil {
		return "unknown", "未知的 TTS 引擎"
	}
	return meta.SupportStatus, meta.SupportMessage
}

// ValidateTTSConfig 校验 TTS 配置输入（对齐单体 ValidateTTSConfigInput）
func (c *ProviderCatalog) ValidateTTSConfig(engine, voiceID, authConfigJSON, paramsJSON string) (string, string, error) {
	normalizedEngine := strings.TrimSpace(engine)
	if normalizedEngine == "" {
		return "", "", fmt.Errorf("tts engine is required")
	}

	// 校验引擎是否支持
	meta := c.Lookup(normalizedEngine)
	if meta == nil {
		return "", "", fmt.Errorf("unsupported tts engine: %s", normalizedEngine)
	}

	if strings.TrimSpace(voiceID) == "" {
		return "", "", fmt.Errorf("voice_id is required")
	}

	// 校验并规范化 JSON
	authMap, err := normalizeJSONObjectString(authConfigJSON)
	if err != nil {
		return "", "", fmt.Errorf("invalid auth_config_json: %w", err)
	}
	paramsMap, err := normalizeJSONObjectString(paramsJSON)
	if err != nil {
		return "", "", fmt.Errorf("invalid params_json: %w", err)
	}

	// 按引擎类型校验必填字段
	switch normalizedEngine {
	case "volcengine":
		if !hasNonEmptyValue(authMap, "api_key") && !(hasNonEmptyValue(authMap, "app_id") && hasNonEmptyValue(authMap, "access_token")) {
			return "", "", fmt.Errorf("volcengine requires api_key or app_id + access_token")
		}
	case "minimax":
		if !hasNonEmptyValue(authMap, "api_key") {
			return "", "", fmt.Errorf("minimax requires api_key")
		}
	case "xiaomi_mimo":
		if !hasNonEmptyValue(authMap, "api_key") {
			return "", "", fmt.Errorf("xiaomi_mimo requires api_key")
		}
	}

	normalizedAuth, err := marshalNormalizedJSONMap(authMap)
	if err != nil {
		return "", "", err
	}
	normalizedParams, err := marshalNormalizedJSONMap(paramsMap)
	if err != nil {
		return "", "", err
	}
	return normalizedAuth, normalizedParams, nil
}

// normalizeJSONObjectString 解析并规范化 JSON 对象字符串
func normalizeJSONObjectString(s string) (map[string]interface{}, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return make(map[string]interface{}), nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return m, nil
}

// hasNonEmptyValue 检查 map 中是否有非空值
func hasNonEmptyValue(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	s, ok := v.(string)
	if !ok {
		return true // 非字符串值视为有值
	}
	return strings.TrimSpace(s) != ""
}

// marshalNormalizedJSONMap 将 map 序列化为规范化 JSON 字符串
func marshalNormalizedJSONMap(m map[string]interface{}) (string, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("marshal JSON: %w", err)
	}
	return string(b), nil
}
