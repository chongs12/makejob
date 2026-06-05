package biz

import (
	"context"
	"time"
)

// QuestionRepo data 层接口
type QuestionRepo interface {
	List(ctx context.Context, filter *QuestionFilter, page, pageSize int32) ([]*Question, int64, error)
	GetByID(ctx context.Context, id uint64) (*Question, error)
}

type RecordRepo interface {
	Create(ctx context.Context, record *UserQuestionRecord) error
	GetByUserAndQuestion(ctx context.Context, userID, questionID uint64) (*UserQuestionRecord, error)
	GetCategoryStats(ctx context.Context, userID uint64) ([]*CategoryStat, error)
	GetWrongQuestions(ctx context.Context, userID uint64, page, pageSize int32) ([]*WrongQuestion, int64, error)
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
}

type CategoryRepo interface {
	ListByIndustry(ctx context.Context, industryCode string) ([]*Category, error)
}

type IndustryRepo interface {
	List(ctx context.Context) ([]*Industry, error)
}

// AI 客户端接口
type QuizAnalyzerClient interface {
	Analyze(ctx context.Context, req *QuizAnalyzerRequest) (*QuizAnalyzerResponse, error)
}

// 领域实体
type Question struct {
	ID             uint64
	Title          string
	Content        string
	Difficulty     string
	Type           string // coding, subjective, multiple_choice
	IndustryCode   string
	CategoryID     uint64
	CategoryName   string
	Tags           []string
	StarterCode    string
	Language       string
	EvaluationMode string
	ReferenceAnswer string
	Explanation    string
	CreatedAt      time.Time
}

type UserQuestionRecord struct {
	ID         uint64
	UserID     uint64
	QuestionID uint64
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
	ID       uint64
	Name     string
	ParentID uint64
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
