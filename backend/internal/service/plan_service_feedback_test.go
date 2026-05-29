package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
)

// TestPlanServiceSubmitTaskFeedbackSuccess 验证服务会在计划归属正确时写入结构化训练反馈。
func TestPlanServiceSubmitTaskFeedbackSuccess(t *testing.T) {
	t.Parallel()

	planRepo := &updateTaskPlanRepositoryStub{
		plan: &model.LearningPlan{
			BaseModel: model.BaseModel{ID: 8},
			UserID:    12,
		},
	}
	taskRepo := &updateTaskPlanTaskRepositoryStub{
		tasks: []model.LearningTask{
			{
				BaseModel: model.BaseModel{ID: 101},
				PlanID:    8,
				Title:     "动态规划入门 - 打家劫舍",
			},
		},
	}
	feedbackRepo := &planTaskFeedbackRepositoryStub{}
	svc := &planService{
		planRepo:     planRepo,
		taskRepo:     taskRepo,
		feedbackRepo: feedbackRepo,
	}

	req := &SubmitTaskFeedbackRequest{
		TrainingType:             model.TrainingTypeCoding,
		MistakeTags:              []string{"状态定义错误", "边界处理遗漏"},
		AttemptCount:             3,
		TimeSpentSeconds:         900,
		DifficultySelfAssessment: model.DifficultyTooHard,
		WrongAnswer:              json.RawMessage(`{"code":"dp[i]=nums[i]","note":"漏掉转移"}`),
		Summary:                  "第一次状态定义偏了，第三次才通过。",
	}

	err := svc.SubmitTaskFeedback(context.Background(), 12, 8, 101, req)
	if err != nil {
		t.Fatalf("SubmitTaskFeedback returned error: %v", err)
	}

	if feedbackRepo.created == nil {
		t.Fatal("expected feedback to be persisted")
	}
	if feedbackRepo.created.PlanID != 8 || feedbackRepo.created.TaskID != 101 || feedbackRepo.created.UserID != 12 {
		t.Fatalf("unexpected feedback owner info: %#v", feedbackRepo.created)
	}
	if feedbackRepo.created.TrainingType != model.TrainingTypeCoding {
		t.Fatalf("expected training type coding, got %s", feedbackRepo.created.TrainingType)
	}
	if feedbackRepo.created.MistakeTagsJSON != `["状态定义错误","边界处理遗漏"]` {
		t.Fatalf("unexpected mistake tags json: %s", feedbackRepo.created.MistakeTagsJSON)
	}
	if feedbackRepo.created.WrongAnswerJSON != `{"code":"dp[i]=nums[i]","note":"漏掉转移"}` {
		t.Fatalf("unexpected wrong answer json: %s", feedbackRepo.created.WrongAnswerJSON)
	}
}

// TestPlanServiceSubmitTaskFeedbackRejectsForeignPlan 验证服务会拒绝写入他人学习计划的反馈。
func TestPlanServiceSubmitTaskFeedbackRejectsForeignPlan(t *testing.T) {
	t.Parallel()

	planRepo := &updateTaskPlanRepositoryStub{
		plan: &model.LearningPlan{
			BaseModel: model.BaseModel{ID: 8},
			UserID:    99,
		},
	}
	taskRepo := &updateTaskPlanTaskRepositoryStub{
		tasks: []model.LearningTask{
			{
				BaseModel: model.BaseModel{ID: 101},
				PlanID:    8,
			},
		},
	}
	feedbackRepo := &planTaskFeedbackRepositoryStub{}
	svc := &planService{
		planRepo:     planRepo,
		taskRepo:     taskRepo,
		feedbackRepo: feedbackRepo,
	}

	err := svc.SubmitTaskFeedback(context.Background(), 12, 8, 101, &SubmitTaskFeedbackRequest{
		TrainingType: model.TrainingTypeGeneric,
	})
	if err == nil {
		t.Fatal("expected forbidden error")
	}

	businessErr, ok := err.(*common.BusinessError)
	if !ok || businessErr.Code != common.CodeForbidden {
		t.Fatalf("expected forbidden business error, got %v", err)
	}
	if feedbackRepo.created != nil {
		t.Fatal("did not expect feedback to be persisted")
	}
}

// TestPlanServiceSubmitTaskFeedbackRejectsTaskOutsidePlan 验证服务会拒绝写入不属于当前计划的任务反馈。
func TestPlanServiceSubmitTaskFeedbackRejectsTaskOutsidePlan(t *testing.T) {
	t.Parallel()

	planRepo := &updateTaskPlanRepositoryStub{
		plan: &model.LearningPlan{
			BaseModel: model.BaseModel{ID: 8},
			UserID:    12,
		},
	}
	taskRepo := &updateTaskPlanTaskRepositoryStub{
		tasks: []model.LearningTask{
			{
				BaseModel: model.BaseModel{ID: 101},
				PlanID:    9,
			},
		},
	}
	feedbackRepo := &planTaskFeedbackRepositoryStub{}
	svc := &planService{
		planRepo:     planRepo,
		taskRepo:     taskRepo,
		feedbackRepo: feedbackRepo,
	}

	err := svc.SubmitTaskFeedback(context.Background(), 12, 8, 101, &SubmitTaskFeedbackRequest{
		TrainingType: model.TrainingTypeChoice,
	})
	if err == nil {
		t.Fatal("expected bad request error")
	}

	businessErr, ok := err.(*common.BusinessError)
	if !ok || businessErr.Code != common.CodeBadRequest {
		t.Fatalf("expected bad request business error, got %v", err)
	}
	if feedbackRepo.created != nil {
		t.Fatal("did not expect feedback to be persisted")
	}
}

// TestPlanServiceSubmitTaskFeedbackRejectsInvalidType 验证服务会拦截非法训练类型，避免脏数据入库。
func TestPlanServiceSubmitTaskFeedbackRejectsInvalidType(t *testing.T) {
	t.Parallel()

	feedbackRepo := &planTaskFeedbackRepositoryStub{}
	svc := &planService{
		planRepo:     &updateTaskPlanRepositoryStub{},
		taskRepo:     &updateTaskPlanTaskRepositoryStub{},
		feedbackRepo: feedbackRepo,
	}

	err := svc.SubmitTaskFeedback(context.Background(), 12, 8, 101, &SubmitTaskFeedbackRequest{
		TrainingType: "essay",
	})
	if err == nil {
		t.Fatal("expected bad request error")
	}

	businessErr, ok := err.(*common.BusinessError)
	if !ok || businessErr.Code != common.CodeBadRequest {
		t.Fatalf("expected bad request business error, got %v", err)
	}
	if feedbackRepo.created != nil {
		t.Fatal("did not expect feedback to be persisted")
	}
}

// TestPlanServiceSubmitTaskFeedbackGeneratesAsyncDiagnosis 验证提交反馈后会异步生成诊断结果并同步写入学习档案。
func TestPlanServiceSubmitTaskFeedbackGeneratesAsyncDiagnosis(t *testing.T) {
	t.Parallel()

	storedPayload, err := buildPlanStoredPayload(ai.LearningPlan{
		Title:       "Go 训练计划",
		Description: "验证异步诊断落库",
	}, planStoredContext{
		IndustryCode: "go",
		EntryPhase:   model.LearningPhaseReview,
	})
	if err != nil {
		t.Fatalf("buildPlanStoredPayload returned error: %v", err)
	}

	planRepo := &updateTaskPlanRepositoryStub{
		plan: &model.LearningPlan{
			BaseModel: model.BaseModel{ID: 8},
			UserID:    12,
			Phase:     model.LearningPhaseReview,
			PhaseGoal: model.BuildLearningPhaseGoal(model.LearningPhaseReview),
			PlanJSON:  string(storedPayload),
		},
	}
	taskRepo := &updateTaskPlanTaskRepositoryStub{
		tasks: []model.LearningTask{
			{
				BaseModel:   model.BaseModel{ID: 101},
				PlanID:      8,
				Title:       "动态规划入门 - 打家劫舍",
				Description: "围绕状态定义和转移方程完成一轮基础训练",
				TaskType:    model.TaskTypePractice,
				Phase:       model.LearningPhaseReview,
				PhaseGoal:   model.BuildLearningPhaseGoal(model.LearningPhaseReview),
			},
		},
	}
	feedbackRepo := &planTaskFeedbackRepositoryStub{}
	diagnosisRepo := &planTaskDiagnosisRepositoryStub{createdCh: make(chan *model.LearningTaskDiagnosis, 1)}
	archiveRepo := &taskFeedbackLearningArchiveRepositoryStub{upsertedCh: make(chan *model.LearningArchiveEntry, 1)}
	svc := &planService{
		planRepo:            planRepo,
		taskRepo:            taskRepo,
		feedbackRepo:        feedbackRepo,
		diagnosisRepo:       diagnosisRepo,
		learningArchiveRepo: archiveRepo,
		quizAnalyzer: stubTaskFeedbackQuizAnalyzer{
			analysis: ai.CodeAnalysis{
				IsCorrect:    true,
				MistakeTags:  []string{"状态定义不清"},
				StrengthTags: []string{"能够写出基本转移"},
				Issues:       []string{"第一次提交把状态定义成了当前位置收益。"},
				Improvements: []string{"先写状态定义，再写转移方程。"},
				Feedback:     "当前实现已经收敛，但状态定义仍需要固定模板。",
			},
		},
	}

	err = svc.SubmitTaskFeedback(context.Background(), 12, 8, 101, &SubmitTaskFeedbackRequest{
		TrainingType:             model.TrainingTypeCoding,
		MistakeTags:              []string{"状态定义不清"},
		AttemptCount:             3,
		TimeSpentSeconds:         2100,
		DifficultySelfAssessment: model.DifficultyTooHard,
		WrongAnswer:              json.RawMessage(`{"code":"dp[i]=nums[i]","language":"go","note":"状态定义写偏了"}`),
		Summary:                  "第三次才把状态转移写对。",
	})
	if err != nil {
		t.Fatalf("SubmitTaskFeedback returned error: %v", err)
	}

	select {
	case diagnosis := <-diagnosisRepo.createdCh:
		if diagnosis.FeedbackID == 0 {
			t.Fatalf("expected diagnosis feedback id to be assigned, got %#v", diagnosis)
		}
		if diagnosis.WeaknessStatus != model.LearningTaskDiagnosisWeaknessUnresolved {
			t.Fatalf("expected unresolved weakness status, got %s", diagnosis.WeaknessStatus)
		}
		if !strings.Contains(diagnosis.ActionJSON, model.LearningTaskDiagnosisActionRepeatSamePattern) {
			t.Fatalf("expected repeat_same_pattern action, got %s", diagnosis.ActionJSON)
		}
		if diagnosis.PlanPhase != model.LearningPhaseReview || diagnosis.EntryPhase != model.LearningPhaseReview {
			t.Fatalf("expected diagnosis to persist review plan phase context, got %#v", diagnosis)
		}
		if diagnosis.TaskPhase != model.LearningPhaseReview || diagnosis.TaskPhaseGoal == "" {
			t.Fatalf("expected diagnosis to persist review task phase context, got %#v", diagnosis)
		}
		if !strings.Contains(diagnosis.Summary, "复盘纠偏阶段") {
			t.Fatalf("expected diagnosis summary to include phase context, got %s", diagnosis.Summary)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for async diagnosis")
	}

	select {
	case entry := <-archiveRepo.upsertedCh:
		if entry.SourceType != model.LearningArchiveSourcePlanTaskFeedback {
			t.Fatalf("expected archive source type plan_task_feedback, got %s", entry.SourceType)
		}
		if entry.IndustryCode != "go" {
			t.Fatalf("expected archive industry code go, got %s", entry.IndustryCode)
		}
		if entry.EntryPhase != model.LearningPhaseReview || entry.TaskPhase != model.LearningPhaseReview {
			t.Fatalf("expected archive to persist review phase context, got %#v", entry)
		}
		if entry.TaskPhaseGoal == "" {
			t.Fatalf("expected archive to persist task phase goal, got %#v", entry)
		}
		if !strings.Contains(entry.MistakeTagsJSON, "状态定义不清") {
			t.Fatalf("expected archive mistake tags to contain 状态定义不清, got %s", entry.MistakeTagsJSON)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for archive upsert")
	}
}

// TestPlanServiceFeedbackToAdjustPlanChain 验证提交训练反馈后，诊断结果能被后续 AdjustPlan 直接消费并回填解释字段。
func TestPlanServiceFeedbackToAdjustPlanChain(t *testing.T) {
	t.Parallel()

	storedPayload, err := buildPlanStoredPayload(ai.LearningPlan{
		Title:       "Go 训练计划",
		Description: "验证反馈到调计划的完整链路",
		Tasks: []ai.PlanTask{
			{
				Title:       "状态定义不清专项练习",
				Description: "围绕状态定义和转移方程完成一轮基础训练",
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
	}, planStoredContext{
		IndustryCode: "go",
		Level:        "intermediate",
		WeakTopics:   []string{"状态定义不清"},
	})
	if err != nil {
		t.Fatalf("buildPlanStoredPayload returned error: %v", err)
	}

	planRepo := &updateTaskPlanRepositoryStub{
		plan: &model.LearningPlan{
			BaseModel:      model.BaseModel{ID: 8},
			UserID:         12,
			IndustryID:     7,
			Title:          "Go 训练计划",
			Description:    "验证反馈到调计划的完整链路",
			Status:         model.PlanStatusActive,
			TotalTasks:     2,
			CompletedTasks: 1,
			PlanJSON:       string(storedPayload),
			StartDate:      func() *time.Time { v := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC); return &v }(),
			EndDate:        func() *time.Time { v := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC); return &v }(),
		},
	}
	taskRepo := &updateTaskPlanTaskRepositoryStub{
		tasks: []model.LearningTask{
			{
				BaseModel:   model.BaseModel{ID: 101},
				PlanID:      8,
				Title:       "状态定义不清专项练习",
				Description: "围绕状态定义和转移方程完成一轮基础训练",
				TaskType:    model.TaskTypePractice,
				Status:      model.TaskStatusCompleted,
				CompletedAt: func() *time.Time { v := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC); return &v }(),
				SortOrder:   0,
			},
			{
				BaseModel:   model.BaseModel{ID: 102},
				PlanID:      8,
				Title:       "状态定义不清复盘",
				Description: "先整理状态定义模板，再进入下一轮训练",
				TaskType:    model.TaskTypeStudy,
				Status:      model.TaskStatusPending,
				SortOrder:   1,
			},
		},
	}
	feedbackRepo := &planTaskFeedbackRepositoryStub{}
	diagnosisRepo := &planTaskDiagnosisRepositoryStub{createdCh: make(chan *model.LearningTaskDiagnosis, 1)}
	svc := &planService{
		planRepo:            planRepo,
		taskRepo:            taskRepo,
		feedbackRepo:        feedbackRepo,
		diagnosisRepo:       diagnosisRepo,
		learningArchiveRepo: &taskFeedbackLearningArchiveRepositoryStub{upsertedCh: make(chan *model.LearningArchiveEntry, 1)},
	}

	err = svc.SubmitTaskFeedback(context.Background(), 12, 8, 101, &SubmitTaskFeedbackRequest{
		TrainingType:             model.TrainingTypeGeneric,
		MistakeTags:              []string{"状态定义不清"},
		AttemptCount:             3,
		TimeSpentSeconds:         1800,
		DifficultySelfAssessment: model.DifficultyTooHard,
		Summary:                  "这一轮状态定义仍不稳定。",
	})
	if err != nil {
		t.Fatalf("SubmitTaskFeedback returned error: %v", err)
	}

	select {
	case <-diagnosisRepo.createdCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for async diagnosis")
	}

	adjustedPlanAgent := &stubPlanAgent{
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
	svc.planAgent = adjustedPlanAgent

	resp, err := svc.AdjustPlan(context.Background(), 12, 8)
	if err != nil {
		t.Fatalf("AdjustPlan returned error: %v", err)
	}
	expectedCollectionHint := resolveMistakeTopicCodeByTag("状态定义不清")
	if len(resp.Tasks) != 3 {
		t.Fatalf("expected completed history plus two diagnosis-driven tasks, got %d", len(resp.Tasks))
	}
	if resp.Tasks[1].Source != "plan_feedback_diagnosis" || resp.Tasks[1].TaskType != model.TaskTypeReview {
		t.Fatalf("expected first adjusted task to be diagnosis review task, got %#v", resp.Tasks[1])
	}
	if resp.Tasks[1].SourceRef != "plan-feedback:501" {
		t.Fatalf("expected first adjusted task to expose feedback source ref, got %#v", resp.Tasks[1])
	}
	if resp.Tasks[1].CollectionHint != expectedCollectionHint {
		t.Fatalf("expected first adjusted task to expose collection hint, got %#v", resp.Tasks[1])
	}
	if resp.Tasks[2].Source != "plan_feedback_diagnosis" || resp.Tasks[2].TaskType != model.TaskTypePractice {
		t.Fatalf("expected second adjusted task to keep diagnosis source, got %#v", resp.Tasks[2])
	}
	if resp.Tasks[2].CollectionHint != expectedCollectionHint {
		t.Fatalf("expected second adjusted task to expose collection hint, got %#v", resp.Tasks[2])
	}
	if adjustedPlanAgent.lastAdjustInput.Performance[model.TaskTypePractice] < 50 {
		t.Fatalf("expected practice performance to reflect unresolved diagnosis, got %#v", adjustedPlanAgent.lastAdjustInput.Performance)
	}
	if adjustedPlanAgent.lastAdjustInput.CurrentPhase != model.LearningPhaseFoundation || adjustedPlanAgent.lastAdjustInput.EntryPhase != model.LearningPhaseReview {
		t.Fatalf("expected feedback-chain path to send foundation->review phase input, got %#v", adjustedPlanAgent.lastAdjustInput)
	}
}

// planTaskFeedbackRepositoryStub 模拟学习任务反馈仓库，用于观察服务层写入结果。
type planTaskFeedbackRepositoryStub struct {
	created *model.LearningTaskFeedback
}

// Create 记录服务层提交的反馈内容。
func (s *planTaskFeedbackRepositoryStub) Create(_ context.Context, feedback *model.LearningTaskFeedback) error {
	if feedback.ID == 0 {
		feedback.ID = 501
	}
	if feedback.CreatedAt.IsZero() {
		feedback.CreatedAt = time.Now()
	}
	clone := *feedback
	s.created = &clone
	return nil
}

// GetByID 按反馈 ID 返回最近一次写入的反馈记录，供异步诊断消费测试复用。
func (s *planTaskFeedbackRepositoryStub) GetByID(_ context.Context, id uint) (*model.LearningTaskFeedback, error) {
	if s.created == nil {
		return nil, nil
	}
	if id != 0 && s.created.ID != id {
		return nil, nil
	}
	clone := *s.created
	return &clone, nil
}

// planTaskDiagnosisRepositoryStub 模拟学习任务诊断仓库，用于接收异步诊断结果。
type planTaskDiagnosisRepositoryStub struct {
	created   *model.LearningTaskDiagnosis
	records   []model.LearningTaskDiagnosis
	createdCh chan *model.LearningTaskDiagnosis
}

// Upsert 记录异步写入的学习任务诊断。
func (s *planTaskDiagnosisRepositoryStub) Upsert(_ context.Context, diagnosis *model.LearningTaskDiagnosis) error {
	clone := *diagnosis
	s.created = &clone
	s.records = append(s.records, clone)
	if s.createdCh != nil {
		s.createdCh <- &clone
	}
	return nil
}

// ListRecentByPlan 返回已记录的诊断结果，便于串联测试继续验证后续计划调整。
func (s *planTaskDiagnosisRepositoryStub) ListRecentByPlan(context.Context, uint, int) ([]model.LearningTaskDiagnosis, error) {
	return append([]model.LearningTaskDiagnosis(nil), s.records...), nil
}

// taskFeedbackLearningArchiveRepositoryStub 模拟学习档案仓库，用于观察异步归档结果。
type taskFeedbackLearningArchiveRepositoryStub struct {
	upserted   *model.LearningArchiveEntry
	upsertedCh chan *model.LearningArchiveEntry
}

// Upsert 记录异步写入的学习档案条目。
func (s *taskFeedbackLearningArchiveRepositoryStub) Upsert(_ context.Context, entry *model.LearningArchiveEntry) error {
	clone := *entry
	s.upserted = &clone
	if s.upsertedCh != nil {
		s.upsertedCh <- &clone
	}
	return nil
}

// ListRecentByUser 满足接口要求，当前测试不依赖该行为。
func (s *taskFeedbackLearningArchiveRepositoryStub) ListRecentByUser(context.Context, uint, int, *uint) ([]model.LearningArchiveEntry, error) {
	return nil, nil
}

// stubTaskFeedbackQuizAnalyzer 为任务反馈诊断测试提供最小题目分析器实现。
type stubTaskFeedbackQuizAnalyzer struct {
	analysis ai.CodeAnalysis
}

// AnalyzeCode 返回预置代码分析结果。
func (s stubTaskFeedbackQuizAnalyzer) AnalyzeCode(context.Context, string, string, string) (ai.CodeAnalysis, error) {
	return s.analysis, nil
}

// DiagnoseInterviewCoding 满足接口要求，当前测试不依赖该行为。
func (s stubTaskFeedbackQuizAnalyzer) DiagnoseInterviewCoding(context.Context, ai.InterviewCodingDiagnosisInput) (ai.CodingQuestionDiagnosis, error) {
	return ai.CodingQuestionDiagnosis{}, nil
}

// ExplainAnswer 满足接口要求，当前测试不依赖该行为。
func (s stubTaskFeedbackQuizAnalyzer) ExplainAnswer(context.Context, string, string, string) (string, error) {
	return "", nil
}

// GenerateHint 满足接口要求，当前测试不依赖该行为。
func (s stubTaskFeedbackQuizAnalyzer) GenerateHint(context.Context, string, string) (string, error) {
	return "", nil
}
