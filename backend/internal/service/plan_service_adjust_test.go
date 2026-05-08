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

// TestPlanServiceAdjustPlanAppliesDiagnosisActions 验证 AdjustPlan 会消费未解决弱点的诊断动作，在后续窗口插入复盘并收紧训练描述。
func TestPlanServiceAdjustPlanAppliesDiagnosisActions(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 7)
	completedAt := startDate.AddDate(0, 0, 1)
	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 11},
			UserID:         18,
			IndustryID:     3,
			Title:          "原始计划",
			Description:    "验证诊断动作应用",
			Status:         model.PlanStatusActive,
			TotalTasks:     2,
			CompletedTasks: 1,
			StartDate:      &startDate,
			EndDate:        &endDate,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel:   model.BaseModel{ID: 301},
				PlanID:      11,
				Title:       "已完成 DP 练习",
				Description: "第一次状态定义写偏了",
				TaskType:    model.TaskTypePractice,
				Status:      model.TaskStatusCompleted,
				CompletedAt: &completedAt,
				SortOrder:   0,
			},
			{
				BaseModel: model.BaseModel{ID: 302},
				PlanID:    11,
				Title:     "旧待开始任务",
				TaskType:  model.TaskTypePractice,
				Status:    model.TaskStatusPending,
				SortOrder: 1,
			},
		},
	}
	agent := &stubPlanAgent{
		adjustedPlan: ai.LearningPlan{
			Title:       "调整后计划",
			Description: "新的后续任务安排",
			Duration:    7,
			Tasks: []ai.PlanTask{
				{
					Title:       "动态规划变形题",
					Description: "继续做一轮动态规划练习",
					TaskType:    model.TaskTypePractice,
					DayNumber:   1,
					Priority:    "medium",
				},
			},
		},
	}
	storedPayload, err := buildPlanStoredPayload(ai.LearningPlan{
		Title:       "原始计划",
		Description: "验证诊断动作应用",
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
		diagnosisRepo: stubPlanTaskDiagnosisRepository{diagnoses: []model.LearningTaskDiagnosis{
			{
				FeedbackID:      601,
				TaskID:          301,
				WeaknessStatus:  model.LearningTaskDiagnosisWeaknessUnresolved,
				ActionJSON:      `[{"action":"add_review_task","target_focus_tag":"状态定义不清","collection_hint":"algorithm-structure","reason":"需要先复盘再继续训练"},{"action":"repeat_same_pattern","target_focus_tag":"状态定义不清","collection_hint":"algorithm-structure","reason":"同类错误仍在反复出现"}]`,
				Summary:         "动态规划状态定义仍不稳定，先复盘再继续同类训练。",
				MistakeTagsJSON: `["状态定义不清"]`,
			},
		}},
	}

	resp, err := svc.AdjustPlan(context.Background(), 18, 11)
	if err != nil {
		t.Fatalf("AdjustPlan returned error: %v", err)
	}

	if len(taskRepo.createdTasks) != 2 {
		t.Fatalf("expected diagnosis actions to produce 2 tasks, got %d", len(taskRepo.createdTasks))
	}
	if taskRepo.createdTasks[0].TaskType != model.TaskTypeReview || !strings.Contains(taskRepo.createdTasks[0].Title, "复盘：状态定义不清") {
		t.Fatalf("expected first created task to be diagnosis review task, got %#v", taskRepo.createdTasks[0])
	}
	if taskRepo.createdTasks[0].Phase != model.LearningPhaseReview || taskRepo.createdTasks[0].PhaseGoal == "" {
		t.Fatalf("expected diagnosis review task to carry review phase fields, got %#v", taskRepo.createdTasks[0])
	}
	if !strings.Contains(taskRepo.createdTasks[1].Title, "同类巩固") {
		t.Fatalf("expected second created task title to indicate same-pattern drill, got %#v", taskRepo.createdTasks[1])
	}
	if !strings.Contains(taskRepo.createdTasks[1].Description, "同类模式训练") {
		t.Fatalf("expected second created task description to include same-pattern guidance, got %#v", taskRepo.createdTasks[1])
	}
	if taskRepo.createdTasks[1].Phase != model.LearningPhaseDrill || taskRepo.createdTasks[1].PhaseGoal == "" {
		t.Fatalf("expected diagnosis drill task to carry drill phase fields, got %#v", taskRepo.createdTasks[1])
	}
	if planRepo.saved == nil || !strings.Contains(planRepo.saved.Description, "本轮调整重点") {
		t.Fatalf("expected saved plan description to include diagnosis summary, got %#v", planRepo.saved)
	}
	if planRepo.saved.Phase != model.LearningPhaseReview || planRepo.saved.PhaseGoal == "" {
		t.Fatalf("expected adjusted plan to persist current phase fields, got %#v", planRepo.saved)
	}
	if len(resp.Tasks) != 3 {
		t.Fatalf("expected response to include completed history plus 2 adjusted tasks, got %d", len(resp.Tasks))
	}
	if resp.Phase != model.LearningPhaseReview || resp.PhaseGoal == "" {
		t.Fatalf("expected adjusted response to expose current phase fields, got %#v", resp)
	}
	if resp.EntryPhase != model.LearningPhaseReview {
		t.Fatalf("expected adjusted response to expose review entry phase, got %#v", resp)
	}
	if len(resp.AdjustmentSummaries) != 1 || !strings.Contains(resp.AdjustmentSummaries[0], "先复盘再继续同类训练") {
		t.Fatalf("expected adjusted response to expose diagnosis summaries, got %#v", resp.AdjustmentSummaries)
	}
	if resp.Tasks[1].Source != "plan_feedback_diagnosis" || resp.Tasks[1].SourceLabel != "训练反馈诊断" {
		t.Fatalf("expected diagnosis review task to expose diagnosis source, got %#v", resp.Tasks[1])
	}
	if resp.Tasks[1].SourceRef != "plan-feedback:601" || resp.Tasks[1].CollectionHint != "algorithm-structure" {
		t.Fatalf("expected diagnosis review task to expose source ref and collection hint, got %#v", resp.Tasks[1])
	}
	if resp.Tasks[2].Source != "plan_feedback_diagnosis" || resp.Tasks[2].CollectionHint != "algorithm-structure" {
		t.Fatalf("expected diagnosis drill task to keep diagnosis explanation fields, got %#v", resp.Tasks[2])
	}
	stored := readPlanStoredPayload(planRepo.saved.PlanJSON)
	if stored.Context.EntryPhase != model.LearningPhaseReview {
		t.Fatalf("expected saved plan context to keep review entry phase, got %#v", stored.Context)
	}
	if len(stored.Context.AdjustmentSummaries) != 1 || !strings.Contains(stored.Context.AdjustmentSummaries[0], "先复盘再继续同类训练") {
		t.Fatalf("expected saved plan context to keep diagnosis summaries, got %#v", stored.Context)
	}
	taskRepo.tasks = append([]model.LearningTask{taskRepo.tasks[0]}, taskRepo.createdTasks...)
	persistedResp, err := svc.GetPlan(context.Background(), 18, 11)
	if err != nil {
		t.Fatalf("GetPlan returned error after adjust: %v", err)
	}
	if persistedResp.EntryPhase != model.LearningPhaseReview || len(persistedResp.AdjustmentSummaries) != 1 {
		t.Fatalf("expected GetPlan to read back adjustment explanation fields, got %#v", persistedResp)
	}
	if agent.lastAdjustInput.CurrentPhase != model.LearningPhaseDrill || agent.lastAdjustInput.EntryPhase != model.LearningPhaseReview {
		t.Fatalf("expected unresolved path to send drill->review phase input, got %#v", agent.lastAdjustInput)
	}
}

// TestPlanServiceAdjustPlanUsesDiagnosisPerformance 验证 AdjustPlan 会用诊断结果覆盖旧的硬编码表现分，并在已改善路径上提升后续任务难度说明。
func TestPlanServiceAdjustPlanUsesDiagnosisPerformance(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 7)
	completedAt := startDate.AddDate(0, 0, 1)
	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 12},
			UserID:         19,
			IndustryID:     3,
			Title:          "原始计划",
			Description:    "验证诊断表现分",
			Status:         model.PlanStatusActive,
			TotalTasks:     2,
			CompletedTasks: 1,
			StartDate:      &startDate,
			EndDate:        &endDate,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel:   model.BaseModel{ID: 401},
				PlanID:      12,
				Title:       "已完成链表题",
				Description: "完成较顺利",
				TaskType:    model.TaskTypePractice,
				Status:      model.TaskStatusCompleted,
				CompletedAt: &completedAt,
				SortOrder:   0,
			},
		},
	}
	agent := &stubPlanAgent{
		adjustedPlan: ai.LearningPlan{
			Title:       "调整后计划",
			Description: "新的后续任务安排",
			Duration:    7,
			Tasks: []ai.PlanTask{
				{
					Title:       "链表综合题",
					Description: "继续做一轮链表训练",
					TaskType:    model.TaskTypePractice,
					DayNumber:   1,
					Priority:    "medium",
				},
			},
		},
	}

	svc := &planService{
		planRepo:  planRepo,
		taskRepo:  taskRepo,
		planAgent: agent,
		diagnosisRepo: stubPlanTaskDiagnosisRepository{diagnoses: []model.LearningTaskDiagnosis{
			{
				FeedbackID:     701,
				TaskID:         401,
				WeaknessStatus: model.LearningTaskDiagnosisWeaknessImproved,
				ActionJSON:     `[{"action":"raise_difficulty","target_focus_tag":"链表指针控制","collection_hint":"linked-list-advance","reason":"当前同类任务通过较快，可以提升一档复杂度"}]`,
				Summary:        "链表指针控制已较稳定，可以继续升难度。",
			},
		}},
	}

	resp, err := svc.AdjustPlan(context.Background(), 19, 12)
	if err != nil {
		t.Fatalf("AdjustPlan returned error: %v", err)
	}

	if score := agent.lastAdjustInput.Performance[model.TaskTypePractice]; score < 90 {
		t.Fatalf("expected diagnosis-driven practice performance >= 90, got %v", agent.lastAdjustInput.Performance)
	}
	if agent.lastAdjustInput.CurrentPhase != model.LearningPhaseDrill || agent.lastAdjustInput.EntryPhase != model.LearningPhaseMock {
		t.Fatalf("expected improved path to send drill->mock phase input, got %#v", agent.lastAdjustInput)
	}
	if len(taskRepo.createdTasks) != 1 {
		t.Fatalf("expected one adjusted task, got %d", len(taskRepo.createdTasks))
	}
	if taskRepo.createdTasks[0].Phase != model.LearningPhaseMock || taskRepo.createdTasks[0].PhaseGoal == "" {
		t.Fatalf("expected raised-difficulty task to unlock mock phase, got %#v", taskRepo.createdTasks[0])
	}
	if !strings.Contains(taskRepo.createdTasks[0].Title, "进阶推进") {
		t.Fatalf("expected adjusted task title to show raised difficulty, got %#v", taskRepo.createdTasks[0])
	}
	if !strings.Contains(taskRepo.createdTasks[0].Description, "复杂度提升一档") {
		t.Fatalf("expected adjusted task description to mention raised difficulty, got %#v", taskRepo.createdTasks[0])
	}
	if len(resp.Tasks) != 2 {
		t.Fatalf("expected response to include completed history plus 1 adjusted task, got %d", len(resp.Tasks))
	}
	if planRepo.saved == nil || planRepo.saved.Phase != model.LearningPhaseMock || planRepo.saved.PhaseGoal == "" {
		t.Fatalf("expected improved path to persist next phase, got %#v", planRepo.saved)
	}
	if resp.Phase != model.LearningPhaseMock || resp.PhaseGoal == "" {
		t.Fatalf("expected improved path response to expose next phase, got %#v", resp)
	}
	if resp.EntryPhase != model.LearningPhaseMock {
		t.Fatalf("expected improved path response to expose mock entry phase, got %#v", resp)
	}
	if len(resp.AdjustmentSummaries) != 1 || !strings.Contains(resp.AdjustmentSummaries[0], "继续升难度") {
		t.Fatalf("expected improved path response to expose diagnosis summaries, got %#v", resp.AdjustmentSummaries)
	}
	if resp.Tasks[1].Source != "plan_feedback_diagnosis" || resp.Tasks[1].SourceRef != "plan-feedback:701" {
		t.Fatalf("expected raised-difficulty task to expose diagnosis source ref, got %#v", resp.Tasks[1])
	}
	if resp.Tasks[1].CollectionHint != "linked-list-advance" {
		t.Fatalf("expected raised-difficulty task to expose collection hint, got %#v", resp.Tasks[1])
	}
	stored := readPlanStoredPayload(planRepo.saved.PlanJSON)
	if stored.Context.EntryPhase != model.LearningPhaseMock {
		t.Fatalf("expected improved path saved context to keep mock entry phase, got %#v", stored.Context)
	}
	if len(stored.Context.AdjustmentSummaries) != 1 || !strings.Contains(stored.Context.AdjustmentSummaries[0], "继续升难度") {
		t.Fatalf("expected improved path saved context to keep diagnosis summaries, got %#v", stored.Context)
	}
}

// TestPlanServiceAdjustPlanPersistsReasonCodes 验证 AdjustPlan 会将诊断动作映射为机器可读原因码并持久化到 plan_json.context。
func TestPlanServiceAdjustPlanPersistsReasonCodes(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 7)
	completedAt := startDate.AddDate(0, 0, 1)
	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 20},
			UserID:         28,
			IndustryID:     3,
			Title:          "原因码验证计划",
			Description:    "验证 reason codes 持久化",
			Status:         model.PlanStatusActive,
			TotalTasks:     2,
			CompletedTasks: 1,
			StartDate:      &startDate,
			EndDate:        &endDate,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel:   model.BaseModel{ID: 501},
				PlanID:      20,
				Title:       "已完成 DP 练习",
				Description: "状态定义写偏了",
				TaskType:    model.TaskTypePractice,
				Status:      model.TaskStatusCompleted,
				CompletedAt: &completedAt,
				SortOrder:   0,
			},
			{
				BaseModel: model.BaseModel{ID: 502},
				PlanID:    20,
				Title:     "旧待开始任务",
				TaskType:  model.TaskTypePractice,
				Status:    model.TaskStatusPending,
				SortOrder: 1,
			},
		},
	}
	agent := &stubPlanAgent{
		adjustedPlan: ai.LearningPlan{
			Title:       "调整后计划",
			Description: "新的后续任务安排",
			Duration:    7,
			Tasks: []ai.PlanTask{
				{
					Title:       "动态规划变形题",
					Description: "继续做一轮动态规划练习",
					TaskType:    model.TaskTypePractice,
					DayNumber:   1,
					Priority:    "medium",
				},
			},
		},
	}
	storedPayload, _ := buildPlanStoredPayload(ai.LearningPlan{
		Title:    "原因码验证计划",
		Duration: 7,
	}, planStoredContext{
		IndustryCode: "go",
	})
	planRepo.plan.PlanJSON = string(storedPayload)

	svc := &planService{
		planRepo:  planRepo,
		taskRepo:  taskRepo,
		planAgent: agent,
		diagnosisRepo: stubPlanTaskDiagnosisRepository{diagnoses: []model.LearningTaskDiagnosis{
			{
				FeedbackID:     801,
				TaskID:         501,
				WeaknessStatus: model.LearningTaskDiagnosisWeaknessUnresolved,
				ActionJSON:     `[{"action":"add_review_task","target_focus_tag":"状态定义不清","reason":"需要先复盘"}]`,
				Summary:        "状态定义仍不稳定，先复盘。",
			},
		}},
	}

	_, err := svc.AdjustPlan(context.Background(), 28, 20)
	if err != nil {
		t.Fatalf("AdjustPlan returned error: %v", err)
	}

	if planRepo.saved == nil {
		t.Fatal("expected adjusted plan to be saved")
	}
	stored := readPlanStoredPayload(planRepo.saved.PlanJSON)
	if len(stored.Context.AdjustmentReasonCodes) == 0 {
		t.Fatal("expected saved context to include adjustment_reason_codes")
	}
	foundWeakness := false
	for _, code := range stored.Context.AdjustmentReasonCodes {
		if code == "weakness_unresolved" {
			foundWeakness = true
		}
	}
	if !foundWeakness {
		t.Fatalf("expected weakness_unresolved in reason codes, got %v", stored.Context.AdjustmentReasonCodes)
	}
}

// TestPlanServiceAdjustPlanPersistsTransitionHistory 验证 AdjustPlan 会在阶段切换时记录阶段过渡历史。
func TestPlanServiceAdjustPlanPersistsTransitionHistory(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 7)
	completedAt := startDate.AddDate(0, 0, 1)
	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 21},
			UserID:         29,
			IndustryID:     3,
			Title:          "过渡历史验证计划",
			Description:    "验证 phase_transition_history 持久化",
			Status:         model.PlanStatusActive,
			TotalTasks:     2,
			CompletedTasks: 1,
			StartDate:      &startDate,
			EndDate:        &endDate,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel:   model.BaseModel{ID: 601},
				PlanID:      21,
				Title:       "已完成链表题",
				Description: "完成较顺利",
				TaskType:    model.TaskTypePractice,
				Status:      model.TaskStatusCompleted,
				CompletedAt: &completedAt,
				SortOrder:   0,
			},
			{
				BaseModel: model.BaseModel{ID: 602},
				PlanID:    21,
				Title:     "旧待开始任务",
				TaskType:  model.TaskTypePractice,
				Status:    model.TaskStatusPending,
				SortOrder: 1,
			},
		},
	}
	agent := &stubPlanAgent{
		adjustedPlan: ai.LearningPlan{
			Title:       "调整后计划",
			Description: "新的后续任务安排",
			Duration:    7,
			Tasks: []ai.PlanTask{
				{
					Title:       "链表综合题",
					Description: "继续做一轮链表训练",
					TaskType:    model.TaskTypePractice,
					DayNumber:   1,
					Priority:    "medium",
				},
			},
		},
	}
	storedPayload, _ := buildPlanStoredPayload(ai.LearningPlan{
		Title:    "过渡历史验证计划",
		Duration: 7,
	}, planStoredContext{
		IndustryCode: "go",
	})
	planRepo.plan.PlanJSON = string(storedPayload)

	svc := &planService{
		planRepo:  planRepo,
		taskRepo:  taskRepo,
		planAgent: agent,
		diagnosisRepo: stubPlanTaskDiagnosisRepository{diagnoses: []model.LearningTaskDiagnosis{
			{
				FeedbackID:     901,
				TaskID:         601,
				WeaknessStatus: model.LearningTaskDiagnosisWeaknessImproved,
				ActionJSON:     `[{"action":"raise_difficulty","target_focus_tag":"链表指针控制","reason":"当前同类任务通过较快"}]`,
				Summary:        "链表指针控制已较稳定。",
			},
		}},
	}

	_, err := svc.AdjustPlan(context.Background(), 29, 21)
	if err != nil {
		t.Fatalf("AdjustPlan returned error: %v", err)
	}

	if planRepo.saved == nil {
		t.Fatal("expected adjusted plan to be saved")
	}
	stored := readPlanStoredPayload(planRepo.saved.PlanJSON)
	if len(stored.Context.PhaseTransitionHistory) == 0 {
		t.Fatal("expected saved context to include phase_transition_history")
	}

	lastEntry := stored.Context.PhaseTransitionHistory[len(stored.Context.PhaseTransitionHistory)-1]
	if lastEntry.At == "" {
		t.Fatal("expected transition entry to have timestamp")
	}
	if lastEntry.FromPhase != model.LearningPhaseDrill {
		t.Fatalf("expected from_phase drill, got %s", lastEntry.FromPhase)
	}
	if lastEntry.EntryPhase != model.LearningPhaseMock {
		t.Fatalf("expected entry_phase mock, got %s", lastEntry.EntryPhase)
	}
	if len(lastEntry.ReasonCodes) == 0 {
		t.Fatal("expected transition entry to have reason codes")
	}
}

// TestPlanServiceAdjustPlanBackwardCompatibility 验证旧计划（没有新字段）在调整后不会报错，新字段会被正常补充。
func TestPlanServiceAdjustPlanBackwardCompatibility(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 7)
	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 30},
			UserID:         39,
			IndustryID:     3,
			Title:          "旧版计划",
			Description:    "没有新字段的历史数据",
			Status:         model.PlanStatusActive,
			TotalTasks:     1,
			CompletedTasks: 0,
			StartDate:      &startDate,
			EndDate:        &endDate,
			PlanJSON:       `{"plan":{"title":"旧版计划","description":"没有新字段的历史数据","duration_days":7,"tasks":[{"title":"旧任务","description":"旧描述","task_type":"study","day_number":1,"duration_minutes":60,"priority":"medium"}]},"context":{"industry_code":"go","level":"beginner","duration_days":7}}`,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel: model.BaseModel{ID: 701},
				PlanID:    30,
				Title:     "旧任务",
				TaskType:  model.TaskTypeStudy,
				Status:    model.TaskStatusPending,
				SortOrder: 0,
			},
		},
	}
	agent := &stubPlanAgent{
		adjustedPlan: ai.LearningPlan{
			Title:       "调整后计划",
			Description: "新的后续任务安排",
			Duration:    7,
			Tasks: []ai.PlanTask{
				{
					Title:       "新任务",
					Description: "新描述",
					TaskType:    model.TaskTypePractice,
					DayNumber:   1,
					Priority:    "medium",
				},
			},
		},
	}

	svc := &planService{
		planRepo:  planRepo,
		taskRepo:  taskRepo,
		planAgent: agent,
	}

	resp, err := svc.AdjustPlan(context.Background(), 39, 30)
	if err != nil {
		t.Fatalf("AdjustPlan on legacy plan returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	stored := readPlanStoredPayload(planRepo.saved.PlanJSON)
	if stored.Context.IndustryCode != "go" {
		t.Fatalf("expected industry code preserved, got %s", stored.Context.IndustryCode)
	}
}

// TestPlanServiceGetPlanBackwardCompatibility 验证旧计划读取时新字段缺失不会报错，且会按任务列表兜底推导阶段蓝图。
func TestPlanServiceGetPlanBackwardCompatibility(t *testing.T) {
	t.Parallel()

	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 31},
			UserID:         40,
			IndustryID:     9,
			Title:          "旧版计划",
			Description:    "没有新字段的历史数据",
			Status:         model.PlanStatusActive,
			TotalTasks:     1,
			CompletedTasks: 0,
			PlanJSON:       `{"plan":{"title":"旧版计划","description":"旧数据","duration_days":7,"tasks":[{"title":"旧任务","description":"旧描述","task_type":"study","day_number":1,"duration_minutes":60,"priority":"medium"}]},"context":{"industry_code":"go","level":"beginner"}}`,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel: model.BaseModel{ID: 801},
				PlanID:    31,
				Title:     "旧任务",
				TaskType:  model.TaskTypeStudy,
				Status:    model.TaskStatusPending,
				SortOrder: 0,
			},
		},
	}
	industryRepo := &stubPlanIndustryRepository{
		byID: map[uint]*model.Industry{
			9: {BaseModel: model.BaseModel{ID: 9}, Code: "go", Name: "Go"},
		},
	}

	svc := NewPlanService(planRepo, taskRepo, &stubPlanAgent{}, nil, nil, nil, nil, nil, industryRepo)
	resp, err := svc.GetPlan(context.Background(), 40, 31)
	if err != nil {
		t.Fatalf("GetPlan on legacy plan returned error: %v", err)
	}
	if resp.AdjustmentReasonCodes != nil {
		t.Fatalf("expected nil reason codes for legacy plan, got %v", resp.AdjustmentReasonCodes)
	}
	// 旧计划没有 phase_blueprint，但 resolvePlanContextPhaseBlueprint 会从存储的任务列表兜底推导。
	if len(resp.PhaseBlueprintSummary) == 0 {
		t.Fatal("expected non-empty blueprint summary for legacy plan via fallback derivation")
	}
}

// TestPlanServiceAdjustPlanMergesTaskExplanations 验证 AdjustPlan 合并新旧任务解释上下文，保留已完成历史任务的解释链路。
func TestPlanServiceAdjustPlanMergesTaskExplanations(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 14)
	completedAt := startDate.AddDate(0, 0, 1)
	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 40},
			UserID:         48,
			IndustryID:     3,
			Title:          "解释合并验证计划",
			Description:    "验证 explanation merge",
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
				BaseModel:   model.BaseModel{ID: 901},
				PlanID:      40,
				Title:       "已完成 goroutine 补强任务",
				Description: "继续补齐 goroutine 弱项",
				TaskType:    model.TaskTypeStudy,
				Phase:       model.LearningPhaseFoundation,
				PhaseGoal:   "补基础",
				Status:      model.TaskStatusCompleted,
				CompletedAt: &completedAt,
				SortOrder:   0,
			},
			{
				BaseModel:   model.BaseModel{ID: 902},
				PlanID:      40,
				Title:       "旧待开始任务 A",
				Description: "旧描述 A",
				TaskType:    model.TaskTypePractice,
				Phase:       model.LearningPhaseDrill,
				PhaseGoal:   "专项训练",
				Status:      model.TaskStatusPending,
				SortOrder:   1,
			},
			{
				BaseModel:   model.BaseModel{ID: 903},
				PlanID:      40,
				Title:       "旧待开始任务 B",
				Description: "旧描述 B",
				TaskType:    model.TaskTypeReview,
				Phase:       model.LearningPhaseReview,
				PhaseGoal:   "复盘",
				Status:      model.TaskStatusPending,
				SortOrder:   2,
			},
		},
	}
	agent := &stubPlanAgent{
		adjustedPlan: ai.LearningPlan{
			Title:       "调整后计划",
			Description: "新的后续任务安排",
			Duration:    14,
			Tasks: []ai.PlanTask{
				{
					Title:       "新任务 C",
					Description: "新描述 C",
					TaskType:    model.TaskTypePractice,
					Phase:       model.LearningPhaseDrill,
					PhaseGoal:   "专项训练",
					DayNumber:   1,
					Priority:    "medium",
				},
			},
		},
	}

	// 构造已有上下文，其中已完成任务有诊断解释，旧 pending 任务也有解释。
	storedPayload, _ := buildPlanStoredPayload(ai.LearningPlan{
		Title:       "解释合并验证计划",
		Description: "验证 explanation merge",
		Duration:    14,
		Tasks: []ai.PlanTask{
			{Title: "已完成 goroutine 补强任务", Description: "继续补齐 goroutine 弱项", TaskType: model.TaskTypeStudy, DayNumber: 1},
			{Title: "旧待开始任务 A", Description: "旧描述 A", TaskType: model.TaskTypePractice, DayNumber: 2},
			{Title: "旧待开始任务 B", Description: "旧描述 B", TaskType: model.TaskTypeReview, DayNumber: 3},
		},
	}, planStoredContext{
		IndustryCode: "go",
		DurationDays: 14,
		TaskExplanations: map[string]planTaskResponseContext{
			buildPlanTaskLookupKey("已完成 goroutine 补强任务", "继续补齐 goroutine 弱项", model.TaskTypeStudy): {
				Source:      "plan_feedback_diagnosis",
				SourceLabel: "训练反馈诊断",
				Reason:      "来自诊断的解释",
				SourceRef:   "plan-feedback:901",
			},
			buildPlanTaskLookupKey("旧待开始任务 A", "旧描述 A", model.TaskTypePractice): {
				Source:      "weak_topic",
				SourceLabel: "弱项补强",
				Reason:      "旧任务 A 的解释",
			},
			buildPlanTaskLookupKey("旧待开始任务 B", "旧描述 B", model.TaskTypeReview): {
				Source:      "default",
				SourceLabel: "基础默认任务",
				Reason:      "旧任务 B 的解释，应该被裁剪",
			},
		},
	})
	planRepo.plan.PlanJSON = string(storedPayload)

	svc := &planService{
		planRepo:  planRepo,
		taskRepo:  taskRepo,
		planAgent: agent,
	}

	resp, err := svc.AdjustPlan(context.Background(), 48, 40)
	if err != nil {
		t.Fatalf("AdjustPlan returned error: %v", err)
	}

	// 已完成任务的解释应保留。
	if resp.Tasks[0].Source != "plan_feedback_diagnosis" || resp.Tasks[0].SourceRef != "plan-feedback:901" {
		t.Fatalf("expected completed task explanation preserved, got %#v", resp.Tasks[0])
	}

	// 旧 pending 任务 B 已被删除，其解释应被裁剪。
	stored := readPlanStoredPayload(planRepo.saved.PlanJSON)
	deletedKey := buildPlanTaskLookupKey("旧待开始任务 B", "旧描述 B", model.TaskTypeReview)
	if _, exists := stored.Context.TaskExplanations[deletedKey]; exists {
		t.Fatal("expected deleted pending task explanation to be pruned")
	}

	// 新任务应有解释。
	if len(resp.Tasks) < 2 {
		t.Fatalf("expected at least 2 tasks in response, got %d", len(resp.Tasks))
	}
}

// TestPlanServiceAdjustPlanRebuildsBlueprint 验证 AdjustPlan 后蓝图会按新入口阶段重建，而非只改 source。
func TestPlanServiceAdjustPlanRebuildsBlueprint(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 14)
	completedAt := startDate.AddDate(0, 0, 1)
	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 41},
			UserID:         49,
			IndustryID:     3,
			Title:          "蓝图重建验证计划",
			Description:    "验证 blueprint rebuild",
			Status:         model.PlanStatusActive,
			TotalTasks:     2,
			CompletedTasks: 1,
			StartDate:      &startDate,
			EndDate:        &endDate,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel:   model.BaseModel{ID: 911},
				PlanID:      41,
				Title:       "已完成 drill 任务",
				Description: "drill 描述",
				TaskType:    model.TaskTypePractice,
				Phase:       model.LearningPhaseDrill,
				Status:      model.TaskStatusCompleted,
				CompletedAt: &completedAt,
				SortOrder:   0,
			},
			{
				BaseModel: model.BaseModel{ID: 912},
				PlanID:    41,
				Title:     "旧待开始任务",
				TaskType:  model.TaskTypePractice,
				Phase:     model.LearningPhaseDrill,
				Status:    model.TaskStatusPending,
				SortOrder: 1,
			},
		},
	}
	// AI 返回的调整后计划首阶段是 review（drill -> review 路径）。
	agent := &stubPlanAgent{
		adjustedPlan: ai.LearningPlan{
			Title:       "调整后计划",
			Description: "进入复盘阶段",
			Duration:    14,
			Tasks: []ai.PlanTask{
				{
					Title:     "复盘任务",
					TaskType:  model.TaskTypeReview,
					Phase:     model.LearningPhaseReview,
					PhaseGoal: "复盘纠偏",
					DayNumber: 1,
					Priority:  "high",
				},
				{
					Title:     "后续 drill 任务",
					TaskType:  model.TaskTypePractice,
					Phase:     model.LearningPhaseDrill,
					PhaseGoal: "继续训练",
					DayNumber: 2,
					Priority:  "medium",
				},
			},
		},
	}
	// 旧蓝图是 drill -> review -> mock 的静态模板。
	storedPayload, _ := buildPlanStoredPayload(ai.LearningPlan{
		Title:    "蓝图重建验证计划",
		Duration: 14,
	}, planStoredContext{
		IndustryCode:   "go",
		DurationDays:   14,
		PhaseBlueprint: buildPhaseBlueprint(14, PhaseBlueprintSourceDuration),
	})
	planRepo.plan.PlanJSON = string(storedPayload)

	svc := &planService{
		planRepo:  planRepo,
		taskRepo:  taskRepo,
		planAgent: agent,
		diagnosisRepo: stubPlanTaskDiagnosisRepository{diagnoses: []model.LearningTaskDiagnosis{
			{
				FeedbackID:     911,
				TaskID:         911,
				WeaknessStatus: model.LearningTaskDiagnosisWeaknessUnresolved,
				ActionJSON:     `[{"action":"add_review_task","target_focus_tag":"薄弱点","reason":"需要复盘"}]`,
				Summary:        "薄弱点未解决，先进入复盘。",
			},
		}},
	}

	_, err := svc.AdjustPlan(context.Background(), 49, 41)
	if err != nil {
		t.Fatalf("AdjustPlan returned error: %v", err)
	}

	stored := readPlanStoredPayload(planRepo.saved.PlanJSON)
	if len(stored.Context.PhaseBlueprint) == 0 {
		t.Fatal("expected rebuilt blueprint to be non-empty")
	}
	// 调整后蓝图首桶应为 review（新入口阶段），而非旧的 foundation。
	if stored.Context.PhaseBlueprint[0].Phase != model.LearningPhaseReview {
		t.Fatalf("expected rebuilt blueprint first phase to be review, got %s", stored.Context.PhaseBlueprint[0].Phase)
	}
	if stored.Context.PhaseBlueprint[0].Source != PhaseBlueprintSourceDiagnosisAdjustment {
		t.Fatalf("expected rebuilt blueprint source to be diagnosis_adjustment, got %s", stored.Context.PhaseBlueprint[0].Source)
	}
}

// TestPlanServiceGetPlanFallbackBlueprint 验证旧计划没有蓝图时，GetPlan 能从存储任务推导出非空 phase_blueprint_summary。
func TestPlanServiceGetPlanFallbackBlueprint(t *testing.T) {
	t.Parallel()

	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 42},
			UserID:         50,
			IndustryID:     3,
			Title:          "蓝图兜底验证计划",
			Description:    "验证 fallback blueprint",
			Status:         model.PlanStatusActive,
			TotalTasks:     2,
			CompletedTasks: 0,
			StartDate:      &time.Time{},
			EndDate:        &time.Time{},
			PlanJSON:       `{"plan":{"title":"蓝图兜底验证计划","description":"验证 fallback blueprint","duration_days":14,"tasks":[{"title":"foundation 任务","description":"基础","task_type":"study","day_number":1,"phase":"foundation","phase_goal":"补基础"},{"title":"drill 任务","description":"训练","task_type":"practice","day_number":2,"phase":"drill","phase_goal":"专项训练"}]},"context":{"industry_code":"go","duration_days":14}}`,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel: model.BaseModel{ID: 921},
				PlanID:    42,
				Title:     "foundation 任务",
				TaskType:  model.TaskTypeStudy,
				Phase:     model.LearningPhaseFoundation,
				PhaseGoal: "补基础",
				Status:    model.TaskStatusPending,
				SortOrder: 0,
			},
			{
				BaseModel: model.BaseModel{ID: 922},
				PlanID:    42,
				Title:     "drill 任务",
				TaskType:  model.TaskTypePractice,
				Phase:     model.LearningPhaseDrill,
				PhaseGoal: "专项训练",
				Status:    model.TaskStatusPending,
				SortOrder: 1,
			},
		},
	}
	industryRepo := &stubPlanIndustryRepository{
		byID: map[uint]*model.Industry{
			3: {BaseModel: model.BaseModel{ID: 3}, Code: "go", Name: "Go"},
		},
	}

	svc := NewPlanService(planRepo, taskRepo, &stubPlanAgent{}, nil, nil, nil, nil, nil, industryRepo)
	resp, err := svc.GetPlan(context.Background(), 50, 42)
	if err != nil {
		t.Fatalf("GetPlan returned error: %v", err)
	}
	if len(resp.PhaseBlueprintSummary) == 0 {
		t.Fatal("expected fallback blueprint summary from stored tasks")
	}
	// 从存储任务推导，首桶应为 foundation。
	if resp.PhaseBlueprintSummary[0].Phase != model.LearningPhaseFoundation {
		t.Fatalf("expected fallback blueprint first phase to be foundation, got %s", resp.PhaseBlueprintSummary[0].Phase)
	}
}

// TestPlanServiceGetPlanFallbackBlueprintDerivesLegacyTaskPhase 验证旧计划任务缺少 phase 时，蓝图兜底仍会按 task_type 推导真实阶段。
func TestPlanServiceGetPlanFallbackBlueprintDerivesLegacyTaskPhase(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 14)
	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 43},
			UserID:         51,
			IndustryID:     3,
			Title:          "旧任务阶段推导验证计划",
			Description:    "验证 fallback blueprint phase derive",
			Status:         model.PlanStatusActive,
			TotalTasks:     2,
			CompletedTasks: 0,
			StartDate:      &startDate,
			EndDate:        &endDate,
			PlanJSON:       `{"plan":{"title":"旧任务阶段推导验证计划","description":"验证 fallback blueprint phase derive","duration_days":14,"tasks":[{"title":"复盘任务","description":"旧任务未带 phase","task_type":"review","day_number":1,"duration_minutes":30,"priority":"high"},{"title":"练习任务","description":"继续补练","task_type":"practice","day_number":7,"duration_minutes":45,"priority":"medium"}]},"context":{"industry_code":"go","duration_days":14}}`,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel:   model.BaseModel{ID: 931},
				PlanID:      43,
				Title:       "复盘任务",
				Description: "旧任务未带 phase",
				TaskType:    model.TaskTypeReview,
				Status:      model.TaskStatusPending,
				DueDate:     timePointer(startDate),
				SortOrder:   0,
			},
			{
				BaseModel:   model.BaseModel{ID: 932},
				PlanID:      43,
				Title:       "练习任务",
				Description: "继续补练",
				TaskType:    model.TaskTypePractice,
				Status:      model.TaskStatusPending,
				DueDate:     timePointer(startDate.AddDate(0, 0, 6)),
				SortOrder:   1,
			},
		},
	}
	industryRepo := &stubPlanIndustryRepository{
		byID: map[uint]*model.Industry{
			3: {BaseModel: model.BaseModel{ID: 3}, Code: "go", Name: "Go"},
		},
	}

	svc := NewPlanService(planRepo, taskRepo, &stubPlanAgent{}, nil, nil, nil, nil, nil, industryRepo)
	resp, err := svc.GetPlan(context.Background(), 51, 43)
	if err != nil {
		t.Fatalf("GetPlan returned error: %v", err)
	}
	if len(resp.PhaseBlueprintSummary) != 2 {
		t.Fatalf("expected 2 blueprint summary entries, got %d", len(resp.PhaseBlueprintSummary))
	}
	if resp.PhaseBlueprintSummary[0].Phase != model.LearningPhaseReview {
		t.Fatalf("expected fallback blueprint first phase to derive review, got %s", resp.PhaseBlueprintSummary[0].Phase)
	}
	if resp.PhaseBlueprintSummary[1].Phase != model.LearningPhaseDrill {
		t.Fatalf("expected fallback blueprint second phase to derive drill, got %s", resp.PhaseBlueprintSummary[1].Phase)
	}
}

// TestPlanServiceGetPlanFallbackBlueprintUsesRealTaskDays 验证数据库任务兜底重建蓝图时会保留真实计划日序，而不是按数组序号压缩。
func TestPlanServiceGetPlanFallbackBlueprintUsesRealTaskDays(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 0, 14)
	planRepo := &stubPlanRepository{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 44},
			UserID:         52,
			IndustryID:     3,
			Title:          "蓝图真实日序验证计划",
			Description:    "验证 fallback blueprint day window",
			Status:         model.PlanStatusActive,
			TotalTasks:     2,
			CompletedTasks: 0,
			StartDate:      &startDate,
			EndDate:        &endDate,
			PlanJSON:       `{"plan":{"title":"蓝图真实日序验证计划","description":"验证 fallback blueprint day window","duration_days":14,"tasks":[]},"context":{"industry_code":"go","duration_days":14}}`,
		},
	}
	taskRepo := &stubPlanTaskRepository{
		tasks: []model.LearningTask{
			{
				BaseModel:   model.BaseModel{ID: 941},
				PlanID:      44,
				Title:       "基础任务",
				Description: "第一天补基础",
				TaskType:    model.TaskTypeStudy,
				Phase:       model.LearningPhaseFoundation,
				PhaseGoal:   "补基础",
				Status:      model.TaskStatusPending,
				DueDate:     timePointer(startDate),
				SortOrder:   0,
			},
			{
				BaseModel:   model.BaseModel{ID: 942},
				PlanID:      44,
				Title:       "模拟任务",
				Description: "第十四天做验证",
				TaskType:    model.TaskTypeInterview,
				Phase:       model.LearningPhaseMock,
				PhaseGoal:   "做验证",
				Status:      model.TaskStatusPending,
				DueDate:     timePointer(startDate.AddDate(0, 0, 13)),
				SortOrder:   1,
			},
		},
	}
	industryRepo := &stubPlanIndustryRepository{
		byID: map[uint]*model.Industry{
			3: {BaseModel: model.BaseModel{ID: 3}, Code: "go", Name: "Go"},
		},
	}

	svc := NewPlanService(planRepo, taskRepo, &stubPlanAgent{}, nil, nil, nil, nil, nil, industryRepo)
	resp, err := svc.GetPlan(context.Background(), 52, 44)
	if err != nil {
		t.Fatalf("GetPlan returned error: %v", err)
	}
	if len(resp.PhaseBlueprintSummary) != 2 {
		t.Fatalf("expected 2 blueprint summary entries, got %d", len(resp.PhaseBlueprintSummary))
	}
	if resp.PhaseBlueprintSummary[0].StartDay != 1 || resp.PhaseBlueprintSummary[0].EndDay != 1 {
		t.Fatalf("expected first fallback blueprint window to stay at day 1, got %#v", resp.PhaseBlueprintSummary[0])
	}
	if resp.PhaseBlueprintSummary[1].StartDay != 14 || resp.PhaseBlueprintSummary[1].EndDay != 14 {
		t.Fatalf("expected second fallback blueprint window to stay at day 14, got %#v", resp.PhaseBlueprintSummary[1])
	}
}

// timePointer 返回给定时间的指针，便于测试构造稳定的日期字段。
func timePointer(value time.Time) *time.Time {
	return &value
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

// stubPlanTaskDiagnosisRepository 模拟任务诊断仓库，供 AdjustPlan 测试读取结构化建议。
type stubPlanTaskDiagnosisRepository struct {
	diagnoses []model.LearningTaskDiagnosis
}

// Upsert 满足接口要求，当前测试不依赖该行为。
func (s stubPlanTaskDiagnosisRepository) Upsert(context.Context, *model.LearningTaskDiagnosis) error {
	return nil
}

// ListRecentByPlan 返回预置诊断结果。
func (s stubPlanTaskDiagnosisRepository) ListRecentByPlan(context.Context, uint, int) ([]model.LearningTaskDiagnosis, error) {
	return append([]model.LearningTaskDiagnosis(nil), s.diagnoses...), nil
}

// stubPlanAgent 模拟计划调整 Agent，返回预置调整结果。
type stubPlanAgent struct {
	generatedPlan        ai.LearningPlan
	adjustedPlan         ai.LearningPlan
	lastGenerateProfile  ai.UserProfile
	lastGenerateIndustry string
	lastAdjustInput      ai.PlanAdjustmentInput
}

// GeneratePlan 返回测试预置的学习计划结果。
func (s *stubPlanAgent) GeneratePlan(_ context.Context, profile ai.UserProfile, industryCode string) (ai.LearningPlan, error) {
	s.lastGenerateProfile = profile
	s.lastGenerateIndustry = industryCode
	return s.generatedPlan, nil
}

// AdjustPlan 返回测试预置的调整后计划。
func (s *stubPlanAgent) AdjustPlan(_ context.Context, input ai.PlanAdjustmentInput) (ai.LearningPlan, error) {
	s.lastAdjustInput = ai.PlanAdjustmentInput{
		PlanID:          input.PlanID,
		CompletedTasks:  append([]string(nil), input.CompletedTasks...),
		Performance:     make(map[string]float64, len(input.Performance)),
		CurrentPhase:    input.CurrentPhase,
		EntryPhase:      input.EntryPhase,
		ActionSummaries: append([]string(nil), input.ActionSummaries...),
	}
	for key, value := range input.Performance {
		s.lastAdjustInput.Performance[key] = value
	}
	return s.adjustedPlan, nil
}

// GetStudySuggestion 满足接口，当前测试不依赖该行为。
func (s *stubPlanAgent) GetStudySuggestion(context.Context, ai.UserProfile) (string, error) {
	return "", nil
}
