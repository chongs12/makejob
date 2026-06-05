package biz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

// mockEmbedder 模拟文本向量化器
type mockEmbedder struct {
	embedFunc func(ctx context.Context, texts []string) ([][]float64, error)
}

func (m *mockEmbedder) EmbedStrings(ctx context.Context, texts []string) ([][]float64, error) {
	return m.embedFunc(ctx, texts)
}

// mockVectorStore 模拟向量存储
type mockVectorStore struct {
	searchFunc func(ctx context.Context, vector []float32, topK int, collection string) ([]Document, error)
	upsertFunc func(ctx context.Context, collection string, docs []VectorDocument) error
	deleteFunc func(ctx context.Context, collection string, ids []string) error
}

func (m *mockVectorStore) Search(ctx context.Context, vector []float32, topK int, collection string) ([]Document, error) {
	return m.searchFunc(ctx, vector, topK, collection)
}

func (m *mockVectorStore) Upsert(ctx context.Context, collection string, docs []VectorDocument) error {
	return m.upsertFunc(ctx, collection, docs)
}

func (m *mockVectorStore) Delete(ctx context.Context, collection string, ids []string) error {
	return m.deleteFunc(ctx, collection, ids)
}

func TestRetrieveUseCase_Retrieve_Success(t *testing.T) {
	embedder := &mockEmbedder{
		embedFunc: func(ctx context.Context, texts []string) ([][]float64, error) {
			return [][]float64{{0.1, 0.2, 0.3}}, nil
		},
	}
	store := &mockVectorStore{
		searchFunc: func(ctx context.Context, vector []float32, topK int, collection string) ([]Document, error) {
			return []Document{
				{ID: "1", Content: "Go并发编程", Score: 0.95},
				{ID: "2", Content: "Go基础语法", Score: 0.85},
			}, nil
		},
	}
	uc := NewRetrieveUseCase(embedder, store, "test_collection", 5, log.DefaultLogger)

	docs, err := uc.Retrieve(context.Background(), "Go并发", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
	if docs[0].ID != "1" {
		t.Errorf("expected first doc ID='1', got '%s'", docs[0].ID)
	}
}

func TestRetrieveUseCase_Retrieve_EmbeddingFailed(t *testing.T) {
	embedder := &mockEmbedder{
		embedFunc: func(ctx context.Context, texts []string) ([][]float64, error) {
			return nil, ErrEmbeddingFailed
		},
	}
	store := &mockVectorStore{}
	uc := NewRetrieveUseCase(embedder, store, "test_collection", 5, log.DefaultLogger)

	_, err := uc.Retrieve(context.Background(), "query", 5)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRetrieveUseCase_Retrieve_NoResults(t *testing.T) {
	embedder := &mockEmbedder{
		embedFunc: func(ctx context.Context, texts []string) ([][]float64, error) {
			return [][]float64{{0.1, 0.2}}, nil
		},
	}
	store := &mockVectorStore{
		searchFunc: func(ctx context.Context, vector []float32, topK int, collection string) ([]Document, error) {
			return nil, nil
		},
	}
	uc := NewRetrieveUseCase(embedder, store, "test_collection", 5, log.DefaultLogger)

	_, err := uc.Retrieve(context.Background(), "query", 5)
	if err != ErrNoResults {
		t.Errorf("expected ErrNoResults, got %v", err)
	}
}

func TestIndexUseCase_IndexQuestions_Success(t *testing.T) {
	embedder := &mockEmbedder{
		embedFunc: func(ctx context.Context, texts []string) ([][]float64, error) {
			result := make([][]float64, len(texts))
			for i := range texts {
				result[i] = []float64{0.1, 0.2}
			}
			return result, nil
		},
	}
	store := &mockVectorStore{
		upsertFunc: func(ctx context.Context, collection string, docs []VectorDocument) error {
			return nil
		},
	}
	uc := NewIndexUseCase(embedder, store, "test_collection", log.DefaultLogger)

	items := []IndexItem{
		{ID: "1", Content: "题目1"},
		{ID: "2", Content: "题目2"},
	}
	indexed, failed, failedIDs := uc.IndexQuestions(context.Background(), items)
	if indexed != 2 {
		t.Errorf("expected indexed=2, got %d", indexed)
	}
	if failed != 0 {
		t.Errorf("expected failed=0, got %d", failed)
	}
	if len(failedIDs) != 0 {
		t.Errorf("expected no failed IDs, got %v", failedIDs)
	}
}

func TestIndexUseCase_IndexQuestions_EmbeddingFailed(t *testing.T) {
	embedder := &mockEmbedder{
		embedFunc: func(ctx context.Context, texts []string) ([][]float64, error) {
			return nil, ErrEmbeddingFailed
		},
	}
	store := &mockVectorStore{
		upsertFunc: func(ctx context.Context, collection string, docs []VectorDocument) error {
			return nil
		},
	}
	uc := NewIndexUseCase(embedder, store, "test_collection", log.DefaultLogger)

	items := []IndexItem{
		{ID: "1", Content: "题目1"},
		{ID: "2", Content: "题目2"},
	}
	indexed, failed, failedIDs := uc.IndexQuestions(context.Background(), items)
	if indexed != 0 {
		t.Errorf("expected indexed=0, got %d", indexed)
	}
	if failed != 2 {
		t.Errorf("expected failed=2, got %d", failed)
	}
	if len(failedIDs) != 2 {
		t.Errorf("expected 2 failed IDs, got %v", failedIDs)
	}
}
