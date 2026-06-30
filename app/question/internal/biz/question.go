package biz

import (
	"context"
	"time"
)

// QuestionRepo data 层接口
type QuestionRepo interface {
	List(ctx context.Context, filter *QuestionFilter, page, pageSize int32) ([]*Question, int64, error)
	GetByID(ctx context.Context, id uint64) (*Question, error)
	Create(ctx context.Context, question *Question) error
	Update(ctx context.Context, question *Question) error
	Delete(ctx context.Context, id uint64) error
	Count(ctx context.Context, filter *QuestionFilter) (int64, error)
	RandomSelect(ctx context.Context, filter *QuestionFilter, count int32) ([]*Question, error)
	// ExistsByTitleAndIndustry 检查题目是否已存在（FIX Q3: 幂等去重）
	ExistsByTitleAndIndustry(ctx context.Context, title, industryCode string) (bool, error)
}

type RecordRepo interface {
	Create(ctx context.Context, record *UserQuestionRecord) error
	// Upsert 按 user_id + question_id 去重，同一题只保留最新答题记录
	Upsert(ctx context.Context, record *UserQuestionRecord) error
	GetByUserAndQuestion(ctx context.Context, userID, questionID uint64) (*UserQuestionRecord, error)
	GetCategoryStats(ctx context.Context, userID uint64) ([]*CategoryStat, error)
	GetWrongQuestions(ctx context.Context, userID uint64, page, pageSize int32) ([]*WrongQuestion, int64, error)
	// GetMistakeTopics 聚合查询用户各分类的错误统计
	GetMistakeTopics(ctx context.Context, userID uint64) ([]*MistakeTopic, error)
	// GetTodayCount 查询用户今天练习的题目数量
	GetTodayCount(ctx context.Context, userID uint64) (int32, error)
	// GetAnsweredQuestionIDs 批量查询用户已答题的题目 ID 集合
	GetAnsweredQuestionIDs(ctx context.Context, userID uint64, questionIDs []uint64) (map[uint64]bool, error)
}

type FavoriteRepo interface {
	Create(ctx context.Context, fav *UserFavorite) error
	Delete(ctx context.Context, userID, questionID uint64) error
	ListByUser(ctx context.Context, userID uint64, page, pageSize int32) ([]*Question, int64, error)
	Exists(ctx context.Context, userID, questionID uint64) (bool, error)
	// GetFavoritedQuestionIDs 批量查询用户已收藏的题目 ID 集合
	GetFavoritedQuestionIDs(ctx context.Context, userID uint64, questionIDs []uint64) (map[uint64]bool, error)
}

type NoteRepo interface {
	Create(ctx context.Context, note *UserNote) error
	Update(ctx context.Context, note *UserNote) error
	ListByUser(ctx context.Context, userID uint64, questionID uint64, page, pageSize int32) ([]*UserNote, int64, error)
	// GetByID 按 ID 获取笔记
	GetByID(ctx context.Context, id uint64) (*UserNote, error)
	// Delete 删除指定笔记（需校验归属）
	Delete(ctx context.Context, id, userID uint64) error
}

type CategoryRepo interface {
	ListByIndustry(ctx context.Context, industryID uint64) ([]*Category, error)
	GetByID(ctx context.Context, id uint64) (*Category, error)
}

type IndustryRepo interface {
	List(ctx context.Context) ([]*Industry, error)
	GetByCode(ctx context.Context, code string) (*Industry, error)
	GetByID(ctx context.Context, id uint64) (*Industry, error)
}

// LearningArchiveRepo 学习档案条目仓储接口（已废弃，由 LearningArchiveClient 替代）
// 保留用于向后兼容，新代码应使用 LearningArchiveClient。
type LearningArchiveRepo interface {
	Upsert(ctx context.Context, entry *LearningArchiveEntry) error
	ListRecentByUser(ctx context.Context, userID uint64, limit int, interviewID *uint64) ([]*LearningArchiveEntry, error)
}

// LearningArchiveClient 学习档案 gRPC 客户端接口，替代本地 LearningArchiveRepo。
type LearningArchiveClient interface {
	WriteEntry(ctx context.Context, entry *LearningArchiveEntry) error
	GetFocusSignals(ctx context.Context, userID uint64, limit int32) ([]FocusSignalData, error)
	GetMistakeTopic(ctx context.Context, code string) (*MistakeTopicCard, bool)
}

// FocusSignalData 从 learning_archive 服务获取的焦点信号数据。
type FocusSignalData struct {
	Tag                       string
	TopicCode                 string
	TopicTitle                string
	TopicProblemPattern       string
	RelatedQuestionSets       []string
	RecommendedActions        []string
	PrimaryQuestionSet        string
	OccurrenceCount           int
	ArchiveOccurrenceCount    int
	InterviewOccurrenceCount  int
	DominantArchivePhase      string
	DominantArchivePhaseLabel string
	Source                    string
	SourceLabel               string
	Reason                    string
}

// QuizAnalyzerClient AI 答案分析客户端接口
type QuizAnalyzerClient interface {
	Analyze(ctx context.Context, req *QuizAnalyzerRequest) (*QuizAnalyzerResponse, error)
}

// CodeRunnerClient 代码运行服务客户端接口
type CodeRunnerClient interface {
	Execute(ctx context.Context, req *CodeRunnerRequest) (*CodeRunnerResponse, error)
}

// QuestionGeneratorClient AI 题目生成客户端接口
type QuestionGeneratorClient interface {
	GenerateQuestions(ctx context.Context, req *GenerateQuestionsRequest) ([]*Question, error)
}

// GenerateQuestionsRequest AI 题目生成请求
type GenerateQuestionsRequest struct {
	IndustryCode     string
	Requirement      string
	AgentPrompt      string
	GenerationMode   string
	CandidateCount   int32
	IncludeScraped   bool
	IncludeGenerated bool
	Sources          []string
}

// ExamRepo 考试记录仓储接口
type ExamRepo interface {
	Create(ctx context.Context, exam *Exam) error
	GetByID(ctx context.Context, id uint64) (*Exam, error)
	Update(ctx context.Context, exam *Exam) error
}

// QuestionSetRepo 题集仓储接口
type QuestionSetRepo interface {
	List(ctx context.Context, industryCode string, page, pageSize int32) ([]*QuestionSet, int64, error)
	GetByID(ctx context.Context, id uint64) (*QuestionSet, error)
	GetQuestions(ctx context.Context, setID uint64) ([]*Question, error)
	// 管理后台 CRUD
	Create(ctx context.Context, set *QuestionSet) error
	Update(ctx context.Context, set *QuestionSet) error
	Delete(ctx context.Context, id uint64) error
	AddQuestions(ctx context.Context, setID uint64, questionIDs []uint64) (int32, error)
	RemoveQuestions(ctx context.Context, setID uint64, questionIDs []uint64) (int32, error)
	GetQuestionIDs(ctx context.Context, setID uint64) ([]uint64, error)
}

// 领域实体
type Question struct {
	ID                 uint64
	Title              string
	Content            string
	Difficulty         string
	Type               string // 对齐单体：choice/multi/code/subjective
	IndustryID         uint64
	IndustryCode       string
	IndustryName       string
	CategoryID         uint64
	CategoryName       string
	Tags               []string
	OptionsJSON        string
	Answer             string
	SolutionJSON       string
	JudgeConfigJSON    string
	AnswerTemplateJSON string
	StarterCode        string
	Language           string
	EvaluationMode     string
	ReferenceAnswer    string
	Explanation        string
	TestCasesJSON      string
	JudgeConfig        *JudgeConfig `gorm:"-"` // 解析后的判题配置（不持久化）
	IsActive           bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type UserQuestionRecord struct {
	ID         uint64
	UserID     uint64
	QuestionID uint64
	ExamID     uint64
	IsCorrect  bool
	Answer     string
	Language   string
	Score      float64
	CreatedAt  time.Time
}

type UserFavorite struct {
	ID         uint64
	UserID     uint64
	QuestionID uint64
	CreatedAt  time.Time
}

type UserNote struct {
	ID         uint64
	UserID     uint64
	QuestionID *uint64 // 对齐单体：可空（全局笔记）
	Title      string
	Content    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Category struct {
	ID           uint64
	Name         string
	ParentID     uint64
	IndustryID   uint64
	IndustryCode string
}

type Industry struct {
	ID   uint64
	Code string
	Name string
	Icon string
}

type CategoryStat struct {
	CategoryName string
	Answered     int32
	Correct      int32
	Accuracy     float64
}

type WrongQuestion struct {
	QuestionID   uint64
	Title        string
	Difficulty   string
	Type         string
	CategoryName string
	CategoryID   uint64
	WrongCount   int32
	LastWrongAt  time.Time
	LastAnswer   string
}

// QuestionFilter 定义题目查询与随机选题的统一筛选条件。
type QuestionFilter struct {
	IndustryID   uint64
	IndustryCode string
	CategoryID   uint64
	Difficulty   string
	Keyword      string
}

type QuizAnalyzerRequest struct {
	Question   string
	Answer     string
	Topic      string
	Difficulty string
}

type QuizAnalyzerResponse struct {
	Score          float64
	IsCorrect      bool
	Feedback       string
	KeyPoints      []string
	Suggestions    string
	CorrectAnswer  string
	EvaluationMode string
	JudgeSummary   *JudgeSummary
}

// JudgeSummary 编程题判题摘要
type JudgeSummary struct {
	AllPassed   bool
	TotalCases  int32
	PassedCases int32
	Results     []JudgeCaseResult
}

// JudgeCaseResult 单个判题用例结果
type JudgeCaseResult struct {
	Input          string
	ExpectedOutput string
	ActualOutput   string
	Passed         bool
	Description    string
}

// CodeRunnerRequest 代码运行请求
type CodeRunnerRequest struct {
	Language  string
	Code      string
	TestCases []CodeTestCase
	TimeoutMs int32
}

// CodeTestCase 代码测试用例
type CodeTestCase struct {
	Input          string
	ExpectedOutput string
}

// CodeRunnerResponse 代码运行结果
type CodeRunnerResponse struct {
	Success         bool
	Output          string
	Error           string
	TestCasesPassed int32
	TotalTestCases  int32
	ExecutionTimeMs int64
	TestResults     []CodeTestResult
}

// CodeTestResult 单个测试用例结果
type CodeTestResult struct {
	Input          string
	ExpectedOutput string
	ActualOutput   string
	Passed         bool
}

// Exam 考试实体
type Exam struct {
	ID           uint64
	UserID       uint64
	IndustryCode string
	QuestionIDs  []uint64
	TimeLimitMin int32
	Status       string // pending, in_progress, completed
	TotalScore   float64
	StartTime    time.Time
	EndTime      time.Time
	CreatedAt    time.Time
}

// ExamResult 考试结果
type ExamResult struct {
	ExamID          uint64
	TotalScore      float64
	MaxScore        float64
	CorrectCount    int32
	TotalQuestions  int32
	Feedback        string
	QuestionResults []*QuestionResult
}

// QuestionResult 单题考试结果
type QuestionResult struct {
	QuestionID    uint64
	IsCorrect     bool
	Score         float64
	Feedback      string
	CorrectAnswer string
}

// QuestionSet 题集实体
type QuestionSet struct {
	ID            uint64
	Name          string
	Description   string
	IndustryCode  string
	CoverImage    string
	QuestionCount int32
	CreatedAt     time.Time
}

// MistakeTopic 错题知识点聚合
type MistakeTopic struct {
	CategoryID   uint64
	CategoryName string
	WrongCount   int32
	TotalCount   int32
	Accuracy     float64
}

// 学习档案来源类型常量
const (
	LearningArchiveSourceInterviewCoding  = "interview_coding"
	LearningArchiveSourcePracticeQuestion = "practice_question"
	LearningArchiveSourcePlanTaskFeedback = "plan_task_feedback"
)

// 学习阶段常量
const (
	LearningPhaseFoundation = "foundation"
	LearningPhaseDrill      = "drill"
	LearningPhaseReview     = "review"
	LearningPhaseMock       = "mock"
)

// LearningArchiveEntry 学习档案条目领域实体
type LearningArchiveEntry struct {
	ID               uint64
	UserID           uint64
	SourceType       string
	SourceRef        string
	InterviewID      uint64
	QuestionIndex    int
	IndustryCode     string
	TaskPhase        string
	TaskPhaseGoal    string
	Language         string
	MistakeTagsJSON  string
	StrengthTagsJSON string
	SuggestionsJSON  string
	EvidenceSummary  string
	OccurredAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
