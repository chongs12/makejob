package data

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/data/model"
)

type learningArchiveRepo struct {
	db *gorm.DB
}

// NewLearningArchiveRepo 创建学习档案仓储实例
func NewLearningArchiveRepo(db *gorm.DB) biz.LearningArchiveRepo {
	return &learningArchiveRepo{db: db}
}

// Upsert 按 user_id + source_ref 创建或更新学习档案条目
func (r *learningArchiveRepo) Upsert(ctx context.Context, entry *biz.LearningArchiveEntry) error {
	if entry == nil {
		return fmt.Errorf("学习档案条目不能为空")
	}

	m := &model.LearningArchiveEntry{
		UserID:           entry.UserID,
		SourceType:       entry.SourceType,
		SourceRef:        entry.SourceRef,
		InterviewID:      entry.InterviewID,
		QuestionIndex:    entry.QuestionIndex,
		IndustryCode:     entry.IndustryCode,
		TaskPhase:        entry.TaskPhase,
		TaskPhaseGoal:    entry.TaskPhaseGoal,
		Language:         entry.Language,
		MistakeTagsJSON:  entry.MistakeTagsJSON,
		StrengthTagsJSON: entry.StrengthTagsJSON,
		SuggestionsJSON:  entry.SuggestionsJSON,
		EvidenceSummary:  entry.EvidenceSummary,
		OccurredAt:       entry.OccurredAt,
	}

	var existing model.LearningArchiveEntry
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND source_ref = ?", entry.UserID, entry.SourceRef).
		First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("查询学习档案条目失败: %w", err)
	}

	if err == gorm.ErrRecordNotFound {
		if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
			return fmt.Errorf("创建学习档案条目失败: %w", err)
		}
		entry.ID = uint64(m.ID)
		entry.CreatedAt = m.CreatedAt
		entry.UpdatedAt = m.UpdatedAt
		return nil
	}

	m.ID = existing.ID
	m.CreatedAt = existing.CreatedAt
	m.UpdatedAt = existing.UpdatedAt
	if err := r.db.WithContext(ctx).Save(m).Error; err != nil {
		return fmt.Errorf("更新学习档案条目失败: %w", err)
	}
	entry.ID = uint64(m.ID)
	entry.CreatedAt = m.CreatedAt
	entry.UpdatedAt = m.UpdatedAt
	return nil
}

// ListRecentByUser 获取用户最近的学习档案条目，可按面试过滤
func (r *learningArchiveRepo) ListRecentByUser(ctx context.Context, userID uint64, limit int, interviewID *uint64) ([]*biz.LearningArchiveEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("COALESCE(occurred_at, updated_at) DESC").
		Limit(limit)
	if interviewID != nil && *interviewID > 0 {
		query = query.Where("interview_id = ?", *interviewID)
	}

	var models []model.LearningArchiveEntry
	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询学习档案条目失败: %w", err)
	}

	entries := make([]*biz.LearningArchiveEntry, len(models))
	for i, m := range models {
		entries[i] = &biz.LearningArchiveEntry{
			ID:               uint64(m.ID),
			UserID:           m.UserID,
			SourceType:       m.SourceType,
			SourceRef:        m.SourceRef,
			InterviewID:      m.InterviewID,
			QuestionIndex:    m.QuestionIndex,
			IndustryCode:     m.IndustryCode,
			TaskPhase:        m.TaskPhase,
			TaskPhaseGoal:    m.TaskPhaseGoal,
			Language:         m.Language,
			MistakeTagsJSON:  m.MistakeTagsJSON,
			StrengthTagsJSON: m.StrengthTagsJSON,
			SuggestionsJSON:  m.SuggestionsJSON,
			EvidenceSummary:  m.EvidenceSummary,
			OccurredAt:       m.OccurredAt,
			CreatedAt:        m.CreatedAt,
			UpdatedAt:        m.UpdatedAt,
		}
	}
	return entries, nil
}
