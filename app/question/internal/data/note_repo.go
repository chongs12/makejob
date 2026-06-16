package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/data/model"
)

type noteRepo struct {
	db *gorm.DB
}

func NewNoteRepo(db *gorm.DB) biz.NoteRepo {
	return &noteRepo{db: db}
}

func (r *noteRepo) Create(ctx context.Context, note *biz.UserNote) error {
	m := &model.UserNote{
		UserID:     note.UserID,
		QuestionID: note.QuestionID,
		Title:      note.Title,
		Content:    note.Content,
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *noteRepo) Update(ctx context.Context, note *biz.UserNote) error {
	return r.db.WithContext(ctx).
		Model(&model.UserNote{}).
		Where("id = ? AND user_id = ?", note.ID, note.UserID).
		Updates(map[string]any{
			"title":   note.Title,
			"content": note.Content,
		}).Error
}

func (r *noteRepo) ListByUser(ctx context.Context, userID uint64, questionID uint64, page, pageSize int32) ([]*biz.UserNote, int64, error) {
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if questionID > 0 {
		query = query.Where("question_id = ?", questionID)
	}

	var total int64
	query.Model(&model.UserNote{}).Count(&total)

	var models []model.UserNote
	err := query.
		Order("id DESC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&models).Error
	if err != nil {
		return nil, 0, err
	}

	items := make([]*biz.UserNote, len(models))
	for i, m := range models {
		var qid *uint64
		if m.QuestionID != nil {
			v := *m.QuestionID
			qid = &v
		}
		items[i] = &biz.UserNote{
			ID:         uint64(m.ID),
			UserID:     m.UserID,
			QuestionID: qid,
			Title:      m.Title,
			Content:    m.Content,
			CreatedAt:  m.CreatedAt,
			UpdatedAt:  m.UpdatedAt,
		}
	}
	return items, total, nil
}

// GetByID 按 ID 获取笔记
func (r *noteRepo) GetByID(ctx context.Context, id uint64) (*biz.UserNote, error) {
	var m model.UserNote
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	var qid *uint64
	if m.QuestionID != nil {
		v := *m.QuestionID
		qid = &v
	}
	return &biz.UserNote{
		ID:         uint64(m.ID),
		UserID:     m.UserID,
		QuestionID: qid,
		Title:      m.Title,
		Content:    m.Content,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}, nil
}

// Delete 删除指定笔记（需校验归属）
func (r *noteRepo) Delete(ctx context.Context, id, userID uint64) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.UserNote{}).Error
}
