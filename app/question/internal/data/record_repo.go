package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/data/model"
)

type recordRepo struct {
	db *gorm.DB
}

func NewRecordRepo(db *gorm.DB) biz.RecordRepo {
	return &recordRepo{db: db}
}

func (r *recordRepo) Create(ctx context.Context, record *biz.UserQuestionRecord) error {
	m := &model.UserQuestionRecord{
		UserID:     record.UserID,
		QuestionID: record.QuestionID,
		IsCorrect:  record.IsCorrect,
		Answer:     record.Answer,
		Language:   record.Language,
		Score:      record.Score,
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *recordRepo) GetByUserAndQuestion(ctx context.Context, userID, questionID uint64) (*biz.UserQuestionRecord, error) {
	var m model.UserQuestionRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND question_id = ?", userID, questionID).
		Order("id DESC").
		First(&m).Error; err != nil {
		return nil, err
	}
	return &biz.UserQuestionRecord{
		ID:         uint64(m.ID),
		UserID:     m.UserID,
		QuestionID: m.QuestionID,
		IsCorrect:  m.IsCorrect,
		Answer:     m.Answer,
		Language:   m.Language,
		Score:      m.Score,
		CreatedAt:  m.CreatedAt,
	}, nil
}

type categoryStatRow struct {
	CategoryName string
	Total        int32
	Correct      int32
}

func (r *recordRepo) GetCategoryStats(ctx context.Context, userID uint64) ([]*biz.CategoryStat, error) {
	var rows []categoryStatRow
	err := r.db.WithContext(ctx).
		Table("user_question_records AS uqr").
		Select("COALESCE(c.name, '未分类') AS category_name, COUNT(*) AS total, SUM(CASE WHEN uqr.is_correct THEN 1 ELSE 0 END) AS correct").
		Joins("LEFT JOIN questions q ON q.id = uqr.question_id").
		Joins("LEFT JOIN categories c ON c.id = q.category_id").
		Where("uqr.user_id = ?", userID).
		Group("c.name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	stats := make([]*biz.CategoryStat, len(rows))
	for i, row := range rows {
		accuracy := float64(0)
		if row.Total > 0 {
			accuracy = float64(row.Correct) / float64(row.Total)
		}
		stats[i] = &biz.CategoryStat{
			CategoryName: row.CategoryName,
			Answered:     row.Total,
			Correct:      row.Correct,
			Accuracy:     accuracy,
		}
	}
	return stats, nil
}

type wrongQuestionRow struct {
	QuestionID  uint64
	Title       string
	WrongCount  int32
	LastWrongAt string
	LastAnswer  string
}

func (r *recordRepo) GetWrongQuestions(ctx context.Context, userID uint64, page, pageSize int32) ([]*biz.WrongQuestion, int64, error) {
	// Subquery: records where is_correct = false
	subQuery := r.db.WithContext(ctx).
		Table("user_question_records").
		Select("question_id, COUNT(*) AS wrong_count, MAX(created_at) AS last_wrong_at").
		Where("user_id = ? AND is_correct = false", userID).
		Group("question_id")

	var total int64
	subQuery.Count(&total)

	var rows []wrongQuestionRow
	err := r.db.WithContext(ctx).
		Table("(?) AS wq", subQuery).
		Select("wq.question_id, q.title, wq.wrong_count, wq.last_wrong_at, '' AS last_answer").
		Joins("LEFT JOIN questions q ON q.id = wq.question_id").
		Order("wq.wrong_count DESC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	items := make([]*biz.WrongQuestion, len(rows))
	for i, row := range rows {
		items[i] = &biz.WrongQuestion{
			QuestionID: row.QuestionID,
			Title:      row.Title,
			WrongCount: row.WrongCount,
			LastAnswer: row.LastAnswer,
		}
	}
	return items, total, nil
}
