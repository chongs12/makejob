package service

import (
	"fmt"
	"strconv"
	"strings"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
	"makejob-backend/internal/rag"
)

// RAGConfigResponse RAG配置响应
type RAGConfigResponse struct {
	Configs  map[string]string   `json:"configs"`
	Items    []model.AdminConfig `json:"items"`
	Status   RAGSystemStatus     `json:"status"`
	Warnings []string            `json:"warnings"`
}

// RAGSystemStatus RAG系统状态
type RAGSystemStatus struct {
	Enabled         bool   `json:"enabled"`
	MilvusConnected bool   `json:"milvus_connected"`
	Collection      string `json:"collection"`
	EmbedModel      string `json:"embed_model"`
}

// RAGConnectionTestResult RAG连接测试结果
type RAGConnectionTestResult struct {
	MilvusOK    bool   `json:"milvus_ok"`
	EmbeddingOK bool   `json:"embedding_ok"`
	Error       string `json:"error,omitempty"`
}

// buildRAGConfigResponse 构建RAG配置响应
func buildRAGConfigResponse(items []model.AdminConfig, baseConfig map[string]string) *RAGConfigResponse {
	configMap := ai.DefaultRuntimeConfig()

	// 合并baseConfig
	for key, value := range baseConfig {
		if ai.IsRAGConfigKey(key) && strings.TrimSpace(value) != "" {
			configMap[key] = value
		}
	}

	// 合并数据库配置
	filtered := make([]model.AdminConfig, 0, len(items))
	for _, item := range items {
		if !ai.IsRAGConfigKey(item.ConfigKey) {
			continue
		}
		filtered = append(filtered, item)
		configMap[item.ConfigKey] = item.ConfigValue
	}

	// 构建状态
	status := RAGSystemStatus{
		Enabled:    configMap[ai.ConfigKeyRAGEnabled] == "true",
		Collection: configMap[ai.ConfigKeyRAGCollection],
		EmbedModel: configMap[ai.ConfigKeyRAGEmbedModel],
	}

	// 验证配置
	var warnings []string
	if status.Enabled {
		if err := rag.ValidateRAGConfig(configMap); err != nil {
			warnings = append(warnings, err.Error())
		}
	}

	return &RAGConfigResponse{
		Configs:  configMap,
		Items:    filtered,
		Status:   status,
		Warnings: warnings,
	}
}

// buildRAGConfigItems 将RAG配置转换为AdminConfig项
func buildRAGConfigItems(configs map[string]string) []model.AdminConfig {
	items := make([]model.AdminConfig, 0)
	for key, value := range configs {
		if !ai.IsRAGConfigKey(key) {
			continue
		}
		items = append(items, model.AdminConfig{
			ConfigKey:   key,
			ConfigValue: value,
			ConfigType:  inferRAGConfigType(key),
			Description: describeRAGConfig(key),
		})
	}
	return items
}

// inferRAGConfigType 推导RAG配置项的类型
func inferRAGConfigType(key string) string {
	switch key {
	case ai.ConfigKeyRAGTopK:
		return model.ConfigTypeNumber
	case ai.ConfigKeyRAGScoreThreshold:
		return model.ConfigTypeNumber
	case ai.ConfigKeyRAGEnabled:
		return model.ConfigTypeBoolean
	default:
		return model.ConfigTypeString
	}
}

// describeRAGConfig 生成RAG配置项的描述
func describeRAGConfig(key string) string {
	descriptions := map[string]string{
		ai.ConfigKeyRAGEnabled:        "是否启用RAG语义检索",
		ai.ConfigKeyRAGEmbedAPIKey:    "火山引擎API Key（留空复用主API Key）",
		ai.ConfigKeyRAGEmbedModel:     "Embedding模型ID（如doubao-embedding-large-text-240915）",
		ai.ConfigKeyRAGEmbedBaseURL:   "Ark API端点",
		ai.ConfigKeyRAGMilvusAddr:     "Milvus服务地址",
		ai.ConfigKeyRAGMilvusUser:     "Milvus用户名",
		ai.ConfigKeyRAGMilvusPassword: "Milvus密码",
		ai.ConfigKeyRAGCollection:     "默认Collection名称",
		ai.ConfigKeyRAGTopK:           "默认返回结果数量（1-50）",
		ai.ConfigKeyRAGScoreThreshold: "相似度阈值（0-1）",
	}
	if desc, ok := descriptions[key]; ok {
		return desc
	}
	return "RAG配置"
}

// normalizeRAGConfigs 标准化RAG配置
func normalizeRAGConfigs(configs map[string]string) (map[string]string, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("配置不能为空")
	}

	normalized := make(map[string]string)
	for key, value := range configs {
		if ai.IsRAGConfigKey(key) {
			normalized[key] = strings.TrimSpace(value)
		}
	}

	// 设置默认值
	if _, ok := normalized[ai.ConfigKeyRAGCollection]; !ok || normalized[ai.ConfigKeyRAGCollection] == "" {
		normalized[ai.ConfigKeyRAGCollection] = "interview_questions"
	}
	if _, ok := normalized[ai.ConfigKeyRAGTopK]; !ok || normalized[ai.ConfigKeyRAGTopK] == "" {
		normalized[ai.ConfigKeyRAGTopK] = "5"
	}
	if _, ok := normalized[ai.ConfigKeyRAGScoreThreshold]; !ok || normalized[ai.ConfigKeyRAGScoreThreshold] == "" {
		normalized[ai.ConfigKeyRAGScoreThreshold] = "0.5"
	}
	if _, ok := normalized[ai.ConfigKeyRAGEmbedModel]; !ok || normalized[ai.ConfigKeyRAGEmbedModel] == "" {
		normalized[ai.ConfigKeyRAGEmbedModel] = "doubao-embedding-large-text-240915"
	}
	if _, ok := normalized[ai.ConfigKeyRAGEmbedBaseURL]; !ok || normalized[ai.ConfigKeyRAGEmbedBaseURL] == "" {
		normalized[ai.ConfigKeyRAGEmbedBaseURL] = "https://ark.cn-beijing.volces.com/api/v3"
	}

	// 验证配置
	if err := rag.ValidateRAGConfig(normalized); err != nil {
		return nil, err
	}

	return normalized, nil
}

// parseRAGTopK 解析TopK配置
func parseRAGTopK(configs map[string]string) int {
	if topKStr := strings.TrimSpace(configs[ai.ConfigKeyRAGTopK]); topKStr != "" {
		if v, err := strconv.Atoi(topKStr); err == nil && v > 0 {
			return v
		}
	}
	return 5
}

// parseRAGScoreThreshold 解析ScoreThreshold配置
func parseRAGScoreThreshold(configs map[string]string) float64 {
	if thresholdStr := strings.TrimSpace(configs[ai.ConfigKeyRAGScoreThreshold]); thresholdStr != "" {
		if v, err := strconv.ParseFloat(thresholdStr, 64); err == nil {
			return v
		}
	}
	return 0.5
}
