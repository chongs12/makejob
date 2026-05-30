package rag

import (
	"fmt"
	"strconv"
	"strings"

	"makejob-backend/internal/ai"
)

// ValidateRAGConfig 验证RAG配置的合法性
func ValidateRAGConfig(configs map[string]string) error {
	enabled := strings.TrimSpace(configs[ai.ConfigKeyRAGEnabled])
	if enabled != "true" {
		return nil // 未启用时不验证
	}

	// 验证Milvus连接参数
	if addr := strings.TrimSpace(configs[ai.ConfigKeyRAGMilvusAddr]); addr == "" {
		return fmt.Errorf("Milvus地址不能为空")
	}

	// 验证Embedding配置
	apiKey := strings.TrimSpace(configs[ai.ConfigKeyRAGEmbedAPIKey])
	if apiKey == "" {
		// 尝试使用主API Key
		apiKey = strings.TrimSpace(configs[ai.ConfigKeyAPIKey])
	}
	if apiKey == "" {
		return fmt.Errorf("Embedding API Key不能为空")
	}

	model := strings.TrimSpace(configs[ai.ConfigKeyRAGEmbedModel])
	if model == "" {
		return fmt.Errorf("Embedding模型ID不能为空")
	}

	// 验证TopK范围
	if topKStr := strings.TrimSpace(configs[ai.ConfigKeyRAGTopK]); topKStr != "" {
		topK, err := strconv.Atoi(topKStr)
		if err != nil {
			return fmt.Errorf("TopK必须是数字")
		}
		if topK < 1 || topK > 50 {
			return fmt.Errorf("TopK必须在1-50之间")
		}
	}

	// 验证ScoreThreshold范围
	if thresholdStr := strings.TrimSpace(configs[ai.ConfigKeyRAGScoreThreshold]); thresholdStr != "" {
		threshold, err := strconv.ParseFloat(thresholdStr, 64)
		if err != nil {
			return fmt.Errorf("相似度阈值必须是数字")
		}
		if threshold < 0 || threshold > 1 {
			return fmt.Errorf("相似度阈值必须在0-1之间")
		}
	}

	return nil
}

// ExtractRAGConfig 从配置map中提取RAG配置
func ExtractRAGConfig(configs map[string]string) Config {
	apiKey := strings.TrimSpace(configs[ai.ConfigKeyRAGEmbedAPIKey])
	if apiKey == "" {
		apiKey = strings.TrimSpace(configs[ai.ConfigKeyAPIKey]) // 复用主API Key
	}

	baseURL := strings.TrimSpace(configs[ai.ConfigKeyRAGEmbedBaseURL])
	if baseURL == "" {
		baseURL = "https://ark.cn-beijing.volces.com/api/v3"
	}

	embedModel := strings.TrimSpace(configs[ai.ConfigKeyRAGEmbedModel])
	if embedModel == "" {
		embedModel = "doubao-embedding-large-text-240915"
	}

	// 提取TopK
	topK := 5
	if topKStr := strings.TrimSpace(configs[ai.ConfigKeyRAGTopK]); topKStr != "" {
		if v, err := strconv.Atoi(topKStr); err == nil && v > 0 {
			topK = v
		}
	}

	return Config{
		MilvusAddr:     strings.TrimSpace(configs[ai.ConfigKeyRAGMilvusAddr]),
		MilvusUser:     strings.TrimSpace(configs[ai.ConfigKeyRAGMilvusUser]),
		MilvusPassword: strings.TrimSpace(configs[ai.ConfigKeyRAGMilvusPassword]),
		Collection:     strings.TrimSpace(configs[ai.ConfigKeyRAGCollection]),
		ArkAPIKey:      apiKey,
		ArkBaseURL:     baseURL,
		EmbedModel:     embedModel,
		TopK:           topK,
	}
}
