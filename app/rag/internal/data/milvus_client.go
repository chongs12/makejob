package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	ark_embed "github.com/cloudwego/eino-ext/components/embedding/ark"

	"makejob/app/rag/internal/biz"
	"makejob/app/rag/internal/conf"
)

// milvusClient 同时实现 Embedder 和 VectorStore 接口
type milvusClient struct {
	embedder  *ark_embed.Embedder
	client    *milvusclient.Client
	logger    *log.Helper
	apiKey    string
	baseURL   string
	modelName string
	mu        sync.RWMutex
}

// 集合字段常量（对齐单体 collection.go）
const (
	DefaultEmbeddingDim = 4096
	FieldNameID         = "id"
	FieldNameContent    = "content"
	FieldNameVector     = "vector"
	FieldNameMetadata   = "metadata"
)

// EnsureCollection 确保 Collection 存在，不存在则创建并建立索引（对齐单体 collection.go）
func EnsureCollection(ctx context.Context, client *milvusclient.Client, collection string, dim int) error {
	if dim <= 0 {
		dim = DefaultEmbeddingDim
	}

	has, err := client.HasCollection(ctx, milvusclient.NewHasCollectionOption(collection))
	if err != nil {
		return fmt.Errorf("检查Collection是否存在失败: %w", err)
	}

	if has {
		return nil
	}

	schema := entity.NewSchema().
		WithField(entity.NewField().WithName(FieldNameID).WithDataType(entity.FieldTypeVarChar).WithIsPrimaryKey(true).WithMaxLength(64)).
		WithField(entity.NewField().WithName(FieldNameContent).WithDataType(entity.FieldTypeVarChar).WithMaxLength(8192)).
		WithField(entity.NewField().WithName(FieldNameVector).WithDataType(entity.FieldTypeFloatVector).WithDim(int64(dim))).
		WithField(entity.NewField().WithName(FieldNameMetadata).WithDataType(entity.FieldTypeJSON))

	err = client.CreateCollection(ctx, milvusclient.NewCreateCollectionOption(collection, schema))
	if err != nil {
		return fmt.Errorf("创建Collection失败: %w", err)
	}

	autoIndex := index.NewAutoIndex(entity.COSINE)
	_, err = client.CreateIndex(ctx, milvusclient.NewCreateIndexOption(collection, FieldNameVector, autoIndex))
	if err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	loadTask, err := client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(collection))
	if err != nil {
		return fmt.Errorf("加载Collection失败: %w", err)
	}
	_ = loadTask

	return nil
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
		return nil, errors.ServiceUnavailable("EMBEDDING_INIT_FAILED", "初始化Ark Embedder失败")
	}
	log.Info("Ark Embedder 初始化成功")

	// 初始化 Milvus 客户端（对齐单体：支持认证 + 10s 超时）
	connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
	defer connectCancel()

	client, err := milvusclient.New(connectCtx, &milvusclient.ClientConfig{
		Address:  cfg.MilvusAddr,
		Username: cfg.MilvusUser,
		Password: cfg.MilvusPassword,
	})
	if err != nil {
		return nil, errors.ServiceUnavailable("RAG_CONNECTION_FAILED", "连接Milvus失败")
	}
	log.Infof("Milvus 客户端连接成功: %s", cfg.MilvusAddr)

	// 确保 Collection 存在（对齐单体 EnsureCollection）
	if err := EnsureCollection(ctx, client, cfg.CollectionName, 0); err != nil {
		client.Close(ctx)
		return nil, errors.ServiceUnavailable("RAG_COLLECTION_INIT_FAILED", "初始化Collection失败")
	}

	return &milvusClient{
		embedder:  embedder,
		client:    client,
		logger:    log,
		apiKey:    cfg.ArkAPIKey,
		baseURL:   cfg.ArkBaseURL,
		modelName: cfg.EmbedModel,
	}, nil
}

// EmbedStrings 调用 Volcengine Ark Embedding API 进行文本向量化
func (c *milvusClient) EmbedStrings(ctx context.Context, texts []string) ([][]float64, error) {
	ctx, span := otel.Tracer("makejob.rag").Start(ctx, "ark.embed",
		trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("llm.provider", "volcengine_ark"),
		attribute.String("embed.model", c.modelName),
		attribute.Int("embed.text_count", len(texts)),
	)
	c.mu.RLock()
	embedder := c.embedder
	c.mu.RUnlock()
	res, err := embedder.EmbedStrings(ctx, texts)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return res, nil
}

// UpdateEmbeddingModel 运行时更新 Embedding 模型，并重建 Ark Embedder。
func (c *milvusClient) UpdateEmbeddingModel(ctx context.Context, modelName string) error {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return errors.BadRequest("INVALID_EMBEDDING_MODEL", "embedding model 不能为空")
	}
	embedder, err := ark_embed.NewEmbedder(ctx, &ark_embed.EmbeddingConfig{
		APIKey:  c.apiKey,
		Model:   modelName,
		BaseURL: c.baseURL,
	})
	if err != nil {
		return errors.ServiceUnavailable("EMBEDDING_INIT_FAILED", "重建Ark Embedder失败")
	}
	c.mu.Lock()
	c.embedder = embedder
	c.modelName = modelName
	c.mu.Unlock()
	return nil
}

// Search 在 Milvus 中进行向量相似度搜索（对齐单体 retriever.go：WithANNSField + 双重 metadata 解析）
func (c *milvusClient) Search(ctx context.Context, vector []float32, topK int, collection string, filters map[string]string) ([]biz.Document, error) {
	ctx, span := otel.Tracer("makejob.rag").Start(ctx, "milvus.search",
		trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "milvus"),
		attribute.String("db.collection", collection),
		attribute.Int("db.top_k", topK),
		attribute.Int("db.filter_count", len(filters)),
	)

	searchOpt := milvusclient.NewSearchOption(collection, topK, []entity.Vector{
		entity.FloatVector(vector),
	}).
		WithANNSField(FieldNameVector).
		WithOutputFields(FieldNameID, FieldNameContent, FieldNameMetadata)

	// 构建 Milvus 过滤表达式
	if len(filters) > 0 {
		filterExpr := buildMilvusFilterExpr(filters)
		searchOpt = searchOpt.WithFilter(filterExpr)
	}

	resultSets, err := c.client.Search(ctx, searchOpt)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
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
		if contentCol := rs.GetColumn(FieldNameContent); contentCol != nil {
			if v, err := contentCol.GetAsString(i); err == nil {
				doc.Content = v
			}
		}

		// 提取 metadata（对齐单体：兼容 []byte 和 string 两种类型）
		if metaCol := rs.GetColumn(FieldNameMetadata); metaCol != nil {
			if v, err := metaCol.Get(i); err == nil {
				switch metaRaw := v.(type) {
				case []byte:
					if len(metaRaw) > 0 {
						var meta map[string]any
						if json.Unmarshal(metaRaw, &meta) == nil {
							doc.MetaData = meta
						}
					}
				case string:
					if len(metaRaw) > 0 {
						var meta map[string]any
						if json.Unmarshal([]byte(metaRaw), &meta) == nil {
							doc.MetaData = meta
						}
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

	span.SetAttributes(attribute.Int("db.results_count", len(docs)))
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
		column.NewColumnVarChar(FieldNameID, ids),
		column.NewColumnVarChar(FieldNameContent, contents),
		column.NewColumnFloatVector(FieldNameVector, dim, vectors),
		column.NewColumnJSONBytes(FieldNameMetadata, metadataBytes),
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

	opt := milvusclient.NewDeleteOption(collection).WithStringIDs(FieldNameID, ids)

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

// TestConnection 测试 Milvus 连接是否可用
func (c *milvusClient) TestConnection(ctx context.Context) error {
	// 通过列出集合来验证连接是否正常
	_, err := c.client.ListCollections(ctx, milvusclient.NewListCollectionOption())
	if err != nil {
		return errors.ServiceUnavailable("RAG_CONNECTION_FAILED", fmt.Sprintf("Milvus 连接测试失败: %v", err))
	}
	return nil
}

// GetCollectionStats 获取集合的文档统计
func (c *milvusClient) GetCollectionStats(ctx context.Context, collection string) (int64, error) {
	stats, err := c.client.GetCollectionStats(ctx, milvusclient.NewGetCollectionStatsOption(collection))
	if err != nil {
		return 0, errors.ServiceUnavailable("RAG_CONNECTION_FAILED", fmt.Sprintf("获取集合统计失败: %v", err))
	}
	// stats 是一个 map[string]string，通常包含 "row_count"
	if rowCountStr, ok := stats["row_count"]; ok {
		var count int64
		if _, err := fmt.Sscanf(rowCountStr, "%d", &count); err == nil {
			return count, nil
		}
	}
	return 0, nil
}

// buildMilvusFilterExpr 将 filters map 转换为 Milvus 过滤表达式
// 例如 {"type": "question", "difficulty": "hard"} → "metadata[\"type\"] == \"question\" and metadata[\"difficulty\"] == \"hard\""
func buildMilvusFilterExpr(filters map[string]string) string {
	if len(filters) == 0 {
		return ""
	}
	var parts []string
	for k, v := range filters {
		parts = append(parts, fmt.Sprintf("metadata[\"%s\"] == \"%s\"", k, v))
	}
	return strings.Join(parts, " and ")
}
