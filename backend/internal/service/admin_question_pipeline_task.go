package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/mq"
	"makejob-backend/internal/scraper"
)

// CreateQuestionPipelineTask 创建一条待执行的异步题目流水线生成任务，供后台先入队、再由 worker 统一消费。
func (s *adminService) CreateQuestionPipelineTask(ctx context.Context, req *AdminQuestionPipelineGenerateRequest) (*model.ScraperTask, error) {
	if s.scraperTaskRepo == nil {
		return nil, fmt.Errorf("question pipeline task repository is required")
	}

	payloadJSON, normalizedReq, err := buildQuestionPipelineTaskPayload(req)
	if err != nil {
		return nil, err
	}
	if _, _, err := s.loadPipelineIndustryContext(ctx, normalizedReq.IndustryCode); err != nil {
		return nil, err
	}

	task := &model.ScraperTask{
		TaskType:      scraper.TaskTypeQuestionPipelineBuild,
		SourceURL:     "manual://question-pipeline",
		SourceTitle:   buildQuestionPipelineTaskTitle(normalizedReq.Requirement),
		Source:        "manual",
		Status:        scraper.TaskStatusPending,
		PayloadJSON:   payloadJSON,
		QuestionCount: normalizeQuestionPipelineCount(normalizedReq.CandidateCount),
	}
	if err := s.scraperTaskRepo.Create(ctx, task); err != nil {
		return nil, err
	}
	if err := s.publishQuestionPipelineTask(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

// RunNextPendingQuestionPipelineTask 领取并执行下一条待处理的题目流水线生成任务，供独立 worker 轮询消费。
func (s *adminService) RunNextPendingQuestionPipelineTask(ctx context.Context) (*model.ScraperTask, bool, error) {
	if s.scraperTaskRepo == nil {
		return nil, false, fmt.Errorf("question pipeline task repository is required")
	}

	task, err := s.scraperTaskRepo.ClaimNextPending(ctx, scraper.TaskTypeQuestionPipelineBuild)
	if err != nil {
		return nil, false, err
	}
	if task == nil {
		return nil, false, nil
	}

	executeErr := s.executeQuestionPipelineTask(ctx, task)
	return task, true, executeErr
}

// buildQuestionPipelineTaskPayload 序列化题目流水线任务载荷，并完成最基础的字段清洗与必填校验。
func buildQuestionPipelineTaskPayload(req *AdminQuestionPipelineGenerateRequest) (string, *AdminQuestionPipelineGenerateRequest, error) {
	if req == nil {
		return "", nil, common.NewBusinessError(common.CodeBadRequest, "pipeline request cannot be empty")
	}

	normalized := &AdminQuestionPipelineGenerateRequest{
		IndustryCode:     strings.TrimSpace(req.IndustryCode),
		Requirement:      strings.TrimSpace(req.Requirement),
		AgentPrompt:      strings.TrimSpace(req.AgentPrompt),
		GenerationMode:   normalizeQuestionPipelineGenerationMode(req.GenerationMode),
		CandidateCount:   normalizeQuestionPipelineCount(req.CandidateCount),
		IncludeScraped:   req.IncludeScraped,
		IncludeGenerated: req.IncludeGenerated,
		Sources:          make([]string, 0, len(req.Sources)),
	}
	if normalized.IndustryCode == "" {
		return "", nil, common.NewBusinessError(common.CodeBadRequest, "industry code is required")
	}
	if normalized.Requirement == "" {
		return "", nil, common.NewBusinessError(common.CodeBadRequest, "requirement is required")
	}
	if !normalized.IncludeScraped && !normalized.IncludeGenerated {
		normalized.IncludeScraped = true
		normalized.IncludeGenerated = true
	}

	seenSources := make(map[string]bool, len(req.Sources))
	for _, source := range req.Sources {
		trimmed := strings.TrimSpace(source)
		if trimmed == "" || seenSources[trimmed] {
			continue
		}
		seenSources[trimmed] = true
		normalized.Sources = append(normalized.Sources, trimmed)
	}

	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", nil, fmt.Errorf("序列化题目流水线任务载荷失败: %w", err)
	}
	return string(payload), normalized, nil
}

// decodeQuestionPipelineTaskPayload 反序列化题目流水线任务载荷，供 worker 执行时恢复原始生成请求。
func decodeQuestionPipelineTaskPayload(raw string) (*AdminQuestionPipelineGenerateRequest, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("empty payload")
	}

	var req AdminQuestionPipelineGenerateRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// buildQuestionPipelineTaskTitle 根据岗位要求生成易读的任务标题，便于后台任务页快速定位来源。
func buildQuestionPipelineTaskTitle(requirement string) string {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" {
		return "题目流水线候选生成"
	}

	runes := []rune(requirement)
	if len(runes) <= 36 {
		return requirement
	}
	return strings.TrimSpace(string(runes[:36]))
}

// executeQuestionPipelineTask 执行已领取的题目流水线生成任务，并将候选题卡结果写回任务表。
func (s *adminService) executeQuestionPipelineTask(ctx context.Context, task *model.ScraperTask) error {
	req, err := decodeQuestionPipelineTaskPayload(task.PayloadJSON)
	if err != nil {
		return s.failQuestionPipelineTask(ctx, task, fmt.Errorf("解析题目流水线任务载荷失败: %w", err))
	}

	taskCtx := withAsyncTaskID(ctx, task.ID)
	result, runErr := s.GenerateQuestionPipeline(taskCtx, req)
	if runErr != nil {
		return s.failQuestionPipelineTask(ctx, task, runErr)
	}

	resultJSON, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return s.failQuestionPipelineTask(ctx, task, fmt.Errorf("序列化题目流水线任务结果失败: %w", marshalErr))
	}

	now := time.Now()
	task.Status = scraper.TaskStatusSucceeded
	task.FinishedAt = &now
	task.ResultJSON = string(resultJSON)
	task.QuestionCount = len(result.Cards)
	task.ErrorMsg = ""
	if err := s.scraperTaskRepo.Update(ctx, task); err != nil {
		return err
	}
	return nil
}

// ProcessQuestionPipelineTask 按任务 ID 执行题目流水线任务，供 RabbitMQ 消费者按消息直接消费。
func (s *adminService) ProcessQuestionPipelineTask(ctx context.Context, taskID uint) error {
	if s.scraperTaskRepo == nil {
		return fmt.Errorf("question pipeline task repository is required")
	}

	task, shouldRun, err := s.scraperTaskRepo.ClaimByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil || !shouldRun {
		return nil
	}

	return s.executeQuestionPipelineTask(ctx, task)
}

// failQuestionPipelineTask 将执行失败的题目流水线任务回写为 failed，确保后台可以直接看到失败原因。
func (s *adminService) failQuestionPipelineTask(ctx context.Context, task *model.ScraperTask, taskErr error) error {
	now := time.Now()
	task.Status = scraper.TaskStatusFailed
	task.FinishedAt = &now
	task.ErrorMsg = taskErr.Error()
	if err := s.scraperTaskRepo.Update(ctx, task); err != nil {
		return err
	}
	return taskErr
}

// publishQuestionPipelineTask 在启用 RabbitMQ 时把后台题目流水线任务投递到异步队列。
func (s *adminService) publishQuestionPipelineTask(ctx context.Context, task *model.ScraperTask) error {
	if !s.asyncEnabled || s.taskPublisher == nil || task == nil {
		return nil
	}

	spec, ok := mq.QueueSpecByTaskType(mq.TaskTypeAdminQuestionPipeline)
	if !ok {
		return fmt.Errorf("未找到题目流水线队列配置")
	}

	payloadBytes, err := json.Marshal(mq.AdminQuestionPipelinePayload{ScraperTaskID: task.ID})
	if err != nil {
		return fmt.Errorf("序列化题目流水线投递消息失败: %w", err)
	}

	message := buildAsyncTaskMessage(
		mq.TaskTypeAdminQuestionPipeline,
		0,
		"scraper_task",
		task.ID,
		"admin-service",
		fmt.Sprintf("question-pipeline:%d", task.ID),
		payloadBytes,
	)
	if err := s.taskPublisher.PublishTask(ctx, spec.RoutingKey, message); err != nil {
		return fmt.Errorf("投递题目流水线任务失败: %w", err)
	}
	return nil
}
