package rag

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
)

const (
	// embedBatchSize Embedding API单次最大处理条数
	embedBatchSize = 64
)

// milvusIndexer Milvus向量索引实现
type milvusIndexer struct {
	client     *milvusclient.Client
	embedder   embedding.Embedder
	collection string
	dim        int
}

// NewMilvusIndexer 创建Milvus Indexer实例
func NewMilvusIndexer(client *milvusclient.Client, embedder embedding.Embedder, collection string, dim int) Indexer {
	if dim <= 0 {
		dim = DefaultEmbeddingDim
	}
	return &milvusIndexer{
		client:     client,
		embedder:   embedder,
		collection: collection,
		dim:        dim,
	}
}

// Index 批量写入文档到Milvus
func (i *milvusIndexer) Index(ctx context.Context, docs []IndexDocument) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(docs))
	allContents := make([]string, 0, len(docs))
	for _, doc := range docs {
		allContents = append(allContents, doc.Content)
		ids = append(ids, doc.ID)
	}

	// 分批Embedding
	allVectors, err := i.batchEmbed(ctx, allContents)
	if err != nil {
		return nil, fmt.Errorf("批量Embedding失败: %w", err)
	}

	// 构建列数据
	idColumn := column.NewColumnVarChar(FieldNameID, ids)
	contentColumn := column.NewColumnVarChar(FieldNameContent, allContents)
	vectorColumn := column.NewColumnFloatVector(FieldNameVector, i.dim, allVectors)

	// 构建JSON元数据列
	metadataBytes := make([][]byte, 0, len(docs))
	for _, doc := range docs {
		if doc.MetaData == nil {
			metadataBytes = append(metadataBytes, []byte("{}"))
			continue
		}
		data, err := json.Marshal(doc.MetaData)
		if err != nil {
			metadataBytes = append(metadataBytes, []byte("{}"))
			continue
		}
		metadataBytes = append(metadataBytes, data)
	}
	metadataColumn := column.NewColumnJSONBytes(FieldNameMetadata, metadataBytes)

	// 批量插入
	_, err = i.client.Insert(ctx, milvusclient.NewColumnBasedInsertOption(
		i.collection, idColumn, contentColumn, vectorColumn, metadataColumn,
	))
	if err != nil {
		return nil, fmt.Errorf("插入Milvus失败: %w", err)
	}

	applogger.Info("RAG索引写入成功",
		zap.Int("count", len(docs)),
		zap.String("collection", i.collection),
	)

	return ids, nil
}

// Delete 根据ID列表删除文档
func (i *milvusIndexer) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// 构建删除表达式: id in ["id1", "id2", ...]
	expr := fmt.Sprintf("%s in [", FieldNameID)
	for idx, id := range ids {
		if idx > 0 {
			expr += ", "
		}
		expr += fmt.Sprintf("%q", id)
	}
	expr += "]"

	_, err := i.client.Delete(ctx, milvusclient.NewDeleteOption(i.collection).WithExpr(expr))
	if err != nil {
		return fmt.Errorf("删除Milvus文档失败: %w", err)
	}

	applogger.Info("RAG索引删除成功",
		zap.Int("count", len(ids)),
		zap.String("collection", i.collection),
	)

	return nil
}

// batchEmbed 分批调用Embedding API
func (i *milvusIndexer) batchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	allVectors := make([][]float32, 0, len(texts))

	for start := 0; start < len(texts); start += embedBatchSize {
		end := start + embedBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[start:end]

		vectors, err := i.embedder.EmbedStrings(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("Embedding批次[%d:%d]失败: %w", start, end, err)
		}

		// float64转float32（Milvus要求）
		for _, vec := range vectors {
			allVectors = append(allVectors, convertToFloat32(vec))
		}
	}

	return allVectors, nil
}

// convertToFloat32 float64向量转float32
func convertToFloat32(src []float64) []float32 {
	dst := make([]float32, len(src))
	for i, v := range src {
		dst[i] = float32(v)
	}
	return dst
}
