package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"makejob/app/companion/internal/biz"
	"makejob/app/companion/internal/conf"
)

// NewTTSProviderFactory 根据配置创建 TTS 供应商实例
func NewTTSProviderFactory(cfg *conf.TTS) biz.TTSProvider {
	if cfg == nil {
		return nil
	}

	// 如果配置了多供应商引擎，使用工厂模式
	if len(cfg.Engines) > 0 {
		return newMultiEngineProvider(cfg)
	}

	// 否则使用默认火山引擎（兼容旧配置）
	if cfg.APIKey != "" {
		voice := cfg.Voice
		if voice == "" {
			voice = "zh_female_shuangkuaisisi_moon_bigtts"
		}
		return NewVolcengineProvider(cfg.APIKey, voice)
	}

	return nil
}

// NewTTSProviderFromConfigRecord 从数据库配置记录构建 TTS Provider
func NewTTSProviderFromConfigRecord(record *biz.TTSConfig) (biz.TTSProvider, error) {
	if record == nil {
		return nil, fmt.Errorf("tts config record is nil")
	}

	// 解析 auth_config_json
	authConfig, err := parseAuthConfig(record.AuthConfigJSON)
	if err != nil {
		return nil, fmt.Errorf("parse auth config: %w", err)
	}

	// 解析 params_json
	paramsConfig, err := parseAuthConfig(record.ParamsJSON)
	if err != nil {
		return nil, fmt.Errorf("parse params config: %w", err)
	}

	engine := strings.ToLower(strings.TrimSpace(record.Engine))
	switch engine {
	case biz.TTSEngineVolcengine:
		apiKey := authConfig["api_key"]
		return NewVolcengineProvider(apiKey, record.VoiceID), nil
	case biz.TTSEngineMinimax:
		apiKey := authConfig["api_key"]
		return NewMiniMaxProvider(apiKey, "", record.VoiceID), nil
	case biz.TTSEngineXiaomiMIMO:
		apiKey := authConfig["api_key"]
		model := paramsConfig["model"]
		// 调试日志
		fmt.Printf("[TTS] MiMo provider: apiKey=%s, model=%s, voice=%s\n", apiKey, model, record.VoiceID)
		return NewMiMoProvider(apiKey, model, record.VoiceID), nil
	default:
		return nil, fmt.Errorf("unsupported tts engine: %s", engine)
	}
}

// parseAuthConfig 解析 auth_config_json 为 map
func parseAuthConfig(authConfigJSON string) (map[string]string, error) {
	authConfigJSON = strings.TrimSpace(authConfigJSON)
	if authConfigJSON == "" {
		return make(map[string]string), nil
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(authConfigJSON), &config); err != nil {
		return nil, fmt.Errorf("invalid auth config JSON: %w", err)
	}

	result := make(map[string]string)
	for k, v := range config {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result, nil
}

// multiEngineProvider 多引擎 TTS 供应商，按 engine 名称选择对应实现
type multiEngineProvider struct {
	providers map[string]biz.TTSProvider
	default_  string
}

// newMultiEngineProvider 创建多引擎 TTS 供应商
func newMultiEngineProvider(cfg *conf.TTS) biz.TTSProvider {
	providers := make(map[string]biz.TTSProvider)

	for engineName, engineCfg := range cfg.Engines {
		if engineCfg == nil || engineCfg.APIKey == "" {
			continue
		}

		engineName = strings.ToLower(strings.TrimSpace(engineName))
		switch engineName {
		case "volcengine":
			voice := engineCfg.VoiceID
			if voice == "" {
				voice = "zh_female_shuangkuaisisi_moon_bigtts"
			}
			providers[engineName] = NewVolcengineProvider(engineCfg.APIKey, voice)
		case "minimax":
			providers[engineName] = NewMiniMaxProvider(engineCfg.APIKey, engineCfg.GroupID, engineCfg.VoiceID)
		case "xiaomi_mimo", "mimo":
			providers["xiaomi_mimo"] = NewMiMoProvider(engineCfg.APIKey, engineCfg.Model, engineCfg.VoiceID)
		}
	}

	if len(providers) == 0 {
		return nil
	}

	defaultEngine := strings.ToLower(strings.TrimSpace(cfg.DefaultEngine))
	if defaultEngine == "" {
		// 默认使用第一个可用的引擎
		for name := range providers {
			defaultEngine = name
			break
		}
	}

	return &multiEngineProvider{
		providers: providers,
		default_:  defaultEngine,
	}
}

func (p *multiEngineProvider) GetSupportedEngines() []string {
	engines := make([]string, 0, len(p.providers))
	for name := range p.providers {
		engines = append(engines, name)
	}
	return engines
}

func (p *multiEngineProvider) Synthesize(ctx context.Context, req biz.TTSRequest) (*biz.TTSResult, error) {
	engine := strings.ToLower(strings.TrimSpace(req.Engine))
	if engine == "" {
		engine = p.default_
	}

	provider, ok := p.providers[engine]
	if !ok {
		// 回退到默认引擎
		provider, ok = p.providers[p.default_]
		if !ok {
			return nil, fmt.Errorf("no TTS provider available for engine: %s", engine)
		}
	}

	return provider.Synthesize(ctx, req)
}

// NewTTSClientAdapter 将 TTSProvider 适配为 TTSClient 接口
func NewTTSClientAdapter(provider biz.TTSProvider, defaultVoice string) biz.TTSClient {
	if provider == nil {
		return nil
	}
	return &ttsClientAdapter{
		provider:     provider,
		defaultVoice: defaultVoice,
	}
}

// ttsClientAdapter 将 TTSProvider 适配为 TTSClient 接口
type ttsClientAdapter struct {
	provider     biz.TTSProvider
	defaultVoice string
}

func (a *ttsClientAdapter) Synthesize(ctx context.Context, text, voice string) (*biz.TTSAudio, error) {
	if voice == "" {
		voice = a.defaultVoice
	}

	result, err := a.provider.Synthesize(ctx, biz.TTSRequest{
		Text:    text,
		VoiceID: voice,
	})
	if err != nil {
		return nil, err
	}

	return &biz.TTSAudio{
		AudioData: result.AudioData,
		AudioURL:  result.AudioURL,
	}, nil
}
