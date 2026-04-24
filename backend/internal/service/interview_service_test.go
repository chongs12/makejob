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

// TestInterviewServiceSubmitAnswerUsesAnsweredCount 验证提交回答时会直接推进下一题且不再即时生成反馈。
func TestInterviewServiceSubmitAnswerUsesAnsweredCount(t *testing.T) {
	t.Parallel()

	messageRepo := &stubInterviewMessageRepository{
		messages: []model.InterviewMessage{
			{Role: model.MessageRoleAI, Content: "第1题", MessageType: model.MessageTypeText},
			{Role: model.MessageRoleUser, Content: "回答1", MessageType: model.MessageTypeText},
			{Role: model.MessageRoleAI, Content: "反馈1", MessageType: model.MessageTypeFeedback},
			{Role: model.MessageRoleAI, Content: "第2题", MessageType: model.MessageTypeText},
		},
	}
	agent := &stubInterviewAgent{
		evaluatedQuestionIndex: -1,
		nextQuestion: ai.InterviewQuestion{
			Question: "第3题",
			Type:     "technical",
		},
		hasNext: true,
	}
	svc := &interviewService{
		interviewRepo: &stubInterviewRepository{
			interview: &model.MockInterview{
				BaseModel:      model.BaseModel{ID: 9},
				UserID:         1,
				Status:         model.InterviewStatusOngoing,
				AISessionID:    "session-9",
				TotalQuestions: 3,
			},
		},
		interviewMessageRepo: messageRepo,
		interviewAgent:       agent,
	}

	resp, err := svc.SubmitAnswer(context.Background(), 1, 9, &InterviewAnswerRequest{Answer: "第二题回答"})
	if err != nil {
		t.Fatalf("SubmitAnswer returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Feedback != nil {
		t.Fatalf("expected no immediate feedback, got %#v", resp.Feedback)
	}
	if resp.NextQuestion == nil || resp.NextQuestion.Question != "第3题" {
		t.Fatalf("expected next question to be returned, got %#v", resp.NextQuestion)
	}
	if agent.evaluatedQuestionIndex != -1 {
		t.Fatalf("expected no immediate evaluation, got %d", agent.evaluatedQuestionIndex)
	}
	if got := len(messageRepo.messages); got != 6 {
		t.Fatalf("expected answer and next question messages to be appended, got %d", got)
	}
}

// TestInterviewServiceFinishInterviewEvaluatesAnswersForReport 验证结束面试时会补齐所有答案评分再生成报告。
func TestInterviewServiceFinishInterviewEvaluatesAnswersForReport(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 4, 21, 9, 0, 0, 0, time.UTC)
	messageRepo := &stubInterviewMessageRepository{
		messages: []model.InterviewMessage{
			{Role: model.MessageRoleAI, Content: "第1题", MessageType: model.MessageTypeText},
			{Role: model.MessageRoleUser, Content: "回答1", MessageType: model.MessageTypeText},
			{Role: model.MessageRoleAI, Content: "第2题", MessageType: model.MessageTypeText},
			{Role: model.MessageRoleUser, Content: "回答2", MessageType: model.MessageTypeText},
		},
	}
	agent := &stubInterviewAgent{
		report: ai.InterviewReport{
			OverallScore:   90,
			TotalQuestions: 2,
			CorrectCount:   2,
			Summary:        "整体表现良好。",
		},
	}
	svc := &interviewService{
		interviewRepo: &stubInterviewRepository{
			interview: &model.MockInterview{
				BaseModel:      model.BaseModel{ID: 15},
				UserID:         1,
				Status:         model.InterviewStatusOngoing,
				AISessionID:    "session-15",
				TotalQuestions: 2,
				StartedAt:      &startedAt,
			},
		},
		interviewMessageRepo: messageRepo,
		interviewAgent:       agent,
	}

	if _, err := svc.FinishInterview(context.Background(), 1, 15); err != nil {
		t.Fatalf("FinishInterview returned error: %v", err)
	}
	if len(agent.evaluatedQuestionIndices) != 2 {
		t.Fatalf("expected 2 deferred evaluations, got %d", len(agent.evaluatedQuestionIndices))
	}
	if agent.evaluatedQuestionIndices[0] != 0 || agent.evaluatedQuestionIndices[1] != 1 {
		t.Fatalf("unexpected deferred evaluation order: %#v", agent.evaluatedQuestionIndices)
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
	report                   ai.InterviewReport
	endedSessionID           string
	feedback                 ai.AnswerFeedback
	nextQuestion             ai.InterviewQuestion
	hasNext                  bool
	evaluatedQuestionIndex   int
	evaluatedQuestionIndices []int
}

// StartInterview 满足接口，当前测试不依赖该行为。
func (s *stubInterviewAgent) StartInterview(context.Context, ai.InterviewConfig) (string, ai.InterviewQuestion, error) {
	return "", ai.InterviewQuestion{}, nil
}

// EvaluateAnswer 满足接口，当前测试不依赖该行为。
func (s *stubInterviewAgent) EvaluateAnswer(_ context.Context, _ string, questionIndex int, _ string) (ai.AnswerFeedback, error) {
	s.evaluatedQuestionIndex = questionIndex
	s.evaluatedQuestionIndices = append(s.evaluatedQuestionIndices, questionIndex)
	return s.feedback, nil
}

// GetNextQuestion 满足接口，当前测试不依赖该行为。
func (s *stubInterviewAgent) GetNextQuestion(context.Context, string) (ai.InterviewQuestion, bool, error) {
	return s.nextQuestion, s.hasNext, nil
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

// stubInterviewMessageRepository 模拟面试消息仓库，供题号计算测试使用。
type stubInterviewMessageRepository struct {
	messages []model.InterviewMessage
}

// Create 追加一条消息，模拟服务层写入面试历史。
func (s *stubInterviewMessageRepository) Create(_ context.Context, msg *model.InterviewMessage) error {
	if msg == nil {
		return errors.New("message is nil")
	}
	s.messages = append(s.messages, *msg)
	return nil
}

// ListByInterview 返回测试预置的消息列表。
func (s *stubInterviewMessageRepository) ListByInterview(context.Context, uint) ([]model.InterviewMessage, error) {
	cloned := make([]model.InterviewMessage, len(s.messages))
	copy(cloned, s.messages)
	return cloned, nil
}

// CountByInterview 满足接口，当前测试不依赖该返回值。
func (s *stubInterviewMessageRepository) CountByInterview(context.Context, uint) (int64, error) {
	return int64(len(s.messages)), nil
}
