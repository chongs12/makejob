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
	query := r.db.WithContext(ctx).Model(&QuestionModel{})

	// 应用过滤条件
	if filter != nil {
		if filter.IndustryCode != "" {
			query = query.Where("industry_code = ?", filter.IndustryCode)
		}
		if filter.CategoryID > 0 {
			query = query.Where("category_id = ?", filter.CategoryID)
		}
		if filter.Difficulty != "" {
			query = query.Where("difficulty = ?", filter.Difficulty)
		}
		if filter.Keyword != "" {
			query = query.Where("(title LIKE ? OR content LIKE ?)", "%"+filter.Keyword+"%", "%"+filter.Keyword+"%")
		}
	}

	var total int64
	query.Count(&total)

	var models []QuestionModel
	offset := (page - 1) * pageSize
	query.Offset(int(offset)).Limit(int(pageSize)).Find(&models)

	questions := make([]*biz.Question, len(models))
	for i, m := range models {
		questions[i] = toBizQuestion(&m)
	}
	return questions, total, nil
}

func (r *questionRepo) GetByID(ctx context.Context, id uint64) (*biz.Question, error) {
	var m QuestionModel
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toBizQuestion(&m), nil
}

// Create 创建题目
func (r *questionRepo) Create(ctx context.Context, question *biz.Question) error {
	m := &QuestionModel{
		Title:           question.Title,
		Content:         question.Content,
		Difficulty:      question.Difficulty,
		Type:            question.Type,
		IndustryCode:    question.IndustryCode,
		CategoryID:      question.CategoryID,
		CategoryName:    question.CategoryName,
		StarterCode:     question.StarterCode,
		Language:        question.Language,
		EvaluationMode:  question.EvaluationMode,
		ReferenceAnswer: question.ReferenceAnswer,
		Explanation:     question.Explanation,
		TestCasesJSON:   question.TestCasesJSON,
	}
	return r.db.WithContext(ctx).Create(m).Error
}

// toBizQuestion 将数据库模型转换为领域实体
func toBizQuestion(m *QuestionModel) *biz.Question {
	return &biz.Question{
		ID:              uint64(m.ID),
		Title:           m.Title,
		Content:         m.Content,
		Difficulty:      m.Difficulty,
		Type:            m.Type,
		IndustryCode:    m.IndustryCode,
		CategoryID:      m.CategoryID,
		CategoryName:    m.CategoryName,
		StarterCode:     m.StarterCode,
		Language:        m.Language,
		EvaluationMode:  m.EvaluationMode,
		ReferenceAnswer: m.ReferenceAnswer,
		Explanation:     m.Explanation,
		TestCasesJSON:   m.TestCasesJSON,
		CreatedAt:       m.CreatedAt,
	}
}

// QuestionModel GORM model
type QuestionModel struct {
	gorm.Model
	Title           string `gorm:"size:500;not null"`
	Content         string `gorm:"type:text"`
	Difficulty      string `gorm:"size:20"`
	Type            string `gorm:"size:30"`
	IndustryCode    string `gorm:"size:50;index"`
	CategoryID      uint64 `gorm:"index"`
	CategoryName    string `gorm:"size:200"`
	StarterCode     string `gorm:"type:text"`
	Language        string `gorm:"size:30"`
	EvaluationMode  string `gorm:"size:30"`
	ReferenceAnswer string `gorm:"type:text"`
	Explanation     string `gorm:"type:text"`
	TestCasesJSON   string `gorm:"type:text"`
}

// RandomSelect 按条件随机选取指定数量的题目
func (r *questionRepo) RandomSelect(ctx context.Context, filter *biz.QuestionFilter, count int32) ([]*biz.Question, error) {
	query := r.db.WithContext(ctx).Model(&QuestionModel{})
	if filter != nil && filter.IndustryCode != "" {
		query = query.Where("industry_code = ?", filter.IndustryCode)
	}
	if filter != nil && filter.CategoryID > 0 {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if filter != nil && filter.Difficulty != "" {
		query = query.Where("difficulty = ?", filter.Difficulty)
	}

	var models []QuestionModel
	if err := query.Order("RANDOM()").Limit(int(count)).Find(&models).Error; err != nil {
		return nil, err
	}

	questions := make([]*biz.Question, len(models))
	for i, m := range models {
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

// ExistsByTitleAndIndustry 检查题目是否已存在（FIX Q3: 幂等去重）
func (r *questionRepo) ExistsByTitleAndIndustry(ctx context.Context, title, industryCode string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&QuestionModel{}).
		Where("title = ? AND industry_code = ?", title, industryCode).
		Count(&count).Error
	return count > 0, err
}

func (QuestionModel) TableName() string { return "questions" }
