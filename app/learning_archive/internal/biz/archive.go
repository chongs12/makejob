package biz

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	// ArchiveSourceTypeInterviewFinishedMarker 用于标记 interview.finished 事件已完成处理。
	ArchiveSourceTypeInterviewFinishedMarker = "interview_finished_marker"
	archiveSourceRefProcessed                = "done"
)

// ArchiveRepo data 层接口
type ArchiveRepo interface {
	Create(ctx context.Context, entry *ArchiveEntry) error
	BatchCreate(ctx context.Context, entries []*ArchiveEntry) (int, error)
	ListByUser(ctx context.Context, userID uint64, limit int32) ([]*ArchiveEntry, error)
	GetWeakTopics(ctx context.Context, userID uint64) ([]string, error)
	GetFocusSignals(ctx context.Context, userID uint64) ([]*FocusSignal, error)
	Transaction(ctx context.Context, fn func(txCtx context.Context) error) error
	GetBySource(ctx context.Context, userID, interviewID uint64, sourceType, sourceRef string) (*ArchiveEntry, error)
	HasInterviewFinishedMarker(ctx context.Context, interviewID, userID uint64) (bool, error)
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
	if entry == nil {
		return nil
	}
	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = time.Now()
	}
	existing, err := uc.findExistingEntry(ctx, entry)
	if err != nil {
		return err
	}
	if existing != nil {
		*entry = *existing
		return nil
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
	exists, err := uc.repo.HasInterviewFinishedMarker(ctx, interviewID, userID)
	if err != nil {
		return err
	}
	if exists {
		logger.Infof("面试完成事件已处理，跳过重复消费 interview_id=%d user_id=%d", interviewID, userID)
		return nil
	}

	now := time.Now()
	allEntries := []*ArchiveEntry{{
		UserID:      userID,
		SourceType:  ArchiveSourceTypeInterviewFinishedMarker,
		SourceRef:   archiveSourceRefProcessed,
		InterviewID: interviewID,
		OccurredAt:  now,
	}}

	// 为每个薄弱知识点创建档案条目
	for _, topic := range weakTopics {
		allEntries = append(allEntries, &ArchiveEntry{
			UserID:      userID,
			SourceType:  "interview_weak",
			SourceRef:   topic,
			InterviewID: interviewID,
			MistakeTags: []string{topic},
			OccurredAt:  now,
		})
	}

	// 为每个优势知识点创建档案条目
	for _, topic := range strengthTopics {
		allEntries = append(allEntries, &ArchiveEntry{
			UserID:       userID,
			SourceType:   "interview_strength",
			SourceRef:    topic,
			InterviewID:  interviewID,
			StrengthTags: []string{topic},
			OccurredAt:   now,
		})
	}

	// 批量写入数据库，并将事务边界限定在 DB 侧；后续事件发布失败不回滚已提交的档案。
	if err := uc.repo.Transaction(ctx, func(txCtx context.Context) error {
		_, err := uc.repo.BatchCreate(txCtx, allEntries)
		return err
	}); err != nil {
		return err
	}

	if len(weakTopics) == 0 && len(strengthTopics) == 0 {
		return nil
	}

	// 发布档案写入事件（发布失败仅记录日志，不重试主流程）
	if uc.publisher != nil {
		if err := uc.publisher.PublishArchiveWritten(ctx, userID, "interview", interviewID, weakTopics, strengthTopics); err != nil {
			logger.Errorf("发布 archive.written 事件失败: %v", err)
		}
	}

	return nil
}

// HasInterviewFinishedArchive 判断 interview.finished 事件是否已经生成过学习档案。
func (uc *ArchiveUseCase) HasInterviewFinishedArchive(ctx context.Context, interviewID, userID uint64) (bool, error) {
	return uc.repo.HasInterviewFinishedMarker(ctx, interviewID, userID)
}

// findExistingEntry 按幂等键查找已存在条目，命中时返回历史记录本身。
func (uc *ArchiveUseCase) findExistingEntry(ctx context.Context, entry *ArchiveEntry) (*ArchiveEntry, error) {
	if entry == nil {
		return nil, nil
	}
	sourceType := strings.TrimSpace(entry.SourceType)
	sourceRef := strings.TrimSpace(entry.SourceRef)
	switch sourceType {
	case "interview_coding":
		if sourceRef == "" && entry.InterviewID > 0 {
			sourceRef = formatUint(entry.InterviewID)
			entry.SourceRef = sourceRef
		}
		return uc.repo.GetBySource(ctx, entry.UserID, entry.InterviewID, sourceType, sourceRef)
	case "interview_weak", "interview_strength":
		if sourceRef == "" {
			return nil, nil
		}
		return uc.repo.GetBySource(ctx, entry.UserID, entry.InterviewID, sourceType, sourceRef)
	default:
		return nil, nil
	}
}

// formatUint 将 uint64 转成稳定字符串，用于构造幂等 source_ref。
func formatUint(value uint64) string {
	return strconv.FormatUint(value, 10)
}
