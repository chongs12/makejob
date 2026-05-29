// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
	"makejob-backend/internal/mq"
)

const interviewAsyncEntityType = "mock_interview"

// ProcessInterviewResumeParseTask 消费简历解析任务，并在成功后把面试推进到可开始状态。
func (s *interviewService) ProcessInterviewResumeParseTask(ctx context.Context, asyncTaskID uint) error {
	if s.asyncTaskRepo == nil {
		return fmt.Errorf("async task repository is required")
	}
	if s.interviewRepo == nil {
		return fmt.Errorf("interview repository is required")
	}

	asyncTask, shouldRun, err := s.asyncTaskRepo.ClaimByID(ctx, asyncTaskID)
	if err != nil {
		return err
	}
	if asyncTask == nil || !shouldRun {
		return nil
	}

	payload, err := decodeInterviewResumeParsePayload(asyncTask.PayloadJSON)
	if err != nil {
		return s.failInterviewAsyncTask(ctx, asyncTask, fmt.Errorf("解析简历任务载荷失败: %w", err), false)
	}

	interview, err := s.interviewRepo.GetByID(ctx, payload.InterviewID)
	if err != nil {
		return s.failInterviewAsyncTask(ctx, asyncTask, err, true)
	}
	if interview == nil {
		return s.failInterviewAsyncTask(ctx, asyncTask, fmt.Errorf("面试记录不存在: %d", payload.InterviewID), false)
	}

	if err := s.parseResumeAndActivateInterview(ctx, interview, payload.ResumeText, payload.JobDescription); err != nil {
		return s.failInterviewAsyncTask(ctx, asyncTask, err, true)
	}

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"interview_id": interview.ID,
		"status":       interview.Status,
	})
	now := time.Now()
	asyncTask.Status = model.AsyncTaskStatusSucceeded
	asyncTask.ResultJSON = string(resultJSON)
	asyncTask.ErrorMsg = ""
	asyncTask.FinishedAt = &now
	return s.asyncTaskRepo.Update(ctx, asyncTask)
}

// ProcessInterviewReportTask 消费面试报告任务，并完成评分补齐、报告生成和学习档案沉淀。
func (s *interviewService) ProcessInterviewReportTask(ctx context.Context, asyncTaskID uint) error {
	if s.asyncTaskRepo == nil {
		return fmt.Errorf("async task repository is required")
	}
	if s.interviewRepo == nil {
		return fmt.Errorf("interview repository is required")
	}

	asyncTask, shouldRun, err := s.asyncTaskRepo.ClaimByID(ctx, asyncTaskID)
	if err != nil {
		return err
	}
	if asyncTask == nil || !shouldRun {
		return nil
	}

	payload, err := decodeInterviewReportPayload(asyncTask.PayloadJSON)
	if err != nil {
		return s.failInterviewAsyncTask(ctx, asyncTask, fmt.Errorf("解析面试报告任务载荷失败: %w", err), false)
	}

	interview, err := s.interviewRepo.GetByID(ctx, payload.InterviewID)
	if err != nil {
		return s.failInterviewAsyncTask(ctx, asyncTask, err, true)
	}
	if interview == nil {
		return s.failInterviewAsyncTask(ctx, asyncTask, fmt.Errorf("面试记录不存在: %d", payload.InterviewID), false)
	}

	report, duration, completedAt, err := s.generateAndPersistInterviewReport(ctx, interview, payload.SessionID)
	if err != nil {
		return s.failInterviewAsyncTask(ctx, asyncTask, err, true)
	}

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"interview_id":      interview.ID,
		"status":            interview.Status,
		"duration_seconds":  duration,
		"completed_at_unix": completedAt.Unix(),
		"overall_score":     report.OverallScore,
	})
	now := time.Now()
	asyncTask.Status = model.AsyncTaskStatusSucceeded
	asyncTask.ResultJSON = string(resultJSON)
	asyncTask.ErrorMsg = ""
	asyncTask.FinishedAt = &now
	return s.asyncTaskRepo.Update(ctx, asyncTask)
}

// enqueueInterviewResumeParseTask 创建并投递简历解析任务，供创建简历驱动面试时异步执行。
func (s *interviewService) enqueueInterviewResumeParseTask(ctx context.Context, interview *model.MockInterview, req *CreateInterviewRequest) (*model.AsyncTask, error) {
	if interview == nil || req == nil {
		return nil, fmt.Errorf("interview and request are required")
	}
	payload := mq.InterviewResumeParsePayload{
		InterviewID:    interview.ID,
		UserID:         interview.UserID,
		IndustryCode:   strings.TrimSpace(req.IndustryCode),
		ResumeText:     strings.TrimSpace(req.ResumeText),
		JobDescription: strings.TrimSpace(req.JobDescription),
		InterviewMode:  strings.TrimSpace(req.InterviewMode),
		Live2DModelKey: strings.TrimSpace(req.Live2DModelKey),
		QuestionCount:  req.QuestionCount,
		Difficulty:     strings.TrimSpace(req.Difficulty),
	}
	return s.enqueueInterviewAsyncTask(ctx, mq.TaskTypeInterviewResumeParse, interview, "interview-service", buildInterviewResumeParseIdempotencyKey(interview.ID), payload)
}

// enqueueInterviewReportTask 创建并投递面试报告任务，供结束面试时异步执行。
func (s *interviewService) enqueueInterviewReportTask(ctx context.Context, interview *model.MockInterview, sessionID string) (*model.AsyncTask, error) {
	if interview == nil {
		return nil, fmt.Errorf("interview is required")
	}
	payload := mq.InterviewReportPayload{
		InterviewID: interview.ID,
		UserID:      interview.UserID,
		SessionID:   strings.TrimSpace(sessionID),
	}
	return s.enqueueInterviewAsyncTask(ctx, mq.TaskTypeInterviewReportGenerate, interview, "interview-service", buildInterviewReportIdempotencyKey(interview.ID), payload)
}

// enqueueInterviewAsyncTask 统一封装面试领域异步任务的建表与发布动作。
func (s *interviewService) enqueueInterviewAsyncTask(ctx context.Context, taskType string, interview *model.MockInterview, source string, idempotencyKey string, payload interface{}) (*model.AsyncTask, error) {
	if s.asyncTaskRepo == nil || s.taskPublisher == nil {
		return nil, fmt.Errorf("async dispatch dependencies are incomplete")
	}

	spec, ok := mq.QueueSpecByTaskType(taskType)
	if !ok {
		return nil, fmt.Errorf("未找到队列配置: %s", taskType)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化异步任务载荷失败: %w", err)
	}

	asyncTask, err := s.prepareInterviewAsyncTask(ctx, spec, interview, source, idempotencyKey, payloadBytes)
	if err != nil {
		return nil, err
	}
	if asyncTask.Status != model.AsyncTaskStatusPending {
		return asyncTask, nil
	}

	message := buildAsyncTaskMessage(taskType, asyncTask.ID, asyncTask.EntityType, asyncTask.EntityID, asyncTask.Source, asyncTask.IdempotencyKey, payloadBytes)
	message.MaxRetries = asyncTask.MaxRetries
	message.Attempt = asyncTask.RetryCount
	if err := s.taskPublisher.PublishTask(ctx, spec.RoutingKey, message); err != nil {
		asyncTask.Status = model.AsyncTaskStatusFailed
		asyncTask.ErrorMsg = err.Error()
		asyncTask.FinishedAt = nil
		_ = s.asyncTaskRepo.Update(ctx, asyncTask)
		return nil, err
	}

	now := time.Now()
	asyncTask.Status = model.AsyncTaskStatusQueued
	asyncTask.PublishedAt = &now
	asyncTask.ErrorMsg = ""
	asyncTask.FinishedAt = nil
	if err := s.asyncTaskRepo.Update(ctx, asyncTask); err != nil {
		return nil, err
	}
	return asyncTask, nil
}

// prepareInterviewAsyncTask 创建或重置一条可重复投递的面试异步任务记录。
func (s *interviewService) prepareInterviewAsyncTask(ctx context.Context, spec mq.QueueSpec, interview *model.MockInterview, source string, idempotencyKey string, payloadBytes []byte) (*model.AsyncTask, error) {
	existingTask, err := s.asyncTaskRepo.GetByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		return nil, err
	}
	if existingTask != nil {
		switch existingTask.Status {
		case model.AsyncTaskStatusQueued, model.AsyncTaskStatusRunning, model.AsyncTaskStatusSucceeded:
			return existingTask, nil
		}
	}

	asyncTask := &model.AsyncTask{
		TaskType:       spec.TaskType,
		Source:         source,
		Status:         model.AsyncTaskStatusPending,
		QueueName:      spec.QueueName,
		RoutingKey:     spec.RoutingKey,
		EntityType:     interviewAsyncEntityType,
		EntityID:       interview.ID,
		IdempotencyKey: idempotencyKey,
		PayloadJSON:    string(payloadBytes),
		MaxRetries:     spec.MaxRetries,
	}
	if existingTask == nil {
		if err := s.asyncTaskRepo.Create(ctx, asyncTask); err != nil {
			return nil, err
		}
		return asyncTask, nil
	}

	existingTask.TaskType = spec.TaskType
	existingTask.Source = source
	existingTask.Status = model.AsyncTaskStatusPending
	existingTask.QueueName = spec.QueueName
	existingTask.RoutingKey = spec.RoutingKey
	existingTask.EntityType = interviewAsyncEntityType
	existingTask.EntityID = interview.ID
	existingTask.PayloadJSON = string(payloadBytes)
	existingTask.ResultJSON = ""
	existingTask.MaxRetries = spec.MaxRetries
	existingTask.ErrorMsg = ""
	existingTask.PublishedAt = nil
	existingTask.StartedAt = nil
	existingTask.FinishedAt = nil
	if err := s.asyncTaskRepo.Update(ctx, existingTask); err != nil {
		return nil, err
	}
	return existingTask, nil
}

// failInterviewAsyncTask 回写面试异步任务失败状态，并根据错误类型决定是否继续等待重试。
func (s *interviewService) failInterviewAsyncTask(ctx context.Context, asyncTask *model.AsyncTask, taskErr error, retryable bool) error {
	now := time.Now()
	asyncTask.ErrorMsg = taskErr.Error()
	asyncTask.FinishedAt = &now
	if retryable && asyncTask.RetryCount < asyncTask.MaxRetries {
		asyncTask.Status = model.AsyncTaskStatusQueued
	} else if retryable && asyncTask.RetryCount >= asyncTask.MaxRetries {
		asyncTask.Status = model.AsyncTaskStatusDead
	} else {
		asyncTask.Status = model.AsyncTaskStatusFailed
	}
	if err := s.asyncTaskRepo.Update(ctx, asyncTask); err != nil {
		return err
	}
	return taskErr
}

// parseResumeAndActivateInterview 解析简历画像并把实时面试从 preparing 切换到 ongoing。
func (s *interviewService) parseResumeAndActivateInterview(ctx context.Context, interview *model.MockInterview, resumeText string, jobDescription string) error {
	if interview == nil {
		return fmt.Errorf("interview is nil")
	}
	metadata := parseRealtimeInterviewMetadata(interview.AIFeedback, interview.TotalQuestions)
	if s.resumeParser != nil {
		profile, err := s.resumeParser.Parse(ctx, resumeText, jobDescription)
		if err != nil {
			return fmt.Errorf("简历解析失败: %w", err)
		}
		if profile != nil {
			if profileJSON, marshalErr := json.Marshal(profile); marshalErr == nil {
				metadata.ResumeProfileJSON = string(profileJSON)
			}
		}
	}
	interview.Status = model.InterviewStatusOngoing
	interview.AIFeedback = metadata.toStorageValue()
	return s.interviewRepo.Update(ctx, interview)
}

// generateAndPersistInterviewReport 执行原有同步后处理链，并将最终报告持久化到面试记录。
func (s *interviewService) generateAndPersistInterviewReport(ctx context.Context, interview *model.MockInterview, sessionID string) (ai.InterviewReport, int64, time.Time, error) {
	if interview == nil {
		return ai.InterviewReport{}, 0, time.Time{}, fmt.Errorf("面试记录不存在")
	}
	if strings.TrimSpace(sessionID) == "" {
		return ai.InterviewReport{}, 0, time.Time{}, fmt.Errorf("面试会话不存在")
	}

	if err := s.evaluateInterviewAnswersForReport(ctx, interview.ID, sessionID); err != nil {
		return ai.InterviewReport{}, 0, time.Time{}, fmt.Errorf("补全作答评分失败: %w", err)
	}

	report, err := s.interviewAgent.GenerateReport(ctx, sessionID)
	if err != nil {
		return ai.InterviewReport{}, 0, time.Time{}, fmt.Errorf("生成面试报告失败: %w", err)
	}
	if err := s.enrichInterviewReportWithCoding(ctx, interview, &report); err != nil {
		return ai.InterviewReport{}, 0, time.Time{}, fmt.Errorf("生成编程题诊断失败: %w", err)
	}
	reportJSON, err := serializeInterviewReport(report)
	if err != nil {
		return ai.InterviewReport{}, 0, time.Time{}, fmt.Errorf("序列化面试报告失败: %w", err)
	}

	completedAt := time.Now()
	if err := s.persistLearningArchiveEntries(ctx, interview, report, completedAt); err != nil {
		return ai.InterviewReport{}, 0, time.Time{}, err
	}

	interview.Status = model.InterviewStatusCompleted
	interview.Score = report.OverallScore
	interview.EndedAt = &completedAt
	interview.ReportJSON = reportJSON
	interview.AIFeedback = buildInterviewReportSummary(report)
	if err := s.interviewRepo.Update(ctx, interview); err != nil {
		return ai.InterviewReport{}, 0, time.Time{}, err
	}

	_ = s.interviewAgent.EndInterview(ctx, sessionID)

	var duration int64
	if interview.StartedAt != nil {
		duration = int64(completedAt.Sub(*interview.StartedAt).Seconds())
	}
	return report, duration, completedAt, nil
}

// buildInterviewReportResponse 根据面试记录和可选任务状态组装统一报告响应。
func (s *interviewService) buildInterviewReportResponse(interview *model.MockInterview, report *ai.InterviewReport, task *model.AsyncTask) *InterviewReportResponse {
	resp := &InterviewReportResponse{
		InterviewID: interview.ID,
		Status:      interview.Status,
		Report:      report,
	}
	if interview != nil && interview.EndedAt != nil {
		resp.CompletedAt = *interview.EndedAt
		if interview.StartedAt != nil {
			resp.Duration = int64(interview.EndedAt.Sub(*interview.StartedAt).Seconds())
		}
	}
	applyInterviewAsyncTaskState(resp, task)
	return resp
}

// loadLatestInterviewAsyncTask 查询当前面试最近一次关键异步任务状态。
func (s *interviewService) loadLatestInterviewAsyncTask(ctx context.Context, interviewID uint) (*model.AsyncTask, error) {
	if s.asyncTaskRepo == nil || interviewID == 0 {
		return nil, nil
	}
	return s.asyncTaskRepo.GetLatestByEntity(ctx, interviewAsyncEntityType, interviewID, mq.TaskTypeInterviewResumeParse, mq.TaskTypeInterviewReportGenerate)
}

// applyInterviewAsyncTaskState 把异步任务状态映射到对外响应结构。
func applyInterviewAsyncTaskState(target interface{}, task *model.AsyncTask) {
	if task == nil {
		return
	}
	switch resp := target.(type) {
	case *InterviewResponse:
		resp.AsyncTaskID = task.ID
		resp.TaskStatus = task.Status
		resp.TaskError = task.ErrorMsg
	case *InterviewDetailResponse:
		resp.AsyncTaskID = task.ID
		resp.TaskStatus = task.Status
		resp.TaskError = task.ErrorMsg
	case *InterviewReportResponse:
		resp.AsyncTaskID = task.ID
		resp.TaskStatus = task.Status
		resp.TaskError = task.ErrorMsg
	}
}

// decodeInterviewResumeParsePayload 解析简历异步任务载荷。
func decodeInterviewResumeParsePayload(raw string) (mq.InterviewResumeParsePayload, error) {
	var payload mq.InterviewResumeParsePayload
	if strings.TrimSpace(raw) == "" {
		return payload, fmt.Errorf("empty payload")
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

// decodeInterviewReportPayload 解析面试报告异步任务载荷。
func decodeInterviewReportPayload(raw string) (mq.InterviewReportPayload, error) {
	var payload mq.InterviewReportPayload
	if strings.TrimSpace(raw) == "" {
		return payload, fmt.Errorf("empty payload")
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

// buildInterviewResumeParseIdempotencyKey 生成简历解析任务幂等键。
func buildInterviewResumeParseIdempotencyKey(interviewID uint) string {
	return fmt.Sprintf("interview-resume-parse:%d", interviewID)
}

// buildInterviewReportIdempotencyKey 生成面试报告任务幂等键。
func buildInterviewReportIdempotencyKey(interviewID uint) string {
	return fmt.Sprintf("interview-report-generate:%d", interviewID)
}
