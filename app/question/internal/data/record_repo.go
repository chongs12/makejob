package data

import (
	"context"
	"time"

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
	now := time.Now().Unix()
	m := &model.UserQuestionRecord{
		UserID:     record.UserID,
		QuestionID: record.QuestionID,
		IsCorrect:  record.IsCorrect,
		UserAnswer: record.Answer,
		Language:   record.Language,
		Score:      record.Score,
		CreatedAt:  now,
	}
	return r.db.WithContext(ctx).Create(m).Error
}

// Upsert 按 user_id + question_id 去重，同一题只保留最新答题记录
func (r *recordRepo) Upsert(ctx context.Context, record *biz.UserQuestionRecord) error {
	now := time.Now().Unix()
	m := &model.UserQuestionRecord{
		UserID:     record.UserID,
		QuestionID: record.QuestionID,
		IsCorrect:  record.IsCorrect,
		UserAnswer: record.Answer,
		Language:   record.Language,
		Score:      record.Score,
		CreatedAt:  now,
	}

	var existing model.UserQuestionRecord
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND question_id = ?", record.UserID, record.QuestionID).
		First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(m).Error
	}

	m.ID = existing.ID
	m.CreatedAt = existing.CreatedAt
	return r.db.WithContext(ctx).Save(m).Error
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
		Answer:     m.UserAnswer,
		Language:   m.Language,
		Score:      m.Score,
		CreatedAt:  time.Unix(m.CreatedAt, 0),
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
	QuestionID   uint64
	Title        string
	Difficulty   string
	Type         string
	CategoryName string
	CategoryID   uint64
	WrongCount   int32
	LastWrongAt  string
	LastAnswer   string
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

	// 获取题目详情字段
	var rows []wrongQuestionRow
	err := r.db.WithContext(ctx).
		Table("(?) AS wq", subQuery).
		Select("wq.question_id, q.title, q.difficulty, q.type, q.category_name, q.category_id, wq.wrong_count, wq.last_wrong_at, "+
			"(SELECT uqr.user_answer FROM user_question_records uqr WHERE uqr.question_id = wq.question_id AND uqr.user_id = ? AND uqr.is_correct = false ORDER BY uqr.id DESC LIMIT 1) AS last_answer", userID).
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
			QuestionID:   row.QuestionID,
			Title:        row.Title,
			Difficulty:   row.Difficulty,
			Type:         row.Type,
			CategoryName: row.CategoryName,
			CategoryID:   row.CategoryID,
			WrongCount:   row.WrongCount,
			LastAnswer:   row.LastAnswer,
		}
	}
	return items, total, nil
}

// GetMistakeTopics 聚合查询用户各分类的错误统计
func (r *recordRepo) GetMistakeTopics(ctx context.Context, userID uint64) ([]*biz.MistakeTopic, error) {
	var rows []struct {
		CategoryID   uint64
		CategoryName string
		WrongCount   int32
		TotalCount   int32
	}

	err := r.db.WithContext(ctx).
		Table("user_question_records AS uqr").
		Select("COALESCE(q.category_id, 0) AS category_id, COALESCE(c.name, '未分类') AS category_name, "+
			"SUM(CASE WHEN uqr.is_correct = false THEN 1 ELSE 0 END) AS wrong_count, COUNT(*) AS total_count").
		Joins("LEFT JOIN questions q ON q.id = uqr.question_id").
		Joins("LEFT JOIN categories c ON c.id = q.category_id").
		Where("uqr.user_id = ?", userID).
		Group("q.category_id, c.name").
		Having("SUM(CASE WHEN uqr.is_correct = false THEN 1 ELSE 0 END) > 0").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	topics := make([]*biz.MistakeTopic, len(rows))
	for i, row := range rows {
		accuracy := float64(0)
		if row.TotalCount > 0 {
			accuracy = float64(row.TotalCount-row.WrongCount) / float64(row.TotalCount)
		}
		topics[i] = &biz.MistakeTopic{
			CategoryID:   row.CategoryID,
			CategoryName: row.CategoryName,
			WrongCount:   row.WrongCount,
			TotalCount:   row.TotalCount,
			Accuracy:     accuracy,
		}
	}
	return topics, nil
}

// GetTodayCount 查询用户今天练习的题目数量。
func (r *recordRepo) GetTodayCount(ctx context.Context, userID uint64) (int32, error) {
	var count int64
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	endOfDay := startOfDay + 86400
	err := r.db.WithContext(ctx).Model(&model.UserQuestionRecord{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, startOfDay, endOfDay).
		Count(&count).Error
	return int32(count), err
}

// GetAnsweredQuestionIDs 批量查询用户已答题的题目 ID 集合
func (r *recordRepo) GetAnsweredQuestionIDs(ctx context.Context, userID uint64, questionIDs []uint64) (map[uint64]bool, error) {
	if len(questionIDs) == 0 {
		return nil, nil
	}
	var ids []uint64
	err := r.db.WithContext(ctx).Model(&model.UserQuestionRecord{}).
		Where("user_id = ? AND question_id IN ?", userID, questionIDs).
		Distinct("question_id").
		Pluck("question_id", &ids).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}
