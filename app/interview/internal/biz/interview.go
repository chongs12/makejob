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
	// IncrementCurrentIndex 递增实时面试已回答题数（current_index），供知识点面试题数达标自动结束判定。
	IncrementCurrentIndex(ctx context.Context, interviewID uint64) error
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
	// 会话式面试接口（对齐单体 InterviewAgent）
	StartInterview(ctx context.Context, req *StartInterviewRequest) (*StartInterviewResponse, error)
	EvaluateAnswer(ctx context.Context, req *EvaluateAnswerRequest) (*EvaluateAnswerResponse, error)
	GetNextQuestionSession(ctx context.Context, req *GetNextQuestionSessionRequest) (*GetNextQuestionSessionResponse, error)
	GenerateInterviewReport(ctx context.Context, req *GenerateInterviewReportRequest) (*GenerateInterviewReportResponse, error)
	EndInterviewSession(ctx context.Context, req *EndInterviewSessionRequest) (*EndInterviewSessionResponse, error)
	// GenerateReportFromHistory 从对话历史生成报告（不依赖 session，供实时面试使用）
	GenerateReportFromHistory(ctx context.Context, req *GenerateReportFromHistoryRequest) (*GenerateInterviewReportResponse, error)
	// GenerateKnowledgeReport 知识点专项面试报告生成（基于完整对话历史）
	GenerateKnowledgeReport(ctx context.Context, req *GenerateKnowledgeReportRequest) (*GenerateKnowledgeReportResponse, error)
	// GenerateJobReport 岗位求职面试报告生成（基于完整对话历史 + 简历画像 + JD）
	GenerateJobReport(ctx context.Context, req *GenerateJobReportRequest) (*GenerateJobReportResponse, error)
}

// LearningArchiveClient 学习档案服务的 gRPC 客户端接口
type LearningArchiveClient interface {
	WriteEntry(ctx context.Context, entry *ArchiveEntry) error
	ListByUser(ctx context.Context, userID uint64, limit int32) ([]*ArchiveEntry, error)
	// ListBySource 按来源类型和面试 ID 服务端过滤，避免全量遍历
	ListBySource(ctx context.Context, userID uint64, sourceType string, interviewID uint64) ([]*ArchiveEntry, error)
}

// IndustryClient 行业服务的 gRPC 客户端接口
type IndustryClient interface {
	GetIndustry(ctx context.Context, code string) (*Industry, error)
}

// MembershipClient 会员服务的 gRPC 客户端接口（用于实时语音面试门禁校验）
type MembershipClient interface {
	CheckFeatureAccess(ctx context.Context, userID uint64, feature string) (bool, string, error)
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
	IndustryID       *uint64    // 对齐表 industry_id 外键，知识点面试无行业时为 nil（DB NULL）
	IndustryCode     string     `gorm:"-"` // 运行期字段，不落库
	Difficulty       string     `gorm:"-"` // 运行期字段，不落库
	Status           string     // created, in_progress, ongoing, report_generating, report_failed, completed
	InterviewMode    string     `gorm:"-"` // 运行期字段，不落库
	InterviewType    string     // 落库：knowledge | job，决定出题与报告模板
	KnowledgeTopics  []string   `gorm:"-"` // 运行期，落库为 knowledge_topics JSON
	Questions        []InterviewQuestion `gorm:"-"` // 运行期，预生成题目，落库为 questions_json JSON
	QuestionCount    int32      // 对应表 total_questions
	CurrentIndex     int32      // 对应表 current_index，实时面试用户每答一题递增
	OverallScore     float64    // 对应表 score
	AIFeedback       string     // 对应表 ai_feedback
	AISessionID      string     // 对应表 ai_session_id
	ReportJSON       string     // 对应表 report_json
	StartedAt        *time.Time // 对应表 started_at
	ResumeText       string     // 落库：简历原文，岗位求职报告依赖
	ResumeParsedJSON string     // 落库：简历解析画像 JSON
	JobDescription   string     // 落库：目标岗位 JD
	Live2DModelKey   string
	FinishedAt       *time.Time // 对应表 ended_at
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
	Topics        []string // 知识点专项面试的自定义知识点
	InterviewType string   // knowledge | job，决定出题 prompt 分支
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
	Emotion            string
	Action             string
	Reply              string
	ExpressionMix      []ExpressionLayer
	ParameterOverrides []ParameterOverride
	MotionKey          string
	MotionGroup        string
	MotionPriority     string
	MotionDurationMS   int
	Intensity          float64
	DurationMS         int
	MouthOpen          *float64
	Source             string
}

type ExpressionLayer struct {
	Key    string
	Weight float64
}

type ParameterOverride struct {
	ID    string
	Value float64
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
	Skills      []string
	Experience  []string
	Education   []string
	Projects    []string
	Summary     string
	Strengths   []string
	WeakSignals []string
}

// --- 会话式面试 DTO（对齐单体 InterviewAgent 接口）---

type StartInterviewRequest struct {
	InterviewID   uint64
	IndustryCode  string
	Difficulty    string
	QuestionCount int32
	ResumeText    string
	JobDescription string
	InterviewMode string
	Topics        []string // 知识点专项面试的自定义知识点
	InterviewType string   // knowledge | job，决定出题 prompt 分支
}

type StartInterviewResponse struct {
	SessionID  string
	Question   string
	Topic      string
	Difficulty string
	Type       string
	Hints      string
}

type EvaluateAnswerRequest struct {
	SessionId     string
	QuestionIndex int32
	Answer        string
	RAGContext    string // RAG 检索到的参考知识，注入到评分 prompt
}

type EvaluateAnswerResponse struct {
	Score      float64
	IsCorrect  bool
	Feedback   string
	KeyPoints  []string
	Suggestions string
	FollowUp   string
}

type GetNextQuestionSessionRequest struct {
	SessionId  string
	RAGContext string // RAG 检索到的参考知识，注入到出题 prompt
}

type GetNextQuestionSessionResponse struct {
	Question   string
	Topic      string
	Difficulty string
	Type       string
	Hints      string
	HasNext    bool
}

type GenerateInterviewReportRequest struct {
	SessionId string
}

type GenerateInterviewReportResponse struct {
	OverallScore    float64
	Summary         string
	DimensionScores map[string]float64
	Strengths       []string
	Weaknesses      []string
	Suggestions     []string
	AiFeedback      string
}

// GenerateReportFromHistoryRequest 从对话历史生成报告请求（不依赖 session）
type GenerateReportFromHistoryRequest struct {
	History        []*InterviewMessage
	IndustryCode   string
	Difficulty     string
	TotalQuestions int32
}

// GenerateKnowledgeReportRequest 知识点专项报告生成请求
type GenerateKnowledgeReportRequest struct {
	History         []*InterviewMessage
	KnowledgeTopics []string
	Difficulty      string
	TotalQuestions  int32
}

// GenerateKnowledgeReportResponse 知识点专项报告生成响应
type GenerateKnowledgeReportResponse struct {
	ReportJSON   string
	OverallScore float64
	Rating       string
}

// GenerateJobReportRequest 岗位求职报告生成请求
type GenerateJobReportRequest struct {
	History          []*InterviewMessage
	ResumeText       string
	ResumeParsedJSON string
	JobDescription   string
	IndustryCode     string
	Difficulty       string
	TotalQuestions   int32
}

// GenerateJobReportResponse 岗位求职报告生成响应
type GenerateJobReportResponse struct {
	ReportJSON         string
	OverallScore       float64
	Rating             string
	HireRecommendation string
}

type EndInterviewSessionRequest struct {
	SessionId string
}

type EndInterviewSessionResponse struct {
	Success bool
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
	ID   uint64
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
	InterviewType  string // knowledge | job
	ResumeText     string
	JobDescription string
	Live2DModelKey string
}

// InterviewReport 面试报告实体
type InterviewReport struct {
	ID                    uint64
	InterviewID           uint64
	OverallScore          float64
	ReportTemplate        string // knowledge | job | ""
	ReportDataJSON        string // 完整结构化报告 JSON，前端按 report_template 渲染
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
