package rag

import (
	"context"
	"fmt"

	applogger "makejob-backend/pkg/logger"
	"makejob-backend/internal/model"

	"go.uber.org/zap"
)

// Service RAG服务，串联Indexer + Retriever
type Service struct {
	indexer   Indexer
	retriever Retriever
	config    Config
}

// NewService 创建RAG Service实例
func NewService(indexer Indexer, retriever Retriever, config Config) *Service {
	return &Service{
		indexer:   indexer,
		retriever: retriever,
		config:    config,
	}
}

// IndexQuestions 批量索引题目到向量库
func (s *Service) IndexQuestions(ctx context.Context, questions []model.Question) error {
	if len(questions) == 0 {
		return nil
	}

	docs := BuildQuestionDocuments(questions)
	_, err := s.indexer.Index(ctx, docs)
	if err != nil {
		return fmt.Errorf("索引题目失败: %w", err)
	}

	applogger.Info("RAG题目索引完成",
		zap.Int("count", len(questions)),
		zap.String("collection", s.config.Collection),
	)
	return nil
}

// RetrieveByQuery 根据查询语义检索相关文档
func (s *Service) RetrieveByQuery(ctx context.Context, query string, topK int) ([]Document, error) {
	docs, err := s.retriever.Retrieve(ctx, query, topK)
	if err != nil {
		return nil, fmt.Errorf("语义检索失败: %w", err)
	}

	applogger.Debug("RAG语义检索完成",
		zap.String("query", query),
		zap.Int("results", len(docs)),
	)
	return docs, nil
}

// DeleteByIDs 删除指定文档
func (s *Service) DeleteByIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	err := s.indexer.Delete(ctx, ids)
	if err != nil {
		return fmt.Errorf("删除文档失败: %w", err)
	}

	applogger.Info("RAG文档删除完成",
		zap.Int("count", len(ids)),
		zap.String("collection", s.config.Collection),
	)
	return nil
}

// Indexer 获取Indexer实例（供外部直接调用）
func (s *Service) Indexer() Indexer {
	return s.indexer
}

// Retriever 获取Retriever实例（供外部直接调用）
func (s *Service) Retriever() Retriever {
	return s.retriever
}
