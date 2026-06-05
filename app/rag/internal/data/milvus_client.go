package data

import (
	"context"
	"encoding/json"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	ark_embed "github.com/cloudwego/eino-ext/components/embedding/ark"

	"makejob/app/rag/internal/biz"
	"makejob/app/rag/internal/conf"
)

// milvusClient 同时实现 Embedder 和 VectorStore 接口
type milvusClient struct {
	embedder *ark_embed.Embedder
	client   *milvusclient.Client
	logger   *log.Helper
}

// NewMilvusClient 创建 Milvus+Ark 集成客户端，同时实现 Embedder 和 VectorStore
func NewMilvusClient(ctx context.Context, cfg *conf.RAG, logger log.Logger) (*milvusClient, error) {
	log := log.NewHelper(logger)

	// 初始化 Ark Embedder
	embedder, err := ark_embed.NewEmbedder(ctx, &ark_embed.EmbeddingConfig{
		APIKey:  cfg.ArkAPIKey,
		Model:   cfg.EmbedModel,
		BaseURL: cfg.ArkBaseURL,
	})
	if err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.ServiceUnavailable("EMBEDDING_INIT_FAILED", "初始化Ark Embedder失败")
	}
	log.Info("Ark Embedder 初始化成功")

	// 初始化 Milvus 客户端
	client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: cfg.MilvusAddr,
	})
	if err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.ServiceUnavailable("RAG_CONNECTION_FAILED", "连接Milvus失败")
	}
	log.Infof("Milvus 客户端连接成功: %s", cfg.MilvusAddr)

	return &milvusClient{
		embedder: embedder,
		client:   client,
		logger:   log,
	}, nil
}

// EmbedStrings 调用 Volcengine Ark Embedding API 进行文本向量化
func (c *milvusClient) EmbedStrings(ctx context.Context, texts []string) ([][]float64, error) {
	return c.embedder.EmbedStrings(ctx, texts)
}

// Search 在 Milvus 中进行向量相似度搜索
func (c *milvusClient) Search(ctx context.Context, vector []float32, topK int, collection string) ([]biz.Document, error) {
	searchOpt := milvusclient.NewSearchOption(collection, topK, []entity.Vector{
		entity.FloatVector(vector),
	}).
		WithOutputFields("id", "content", "metadata")

	resultSets, err := c.client.Search(ctx, searchOpt)
	if err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.ServiceUnavailable("RAG_CONNECTION_FAILED", "Milvus搜索失败")
	}
	if len(resultSets) == 0 {
		return nil, nil
	}

	rs := resultSets[0]
	docs := make([]biz.Document, 0, rs.ResultCount)
	for i := 0; i < rs.ResultCount; i++ {
		doc := biz.Document{}

		// 提取 id
		if rs.IDs != nil {
			if v, err := rs.IDs.GetAsString(i); err == nil {
				doc.ID = v
			}
		}

		// 提取 content
		if contentCol := rs.GetColumn("content"); contentCol != nil {
			if v, err := contentCol.GetAsString(i); err == nil {
				doc.Content = v
			}
		}

		// 提取 metadata
		if metaCol := rs.GetColumn("metadata"); metaCol != nil {
			if v, err := metaCol.Get(i); err == nil {
				if metaBytes, ok := v.([]byte); ok {
					var meta map[string]any
					if json.Unmarshal(metaBytes, &meta) == nil {
						doc.MetaData = meta
					}
				}
			}
		}

		// 提取分数
		if i < len(rs.Scores) {
			doc.Score = float64(rs.Scores[i])
		}

		docs = append(docs, doc)
	}

	return docs, nil
}

// Upsert 批量写入或更新向量文档到 Milvus
func (c *milvusClient) Upsert(ctx context.Context, collection string, docs []biz.VectorDocument) error {
	if len(docs) == 0 {
		return nil
	}

	ids := make([]string, len(docs))
	contents := make([]string, len(docs))
	vectors := make([][]float32, len(docs))
	metadataBytes := make([][]byte, len(docs))

	for i, doc := range docs {
		ids[i] = doc.ID
		contents[i] = doc.Content
		vectors[i] = doc.Vector
		if doc.Metadata != nil {
			bs, err := json.Marshal(doc.Metadata)
			if err != nil {
				// FIX: 替换fmt.Errorf为kratos errors
				return errors.InternalServer("RAG_METADATA_SERIALIZE_FAILED", "序列化metadata失败")
			}
			metadataBytes[i] = bs
		} else {
			metadataBytes[i] = []byte("{}")
		}
	}

	dim := 0
	if len(vectors) > 0 {
		dim = len(vectors[0])
	}

	opt := milvusclient.NewColumnBasedInsertOption(collection,
		column.NewColumnVarChar("id", ids),
		column.NewColumnVarChar("content", contents),
		column.NewColumnFloatVector("vector", dim, vectors),
		column.NewColumnJSONBytes("metadata", metadataBytes),
	)

	_, err := c.client.Upsert(ctx, opt)
	if err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return errors.ServiceUnavailable("RAG_CONNECTION_FAILED", "Milvus Upsert失败")
	}

	c.logger.Infof("成功写入 %d 条向量文档到 %s", len(docs), collection)
	return nil
}

// Delete 从 Milvus 中删除指定 ID 的文档
func (c *milvusClient) Delete(ctx context.Context, collection string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	opt := milvusclient.NewDeleteOption(collection).WithStringIDs("id", ids)

	_, err := c.client.Delete(ctx, opt)
	if err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return errors.ServiceUnavailable("RAG_CONNECTION_FAILED", "Milvus Delete失败")
	}

	c.logger.Infof("成功从 %s 删除 %d 条文档", collection, len(ids))
	return nil
}

// Close 关闭 Milvus 客户端连接
func (c *milvusClient) Close() error {
	if c.client != nil {
		return c.client.Close(context.Background())
	}
	return nil
}
