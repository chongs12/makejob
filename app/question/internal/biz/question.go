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
	GetByUserAndQuestion(ctx context.Context, userID, questionID uint64) (*UserQuestionRecord, error)
	GetCategoryStats(ctx context.Context, userID uint64) ([]*CategoryStat, error)
	GetWrongQuestions(ctx context.Context, userID uint64, page, pageSize int32) ([]*WrongQuestion, int64, error)
	// GetMistakeTopics 聚合查询用户各分类的错误统计
	GetMistakeTopics(ctx context.Context, userID uint64) ([]*MistakeTopic, error)
}

type FavoriteRepo interface {
	Create(ctx context.Context, fav *UserFavorite) error
	Delete(ctx context.Context, userID, questionID uint64) error
	ListByUser(ctx context.Context, userID uint64, page, pageSize int32) ([]*Question, int64, error)
	Exists(ctx context.Context, userID, questionID uint64) (bool, error)
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
	ListByIndustry(ctx context.Context, industryCode string) ([]*Category, error)
	GetByID(ctx context.Context, id uint64) (*Category, error)
}

type IndustryRepo interface {
	List(ctx context.Context) ([]*Industry, error)
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
}

// 领域实体
type Question struct {
	ID                 uint64
	Title              string
	Content            string
	Difficulty         string
	Type               string // coding, subjective, multiple_choice
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
	QuestionID uint64
	Content    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Category struct {
	ID           uint64
	Name         string
	ParentID     uint64
	IndustryCode string
}

type Industry struct {
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
	QuestionID  uint64
	Title       string
	WrongCount  int32
	LastWrongAt time.Time
	LastAnswer  string
}

type QuestionFilter struct {
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
	Score         float64
	IsCorrect     bool
	Feedback      string
	KeyPoints     []string
	Suggestions   string
	CorrectAnswer string
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
