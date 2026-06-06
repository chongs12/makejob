package biz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
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
	repo      ArchiveRepo
	publisher MQPublisher
}

// MQPublisher MQ 消息发布接口
type MQPublisher interface {
	PublishArchiveWritten(ctx context.Context, userID uint64, source string, sourceID uint64, weakTopicsAdded, strengthTopicsAdded []string) error
}

func NewArchiveUseCase(repo ArchiveRepo, publisher MQPublisher) *ArchiveUseCase {
	return &ArchiveUseCase{repo: repo, publisher: publisher}
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

// HandleInterviewFinished 处理面试完成事件：将薄弱/优势知识点写入学习档案，并发布档案写入事件
func (uc *ArchiveUseCase) HandleInterviewFinished(ctx context.Context, interviewID, userID uint64, score float64, weakTopics, strengthTopics []string) error {
	logger := log.NewHelper(log.DefaultLogger)

	if userID == 0 {
		logger.Warn("interview.finished 事件 user_id 为 0，丢弃消息")
		return nil
	}

	now := time.Now()

	// 为每个薄弱知识点创建档案条目
	weakEntries := make([]*ArchiveEntry, 0, len(weakTopics))
	for _, topic := range weakTopics {
		weakEntries = append(weakEntries, &ArchiveEntry{
			UserID:      userID,
			SourceType:  "interview_weak",
			SourceRef:   topic,
			InterviewID: interviewID,
			MistakeTags: []string{topic},
			OccurredAt:  now,
		})
	}

	// 为每个优势知识点创建档案条目
	strengthEntries := make([]*ArchiveEntry, 0, len(strengthTopics))
	for _, topic := range strengthTopics {
		strengthEntries = append(strengthEntries, &ArchiveEntry{
			UserID:       userID,
			SourceType:   "interview_strength",
			SourceRef:    topic,
			InterviewID:  interviewID,
			StrengthTags: []string{topic},
			OccurredAt:   now,
		})
	}

	allEntries := append(weakEntries, strengthEntries...)

	// 批量写入数据库
	if len(allEntries) > 0 {
		if _, err := uc.repo.BatchCreate(ctx, allEntries); err != nil {
			return err
		}
	}

	// 发布档案写入事件（发布失败仅记录日志，不重试主流程）
	if err := uc.publisher.PublishArchiveWritten(ctx, userID, "interview", interviewID, weakTopics, strengthTopics); err != nil {
		logger.Errorf("发布 archive.written 事件失败: %v", err)
	}

	return nil
}
