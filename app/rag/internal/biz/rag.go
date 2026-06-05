package biz

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
)

// Document 检索结果领域实体
type Document struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Score    float64        `json:"score"`
	MetaData map[string]any `json:"metadata"`
}

// VectorDocument 向量文档领域实体，用于写入向量库
type VectorDocument struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Vector   []float32      `json:"vector"`
	Metadata map[string]any `json:"metadata"`
}

// Embedder 文本向量化接口，data 层实现
type Embedder interface {
	EmbedStrings(ctx context.Context, texts []string) ([][]float64, error)
}

// VectorStore 向量存储接口，data 层实现
type VectorStore interface {
	Search(ctx context.Context, vector []float32, topK int, collection string) ([]Document, error)
	Upsert(ctx context.Context, collection string, docs []VectorDocument) error
	Delete(ctx context.Context, collection string, ids []string) error
}

// RetrieveUseCase 语义检索业务用例
type RetrieveUseCase struct {
	embedder    Embedder
	vectorStore VectorStore
	collection  string
	defaultTopK int
	logger      *log.Helper
}

// NewRetrieveUseCase 创建语义检索用例
func NewRetrieveUseCase(embedder Embedder, store VectorStore, collection string, defaultTopK int, logger log.Logger) *RetrieveUseCase {
	return &RetrieveUseCase{
		embedder:    embedder,
		vectorStore: store,
		collection:  collection,
		defaultTopK: defaultTopK,
		logger:      log.NewHelper(logger),
	}
}

// Retrieve 将查询文本向量化后在 Milvus 中检索最相似的文档
func (uc *RetrieveUseCase) Retrieve(ctx context.Context, query string, topK int) ([]Document, error) {
	if topK <= 0 {
		topK = uc.defaultTopK
	}

	// 向量化查询文本
	embeddings, err := uc.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, ErrEmbeddingFailed.WithCause(err)
	}
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return nil, ErrEmbeddingFailed.WithCause(fmt.Errorf("向量化结果为空"))
	}

	// float64 → float32 转换
	vec64 := embeddings[0]
	vec32 := make([]float32, len(vec64))
	for i, v := range vec64 {
		vec32[i] = float32(v)
	}

	// 在向量库中搜索
	docs, err := uc.vectorStore.Search(ctx, vec32, topK, uc.collection)
	if err != nil {
		return nil, ErrRAGConnectionFailed.WithCause(err)
	}
	if len(docs) == 0 {
		return nil, ErrNoResults
	}

	return docs, nil
}

// IndexUseCase 批量索引业务用例
type IndexUseCase struct {
	embedder    Embedder
	vectorStore VectorStore
	collection  string
	logger      *log.Helper
}

// IndexItem 索引条目
type IndexItem struct {
	ID       string
	Content  string
	Metadata map[string]any
}

// NewIndexUseCase 创建批量索引用例
func NewIndexUseCase(embedder Embedder, store VectorStore, collection string, logger log.Logger) *IndexUseCase {
	return &IndexUseCase{
		embedder:    embedder,
		vectorStore: store,
		collection:  collection,
		logger:      log.NewHelper(logger),
	}
}

// IndexQuestions 批量索引题目，每批最多 16 条
func (uc *IndexUseCase) IndexQuestions(ctx context.Context, items []IndexItem) (indexed int, failed int, failedIDs []string) {
	const batchSize = 16

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[i:end]

		// 提取文本列表用于向量化
		texts := make([]string, len(batch))
		for j, item := range batch {
			texts[j] = item.Content
		}

		// 批量向量化
		embeddings, err := uc.embedder.EmbedStrings(ctx, texts)
		if err != nil {
			uc.logger.Errorf("批量向量化失败 (batch %d-%d): %v", i, end, err)
			for _, item := range batch {
				failedIDs = append(failedIDs, item.ID)
				failed++
			}
			continue
		}

		// 构建向量文档
		docs := make([]VectorDocument, 0, len(batch))
		for j, item := range batch {
			if j >= len(embeddings) {
				break
			}
			vec64 := embeddings[j]
			vec32 := make([]float32, len(vec64))
			for k, v := range vec64 {
				vec32[k] = float32(v)
			}
			docs = append(docs, VectorDocument{
				ID:       item.ID,
				Content:  item.Content,
				Vector:   vec32,
				Metadata: item.Metadata,
			})
		}

		// 写入向量库
		if err := uc.vectorStore.Upsert(ctx, uc.collection, docs); err != nil {
			uc.logger.Errorf("批量写入向量库失败 (batch %d-%d): %v", i, end, err)
			for _, item := range batch {
				failedIDs = append(failedIDs, item.ID)
				failed++
			}
			continue
		}

		indexed += len(docs)
	}

	return indexed, failed, failedIDs
}

// SyncHandler 消息同步处理器，处理题目变更事件
type SyncHandler struct {
	embedder    Embedder
	vectorStore VectorStore
	collection  string
	logger      *log.Helper
}

// NewSyncHandler 创建同步处理器
func NewSyncHandler(embedder Embedder, store VectorStore, collection string, logger log.Logger) *SyncHandler {
	return &SyncHandler{
		embedder:    embedder,
		vectorStore: store,
		collection:  collection,
		logger:      log.NewHelper(logger),
	}
}

// HandleQuestionChanged 处理题目变更消息，根据 action 类型同步向量库
func (h *SyncHandler) HandleQuestionChanged(ctx context.Context, questionID uint64, action string, content string, metadata map[string]any) error {
	docID := fmt.Sprintf("%d", questionID)

	switch action {
	case "create", "update":
		// 向量化内容
		embeddings, err := h.embedder.EmbedStrings(ctx, []string{content})
		if err != nil {
			return ErrEmbeddingFailed.WithCause(err)
		}
		if len(embeddings) == 0 || len(embeddings[0]) == 0 {
			return ErrEmbeddingFailed.WithCause(fmt.Errorf("向量化结果为空"))
		}

		vec64 := embeddings[0]
		vec32 := make([]float32, len(vec64))
		for i, v := range vec64 {
			vec32[i] = float32(v)
		}

		docs := []VectorDocument{{
			ID:       docID,
			Content:  content,
			Vector:   vec32,
			Metadata: metadata,
		}}

		if err := h.vectorStore.Upsert(ctx, h.collection, docs); err != nil {
			return ErrRAGConnectionFailed.WithCause(err)
		}
		h.logger.Infof("题目 %s 同步成功 (action=%s)", docID, action)

	case "delete":
		if err := h.vectorStore.Delete(ctx, h.collection, []string{docID}); err != nil {
			return ErrRAGConnectionFailed.WithCause(err)
		}
		h.logger.Infof("题目 %s 已从向量库删除", docID)

	default:
		h.logger.Warnf("未知的同步动作: %s, 忽略消息", action)
	}

	return nil
}
