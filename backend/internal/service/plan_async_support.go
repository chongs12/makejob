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

const planAsyncEntityType = "learning_plan"

// ProcessPlanGenerateTask 消费学习计划生成任务，并在成功后把占位计划切换为正式计划。
func (s *planService) ProcessPlanGenerateTask(ctx context.Context, asyncTaskID uint) error {
	if s.asyncTaskRepo == nil {
		return fmt.Errorf("async task repository is required")
	}
	if s.planRepo == nil || s.taskRepo == nil {
		return fmt.Errorf("plan generation task dependencies are incomplete")
	}

	asyncTask, shouldRun, err := s.asyncTaskRepo.ClaimByID(ctx, asyncTaskID)
	if err != nil {
		return err
	}
	if asyncTask == nil || !shouldRun {
		return nil
	}

	payload, err := decodePlanGeneratePayload(asyncTask.PayloadJSON)
	if err != nil {
		return s.failPlanAsyncTask(ctx, asyncTask, fmt.Errorf("解析学习计划生成载荷失败: %w", err), false)
	}

	plan, err := s.planRepo.GetByID(ctx, payload.PlanID)
	if err != nil {
		return s.failPlanAsyncTask(ctx, asyncTask, err, true)
	}
	if plan == nil {
		return s.failPlanAsyncTask(ctx, asyncTask, fmt.Errorf("学习计划不存在: %d", payload.PlanID), false)
	}

	storedContext := readPlanStoredContext(plan.PlanJSON)
	profile := ai.UserProfile{
		Level:           strings.TrimSpace(storedContext.Level),
		WeakTopics:      append([]string(nil), storedContext.WeakTopics...),
		DailyStudyTime:  storedContext.DailyStudyTime,
		DurationDays:    storedContext.DurationDays,
		GoalDescription: strings.TrimSpace(storedContext.GoalDescription),
	}
	industryCode := strings.TrimSpace(storedContext.IndustryCode)
	if industryCode == "" {
		industryCode = strings.TrimSpace(payload.IndustryCode)
	}
	if industryCode == "" {
		industryCode = s.resolvePlanIndustryCode(ctx, plan.IndustryID)
	}

	resultPlan, tasks, err := s.generateAndPersistLearningPlan(ctx, plan, profile, industryCode, storedContext)
	if err != nil {
		return s.failPlanAsyncTask(ctx, asyncTask, err, true)
	}

	resultJSON, err := json.Marshal(map[string]interface{}{
		"plan_id":         resultPlan.ID,
		"status":          resultPlan.Status,
		"total_tasks":     resultPlan.TotalTasks,
		"completed_tasks": resultPlan.CompletedTasks,
		"generated_count": len(tasks),
		"industry_code":   industryCode,
	})
	if err != nil {
		return s.failPlanAsyncTask(ctx, asyncTask, fmt.Errorf("序列化学习计划生成结果失败: %w", err), false)
	}

	now := time.Now()
	asyncTask.Status = model.AsyncTaskStatusSucceeded
	asyncTask.ResultJSON = string(resultJSON)
	asyncTask.ErrorMsg = ""
	asyncTask.FinishedAt = &now
	return s.asyncTaskRepo.Update(ctx, asyncTask)
}

// enqueuePlanGenerateTask 创建并投递学习计划生成任务，供接口层快速返回生成中状态。
func (s *planService) enqueuePlanGenerateTask(
	ctx context.Context,
	userID uint,
	industryID uint,
	industryCode string,
	req *GeneratePlanRequest,
	storedContext planStoredContext,
) (*model.LearningPlan, *model.AsyncTask, error) {
	if s.asyncTaskRepo == nil || s.taskPublisher == nil {
		return nil, nil, fmt.Errorf("async dispatch dependencies are incomplete")
	}
	if req == nil {
		return nil, nil, fmt.Errorf("generate plan request is required")
	}

	spec, ok := mq.QueueSpecByTaskType(mq.TaskTypePlanGenerate)
	if !ok {
		return nil, nil, fmt.Errorf("未找到队列配置: %s", mq.TaskTypePlanGenerate)
	}

	plan, err := s.createGeneratingPlanPlaceholder(ctx, userID, industryID, storedContext)
	if err != nil {
		return nil, nil, err
	}

	requestJSON, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("序列化学习计划请求失败: %w", err)
	}
	payload := mq.PlanGeneratePayload{
		PlanID:       plan.ID,
		UserID:       userID,
		RequestJSON:  requestJSON,
		IndustryCode: strings.TrimSpace(industryCode),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("序列化学习计划任务载荷失败: %w", err)
	}

	asyncTask, err := s.preparePlanAsyncTask(ctx, spec, plan, payloadBytes)
	if err != nil {
		return nil, nil, err
	}
	if asyncTask.Status != model.AsyncTaskStatusPending {
		return plan, asyncTask, nil
	}

	message := buildAsyncTaskMessage(
		mq.TaskTypePlanGenerate,
		asyncTask.ID,
		asyncTask.EntityType,
		asyncTask.EntityID,
		asyncTask.Source,
		asyncTask.IdempotencyKey,
		payloadBytes,
	)
	message.MaxRetries = asyncTask.MaxRetries
	message.Attempt = asyncTask.RetryCount
	if err := s.taskPublisher.PublishTask(ctx, spec.RoutingKey, message); err != nil {
		asyncTask.Status = model.AsyncTaskStatusFailed
		asyncTask.ErrorMsg = err.Error()
		asyncTask.FinishedAt = nil
		_ = s.asyncTaskRepo.Update(ctx, asyncTask)
		return plan, asyncTask, err
	}

	now := time.Now()
	asyncTask.Status = model.AsyncTaskStatusQueued
	asyncTask.PublishedAt = &now
	asyncTask.ErrorMsg = ""
	asyncTask.FinishedAt = nil
	if err := s.asyncTaskRepo.Update(ctx, asyncTask); err != nil {
		return nil, nil, err
	}
	return plan, asyncTask, nil
}

// createGeneratingPlanPlaceholder 创建学习计划生成中的占位记录，供前端轮询同一业务实体。
func (s *planService) createGeneratingPlanPlaceholder(
	ctx context.Context,
	userID uint,
	industryID uint,
	storedContext planStoredContext,
) (*model.LearningPlan, error) {
	if err := s.planRepo.PauseActivePlans(ctx, userID); err != nil {
		return nil, err
	}

	now := time.Now()
	startDate := now
	endDate := now.AddDate(0, 0, maxInt(storedContext.DurationDays, 1))
	placeholderPlan := ai.LearningPlan{
		Title:       "学习计划生成中",
		Description: "系统正在根据你的目标、弱项和最近训练表现生成个性化学习计划。",
		Phase:       model.LearningPhaseFoundation,
		PhaseGoal:   "正在整理个性化训练主线，请稍候刷新查看结果。",
		Duration:    maxInt(storedContext.DurationDays, 1),
	}
	placeholderJSON, err := buildPlanStoredPayload(placeholderPlan, storedContext)
	if err != nil {
		return nil, fmt.Errorf("序列化占位学习计划失败: %w", err)
	}

	plan := &model.LearningPlan{
		UserID:         userID,
		IndustryID:     industryID,
		Title:          placeholderPlan.Title,
		Description:    placeholderPlan.Description,
		Phase:          placeholderPlan.Phase,
		PhaseGoal:      placeholderPlan.PhaseGoal,
		PlanJSON:       string(placeholderJSON),
		Status:         model.PlanStatusGenerating,
		TotalTasks:     0,
		CompletedTasks: 0,
		StartDate:      &startDate,
		EndDate:        &endDate,
	}
	if err := s.planRepo.Create(ctx, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

// preparePlanAsyncTask 创建或重置学习计划生成任务，保证同一占位计划可重试消费。
func (s *planService) preparePlanAsyncTask(
	ctx context.Context,
	spec mq.QueueSpec,
	plan *model.LearningPlan,
	payloadBytes []byte,
) (*model.AsyncTask, error) {
	idempotencyKey := buildPlanGenerateIdempotencyKey(plan.ID)
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
		Source:         "plan-service",
		Status:         model.AsyncTaskStatusPending,
		QueueName:      spec.QueueName,
		RoutingKey:     spec.RoutingKey,
		EntityType:     planAsyncEntityType,
		EntityID:       plan.ID,
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
	existingTask.Source = "plan-service"
	existingTask.Status = model.AsyncTaskStatusPending
	existingTask.QueueName = spec.QueueName
	existingTask.RoutingKey = spec.RoutingKey
	existingTask.EntityType = planAsyncEntityType
	existingTask.EntityID = plan.ID
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

// generateAndPersistLearningPlan 统一执行 AI 生成与计划落库，支持同步创建和异步任务回填复用同一套逻辑。
func (s *planService) generateAndPersistLearningPlan(
	ctx context.Context,
	existingPlan *model.LearningPlan,
	profile ai.UserProfile,
	industryCode string,
	storedContext planStoredContext,
) (*model.LearningPlan, []model.LearningTask, error) {
	if s.planAgent == nil {
		return nil, nil, fmt.Errorf("plan agent is unavailable")
	}

	aiPlan, err := s.planAgent.GeneratePlan(ctx, profile, industryCode)
	if err != nil {
		return nil, nil, fmt.Errorf("AI生成学习计划失败: %w", err)
	}
	aiPlan = normalizeLearningPlanPhases(aiPlan)
	aiPlan = arrangeLearningPlanByPhase(aiPlan, false)
	storedContext.PhaseBlueprintVersion = PhaseBlueprintVersion
	storedContext.PhaseBlueprint = buildPhaseBlueprintFromPlanTasks(aiPlan.Tasks, profile.DurationDays, PhaseBlueprintSourceDuration)
	return s.persistGeneratedLearningPlan(ctx, existingPlan, profile.DurationDays, aiPlan, storedContext)
}

// persistGeneratedLearningPlan 将 AI 计划结果回写到学习计划主表和任务表，兼容新建与占位更新两种路径。
func (s *planService) persistGeneratedLearningPlan(
	ctx context.Context,
	existingPlan *model.LearningPlan,
	durationDays int,
	aiPlan ai.LearningPlan,
	storedContext planStoredContext,
) (*model.LearningPlan, []model.LearningTask, error) {
	if s.planRepo == nil || s.taskRepo == nil {
		return nil, nil, fmt.Errorf("plan persistence dependencies are incomplete")
	}

	now := time.Now()
	startDate := now
	endDate := now.AddDate(0, 0, maxInt(durationDays, 1))
	planJSON, err := buildPlanStoredPayload(aiPlan, storedContext)
	if err != nil {
		return nil, nil, fmt.Errorf("序列化计划失败: %w", err)
	}

	plan := existingPlan
	if plan == nil {
		plan = &model.LearningPlan{}
	}
	plan.Title = aiPlan.Title
	plan.Description = aiPlan.Description
	plan.Phase = aiPlan.Phase
	plan.PhaseGoal = aiPlan.PhaseGoal
	plan.PlanJSON = string(planJSON)
	plan.Status = model.PlanStatusActive
	plan.TotalTasks = len(aiPlan.Tasks)
	plan.CompletedTasks = 0
	plan.StartDate = &startDate
	plan.EndDate = &endDate

	if existingPlan == nil || existingPlan.ID == 0 {
		if err := s.planRepo.Create(ctx, plan); err != nil {
			return nil, nil, err
		}
	} else {
		if err := s.planRepo.Update(ctx, plan); err != nil {
			return nil, nil, err
		}
		if err := s.taskRepo.DeleteByPlan(ctx, plan.ID); err != nil {
			return nil, nil, err
		}
	}

	tasks := make([]model.LearningTask, 0, len(aiPlan.Tasks))
	for index, task := range aiPlan.Tasks {
		dueDate := startDate.AddDate(0, 0, task.DayNumber-1)
		tasks = append(tasks, model.LearningTask{
			PlanID:      plan.ID,
			Title:       task.Title,
			Description: task.Description,
			TaskType:    task.TaskType,
			Phase:       task.Phase,
			PhaseGoal:   task.PhaseGoal,
			Status:      model.TaskStatusPending,
			DueDate:     &dueDate,
			SortOrder:   index,
		})
	}
	if err := s.taskRepo.BatchCreate(ctx, tasks); err != nil {
		return nil, nil, err
	}
	return plan, tasks, nil
}

// loadLatestPlanAsyncTask 查询当前学习计划最近一次生成任务状态，供接口轮询展示。
func (s *planService) loadLatestPlanAsyncTask(ctx context.Context, planID uint) (*model.AsyncTask, error) {
	if s.asyncTaskRepo == nil || planID == 0 {
		return nil, nil
	}
	return s.asyncTaskRepo.GetLatestByEntity(ctx, planAsyncEntityType, planID, mq.TaskTypePlanGenerate)
}

// applyPlanAsyncTaskState 把异步任务状态映射到学习计划详情响应上。
func applyPlanAsyncTaskState(resp *PlanDetailResponse, task *model.AsyncTask) {
	if resp == nil || task == nil {
		return
	}
	resp.AsyncTaskID = task.ID
	resp.TaskStatus = task.Status
	resp.TaskError = task.ErrorMsg
}

// decodePlanGeneratePayload 解析学习计划生成任务载荷。
func decodePlanGeneratePayload(raw string) (mq.PlanGeneratePayload, error) {
	var payload mq.PlanGeneratePayload
	if strings.TrimSpace(raw) == "" {
		return payload, fmt.Errorf("empty payload")
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

// failPlanAsyncTask 回写学习计划生成任务失败状态，并根据是否可重试决定最终状态。
func (s *planService) failPlanAsyncTask(ctx context.Context, asyncTask *model.AsyncTask, taskErr error, retryable bool) error {
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

// buildPlanGenerateIdempotencyKey 生成学习计划生成任务的稳定幂等键。
func buildPlanGenerateIdempotencyKey(planID uint) string {
	return fmt.Sprintf("plan-generate:%d", planID)
}

// maxInt 返回两个整数中的较大值，避免生成占位计划时出现非法周期。
func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
