package service

import (
	"context"
	"testing"
	"time"

	"makejob-backend/internal/ai"
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
	storedPayload, err := buildPlanStoredPayload(ai.LearningPlan{
		Title:       "Go 强化计划",
		Description: "围绕近期高频问题的补强计划",
		Tasks: []ai.PlanTask{
			{
				Title:       "并发复习",
				Description: "理解并发模型",
				TaskType:    model.TaskTypeStudy,
				DayNumber:   1,
			},
			{
				Title:       "状态定义不清专项训练",
				Description: "围绕状态定义不清做一轮动态规划专项补练",
				TaskType:    model.TaskTypePractice,
				DayNumber:   2,
				Priority:    "high",
			},
		},
	}, planStoredContext{
		IndustryCode: "go",
		FocusSignals: []trainingFocusSignal{
			{
				Tag:                    "状态定义不清",
				TopicCode:              "state-definition",
				TopicTitle:             "状态定义不清",
				PrimaryQuestionSet:     "algorithm-structure",
				Source:                 "learning_archive",
				SourceLabel:            "练习归档",
				Reason:                 "最近练习归档里“状态定义不清”累计出现 2 次，说明这个问题还在持续影响你的输出。",
				SourceRef:              "practice:7:701",
				CollectionHint:         "algorithm-structure",
				OccurrenceCount:        2,
				ArchiveOccurrenceCount: 2,
			},
		},
	})
	if err != nil {
		t.Fatalf("buildPlanStoredPayload returned error: %v", err)
	}
	currentPlan.PlanJSON = string(storedPayload)
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
			BaseModel:   model.BaseModel{ID: 502},
			PlanID:      401,
			Title:       "状态定义不清专项训练",
			Description: "围绕状态定义不清做一轮动态规划专项补练",
			Status:      model.TaskStatusInProgress,
			TaskType:    model.TaskTypePractice,
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
		learningArchiveRepo: growthLearningArchiveRepositoryStub{
			entries: []model.LearningArchiveEntry{
				{
					IndustryCode:    "go",
					SourceRef:       "practice:7:701",
					MistakeTagsJSON: `["状态定义不清"]`,
					SuggestionsJSON: `["先口述状态定义，再开始写代码。"]`,
				},
				{
					IndustryCode:    "go",
					SourceRef:       "practice:7:702",
					MistakeTagsJSON: `["状态定义不清"]`,
					SuggestionsJSON: `["把变量命名改成能反映语义的名称。"]`,
				},
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
	if resp.CurrentPlan.NextTaskTitle != "状态定义不清专项训练" {
		t.Fatalf("expected next task 状态定义不清专项训练, got %q", resp.CurrentPlan.NextTaskTitle)
	}
	if resp.CurrentPlan.NextTaskSource != "practice_recommendation" {
		t.Fatalf("expected next task source practice_recommendation, got %s", resp.CurrentPlan.NextTaskSource)
	}
	if resp.CurrentPlan.NextTaskSourceRef != "practice:7:701" || resp.CurrentPlan.NextTaskCollectionHint != "algorithm-structure" {
		t.Fatalf("unexpected next task source ref or collection hint: %#v", resp.CurrentPlan)
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
	if len(resp.FocusSignals) != 1 || resp.FocusSignals[0].FocusTag != "状态定义不清" {
		t.Fatalf("unexpected focus signals: %#v", resp.FocusSignals)
	}
	if resp.TrendSummary == nil || resp.TrendSummary.TopFocusTag != "状态定义不清" {
		t.Fatalf("unexpected trend summary: %#v", resp.TrendSummary)
	}
}

// TestGrowthServiceGetGrowthSummaryUsesRecentInterviewsForTrend 验证成长页趋势摘要只会消费最近窗口的面试记录。
func TestGrowthServiceGetGrowthSummaryUsesRecentInterviewsForTrend(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 25, 20, 0, 0, 0, time.UTC)
	interviews := make([]model.MockInterview, 0, 11)
	for i := 0; i < 5; i++ {
		interviews = append(interviews, model.MockInterview{
			BaseModel:      model.BaseModel{ID: uint(900 + i), CreatedAt: now.Add(time.Duration(-i) * time.Hour)},
			IndustryID:     7,
			Status:         model.InterviewStatusCompleted,
			Score:          90,
			TotalQuestions: 6,
			ReportJSON:     `{"weaknesses":["状态定义不清"],"suggestions":["先口述状态定义，再开始写代码。"]}`,
		})
	}
	for i := 0; i < 6; i++ {
		interviews = append(interviews, model.MockInterview{
			BaseModel:      model.BaseModel{ID: uint(950 + i), CreatedAt: now.Add(time.Duration(-(i + 5)) * time.Hour)},
			IndustryID:     7,
			Status:         model.InterviewStatusCompleted,
			Score:          60,
			TotalQuestions: 6,
			ReportJSON:     `{"weaknesses":["边界条件生疏"],"suggestions":["写完主流程后单独列一组边界样例再检查。"]}`,
		})
	}

	svc := &growthService{
		studyLogRepo: &growthStudyLogRepositoryStub{},
		recordRepo: &growthQuestionRecordRepositoryStub{
			stats: &repository.UserPracticeStats{},
		},
		interviewRepo: &growthInterviewRepositoryStub{
			interviews: interviews,
		},
		planRepo: &growthPlanRepositoryStub{},
		taskRepo: &growthPlanTaskRepositoryStub{},
		learningArchiveRepo: growthLearningArchiveRepositoryStub{
			entries: []model.LearningArchiveEntry{},
		},
	}

	resp, err := svc.GetGrowthSummary(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetGrowthSummary returned error: %v", err)
	}

	if resp.InterviewCount != int64(len(interviews)) {
		t.Fatalf("expected interview count %d, got %d", len(interviews), resp.InterviewCount)
	}
	if resp.TrendSummary == nil {
		t.Fatal("expected trend summary")
	}
	if resp.TrendSummary.TopFocusTag != "状态定义不清" {
		t.Fatalf("expected recent trend to focus 状态定义不清, got %#v", resp.TrendSummary)
	}
	if len(resp.FocusSignals) == 0 || resp.FocusSignals[0].FocusTag != "状态定义不清" {
		t.Fatalf("expected focus signals to use recent interviews, got %#v", resp.FocusSignals)
	}
}

// TestGrowthServiceGetWeeklyFocusUsesStructuredSignals 验证本周重点补强会直接复用统一训练重点信号结构。
func TestGrowthServiceGetWeeklyFocusUsesStructuredSignals(t *testing.T) {
	t.Parallel()

	reportJSON := `{"weaknesses":["状态定义不清"],"suggestions":["先口述状态定义，再开始写代码。"]}`
	svc := &growthService{
		interviewRepo: &growthInterviewRepositoryStub{
			interviews: []model.MockInterview{
				{
					BaseModel:  model.BaseModel{ID: 801},
					Status:     model.InterviewStatusCompleted,
					ReportJSON: reportJSON,
				},
			},
		},
		learningArchiveRepo: growthLearningArchiveRepositoryStub{
			entries: []model.LearningArchiveEntry{
				{
					IndustryCode:    "go",
					SourceRef:       "practice:7:801",
					MistakeTagsJSON: `["状态定义不清"]`,
					SuggestionsJSON: `["把变量命名改成能反映语义的名称。"]`,
				},
			},
		},
	}

	resp, err := svc.GetWeeklyFocus(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetWeeklyFocus returned error: %v", err)
	}
	if len(resp.Themes) != 1 {
		t.Fatalf("expected 1 weekly focus theme, got %d", len(resp.Themes))
	}
	theme := resp.Themes[0]
	if theme.Source != "mixed" || len(theme.RelatedQuestionSets) == 0 {
		t.Fatalf("expected mixed theme with question sets, got %#v", theme)
	}
	if theme.OccurrenceCount != 2 || theme.InterviewOccurrenceCount != 1 {
		t.Fatalf("unexpected occurrence stats: %#v", theme)
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

// growthLearningArchiveRepositoryStub 模拟学习档案仓库，供成长档案服务聚合训练重点信号。
type growthLearningArchiveRepositoryStub struct {
	entries []model.LearningArchiveEntry
}

// Upsert 满足接口要求，当前测试不依赖写入行为。
func (s growthLearningArchiveRepositoryStub) Upsert(context.Context, *model.LearningArchiveEntry) error {
	return nil
}

// ListRecentByUser 返回预置学习档案列表。
func (s growthLearningArchiveRepositoryStub) ListRecentByUser(context.Context, uint, int, *uint) ([]model.LearningArchiveEntry, error) {
	return append([]model.LearningArchiveEntry(nil), s.entries...), nil
}
