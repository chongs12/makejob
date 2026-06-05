package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/question/internal/biz"
)

type questionRepo struct{ db *gorm.DB }

func NewQuestionRepo(db *gorm.DB) biz.QuestionRepo {
	return &questionRepo{db: db}
}

func (r *questionRepo) List(ctx context.Context, filter *biz.QuestionFilter, page, pageSize int32) ([]*biz.Question, int64, error) {
	// 简化实现 - 实际应根据 filter 构建查询
	var total int64
	r.db.WithContext(ctx).Model(&QuestionModel{}).Count(&total)

	var models []QuestionModel
	offset := (page - 1) * pageSize
	r.db.WithContext(ctx).Offset(int(offset)).Limit(int(pageSize)).Find(&models)

	questions := make([]*biz.Question, len(models))
	for i, m := range models {
		questions[i] = &biz.Question{
			ID:           uint64(m.ID),
			Title:        m.Title,
			Content:      m.Content,
			Difficulty:   m.Difficulty,
			Type:         m.Type,
			IndustryCode: m.IndustryCode,
		}
	}
	return questions, total, nil
}

func (r *questionRepo) GetByID(ctx context.Context, id uint64) (*biz.Question, error) {
	var m QuestionModel
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &biz.Question{
		ID:           uint64(m.ID),
		Title:        m.Title,
		Content:      m.Content,
		Difficulty:   m.Difficulty,
		Type:         m.Type,
		IndustryCode: m.IndustryCode,
	}, nil
}

// QuestionModel GORM model
type QuestionModel struct {
	gorm.Model
	Title        string `gorm:"size:500;not null"`
	Content      string `gorm:"type:text"`
	Difficulty   string `gorm:"size:20"`
	Type         string `gorm:"size:30"`
	IndustryCode string `gorm:"size:50;index"`
	CategoryID   uint64 `gorm:"index"`
}

func (QuestionModel) TableName() string { return "questions" }
