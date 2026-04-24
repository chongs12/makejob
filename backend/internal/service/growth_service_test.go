package service

import (
	"context"
	"testing"
	"time"

	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

// TestGrowthServiceSyncStudyLogSanitizesPayload 验证学习日志同步会清洗标题列表并按日期写入服务端。
func TestGrowthServiceSyncStudyLogSanitizesPayload(t *testing.T) {
	t.Parallel()

	studyLogRepo := &growthStudyLogRepositoryStub{}
	svc := &growthService{
		studyLogRepo: studyLogRepo,
	}

	planID := uint(18)
	resp, err := svc.SyncStudyLog(context.Background(), 7, &SyncStudyLogRequest{
		DateKey:          "2026-04-24",
		PlanID:           &planID,
		Summary:          "  今天完成了并发复习  ",
		FocusTaskTitle:   "  并发复习  ",
		CompletedCount:   0,
		SkippedCount:     1,
		CompletedTitles:  []string{" 并发复习 ", "", "并发复习"},
		SkippedTitles:    []string{" 旧任务 ", "旧任务"},
		LatestActionText: "  已将「并发复习」更新为已完成。  ",
	})
	if err != nil {
		t.Fatalf("SyncStudyLog returned error: %v", err)
	}

	if studyLogRepo.upsertedLog == nil {
		t.Fatal("expected study log to be persisted")
	}
	if studyLogRepo.upsertedLog.UserID != 7 {
		t.Fatalf("expected user id 7, got %d", studyLogRepo.upsertedLog.UserID)
	}
	if studyLogRepo.upsertedLog.PlanID == nil || *studyLogRepo.upsertedLog.PlanID != planID {
		t.Fatalf("expected plan id %d, got %#v", planID, studyLogRepo.upsertedLog.PlanID)
	}
	if studyLogRepo.upsertedLog.CompletedCount != 1 {
		t.Fatalf("expected completed count 1, got %d", studyLogRepo.upsertedLog.CompletedCount)
	}
	if studyLogRepo.upsertedLog.SkippedCount != 1 {
		t.Fatalf("expected skipped count 1, got %d", studyLogRepo.upsertedLog.SkippedCount)
	}
	if studyLogRepo.upsertedLog.Summary != "今天完成了并发复习" {
		t.Fatalf("expected trimmed summary, got %q", studyLogRepo.upsertedLog.Summary)
	}
	if len(resp.CompletedTitles) != 1 || resp.CompletedTitles[0] != "并发复习" {
		t.Fatalf("unexpected completed titles: %#v", resp.CompletedTitles)
	}
	if len(resp.SkippedTitles) != 1 || resp.SkippedTitles[0] != "旧任务" {
		t.Fatalf("unexpected skipped titles: %#v", resp.SkippedTitles)
	}
	if resp.LatestActionText != "已将「并发复习」更新为已完成。" {
		t.Fatalf("unexpected latest action text: %q", resp.LatestActionText)
	}
}

// TestGrowthServiceGetGrowthSummaryAggregatesData 验证成长档案聚合接口会整合练习、面试、计划与学习日志数据。
func TestGrowthServiceGetGrowthSummaryAggregatesData(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 24, 20, 0, 0, 0, time.UTC)
	startDate := now.AddDate(0, 0, -7)
	endDate := now.AddDate(0, 0, 7)
	logUpdatedAt := now.Add(-2 * time.Hour)
	finishedAt := now.Add(-24 * time.Hour)
	studyLogs := []model.StudyLog{
		{
			BaseModel:           model.BaseModel{ID: 11, UpdatedAt: logUpdatedAt},
			UserID:              7,
			LogDate:             now,
			Summary:             "今天完成了并发复习",
			FocusTaskTitle:      "并发复习",
			CompletedCount:      2,
			SkippedCount:        1,
			CompletedTitlesJSON: `["并发复习","网络补漏"]`,
			SkippedTitlesJSON:   `["旧任务"]`,
			LatestActionText:    "已将「并发复习」更新为已完成。",
		},
	}
	interviews := []model.MockInterview{
		{
			BaseModel:      model.BaseModel{ID: 301, CreatedAt: now.Add(-48 * time.Hour)},
			Status:         model.InterviewStatusCompleted,
			Score:          88,
			TotalQuestions: 6,
			EndedAt:        &finishedAt,
		},
		{
			BaseModel:      model.BaseModel{ID: 302, CreatedAt: now.Add(-72 * time.Hour)},
			Status:         model.InterviewStatusCompleted,
			Score:          92,
			TotalQuestions: 8,
		},
		{
			BaseModel:      model.BaseModel{ID: 303, CreatedAt: now.Add(-96 * time.Hour)},
			Status:         model.InterviewStatusOngoing,
			Score:          0,
			TotalQuestions: 4,
		},
	}
	currentPlan := &model.LearningPlan{
		BaseModel:      model.BaseModel{ID: 401, CreatedAt: now.Add(-72 * time.Hour)},
		UserID:         7,
		Title:          "Go 强化计划",
		Status:         model.PlanStatusActive,
		TotalTasks:     4,
		CompletedTasks: 2,
		StartDate:      &startDate,
		EndDate:        &endDate,
	}
	recentPlans := []model.LearningPlan{
		*currentPlan,
		{
			BaseModel:      model.BaseModel{ID: 402, CreatedAt: now.Add(-10 * 24 * time.Hour)},
			UserID:         7,
			Title:          "Go 基础计划",
			Status:         model.PlanStatusCompleted,
			TotalTasks:     6,
			CompletedTasks: 6,
			StartDate:      &startDate,
			EndDate:        &finishedAt,
		},
	}
	tasks := []model.LearningTask{
		{
			BaseModel: model.BaseModel{ID: 501},
			PlanID:    401,
			Title:     "并发复习",
			Status:    model.TaskStatusCompleted,
		},
		{
			BaseModel: model.BaseModel{ID: 502},
			PlanID:    401,
			Title:     "项目拆解",
			Status:    model.TaskStatusInProgress,
		},
		{
			BaseModel: model.BaseModel{ID: 503},
			PlanID:    401,
			Title:     "面试模拟",
			Status:    model.TaskStatusPending,
		},
	}

	svc := &growthService{
		studyLogRepo: &growthStudyLogRepositoryStub{
			recentLogs: studyLogs,
			studyDays:  9,
		},
		recordRepo: &growthQuestionRecordRepositoryStub{
			stats: &repository.UserPracticeStats{
				TotalAnswered: 42,
				CorrectCount:  30,
				WrongCount:    12,
				AccuracyRate:  71.4,
				TodayCount:    5,
				StreakDays:    6,
				CategoryStats: []repository.CategoryStat{
					{CategoryID: 1, CategoryName: "并发", Total: 10, Correct: 8, AccuracyRate: 80},
				},
			},
		},
		interviewRepo: &growthInterviewRepositoryStub{
			interviews: interviews,
		},
		planRepo: &growthPlanRepositoryStub{
			currentPlan: currentPlan,
			plans:       recentPlans,
		},
		taskRepo: &growthPlanTaskRepositoryStub{
			tasksByPlan: map[uint][]model.LearningTask{
				401: tasks,
			},
		},
	}

	resp, err := svc.GetGrowthSummary(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetGrowthSummary returned error: %v", err)
	}

	if resp.StudyDays != 9 {
		t.Fatalf("expected study days 9, got %d", resp.StudyDays)
	}
	if resp.InterviewCount != 3 {
		t.Fatalf("expected interview count 3, got %d", resp.InterviewCount)
	}
	if resp.CompletedInterviewCount != 2 {
		t.Fatalf("expected completed interview count 2, got %d", resp.CompletedInterviewCount)
	}
	if resp.AverageInterviewScore != 90 {
		t.Fatalf("expected average interview score 90, got %v", resp.AverageInterviewScore)
	}
	if resp.PlanCount != 2 {
		t.Fatalf("expected plan count 2, got %d", resp.PlanCount)
	}
	if resp.CurrentPlan == nil {
		t.Fatal("expected current plan summary")
	}
	if resp.CurrentPlan.NextTaskTitle != "项目拆解" {
		t.Fatalf("expected next task 项目拆解, got %q", resp.CurrentPlan.NextTaskTitle)
	}
	if len(resp.RecentStudyLogs) != 1 || resp.RecentStudyLogs[0].CompletedTitles[0] != "并发复习" {
		t.Fatalf("unexpected study logs: %#v", resp.RecentStudyLogs)
	}
	if len(resp.RecentInterviews) != 3 {
		t.Fatalf("expected 3 recent interviews, got %d", len(resp.RecentInterviews))
	}
	if len(resp.RecentPlans) != 2 {
		t.Fatalf("expected 2 recent plans, got %d", len(resp.RecentPlans))
	}
	if resp.PracticeStats == nil || resp.PracticeStats.StreakDays != 6 {
		t.Fatalf("unexpected practice stats: %#v", resp.PracticeStats)
	}
}

// growthStudyLogRepositoryStub 模拟学习日志仓库，供成长档案服务测试使用。
type growthStudyLogRepositoryStub struct {
	upsertedLog *model.StudyLog
	recentLogs  []model.StudyLog
	studyDays   int64
}

// UpsertDaily 记录最后一次写入的学习日志。
func (s *growthStudyLogRepositoryStub) UpsertDaily(_ context.Context, log *model.StudyLog) error {
	clone := *log
	if clone.ID == 0 {
		clone.ID = 1
	}
	if clone.UpdatedAt.IsZero() {
		clone.UpdatedAt = time.Date(2026, 4, 24, 20, 0, 0, 0, time.UTC)
	}
	s.upsertedLog = &clone
	return nil
}

// ListRecentByUser 返回预置的最近学习日志列表。
func (s *growthStudyLogRepositoryStub) ListRecentByUser(context.Context, uint, int) ([]model.StudyLog, error) {
	return append([]model.StudyLog(nil), s.recentLogs...), nil
}

// CountStudyDays 返回预置的累计学习天数。
func (s *growthStudyLogRepositoryStub) CountStudyDays(context.Context, uint) (int64, error) {
	return s.studyDays, nil
}

// growthQuestionRecordRepositoryStub 模拟答题记录仓库，供成长档案服务测试使用。
type growthQuestionRecordRepositoryStub struct {
	stats *repository.UserPracticeStats
}

// Create 满足接口要求，当前测试不依赖创建行为。
func (s *growthQuestionRecordRepositoryStub) Create(context.Context, *model.UserQuestionRecord) error {
	return nil
}

// GetByUserAndQuestion 满足接口要求，当前测试不依赖该行为。
func (s *growthQuestionRecordRepositoryStub) GetByUserAndQuestion(context.Context, uint, uint) ([]model.UserQuestionRecord, error) {
	return nil, nil
}

// GetWrongQuestions 满足接口要求，当前测试不依赖该行为。
func (s *growthQuestionRecordRepositoryStub) GetWrongQuestions(context.Context, uint, int, int) ([]model.UserQuestionRecord, int64, error) {
	return nil, 0, nil
}

// GetUserStats 返回预置练习统计。
func (s *growthQuestionRecordRepositoryStub) GetUserStats(context.Context, uint) (*repository.UserPracticeStats, error) {
	return s.stats, nil
}

// GetDailyCount 满足接口要求，当前测试不依赖该行为。
func (s *growthQuestionRecordRepositoryStub) GetDailyCount(context.Context, uint, time.Time) (int64, error) {
	return 0, nil
}

// growthInterviewRepositoryStub 模拟面试仓库，供成长档案服务测试使用。
type growthInterviewRepositoryStub struct {
	interviews []model.MockInterview
}

// Create 满足接口要求，当前测试不依赖创建行为。
func (s *growthInterviewRepositoryStub) Create(context.Context, *model.MockInterview) error {
	return nil
}

// GetByID 满足接口要求，当前测试不依赖该行为。
func (s *growthInterviewRepositoryStub) GetByID(context.Context, uint) (*model.MockInterview, error) {
	return nil, nil
}

// Update 满足接口要求，当前测试不依赖该行为。
func (s *growthInterviewRepositoryStub) Update(context.Context, *model.MockInterview) error {
	return nil
}

// ListByUser 返回按时间倒序排列的预置面试列表。
func (s *growthInterviewRepositoryStub) ListByUser(context.Context, uint, int, int) ([]model.MockInterview, int64, error) {
	return append([]model.MockInterview(nil), s.interviews...), int64(len(s.interviews)), nil
}

// GetUserDailyCount 满足接口要求，当前测试不依赖该行为。
func (s *growthInterviewRepositoryStub) GetUserDailyCount(context.Context, uint, time.Time) (int64, error) {
	return 0, nil
}

// growthPlanRepositoryStub 模拟学习计划仓库，供成长档案服务测试使用。
type growthPlanRepositoryStub struct {
	currentPlan *model.LearningPlan
	plans       []model.LearningPlan
}

// Create 满足接口要求，当前测试不依赖创建行为。
func (s *growthPlanRepositoryStub) Create(context.Context, *model.LearningPlan) error {
	return nil
}

// GetByID 满足接口要求，当前测试不依赖该行为。
func (s *growthPlanRepositoryStub) GetByID(context.Context, uint) (*model.LearningPlan, error) {
	return nil, nil
}

// GetCurrentByUser 返回预置的当前计划。
func (s *growthPlanRepositoryStub) GetCurrentByUser(context.Context, uint) (*model.LearningPlan, error) {
	if s.currentPlan == nil {
		return nil, nil
	}
	clone := *s.currentPlan
	return &clone, nil
}

// Update 满足接口要求，当前测试不依赖该行为。
func (s *growthPlanRepositoryStub) Update(context.Context, *model.LearningPlan) error {
	return nil
}

// ListByUser 返回预置的计划列表与总数。
func (s *growthPlanRepositoryStub) ListByUser(context.Context, uint, int, int) ([]model.LearningPlan, int64, error) {
	return append([]model.LearningPlan(nil), s.plans...), int64(len(s.plans)), nil
}

// PauseActivePlans 满足接口要求，当前测试不依赖该行为。
func (s *growthPlanRepositoryStub) PauseActivePlans(context.Context, uint) error {
	return nil
}

// growthPlanTaskRepositoryStub 模拟学习任务仓库，供成长档案服务测试使用。
type growthPlanTaskRepositoryStub struct {
	tasksByPlan map[uint][]model.LearningTask
}

// Create 满足接口要求，当前测试不依赖创建行为。
func (s *growthPlanTaskRepositoryStub) Create(context.Context, *model.LearningTask) error {
	return nil
}

// BatchCreate 满足接口要求，当前测试不依赖该行为。
func (s *growthPlanTaskRepositoryStub) BatchCreate(context.Context, []model.LearningTask) error {
	return nil
}

// GetByID 满足接口要求，当前测试不依赖该行为。
func (s *growthPlanTaskRepositoryStub) GetByID(context.Context, uint) (*model.LearningTask, error) {
	return nil, nil
}

// Update 满足接口要求，当前测试不依赖该行为。
func (s *growthPlanTaskRepositoryStub) Update(context.Context, *model.LearningTask) error {
	return nil
}

// ListByPlan 返回指定计划下的预置任务列表。
func (s *growthPlanTaskRepositoryStub) ListByPlan(_ context.Context, planID uint) ([]model.LearningTask, error) {
	return append([]model.LearningTask(nil), s.tasksByPlan[planID]...), nil
}

// CountByPlanAndStatus 满足接口要求，当前测试不依赖该行为。
func (s *growthPlanTaskRepositoryStub) CountByPlanAndStatus(context.Context, uint, string) (int64, error) {
	return 0, nil
}

// DeleteByPlan 满足接口要求，当前测试不依赖该行为。
func (s *growthPlanTaskRepositoryStub) DeleteByPlan(context.Context, uint) error {
	return nil
}

// DeleteIncompleteByPlan 满足接口要求，当前测试不依赖该行为。
func (s *growthPlanTaskRepositoryStub) DeleteIncompleteByPlan(context.Context, uint) error {
	return nil
}
