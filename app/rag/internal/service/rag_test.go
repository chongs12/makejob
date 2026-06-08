package service

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	ragv1 "makejob/api/makejob/rag/v1"
	"makejob/app/rag/internal/biz"
)

// serviceEmbedderStub 为 RAG 服务测试提供空向量化器。
type serviceEmbedderStub struct{}

// EmbedStrings 返回固定向量，满足用例构造依赖。
func (s *serviceEmbedderStub) EmbedStrings(context.Context, []string) ([][]float64, error) {
	return [][]float64{{0.1, 0.2}}, nil
}

// serviceVectorStoreStub 为 RAG 服务测试提供可观察的向量库桩。
type serviceVectorStoreStub struct {
	lastUpdatedModel string
}

// Search 返回空结果，满足接口要求。
func (s *serviceVectorStoreStub) Search(context.Context, []float32, int, string, map[string]string) ([]biz.Document, error) {
	return nil, nil
}

// Upsert 返回空结果，满足接口要求。
func (s *serviceVectorStoreStub) Upsert(context.Context, string, []biz.VectorDocument) error {
	return nil
}

// Delete 返回空结果，满足接口要求。
func (s *serviceVectorStoreStub) Delete(context.Context, string, []string) error {
	return nil
}

// TestConnection 返回空结果，满足接口要求。
func (s *serviceVectorStoreStub) TestConnection(context.Context) error {
	return nil
}

// GetCollectionStats 返回空统计，满足接口要求。
func (s *serviceVectorStoreStub) GetCollectionStats(context.Context, string) (int64, error) {
	return 0, nil
}

// UpdateEmbeddingModel 记录最新模型名，模拟运行时切换。
func (s *serviceVectorStoreStub) UpdateEmbeddingModel(_ context.Context, modelName string) error {
	s.lastUpdatedModel = modelName
	return nil
}

// TestUpdateConfigRefreshesRuntimeState 验证 UpdateConfig 会同步刷新服务与三条业务链路的运行时配置。
func TestUpdateConfigRefreshesRuntimeState(t *testing.T) {
	store := &serviceVectorStoreStub{}
	embedder := &serviceEmbedderStub{}
	retrieveUC := biz.NewRetrieveUseCase(embedder, store, "old_collection", 5, log.DefaultLogger)
	indexUC := biz.NewIndexUseCase(embedder, store, "old_collection", log.DefaultLogger)
	syncHandler := biz.NewSyncHandler(embedder, store, "old_collection", log.DefaultLogger)
	svc := NewRAGService(retrieveUC, indexUC, syncHandler, store, "old_collection", "old-model")

	resp, err := svc.UpdateConfig(context.Background(), &ragv1.UpdateConfigRequest{
		CollectionName:     "new_collection",
		EmbeddingDimension: 1536,
		EmbeddingModel:     "new-model",
	})
	if err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}
	if resp.GetCollectionName() != "new_collection" {
		t.Fatalf("expected collection_name to be updated, got %s", resp.GetCollectionName())
	}
	if resp.GetEmbeddingDimension() != 1536 {
		t.Fatalf("expected embedding_dimension=1536, got %d", resp.GetEmbeddingDimension())
	}
	if resp.GetEmbeddingModel() != "new-model" {
		t.Fatalf("expected embedding_model to be updated, got %s", resp.GetEmbeddingModel())
	}
	if retrieveUC.CollectionName() != "new_collection" {
		t.Fatalf("expected retrieve usecase to use new collection, got %s", retrieveUC.CollectionName())
	}
	if indexUC.CollectionName() != "new_collection" {
		t.Fatalf("expected index usecase to use new collection, got %s", indexUC.CollectionName())
	}
	if syncHandler.CollectionName() != "new_collection" {
		t.Fatalf("expected sync handler to use new collection, got %s", syncHandler.CollectionName())
	}
	if store.lastUpdatedModel != "new-model" {
		t.Fatalf("expected store embedder model to be refreshed, got %s", store.lastUpdatedModel)
	}
}
