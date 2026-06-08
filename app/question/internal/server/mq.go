package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"google.golang.org/protobuf/types/known/timestamppb"
	adminv1 "makejob/api/makejob/admin/v1"
	ragv1 "makejob/api/makejob/rag/v1"
	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/data"
	"makejob/pkg/auth"
	"makejob/pkg/mq"
)

// MQConsumer handles MQ messages for the question service.
type MQConsumer struct {
	consumer         *mq.Consumer
	uc               *biz.QuestionUseCase
	rag              data.RAGClient
	adminClient      adminv1.AdminServiceClient
	adminAccessToken string
	logger           *log.Helper
}

// NewMQConsumer 创建题目服务的 MQ 消费者，并注入 Admin 回写所需的服务令牌。
func NewMQConsumer(url string, uc *biz.QuestionUseCase, rag data.RAGClient, adminClient adminv1.AdminServiceClient, adminAccessToken string, logger log.Logger) (*MQConsumer, error) {
	consumer, err := mq.NewConsumer(url)
	if err != nil {
		return nil, fmt.Errorf("failed to create mq consumer: %w", err)
	}

	mc := &MQConsumer{
		consumer:         consumer,
		uc:               uc,
		rag:              rag,
		adminClient:      adminClient,
		adminAccessToken: adminAccessToken,
		logger:           log.NewHelper(logger),
	}

	// 注册 RAG 同步题目处理器
	consumer.Register(mq.QueueRAGSyncQuestion, mq.TaskHandlerFunc(mc.handleRAGSyncQuestion))
	// 注册题目 pipeline 构建处理器
	consumer.Register(mq.QueueAdminQuestionPipeline, pipelineBuildTaskHandler{consumer: mc})
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

type pipelineBuildTaskHandler struct {
	consumer *MQConsumer
}

// Handle 执行题目流水线异步任务。
func (h pipelineBuildTaskHandler) Handle(ctx context.Context, msg mq.TaskMessage) error {
	return h.consumer.handlePipelineBuild(ctx, msg)
}

// HandleFinalFailure 在 MQ 重试耗尽后统一回写题目流水线失败状态。
func (h pipelineBuildTaskHandler) HandleFinalFailure(ctx context.Context, msg mq.TaskMessage, lastErr error) error {
	return h.consumer.handlePipelineBuildFinalFailure(ctx, msg, lastErr)
}

// handlePipelineBuild 处理题目 pipeline 构建消息
// 调用 AI Gateway 生成题目并写入题库，同时把进度持续回写到 Admin 任务表。
func (c *MQConsumer) handlePipelineBuild(ctx context.Context, msg mq.TaskMessage) error {
	c.logger.Infof("processing pipeline build message: entity_id=%d", msg.EntityID)

	var payload mq.PipelineBuildPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal pipeline build payload: %w", err)
	}

	if payload.TaskID == 0 {
		return fmt.Errorf("pipeline build: task_id is required")
	}
	if payload.IndustryCode == "" {
		return fmt.Errorf("pipeline build: industry_code is required")
	}
	if payload.Requirement == "" {
		return fmt.Errorf("pipeline build: requirement is required")
	}
	if payload.CandidateCount <= 0 {
		payload.CandidateCount = 5
	}
	startedAt := time.Now()
	startResp, err := c.updateQuestionPipelineTask(ctx, &adminv1.UpdateQuestionPipelineTaskRequest{
		TaskId:        payload.TaskID,
		Status:        "running",
		QuestionCount: payload.CandidateCount,
		ImportedCount: 0,
		StartedAt:     timestamppb.New(startedAt),
	})
	if err != nil {
		return fmt.Errorf("pipeline build start update failed: %w", err)
	}
	if startResp != nil && !startResp.GetApplied() {
		c.logger.Infof("pipeline build skipped because task is already terminal: task_id=%d status=%s", payload.TaskID, startResp.GetTask().GetStatus())
		return nil
	}

	c.logger.Infof("pipeline build: task_id=%d industry=%s count=%d mode=%s",
		payload.TaskID, payload.IndustryCode, payload.CandidateCount, payload.GenerationMode)

	createdQuestions := make([]map[string]string, 0, int(payload.CandidateCount))
	created, err := c.uc.PipelineGenerateQuestions(ctx, &biz.GenerateQuestionsRequest{
		IndustryCode:     payload.IndustryCode,
		Requirement:      payload.Requirement,
		AgentPrompt:      payload.AgentPrompt,
		GenerationMode:   payload.GenerationMode,
		CandidateCount:   payload.CandidateCount,
		IncludeScraped:   payload.IncludeScraped,
		IncludeGenerated: payload.IncludeGenerated,
		Sources:          append([]string(nil), payload.Sources...),
	}, func(current int, question *biz.Question) error {
		createdQuestions = append(createdQuestions, map[string]string{
			"title":      question.Title,
			"type":       question.Type,
			"difficulty": question.Difficulty,
		})
		_, err := c.updateQuestionPipelineTask(ctx, &adminv1.UpdateQuestionPipelineTaskRequest{
			TaskId:        payload.TaskID,
			Status:        "running",
			QuestionCount: payload.CandidateCount,
			ImportedCount: int32(current),
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("pipeline build failed: %w", err)
	}

	resultBytes, err := json.Marshal(map[string]interface{}{
		"task_id":         payload.TaskID,
		"industry_code":   payload.IndustryCode,
		"requirement":     payload.Requirement,
		"generation_mode": payload.GenerationMode,
		"requested_count": payload.CandidateCount,
		"total_generated": created,
		"total_failed":    maxInt32(payload.CandidateCount-int32(created), 0),
		"questions":       createdQuestions,
		"completed_at":    time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("pipeline build result encode failed: %w", err)
	}
	finishedAt := time.Now()
	if _, err := c.updateQuestionPipelineTask(ctx, &adminv1.UpdateQuestionPipelineTaskRequest{
		TaskId:        payload.TaskID,
		Status:        "completed",
		QuestionCount: payload.CandidateCount,
		ImportedCount: int32(created),
		ResultJson:    string(resultBytes),
		FinishedAt:    timestamppb.New(finishedAt),
	}); err != nil {
		return fmt.Errorf("pipeline build completion update failed: %w", err)
	}

	c.logger.Infof("pipeline build completed: task_id=%d, created=%d/%d",
		payload.TaskID, created, payload.CandidateCount)
	return nil
}

// handlePipelineBuildFinalFailure 在 MQ 重试全部失败后统一把任务标记为 failed。
func (c *MQConsumer) handlePipelineBuildFinalFailure(ctx context.Context, msg mq.TaskMessage, lastErr error) error {
	var payload mq.PipelineBuildPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}
	if payload.TaskID == 0 {
		return nil
	}
	finishedAt := time.Now()
	_, err := c.updateQuestionPipelineTask(ctx, &adminv1.UpdateQuestionPipelineTaskRequest{
		TaskId:        payload.TaskID,
		Status:        "failed",
		QuestionCount: payload.CandidateCount,
		ErrorMsg:      lastErr.Error(),
		FinishedAt:    timestamppb.New(finishedAt),
	})
	return err
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

// updateQuestionPipelineTask 调用 Admin 服务回写题目流水线任务状态。
func (c *MQConsumer) updateQuestionPipelineTask(ctx context.Context, req *adminv1.UpdateQuestionPipelineTaskRequest) (*adminv1.UpdateQuestionPipelineTaskResponse, error) {
	if c.adminClient == nil {
		return nil, fmt.Errorf("admin client not configured")
	}
	return c.adminClient.UpdateQuestionPipelineTask(c.adminRequestContext(ctx), req)
}

// adminRequestContext 为 Admin 回写 RPC 构造带认证信息的出站上下文。
func (c *MQConsumer) adminRequestContext(ctx context.Context) context.Context {
	token := auth.GetAccessTokenFromContext(ctx)
	if token == "" {
		token = auth.GetAccessTokenFromMetadata(ctx)
	}
	if token != "" {
		return auth.WithOutgoingAccessToken(ctx, token)
	}
	return auth.WithOutgoingAccessToken(ctx, c.adminAccessToken)
}

// maxInt32 返回两个 int32 中的较大值，供结果统计字段复用。
func maxInt32(left int32, right int32) int32 {
	if left > right {
		return left
	}
	return right
}
