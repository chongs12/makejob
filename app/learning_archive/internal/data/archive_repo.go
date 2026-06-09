package data

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"makejob/app/learning_archive/internal/biz"
	"makejob/app/learning_archive/internal/data/model"
)

// txContextKey 用于在 context 中透传事务 DB。
type txContextKey struct{}

type archiveRepo struct {
	db *gorm.DB
}

func NewArchiveRepo(db *gorm.DB) biz.ArchiveRepo {
	return &archiveRepo{db: db}
}

// getDB 从上下文中提取事务 DB；若不存在则回退到默认连接。
func (r *archiveRepo) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db
}

// Transaction 在事务中执行学习档案写入逻辑，并把事务连接透传到下游仓储方法。
func (r *archiveRepo) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txContextKey{}, tx)
		return fn(txCtx)
	})
}

// Create 创建单条学习档案，并在事务上下文中复用同一连接。
func (r *archiveRepo) Create(ctx context.Context, entry *biz.ArchiveEntry) error {
	m := toModel(entry)
	if err := r.getDB(ctx).WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	entry.ID = uint64(m.ID)
	entry.CreatedAt = m.CreatedAt
	return nil
}

// BatchCreate 批量写入学习档案，并在事务上下文中复用同一连接。
func (r *archiveRepo) BatchCreate(ctx context.Context, entries []*biz.ArchiveEntry) (int, error) {
	models := make([]model.LearningArchiveEntry, len(entries))
	for i, e := range entries {
		models[i] = *toModel(e)
	}
	if err := r.getDB(ctx).WithContext(ctx).CreateInBatches(models, 100).Error; err != nil {
		return 0, err
	}
	return len(entries), nil
}

// ListByUser 按用户读取学习档案列表，并支持事务内一致性读取。
func (r *archiveRepo) ListByUser(ctx context.Context, userID uint64, limit int32) ([]*biz.ArchiveEntry, error) {
	var models []model.LearningArchiveEntry
	if err := r.getDB(ctx).WithContext(ctx).
		Where("user_id = ?", userID).
		Where("source_type <> ?", biz.ArchiveSourceTypeInterviewFinishedMarker).
		Order("occurred_at DESC").
		Limit(int(limit)).
		Find(&models).Error; err != nil {
		return nil, err
	}
	entries := make([]*biz.ArchiveEntry, len(models))
	for i, m := range models {
		entries[i] = toBiz(&m)
	}
	return entries, nil
}

// GetBySource 按幂等来源键读取历史条目，若不存在则返回 nil。
func (r *archiveRepo) GetBySource(ctx context.Context, userID, interviewID uint64, sourceType, sourceRef string) (*biz.ArchiveEntry, error) {
	query := r.getDB(ctx).WithContext(ctx).Model(&model.LearningArchiveEntry{}).
		Where("user_id = ? AND interview_id = ? AND source_type = ?", userID, interviewID, sourceType)
	if sourceRef == "" {
		query = query.Where("COALESCE(source_ref, '') = ''")
	} else {
		query = query.Where("source_ref = ?", sourceRef)
	}
	var entity model.LearningArchiveEntry
	if err := query.Order("id ASC").First(&entity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toBiz(&entity), nil
}

// HasInterviewFinishedMarker 判断 interview.finished 事件是否已经写入处理标记。
func (r *archiveRepo) HasInterviewFinishedMarker(ctx context.Context, interviewID, userID uint64) (bool, error) {
	var count int64
	if err := r.getDB(ctx).WithContext(ctx).Model(&model.LearningArchiveEntry{}).
		Where("user_id = ? AND interview_id = ? AND source_type = ?", userID, interviewID, biz.ArchiveSourceTypeInterviewFinishedMarker).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *archiveRepo) GetWeakTopics(ctx context.Context, userID uint64) ([]string, error) {
	// 查询用户最近的错误标签，按出现次数排序
	type topicCount struct {
		Tag   string
		Count int
	}

	// Raw SQL 不会自动继承 GORM 软删除条件，这里显式过滤 deleted_at。
	rows, err := r.db.WithContext(ctx).
		Raw(`SELECT jsonb_array_elements_text(mistake_tags::jsonb) as tag, COUNT(*) as count
			 FROM learning_archive_entries
			 WHERE user_id = ? AND deleted_at IS NULL AND mistake_tags IS NOT NULL AND mistake_tags != ''
			 GROUP BY tag ORDER BY count DESC LIMIT 10`, userID).
		Rows()
	if err != nil {
		// 如果 JSON 查询失败，返回空
		return nil, err
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var tc topicCount
		if err := rows.Scan(&tc.Tag, &tc.Count); err != nil {
			continue
		}
		topics = append(topics, tc.Tag)
	}
	return topics, nil
}

func (r *archiveRepo) GetFocusSignals(ctx context.Context, userID uint64) ([]*biz.FocusSignal, error) {
	// 查询最近 30 天的学习记录，按来源类型聚合
	var signals []*biz.FocusSignal

	type sourceCount struct {
		SourceType string
		Count      int
	}
	var results []sourceCount

	if err := r.db.WithContext(ctx).
		Model(&model.LearningArchiveEntry{}).
		Select("source_type, COUNT(*) as count").
		Where("user_id = ? AND occurred_at > ? AND source_type <> ?", userID, time.Now().AddDate(0, 0, -30), biz.ArchiveSourceTypeInterviewFinishedMarker).
		Group("source_type").
		Find(&results).Error; err != nil {
		return nil, err
	}

	for _, res := range results {
		signals = append(signals, &biz.FocusSignal{
			Topic:  res.SourceType,
			Weight: float64(res.Count),
			Source: "learning_archive",
		})
	}

	if signals == nil {
		return []*biz.FocusSignal{}, nil
	}
	return signals, nil
}

// --- 转换函数 ---

func toModel(e *biz.ArchiveEntry) *model.LearningArchiveEntry {
	mistakeTags, _ := json.Marshal(e.MistakeTags)
	strengthTags, _ := json.Marshal(e.StrengthTags)
	suggestions, _ := json.Marshal(e.Suggestions)

	return &model.LearningArchiveEntry{
		UserID:          e.UserID,
		SourceType:      e.SourceType,
		SourceRef:       e.SourceRef,
		InterviewID:     e.InterviewID,
		QuestionIndex:   e.QuestionIndex,
		IndustryCode:    e.IndustryCode,
		PlanPhase:       e.PlanPhase,
		PlanPhaseGoal:   e.PlanPhaseGoal,
		Language:        e.Language,
		MistakeTags:     string(mistakeTags),
		StrengthTags:    string(strengthTags),
		Suggestions:     string(suggestions),
		EvidenceSummary: e.EvidenceSummary,
		OccurredAt:      e.OccurredAt,
	}
}

func toBiz(m *model.LearningArchiveEntry) *biz.ArchiveEntry {
	var mistakeTags, strengthTags, suggestions []string
	if err := json.Unmarshal([]byte(m.MistakeTags), &mistakeTags); err != nil {
		mistakeTags = []string{}
	}
	if err := json.Unmarshal([]byte(m.StrengthTags), &strengthTags); err != nil {
		strengthTags = []string{}
	}
	if err := json.Unmarshal([]byte(m.Suggestions), &suggestions); err != nil {
		suggestions = []string{}
	}

	return &biz.ArchiveEntry{
		ID:              uint64(m.ID),
		UserID:          m.UserID,
		SourceType:      m.SourceType,
		SourceRef:       m.SourceRef,
		InterviewID:     m.InterviewID,
		QuestionIndex:   m.QuestionIndex,
		IndustryCode:    m.IndustryCode,
		PlanPhase:       m.PlanPhase,
		PlanPhaseGoal:   m.PlanPhaseGoal,
		Language:        m.Language,
		MistakeTags:     mistakeTags,
		StrengthTags:    strengthTags,
		Suggestions:     suggestions,
		EvidenceSummary: m.EvidenceSummary,
		OccurredAt:      m.OccurredAt,
		CreatedAt:       m.CreatedAt,
	}
}
