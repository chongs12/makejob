package service

import (
	"context"
	"testing"
	"time"

	"makejob-backend/internal/model"
)

// TestPlanServiceUpdateTaskStatusReopensCompletedPlan 验证任务从已完成退回待办后，会同步清理完成时间并把计划恢复为进行中。
func TestPlanServiceUpdateTaskStatusReopensCompletedPlan(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	planRepo := &updateTaskPlanRepositoryStub{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 8},
			UserID:         12,
			Status:         model.PlanStatusCompleted,
			TotalTasks:     2,
			CompletedTasks: 2,
		},
	}
	taskRepo := &updateTaskPlanTaskRepositoryStub{
		tasks: []model.LearningTask{
			{
				BaseModel:   model.BaseModel{ID: 101},
				PlanID:      8,
				Title:       "任务一",
				Status:      model.TaskStatusCompleted,
				CompletedAt: &completedAt,
			},
			{
				BaseModel:   model.BaseModel{ID: 102},
				PlanID:      8,
				Title:       "任务二",
				Status:      model.TaskStatusCompleted,
				CompletedAt: &completedAt,
			},
		},
	}
	svc := &planService{
		planRepo: planRepo,
		taskRepo: taskRepo,
	}

	err := svc.UpdateTaskStatus(context.Background(), 12, 8, 101, &UpdateTaskStatusRequest{
		Status: model.TaskStatusPending,
	})
	if err != nil {
		t.Fatalf("UpdateTaskStatus returned error: %v", err)
	}

	if taskRepo.updatedTask == nil {
		t.Fatal("expected task update to be persisted")
	}
	if taskRepo.updatedTask.Status != model.TaskStatusPending {
		t.Fatalf("expected task status pending, got %s", taskRepo.updatedTask.Status)
	}
	if taskRepo.updatedTask.CompletedAt != nil {
		t.Fatalf("expected completed_at to be cleared, got %v", taskRepo.updatedTask.CompletedAt)
	}
	if planRepo.savedPlan == nil {
		t.Fatal("expected plan update to be persisted")
	}
	if planRepo.savedPlan.Status != model.PlanStatusActive {
		t.Fatalf("expected plan status active, got %s", planRepo.savedPlan.Status)
	}
	if planRepo.savedPlan.CompletedTasks != 1 {
		t.Fatalf("expected completed task count 1, got %d", planRepo.savedPlan.CompletedTasks)
	}
}

// TestPlanServiceUpdateTaskStatusCompletesPlan 验证最后一项任务完成后，计划会被自动标记为已完成。
func TestPlanServiceUpdateTaskStatusCompletesPlan(t *testing.T) {
	t.Parallel()

	planRepo := &updateTaskPlanRepositoryStub{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 9},
			UserID:         21,
			Status:         model.PlanStatusActive,
			TotalTasks:     2,
			CompletedTasks: 1,
		},
	}
	taskRepo := &updateTaskPlanTaskRepositoryStub{
		tasks: []model.LearningTask{
			{
				BaseModel: model.BaseModel{ID: 201},
				PlanID:    9,
				Title:     "任务一",
				Status:    model.TaskStatusCompleted,
			},
			{
				BaseModel: model.BaseModel{ID: 202},
				PlanID:    9,
				Title:     "任务二",
				Status:    model.TaskStatusInProgress,
			},
		},
	}
	svc := &planService{
		planRepo: planRepo,
		taskRepo: taskRepo,
	}

	err := svc.UpdateTaskStatus(context.Background(), 21, 9, 202, &UpdateTaskStatusRequest{
		Status: model.TaskStatusCompleted,
	})
	if err != nil {
		t.Fatalf("UpdateTaskStatus returned error: %v", err)
	}

	if taskRepo.updatedTask == nil || taskRepo.updatedTask.CompletedAt == nil {
		t.Fatal("expected completing task to set completed_at")
	}
	if planRepo.savedPlan == nil {
		t.Fatal("expected plan update to be persisted")
	}
	if planRepo.savedPlan.Status != model.PlanStatusCompleted {
		t.Fatalf("expected plan status completed, got %s", planRepo.savedPlan.Status)
	}
	if planRepo.savedPlan.CompletedTasks != 2 {
		t.Fatalf("expected completed task count 2, got %d", planRepo.savedPlan.CompletedTasks)
	}
}

// updateTaskPlanRepositoryStub 模拟学习计划仓库，用于观察任务状态更新后的计划写回结果。
type updateTaskPlanRepositoryStub struct {
	plan      *model.LearningPlan
	savedPlan *model.LearningPlan
}

// Create 满足接口要求，当前测试不依赖创建行为。
func (s *updateTaskPlanRepositoryStub) Create(context.Context, *model.LearningPlan) error {
	return nil
}

// GetByID 返回预置学习计划。
func (s *updateTaskPlanRepositoryStub) GetByID(context.Context, uint) (*model.LearningPlan, error) {
	if s.plan == nil {
		return nil, nil
	}
	clone := *s.plan
	return &clone, nil
}

// GetCurrentByUser 满足接口要求，当前测试不依赖该行为。
func (s *updateTaskPlanRepositoryStub) GetCurrentByUser(context.Context, uint) (*model.LearningPlan, error) {
	return nil, nil
}

// Update 记录服务层写回的计划状态。
func (s *updateTaskPlanRepositoryStub) Update(_ context.Context, plan *model.LearningPlan) error {
	clone := *plan
	s.savedPlan = &clone
	s.plan = &clone
	return nil
}

// ListByUser 满足接口要求，当前测试不依赖该行为。
func (s *updateTaskPlanRepositoryStub) ListByUser(context.Context, uint, int, int) ([]model.LearningPlan, int64, error) {
	return nil, 0, nil
}

// PauseActivePlans 满足接口要求，当前测试不依赖该行为。
func (s *updateTaskPlanRepositoryStub) PauseActivePlans(context.Context, uint) error {
	return nil
}

// updateTaskPlanTaskRepositoryStub 模拟学习任务仓库，用于观察任务状态更新前后的任务列表变化。
type updateTaskPlanTaskRepositoryStub struct {
	tasks       []model.LearningTask
	updatedTask *model.LearningTask
}

// Create 满足接口要求，当前测试不依赖创建行为。
func (s *updateTaskPlanTaskRepositoryStub) Create(context.Context, *model.LearningTask) error {
	return nil
}

// BatchCreate 满足接口要求，当前测试不依赖批量创建行为。
func (s *updateTaskPlanTaskRepositoryStub) BatchCreate(context.Context, []model.LearningTask) error {
	return nil
}

// GetByID 返回指定任务的拷贝，避免服务层直接修改测试桩内部数据。
func (s *updateTaskPlanTaskRepositoryStub) GetByID(_ context.Context, id uint) (*model.LearningTask, error) {
	for _, task := range s.tasks {
		if task.ID == id {
			clone := task
			return &clone, nil
		}
	}
	return nil, nil
}

// Update 记录写回的任务，并同步更新内部任务列表供后续统计使用。
func (s *updateTaskPlanTaskRepositoryStub) Update(_ context.Context, task *model.LearningTask) error {
	clone := *task
	s.updatedTask = &clone
	for index, item := range s.tasks {
		if item.ID == task.ID {
			s.tasks[index] = clone
			return nil
		}
	}
	s.tasks = append(s.tasks, clone)
	return nil
}

// ListByPlan 返回当前任务列表快照。
func (s *updateTaskPlanTaskRepositoryStub) ListByPlan(context.Context, uint) ([]model.LearningTask, error) {
	return append([]model.LearningTask(nil), s.tasks...), nil
}

// CountByPlanAndStatus 满足接口要求，当前测试不依赖该行为。
func (s *updateTaskPlanTaskRepositoryStub) CountByPlanAndStatus(context.Context, uint, string) (int64, error) {
	return 0, nil
}

// DeleteByPlan 满足接口要求，当前测试不依赖该行为。
func (s *updateTaskPlanTaskRepositoryStub) DeleteByPlan(context.Context, uint) error {
	return nil
}

// DeleteIncompleteByPlan 满足接口要求，当前测试不依赖该行为。
func (s *updateTaskPlanTaskRepositoryStub) DeleteIncompleteByPlan(context.Context, uint) error {
	return nil
}
