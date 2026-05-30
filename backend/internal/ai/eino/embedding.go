package eino

import (
	"context"
	"fmt"
	"strings"

	ark "github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino/components/embedding"
)

// NewEmbedder 创建基于豆包的Embedding实例。
// apiKey: 火山引擎API Key。
// model: 模型ID（如doubao-embedding-large-text-240915）。
// baseURL: Ark API端点（可选，默认使用北京区域）。
func NewEmbedder(ctx context.Context, apiKey string, model string, baseURL string) (embedding.Embedder, error) {
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)
	if apiKey == "" {
		return nil, fmt.Errorf("创建Embedder失败: apiKey不能为空")
	}
	if model == "" {
		return nil, fmt.Errorf("创建Embedder失败: model不能为空")
	}

	config := &ark.EmbeddingConfig{
		APIKey: apiKey,
		Model:  model,
	}

	// 设置自定义Endpoint
	if strings.TrimSpace(baseURL) != "" {
		config.BaseURL = strings.TrimSpace(baseURL)
	}

	emb, err := ark.NewEmbedder(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("创建Ark Embedder失败: %w", err)
	}
	return emb, nil
}
