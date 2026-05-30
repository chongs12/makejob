package rag

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"makejob-backend/internal/model"
	"makejob-backend/internal/mq"
	"makejob-backend/internal/repository"
	applogger "makejob-backend/pkg/logger"
)

// SyncConsumer RAG同步消费者，处理题目增删改事件
type SyncConsumer struct {
	service *Service
	repo    repository.QuestionRepository
}

// NewSyncConsumer 创建RAG同步消费者
func NewSyncConsumer(service *Service, repo repository.QuestionRepository) *SyncConsumer {
	return &SyncConsumer{
		service: service,
		repo:    repo,
	}
}

// Handle 处理RAG同步消息
func (c *SyncConsumer) Handle(ctx context.Context, payload []byte) error {
	var msg mq.RAGSyncPayload
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("解析RAG同步消息失败: %w", err)
	}

	if msg.QuestionID == 0 {
		return fmt.Errorf("RAG同步消息缺少question_id")
	}

	switch msg.Action {
	case "index", "update":
		return c.handleIndex(ctx, msg.QuestionID)
	case "delete":
		return c.handleDelete(ctx, msg.QuestionID)
	default:
		return fmt.Errorf("未知的RAG同步动作: %s", msg.Action)
	}
}

// handleIndex 处理索引/更新操作
func (c *SyncConsumer) handleIndex(ctx context.Context, questionID uint) error {
	question, err := c.repo.GetByID(ctx, questionID)
	if err != nil {
		return fmt.Errorf("查询题目失败: %w", err)
	}
	if question == nil {
		applogger.Warn("RAG同步: 题目不存在，跳过", zap.Uint("question_id", questionID))
		return nil
	}

	docs := BuildQuestionDocuments([]model.Question{*question})
	_, err = c.service.Indexer().Index(ctx, docs)
	if err != nil {
		return fmt.Errorf("索引题目失败: %w", err)
	}

	applogger.Info("RAG同步: 题目索引成功",
		zap.Uint("question_id", questionID),
	)
	return nil
}

// handleDelete 处理删除操作
func (c *SyncConsumer) handleDelete(ctx context.Context, questionID uint) error {
	docID := QuestionIDToDocID(questionID)
	err := c.service.DeleteByIDs(ctx, []string{docID})
	if err != nil {
		return fmt.Errorf("删除题目索引失败: %w", err)
	}

	applogger.Info("RAG同步: 题目索引删除成功",
		zap.Uint("question_id", questionID),
	)
	return nil
}
