package data

import (
	"context"
	"encoding/json"
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
	base.Distinct("log_date").Count(&totalDays)

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
		Select("DISTINCT log_date").
		Order("log_date DESC").
		Limit(60).
		Pluck("log_date", &dates)

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

// UpsertStudyLog 插入或更新学习记录，基于 (user_id, log_date, action, ref_id) 唯一键
func (r *growthRepo) UpsertStudyLog(ctx context.Context, log *biz.StudyLog) error {
	completedTitlesJSON := joinStudyLogTitles(log.CompletedTitles)
	skippedTitlesJSON := joinStudyLogTitles(log.SkippedTitles)

	m := &model.StudyLog{
		UserID:           log.UserID,
		DateKey:          log.DateKey,
		PlanID:           log.PlanID,
		Summary:          log.Summary,
		Action:           log.Action,
		RefID:            log.RefID,
		RefType:          log.RefType,
		DurationMinutes:  log.DurationMinutes,
		Source:           log.Source,
		FocusTaskTitle:   log.FocusTaskTitle,
		CompletedCount:   log.CompletedCount,
		SkippedCount:     log.SkippedCount,
		CompletedTitles:  completedTitlesJSON,
		SkippedTitles:    skippedTitlesJSON,
		LatestActionText: log.LatestActionText,
	}

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "log_date"},
				{Name: "action"},
				{Name: "ref_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"duration_minutes", "summary", "updated_at",
				"focus_task_title", "completed_count", "skipped_count",
				"completed_titles", "skipped_titles", "latest_action_text",
			}),
		}).
		Create(m).Error
	if err != nil {
		return err
	}

	log.ID = uint64(m.ID)
	log.CreatedAt = m.CreatedAt
	return nil
}

// joinStudyLogTitles 将标题列表序列化为 JSON 字符串用于存储。
func joinStudyLogTitles(titles []string) string {
	if len(titles) == 0 {
		return "[]"
	}
	data, err := json.Marshal(titles)
	if err != nil {
		return "[]"
	}
	return string(data)
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
		Where("user_id = ? AND log_date >= ?", userID, weekAgo).
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

// GetWeeklyStats 查询最近 N 周的学习统计数据。
func (r *growthRepo) GetWeeklyStats(ctx context.Context, userID uint64, weeks int) ([]*biz.WeeklyStat, error) {
	type weeklyRow struct {
		Week             string
		QuestionsAnswered int64
		InterviewsTaken  int64
	}
	var rows []weeklyRow

	weekAgo := time.Now().AddDate(0, 0, -weeks*7).Format("2006-01-02")
	err := r.db.WithContext(ctx).
		Model(&model.StudyLog{}).
		Where("user_id = ? AND log_date >= ?", userID, weekAgo).
		Select(
			"TO_CHAR(log_date, 'IYYY-IW') as week, " +
				"SUM(CASE WHEN action IN ('practice','question') THEN 1 ELSE 0 END) as questions_answered, " +
				"SUM(CASE WHEN action = 'interview' THEN 1 ELSE 0 END) as interviews_taken",
		).
		Group("week").
		Order("week DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	stats := make([]*biz.WeeklyStat, 0, len(rows))
	for _, row := range rows {
		stats = append(stats, &biz.WeeklyStat{
			Week:              row.Week,
			QuestionsAnswered: int32(row.QuestionsAnswered),
			InterviewsTaken:   int32(row.InterviewsTaken),
		})
	}
	return stats, nil
}

// GetRecentStudyLogs 查询最近 N 条学习日志。
func (r *growthRepo) GetRecentStudyLogs(ctx context.Context, userID uint64, limit int) ([]*biz.GrowthStudyLog, error) {
	var logs []model.StudyLog
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	if err != nil {
		return nil, err
	}

	result := make([]*biz.GrowthStudyLog, 0, len(logs))
	for _, l := range logs {
		result = append(result, &biz.GrowthStudyLog{
			ID:               int32(l.ID),
			DateKey:          l.DateKey,
			Summary:          l.Summary,
			FocusTaskTitle:   l.FocusTaskTitle,
			CompletedCount:   l.CompletedCount,
			SkippedCount:     l.SkippedCount,
			CompletedTitles:  splitStudyLogTitles(l.CompletedTitles),
			SkippedTitles:    splitStudyLogTitles(l.SkippedTitles),
			LatestActionText: l.LatestActionText,
			UpdatedAt:        l.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return result, nil
}

// splitStudyLogTitles 将存储的 JSON 字符串反序列化为标题列表。
func splitStudyLogTitles(raw string) []string {
	if raw == "" || raw == "[]" {
		return []string{}
	}
	var titles []string
	if err := json.Unmarshal([]byte(raw), &titles); err != nil {
		return []string{}
	}
	return titles
}
