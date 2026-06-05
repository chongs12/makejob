package biz

import (
	"context"
	"time"
)

// ArchiveRepo data 层接口
type ArchiveRepo interface {
	Create(ctx context.Context, entry *ArchiveEntry) error
	BatchCreate(ctx context.Context, entries []*ArchiveEntry) (int, error)
	ListByUser(ctx context.Context, userID uint64, limit int32) ([]*ArchiveEntry, error)
	GetWeakTopics(ctx context.Context, userID uint64) ([]string, error)
	GetFocusSignals(ctx context.Context, userID uint64) ([]*FocusSignal, error)
}

// ArchiveEntry 学习档案条目
type ArchiveEntry struct {
	ID              uint64
	UserID          uint64
	SourceType      string
	SourceRef       string
	InterviewID     uint64
	QuestionIndex   int32
	IndustryCode    string
	PlanPhase       string
	PlanPhaseGoal   string
	Language        string
	MistakeTags     []string
	StrengthTags    []string
	Suggestions     []string
	EvidenceSummary string
	OccurredAt      time.Time
	CreatedAt       time.Time
}

// FocusSignal 聚焦信号
type FocusSignal struct {
	Topic  string
	Weight float64
	Source string
}

// ArchiveUseCase 学习档案业务用例
type ArchiveUseCase struct {
	repo ArchiveRepo
}

func NewArchiveUseCase(repo ArchiveRepo) *ArchiveUseCase {
	return &ArchiveUseCase{repo: repo}
}

// WriteEntry 写入学习档案条目
func (uc *ArchiveUseCase) WriteEntry(ctx context.Context, entry *ArchiveEntry) error {
	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = time.Now()
	}
	return uc.repo.Create(ctx, entry)
}

// BatchWriteEntries 批量写入
func (uc *ArchiveUseCase) BatchWriteEntries(ctx context.Context, entries []*ArchiveEntry) (int, error) {
	for _, e := range entries {
		if e.OccurredAt.IsZero() {
			e.OccurredAt = time.Now()
		}
	}
	return uc.repo.BatchCreate(ctx, entries)
}

// ListByUser 获取用户学习档案
func (uc *ArchiveUseCase) ListByUser(ctx context.Context, userID uint64, limit int32) ([]*ArchiveEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	return uc.repo.ListByUser(ctx, userID, limit)
}

// GetWeakTopics 获取用户薄弱知识点
func (uc *ArchiveUseCase) GetWeakTopics(ctx context.Context, userID uint64) ([]string, error) {
	return uc.repo.GetWeakTopics(ctx, userID)
}

// GetFocusSignals 获取用户聚焦信号
func (uc *ArchiveUseCase) GetFocusSignals(ctx context.Context, userID uint64) ([]*FocusSignal, error) {
	return uc.repo.GetFocusSignals(ctx, userID)
}
