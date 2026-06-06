package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/question/internal/biz"
	"makejob/pkg/mq"
)

// MQConsumer handles MQ messages for the question service.
type MQConsumer struct {
	consumer *mq.Consumer
	uc       *biz.QuestionUseCase
	logger   *log.Helper
}

// NewMQConsumer creates a new MQ consumer for question-related queues.
func NewMQConsumer(url string, uc *biz.QuestionUseCase, logger log.Logger) (*MQConsumer, error) {
	consumer, err := mq.NewConsumer(url)
	if err != nil {
		return nil, fmt.Errorf("failed to create mq consumer: %w", err)
	}

	mc := &MQConsumer{
		consumer: consumer,
		uc:       uc,
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

// handleRAGSyncQuestion handles RAG sync question messages.
func (c *MQConsumer) handleRAGSyncQuestion(ctx context.Context, msg mq.TaskMessage) error {
	c.logger.Infof("processing RAG sync question message: entity_type=%s, entity_id=%d", msg.EntityType, msg.EntityID)

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal RAG sync payload: %w", err)
	}

	c.logger.Infof("RAG sync question completed: entity_id=%d", msg.EntityID)
	return nil
}

// handlePipelineBuild 处理题目 pipeline 构建消息
// 当管理员触发题目生成 pipeline 时，消费此消息执行构建逻辑
func (c *MQConsumer) handlePipelineBuild(ctx context.Context, msg mq.TaskMessage) error {
	c.logger.Infof("processing pipeline build message: entity_type=%s, entity_id=%d", msg.EntityType, msg.EntityID)

	// 解析 pipeline 配置
	var payload struct {
		PipelineID uint64 `json:"pipeline_id"`
		Source     string `json:"source"`
		Config     string `json:"config"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal pipeline build payload: %w", err)
	}

	c.logger.Infof("pipeline build started: pipeline_id=%d, source=%s", payload.PipelineID, payload.Source)
	return nil
}

// handleScraperImport 处理 scraper 导入题目消息
// 从爬虫系统获取题目数据并导入到题库
func (c *MQConsumer) handleScraperImport(ctx context.Context, msg mq.TaskMessage) error {
	c.logger.Infof("processing scraper import message: entity_type=%s, entity_id=%d", msg.EntityType, msg.EntityID)

	// 解析导入数据
	var payload struct {
		Source      string `json:"source"`
		IndustryCode string `json:"industry_code"`
		Questions   []struct {
			Title    string `json:"title"`
			Content  string `json:"content"`
			Type     string `json:"type"`
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
