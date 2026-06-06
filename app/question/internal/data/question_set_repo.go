package data

import (
	"context"

	"gorm.io/gorm"

	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/data/model"
)

type questionSetRepo struct {
	db *gorm.DB
}

// NewQuestionSetRepo 创建题集仓储实例
func NewQuestionSetRepo(db *gorm.DB) biz.QuestionSetRepo {
	return &questionSetRepo{db: db}
}

// List 获取题集列表（支持行业筛选和分页）
func (r *questionSetRepo) List(ctx context.Context, industryCode string, page, pageSize int32) ([]*biz.QuestionSet, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.QuestionSet{})
	if industryCode != "" {
		query = query.Where("industry_code = ?", industryCode)
	}

	var total int64
	query.Count(&total)

	var models []model.QuestionSet
	err := query.Order("id DESC").
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		Find(&models).Error
	if err != nil {
		return nil, 0, err
	}

	items := make([]*biz.QuestionSet, len(models))
	for i, m := range models {
		items[i] = &biz.QuestionSet{
			ID:            uint64(m.ID),
			Name:          m.Name,
			Description:   m.Description,
			IndustryCode:  m.IndustryCode,
			CoverImage:    m.CoverImage,
			QuestionCount: m.QuestionCount,
			CreatedAt:     m.CreatedAt,
		}
	}
	return items, total, nil
}

// GetByID 按 ID 获取题集详情
func (r *questionSetRepo) GetByID(ctx context.Context, id uint64) (*biz.QuestionSet, error) {
	var m model.QuestionSet
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &biz.QuestionSet{
		ID:            uint64(m.ID),
		Name:          m.Name,
		Description:   m.Description,
		IndustryCode:  m.IndustryCode,
		CoverImage:    m.CoverImage,
		QuestionCount: m.QuestionCount,
		CreatedAt:     m.CreatedAt,
	}, nil
}

// GetQuestions 获取题集关联的题目列表
func (r *questionSetRepo) GetQuestions(ctx context.Context, setID uint64) ([]*biz.Question, error) {
	var items []model.QuestionSetItem
	if err := r.db.WithContext(ctx).
		Where("set_id = ?", setID).
		Order("sort_order ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, nil
	}

	qIDs := make([]uint64, len(items))
	for i, item := range items {
		qIDs[i] = item.QuestionID
	}

	var qModels []QuestionModel
	if err := r.db.WithContext(ctx).Where("id IN ?", qIDs).Find(&qModels).Error; err != nil {
		return nil, err
	}

	questions := make([]*biz.Question, len(qModels))
	for i, m := range qModels {
		questions[i] = &biz.Question{
			ID:           uint64(m.ID),
			Title:        m.Title,
			Content:      m.Content,
			Difficulty:   m.Difficulty,
			Type:         m.Type,
			IndustryCode: m.IndustryCode,
			CategoryID:   m.CategoryID,
		}
	}
	return questions, nil
}
