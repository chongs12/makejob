package data

import (
	"context"
	"encoding/json"
	"errors"

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

// ListRecentByUser 按用户读取最近档案，支持可选的 interviewID 过滤和更大的 limit。
func (r *archiveRepo) ListRecentByUser(ctx context.Context, userID uint64, limit int32, interviewID *uint64) ([]*biz.ArchiveEntry, error) {
	query := r.getDB(ctx).WithContext(ctx).
		Where("user_id = ?", userID).
		Where("source_type <> ?", biz.ArchiveSourceTypeInterviewFinishedMarker)
	if interviewID != nil {
		query = query.Where("interview_id = ?", *interviewID)
	}
	var models []model.LearningArchiveEntry
	if err := query.Order("occurred_at DESC").Limit(int(limit)).Find(&models).Error; err != nil {
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

// GetWeakTopics 查询用户高频薄弱标签，按出现次数降序返回。
func (r *archiveRepo) GetWeakTopics(ctx context.Context, userID uint64, limit int32) ([]string, error) {
	type topicCount struct {
		Tag   string
		Count int
	}

	// 兼容 mistake_tags 为 JSON 数组 ["tag1","tag2"] 或标量 "tag1" 两种格式
	rows, err := r.db.WithContext(ctx).
		Raw(`SELECT tag, COUNT(*) as count FROM (
				SELECT jsonb_array_elements_text(
					CASE WHEN jsonb_typeof(mistake_tags::jsonb) = 'array'
						THEN mistake_tags::jsonb
						ELSE jsonb_build_array(mistake_tags::jsonb)
					END
				) as tag
				FROM learning_archive_entries
				WHERE user_id = ? AND deleted_at IS NULL AND mistake_tags IS NOT NULL AND mistake_tags != ''
			) t
			GROUP BY tag ORDER BY count DESC LIMIT ?`, userID, limit).
		Rows()
	if err != nil {
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
		EntryPhase:      e.EntryPhase,
		TaskPhase:       e.TaskPhase,
		TaskPhaseGoal:   e.TaskPhaseGoal,
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
		EntryPhase:      m.EntryPhase,
		TaskPhase:       m.TaskPhase,
		TaskPhaseGoal:   m.TaskPhaseGoal,
		Language:        m.Language,
		MistakeTags:     mistakeTags,
		StrengthTags:    strengthTags,
		Suggestions:     suggestions,
		EvidenceSummary: m.EvidenceSummary,
		OccurredAt:      m.OccurredAt,
		CreatedAt:       m.CreatedAt,
	}
}


