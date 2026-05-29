package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
	"makejob-backend/internal/mq"
)

// TestInterviewServiceCreateInterviewQueuesResumeParse 验证简历驱动实时面试会先入队简历解析任务并返回 preparing。
func TestInterviewServiceCreateInterviewQueuesResumeParse(t *testing.T) {
	t.Parallel()

	interviewRepo := &stubInterviewRepository{}
	asyncTaskRepo := &stubAsyncTaskRepository{}
	publisher := &stubTaskPublisher{}
	svc := &interviewService{
		interviewRepo:   interviewRepo,
		resumeParser:    &stubResumeParser{},
		realtimeEnabled: true,
		taskPublisher:   publisher,
		asyncTaskRepo:   asyncTaskRepo,
		asyncEnabled:    true,
	}

	resp, err := svc.CreateInterview(context.Background(), 3, &CreateInterviewRequest{
		IndustryCode:   "go",
		InterviewMode:  "resume_driven",
		ResumeText:     "5年 Go 后端开发经验",
		JobDescription: "负责高并发系统设计",
	})
	if err != nil {
		t.Fatalf("CreateInterview returned error: %v", err)
	}
	if resp.Status != model.InterviewStatusPreparing {
		t.Fatalf("expected preparing status, got %q", resp.Status)
	}
	if resp.AsyncTaskID == 0 {
		t.Fatal("expected async task id to be returned")
	}
	if resp.TaskStatus != model.AsyncTaskStatusQueued {
		t.Fatalf("expected queued task status, got %q", resp.TaskStatus)
	}
	if interviewRepo.saved == nil || interviewRepo.saved.Status != model.InterviewStatusPreparing {
		t.Fatalf("expected interview saved as preparing, got %#v", interviewRepo.saved)
	}
	if publisher.lastRoutingKey != "interview.resume.parse" {
		t.Fatalf("unexpected routing key: %q", publisher.lastRoutingKey)
	}
}

// TestInterviewServiceCreateInterviewFallsBackToLocalResumeParse 验证 MQ 不可用时会回退本地简历解析并直接进入 ongoing。
func TestInterviewServiceCreateInterviewFallsBackToLocalResumeParse(t *testing.T) {
	t.Parallel()

	interviewRepo := &stubInterviewRepository{}
	asyncTaskRepo := &stubAsyncTaskRepository{}
	publisher := &stubTaskPublisher{publishErr: errors.New("mq unavailable")}
	resumeParser := &stubResumeParser{
		profile: &ai.ResumeProfile{
			Summary: "Go 工程师",
			Skills:  []string{"Go", "MySQL"},
		},
	}
	svc := &interviewService{
		interviewRepo:   interviewRepo,
		resumeParser:    resumeParser,
		realtimeEnabled: true,
		taskPublisher:   publisher,
		asyncTaskRepo:   asyncTaskRepo,
		asyncEnabled:    true,
	}

	resp, err := svc.CreateInterview(context.Background(), 4, &CreateInterviewRequest{
		IndustryCode:   "go",
		InterviewMode:  "resume_driven",
		ResumeText:     "熟悉 Go 和 MySQL",
		JobDescription: "服务端开发",
	})
	if err != nil {
		t.Fatalf("CreateInterview returned error: %v", err)
	}
	if resp.Status != model.InterviewStatusOngoing {
		t.Fatalf("expected ongoing status, got %q", resp.Status)
	}
	if resumeParser.called == 0 {
		t.Fatal("expected local resume parser fallback to run")
	}
	if interviewRepo.saved == nil || interviewRepo.saved.Status != model.InterviewStatusOngoing {
		t.Fatalf("expected interview to be activated locally, got %#v", interviewRepo.saved)
	}
}

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

// TestInterviewServiceFinishInterviewIsIdempotent 验证重复结束已完成面试时会直接返回已有报告。
func TestInterviewServiceFinishInterviewIsIdempotent(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 4, 21, 9, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(23 * time.Minute)
	report := ai.InterviewReport{
		OverallScore:   88,
		TotalQuestions: 4,
		CorrectCount:   3,
		Summary:        "报告已生成。",
	}
	reportJSON, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report failed: %v", err)
	}

	svc := &interviewService{
		interviewRepo: &stubInterviewRepository{
			interview: &model.MockInterview{
				BaseModel:      model.BaseModel{ID: 19},
				UserID:         1,
				Status:         model.InterviewStatusCompleted,
				Score:          report.OverallScore,
				AIFeedback:     "旧摘要",
				ReportJSON:     string(reportJSON),
				TotalQuestions: report.TotalQuestions,
				StartedAt:      &startedAt,
				EndedAt:        &endedAt,
			},
		},
		interviewAgent: &stubInterviewAgent{},
	}

	resp, err := svc.FinishInterview(context.Background(), 1, 19)
	if err != nil {
		t.Fatalf("FinishInterview returned error: %v", err)
	}
	if resp == nil || resp.Report == nil {
		t.Fatal("expected report response, got nil")
	}
	if resp.Report.Summary != report.Summary {
		t.Fatalf("unexpected report summary: got %q want %q", resp.Report.Summary, report.Summary)
	}
}

// TestInterviewServiceFinishInterviewQueuesAsyncReport 验证结束面试时会异步排队报告生成并立即返回处理中状态。
func TestInterviewServiceFinishInterviewQueuesAsyncReport(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 4, 21, 9, 0, 0, 0, time.UTC)
	interviewRepo := &stubInterviewRepository{
		interview: &model.MockInterview{
			BaseModel:      model.BaseModel{ID: 31},
			UserID:         1,
			Status:         model.InterviewStatusOngoing,
			AISessionID:    "session-31",
			TotalQuestions: 3,
			StartedAt:      &startedAt,
		},
	}
	asyncTaskRepo := &stubAsyncTaskRepository{}
	publisher := &stubTaskPublisher{}
	svc := &interviewService{
		interviewRepo:  interviewRepo,
		interviewAgent: &stubInterviewAgent{},
		taskPublisher:  publisher,
		asyncTaskRepo:  asyncTaskRepo,
		asyncEnabled:   true,
	}

	resp, err := svc.FinishInterview(context.Background(), 1, 31)
	if err != nil {
		t.Fatalf("FinishInterview returned error: %v", err)
	}
	if resp.Status != model.InterviewStatusReportGenerating {
		t.Fatalf("expected report_generating status, got %q", resp.Status)
	}
	if resp.Report != nil {
		t.Fatalf("expected nil report while generating, got %#v", resp.Report)
	}
	if resp.AsyncTaskID == 0 || resp.TaskStatus != model.AsyncTaskStatusQueued {
		t.Fatalf("expected queued async task, got id=%d status=%q", resp.AsyncTaskID, resp.TaskStatus)
	}
	if interviewRepo.saved == nil || interviewRepo.saved.Status != model.InterviewStatusReportGenerating {
		t.Fatalf("expected interview persisted as report_generating, got %#v", interviewRepo.saved)
	}
}

// TestInterviewServiceGetReportReturnsProcessingState 验证报告生成中时接口返回处理中状态和任务信息。
func TestInterviewServiceGetReportReturnsProcessingState(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 4, 21, 9, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(12 * time.Minute)
	asyncTaskRepo := &stubAsyncTaskRepository{
		latestTask: &model.AsyncTask{
			BaseModel: model.BaseModel{ID: 88},
			Status:    model.AsyncTaskStatusRunning,
		},
	}
	svc := &interviewService{
		interviewRepo: &stubInterviewRepository{
			interview: &model.MockInterview{
				BaseModel:      model.BaseModel{ID: 41},
				UserID:         9,
				Status:         model.InterviewStatusReportGenerating,
				StartedAt:      &startedAt,
				EndedAt:        &endedAt,
				TotalQuestions: 5,
			},
		},
		asyncTaskRepo: asyncTaskRepo,
	}

	resp, err := svc.GetReport(context.Background(), 9, 41)
	if err != nil {
		t.Fatalf("GetReport returned error: %v", err)
	}
	if resp.Status != model.InterviewStatusReportGenerating {
		t.Fatalf("expected processing status, got %q", resp.Status)
	}
	if resp.Report != nil {
		t.Fatalf("expected nil report, got %#v", resp.Report)
	}
	if resp.AsyncTaskID != 88 || resp.TaskStatus != model.AsyncTaskStatusRunning {
		t.Fatalf("unexpected task state: %#v", resp)
	}
}

// TestInterviewServiceGetRealtimeContextRejectsPreparing 验证简历驱动实时面试在 preparing 阶段不会提前建立实时会话。
func TestInterviewServiceGetRealtimeContextRejectsPreparing(t *testing.T) {
	t.Parallel()

	svc := &interviewService{
		interviewRepo: &stubInterviewRepository{
			interview: &model.MockInterview{
				BaseModel: model.BaseModel{ID: 51},
				UserID:    4,
				Status:    model.InterviewStatusPreparing,
			},
		},
		interviewMessageRepo: &stubInterviewMessageRepository{},
	}

	_, err := svc.GetRealtimeContext(context.Background(), 4, 51)
	if err == nil {
		t.Fatal("expected preparing interview to reject realtime bootstrap")
	}
	if err.Error() != "简历解析中，请稍后开始面试" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestInterviewServiceAppendRealtimeAssistantReplyRejectsPreparing 验证 preparing 阶段即使收到首轮回复也不会误写入消息流。
func TestInterviewServiceAppendRealtimeAssistantReplyRejectsPreparing(t *testing.T) {
	t.Parallel()

	messageRepo := &stubInterviewMessageRepository{}
	svc := &interviewService{
		interviewRepo: &stubInterviewRepository{
			interview: &model.MockInterview{
				BaseModel:      model.BaseModel{ID: 52},
				UserID:         4,
				Status:         model.InterviewStatusPreparing,
				TotalQuestions: 20,
			},
		},
		interviewMessageRepo: messageRepo,
	}

	_, _, _, err := svc.AppendRealtimeAssistantReply(context.Background(), 4, 52, "你好，我们先从自我介绍开始。")
	if err == nil {
		t.Fatal("expected preparing interview to reject assistant reply")
	}
	if err.Error() != "简历解析中，请稍后开始面试" {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messageRepo.messages) != 0 {
		t.Fatalf("expected no messages to be written, got %d", len(messageRepo.messages))
	}
}

// stubInterviewRepository 模拟面试仓库，供服务层测试验证持久化结果。
type stubInterviewRepository struct {
	interview *model.MockInterview
	saved     *model.MockInterview
}

// Create 满足接口，当前测试不依赖该行为。
func (s *stubInterviewRepository) Create(_ context.Context, interview *model.MockInterview) error {
	if interview == nil {
		return errors.New("interview is nil")
	}
	clone := *interview
	if clone.ID == 0 {
		clone.ID = 1
	}
	s.interview = &clone
	s.saved = &clone
	interview.ID = clone.ID
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

// stubResumeParser 模拟简历解析器，便于验证异步回退行为。
type stubResumeParser struct {
	profile *ai.ResumeProfile
	err     error
	called  int
}

// Parse 返回预置简历画像。
func (s *stubResumeParser) Parse(context.Context, string, string) (*ai.ResumeProfile, error) {
	s.called++
	if s.err != nil {
		return nil, s.err
	}
	return s.profile, nil
}

// stubTaskPublisher 模拟 RabbitMQ 发布器。
type stubTaskPublisher struct {
	lastRoutingKey string
	lastMessage    mq.TaskMessage
	publishErr     error
}

// PublishTask 记录最近一次发布请求。
func (s *stubTaskPublisher) PublishTask(_ context.Context, routingKey string, message mq.TaskMessage) error {
	s.lastRoutingKey = routingKey
	s.lastMessage = message
	return s.publishErr
}

// Close 满足发布器接口。
func (s *stubTaskPublisher) Close() error {
	return nil
}

// stubAsyncTaskRepository 模拟异步任务仓库，供 interview 异步化测试使用。
type stubAsyncTaskRepository struct {
	nextID     uint
	tasks      map[uint]*model.AsyncTask
	latestTask *model.AsyncTask
}

// Create 保存异步任务并分配 ID。
func (s *stubAsyncTaskRepository) Create(_ context.Context, task *model.AsyncTask) error {
	if task == nil {
		return errors.New("task is nil")
	}
	if s.tasks == nil {
		s.tasks = map[uint]*model.AsyncTask{}
	}
	if s.nextID == 0 {
		s.nextID = 1
	}
	clone := *task
	clone.ID = s.nextID
	s.nextID++
	s.tasks[clone.ID] = &clone
	s.latestTask = &clone
	task.ID = clone.ID
	return nil
}

// GetByID 返回指定任务。
func (s *stubAsyncTaskRepository) GetByID(_ context.Context, id uint) (*model.AsyncTask, error) {
	if s.tasks == nil {
		return nil, nil
	}
	task, ok := s.tasks[id]
	if !ok {
		return nil, nil
	}
	clone := *task
	return &clone, nil
}

// GetByIdempotencyKey 根据幂等键查任务。
func (s *stubAsyncTaskRepository) GetByIdempotencyKey(_ context.Context, key string) (*model.AsyncTask, error) {
	for _, task := range s.tasks {
		if task.IdempotencyKey == key {
			clone := *task
			return &clone, nil
		}
	}
	return nil, nil
}

// GetLatestByEntity 返回预置的最新任务状态。
func (s *stubAsyncTaskRepository) GetLatestByEntity(context.Context, string, uint, ...string) (*model.AsyncTask, error) {
	if s.latestTask == nil {
		return nil, nil
	}
	clone := *s.latestTask
	return &clone, nil
}

// Update 覆盖保存任务状态。
func (s *stubAsyncTaskRepository) Update(_ context.Context, task *model.AsyncTask) error {
	if task == nil {
		return errors.New("task is nil")
	}
	if s.tasks == nil {
		s.tasks = map[uint]*model.AsyncTask{}
	}
	clone := *task
	s.tasks[clone.ID] = &clone
	s.latestTask = &clone
	return nil
}

// ClaimByID 返回任务并允许执行，满足消费测试接口。
func (s *stubAsyncTaskRepository) ClaimByID(_ context.Context, id uint) (*model.AsyncTask, bool, error) {
	task, err := s.GetByID(context.Background(), id)
	if err != nil || task == nil {
		return task, false, err
	}
	task.RetryCount++
	task.Status = model.AsyncTaskStatusRunning
	_ = s.Update(context.Background(), task)
	return task, true, nil
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
