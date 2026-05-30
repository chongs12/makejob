package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/milvus-io/milvus/client/v2/milvusclient"

	"makejob-backend/internal/ai"
	eino "makejob-backend/internal/ai/eino"
	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
)

// InitResult RAG初始化结果
type InitResult struct {
	Service *Service
	Closer  func() // 关闭Milvus连接
}

// Init 初始化RAG系统
// 1. 连接Milvus
// 2. 确保Collection存在
// 3. 创建Embedder
// 4. 创建Indexer和Retriever
// 5. 组装Service
func Init(ctx context.Context, cfg Config) (*InitResult, error) {
	if cfg.Collection == "" {
		cfg.Collection = "interview_questions"
	}
	if cfg.TopK <= 0 {
		cfg.TopK = 5
	}

	// 连接Milvus（带超时，避免Milvus未启动时阻塞）
	connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
	defer connectCancel()

	milvusClient, err := milvusclient.New(connectCtx, &milvusclient.ClientConfig{
		Address:  cfg.MilvusAddr,
		Username: cfg.MilvusUser,
		Password: cfg.MilvusPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("连接Milvus失败: %w", err)
	}

	applogger.Info("Milvus连接成功",
		zap.String("addr", cfg.MilvusAddr),
	)

	// 确保Collection存在
	dim := DefaultEmbeddingDim
	if err := EnsureCollection(ctx, milvusClient, cfg.Collection, dim); err != nil {
		milvusClient.Close(ctx)
		return nil, fmt.Errorf("初始化Collection失败: %w", err)
	}

	// 创建Embedder
	embedder, err := eino.NewEmbedder(ctx, cfg.ArkAPIKey, cfg.EmbedModel, cfg.ArkBaseURL)
	if err != nil {
		milvusClient.Close(ctx)
		return nil, fmt.Errorf("创建Embedder失败: %w", err)
	}

	// 创建Indexer和Retriever
	indexer := NewMilvusIndexer(milvusClient, embedder, cfg.Collection, dim)
	retriever := NewMilvusRetriever(milvusClient, embedder, cfg.Collection, cfg.TopK)

	// 组装Service
	service := NewService(indexer, retriever, cfg)

	applogger.Info("RAG系统初始化完成",
		zap.String("collection", cfg.Collection),
		zap.Int("topK", cfg.TopK),
	)

	return &InitResult{
		Service: service,
		Closer: func() {
			milvusClient.Close(ctx)
			applogger.Info("Milvus连接已关闭")
		},
	}, nil
}

// InitFromConfigs 从配置map初始化RAG系统
// 配置优先级：configs参数 > config.yaml默认值
func InitFromConfigs(ctx context.Context, configs map[string]string) (*InitResult, error) {
	// 检查RAG是否启用
	if strings.TrimSpace(configs[ai.ConfigKeyRAGEnabled]) != "true" {
		return nil, fmt.Errorf("RAG未启用")
	}

	// 验证配置
	if err := ValidateRAGConfig(configs); err != nil {
		return nil, fmt.Errorf("RAG配置验证失败: %w", err)
	}

	// 提取配置
	cfg := ExtractRAGConfig(configs)

	// 调用Init初始化
	return Init(ctx, cfg)
}

// IsRAGEnabled 检查RAG是否启用
func IsRAGEnabled(configs map[string]string) bool {
	return strings.TrimSpace(configs[ai.ConfigKeyRAGEnabled]) == "true"
}

// GetRAGConfigFingerprint 计算RAG配置指纹
func GetRAGConfigFingerprint(configs map[string]string) string {
	keys := []string{
		ai.ConfigKeyRAGEnabled,
		ai.ConfigKeyRAGEmbedAPIKey,
		ai.ConfigKeyRAGEmbedModel,
		ai.ConfigKeyRAGEmbedBaseURL,
		ai.ConfigKeyRAGMilvusAddr,
		ai.ConfigKeyRAGMilvusUser,
		ai.ConfigKeyRAGMilvusPassword,
		ai.ConfigKeyRAGCollection,
		ai.ConfigKeyRAGTopK,
		ai.ConfigKeyRAGScoreThreshold,
	}

	var parts []string
	for _, key := range keys {
		parts = append(parts, key+"="+configs[key])
	}
	return strings.Join(parts, "|")
}
