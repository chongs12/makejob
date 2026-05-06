package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

// TestPlanServiceAdjustPlanKeepsCompletedTasks 验证调整计划时会保留已完成任务，仅替换未完成任务。
func TestPlanServiceAdjustPlanKeepsCompletedTasks(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 7)
	completedAt := startDate.AddDate(0, 0, 1)
	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 5},
			UserID:         9,
			IndustryID:     3,
			Title:          "原始计划",
			Description:    "用于验证调整行为",
			Status:         model.PlanStatusActive,
			TotalTasks:     3,
			CompletedTasks: 1,
			StartDate:      &startDate,
			EndDate:        &endDate,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel:   model.BaseModel{ID: 101},
				PlanID:      5,
				Title:       "已完成 goroutine 补强任务",
				Description: "继续补齐 goroutine 弱项",
				TaskType:    model.TaskTypeStudy,
				Status:      model.TaskStatusCompleted,
				CompletedAt: &completedAt,
				SortOrder:   0,
			},
			{
				BaseModel: model.BaseModel{ID: 102},
				PlanID:    5,
				Title:     "旧进行中任务",
				TaskType:  model.TaskTypePractice,
				Status:    model.TaskStatusInProgress,
				SortOrder: 1,
			},
			{
				BaseModel: model.BaseModel{ID: 103},
				PlanID:    5,
				Title:     "旧待开始任务",
				TaskType:  model.TaskTypeReview,
				Status:    model.TaskStatusPending,
				SortOrder: 2,
			},
		},
	}
	agent := &stubPlanAgent{
		adjustedPlan: ai.LearningPlan{
			Title:       "调整后计划",
			Description: "新的后续任务安排",
			Duration:    10,
			Tasks: []ai.PlanTask{
				{
					Title:       "新任务 A：goroutine 实战",
					Description: "继续补齐 goroutine 弱项",
					TaskType:    model.TaskTypePractice,
					DayNumber:   1,
					Priority:    "high",
				},
				{
					Title:       "新任务 B",
					Description: "围绕阶段目标继续推进",
					TaskType:    model.TaskTypeInterview,
					DayNumber:   2,
					Priority:    "medium",
				},
			},
		},
	}
	storedPayload, err := buildPlanStoredPayload(ai.LearningPlan{
		Title:       "原始计划",
		Description: "用于验证调整行为",
		Tasks: []ai.PlanTask{
			{
				Title:       "已完成 goroutine 补强任务",
				Description: "继续补齐 goroutine 弱项",
				TaskType:    model.TaskTypeStudy,
				DayNumber:   1,
				Priority:    "high",
			},
			{
				Title:       "旧进行中任务",
				Description: "围绕阶段目标继续推进",
				TaskType:    model.TaskTypePractice,
				DayNumber:   2,
				Priority:    "medium",
			},
			{
				Title:       "旧待开始任务",
				Description: "围绕阶段目标继续推进",
				TaskType:    model.TaskTypeReview,
				DayNumber:   3,
				Priority:    "low",
			},
		},
	}, planStoredContext{
		IndustryCode:    "go",
		Level:           "beginner",
		WeakTopics:      []string{"goroutine"},
		GoalDescription: "完成 Go 并发目标训练",
		DailyStudyTime:  60,
		DurationDays:    7,
	})
	if err != nil {
		t.Fatalf("buildPlanStoredPayload returned error: %v", err)
	}
	planRepo.plan.PlanJSON = string(storedPayload)

	svc := &planService{
		planRepo:  planRepo,
		taskRepo:  taskRepo,
		planAgent: agent,
	}

	resp, err := svc.AdjustPlan(context.Background(), 9, 5)
	if err != nil {
		t.Fatalf("AdjustPlan returned error: %v", err)
	}

	if !taskRepo.deleteIncompleteCalled {
		t.Fatal("expected incomplete tasks to be deleted")
	}
	if len(taskRepo.createdTasks) != 2 {
		t.Fatalf("expected 2 new tasks, got %d", len(taskRepo.createdTasks))
	}
	if taskRepo.createdTasks[0].SortOrder != 1 {
		t.Fatalf("expected first new task sort order 1, got %d", taskRepo.createdTasks[0].SortOrder)
	}
	if planRepo.saved == nil {
		t.Fatal("expected adjusted plan to be saved")
	}
	if planRepo.saved.CompletedTasks != 1 {
		t.Fatalf("expected completed task count to remain 1, got %d", planRepo.saved.CompletedTasks)
	}
	if planRepo.saved.TotalTasks != 3 {
		t.Fatalf("expected total tasks to be 3, got %d", planRepo.saved.TotalTasks)
	}
	if len(resp.Tasks) != 3 {
		t.Fatalf("expected response to include preserved completed task and 2 new tasks, got %d", len(resp.Tasks))
	}
	if resp.Tasks[0].Title != "已完成 goroutine 补强任务" || resp.Tasks[0].Status != model.TaskStatusCompleted {
		t.Fatalf("expected first task to remain completed history, got %#v", resp.Tasks[0])
	}
	if resp.Tasks[0].Source != "weak_topic" || resp.Tasks[0].PriorityExplanation == "" {
		t.Fatalf("expected preserved task to keep explanation fields, got %#v", resp.Tasks[0])
	}
	if resp.Tasks[1].Source != "weak_topic" || resp.Tasks[1].SourceLabel != "弱项补强" {
		t.Fatalf("expected first new task to inherit weak topic explanation, got %#v", resp.Tasks[1])
	}
	if resp.Tasks[2].Source != "goal" || resp.Tasks[2].SourceLabel != "目标拆解" {
		t.Fatalf("expected second new task to inherit goal explanation, got %#v", resp.Tasks[2])
	}
}

// TestPlanServiceAdjustPlanFiltersFocusSignalsByIndustry 验证调整计划时只会写回当前行业下的训练重点信号。
func TestPlanServiceAdjustPlanFiltersFocusSignalsByIndustry(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 7)
	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 6},
			UserID:         10,
			IndustryID:     7,
			Title:          "Go 计划",
			Description:    "验证行业过滤",
			Status:         model.PlanStatusActive,
			TotalTasks:     2,
			CompletedTasks: 0,
			StartDate:      &startDate,
			EndDate:        &endDate,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel: model.BaseModel{ID: 201},
				PlanID:    6,
				Title:     "旧任务",
				TaskType:  model.TaskTypePractice,
				Status:    model.TaskStatusPending,
				SortOrder: 0,
			},
		},
	}
	agent := &stubPlanAgent{
		adjustedPlan: ai.LearningPlan{
			Title:       "调整后 Go 计划",
			Description: "新的后续任务安排",
			Duration:    7,
			Tasks: []ai.PlanTask{
				{
					Title:       "状态定义不清专项练习",
					Description: "围绕状态定义不清做一轮动态规划专项补练",
					TaskType:    model.TaskTypePractice,
					DayNumber:   1,
					Priority:    "high",
				},
			},
		},
	}
	storedPayload, err := buildPlanStoredPayload(ai.LearningPlan{
		Title:       "Go 计划",
		Description: "验证行业过滤",
	}, planStoredContext{
		IndustryCode: "go",
	})
	if err != nil {
		t.Fatalf("buildPlanStoredPayload returned error: %v", err)
	}
	planRepo.plan.PlanJSON = string(storedPayload)

	svc := &planService{
		planRepo:  planRepo,
		taskRepo:  taskRepo,
		planAgent: agent,
		learningArchiveRepo: growthLearningArchiveRepositoryStub{
			entries: []model.LearningArchiveEntry{
				{
					IndustryCode:    "go",
					SourceRef:       "practice:10:701",
					MistakeTagsJSON: `["状态定义不清"]`,
					SuggestionsJSON: `["先口述状态定义，再开始写代码。"]`,
				},
				{
					IndustryCode:    "java",
					SourceRef:       "practice:10:702",
					MistakeTagsJSON: `["边界条件生疏"]`,
					SuggestionsJSON: `["写完主流程后单独列一组边界样例再检查。"]`,
				},
			},
		},
		interviewRepo: &growthInterviewRepositoryStub{
			interviews: []model.MockInterview{
				{
					BaseModel:  model.BaseModel{ID: 801},
					IndustryID: 7,
					Status:     model.InterviewStatusCompleted,
					ReportJSON: `{"weaknesses":["状态定义不清"],"suggestions":["先口述状态定义，再开始写代码。"]}`,
				},
				{
					BaseModel:  model.BaseModel{ID: 802},
					IndustryID: 9,
					Status:     model.InterviewStatusCompleted,
					ReportJSON: `{"weaknesses":["边界条件生疏"],"suggestions":["写完主流程后单独列一组边界样例再检查。"]}`,
				},
			},
		},
	}

	resp, err := svc.AdjustPlan(context.Background(), 10, 6)
	if err != nil {
		t.Fatalf("AdjustPlan returned error: %v", err)
	}
	if planRepo.saved == nil {
		t.Fatal("expected adjusted plan to be saved")
	}

	stored := readPlanStoredPayload(planRepo.saved.PlanJSON)
	if len(stored.Context.FocusSignals) == 0 {
		t.Fatalf("expected saved plan to include focus signals, got %#v", stored.Context)
	}
	for _, signal := range stored.Context.FocusSignals {
		if signal.Tag == "边界条件生疏" {
			t.Fatalf("expected saved focus signals to exclude other industry, got %#v", stored.Context.FocusSignals)
		}
	}
	if len(resp.Tasks) != 1 {
		t.Fatalf("expected 1 adjusted task, got %d", len(resp.Tasks))
	}
	if resp.Tasks[0].SourceRef == "" || !strings.HasPrefix(resp.Tasks[0].SourceRef, "practice:10:701") {
		t.Fatalf("expected adjusted task to use go-industry source ref, got %#v", resp.Tasks[0])
	}
}

// stubPlanRepository 模拟学习计划仓库，供调整逻辑测试验证写回结果。
type stubPlanRepository struct {
	plan    *model.LearningPlan
	created *model.LearningPlan
	saved   *model.LearningPlan
}

// Create 记录服务层创建的学习计划，并为测试补一个稳定ID。
func (s *stubPlanRepository) Create(_ context.Context, plan *model.LearningPlan) error {
	clone := *plan
	if clone.ID == 0 {
		clone.ID = 1
		plan.ID = clone.ID
	}
	s.created = &clone
	s.plan = &clone
	return nil
}

// GetByID 返回预置学习计划。
func (s *stubPlanRepository) GetByID(context.Context, uint) (*model.LearningPlan, error) {
	if s.plan == nil {
		return nil, nil
	}
	clone := *s.plan
	return &clone, nil
}

// GetCurrentByUser 满足接口，当前测试不依赖该行为。
func (s *stubPlanRepository) GetCurrentByUser(context.Context, uint) (*model.LearningPlan, error) {
	return nil, nil
}

// Update 记录服务层写回的学习计划。
func (s *stubPlanRepository) Update(_ context.Context, plan *model.LearningPlan) error {
	clone := *plan
	s.saved = &clone
	s.plan = &clone
	return nil
}

// ListByUser 满足接口，当前测试不依赖该行为。
func (s *stubPlanRepository) ListByUser(context.Context, uint, int, int) ([]model.LearningPlan, int64, error) {
	return nil, 0, nil
}

// PauseActivePlans 满足接口，当前测试不依赖该行为。
func (s *stubPlanRepository) PauseActivePlans(context.Context, uint) error {
	return nil
}

// stubPlanTaskRepository 模拟学习任务仓库，供测试观察删除和新建行为。
type stubPlanTaskRepository struct {
	tasks                  []model.LearningTask
	createdTasks           []model.LearningTask
	deleteIncompleteCalled bool
}

// Create 满足接口，当前测试不依赖该行为。
func (s *stubPlanTaskRepository) Create(context.Context, *model.LearningTask) error {
	return nil
}

// BatchCreate 记录新建任务列表。
func (s *stubPlanTaskRepository) BatchCreate(_ context.Context, tasks []model.LearningTask) error {
	s.createdTasks = append([]model.LearningTask(nil), tasks...)
	return nil
}

// GetByID 满足接口，当前测试不依赖该行为。
func (s *stubPlanTaskRepository) GetByID(context.Context, uint) (*model.LearningTask, error) {
	return nil, nil
}

// Update 满足接口，当前测试不依赖该行为。
func (s *stubPlanTaskRepository) Update(context.Context, *model.LearningTask) error {
	return nil
}

// ListByPlan 返回预置任务列表。
func (s *stubPlanTaskRepository) ListByPlan(context.Context, uint) ([]model.LearningTask, error) {
	return append([]model.LearningTask(nil), s.tasks...), nil
}

// CountByPlanAndStatus 满足接口，当前测试不依赖该行为。
func (s *stubPlanTaskRepository) CountByPlanAndStatus(context.Context, uint, string) (int64, error) {
	return 0, nil
}

// DeleteByPlan 满足接口，当前测试不依赖该行为。
func (s *stubPlanTaskRepository) DeleteByPlan(context.Context, uint) error {
	return nil
}

// DeleteIncompleteByPlan 记录仅删除未完成任务的调用。
func (s *stubPlanTaskRepository) DeleteIncompleteByPlan(context.Context, uint) error {
	s.deleteIncompleteCalled = true
	return nil
}

// stubPlanAgent 模拟计划调整 Agent，返回预置调整结果。
type stubPlanAgent struct {
	generatedPlan        ai.LearningPlan
	adjustedPlan         ai.LearningPlan
	lastGenerateProfile  ai.UserProfile
	lastGenerateIndustry string
}

// GeneratePlan 返回测试预置的学习计划结果。
func (s *stubPlanAgent) GeneratePlan(_ context.Context, profile ai.UserProfile, industryCode string) (ai.LearningPlan, error) {
	s.lastGenerateProfile = profile
	s.lastGenerateIndustry = industryCode
	return s.generatedPlan, nil
}

// AdjustPlan 返回测试预置的调整后计划。
func (s *stubPlanAgent) AdjustPlan(context.Context, string, []string, map[string]float64) (ai.LearningPlan, error) {
	return s.adjustedPlan, nil
}

// GetStudySuggestion 满足接口，当前测试不依赖该行为。
func (s *stubPlanAgent) GetStudySuggestion(context.Context, ai.UserProfile) (string, error) {
	return "", nil
}
