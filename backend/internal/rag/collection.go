package rag

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
)

const (
	// DefaultEmbeddingDim 豆包Embedding默认输出维度
	// doubao-embedding-large-text-240915 输出 4096 维向量
	DefaultEmbeddingDim = 4096

	// FieldNameID 主键字段名
	FieldNameID = "id"
	// FieldNameContent 内容字段名
	FieldNameContent = "content"
	// FieldNameVector 向量字段名
	FieldNameVector = "vector"
	// FieldNameMetadata 元数据字段名
	FieldNameMetadata = "metadata"
)

// EnsureCollection 确保Collection存在，不存在则创建并建立索引。
// Schema:
//   - id: VarChar(64) [主键]
//   - content: VarChar(8192)
//   - vector: FloatVector(4096)
//   - metadata: JSON
func EnsureCollection(ctx context.Context, client *milvusclient.Client, collection string, dim int) error {
	if dim <= 0 {
		dim = DefaultEmbeddingDim
	}

	has, err := client.HasCollection(ctx, milvusclient.NewHasCollectionOption(collection))
	if err != nil {
		return fmt.Errorf("检查Collection是否存在失败: %w", err)
	}

	if has {
		applogger.Info("Collection已存在，跳过创建", zap.String("collection", collection))
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

	applogger.Info("Collection创建成功", zap.String("collection", collection), zap.Int("dim", dim))

	// 为vector字段创建AUTOINDEX索引
	autoIndex := index.NewAutoIndex(entity.COSINE)
	_, err = client.CreateIndex(ctx, milvusclient.NewCreateIndexOption(collection, FieldNameVector, autoIndex))
	if err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	applogger.Info("索引创建成功", zap.String("collection", collection))

	// 加载Collection到内存
	loadTask, err := client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(collection))
	if err != nil {
		return fmt.Errorf("加载Collection失败: %w", err)
	}
	_ = loadTask

	applogger.Info("Collection加载成功", zap.String("collection", collection))
	return nil
}
