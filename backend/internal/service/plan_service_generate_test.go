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

	svc := NewPlanService(planRepo, taskRepo, agent, nil, nil, nil, nil, nil, industryRepo)

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
	if planRepo.created.Phase != model.LearningPhaseFoundation || planRepo.created.PhaseGoal == "" {
		t.Fatalf("expected created plan to persist foundation phase fields, got %#v", planRepo.created)
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
	if taskRepo.createdTasks[0].Phase != model.LearningPhaseFoundation || taskRepo.createdTasks[1].Phase != model.LearningPhaseReview {
		t.Fatalf("expected generated tasks to persist derived phases, got %#v", taskRepo.createdTasks)
	}
	if resp == nil || resp.Title != "Go 学习计划" {
		t.Fatalf("expected plan response title Go 学习计划, got %#v", resp)
	}
	if resp.IndustryID != goIndustry.ID || resp.IndustryCode != "go" {
		t.Fatalf("expected response industry (%d, go), got (%d, %s)", goIndustry.ID, resp.IndustryID, resp.IndustryCode)
	}
	if resp.Phase != model.LearningPhaseFoundation || resp.PhaseGoal == "" {
		t.Fatalf("expected response plan phase fields, got %#v", resp)
	}
	if len(resp.Tasks) != 2 {
		t.Fatalf("expected 2 response tasks, got %d", len(resp.Tasks))
	}
	if resp.Tasks[0].Phase != model.LearningPhaseFoundation || resp.Tasks[1].Phase != model.LearningPhaseReview {
		t.Fatalf("expected response tasks to include phase fields, got %#v", resp.Tasks)
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
		nil,
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

	svc := NewPlanService(planRepo, taskRepo, agent, nil, nil, nil, nil, nil, industryRepo)
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
		nil,
		nil,
		nil,
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
	if resp.Tasks[0].Phase != model.LearningPhaseFoundation || resp.Tasks[0].Source != "weekly_focus" {
		t.Fatalf("expected first task to become foundation-stage weekly focus task, got %#v", resp.Tasks[0])
	}
	if resp.Tasks[0].CollectionHint != "algorithm-structure" {
		t.Fatalf("expected collection hint algorithm-structure, got %#v", resp.Tasks[0])
	}
	if resp.Tasks[1].Phase != model.LearningPhaseDrill || resp.Tasks[1].Source != "practice_recommendation" || resp.Tasks[1].SourceRef != "practice:31:901" {
		t.Fatalf("expected second task to use practice recommendation source in drill phase, got %#v", resp.Tasks[1])
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

// TestPlanServiceGeneratePlanReordersPhaseWindows 验证生成计划在 phase 乱序且部分缺失时，仍会被整理成稳定阶段窗口。
func TestPlanServiceGeneratePlanReordersPhaseWindows(t *testing.T) {
	t.Parallel()

	goIndustry := &model.Industry{
		BaseModel: model.BaseModel{ID: 10},
		Code:      "go",
		Name:      "Go",
	}
	planRepo := &stubPlanRepository{}
	taskRepo := &stubPlanTaskRepository{}
	agent := &stubPlanAgent{
		generatedPlan: ai.LearningPlan{
			Title:       "阶段乱序计划",
			Description: "验证生成侧阶段窗口整理",
			Phase:       model.LearningPhaseMock,
			Duration:    21,
			Tasks: []ai.PlanTask{
				{
					Title:       "限时模拟面试",
					Description: "先给出一轮 mock 验证",
					TaskType:    model.TaskTypeInterview,
					Phase:       model.LearningPhaseMock,
					DayNumber:   1,
					Priority:    "medium",
				},
				{
					Title:       "补齐并发基础概念",
					Description: "回到基础理解 goroutine 和 channel",
					TaskType:    model.TaskTypeStudy,
					Phase:       "",
					DayNumber:   2,
					Priority:    "high",
				},
				{
					Title:       "专项刷题：并发模式",
					Description: "围绕高频并发题做 drill",
					TaskType:    model.TaskTypePractice,
					Phase:       model.LearningPhaseDrill,
					DayNumber:   3,
					Priority:    "high",
				},
				{
					Title:       "复盘近期错误",
					Description: "整理最近的并发薄弱点",
					TaskType:    model.TaskTypeReview,
					Phase:       model.LearningPhaseReview,
					DayNumber:   4,
					Priority:    "low",
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

	svc := NewPlanService(planRepo, taskRepo, agent, nil, nil, nil, nil, nil, industryRepo)
	resp, err := svc.GeneratePlan(context.Background(), 41, &GeneratePlanRequest{
		Level:           "intermediate",
		DailyStudyTime:  90,
		WeakTopics:      []string{"goroutine", "channel"},
		GoalDescription: "把并发主线补齐后再进入模拟验证",
		DurationDays:    21,
		IndustryCode:    "go",
	})
	if err != nil {
		t.Fatalf("GeneratePlan returned error: %v", err)
	}

	if planRepo.created == nil {
		t.Fatal("expected learning plan to be created")
	}
	if planRepo.created.Phase != model.LearningPhaseFoundation || planRepo.created.PhaseGoal == "" {
		t.Fatalf("expected generated plan to be rewritten to foundation entry, got %#v", planRepo.created)
	}
	if len(taskRepo.createdTasks) != 4 {
		t.Fatalf("expected 4 persisted tasks, got %d", len(taskRepo.createdTasks))
	}
	expectedPhases := []string{
		model.LearningPhaseFoundation,
		model.LearningPhaseDrill,
		model.LearningPhaseReview,
		model.LearningPhaseMock,
	}
	for index, phase := range expectedPhases {
		if taskRepo.createdTasks[index].Phase != phase {
			t.Fatalf("expected persisted phase order %v, got %#v", expectedPhases, taskRepo.createdTasks)
		}
		if taskRepo.createdTasks[index].PhaseGoal == "" {
			t.Fatalf("expected persisted task %d to have phase goal, got %#v", index, taskRepo.createdTasks[index])
		}
	}
	if !(taskRepo.createdTasks[0].DueDate.Before(*taskRepo.createdTasks[1].DueDate) &&
		taskRepo.createdTasks[1].DueDate.Before(*taskRepo.createdTasks[2].DueDate) &&
		taskRepo.createdTasks[2].DueDate.Before(*taskRepo.createdTasks[3].DueDate)) {
		t.Fatalf("expected due dates to be compacted by phase window, got %#v", taskRepo.createdTasks)
	}
	if resp.Phase != model.LearningPhaseFoundation || resp.PhaseGoal == "" {
		t.Fatalf("expected response phase to be foundation after rewrite, got %#v", resp)
	}
	for index, phase := range expectedPhases {
		if resp.Tasks[index].Phase != phase {
			t.Fatalf("expected response phase order %v, got %#v", expectedPhases, resp.Tasks)
		}
	}
}

// TestPlanServiceGeneratePlanCompactsShortDurationPhaseWindows 验证 7-13 天短周期会以 foundation/drill/review 收口且不强制 mock。
func TestPlanServiceGeneratePlanCompactsShortDurationPhaseWindows(t *testing.T) {
	t.Parallel()

	goIndustry := &model.Industry{
		BaseModel: model.BaseModel{ID: 11},
		Code:      "go",
		Name:      "Go",
	}
	planRepo := &stubPlanRepository{}
	taskRepo := &stubPlanTaskRepository{}
	agent := &stubPlanAgent{
		generatedPlan: ai.LearningPlan{
			Title:       "短周期阶段计划",
			Description: "验证短周期阶段收口",
			Duration:    10,
			Tasks: []ai.PlanTask{
				{
					Title:       "并发基础概念",
					Description: "先补基础概念与模型",
					TaskType:    model.TaskTypeStudy,
					Phase:       model.LearningPhaseFoundation,
					DayNumber:   1,
					Priority:    "high",
				},
				{
					Title:       "专项练习：channel",
					Description: "集中做 drill 练习",
					TaskType:    model.TaskTypePractice,
					Phase:       model.LearningPhaseDrill,
					DayNumber:   2,
					Priority:    "high",
				},
				{
					Title:       "短复盘：并发错因",
					Description: "用 review 收口本轮训练",
					TaskType:    model.TaskTypeReview,
					Phase:       model.LearningPhaseReview,
					DayNumber:   3,
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

	svc := NewPlanService(planRepo, taskRepo, agent, nil, nil, nil, nil, nil, industryRepo)
	resp, err := svc.GeneratePlan(context.Background(), 42, &GeneratePlanRequest{
		Level:           "beginner",
		DailyStudyTime:  60,
		WeakTopics:      []string{"channel"},
		GoalDescription: "先补基础再完成一轮短周期纠偏",
		DurationDays:    10,
		IndustryCode:    "go",
	})
	if err != nil {
		t.Fatalf("GeneratePlan returned error: %v", err)
	}

	if len(resp.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(resp.Tasks))
	}
	expectedPhases := []string{
		model.LearningPhaseFoundation,
		model.LearningPhaseDrill,
		model.LearningPhaseReview,
	}
	expectedDays := []int{1, 4, 7}
	for index, phase := range expectedPhases {
		if resp.Tasks[index].Phase != phase {
			t.Fatalf("expected short-cycle phases %v, got %#v", expectedPhases, resp.Tasks)
		}
		if resp.Tasks[index].DayNumber != expectedDays[index] {
			t.Fatalf("expected short-cycle day windows %v, got %#v", expectedDays, resp.Tasks)
		}
	}
	if resp.Phase != model.LearningPhaseFoundation {
		t.Fatalf("expected short-cycle plan to enter foundation first, got %#v", resp)
	}
}

// TestPlanServiceGeneratePlanCompactsMediumDurationPhaseWindows 验证 14-20 天周期会保留 review 后的小规模 mock 窗口。
func TestPlanServiceGeneratePlanCompactsMediumDurationPhaseWindows(t *testing.T) {
	t.Parallel()

	goIndustry := &model.Industry{
		BaseModel: model.BaseModel{ID: 12},
		Code:      "go",
		Name:      "Go",
	}
	planRepo := &stubPlanRepository{}
	taskRepo := &stubPlanTaskRepository{}
	agent := &stubPlanAgent{
		generatedPlan: ai.LearningPlan{
			Title:       "中周期阶段计划",
			Description: "验证中周期阶段收口",
			Duration:    18,
			Tasks: []ai.PlanTask{
				{
					Title:       "专项练习：逃逸分析",
					Description: "先给一个 drill 任务",
					TaskType:    model.TaskTypePractice,
					Phase:       model.LearningPhaseDrill,
					DayNumber:   1,
					Priority:    "high",
				},
				{
					Title:       "复盘：内存模型",
					Description: "中后段做 review 收口",
					TaskType:    model.TaskTypeReview,
					Phase:       model.LearningPhaseReview,
					DayNumber:   2,
					Priority:    "medium",
				},
				{
					Title:       "概念补齐：调度器",
					Description: "仍需先回到 foundation",
					TaskType:    model.TaskTypeStudy,
					Phase:       model.LearningPhaseFoundation,
					DayNumber:   3,
					Priority:    "high",
				},
				{
					Title:       "限时模拟：并发问答",
					Description: "最后做一轮轻 mock",
					TaskType:    model.TaskTypeInterview,
					Phase:       model.LearningPhaseMock,
					DayNumber:   4,
					Priority:    "low",
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

	svc := NewPlanService(planRepo, taskRepo, agent, nil, nil, nil, nil, nil, industryRepo)
	resp, err := svc.GeneratePlan(context.Background(), 43, &GeneratePlanRequest{
		Level:           "intermediate",
		DailyStudyTime:  90,
		WeakTopics:      []string{"逃逸分析", "调度器"},
		GoalDescription: "两周多内完成一轮复盘后再做轻 mock",
		DurationDays:    18,
		IndustryCode:    "go",
	})
	if err != nil {
		t.Fatalf("GeneratePlan returned error: %v", err)
	}

	if len(resp.Tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(resp.Tasks))
	}
	expectedPhases := []string{
		model.LearningPhaseFoundation,
		model.LearningPhaseDrill,
		model.LearningPhaseReview,
		model.LearningPhaseMock,
	}
	expectedDays := []int{1, 5, 10, 14}
	for index, phase := range expectedPhases {
		if resp.Tasks[index].Phase != phase {
			t.Fatalf("expected medium-cycle phases %v, got %#v", expectedPhases, resp.Tasks)
		}
		if resp.Tasks[index].DayNumber != expectedDays[index] {
			t.Fatalf("expected medium-cycle day windows %v, got %#v", expectedDays, resp.Tasks)
		}
	}
	if resp.Tasks[3].TaskType != model.TaskTypeInterview {
		t.Fatalf("expected medium-cycle final task to stay in mock window, got %#v", resp.Tasks[3])
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

// TestPlanServiceGetPlanFallsBackToDerivedPhase 验证旧计划缺少阶段字段时，详情接口仍会按任务类型回填阶段信息。
func TestPlanServiceGetPlanFallsBackToDerivedPhase(t *testing.T) {
	t.Parallel()

	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 18},
			UserID:         31,
			IndustryID:     9,
			Title:          "旧版计划",
			Description:    "未持久化阶段字段的历史数据",
			Status:         model.PlanStatusActive,
			TotalTasks:     2,
			CompletedTasks: 0,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel:   model.BaseModel{ID: 801},
				PlanID:      18,
				Title:       "复盘数组边界处理",
				Description: "先复盘，再继续训练",
				TaskType:    model.TaskTypeReview,
				Status:      model.TaskStatusPending,
				SortOrder:   0,
			},
			{
				BaseModel:   model.BaseModel{ID: 802},
				PlanID:      18,
				Title:       "继续做同类练习",
				Description: "围绕边界处理补练",
				TaskType:    model.TaskTypePractice,
				Status:      model.TaskStatusPending,
				SortOrder:   1,
			},
		},
	}
	industryRepo := &stubPlanIndustryRepository{
		byID: map[uint]*model.Industry{
			9: {
				BaseModel: model.BaseModel{ID: 9},
				Code:      "go",
				Name:      "Go",
			},
		},
	}

	svc := NewPlanService(planRepo, taskRepo, &stubPlanAgent{}, nil, nil, nil, nil, nil, industryRepo)
	resp, err := svc.GetPlan(context.Background(), 31, 18)
	if err != nil {
		t.Fatalf("GetPlan returned error: %v", err)
	}
	if resp.Phase != model.LearningPhaseReview || resp.PhaseGoal == "" {
		t.Fatalf("expected legacy plan phase to fall back to review, got %#v", resp)
	}
	if len(resp.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(resp.Tasks))
	}
	if resp.Tasks[0].Phase != model.LearningPhaseReview || resp.Tasks[1].Phase != model.LearningPhaseDrill {
		t.Fatalf("expected legacy tasks to derive phase fields, got %#v", resp.Tasks)
	}
}
