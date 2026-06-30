package biz

import (
	"context"
	"encoding/json"

	"github.com/go-kratos/kratos/v2/log"
)

// FallbackLLMClient 主模型失败时自动切换备用模型的包装器。
// 两个模型都失败时返回主模型的原始错误。
type FallbackLLMClient struct {
	primary  LLMClient
	fallback LLMClient
	logger   log.Helper
}

// NewFallbackLLMClient 创建带备用模型的 LLM 客户端。
func NewFallbackLLMClient(primary, fallback LLMClient, logger log.Logger) LLMClient {
	return &FallbackLLMClient{primary: primary, fallback: fallback, logger: *log.NewHelper(logger)}
}

// Chat 先调用主模型，失败后构造备用模型配置并切换。
func (c *FallbackLLMClient) Chat(ctx context.Context, messages []Message, config *AIConfig) (*LLMResponse, error) {
	resp, err := c.primary.Chat(ctx, messages, config)
	if err == nil {
		return resp, nil
	}

	c.logger.Warnf("主模型调用失败，切换备用模型: %v", err)
	fallbackConfig := c.buildFallbackConfig(config)
	resp, fallbackErr := c.fallback.Chat(ctx, messages, fallbackConfig)
	if fallbackErr != nil {
		c.logger.Errorf("备用模型也失败: %v", fallbackErr)
		return nil, err
	}

	return resp, nil
}

// buildFallbackConfig 将 ai_fallback_* 配置提升为 ai_*，供 openai_client 使用。
func (c *FallbackLLMClient) buildFallbackConfig(config *AIConfig) *AIConfig {
	if config == nil {
		return &AIConfig{}
	}
	fallback := *config
	extras := make(map[string]string)
	if config.ExtraParamsJSON != "" {
		_ = json.Unmarshal([]byte(config.ExtraParamsJSON), &extras)
	}
	if v := extras["ai_fallback_api_key"]; v != "" {
		extras["ai_api_key"] = v
	}
	if v := extras["ai_fallback_base_url"]; v != "" {
		extras["ai_base_url"] = v
	}
	if v := extras["ai_fallback_model"]; v != "" {
		fallback.Model = v
	}
	delete(extras, "ai_fallback_api_key")
	delete(extras, "ai_fallback_base_url")
	delete(extras, "ai_fallback_model")
	if len(extras) > 0 {
		b, _ := json.Marshal(extras)
		fallback.ExtraParamsJSON = string(b)
	}
	return &fallback
}
