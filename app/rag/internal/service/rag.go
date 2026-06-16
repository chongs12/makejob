package service

import (
	"context"
	"fmt"
	"sync"

	ragv1 "makejob/api/makejob/rag/v1"
	"makejob/app/rag/internal/biz"
)

// RAGService RAG 知识库 gRPC 服务实现
type RAGService struct {
	ragv1.UnimplementedRAGServiceServer
	retrieveUC  *biz.RetrieveUseCase
	indexUC     *biz.IndexUseCase
	syncHandler *biz.SyncHandler
	store       biz.VectorStore
	collection  string
	embedModel  string
	embedDim    int32
	mu          sync.RWMutex
}

// embeddingModelUpdater 描述支持运行时切换 Embedding 模型的数据层能力。
type embeddingModelUpdater interface {
	UpdateEmbeddingModel(ctx context.Context, modelName string) error
}

// NewRAGService 创建 RAG 知识库 gRPC 服务
func NewRAGService(retrieveUC *biz.RetrieveUseCase, indexUC *biz.IndexUseCase, syncHandler *biz.SyncHandler, store biz.VectorStore, collection, embedModel string) *RAGService {
	return &RAGService{
		retrieveUC:  retrieveUC,
		indexUC:     indexUC,
		syncHandler: syncHandler,
		store:       store,
		collection:  collection,
		embedModel:  embedModel,
		embedDim:    1024,
	}
}

// Retrieve 检索相关文档（FIX C7: 透传 filters 参数）
func (s *RAGService) Retrieve(ctx context.Context, req *ragv1.RetrieveRequest) (*ragv1.RetrieveResponse, error) {
	if req.Query == "" {
		return nil, biz.ErrInvalidQuery
	}

	topK := int(req.TopK)
	filters := req.GetFilters()
	if filters == nil {
		filters = make(map[string]string)
	}

	docs, err := s.retrieveUC.Retrieve(ctx, req.Query, topK, filters)
	if err != nil {
		return nil, err
	}

	protoDocs := make([]*ragv1.Document, len(docs))
	for i, doc := range docs {
		protoDocs[i] = toProtoDocument(doc)
	}

	return &ragv1.RetrieveResponse{Documents: protoDocs}, nil
}

// IndexQuestions 索引题目数据
func (s *RAGService) IndexQuestions(ctx context.Context, req *ragv1.IndexQuestionsRequest) (*ragv1.IndexQuestionsResponse, error) {
	if len(req.Items) == 0 {
		return &ragv1.IndexQuestionsResponse{}, nil
	}

	items := make([]biz.IndexItem, len(req.Items))
	for i, item := range req.Items {
		meta := make(map[string]any, len(item.Metadata))
		for k, v := range item.Metadata {
			meta[k] = v
		}
		items[i] = biz.IndexItem{
			ID:       biz.QuestionIDToDocID(uint64(item.QuestionId)),
			Content:  item.Content,
			Metadata: meta,
		}
	}

	indexed, _, failedIDs := s.indexUC.IndexQuestions(ctx, items)

	return &ragv1.IndexQuestionsResponse{
		IndexedCount: int32(indexed),
		FailedIds:    failedIDs,
	}, nil
}

// IndexDocuments 索引文档数据
func (s *RAGService) IndexDocuments(ctx context.Context, req *ragv1.IndexDocumentsRequest) (*ragv1.IndexDocumentsResponse, error) {
	if len(req.Items) == 0 {
		return &ragv1.IndexDocumentsResponse{}, nil
	}

	items := make([]biz.IndexItem, len(req.Items))
	for i, item := range req.Items {
		meta := make(map[string]any, len(item.Metadata))
		for k, v := range item.Metadata {
			meta[k] = v
		}
		if item.Source != "" {
			meta["source"] = item.Source
		}
		items[i] = biz.IndexItem{
			ID:       item.Id,
			Content:  item.Content,
			Metadata: meta,
		}
	}

	indexed, _, failedIDs := s.indexUC.IndexQuestions(ctx, items)

	return &ragv1.IndexDocumentsResponse{
		IndexedCount: int32(indexed),
		FailedIds:    failedIDs,
	}, nil
}

// DeleteIndex 删除索引
func (s *RAGService) DeleteIndex(ctx context.Context, req *ragv1.DeleteIndexRequest) (*ragv1.DeleteIndexResponse, error) {
	if len(req.Ids) == 0 {
		return &ragv1.DeleteIndexResponse{DeletedCount: 0}, nil
	}

	s.mu.RLock()
	collection := s.collection
	s.mu.RUnlock()
	if err := s.store.Delete(ctx, collection, req.Ids); err != nil {
		return nil, err
	}

	return &ragv1.DeleteIndexResponse{DeletedCount: int32(len(req.Ids))}, nil
}

// GetConfig 获取 RAG 配置
func (s *RAGService) GetConfig(ctx context.Context, req *ragv1.GetConfigRequest) (*ragv1.RAGConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &ragv1.RAGConfig{
		CollectionName:     s.collection,
		EmbeddingDimension: s.embedDim,
		EmbeddingModel:     s.embedModel,
	}, nil
}

// UpdateConfig 更新 RAG 运行时配置，并同步到检索/索引/MQ 消费三条链路。
func (s *RAGService) UpdateConfig(ctx context.Context, req *ragv1.UpdateConfigRequest) (*ragv1.RAGConfig, error) {
	collection := req.GetCollectionName()
	if collection == "" {
		s.mu.RLock()
		collection = s.collection
		s.mu.RUnlock()
	}
	embedModel := req.GetEmbeddingModel()
	if embedModel == "" {
		s.mu.RLock()
		embedModel = s.embedModel
		s.mu.RUnlock()
	}
	embedDim := req.GetEmbeddingDimension()
	if embedDim <= 0 {
		s.mu.RLock()
		embedDim = s.embedDim
		s.mu.RUnlock()
	}
	if updater, ok := s.store.(embeddingModelUpdater); ok {
		s.mu.RLock()
		currentModel := s.embedModel
		s.mu.RUnlock()
		if embedModel != currentModel {
			if err := updater.UpdateEmbeddingModel(ctx, embedModel); err != nil {
				return nil, err
			}
		}
	}
	s.mu.Lock()
	s.collection = collection
	s.embedModel = embedModel
	s.embedDim = embedDim
	s.mu.Unlock()
	s.retrieveUC.UpdateCollectionName(collection)
	s.indexUC.UpdateCollectionName(collection)
	if s.syncHandler != nil {
		s.syncHandler.UpdateCollectionName(collection)
	}
	return s.GetConfig(ctx, &ragv1.GetConfigRequest{})
}

// TestConnection 测试向量数据库连接
func (s *RAGService) TestConnection(ctx context.Context, req *ragv1.TestConnectionRequest) (*ragv1.TestConnectionResponse, error) {
	err := s.store.TestConnection(ctx)
	if err != nil {
		return &ragv1.TestConnectionResponse{
			Connected: false,
			Message:   fmt.Sprintf("连接失败: %v", err),
		}, nil
	}
	return &ragv1.TestConnectionResponse{
		Connected: true,
		Message:   "连接正常",
	}, nil
}

// GetDocumentStats 获取文档统计信息（FIX C3/M2: 不伪造时间戳，不混淆文档/题目计数）
func (s *RAGService) GetDocumentStats(ctx context.Context, req *ragv1.GetDocumentStatsRequest) (*ragv1.DocumentStats, error) {
	s.mu.RLock()
	collection := s.collection
	s.mu.RUnlock()
	totalDocs, err := s.store.GetCollectionStats(ctx, collection)
	if err != nil {
		return nil, err
	}

	return &ragv1.DocumentStats{
		TotalDocuments: totalDocs,
		TotalQuestions: totalDocs, // 当前集合仅存储题目，语义上正确
		LastIndexedAt:  nil,       // FIX C3: 不伪造时间戳，无真实索引跟踪时返回 nil
	}, nil
}

// toProtoDocument 将 biz.Document 转换为 proto Document（FIX M1: 保留非 string metadata 值）
func toProtoDocument(doc biz.Document) *ragv1.Document {
	metadata := make(map[string]string, len(doc.MetaData))
	for k, v := range doc.MetaData {
		metadata[k] = fmt.Sprintf("%v", v)
	}
	return &ragv1.Document{
		Id:       doc.ID,
		Content:  doc.Content,
		Score:    float32(doc.Score),
		Metadata: metadata,
	}
}
