package data

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"makejob/app/growth/internal/biz"
	"makejob/app/growth/internal/data/model"
)

type growthRepo struct {
	db *gorm.DB
}

// NewGrowthRepo 创建成长仓库实现
func NewGrowthRepo(db *gorm.DB) biz.GrowthRepo {
	return &growthRepo{db: db}
}

func (r *growthRepo) GetStudyLogStats(ctx context.Context, userID uint64) (*biz.GrowthSummary, error) {
	base := r.db.WithContext(ctx).Model(&model.StudyLog{}).Where("user_id = ?", userID)

	// 统计学习天数
	var totalDays int64
	base.Distinct("date_key").Count(&totalDays)

	// 统计题目数量
	var totalQuestions int64
	base.Where("action IN ?", []string{"practice", "question"}).Count(&totalQuestions)

	// 统计面试数量
	var totalInterviews int64
	base.Where("action = ?", "interview").Count(&totalInterviews)

	// 计算连续学习天数
	streak := r.calculateStreak(ctx, userID)

	return &biz.GrowthSummary{
		TotalStudyDays:  int32(totalDays),
		TotalQuestions:  int32(totalQuestions),
		TotalInterviews: int32(totalInterviews),
		CurrentStreak:   streak,
		AvgScore:        0,
		WeeklyStats:     []*biz.WeeklyStat{},
		WeakTopics:      []*biz.TopicWeakness{},
	}, nil
}

// calculateStreak 计算连续学习天数（从今天往前）
func (r *growthRepo) calculateStreak(ctx context.Context, userID uint64) int32 {
	var dates []string
	r.db.WithContext(ctx).
		Model(&model.StudyLog{}).
		Where("user_id = ?", userID).
		Select("DISTINCT date_key").
		Order("date_key DESC").
		Limit(60).
		Pluck("date_key", &dates)

	if len(dates) == 0 {
		return 0
	}

	today := time.Now().Format("2006-01-02")
	streak := int32(0)
	expected := time.Now()

	for _, d := range dates {
		expectedStr := expected.Format("2006-01-02")
		if d == expectedStr {
			streak++
			expected = expected.AddDate(0, 0, -1)
		} else if d == today {
			continue
		} else {
			break
		}
	}
	return streak
}

// UpsertStudyLog 插入或更新学习记录，基于 (user_id, date_key, action, ref_id) 唯一键
func (r *growthRepo) UpsertStudyLog(ctx context.Context, log *biz.StudyLog) error {
	m := &model.StudyLog{
		UserID:          log.UserID,
		DateKey:         log.DateKey,
		PlanID:          log.PlanID,
		Summary:         log.Summary,
		Action:          log.Action,
		RefID:           log.RefID,
		RefType:         log.RefType,
		DurationMinutes: log.DurationMinutes,
		Source:          log.Source,
	}

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "date_key"},
				{Name: "action"},
				{Name: "ref_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"duration_minutes", "summary", "updated_at"}),
		}).
		Create(m).Error
	if err != nil {
		return err
	}

	log.ID = uint64(m.ID)
	log.CreatedAt = m.CreatedAt
	return nil
}

func (r *growthRepo) GetWeeklyFocusItems(ctx context.Context, userID uint64) ([]*biz.FocusItem, error) {
	type actionCount struct {
		Action string
		Count  int64
	}
	var results []actionCount
	weekAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")

	r.db.WithContext(ctx).
		Model(&model.StudyLog{}).
		Where("user_id = ? AND date_key >= ?", userID, weekAgo).
		Select("action, COUNT(*) as count").
		Group("action").
		Order("count DESC").
		Scan(&results)

	items := make([]*biz.FocusItem, 0, len(results))
	for _, res := range results {
		suggestion := ""
		switch res.Action {
		case "practice", "question":
			suggestion = "继续刷题以提高正确率"
		case "interview":
			suggestion = "多练习模拟面试"
		default:
			suggestion = "保持当前学习节奏"
		}
		items = append(items, &biz.FocusItem{
			Topic:      res.Action,
			Source:     "study_logs",
			Weight:     float64(res.Count),
			Suggestion: suggestion,
		})
	}

	return items, nil
}
