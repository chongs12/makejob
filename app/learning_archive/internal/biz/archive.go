package biz

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	// ArchiveSourceTypeInterviewFinishedMarker 用于标记 interview.finished 事件已完成处理。
	ArchiveSourceTypeInterviewFinishedMarker = "interview_finished_marker"
	// ArchiveSourceTypeInterviewCoding 编程题错因归档
	ArchiveSourceTypeInterviewCoding = "interview_coding"
	archiveSourceRefProcessed                = "done"
)

// InterviewFinishedContext 面试完成事件上下文，携带面试快照数据。
type InterviewFinishedContext struct {
	InterviewID       uint64
	UserID            uint64
	Score             float64
	InterviewType     string
	WeakTopics        []string
	StrengthTopics    []string
	CodingMistakeTags []string
	Summary           string
	DurationSeconds   int32
}

// ArchiveRepo data 层接口
type ArchiveRepo interface {
	Create(ctx context.Context, entry *ArchiveEntry) error
	BatchCreate(ctx context.Context, entries []*ArchiveEntry) (int, error)
	ListByUser(ctx context.Context, userID uint64, limit int32) ([]*ArchiveEntry, error)
	ListRecentByUser(ctx context.Context, userID uint64, limit int32, interviewID *uint64) ([]*ArchiveEntry, error)
	GetWeakTopics(ctx context.Context, userID uint64, limit int32) ([]string, error)
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
	EntryPhase      string
	TaskPhase       string
	TaskPhaseGoal   string
	Language        string
	MistakeTags     []string
	StrengthTags    []string
	Suggestions     []string
	EvidenceSummary string
	OccurredAt      time.Time
	CreatedAt       time.Time
}

// ArchiveUseCase 学习档案业务用例
type ArchiveUseCase struct {
	repo      ArchiveRepo
	publisher MQPublisher
	logger    *log.Helper
}

// MQPublisher MQ 消息发布接口
type MQPublisher interface {
	PublishArchiveWritten(ctx context.Context, userID uint64, source string, sourceID uint64, weakTopicsAdded, strengthTopicsAdded []string) error
}

func NewArchiveUseCase(repo ArchiveRepo, publisher MQPublisher) *ArchiveUseCase {
	return &ArchiveUseCase{
		repo:      repo,
		publisher: publisher,
		logger:    log.NewHelper(log.DefaultLogger),
	}
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

// GetWeakTopics 获取用户薄弱知识点（支持 limit 参数），并过滤无训练价值的兜底标签（如"综合能力"），与焦点信号保持一致。
func (uc *ArchiveUseCase) GetWeakTopics(ctx context.Context, userID uint64, limit int32) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}
	topics, err := uc.repo.GetWeakTopics(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	filtered := topics[:0]
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" || isNonActionableFocusTag(topic) {
			continue
		}
		filtered = append(filtered, topic)
	}
	return filtered, nil
}

// GetFocusSignals 获取用户聚焦信号（合并档案 + 面试归档两路数据源，多级排序，水合专题卡片）
func (uc *ArchiveUseCase) GetFocusSignals(ctx context.Context, userID uint64, limit int32, industryCode string) ([]*TrainingFocusSignal, *GrowthTrendSummary, error) {
	if limit <= 0 {
		limit = defaultFocusSignalLimit
	}

	entries, err := uc.repo.ListRecentByUser(ctx, userID, 200, nil)
	if err != nil {
		return nil, nil, err
	}

	if industryCode != "" {
		filtered := make([]*ArchiveEntry, 0, len(entries))
		for _, e := range entries {
			if strings.TrimSpace(e.IndustryCode) == industryCode {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	signals := BuildTrainingFocusSignals(entries, int(limit))

	ptrs := make([]*TrainingFocusSignal, len(signals))
	for i := range signals {
		ptrs[i] = &signals[i]
	}

	var trendSummary *GrowthTrendSummary
	if len(signals) > 0 {
		top := signals[0]
		trendSummary = &GrowthTrendSummary{
			DominantSource:      top.Source,
			DominantSourceLabel: top.SourceLabel,
			TopFocusTag:         top.Tag,
			TopTopicCode:        top.TopicCode,
			TopTopicTitle:       top.TopicTitle,
			Summary:             top.Reason,
		}
	}

	return ptrs, trendSummary, nil
}

// GetPracticeRecommendations 基于焦点信号为用户推荐练习题目关键词
func (uc *ArchiveUseCase) GetPracticeRecommendations(ctx context.Context, userID uint64, limit int32, interviewID *uint64) ([]PracticeRecommendation, error) {
	if limit <= 0 {
		limit = defaultFocusSignalLimit
	}

	entries, err := uc.repo.ListRecentByUser(ctx, userID, 200, interviewID)
	if err != nil {
		return nil, err
	}

	signals := BuildTrainingFocusSignals(entries, int(limit))

	recs := make([]PracticeRecommendation, 0, len(signals))
	for _, sig := range signals {
		keywords := buildRecommendationKeywords(sig)
		recs = append(recs, PracticeRecommendation{
			FocusTag:        sig.Tag,
			TopicCode:       sig.TopicCode,
			TopicTitle:      sig.TopicTitle,
			Keywords:        keywords,
			OccurrenceCount: sig.OccurrenceCount,
			Reason:          sig.Reason,
		})
	}
	return recs, nil
}

// ListMistakeTopics 返回全部内置错因专题概要
func (uc *ArchiveUseCase) ListMistakeTopics() []MistakeTopicSummary {
	catalog := BuildMistakeTopicCatalog()
	result := make([]MistakeTopicSummary, len(catalog))
	for i, t := range catalog {
		result[i] = MistakeTopicSummary{
			Code:           t.Code,
			Tag:            t.Tag,
			Title:          t.Title,
			ProblemPattern: t.ProblemPattern,
		}
	}
	return result
}

// GetMistakeTopic 按编码查询单个错因专题详情
func (uc *ArchiveUseCase) GetMistakeTopic(code string) (*MistakeTopicCard, bool) {
	return ResolveMistakeTopicByCode(code)
}

// HandleInterviewFinished 处理面试完成事件：将面试快照、薄弱/优势知识点、编程错因写入学习档案，并发布档案写入事件
func (uc *ArchiveUseCase) HandleInterviewFinished(ctx context.Context, payload InterviewFinishedContext) error {
	if payload.UserID == 0 {
		uc.logger.Warn("interview.finished 事件 user_id 为 0，丢弃消息")
		return nil
	}
	exists, err := uc.repo.HasInterviewFinishedMarker(ctx, payload.InterviewID, payload.UserID)
	if err != nil {
		return err
	}
	if exists {
		uc.logger.Infof("面试完成事件已处理，跳过重复消费 interview_id=%d user_id=%d", payload.InterviewID, payload.UserID)
		return nil
	}

	now := time.Now()
	// 标记条目同时携带面试快照数据（摘要存 EvidenceSummary，分数/类型/时长存 Suggestions）
	snapshotMeta := []string{}
	if payload.Score > 0 {
		snapshotMeta = append(snapshotMeta, fmt.Sprintf("score:%.1f", payload.Score))
	}
	if payload.InterviewType != "" {
		snapshotMeta = append(snapshotMeta, fmt.Sprintf("type:%s", payload.InterviewType))
	}
	if payload.DurationSeconds > 0 {
		snapshotMeta = append(snapshotMeta, fmt.Sprintf("duration:%d", payload.DurationSeconds))
	}

	allEntries := []*ArchiveEntry{{
		UserID:          payload.UserID,
		SourceType:      ArchiveSourceTypeInterviewFinishedMarker,
		SourceRef:       archiveSourceRefProcessed,
		InterviewID:     payload.InterviewID,
		EvidenceSummary: payload.Summary,
		Suggestions:     snapshotMeta,
		OccurredAt:      now,
	}}

	// 薄弱知识点条目
	for _, topic := range payload.WeakTopics {
		allEntries = append(allEntries, &ArchiveEntry{
			UserID:      payload.UserID,
			SourceType:  "interview_weak",
			SourceRef:   topic,
			InterviewID: payload.InterviewID,
			MistakeTags: []string{topic},
			TaskPhase:   "mock",
			OccurredAt:  now,
		})
	}

	// 优势知识点条目
	for _, topic := range payload.StrengthTopics {
		allEntries = append(allEntries, &ArchiveEntry{
			UserID:       payload.UserID,
			SourceType:   "interview_strength",
			SourceRef:    topic,
			InterviewID:  payload.InterviewID,
			StrengthTags: []string{topic},
			TaskPhase:    "mock",
			OccurredAt:   now,
		})
	}

	// 编程错因标签条目
	for _, tag := range payload.CodingMistakeTags {
		allEntries = append(allEntries, &ArchiveEntry{
			UserID:      payload.UserID,
			SourceType:  ArchiveSourceTypeInterviewCoding,
			SourceRef:   tag,
			InterviewID: payload.InterviewID,
			MistakeTags: []string{tag},
			TaskPhase:   "mock",
			OccurredAt:  now,
		})
	}

	if err := uc.repo.Transaction(ctx, func(txCtx context.Context) error {
		_, err := uc.repo.BatchCreate(txCtx, allEntries)
		return err
	}); err != nil {
		return err
	}

	// 仅当有弱项或编程错因时才发布 archive.written 事件
	if len(payload.WeakTopics) == 0 && len(payload.CodingMistakeTags) == 0 {
		return nil
	}

	allWeakTopics := make([]string, 0, len(payload.WeakTopics)+len(payload.CodingMistakeTags))
	allWeakTopics = append(allWeakTopics, payload.WeakTopics...)
	allWeakTopics = append(allWeakTopics, payload.CodingMistakeTags...)

	if uc.publisher != nil {
		if err := uc.publisher.PublishArchiveWritten(ctx, payload.UserID, "interview", payload.InterviewID, allWeakTopics, payload.StrengthTopics); err != nil {
			uc.logger.Errorf("发布 archive.written 事件失败: %v", err)
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
	case "practice_question", "plan_task_feedback":
		if sourceRef == "" {
			return nil, nil
		}
		return uc.repo.GetBySource(ctx, entry.UserID, 0, sourceType, sourceRef)
	default:
		return nil, nil
	}
}

// formatUint 将 uint64 转成稳定字符串，用于构造幂等 source_ref。
func formatUint(value uint64) string {
	return strconv.FormatUint(value, 10)
}

// PracticeRecommendation 练习推荐结果
type PracticeRecommendation struct {
	FocusTag        string   `json:"focus_tag"`
	TopicCode       string   `json:"topic_code"`
	TopicTitle      string   `json:"topic_title"`
	Keywords        []string `json:"keywords"`
	OccurrenceCount int      `json:"occurrence_count"`
	Reason          string   `json:"reason"`
}

// MistakeTopicSummary 专题概要（列表页用）
type MistakeTopicSummary struct {
	Code           string `json:"code"`
	Tag            string `json:"tag"`
	Title          string `json:"title"`
	ProblemPattern string `json:"problem_pattern"`
}

// GrowthTrendSummary 从焦点信号中派生的趋势摘要
type GrowthTrendSummary struct {
	DominantSource      string `json:"dominant_source"`
	DominantSourceLabel string `json:"dominant_source_label"`
	TopFocusTag         string `json:"top_focus_tag"`
	TopTopicCode        string `json:"top_topic_code"`
	TopTopicTitle       string `json:"top_topic_title"`
	Summary             string `json:"summary"`
}

// buildRecommendationKeywords 从焦点信号中构造用于搜索题目的关键词列表。
func buildRecommendationKeywords(sig TrainingFocusSignal) []string {
	keywords := []string{}
	if sig.Tag != "" {
		keywords = append(keywords, sig.Tag)
	}
	if sig.TopicTitle != "" && sig.TopicTitle != sig.Tag {
		keywords = append(keywords, sig.TopicTitle)
	}
	if topic, ok := ResolveMistakeTopicByCode(sig.TopicCode); ok {
		for _, dir := range topic.PracticeDirections {
			keywords = appendUniqueStrings(keywords, dir)
		}
	}
	return keywords
}
