package rag

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// milvusRetriever Milvus语义检索实现
type milvusRetriever struct {
	client      *milvusclient.Client
	embedder    embedding.Embedder
	collection  string
	defaultTopK int
}

// NewMilvusRetriever 创建Milvus Retriever实例
func NewMilvusRetriever(client *milvusclient.Client, embedder embedding.Embedder, collection string, defaultTopK int) Retriever {
	if defaultTopK <= 0 {
		defaultTopK = 5
	}
	return &milvusRetriever{
		client:      client,
		embedder:    embedder,
		collection:  collection,
		defaultTopK: defaultTopK,
	}
}

// Retrieve 语义检索
func (r *milvusRetriever) Retrieve(ctx context.Context, query string, topK int) ([]Document, error) {
	if topK <= 0 {
		topK = r.defaultTopK
	}

	// 将查询文本转为向量
	vectors, err := r.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("查询Embedding失败: %w", err)
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return nil, fmt.Errorf("查询Embedding返回空向量")
	}

	queryVector := convertToFloat32(vectors[0])

	// 构建搜索请求
	searchOpt := milvusclient.NewSearchOption(r.collection, topK, []entity.Vector{
		entity.FloatVector(queryVector),
	}).
		WithANNSField(FieldNameVector).
		WithOutputFields(FieldNameID, FieldNameContent, FieldNameMetadata)

	// 执行搜索
	resultSets, err := r.client.Search(ctx, searchOpt)
	if err != nil {
		return nil, fmt.Errorf("Milvus搜索失败: %w", err)
	}
	if len(resultSets) == 0 {
		return nil, nil
	}

	// 解析结果
	return r.parseResults(resultSets[0])
}

// parseResults 解析Milvus搜索结果
func (r *milvusRetriever) parseResults(rs milvusclient.ResultSet) ([]Document, error) {
	if rs.Err != nil {
		return nil, fmt.Errorf("搜索结果错误: %w", rs.Err)
	}

	idColumn := rs.GetColumn(FieldNameID)
	contentColumn := rs.GetColumn(FieldNameContent)
	metadataColumn := rs.GetColumn(FieldNameMetadata)
	scores := rs.Scores

	docs := make([]Document, 0, len(scores))

	for i := 0; i < len(scores); i++ {
		doc := Document{
			Score: float64(scores[i]),
		}

		// 提取ID
		if idColumn != nil {
			if id, err := idColumn.GetAsString(i); err == nil {
				doc.ID = id
			}
		}

		// 提取Content
		if contentColumn != nil {
			if content, err := contentColumn.GetAsString(i); err == nil {
				doc.Content = content
			}
		}

		// 提取Metadata
		if metadataColumn != nil {
			if metaRaw, err := metadataColumn.Get(i); err == nil {
				switch v := metaRaw.(type) {
				case []byte:
					if len(v) > 0 {
						var meta map[string]any
						if jsonErr := json.Unmarshal(v, &meta); jsonErr == nil {
							doc.MetaData = meta
						}
					}
				case string:
					if len(v) > 0 {
						var meta map[string]any
						if jsonErr := json.Unmarshal([]byte(v), &meta); jsonErr == nil {
							doc.MetaData = meta
						}
					}
				}
			}
		}

		docs = append(docs, doc)
	}

	return docs, nil
}
