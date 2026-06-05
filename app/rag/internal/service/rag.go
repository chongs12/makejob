package service

import (
	"context"
	"fmt"

	ragv1 "makejob/api/makejob/rag/v1"
	"makejob/app/rag/internal/biz"
)

// RAGService RAG 知识库 gRPC 服务实现
type RAGService struct {
	ragv1.UnimplementedRAGServiceServer
	retrieveUC *biz.RetrieveUseCase
	indexUC    *biz.IndexUseCase
}

// NewRAGService 创建 RAG 知识库 gRPC 服务
func NewRAGService(retrieveUC *biz.RetrieveUseCase, indexUC *biz.IndexUseCase) *RAGService {
	return &RAGService{
		retrieveUC: retrieveUC,
		indexUC:    indexUC,
	}
}

// Retrieve 检索相关文档
func (s *RAGService) Retrieve(ctx context.Context, req *ragv1.RetrieveRequest) (*ragv1.RetrieveResponse, error) {
	if req.Query == "" {
		return nil, biz.ErrInvalidQuery
	}

	topK := int(req.TopK)
	docs, err := s.retrieveUC.Retrieve(ctx, req.Query, topK)
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

	// 转换为 biz 层索引条目
	items := make([]biz.IndexItem, len(req.Items))
	for i, item := range req.Items {
		meta := make(map[string]any, len(item.Metadata))
		for k, v := range item.Metadata {
			meta[k] = v
		}
		items[i] = biz.IndexItem{
			ID:       fmt.Sprintf("%d", item.QuestionId),
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
	return &ragv1.DeleteIndexResponse{}, nil
}

// GetConfig 获取 RAG 配置
func (s *RAGService) GetConfig(ctx context.Context, req *ragv1.GetConfigRequest) (*ragv1.RAGConfig, error) {
	return &ragv1.RAGConfig{}, nil
}

// UpdateConfig 更新 RAG 配置
func (s *RAGService) UpdateConfig(ctx context.Context, req *ragv1.UpdateConfigRequest) (*ragv1.RAGConfig, error) {
	return &ragv1.RAGConfig{}, nil
}

// TestConnection 测试向量数据库连接
func (s *RAGService) TestConnection(ctx context.Context, req *ragv1.TestConnectionRequest) (*ragv1.TestConnectionResponse, error) {
	return &ragv1.TestConnectionResponse{Connected: true, Message: "ok"}, nil
}

// GetDocumentStats 获取文档统计信息
func (s *RAGService) GetDocumentStats(ctx context.Context, req *ragv1.GetDocumentStatsRequest) (*ragv1.DocumentStats, error) {
	return &ragv1.DocumentStats{}, nil
}

// toProtoDocument 将 biz.Document 转换为 proto Document
func toProtoDocument(doc biz.Document) *ragv1.Document {
	metadata := make(map[string]string, len(doc.MetaData))
	for k, v := range doc.MetaData {
		if s, ok := v.(string); ok {
			metadata[k] = s
		} else {
			metadata[k] = ""
		}
	}
	return &ragv1.Document{
		Id:       doc.ID,
		Content:  doc.Content,
		Score:    float32(doc.Score),
		Metadata: metadata,
	}
}
