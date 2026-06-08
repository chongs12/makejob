package data

import (
	"context"
	"strings"

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
		IndustryID:      question.IndustryID,
		IndustryCode:    question.IndustryCode,
		IndustryName:    question.IndustryName,
		CategoryID:      question.CategoryID,
		CategoryName:    question.CategoryName,
		OptionsJSON:     question.OptionsJSON,
		StarterCode:     question.StarterCode,
		Language:        question.Language,
		EvaluationMode:  question.EvaluationMode,
		Answer:          question.Answer,
		ReferenceAnswer: question.ReferenceAnswer,
		Explanation:     question.Explanation,
		SolutionJSON:    question.SolutionJSON,
		JudgeConfigJSON: question.JudgeConfigJSON,
		AnswerTemplateJSON: question.AnswerTemplateJSON,
		Tags:            strings.Join(question.Tags, ","),
		TestCasesJSON:   question.TestCasesJSON,
		IsActive:        question.IsActive,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	question.ID = uint64(m.ID)
	question.CreatedAt = m.CreatedAt
	question.UpdatedAt = m.UpdatedAt
	return nil
}

// Update 更新题目主体信息，供管理后台 CRUD 使用。
func (r *questionRepo) Update(ctx context.Context, question *biz.Question) error {
	updates := map[string]any{
		"title":                question.Title,
		"content":              question.Content,
		"difficulty":           question.Difficulty,
		"type":                 question.Type,
		"industry_id":          question.IndustryID,
		"industry_code":        question.IndustryCode,
		"industry_name":        question.IndustryName,
		"category_id":          question.CategoryID,
		"category_name":        question.CategoryName,
		"options_json":         question.OptionsJSON,
		"answer":               question.Answer,
		"reference_answer":     question.ReferenceAnswer,
		"explanation":          question.Explanation,
		"solution_json":        question.SolutionJSON,
		"judge_config_json":    question.JudgeConfigJSON,
		"answer_template_json": question.AnswerTemplateJSON,
		"tags":                 strings.Join(question.Tags, ","),
		"starter_code":         question.StarterCode,
		"language":             question.Language,
		"evaluation_mode":      question.EvaluationMode,
		"test_cases_json":      question.TestCasesJSON,
	}
	if question.IsActive {
		updates["is_active"] = true
	} else {
		updates["is_active"] = false
	}
	return r.db.WithContext(ctx).Model(&QuestionModel{}).Where("id = ?", question.ID).Updates(updates).Error
}

// Delete 软删除题目。
func (r *questionRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&QuestionModel{}, id).Error
}

// Count 返回指定过滤条件下的题目总数。
func (r *questionRepo) Count(ctx context.Context, filter *biz.QuestionFilter) (int64, error) {
	query := r.db.WithContext(ctx).Model(&QuestionModel{})
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
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// toBizQuestion 将数据库模型转换为领域实体
func toBizQuestion(m *QuestionModel) *biz.Question {
	return &biz.Question{
		ID:              uint64(m.ID),
		Title:           m.Title,
		Content:         m.Content,
		Difficulty:      m.Difficulty,
		Type:            m.Type,
		IndustryID:      m.IndustryID,
		IndustryCode:    m.IndustryCode,
		IndustryName:    m.IndustryName,
		CategoryID:      m.CategoryID,
		CategoryName:    m.CategoryName,
		Tags:            splitQuestionTags(m.Tags),
		OptionsJSON:     m.OptionsJSON,
		Answer:          m.Answer,
		SolutionJSON:    m.SolutionJSON,
		JudgeConfigJSON: m.JudgeConfigJSON,
		AnswerTemplateJSON: m.AnswerTemplateJSON,
		StarterCode:     m.StarterCode,
		Language:        m.Language,
		EvaluationMode:  m.EvaluationMode,
		ReferenceAnswer: m.ReferenceAnswer,
		Explanation:     m.Explanation,
		TestCasesJSON:   m.TestCasesJSON,
		IsActive:        m.IsActive,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

// QuestionModel GORM model
type QuestionModel struct {
	gorm.Model
	Title           string `gorm:"size:500;not null"`
	Content         string `gorm:"type:text"`
	Difficulty      string `gorm:"size:20"`
	Type            string `gorm:"size:30"`
	IndustryID      uint64 `gorm:"index"`
	IndustryCode    string `gorm:"size:50;index"`
	IndustryName    string `gorm:"size:200"`
	CategoryID      uint64 `gorm:"index"`
	CategoryName    string `gorm:"size:200"`
	OptionsJSON     string `gorm:"type:text"`
	Answer          string `gorm:"type:text"`
	SolutionJSON    string `gorm:"type:text"`
	JudgeConfigJSON string `gorm:"type:text"`
	AnswerTemplateJSON string `gorm:"type:text"`
	Tags            string `gorm:"size:500"`
	StarterCode     string `gorm:"type:text"`
	Language        string `gorm:"size:30"`
	EvaluationMode  string `gorm:"size:30"`
	ReferenceAnswer string `gorm:"type:text"`
	Explanation     string `gorm:"type:text"`
	TestCasesJSON   string `gorm:"type:text"`
	IsActive        bool   `gorm:"not null;default:true"`
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
			IndustryID:   m.IndustryID,
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

// splitQuestionTags 将逗号分隔标签还原为字符串切片。
func splitQuestionTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}
