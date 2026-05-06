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
					Title:       "学习 goroutine 调度",
					Description: "理解并发模型",
					TaskType:    model.TaskTypeStudy,
					DayNumber:   1,
					Priority:    "high",
				},
				{
					Title:       "完成项目目标复盘",
					Description: "围绕阶段目标整理训练动作",
					TaskType:    model.TaskTypeReview,
					DayNumber:   2,
					Priority:    "medium",
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

	svc := NewPlanService(planRepo, taskRepo, agent, nil, nil, industryRepo)

	resp, err := svc.GeneratePlan(context.Background(), 23, &GeneratePlanRequest{
		Level:           "beginner",
		DailyStudyTime:  60,
		WeakTopics:      []string{"goroutine"},
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
	if len(taskRepo.createdTasks) != 2 {
		t.Fatalf("expected 2 generated tasks, got %d", len(taskRepo.createdTasks))
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
	if len(resp.Tasks) != 2 {
		t.Fatalf("expected 2 response tasks, got %d", len(resp.Tasks))
	}
	if resp.Tasks[0].Source != "weak_topic" || resp.Tasks[0].SourceLabel != "弱项补强" {
		t.Fatalf("expected first task to be weak topic task, got %#v", resp.Tasks[0])
	}
	if resp.Tasks[0].Reason == "" || resp.Tasks[0].PriorityExplanation == "" {
		t.Fatalf("expected first task to include explanation fields, got %#v", resp.Tasks[0])
	}
	if resp.Tasks[1].Source != "goal" || resp.Tasks[1].SourceLabel != "目标拆解" {
		t.Fatalf("expected second task to be goal task, got %#v", resp.Tasks[1])
	}
}

// TestPlanServiceGeneratePlanRejectsUnknownIndustry 验证行业编码和行业ID都无效时会返回可读业务错误。
func TestPlanServiceGeneratePlanRejectsUnknownIndustry(t *testing.T) {
	t.Parallel()

	svc := NewPlanService(
		&stubPlanRepository{},
		&stubPlanTaskRepository{},
		&stubPlanAgent{},
		nil,
		nil,
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

// TestPlanServiceGeneratePlanFallsBackToDefaultSource 验证未提供弱项和目标时，任务会回退为默认来源解释。
func TestPlanServiceGeneratePlanFallsBackToDefaultSource(t *testing.T) {
	t.Parallel()

	goIndustry := &model.Industry{
		BaseModel: model.BaseModel{ID: 8},
		Code:      "go",
		Name:      "Go",
	}
	planRepo := &stubPlanRepository{}
	taskRepo := &stubPlanTaskRepository{}
	agent := &stubPlanAgent{
		generatedPlan: ai.LearningPlan{
			Title:       "默认计划",
			Description: "验证默认解释路径",
			Tasks: []ai.PlanTask{
				{
					Title:       "阅读 Go 标准库示例",
					Description: "熟悉基础库的常见用法",
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

	svc := NewPlanService(planRepo, taskRepo, agent, nil, nil, industryRepo)
	resp, err := svc.GeneratePlan(context.Background(), 31, &GeneratePlanRequest{
		Level:          "intermediate",
		DailyStudyTime: 45,
		DurationDays:   7,
		IndustryCode:   "go",
	})
	if err != nil {
		t.Fatalf("GeneratePlan returned error: %v", err)
	}

	if len(resp.Tasks) != 1 {
		t.Fatalf("expected 1 response task, got %d", len(resp.Tasks))
	}
	if resp.Tasks[0].Source != "default" || resp.Tasks[0].SourceLabel != "基础默认任务" {
		t.Fatalf("expected default source task, got %#v", resp.Tasks[0])
	}
	if resp.Tasks[0].Reason == "" || resp.Tasks[0].PriorityExplanation == "" {
		t.Fatalf("expected default task explanations, got %#v", resp.Tasks[0])
	}
}

// TestPlanServiceGeneratePlanUsesFocusSignals 验证计划生成后会基于统一训练重点信号标记任务来源与题单提示。
func TestPlanServiceGeneratePlanUsesFocusSignals(t *testing.T) {
	t.Parallel()

	goIndustry := &model.Industry{
		BaseModel: model.BaseModel{ID: 9},
		Code:      "go",
		Name:      "Go",
	}
	planRepo := &stubPlanRepository{}
	taskRepo := &stubPlanTaskRepository{}
	agent := &stubPlanAgent{
		generatedPlan: ai.LearningPlan{
			Title:       "信号驱动计划",
			Description: "验证统一训练重点信号接入",
			Tasks: []ai.PlanTask{
				{
					Title:       "状态定义不清专项练习",
					Description: "围绕状态定义不清做一轮动态规划专项补练",
					TaskType:    model.TaskTypePractice,
					DayNumber:   1,
					Priority:    "high",
				},
				{
					Title:       "状态定义不清复盘",
					Description: "先整理状态定义模板，再进入下一轮训练",
					TaskType:    model.TaskTypeStudy,
					DayNumber:   2,
					Priority:    "medium",
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

	svc := NewPlanService(
		planRepo,
		taskRepo,
		agent,
		growthLearningArchiveRepositoryStub{
			entries: []model.LearningArchiveEntry{
				{
					IndustryCode:    "go",
					SourceRef:       "practice:31:901",
					MistakeTagsJSON: `["状态定义不清"]`,
					SuggestionsJSON: `["先口述状态定义，再开始写代码。"]`,
				},
				{
					IndustryCode:    "java",
					SourceRef:       "practice:31:999",
					MistakeTagsJSON: `["边界条件生疏"]`,
					SuggestionsJSON: `["写完主流程后单独列一组边界样例再检查。"]`,
				},
			},
		},
		&growthInterviewRepositoryStub{},
		industryRepo,
	)

	resp, err := svc.GeneratePlan(context.Background(), 31, &GeneratePlanRequest{
		Level:          "intermediate",
		DailyStudyTime: 45,
		DurationDays:   7,
		IndustryCode:   "go",
	})
	if err != nil {
		t.Fatalf("GeneratePlan returned error: %v", err)
	}
	if len(resp.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(resp.Tasks))
	}
	if resp.Tasks[0].Source != "practice_recommendation" || resp.Tasks[0].SourceRef != "practice:31:901" {
		t.Fatalf("expected first task to use practice recommendation source, got %#v", resp.Tasks[0])
	}
	if resp.Tasks[0].CollectionHint != "algorithm-structure" {
		t.Fatalf("expected collection hint algorithm-structure, got %#v", resp.Tasks[0])
	}
	if resp.Tasks[1].Source != "weekly_focus" {
		t.Fatalf("expected second task to use weekly focus source, got %#v", resp.Tasks[1])
	}
	if len(agent.lastGenerateProfile.WeakTopics) == 0 || agent.lastGenerateProfile.WeakTopics[0] != "状态定义不清" {
		t.Fatalf("expected focus signal tag to be merged into profile weak topics, got %#v", agent.lastGenerateProfile.WeakTopics)
	}
	for _, topic := range agent.lastGenerateProfile.WeakTopics {
		if topic == "边界条件生疏" {
			t.Fatalf("expected other-industry weak topic to be filtered out, got %#v", agent.lastGenerateProfile.WeakTopics)
		}
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
