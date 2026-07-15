package biz

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// codeRunnerStub 记录 CodeRunner 调用参数，用于断言
type codeRunnerStub struct {
	capturedLanguage  string
	capturedCode      string
	capturedTestCases []CodeTestCase
	result            *CodeRunnerResult
	err               error
}

func (c *codeRunnerStub) Execute(_ context.Context, language, code string, testCases []CodeTestCase) (*CodeRunnerResult, error) {
	c.capturedLanguage = language
	c.capturedCode = code
	c.capturedTestCases = testCases
	if c.result != nil {
		return c.result, nil
	}
	return nil, c.err
}

// interviewRepoStub 提供面试用例测试所需的最小仓储行为。
type interviewRepoStub struct {
	inTransaction    bool
	transactionCalls int
	createCalls      int
	messageCalls     int
	nextInterviewID  uint64
	messages         []*InterviewMessage
	interview        *Interview
}

// Create 记录面试创建调用并回填伪造主键。
func (r *interviewRepoStub) Create(_ context.Context, interview *Interview) error {
	r.createCalls++
	if r.nextInterviewID == 0 {
		r.nextInterviewID = 1
	}
	interview.ID = r.nextInterviewID
	interview.CreatedAt = time.Unix(1700000000, 0)
	return nil
}

// GetByID 返回预置面试记录。
func (r *interviewRepoStub) GetByID(_ context.Context, id uint64) (*Interview, error) {
	if r.interview != nil {
		return r.interview, nil
	}
	return nil, errors.New("not implemented")
}

// ListByUser 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) ListByUser(context.Context, uint64, int32, int32) ([]*Interview, int64, error) {
	return nil, 0, errors.New("not implemented")
}

// Update 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) Update(context.Context, *Interview) error {
	return errors.New("not implemented")
}

// CreateMessage 记录首题消息创建调用。
func (r *interviewRepoStub) CreateMessage(_ context.Context, msg *InterviewMessage) error {
	r.messageCalls++
	if msg.InterviewID == 0 {
		return errors.New("message interview_id should be assigned")
	}
	return nil
}

// ListMessages 返回预置消息列表。
func (r *interviewRepoStub) ListMessages(context.Context, uint64) ([]*InterviewMessage, error) {
	if r.messages != nil {
		return r.messages, nil
	}
	return nil, errors.New("not implemented")
}

// ListMessagesLimited 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) ListMessagesLimited(context.Context, uint64, int32) ([]*InterviewMessage, error) {
	return nil, errors.New("not implemented")
}

// CreateCodingAttempt 记录编程答题记录创建调用。
func (r *interviewRepoStub) CreateCodingAttempt(_ context.Context, _ *CodingAttempt) error {
	return nil
}

// UpdateCodingAttempt 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) UpdateCodingAttempt(context.Context, *CodingAttempt) error {
	return nil
}

// ListCodingAttempts 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) ListCodingAttempts(context.Context, uint64) ([]*CodingAttempt, error) {
	return nil, errors.New("not implemented")
}

// BindRealtimeDialog 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) BindRealtimeDialog(context.Context, uint64, string) error {
	return errors.New("not implemented")
}

// AppendMessageAndBumpIndex 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) AppendMessageAndBumpIndex(context.Context, *InterviewMessage) error {
	return errors.New("not implemented")
}

// Transaction 用于检测事务闭包执行期间的状态。
func (r *interviewRepoStub) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	r.transactionCalls++
	r.inTransaction = true
	defer func() {
		r.inTransaction = false
	}()
	return fn(ctx)
}

// GetStats 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) GetStats(context.Context, uint64) (*InterviewStats, error) {
	return nil, errors.New("not implemented")
}

// GetAdminStats 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) GetAdminStats(context.Context) (int64, error) {
	return 0, errors.New("not implemented")
}

// interviewAIStub 提供首题生成的测试双桩。
type interviewAIStub struct {
	repo          *interviewRepoStub
	question      *InterviewQuestion
	err           error
	interviewCall int
}

// InterviewAgent 返回预置首题，并断言不会在事务中被调用。
func (a *interviewAIStub) InterviewAgent(_ context.Context, req *InterviewAgentRequest) (*InterviewAgentResponse, error) {
	a.interviewCall++
	if a.repo != nil && a.repo.inTransaction {
		return nil, errors.New("ai should not be called inside transaction")
	}
	if a.err != nil {
		return nil, a.err
	}
	return &InterviewAgentResponse{Question: a.question}, nil
}

// QuizAnalyzer 返回未实现错误，当前测试不会走到这里。
func (a *interviewAIStub) QuizAnalyzer(context.Context, *QuizAnalyzerRequest) (*QuizAnalyzerResponse, error) {
	return nil, errors.New("not implemented")
}

// ResumeParser 返回未实现错误，当前测试不会走到这里。
func (a *interviewAIStub) ResumeParser(context.Context, *ResumeParserRequest) (*ResumeParserResponse, error) {
	return nil, errors.New("not implemented")
}

// StartInterview 返回预置首题和 sessionID（对齐单体 InterviewAgent.StartInterview）。
func (a *interviewAIStub) StartInterview(_ context.Context, req *StartInterviewRequest) (*StartInterviewResponse, error) {
	a.interviewCall++
	if a.repo != nil && a.repo.inTransaction {
		return nil, errors.New("ai should not be called inside transaction")
	}
	if a.err != nil {
		return nil, a.err
	}
	return &StartInterviewResponse{
		SessionID:  "test-session-id",
		Question:   a.question.Question,
		Topic:      a.question.Topic,
		Difficulty: a.question.Difficulty,
		Type:       a.question.Type,
		Hints:      a.question.Hints,
	}, nil
}

// EvaluateAnswer 返回未实现错误，当前测试不会走到这里。
func (a *interviewAIStub) EvaluateAnswer(context.Context, *EvaluateAnswerRequest) (*EvaluateAnswerResponse, error) {
	return nil, errors.New("not implemented")
}

// GetNextQuestionSession 返回未实现错误，当前测试不会走到这里。
func (a *interviewAIStub) GetNextQuestionSession(context.Context, *GetNextQuestionSessionRequest) (*GetNextQuestionSessionResponse, error) {
	return nil, errors.New("not implemented")
}

// GenerateInterviewReport 返回未实现错误，当前测试不会走到这里。
func (a *interviewAIStub) GenerateInterviewReport(context.Context, *GenerateInterviewReportRequest) (*GenerateInterviewReportResponse, error) {
	return nil, errors.New("not implemented")
}

// EndInterviewSession 返回未实现错误，当前测试不会走到这里。
func (a *interviewAIStub) EndInterviewSession(context.Context, *EndInterviewSessionRequest) (*EndInterviewSessionResponse, error) {
	return nil, errors.New("not implemented")
}

// GenerateReportFromHistory 返回未实现错误，当前测试不会走到这里。
func (a *interviewAIStub) GenerateReportFromHistory(context.Context, *GenerateReportFromHistoryRequest) (*GenerateInterviewReportResponse, error) {
	return nil, errors.New("not implemented")
}

// GenerateKnowledgeReport 知识点专项报告，测试不会走到这里。
func (a *interviewAIStub) GenerateKnowledgeReport(context.Context, *GenerateKnowledgeReportRequest) (*GenerateKnowledgeReportResponse, error) {
	return nil, errors.New("not implemented")
}

// GenerateJobReport 岗位求职报告，测试不会走到这里。
func (a *interviewAIStub) GenerateJobReport(context.Context, *GenerateJobReportRequest) (*GenerateJobReportResponse, error) {
	return nil, errors.New("not implemented")
}

// interviewIndustryStub 实现 IndustryClient 接口
type interviewIndustryStub struct{}

func (interviewIndustryStub) GetIndustry(_ context.Context, _ string) (*Industry, error) {
	return &Industry{ID: 1, Code: "backend", Name: "后端"}, nil
}

// archiveStub 记录学习档案调用，用于断言
type archiveStub struct {
	writeCalls      []*ArchiveEntry
	writeErr        error
	listByUserCalls int
	filteredEntries []*ArchiveEntry // ListBySource 返回的预设数据
}

func (a *archiveStub) WriteEntry(_ context.Context, entry *ArchiveEntry) error {
	a.writeCalls = append(a.writeCalls, entry)
	return a.writeErr
}

func (a *archiveStub) ListByUser(_ context.Context, _ uint64, _ int32) ([]*ArchiveEntry, error) {
	a.listByUserCalls++
	return nil, nil
}

func (a *archiveStub) ListBySource(_ context.Context, userID uint64, sourceType string, interviewID uint64) ([]*ArchiveEntry, error) {
	if a.filteredEntries != nil {
		return a.filteredEntries, nil
	}
	return nil, nil
}

// reportRepoStub 实现 ReportRepo 接口
type reportRepoStub struct{}

func (r *reportRepoStub) Create(_ context.Context, _ *InterviewReport) error { return nil }
func (r *reportRepoStub) GetByInterviewID(_ context.Context, _ uint64) (*InterviewReport, error) {
	return nil, errors.New("not implemented")
}

// quizAnalyzerStub 返回预设 AI 评分结果
type quizAnalyzerStub struct {
	result *QuizAnalyzerResponse
	err    error
}

func (q *quizAnalyzerStub) QuizAnalyzer(_ context.Context, _ *QuizAnalyzerRequest) (*QuizAnalyzerResponse, error) {
	return q.result, q.err
}

func (q *quizAnalyzerStub) ResumeParser(_ context.Context, _ *ResumeParserRequest) (*ResumeParserResponse, error) {
	return nil, errors.New("not implemented")
}

func (q *quizAnalyzerStub) InterviewAgent(_ context.Context, _ *InterviewAgentRequest) (*InterviewAgentResponse, error) {
	return nil, errors.New("not implemented")
}

func (q *quizAnalyzerStub) StartInterview(_ context.Context, _ *StartInterviewRequest) (*StartInterviewResponse, error) {
	return nil, errors.New("not implemented")
}

func (q *quizAnalyzerStub) EvaluateAnswer(_ context.Context, _ *EvaluateAnswerRequest) (*EvaluateAnswerResponse, error) {
	return nil, errors.New("not implemented")
}

func (q *quizAnalyzerStub) GetNextQuestionSession(_ context.Context, _ *GetNextQuestionSessionRequest) (*GetNextQuestionSessionResponse, error) {
	return nil, errors.New("not implemented")
}

func (q *quizAnalyzerStub) GenerateInterviewReport(_ context.Context, _ *GenerateInterviewReportRequest) (*GenerateInterviewReportResponse, error) {
	return nil, errors.New("not implemented")
}

func (q *quizAnalyzerStub) EndInterviewSession(_ context.Context, _ *EndInterviewSessionRequest) (*EndInterviewSessionResponse, error) {
	return nil, errors.New("not implemented")
}

func (q *quizAnalyzerStub) GenerateReportFromHistory(_ context.Context, _ *GenerateReportFromHistoryRequest) (*GenerateInterviewReportResponse, error) {
	return nil, errors.New("not implemented")
}

func (q *quizAnalyzerStub) GenerateKnowledgeReport(_ context.Context, _ *GenerateKnowledgeReportRequest) (*GenerateKnowledgeReportResponse, error) {
	return nil, errors.New("not implemented")
}

func (q *quizAnalyzerStub) GenerateJobReport(_ context.Context, _ *GenerateJobReportRequest) (*GenerateJobReportResponse, error) {
	return nil, errors.New("not implemented")
}

// TestSubmitCodingAnswerPassesHintsAsStdin 验证编程题有 Hints 时，CodeRunner 收到 stdin
func TestSubmitCodingAnswerPassesHintsAsStdin(t *testing.T) {
	questionContent := EncodeQuestionContent(&InterviewQuestion{
		Question: "写一个函数反转字符串",
		Topic:    "字符串",
		Hints:    "hello",
	})
	repo := &interviewRepoStub{
		interview: &Interview{
			ID:          1,
			UserID:      9,
			Status:      "ongoing",
			Difficulty:  "medium",
			IndustryCode: "backend",
		},
		messages: []*InterviewMessage{
			{ID: 10, Role: "assistant", Content: questionContent, QuestionIndex: 0},
		},
	}
	runner := &codeRunnerStub{
		result: &CodeRunnerResult{Success: true, Stdout: "olleh"},
	}
	ai := &quizAnalyzerStub{
		result: &QuizAnalyzerResponse{Score: 85, Feedback: "不错"},
	}
	uc := NewInterviewUseCase(repo, ai, nil, interviewIndustryStub{}, nil, nil, runner, &reportRepoStub{}, nil, log.DefaultLogger, 0)

	_, err := uc.SubmitCodingAnswer(context.Background(), 1, 9, 0, "go", "func reverse(s string) string {}")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(runner.capturedTestCases) != 1 {
		t.Fatalf("expected 1 test case (hints as stdin), got %d", len(runner.capturedTestCases))
	}
	if runner.capturedTestCases[0].Input != "hello" {
		t.Fatalf("expected stdin='hello', got '%s'", runner.capturedTestCases[0].Input)
	}
}

// TestSubmitCodingAnswerNoHintsPassesNilTestCases 验证编程题无 Hints 时，CodeRunner 收到 nil
func TestSubmitCodingAnswerNoHintsPassesNilTestCases(t *testing.T) {
	questionContent := EncodeQuestionContent(&InterviewQuestion{
		Question: "写一个函数反转字符串",
		Topic:    "字符串",
	})
	repo := &interviewRepoStub{
		interview: &Interview{
			ID:          1,
			UserID:      9,
			Status:      "ongoing",
			Difficulty:  "medium",
			IndustryCode: "backend",
		},
		messages: []*InterviewMessage{
			{ID: 10, Role: "assistant", Content: questionContent, QuestionIndex: 0},
		},
	}
	runner := &codeRunnerStub{
		result: &CodeRunnerResult{Success: true, Stdout: "olleh"},
	}
	ai := &quizAnalyzerStub{
		result: &QuizAnalyzerResponse{Score: 85, Feedback: "不错"},
	}
	archive := &archiveStub{}
	uc := NewInterviewUseCase(repo, ai, archive, interviewIndustryStub{}, nil, nil, runner, &reportRepoStub{}, nil, log.DefaultLogger, 0)

	_, err := uc.SubmitCodingAnswer(context.Background(), 1, 9, 0, "go", "func reverse(s string) string {}")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if runner.capturedTestCases != nil {
		t.Fatalf("expected nil test cases when no hints, got %v", runner.capturedTestCases)
	}
}

// TestHasCodingArchiveUsesFilteredQuery 验证 HasCodingArchive 使用服务端过滤而非全量遍历
func TestHasCodingArchiveUsesFilteredQuery(t *testing.T) {
	archive := &archiveStub{
		filteredEntries: []*ArchiveEntry{
			{SourceType: "interview_coding", InterviewID: 1},
		},
	}
	repo := &interviewRepoStub{}
	uc := NewInterviewUseCase(repo, nil, archive, nil, nil, nil, nil, &reportRepoStub{}, nil, log.DefaultLogger, 0)

	exists, err := uc.HasCodingArchive(context.Background(), 1, 9)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !exists {
		t.Fatal("expected HasCodingArchive to return true")
	}
	// 验证没有调用 ListByUser（全量遍历）
	if archive.listByUserCalls > 0 {
		t.Fatal("expected ListByUser NOT to be called (should use ListBySource)")
	}
}

// TestHasCodingArchiveReturnsFalseWhenEmpty 验证无归档时返回 false
func TestHasCodingArchiveReturnsFalseWhenEmpty(t *testing.T) {
	archive := &archiveStub{
		filteredEntries: []*ArchiveEntry{},
	}
	repo := &interviewRepoStub{}
	uc := NewInterviewUseCase(repo, nil, archive, nil, nil, nil, nil, &reportRepoStub{}, nil, log.DefaultLogger, 0)

	exists, err := uc.HasCodingArchive(context.Background(), 1, 9)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if exists {
		t.Fatal("expected HasCodingArchive to return false when empty")
	}
}

// TestResolveTopicsByIndustryGo 验证 Go 行业返回正确的主题列表
func TestResolveTopicsByIndustryGo(t *testing.T) {
	topics := resolveTopicsByIndustry("go")
	if len(topics) == 0 {
		t.Fatal("expected non-empty topics for 'go' industry")
	}
	found := false
	for _, topic := range topics {
		if strings.Contains(topic, "Go") || strings.Contains(topic, "并发") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Go-related topics, got %v", topics)
	}
}

// TestResolveTopicsByIndustryEmpty 验证空行业返回通用主题
func TestResolveTopicsByIndustryEmpty(t *testing.T) {
	topics := resolveTopicsByIndustry("")
	if len(topics) == 0 {
		t.Fatal("expected non-empty topics for empty industry")
	}
	if topics[0] != "编程基础" {
		t.Fatalf("expected generic topics, got %v", topics)
	}
}

// TestBuildQuestionAnswerPairsGroupsByIndex 验证消息按 QuestionIndex 正确配对
func TestBuildQuestionAnswerPairsGroupsByIndex(t *testing.T) {
	messages := []*InterviewMessage{
		{ID: 1, Role: "assistant", QuestionIndex: 0, Content: `{"question":"Q1","topic":"Go基础"}`},
		{ID: 2, Role: "user", QuestionIndex: 0, Content: "answer1"},
		{ID: 3, Role: "assistant", QuestionIndex: 1, Content: `{"question":"Q2","topic":"并发"}`},
		{ID: 4, Role: "user", QuestionIndex: 1, Content: "answer2"},
		{ID: 5, Role: "assistant", QuestionIndex: 2, Content: `{"question":"Q3","topic":"数据库"}`},
		{ID: 6, Role: "user", QuestionIndex: 2, Content: "answer3"},
	}

	pairs := BuildQuestionAnswerPairs(messages)
	if len(pairs) != 3 {
		t.Fatalf("expected 3 pairs, got %d", len(pairs))
	}
	for i, pair := range pairs {
		if pair.Question == nil {
			t.Fatalf("pair %d: expected non-nil question", i)
		}
		if pair.Answer == nil {
			t.Fatalf("pair %d: expected non-nil answer", i)
		}
		if pair.Index != int32(i) {
			t.Fatalf("pair %d: expected index %d, got %d", i, i, pair.Index)
		}
	}
}

// TestBuildQuestionAnswerPairsSkipsZeroIndex 验证 QuestionIndex 全为 0 时只配对一组
func TestBuildQuestionAnswerPairsSkipsZeroIndex(t *testing.T) {
	messages := []*InterviewMessage{
		{ID: 1, Role: "assistant", QuestionIndex: 0, Content: `{"question":"Q1"}`},
		{ID: 2, Role: "user", QuestionIndex: 0, Content: "a1"},
		{ID: 3, Role: "assistant", QuestionIndex: 0, Content: `{"question":"Q2"}`},
		{ID: 4, Role: "user", QuestionIndex: 0, Content: "a2"},
	}

	pairs := BuildQuestionAnswerPairs(messages)
	// 所有消息 index=0，只应配对出 1 组
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair when all index=0, got %d", len(pairs))
	}
}
