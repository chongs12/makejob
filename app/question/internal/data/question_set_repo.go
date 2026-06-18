package data

import (
	"context"
	"fmt"

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
		items[i] = toBizQuestionSet(&m)
	}
	return items, total, nil
}

// GetByID 按 ID 获取题集详情
func (r *questionSetRepo) GetByID(ctx context.Context, id uint64) (*biz.QuestionSet, error) {
	var m model.QuestionSet
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toBizQuestionSet(&m), nil
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
		return []*biz.Question{}, nil
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
			CategoryID:   uint64(m.CategoryID),
		}
	}
	return questions, nil
}

// Create 创建题单
func (r *questionSetRepo) Create(ctx context.Context, set *biz.QuestionSet) error {
	m := &model.QuestionSet{
		Name:         set.Name,
		Description:  set.Description,
		IndustryCode: set.IndustryCode,
		CoverImage:   set.CoverImage,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return fmt.Errorf("创建题单失败: %w", err)
	}
	set.ID = uint64(m.ID)
	set.CreatedAt = m.CreatedAt
	return nil
}

// Update 更新题单基本信息
func (r *questionSetRepo) Update(ctx context.Context, set *biz.QuestionSet) error {
	updates := map[string]any{
		"name":          set.Name,
		"description":   set.Description,
		"industry_code": set.IndustryCode,
		"cover_image":   set.CoverImage,
	}
	return r.db.WithContext(ctx).Model(&model.QuestionSet{}).Where("id = ?", set.ID).Updates(updates).Error
}

// Delete 删除题单及其关联项
func (r *questionSetRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("set_id = ?", id).Delete(&model.QuestionSetItem{}).Error; err != nil {
			return fmt.Errorf("删除题单关联项失败: %w", err)
		}
		if err := tx.Delete(&model.QuestionSet{}, id).Error; err != nil {
			return fmt.Errorf("删除题单失败: %w", err)
		}
		return nil
	})
}

// AddQuestions 向题单添加题目（幂等去重，自动更新 question_count）
func (r *questionSetRepo) AddQuestions(ctx context.Context, setID uint64, questionIDs []uint64) (int32, error) {
	var added int32
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 查询已存在的关联
		var existing []model.QuestionSetItem
		if err := tx.Where("set_id = ? AND question_id IN ?", setID, questionIDs).Find(&existing).Error; err != nil {
			return err
		}
		existingMap := make(map[uint64]bool, len(existing))
		for _, e := range existing {
			existingMap[e.QuestionID] = true
		}

		// 获取当前最大 sort_order
		var maxOrder int32
		tx.Model(&model.QuestionSetItem{}).Where("set_id = ?", setID).Select("COALESCE(MAX(sort_order), 0)").Scan(&maxOrder)

		// 插入新关联
		for _, qID := range questionIDs {
			if existingMap[qID] {
				continue
			}
			maxOrder++
			item := &model.QuestionSetItem{
				SetID:      setID,
				QuestionID: qID,
				SortOrder:  maxOrder,
			}
			if err := tx.Create(item).Error; err != nil {
				return err
			}
			added++
		}

		// 更新 question_count
		if added > 0 {
			var count int64
			tx.Model(&model.QuestionSetItem{}).Where("set_id = ?", setID).Count(&count)
			tx.Model(&model.QuestionSet{}).Where("id = ?", setID).Update("question_count", count)
		}

		return nil
	})
	return added, err
}

// RemoveQuestions 从题单移除题目（自动更新 question_count）
func (r *questionSetRepo) RemoveQuestions(ctx context.Context, setID uint64, questionIDs []uint64) (int32, error) {
	var removed int32
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("set_id = ? AND question_id IN ?", setID, questionIDs).Delete(&model.QuestionSetItem{})
		if result.Error != nil {
			return result.Error
		}
		removed = int32(result.RowsAffected)

		if removed > 0 {
			var count int64
			tx.Model(&model.QuestionSetItem{}).Where("set_id = ?", setID).Count(&count)
			tx.Model(&model.QuestionSet{}).Where("id = ?", setID).Update("question_count", count)
		}
		return nil
	})
	return removed, err
}

// GetQuestionIDs 获取题单关联的所有题目 ID
func (r *questionSetRepo) GetQuestionIDs(ctx context.Context, setID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Model(&model.QuestionSetItem{}).
		Where("set_id = ?", setID).
		Order("sort_order ASC").
		Pluck("question_id", &ids).Error
	return ids, err
}

// toBizQuestionSet 将数据库模型转换为领域实体
func toBizQuestionSet(m *model.QuestionSet) *biz.QuestionSet {
	return &biz.QuestionSet{
		ID:            uint64(m.ID),
		Name:          m.Name,
		Description:   m.Description,
		IndustryCode:  m.IndustryCode,
		CoverImage:    m.CoverImage,
		QuestionCount: m.QuestionCount,
		CreatedAt:     m.CreatedAt,
	}
}
