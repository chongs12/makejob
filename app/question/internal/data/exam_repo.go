package data

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/data/model"
)

type examRepo struct {
	db *gorm.DB
}

// NewExamRepo 创建考试仓储实例
func NewExamRepo(db *gorm.DB) biz.ExamRepo {
	return &examRepo{db: db}
}

// Create 创建考试记录
func (r *examRepo) Create(ctx context.Context, exam *biz.Exam) error {
	qIDsJSON, _ := json.Marshal(exam.QuestionIDs)
	m := &model.Exam{
		UserID:       exam.UserID,
		IndustryCode: exam.IndustryCode,
		QuestionIDs:  string(qIDsJSON),
		TimeLimitMin: exam.TimeLimitMin,
		Status:       exam.Status,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	exam.ID = uint64(m.ID)
	return nil
}

// GetByID 按 ID 获取考试记录
func (r *examRepo) GetByID(ctx context.Context, id uint64) (*biz.Exam, error) {
	var m model.Exam
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	var qIDs []uint64
	_ = json.Unmarshal([]byte(m.QuestionIDs), &qIDs)
	return &biz.Exam{
		ID:           uint64(m.ID),
		UserID:       m.UserID,
		IndustryCode: m.IndustryCode,
		QuestionIDs:  qIDs,
		TimeLimitMin: m.TimeLimitMin,
		Status:       m.Status,
		TotalScore:   m.TotalScore,
		CreatedAt:    m.CreatedAt,
	}, nil
}

// Update 更新考试记录
func (r *examRepo) Update(ctx context.Context, exam *biz.Exam) error {
	updates := map[string]interface{}{
		"status":      exam.Status,
		"total_score": exam.TotalScore,
	}
	return r.db.WithContext(ctx).Model(&model.Exam{}).Where("id = ?", exam.ID).Updates(updates).Error
}
