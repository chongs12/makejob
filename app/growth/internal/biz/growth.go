package biz

import (
	"context"
	"time"
)

// GrowthRepo data 层必须实现的接口
type GrowthRepo interface {
	GetUserGrowthSummary(ctx context.Context, userID uint64) (*GrowthSummary, error)
	CreateStudyLog(ctx context.Context, log *StudyLog) error
	GetStudyLogStats(ctx context.Context, userID uint64) (*GrowthSummary, error)
	GetWeeklyFocusItems(ctx context.Context, userID uint64) ([]*FocusItem, error)
}

// --- 领域实体 ---

type GrowthSummary struct {
	TotalStudyDays  int32
	TotalQuestions  int32
	TotalInterviews int32
	CurrentStreak   int32
	AvgScore        float64
	WeeklyStats     []*WeeklyStat
	WeakTopics      []*TopicWeakness
}

type WeeklyStat struct {
	Week              string
	QuestionsAnswered int32
	InterviewsTaken   int32
	AvgScore          float64
}

type TopicWeakness struct {
	Topic         string
	WeaknessScore float64
	MistakeCount  int32
}

type FocusItem struct {
	Topic      string
	Source     string
	Weight     float64
	Suggestion string
}

type StudyLog struct {
	ID        uint64
	UserID    uint64
	Action    string
	RefID     uint64
	Duration  int32
	CreatedAt time.Time
}

// GrowthUseCase 成长业务用例
type GrowthUseCase struct {
	repo GrowthRepo
}

// NewGrowthUseCase 创建成长用例
func NewGrowthUseCase(repo GrowthRepo) *GrowthUseCase {
	return &GrowthUseCase{repo: repo}
}

// GetGrowthSummary 获取用户成长摘要
func (uc *GrowthUseCase) GetGrowthSummary(ctx context.Context, userID uint64) (*GrowthSummary, error) {
	return uc.repo.GetStudyLogStats(ctx, userID)
}

// GetWeeklyFocus 获取本周学习重点
func (uc *GrowthUseCase) GetWeeklyFocus(ctx context.Context, userID uint64) ([]*FocusItem, error) {
	return uc.repo.GetWeeklyFocusItems(ctx, userID)
}

// SyncStudyLog 同步学习记录
func (uc *GrowthUseCase) SyncStudyLog(ctx context.Context, userID uint64, action string, refID uint64, duration int32) (*StudyLog, error) {
	log := &StudyLog{
		UserID:   userID,
		Action:   action,
		RefID:    refID,
		Duration: duration,
	}
	if err := uc.repo.CreateStudyLog(ctx, log); err != nil {
		return nil, err
	}
	return log, nil
}
