package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"google.golang.org/protobuf/types/known/timestamppb"
	adminv1 "makejob/api/makejob/admin/v1"
	"makejob/app/question/internal/biz"
	"makejob/pkg/auth"
	"makejob/pkg/mq"
)

// MQConsumer handles MQ messages for the question service.
type MQConsumer struct {
	consumer         *mq.Consumer
	uc               *biz.QuestionUseCase
	adminClient      adminv1.AdminServiceClient
	adminAccessToken string
	logger           *log.Helper
}

// NewMQConsumer 创建题目服务的 MQ 消费者，并注入 Admin 回写所需的服务令牌。
func NewMQConsumer(url string, uc *biz.QuestionUseCase, adminClient adminv1.AdminServiceClient, adminAccessToken string, logger log.Logger) (*MQConsumer, error) {
	consumer, err := mq.NewConsumer(url)
	if err != nil {
		return nil, fmt.Errorf("failed to create mq consumer: %w", err)
	}

	mc := &MQConsumer{
		consumer:         consumer,
		uc:               uc,
		adminClient:      adminClient,
		adminAccessToken: adminAccessToken,
		logger:           log.NewHelper(logger),
	}

	// 注册题目 pipeline 构建处理器
	consumer.Register(mq.QueueAdminQuestionPipeline, pipelineBuildTaskHandler{consumer: mc})
	// 注册 scraper 导入题目处理器
	consumer.Register(mq.QueueScraperImport, scraperImportTaskHandler{consumer: mc})

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

// scraperImportTaskHandler 包装 scraper 导入任务处理，并在最终失败时统一回写任务状态。
type scraperImportTaskHandler struct {
	consumer *MQConsumer
}

// Handle 执行 scraper 导入任务的单次消费。
func (h scraperImportTaskHandler) Handle(ctx context.Context, msg mq.TaskMessage) error {
	return h.consumer.handleScraperImport(ctx, msg)
}

// HandleFinalFailure 在 scraper 导入消息重试耗尽后统一标记任务失败。
func (h scraperImportTaskHandler) HandleFinalFailure(ctx context.Context, msg mq.TaskMessage, lastErr error) error {
	return h.consumer.handleScraperImportFinalFailure(ctx, msg, lastErr)
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

	var payload mq.ScraperImportPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal scraper import payload: %w", err)
	}
	if payload.TaskID == 0 {
		payload.TaskID = msg.EntityID
	}
	if payload.TaskID == 0 {
		return fmt.Errorf("scraper import: task_id is required")
	}
	startedAt := time.Now()
	startResp, err := c.updateQuestionPipelineTask(ctx, &adminv1.UpdateQuestionPipelineTaskRequest{
		TaskId:        payload.TaskID,
		Status:        "running",
		QuestionCount: int32(len(payload.Questions)),
		ImportedCount: 0,
		StartedAt:     timestamppb.New(startedAt),
	})
	if err != nil {
		return fmt.Errorf("scraper import start update failed: %w", err)
	}
	if startResp != nil && !startResp.GetApplied() {
		c.logger.Infof("scraper import skipped because task is already terminal: task_id=%d status=%s", payload.TaskID, startResp.GetTask().GetStatus())
		return nil
	}

	// 转换为领域实体并导入
	questions := make([]*biz.Question, len(payload.Questions))
	for i, q := range payload.Questions {
		questions[i] = &biz.Question{
			Title:           q.Title,
			Content:         q.Content,
			Type:            q.Type,
			Difficulty:      q.Difficulty,
			IndustryCode:    payload.IndustryCode,
			CategoryName:    q.CategoryName,
			OptionsJSON:     q.OptionsJSON,
			Answer:          q.Answer,
			ReferenceAnswer: q.Answer,
			Explanation:     q.Explanation,
			Tags:            splitScraperImportTags(q.Tags),
		}
	}

	imported, err := c.uc.ImportQuestions(ctx, questions)
	if err != nil {
		return fmt.Errorf("failed to import questions: %w", err)
	}
	resultBytes, err := json.Marshal(map[string]interface{}{
		"task_id":         payload.TaskID,
		"source":          payload.Source,
		"source_url":      payload.SourceURL,
		"source_title":    payload.SourceTitle,
		"industry_code":   payload.IndustryCode,
		"requested_count": len(payload.Questions),
		"imported_count":  imported,
		"completed_at":    time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("scraper import result encode failed: %w", err)
	}
	finishedAt := time.Now()
	if _, err := c.updateQuestionPipelineTask(ctx, &adminv1.UpdateQuestionPipelineTaskRequest{
		TaskId:        payload.TaskID,
		Status:        "completed",
		QuestionCount: int32(len(payload.Questions)),
		ImportedCount: int32(imported),
		ResultJson:    string(resultBytes),
		FinishedAt:    timestamppb.New(finishedAt),
	}); err != nil {
		return fmt.Errorf("scraper import completion update failed: %w", err)
	}

	c.logger.Infof("scraper import completed: source=%s, imported=%d/%d", payload.Source, imported, len(questions))
	return nil
}

// handleScraperImportFinalFailure 在 scraper 导入任务最终失败后统一回写失败状态。
func (c *MQConsumer) handleScraperImportFinalFailure(ctx context.Context, msg mq.TaskMessage, lastErr error) error {
	var payload mq.ScraperImportPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}
	if payload.TaskID == 0 {
		payload.TaskID = msg.EntityID
	}
	if payload.TaskID == 0 {
		return nil
	}
	finishedAt := time.Now()
	_, err := c.updateQuestionPipelineTask(ctx, &adminv1.UpdateQuestionPipelineTaskRequest{
		TaskId:        payload.TaskID,
		Status:        "failed",
		QuestionCount: int32(len(payload.Questions)),
		ErrorMsg:      lastErr.Error(),
		FinishedAt:    timestamppb.New(finishedAt),
	})
	return err
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

// splitScraperImportTags 将 MQ 里的逗号标签字符串还原为领域切片。
func splitScraperImportTags(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}

// maxInt32 返回两个 int32 中的较大值，供结果统计字段复用。
func maxInt32(left int32, right int32) int32 {
	if left > right {
		return left
	}
	return right
}
