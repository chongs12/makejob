package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	ragv1 "makejob/api/makejob/rag/v1"
	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/data"
	"makejob/pkg/mq"
)

// MQConsumer handles MQ messages for the question service.
type MQConsumer struct {
	consumer *mq.Consumer
	uc       *biz.QuestionUseCase
	rag      data.RAGClient
	logger   *log.Helper
}

// NewMQConsumer creates a new MQ consumer for question-related queues.
func NewMQConsumer(url string, uc *biz.QuestionUseCase, rag data.RAGClient, logger log.Logger) (*MQConsumer, error) {
	consumer, err := mq.NewConsumer(url)
	if err != nil {
		return nil, fmt.Errorf("failed to create mq consumer: %w", err)
	}

	mc := &MQConsumer{
		consumer: consumer,
		uc:       uc,
		rag:      rag,
		logger:   log.NewHelper(logger),
	}

	// 注册 RAG 同步题目处理器
	consumer.Register(mq.QueueRAGSyncQuestion, mq.TaskHandlerFunc(mc.handleRAGSyncQuestion))
	// 注册题目 pipeline 构建处理器
	consumer.Register(mq.QueueAdminQuestionPipeline, mq.TaskHandlerFunc(mc.handlePipelineBuild))
	// 注册 scraper 导入题目处理器
	consumer.Register(mq.QueueScraperImport, mq.TaskHandlerFunc(mc.handleScraperImport))

	return mc, nil
}

// Start starts the MQ consumer.
func (c *MQConsumer) Start(ctx context.Context) error {
	return c.consumer.Start(ctx)
}

// Stop stops the MQ consumer.
func (c *MQConsumer) Stop(ctx context.Context) error {
	return c.consumer.Stop(ctx)
}

// ragSyncPayload RAG 同步题目消息负载
type ragSyncPayload struct {
	QuestionID uint64            `json:"question_id"`
	Action     string            `json:"action"` // create | update | delete
	Content    string            `json:"content"`
	Metadata   map[string]string `json:"metadata"`
}

// handleRAGSyncQuestion 处理 RAG 同步题目消息
// create/update → 调用 RAG.IndexQuestions 索引题目
// delete → 调用 RAG.DeleteIndex 删除索引
func (c *MQConsumer) handleRAGSyncQuestion(ctx context.Context, msg mq.TaskMessage) error {
	c.logger.Infof("processing RAG sync question: entity_id=%d", msg.EntityID)

	var payload ragSyncPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal RAG sync payload: %w", err)
	}

	// 使用 entity_id 作为 fallback
	if payload.QuestionID == 0 {
		payload.QuestionID = msg.EntityID
	}

	if c.rag == nil {
		c.logger.Warnf("RAG client not configured, skipping sync for question_id=%d", payload.QuestionID)
		return nil
	}

	switch payload.Action {
	case "create", "update":
		if payload.Content == "" {
			c.logger.Warnf("RAG sync: content empty for question_id=%d, skipping", payload.QuestionID)
			return nil
		}
		items := []*ragv1.IndexItem{
			{
				QuestionId: payload.QuestionID,
				Content:    payload.Content,
				Metadata:   payload.Metadata,
			},
		}
		indexed, err := c.rag.IndexQuestions(ctx, items)
		if err != nil {
			return fmt.Errorf("RAG index failed for question_id=%d: %w", payload.QuestionID, err)
		}
		c.logger.Infof("RAG sync: indexed question_id=%d, count=%d", payload.QuestionID, indexed)

	case "delete":
		ids := []string{fmt.Sprintf("%d", payload.QuestionID)}
		deleted, err := c.rag.DeleteIndex(ctx, ids)
		if err != nil {
			return fmt.Errorf("RAG delete failed for question_id=%d: %w", payload.QuestionID, err)
		}
		c.logger.Infof("RAG sync: deleted question_id=%d, count=%d", payload.QuestionID, deleted)

	default:
		c.logger.Warnf("RAG sync: unknown action=%s for question_id=%d, skipping", payload.Action, payload.QuestionID)
		return nil
	}

	return nil
}

// handlePipelineBuild 处理题目 pipeline 构建消息
// 调用 AI Gateway 生成题目并写入题库
func (c *MQConsumer) handlePipelineBuild(ctx context.Context, msg mq.TaskMessage) error {
	c.logger.Infof("processing pipeline build message: entity_id=%d", msg.EntityID)

	var payload mq.PipelineBuildPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal pipeline build payload: %w", err)
	}

	if payload.IndustryCode == "" {
		return fmt.Errorf("pipeline build: industry_code is required")
	}

	// 设置默认值
	if payload.Count <= 0 {
		payload.Count = 5
	}
	if payload.Difficulty == "" {
		payload.Difficulty = "medium"
	}

	c.logger.Infof("pipeline build: industry=%s, difficulty=%s, count=%d",
		payload.IndustryCode, payload.Difficulty, payload.Count)

	// 调用 biz 层生成题目
	created, err := c.uc.PipelineGenerateQuestions(ctx, &biz.GenerateQuestionsRequest{
		IndustryCode: payload.IndustryCode,
		Difficulty:   payload.Difficulty,
		Count:        payload.Count,
		Topics:       payload.Topics,
	})
	if err != nil {
		return fmt.Errorf("pipeline build failed: %w", err)
	}

	c.logger.Infof("pipeline build completed: pipeline_id=%d, created=%d/%d",
		payload.PipelineID, created, payload.Count)
	return nil
}

// handleScraperImport 处理 scraper 导入题目消息
// 从爬虫系统获取题目数据并导入到题库
func (c *MQConsumer) handleScraperImport(ctx context.Context, msg mq.TaskMessage) error {
	c.logger.Infof("processing scraper import message: entity_type=%s, entity_id=%d", msg.EntityType, msg.EntityID)

	// 解析导入数据
	var payload struct {
		Source       string `json:"source"`
		IndustryCode string `json:"industry_code"`
		Questions    []struct {
			Title      string `json:"title"`
			Content    string `json:"content"`
			Type       string `json:"type"`
			Difficulty string `json:"difficulty"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal scraper import payload: %w", err)
	}

	// 转换为领域实体并导入
	questions := make([]*biz.Question, len(payload.Questions))
	for i, q := range payload.Questions {
		questions[i] = &biz.Question{
			Title:        q.Title,
			Content:      q.Content,
			Type:         q.Type,
			Difficulty:   q.Difficulty,
			IndustryCode: payload.IndustryCode,
		}
	}

	imported, err := c.uc.ImportQuestions(ctx, questions)
	if err != nil {
		return fmt.Errorf("failed to import questions: %w", err)
	}

	c.logger.Infof("scraper import completed: source=%s, imported=%d/%d", payload.Source, imported, len(questions))
	return nil
}
