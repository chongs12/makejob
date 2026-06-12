package biz

import (
	"context"
	"time"
)

// InterviewRepo data 层必须实现的接口（接口隔离原则）
type InterviewRepo interface {
	Create(ctx context.Context, interview *Interview) error
	GetByID(ctx context.Context, id uint64) (*Interview, error)
	ListByUser(ctx context.Context, userID uint64, page, pageSize int32) ([]*Interview, int64, error)
	Update(ctx context.Context, interview *Interview) error
	CreateMessage(ctx context.Context, msg *InterviewMessage) error
	ListMessages(ctx context.Context, interviewID uint64) ([]*InterviewMessage, error)
	ListMessagesLimited(ctx context.Context, interviewID uint64, limit int32) ([]*InterviewMessage, error)
	CreateCodingAttempt(ctx context.Context, attempt *CodingAttempt) error
	UpdateCodingAttempt(ctx context.Context, attempt *CodingAttempt) error
	ListCodingAttempts(ctx context.Context, interviewID uint64) ([]*CodingAttempt, error)
	BindRealtimeDialog(ctx context.Context, interviewID uint64, dialogID string) error
	AppendMessageAndBumpIndex(ctx context.Context, msg *InterviewMessage) error
	// Transaction 在事务中执行操作（FIX I1）
	Transaction(ctx context.Context, fn func(txCtx context.Context) error) error
	// GetStats SQL 聚合查询面试统计（FIX I3）
	GetStats(ctx context.Context, userID uint64) (*InterviewStats, error)
	// GetAdminStats SQL 聚合查询全站面试总量，供管理后台使用。
	GetAdminStats(ctx context.Context) (int64, error)
}

// AIServiceClient AI 服务的 gRPC 客户端接口
type AIServiceClient interface {
	InterviewAgent(ctx context.Context, req *InterviewAgentRequest) (*InterviewAgentResponse, error)
	QuizAnalyzer(ctx context.Context, req *QuizAnalyzerRequest) (*QuizAnalyzerResponse, error)
	ResumeParser(ctx context.Context, req *ResumeParserRequest) (*ResumeParserResponse, error)
}

// LearningArchiveClient 学习档案服务的 gRPC 客户端接口
type LearningArchiveClient interface {
	WriteEntry(ctx context.Context, entry *ArchiveEntry) error
	ListByUser(ctx context.Context, userID uint64, limit int32) ([]*ArchiveEntry, error)
}

// IndustryClient 行业服务的 gRPC 客户端接口
type IndustryClient interface {
	GetIndustry(ctx context.Context, code string) (*Industry, error)
}

// RAGClient RAG 检索服务的 gRPC 客户端接口
type RAGClient interface {
	Retrieve(ctx context.Context, query string, topK int32) ([]*RAGDocument, error)
}

// CodeRunnerClient 代码执行服务客户端接口
type CodeRunnerClient interface {
	Execute(ctx context.Context, language, code string, testCases []CodeTestCase) (*CodeRunnerResult, error)
}

// CodeTestCase 代码执行测试用例
type CodeTestCase struct {
	Input          string
	ExpectedOutput string
}

// CodeRunnerResult 代码执行结果
type CodeRunnerResult struct {
	Success         bool
	Stdout          string
	Stderr          string
	PassedCount     int32
	TotalCount      int32
	ExecutionTimeMs int64
}

// RAGDocument RAG 检索返回的文档
type RAGDocument struct {
	ID      string
	Content string
	Score   float64
}

// --- 领域实体 ---

type Interview struct {
	ID               uint64
	UserID           uint64
	IndustryCode     string
	Difficulty       string
	Status           string // created, in_progress, ongoing, report_generating, report_failed, completed
	InterviewMode    string // standard, realtime_voice, coding
	QuestionCount    int32
	CurrentIndex     int32
	OverallScore     float64
	ResumeText       string
	ResumeParsedJSON string // AI 解析简历后的结构化 JSON
	JobDescription   string
	Live2DModelKey   string
	RealtimeDialogID string
	FinishedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type InterviewMessage struct {
	ID            uint64
	InterviewID   uint64
	Role          string // user, assistant
	Content       string
	MessageType   string // text, code, audio
	QuestionIndex int32
	CreatedAt     time.Time
}

type CodingAttempt struct {
	ID              uint64
	InterviewID     uint64
	QuestionIndex   int32
	Language        string
	Code            string
	Passed          bool
	TestCasesPassed int32
	TotalTestCases  int32
	Output          string
	ErrorMsg        string
	AIScore         float64
	AIFeedback      string
	CreatedAt       time.Time
}

// --- AI 请求/响应 DTO ---

type InterviewAgentRequest struct {
	InterviewID   uint64
	IndustryCode  string
	Difficulty    string
	History       []*InterviewMessage
	UserAnswer    string
	QuestionIndex int32
	ResumeText    string
	JobDesc       string
	Mode          string // "question", "report", "evaluate"
}

type InterviewAgentResponse struct {
	Question        *InterviewQuestion
	Feedback        *AnswerFeedback
	ShouldEnd       bool
	Live2DDirective *Live2DDirective
}

type InterviewQuestion struct {
	Question        string
	Topic           string
	Difficulty      string
	Type            string
	Hints           string
	Language        string
	StarterCode     string
	EditorMode      string
	EvalMode        string
	Live2DDirective *Live2DDirective
}

type AnswerFeedback struct {
	Score       float64
	IsCorrect   bool
	Feedback    string
	KeyPoints   []string
	Suggestions string
	FollowUp    string
}

type Live2DDirective struct {
	Emotion string
	Action  string
}

type QuizAnalyzerRequest struct {
	Question   string
	Answer     string
	Topic      string
	Difficulty string
}

type QuizAnalyzerResponse struct {
	Score         float64
	IsCorrect     bool
	Feedback      string
	KeyPoints     []string
	Suggestions   string
	CorrectAnswer string
}

type ResumeParserRequest struct {
	ResumeText string
}

type ResumeParserResponse struct {
	Skills     []string
	Experience []string
	Education  []string
}

type ArchiveEntry struct {
	ID              uint64
	UserID          uint64
	SourceType      string
	SourceRef       string
	InterviewID     uint64
	QuestionIndex   int32
	IndustryCode    string
	Language        string
	MistakeTags     []string
	StrengthTags    []string
	Suggestions     []string
	EvidenceSummary string
	OccurredAt      time.Time
	CreatedAt       time.Time
}

type Industry struct {
	Code string
	Name string
}

type InterviewStats struct {
	TotalInterviews        int32
	AvgScore               float64
	TotalQuestionsAnswered int32
	AvgAccuracy            float64
	CompletedInterviews    int32
	TodayCount             int32
}

type CreateInterviewRequest struct {
	UserID         uint64
	IndustryCode   string
	Difficulty     string
	Topics         []string
	QuestionCount  int32
	InterviewMode  string
	ResumeText     string
	JobDescription string
	Live2DModelKey string
}

// InterviewReport 面试报告实体
type InterviewReport struct {
	ID                    uint64
	InterviewID           uint64
	OverallScore          float64
	DimensionScoresJSON   string
	StrengthsJSON         string
	WeaknessesJSON        string
	SuggestionsJSON       string
	Summary               string
	CodingDiagnosticsJSON string
	CreatedAt             time.Time
}

// ReportRepo 面试报告仓库接口
type ReportRepo interface {
	Create(ctx context.Context, report *InterviewReport) error
	GetByInterviewID(ctx context.Context, interviewID uint64) (*InterviewReport, error)
}

// MQPublisher MQ 消息发布接口
type MQPublisher interface {
	PublishInterviewResumeParse(ctx context.Context, interviewID, userID uint64, resumeText string) error
	PublishInterviewReportGenerate(ctx context.Context, interviewID, userID uint64) error
	PublishInterviewFinished(ctx context.Context, interviewID, userID uint64, score float64, weakTopics, strengthTopics []string) error
}
