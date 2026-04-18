// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// QuestionListParams 题目列表查询参数
type QuestionListParams struct {
	Page       int
	PageSize   int
	CategoryID *uint
	IndustryID *uint
	Type       string
	Difficulty string
	Keyword    string
	Tags       string
	IsActive   *bool
}

// RandomQuestionParams 随机题目查询参数
type RandomQuestionParams struct {
	IndustryID *uint
	CategoryID *uint
	Difficulty string
	Count      int
	ExcludeIDs []uint // 排除已做过的题
}

// QuestionRepository 题目数据访问接口
type QuestionRepository interface {
	List(ctx context.Context, params QuestionListParams) ([]model.Question, int64, error)
	GetByID(ctx context.Context, id uint) (*model.Question, error)
	GetByIDs(ctx context.Context, ids []uint) ([]model.Question, error)
	GetRandomByParams(ctx context.Context, params RandomQuestionParams) ([]model.Question, error)
}

// questionRepository 题目数据访问实现
type questionRepository struct {
	db *gorm.DB
}

// NewQuestionRepository 创建题目仓库实例
func NewQuestionRepository(db *gorm.DB) QuestionRepository {
	return &questionRepository{
		db: db,
	}
}

// List 获取题目列表
func (r *questionRepository) List(ctx context.Context, params QuestionListParams) ([]model.Question, int64, error) {
	var questions []model.Question
	var total int64

	// 构建基础查询
	query := r.db.WithContext(ctx).Model(&model.Question{})

	// 应用筛选条件
	if params.CategoryID != nil {
		query = query.Where("category_id = ?", *params.CategoryID)
	}
	if params.IndustryID != nil {
		query = query.Where("industry_id = ?", *params.IndustryID)
	}
	if params.Type != "" {
		query = query.Where("type = ?", params.Type)
	}
	if params.Difficulty != "" {
		query = query.Where("difficulty = ?", params.Difficulty)
	}
	if params.Keyword != "" {
		keyword := "%" + params.Keyword + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", keyword, keyword)
	}
	if params.Tags != "" {
		tags := strings.Split(params.Tags, ",")
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				query = query.Where("tags LIKE ?", "%"+tag+"%")
			}
		}
	}
	if params.IsActive != nil {
		query = query.Where("is_active = ?", *params.IsActive)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计题目数量失败: %w", err)
	}

	// 分页查询
	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Limit(pageSize).Offset(offset).Find(&questions).Error; err != nil {
		return nil, 0, fmt.Errorf("查询题目列表失败: %w", err)
	}

	return questions, total, nil
}

// GetByID 根据ID获取题目
func (r *questionRepository) GetByID(ctx context.Context, id uint) (*model.Question, error) {
	var question model.Question
	if err := r.db.WithContext(ctx).First(&question, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询题目失败: %w", err)
	}
	return &question, nil
}

// GetByIDs 根据ID列表批量获取题目
func (r *questionRepository) GetByIDs(ctx context.Context, ids []uint) ([]model.Question, error) {
	var questions []model.Question
	if len(ids) == 0 {
		return questions, nil
	}
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("批量查询题目失败: %w", err)
	}
	return questions, nil
}

// GetRandomByParams 根据参数随机获取题目
func (r *questionRepository) GetRandomByParams(ctx context.Context, params RandomQuestionParams) ([]model.Question, error) {
	var questions []model.Question

	// 构建基础查询
	query := r.db.WithContext(ctx).Model(&model.Question{}).
		Where("is_active = ?", true)

	if params.IndustryID != nil {
		query = query.Where("industry_id = ?", *params.IndustryID)
	}
	if params.CategoryID != nil {
		query = query.Where("category_id = ?", *params.CategoryID)
	}
	if params.Difficulty != "" {
		query = query.Where("difficulty = ?", params.Difficulty)
	}
	if len(params.ExcludeIDs) > 0 {
		query = query.Where("id NOT IN ?", params.ExcludeIDs)
	}

	// 使用随机排序并限制数量
	count := params.Count
	if count <= 0 {
		count = 10
	}
	if count > 100 {
		count = 100
	}

	// 根据数据库方言选择随机函数，兼容 PostgreSQL / SQLite / MySQL。
	if err := query.Order(randomOrderExpression(r.db)).Limit(count).Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("随机查询题目失败: %w", err)
	}

	return questions, nil
}

// randomOrderExpression 根据当前数据库方言返回可用的随机排序表达式。
func randomOrderExpression(db *gorm.DB) string {
	if db == nil || db.Dialector == nil {
		return "RANDOM()"
	}

	switch strings.ToLower(strings.TrimSpace(db.Dialector.Name())) {
	case "mysql":
		return "RAND()"
	default:
		return "RANDOM()"
	}
}
