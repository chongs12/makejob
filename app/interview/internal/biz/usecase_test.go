package biz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// interviewRepoStub 提供面试用例测试所需的最小仓储行为。
type interviewRepoStub struct {
	inTransaction    bool
	transactionCalls int
	createCalls      int
	messageCalls     int
	nextInterviewID  uint64
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

// GetByID 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) GetByID(context.Context, uint64) (*Interview, error) {
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

// ListMessages 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) ListMessages(context.Context, uint64) ([]*InterviewMessage, error) {
	return nil, errors.New("not implemented")
}

// ListMessagesLimited 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) ListMessagesLimited(context.Context, uint64, int32) ([]*InterviewMessage, error) {
	return nil, errors.New("not implemented")
}

// CreateCodingAttempt 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) CreateCodingAttempt(context.Context, *CodingAttempt) error {
	return errors.New("not implemented")
}

// UpdateCodingAttempt 返回未实现错误，当前测试不会走到这里。
func (r *interviewRepoStub) UpdateCodingAttempt(context.Context, *CodingAttempt) error {
	return errors.New("not implemented")
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

// interviewIndustryStub 始终返回固定行业，用于通过行业校验。
type interviewIndustryStub struct{}

// GetIndustry 返回固定行业信息。
func (interviewIndustryStub) GetIndustry(context.Context, string) (*Industry, error) {
	return &Industry{Code: "backend", Name: "后端"}, nil
}

// TestInterviewUseCaseCreateInterviewCallsAIBeforeTransaction 验证首题 AI 生成发生在事务外。
func TestInterviewUseCaseCreateInterviewCallsAIBeforeTransaction(t *testing.T) {
	repo := &interviewRepoStub{nextInterviewID: 42}
	ai := &interviewAIStub{
		repo: repo,
		question: &InterviewQuestion{
			Question:   "介绍一下 Go 的 GMP 模型",
			Topic:      "goroutine",
			Difficulty: "medium",
			Type:       "text",
		},
	}
	uc := NewInterviewUseCase(repo, ai, nil, interviewIndustryStub{}, nil, nil, nil, nil, log.DefaultLogger)

	interview, firstQuestion, err := uc.CreateInterview(context.Background(), &CreateInterviewRequest{
		UserID:        9,
		IndustryCode:  "backend",
		Difficulty:    "medium",
		InterviewMode: "standard",
		QuestionCount: 5,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ai.interviewCall != 1 {
		t.Fatalf("expected ai to be called once, got %d", ai.interviewCall)
	}
	if repo.transactionCalls != 1 || repo.createCalls != 1 || repo.messageCalls != 1 {
		t.Fatalf("unexpected repo calls: tx=%d create=%d message=%d", repo.transactionCalls, repo.createCalls, repo.messageCalls)
	}
	if interview == nil || interview.ID != 42 {
		t.Fatalf("expected created interview with id=42, got %#v", interview)
	}
	if firstQuestion == nil || firstQuestion.Question == "" {
		t.Fatalf("expected first question to be returned, got %#v", firstQuestion)
	}
}

// TestInterviewUseCaseCreateInterviewSkipsWriteWhenAIFails 验证首题 AI 失败时不会写入面试记录。
func TestInterviewUseCaseCreateInterviewSkipsWriteWhenAIFails(t *testing.T) {
	repo := &interviewRepoStub{nextInterviewID: 42}
	ai := &interviewAIStub{
		repo: repo,
		err:  errors.New("ai unavailable"),
	}
	uc := NewInterviewUseCase(repo, ai, nil, interviewIndustryStub{}, nil, nil, nil, nil, log.DefaultLogger)

	_, _, err := uc.CreateInterview(context.Background(), &CreateInterviewRequest{
		UserID:        9,
		IndustryCode:  "backend",
		Difficulty:    "medium",
		InterviewMode: "standard",
		QuestionCount: 5,
	})
	if err == nil {
		t.Fatal("expected error when ai fails")
	}
	if repo.transactionCalls != 0 || repo.createCalls != 0 || repo.messageCalls != 0 {
		t.Fatalf("expected no writes on ai failure, got tx=%d create=%d message=%d", repo.transactionCalls, repo.createCalls, repo.messageCalls)
	}
}
