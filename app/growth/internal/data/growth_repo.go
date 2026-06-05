package data

import (
	"context"
	"time"

	"gorm.io/gorm"

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

func (r *growthRepo) GetUserGrowthSummary(ctx context.Context, userID uint64) (*biz.GrowthSummary, error) {
	return r.GetStudyLogStats(ctx, userID)
}

func (r *growthRepo) CreateStudyLog(ctx context.Context, log *biz.StudyLog) error {
	m := &model.StudyLog{
		UserID:   log.UserID,
		Action:   log.Action,
		RefID:    log.RefID,
		Duration: log.Duration,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	log.ID = uint64(m.ID)
	log.CreatedAt = m.CreatedAt
	return nil
}

func (r *growthRepo) GetStudyLogStats(ctx context.Context, userID uint64) (*biz.GrowthSummary, error) {
	base := r.db.WithContext(ctx).Model(&model.StudyLog{}).Where("user_id = ?", userID)

	// Count distinct study days
	var totalDays int64
	base.Distinct("DATE(created_at)").Count(&totalDays)

	// Count questions (action = "practice" or "question")
	var totalQuestions int64
	base.Where("action IN ?", []string{"practice", "question"}).Count(&totalQuestions)

	// Count interviews (action = "interview")
	var totalInterviews int64
	base.Where("action = ?", "interview").Count(&totalInterviews)

	// Calculate current streak: consecutive days with at least one log
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

// calculateStreak calculates consecutive study days ending today
func (r *growthRepo) calculateStreak(ctx context.Context, userID uint64) int32 {
	var dates []string
	r.db.WithContext(ctx).
		Model(&model.StudyLog{}).
		Where("user_id = ?", userID).
		Select("DISTINCT DATE(created_at)").
		Order("DATE(created_at) DESC").
		Limit(60).
		Pluck("DATE(created_at)", &dates)

	if len(dates) == 0 {
		return 0
	}

	today := time.Now().UTC().Format("2006-01-02")
	streak := int32(0)
	expected := time.Now().UTC()

	for _, d := range dates {
		expectedStr := expected.Format("2006-01-02")
		if d == expectedStr {
			streak++
			expected = expected.AddDate(0, 0, -1)
		} else if d == today {
			// today counts even if partial
			continue
		} else {
			break
		}
	}
	return streak
}

func (r *growthRepo) GetWeeklyFocusItems(ctx context.Context, userID uint64) ([]*biz.FocusItem, error) {
	// Get action distribution from the past 7 days
	type actionCount struct {
		Action string
		Count  int64
	}
	var results []actionCount
	weekAgo := time.Now().UTC().AddDate(0, 0, -7)

	r.db.WithContext(ctx).
		Model(&model.StudyLog{}).
		Where("user_id = ? AND created_at >= ?", userID, weekAgo).
		Select("action, COUNT(*) as count").
		Group("action").
		Order("count DESC").
		Scan(&results)

	items := make([]*biz.FocusItem, 0, len(results))
	for _, r := range results {
		suggestion := ""
		switch r.Action {
		case "practice", "question":
			suggestion = "continue practicing to improve accuracy"
		case "interview":
			suggestion = "practice more mock interviews"
		default:
			suggestion = "keep up the good work"
		}
		items = append(items, &biz.FocusItem{
			Topic:      r.Action,
			Source:     "study_logs",
			Weight:     float64(r.Count),
			Suggestion: suggestion,
		})
	}

	return items, nil
}
