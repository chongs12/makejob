package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

// TestInterviewServiceFinishInterviewPersistsReport 验证结束面试时会持久化完整报告与摘要。
func TestInterviewServiceFinishInterviewPersistsReport(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 4, 21, 9, 0, 0, 0, time.UTC)
	interviewRepo := &stubInterviewRepository{
		interview: &model.MockInterview{
			BaseModel:      model.BaseModel{ID: 7},
			UserID:         1,
			Status:         model.InterviewStatusOngoing,
			AISessionID:    "session-7",
			TotalQuestions: 5,
			StartedAt:      &startedAt,
		},
	}
	agent := &stubInterviewAgent{
		report: ai.InterviewReport{
			OverallScore:   92,
			TotalQuestions: 5,
			CorrectCount:   4,
			DimensionScores: map[string]float64{
				"技术深度": 91,
				"表达":   88,
			},
			Strengths:   []string{"并发理解扎实"},
			Weaknesses:  []string{"缓存细节欠缺"},
			Suggestions: []string{"补 Redis 淘汰策略"},
			Summary:     "整体表现稳定，已具备较强 Go 面试基础。",
		},
	}

	svc := &interviewService{
		interviewRepo:  interviewRepo,
		interviewAgent: agent,
	}

	resp, err := svc.FinishInterview(context.Background(), 1, 7)
	if err != nil {
		t.Fatalf("FinishInterview returned error: %v", err)
	}

	if resp.Report == nil {
		t.Fatal("expected report in response, got nil")
	}
	if interviewRepo.saved == nil {
		t.Fatal("expected interview to be persisted")
	}
	if interviewRepo.saved.AISessionID != "session-7" {
		t.Fatalf("expected ai session id to be preserved, got %q", interviewRepo.saved.AISessionID)
	}
	if interviewRepo.saved.AIFeedback == "" {
		t.Fatal("expected summary to be persisted into ai_feedback")
	}
	if interviewRepo.saved.ReportJSON == "" {
		t.Fatal("expected report json to be persisted")
	}

	var stored ai.InterviewReport
	if err := json.Unmarshal([]byte(interviewRepo.saved.ReportJSON), &stored); err != nil {
		t.Fatalf("expected valid report json, got error: %v", err)
	}
	if stored.Summary != agent.report.Summary {
		t.Fatalf("unexpected stored summary: got %q want %q", stored.Summary, agent.report.Summary)
	}
	if agent.endedSessionID != "session-7" {
		t.Fatalf("expected EndInterview to use persisted session id, got %q", agent.endedSessionID)
	}
}

// TestInterviewServiceGetReportReadsStoredReport 验证读取报告时优先返回持久化的完整报告。
func TestInterviewServiceGetReportReadsStoredReport(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 4, 21, 9, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(18 * time.Minute)
	report := ai.InterviewReport{
		OverallScore:   87,
		TotalQuestions: 6,
		CorrectCount:   4,
		DimensionScores: map[string]float64{
			"技术深度": 86,
		},
		Strengths:   []string{"思路清晰"},
		Weaknesses:  []string{"边界条件覆盖不足"},
		Suggestions: []string{"补充错误处理案例"},
		Summary:     "完成度较高，但还需要补足细节。",
	}
	reportJSON, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report failed: %v", err)
	}

	svc := &interviewService{
		interviewRepo: &stubInterviewRepository{
			interview: &model.MockInterview{
				BaseModel:      model.BaseModel{ID: 11},
				UserID:         2,
				Status:         model.InterviewStatusCompleted,
				Score:          report.OverallScore,
				AIFeedback:     "旧摘要",
				ReportJSON:     string(reportJSON),
				TotalQuestions: report.TotalQuestions,
				StartedAt:      &startedAt,
				EndedAt:        &endedAt,
			},
		},
	}

	resp, err := svc.GetReport(context.Background(), 2, 11)
	if err != nil {
		t.Fatalf("GetReport returned error: %v", err)
	}

	if resp.Report == nil {
		t.Fatal("expected report, got nil")
	}
	if resp.Report.Summary != report.Summary {
		t.Fatalf("unexpected summary: got %q want %q", resp.Report.Summary, report.Summary)
	}
	if len(resp.Report.Strengths) != 1 || resp.Report.Strengths[0] != "思路清晰" {
		t.Fatalf("expected persisted strengths, got %#v", resp.Report.Strengths)
	}
}

// TestInterviewServiceResolveSessionIDFallback 验证旧数据仍可从 ai_feedback 回退读取会话 ID。
func TestInterviewServiceResolveSessionIDFallback(t *testing.T) {
	t.Parallel()

	interview := &model.MockInterview{
		Status:     model.InterviewStatusOngoing,
		AIFeedback: "legacy-session-id",
	}
	if got := resolveInterviewSessionID(interview); got != "legacy-session-id" {
		t.Fatalf("unexpected fallback session id: got %q", got)
	}
}

// stubInterviewRepository 模拟面试仓库，供服务层测试验证持久化结果。
type stubInterviewRepository struct {
	interview *model.MockInterview
	saved     *model.MockInterview
}

// Create 满足接口，当前测试不依赖该行为。
func (s *stubInterviewRepository) Create(context.Context, *model.MockInterview) error {
	return nil
}

// GetByID 返回预置的面试记录。
func (s *stubInterviewRepository) GetByID(context.Context, uint) (*model.MockInterview, error) {
	if s.interview == nil {
		return nil, nil
	}
	clone := *s.interview
	return &clone, nil
}

// Update 记录服务层最终写回的面试数据。
func (s *stubInterviewRepository) Update(_ context.Context, interview *model.MockInterview) error {
	if interview == nil {
		return errors.New("interview is nil")
	}
	clone := *interview
	s.saved = &clone
	s.interview = &clone
	return nil
}

// ListByUser 满足接口，当前测试不依赖该行为。
func (s *stubInterviewRepository) ListByUser(context.Context, uint, int, int) ([]model.MockInterview, int64, error) {
	return nil, 0, nil
}

// GetUserDailyCount 满足接口，当前测试不依赖该行为。
func (s *stubInterviewRepository) GetUserDailyCount(context.Context, uint, time.Time) (int64, error) {
	return 0, nil
}

// stubInterviewAgent 模拟面试 Agent，仅提供报告生成与会话结束行为。
type stubInterviewAgent struct {
	report         ai.InterviewReport
	endedSessionID string
}

// StartInterview 满足接口，当前测试不依赖该行为。
func (s *stubInterviewAgent) StartInterview(context.Context, ai.InterviewConfig) (string, ai.InterviewQuestion, error) {
	return "", ai.InterviewQuestion{}, nil
}

// EvaluateAnswer 满足接口，当前测试不依赖该行为。
func (s *stubInterviewAgent) EvaluateAnswer(context.Context, string, int, string) (ai.AnswerFeedback, error) {
	return ai.AnswerFeedback{}, nil
}

// GetNextQuestion 满足接口，当前测试不依赖该行为。
func (s *stubInterviewAgent) GetNextQuestion(context.Context, string) (ai.InterviewQuestion, bool, error) {
	return ai.InterviewQuestion{}, false, nil
}

// GenerateReport 返回测试预置的完整面试报告。
func (s *stubInterviewAgent) GenerateReport(context.Context, string) (ai.InterviewReport, error) {
	return s.report, nil
}

// EndInterview 记录结束会话时使用的 sessionID。
func (s *stubInterviewAgent) EndInterview(_ context.Context, sessionID string) error {
	s.endedSessionID = sessionID
	return nil
}
