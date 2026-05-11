// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// CategoryStat 分类统计
type CategoryStat struct {
	CategoryID   uint    `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Total        int64   `json:"total"`
	Correct      int64   `json:"correct"`
	AccuracyRate float64 `json:"accuracy_rate"`
}

// UserPracticeStats 用户练习统计
type UserPracticeStats struct {
	TotalAnswered int64          `json:"total_answered"`
	CorrectCount  int64          `json:"correct_count"`
	WrongCount    int64          `json:"wrong_count"`
	AccuracyRate  float64        `json:"accuracy_rate"`
	TodayCount    int64          `json:"today_count"`
	StreakDays    int            `json:"streak_days"`
	CategoryStats []CategoryStat `json:"category_stats"`
}

// QuestionRecordRepository 答题记录数据访问接口
type QuestionRecordRepository interface {
	Create(ctx context.Context, record *model.UserQuestionRecord) error
	Upsert(ctx context.Context, record *model.UserQuestionRecord) error
	GetByUserAndQuestion(ctx context.Context, userID, questionID uint) ([]model.UserQuestionRecord, error)
	GetWrongQuestions(ctx context.Context, userID uint, page, pageSize int) ([]model.UserQuestionRecord, int64, error)
	GetUserStats(ctx context.Context, userID uint) (*UserPracticeStats, error)
	GetDailyCount(ctx context.Context, userID uint, date time.Time) (int64, error)
}

// questionRecordRepository 答题记录数据访问实现
type questionRecordRepository struct {
	db *gorm.DB
}

// NewQuestionRecordRepository 创建答题记录仓库实例
func NewQuestionRecordRepository(db *gorm.DB) QuestionRecordRepository {
	return &questionRecordRepository{
		db: db,
	}
}

// Create 创建答题记录
func (r *questionRecordRepository) Create(ctx context.Context, record *model.UserQuestionRecord) error {
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("创建答题记录失败: %w", err)
	}
	return nil
}

// Upsert 创建或更新答题记录（按 user_id + question_id 去重）
func (r *questionRecordRepository) Upsert(ctx context.Context, record *model.UserQuestionRecord) error {
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND question_id = ?", record.UserID, record.QuestionID).
		Assign(map[string]interface{}{
			"user_answer":   record.UserAnswer,
			"is_correct":    record.IsCorrect,
			"time_spent":    record.TimeSpent,
			"analysis_json": record.AnalysisJSON,
		}).
		FirstOrCreate(record).Error
	if err != nil {
		return fmt.Errorf("保存答题记录失败: %w", err)
	}
	return nil
}

// GetByUserAndQuestion 获取用户的某题答题记录
func (r *questionRecordRepository) GetByUserAndQuestion(ctx context.Context, userID, questionID uint) ([]model.UserQuestionRecord, error) {
	var records []model.UserQuestionRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND question_id = ?", userID, questionID).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("查询答题记录失败: %w", err)
	}
	return records, nil
}

// GetWrongQuestions 获取用户的错题列表
func (r *questionRecordRepository) GetWrongQuestions(ctx context.Context, userID uint, page, pageSize int) ([]model.UserQuestionRecord, int64, error) {
	var records []model.UserQuestionRecord
	var total int64

	// 统计总数 - 使用子查询获取用户答错的题目（最新的答题记录）
	subQuery := r.db.WithContext(ctx).Model(&model.UserQuestionRecord{}).
		Select("question_id, MAX(created_at) as max_created_at").
		Where("user_id = ?", userID).
		Group("question_id")

	// 统计答错的题目数量
	countQuery := r.db.WithContext(ctx).Model(&model.UserQuestionRecord{}).
		Joins("JOIN (?) as latest ON user_question_records.question_id = latest.question_id AND user_question_records.created_at = latest.max_created_at", subQuery).
		Where("user_question_records.user_id = ? AND user_question_records.is_correct = ?", userID, false)

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计错题数量失败: %w", err)
	}

	// 分页查询错题
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// 获取错题记录，同时预加载题目信息
	if err := r.db.WithContext(ctx).
		Joins("JOIN (?) as latest ON user_question_records.question_id = latest.question_id AND user_question_records.created_at = latest.max_created_at", subQuery).
		Where("user_question_records.user_id = ? AND user_question_records.is_correct = ?", userID, false).
		Preload("Question").
		Order("user_question_records.created_at DESC").
		Limit(pageSize).Offset(offset).
		Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("查询错题列表失败: %w", err)
	}

	return records, total, nil
}

// GetUserStats 获取用户练习统计
func (r *questionRecordRepository) GetUserStats(ctx context.Context, userID uint) (*UserPracticeStats, error) {
	stats := &UserPracticeStats{
		CategoryStats: []CategoryStat{},
	}

	// 获取总答题数和正确数
	type Result struct {
		Total   int64
		Correct int64
	}
	var result Result
	if err := r.db.WithContext(ctx).Model(&model.UserQuestionRecord{}).
		Select("COUNT(*) as total, SUM(CASE WHEN is_correct = true THEN 1 ELSE 0 END) as correct").
		Where("user_id = ?", userID).
		Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("查询用户答题统计失败: %w", err)
	}

	stats.TotalAnswered = result.Total
	stats.CorrectCount = result.Correct
	stats.WrongCount = result.Total - result.Correct
	if result.Total > 0 {
		stats.AccuracyRate = float64(result.Correct) / float64(result.Total) * 100
	}

	// 获取今日答题数
	today := time.Now().Truncate(24 * time.Hour)
	todayCount, err := r.GetDailyCount(ctx, userID, today)
	if err != nil {
		return nil, err
	}
	stats.TodayCount = todayCount

	// 获取连续答题天数（简化实现：检查最近7天是否有答题记录）
	stats.StreakDays = r.calculateStreakDays(ctx, userID)

	// 获取分类统计
	categoryStats, err := r.getCategoryStats(ctx, userID)
	if err != nil {
		return nil, err
	}
	stats.CategoryStats = categoryStats

	return stats, nil
}

// GetDailyCount 获取某天的答题数量
func (r *questionRecordRepository) GetDailyCount(ctx context.Context, userID uint, date time.Time) (int64, error) {
	var count int64
	startOfDay := date.Truncate(24 * time.Hour)
	endOfDay := startOfDay.Add(24 * time.Hour)

	if err := r.db.WithContext(ctx).Model(&model.UserQuestionRecord{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, startOfDay.Unix(), endOfDay.Unix()).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("查询每日答题数失败: %w", err)
	}
	return count, nil
}

// calculateStreakDays 计算连续答题天数
func (r *questionRecordRepository) calculateStreakDays(ctx context.Context, userID uint) int {
	// 简化实现：检查最近7天中答题的天数
	streak := 0
	now := time.Now()

	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		count, err := r.GetDailyCount(ctx, userID, date)
		if err != nil || count == 0 {
			if i > 0 {
				break // 中断连续
			}
			continue
		}
		streak++
	}

	return streak
}

// getCategoryStats 获取分类统计
func (r *questionRecordRepository) getCategoryStats(ctx context.Context, userID uint) ([]CategoryStat, error) {
	var stats []CategoryStat

	// 使用JOIN查询分类统计
	type RawStat struct {
		CategoryID   uint
		CategoryName string
		Total        int64
		Correct      int64
	}

	var rawStats []RawStat
	if err := r.db.WithContext(ctx).Model(&model.UserQuestionRecord{}).
		Select("questions.category_id as category_id, categories.name as category_name, COUNT(*) as total, SUM(CASE WHEN user_question_records.is_correct = true THEN 1 ELSE 0 END) as correct").
		Joins("JOIN questions ON user_question_records.question_id = questions.id").
		Joins("JOIN categories ON questions.category_id = categories.id").
		Where("user_question_records.user_id = ?", userID).
		Group("questions.category_id, categories.name").
		Scan(&rawStats).Error; err != nil {
		return nil, fmt.Errorf("查询分类统计失败: %w", err)
	}

	for _, rs := range rawStats {
		accuracyRate := float64(0)
		if rs.Total > 0 {
			accuracyRate = float64(rs.Correct) / float64(rs.Total) * 100
		}
		stats = append(stats, CategoryStat{
			CategoryID:   rs.CategoryID,
			CategoryName: rs.CategoryName,
			Total:        rs.Total,
			Correct:      rs.Correct,
			AccuracyRate: accuracyRate,
		})
	}

	return stats, nil
}
