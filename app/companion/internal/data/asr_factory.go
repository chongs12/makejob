package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"makejob/app/companion/internal/biz"
)

// NewASRProviderFromConfigRecord 从数据库配置记录构建 ASR Provider。
func NewASRProviderFromConfigRecord(record *biz.ASRConfig) (biz.ASRProvider, error) {
	if record == nil {
		return nil, fmt.Errorf("asr config record is nil")
	}

	authConfig, err := parseASRAuthConfig(record.AuthConfigJSON)
	if err != nil {
		return nil, fmt.Errorf("parse asr auth config: %w", err)
	}

	engine := strings.ToLower(strings.TrimSpace(record.Engine))
	switch engine {
	case biz.ASREngineVolcengine:
		appID := authConfig["app_id"]
		accessToken := authConfig["access_token"]
		if appID == "" || accessToken == "" {
			return nil, fmt.Errorf("volcengine asr requires app_id and access_token")
		}
		paramsConfig, _ := parseASRAuthConfig(record.ParamsJSON)
		cluster := paramsConfig["cluster"]
		language := paramsConfig["language"]
		return NewVolcengineASRProvider(appID, accessToken, cluster, language), nil
	case biz.ASREngineXiaomiMiMo:
		apiKey := authConfig["api_key"]
		if apiKey == "" {
			return nil, fmt.Errorf("xiaomi_mimo asr requires api_key")
		}
		paramsConfig, _ := parseASRAuthConfig(record.ParamsJSON)
		return NewMiMoASRProvider(apiKey, paramsConfig["model"], paramsConfig["language"], paramsConfig["base_url"]), nil
	default:
		return nil, fmt.Errorf("unsupported asr engine: %s", engine)
	}
}

// parseASRAuthConfig 解析 ASR 鉴权配置 JSON。
func parseASRAuthConfig(authConfigJSON string) (map[string]string, error) {
	authConfigJSON = strings.TrimSpace(authConfigJSON)
	if authConfigJSON == "" {
		return make(map[string]string), nil
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(authConfigJSON), &config); err != nil {
		return nil, fmt.Errorf("invalid asr auth config JSON: %w", err)
	}

	result := make(map[string]string)
	for k, v := range config {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result, nil
}

// NewASRClientAdapter 将 ASRProvider 适配为 ASRClient 接口。
func NewASRClientAdapter(provider biz.ASRProvider) biz.ASRClient {
	if provider == nil {
		return nil
	}
	return &asrClientAdapter{provider: provider}
}

// asrClientAdapter 将 ASRProvider 适配为 ASRClient 接口。
type asrClientAdapter struct {
	provider biz.ASRProvider
}

func (a *asrClientAdapter) Recognize(ctx context.Context, req biz.ASRRequest) (*biz.ASRResult, error) {
	return a.provider.Recognize(ctx, req)
}
