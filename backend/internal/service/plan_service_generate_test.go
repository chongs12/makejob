package service

import (
	"context"
	"testing"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
)

// TestPlanServiceGeneratePlanUsesIndustryCode 验证生成学习计划时会优先按行业编码解析真实行业主键。
func TestPlanServiceGeneratePlanUsesIndustryCode(t *testing.T) {
	t.Parallel()

	goIndustry := &model.Industry{
		BaseModel: model.BaseModel{ID: 7},
		Code:      "go",
		Name:      "Go",
	}
	planRepo := &stubPlanRepository{}
	taskRepo := &stubPlanTaskRepository{}
	agent := &stubPlanAgent{
		generatedPlan: ai.LearningPlan{
			Title:       "Go 学习计划",
			Description: "覆盖基础与实践的学习安排",
			Tasks: []ai.PlanTask{
				{
					Title:       "学习 goroutine",
					Description: "理解并发模型",
					TaskType:    model.TaskTypeStudy,
					DayNumber:   1,
				},
			},
		},
	}
	industryRepo := &stubPlanIndustryRepository{
		byCode: map[string]*model.Industry{
			"go": goIndustry,
		},
		byID: map[uint]*model.Industry{
			goIndustry.ID: goIndustry,
		},
	}

	svc := NewPlanService(planRepo, taskRepo, agent, industryRepo)

	resp, err := svc.GeneratePlan(context.Background(), 23, &GeneratePlanRequest{
		Level:           "beginner",
		DailyStudyTime:  60,
		GoalDescription: "补齐并发和网络编程基础",
		DurationDays:    14,
		IndustryID:      1,
		IndustryCode:    "go",
	})
	if err != nil {
		t.Fatalf("GeneratePlan returned error: %v", err)
	}

	if planRepo.created == nil {
		t.Fatal("expected learning plan to be created")
	}
	if planRepo.created.IndustryID != goIndustry.ID {
		t.Fatalf("expected created plan industry id %d, got %d", goIndustry.ID, planRepo.created.IndustryID)
	}
	if agent.lastGenerateIndustry != "go" {
		t.Fatalf("expected agent to receive industry code go, got %s", agent.lastGenerateIndustry)
	}
	if len(taskRepo.createdTasks) != 1 {
		t.Fatalf("expected 1 generated task, got %d", len(taskRepo.createdTasks))
	}
	if taskRepo.createdTasks[0].PlanID != planRepo.created.ID {
		t.Fatalf("expected generated task to belong to created plan %d, got %d", planRepo.created.ID, taskRepo.createdTasks[0].PlanID)
	}
	if resp == nil || resp.Title != "Go 学习计划" {
		t.Fatalf("expected plan response title Go 学习计划, got %#v", resp)
	}
	if resp.IndustryID != goIndustry.ID || resp.IndustryCode != "go" {
		t.Fatalf("expected response industry (%d, go), got (%d, %s)", goIndustry.ID, resp.IndustryID, resp.IndustryCode)
	}
}

// TestPlanServiceGeneratePlanRejectsUnknownIndustry 验证行业编码和行业ID都无效时会返回可读业务错误。
func TestPlanServiceGeneratePlanRejectsUnknownIndustry(t *testing.T) {
	t.Parallel()

	svc := NewPlanService(
		&stubPlanRepository{},
		&stubPlanTaskRepository{},
		&stubPlanAgent{},
		&stubPlanIndustryRepository{},
	)

	_, err := svc.GeneratePlan(context.Background(), 23, &GeneratePlanRequest{
		Level:           "beginner",
		DailyStudyTime:  60,
		GoalDescription: "验证异常路径",
		DurationDays:    14,
		IndustryID:      99,
		IndustryCode:    "missing",
	})
	if err == nil {
		t.Fatal("expected GeneratePlan to fail for unknown industry")
	}

	businessErr, ok := err.(*common.BusinessError)
	if !ok {
		t.Fatalf("expected business error, got %T", err)
	}
	if businessErr.Code != common.CodeBadRequest {
		t.Fatalf("expected bad request code, got %d", businessErr.Code)
	}
	if businessErr.Message != "所选学习方向不存在" {
		t.Fatalf("expected readable error message, got %s", businessErr.Message)
	}
}

// stubPlanIndustryRepository 模拟行业仓库，供学习计划服务测试解析真实行业主键。
type stubPlanIndustryRepository struct {
	byCode map[string]*model.Industry
	byID   map[uint]*model.Industry
}

// List 返回空列表以满足接口要求。
func (s *stubPlanIndustryRepository) List(context.Context) ([]model.Industry, error) {
	return nil, nil
}

// GetByID 返回预置的行业信息。
func (s *stubPlanIndustryRepository) GetByID(_ context.Context, id uint) (*model.Industry, error) {
	if s == nil || s.byID == nil {
		return nil, nil
	}
	return s.byID[id], nil
}

// Create 不在当前测试中使用。
func (s *stubPlanIndustryRepository) Create(context.Context, *model.Industry) error {
	return nil
}

// Update 不在当前测试中使用。
func (s *stubPlanIndustryRepository) Update(context.Context, *model.Industry) error {
	return nil
}

// GetByCode 返回预置的行业信息。
func (s *stubPlanIndustryRepository) GetByCode(_ context.Context, code string) (*model.Industry, error) {
	if s == nil || s.byCode == nil {
		return nil, nil
	}
	return s.byCode[code], nil
}
