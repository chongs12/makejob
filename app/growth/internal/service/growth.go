package service

import (
	"context"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	growthv1 "makejob/api/makejob/growth/v1"
	"makejob/app/growth/internal/biz"
)

// GrowthService 实现 gRPC GrowthServiceServer
type GrowthService struct {
	growthv1.UnimplementedGrowthServiceServer
	uc *biz.GrowthUseCase
}

// NewGrowthService 创建成长服务
func NewGrowthService(uc *biz.GrowthUseCase) *GrowthService {
	return &GrowthService{uc: uc}
}

// GetGrowthSummary 获取用户成长摘要
func (s *GrowthService) GetGrowthSummary(ctx context.Context, req *growthv1.UserIDRequest) (*growthv1.GrowthSummary, error) {
	summary, err := s.uc.GetGrowthSummary(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	weeklyStats := make([]*growthv1.WeeklyStat, len(summary.WeeklyStats))
	for i, ws := range summary.WeeklyStats {
		weeklyStats[i] = &growthv1.WeeklyStat{
			Week:              ws.Week,
			QuestionsAnswered: ws.QuestionsAnswered,
			InterviewsTaken:   ws.InterviewsTaken,
			AvgScore:          ws.AvgScore,
		}
	}
	weakTopics := make([]*growthv1.TopicWeakness, len(summary.WeakTopics))
	for i, wt := range summary.WeakTopics {
		weakTopics[i] = &growthv1.TopicWeakness{
			Topic:         wt.Topic,
			WeaknessScore: wt.WeaknessScore,
			MistakeCount:  wt.MistakeCount,
		}
	}
	return &growthv1.GrowthSummary{
		TotalStudyDays:  summary.TotalStudyDays,
		TotalQuestions:  summary.TotalQuestions,
		TotalInterviews: summary.TotalInterviews,
		CurrentStreak:   summary.CurrentStreak,
		AvgScore:        summary.AvgScore,
		WeeklyStats:     weeklyStats,
		WeakTopics:      weakTopics,

		PracticeStats:           toPracticeStatsProto(summary.PracticeStats),
		StudyDays:               summary.StudyDays,
		InterviewCount:          summary.InterviewCount,
		CompletedInterviewCount: summary.CompletedInterviewCount,
		AverageInterviewScore:   summary.AverageInterviewScore,
		PlanCount:               summary.PlanCount,
		CurrentPlan:             toCurrentPlanProto(summary.CurrentPlan),
		FocusSignals:            toFocusSignalsProto(summary.FocusSignals),
		TrendSummary:            toTrendSummaryProto(summary.TrendSummary),
		RecentStudyLogs:         toStudyLogsProto(summary.RecentStudyLogs),
		RecentInterviews:        toInterviewSnapshotsProto(summary.RecentInterviews),
		RecentPlans:             toPlanSnapshotsProto(summary.RecentPlans),
	}, nil
}

func toPracticeStatsProto(s *biz.GrowthPracticeStats) *growthv1.GrowthPracticeStats {
	if s == nil {
		return nil
	}
	cats := make([]*growthv1.GrowthCategoryStat, 0, len(s.CategoryStats))
	for _, c := range s.CategoryStats {
		cats = append(cats, &growthv1.GrowthCategoryStat{
			CategoryId:   c.CategoryID,
			CategoryName: c.CategoryName,
			Total:        c.Total,
			Correct:      c.Correct,
			AccuracyRate: c.AccuracyRate,
		})
	}
	return &growthv1.GrowthPracticeStats{
		TotalAnswered: s.TotalAnswered,
		CorrectCount:  s.CorrectCount,
		WrongCount:    s.WrongCount,
		AccuracyRate:  s.AccuracyRate,
		TodayCount:    s.TodayCount,
		StreakDays:    s.StreakDays,
		CategoryStats: cats,
	}
}

func toCurrentPlanProto(p *biz.GrowthCurrentPlan) *growthv1.GrowthCurrentPlan {
	if p == nil {
		return nil
	}
	return &growthv1.GrowthCurrentPlan{
		Id:                     p.ID,
		Title:                  p.Title,
		Status:                 p.Status,
		TotalTasks:             p.TotalTasks,
		CompletedTasks:         p.CompletedTasks,
		Progress:               p.Progress,
		NextTaskTitle:          p.NextTaskTitle,
		NextTaskSource:         p.NextTaskSource,
		NextTaskReason:         p.NextTaskReason,
		NextTaskSourceRef:      p.NextTaskSourceRef,
		NextTaskCollectionHint: p.NextTaskCollectionHint,
	}
}

func toFocusSignalsProto(signals []*biz.GrowthFocusSignal) []*growthv1.GrowthFocusSignal {
	result := make([]*growthv1.GrowthFocusSignal, 0, len(signals))
	for _, s := range signals {
		result = append(result, &growthv1.GrowthFocusSignal{
			FocusTag:                  s.FocusTag,
			TopicCode:                 s.TopicCode,
			TopicTitle:                s.TopicTitle,
			TopicProblemPattern:       s.TopicProblemPattern,
			RelatedQuestionSets:       s.RelatedQuestionSets,
			RecommendedActions:        s.RecommendedActions,
			PrimaryQuestionSet:        s.PrimaryQuestionSet,
			DominantArchivePhase:      s.DominantArchivePhase,
			DominantArchivePhaseLabel: s.DominantArchivePhaseLabel,
			OccurrenceCount:           s.OccurrenceCount,
			ArchiveOccurrenceCount:    s.ArchiveOccurrenceCount,
			InterviewOccurrenceCount:  s.InterviewOccurrenceCount,
			Source:                    s.Source,
			SourceLabel:               s.SourceLabel,
			Reason:                    s.Reason,
		})
	}
	return result
}

func toTrendSummaryProto(t *biz.GrowthTrendSummary) *growthv1.GrowthTrendSummary {
	if t == nil {
		return nil
	}
	return &growthv1.GrowthTrendSummary{
		DominantSource:      t.DominantSource,
		DominantSourceLabel: t.DominantSourceLabel,
		TopFocusTag:         t.TopFocusTag,
		TopTopicCode:        t.TopTopicCode,
		TopTopicTitle:       t.TopTopicTitle,
		Summary:             t.Summary,
	}
}

func toStudyLogsProto(logs []*biz.GrowthStudyLog) []*growthv1.GrowthStudyLog {
	result := make([]*growthv1.GrowthStudyLog, 0, len(logs))
	for _, l := range logs {
		result = append(result, &growthv1.GrowthStudyLog{
			Id:               l.ID,
			DateKey:          l.DateKey,
			Summary:          l.Summary,
			FocusTaskTitle:   l.FocusTaskTitle,
			CompletedCount:   l.CompletedCount,
			SkippedCount:     l.SkippedCount,
			CompletedTitles:  l.CompletedTitles,
			SkippedTitles:    l.SkippedTitles,
			LatestActionText: l.LatestActionText,
			UpdatedAt:        l.UpdatedAt,
		})
	}
	return result
}

func toInterviewSnapshotsProto(items []*biz.GrowthInterviewSnapshot) []*growthv1.GrowthInterviewSnapshot {
	result := make([]*growthv1.GrowthInterviewSnapshot, 0, len(items))
	for _, i := range items {
		result = append(result, &growthv1.GrowthInterviewSnapshot{
			Id:             i.ID,
			Status:         i.Status,
			Score:          i.Score,
			TotalQuestions: i.TotalQuestions,
			CreatedAt:      i.CreatedAt,
			EndedAt:        i.EndedAt,
		})
	}
	return result
}

func toPlanSnapshotsProto(items []*biz.GrowthPlanSnapshot) []*growthv1.GrowthPlanSnapshot {
	result := make([]*growthv1.GrowthPlanSnapshot, 0, len(items))
	for _, p := range items {
		result = append(result, &growthv1.GrowthPlanSnapshot{
			Id:             p.ID,
			Title:          p.Title,
			Status:         p.Status,
			TotalTasks:     p.TotalTasks,
			CompletedTasks: p.CompletedTasks,
			Progress:       p.Progress,
			StartDate:      p.StartDate,
			EndDate:        p.EndDate,
		})
	}
	return result
}

// GetWeeklyFocus 获取本周学习重点
func (s *GrowthService) GetWeeklyFocus(ctx context.Context, req *growthv1.UserIDRequest) (*growthv1.WeeklyFocus, error) {
	resp, err := s.uc.GetWeeklyFocus(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	items := make([]*growthv1.FocusItem, len(resp.Items))
	for i, fi := range resp.Items {
		items[i] = &growthv1.FocusItem{
			Topic:      fi.Topic,
			Source:     fi.Source,
			Weight:     fi.Weight,
			Suggestion: fi.Suggestion,
		}
	}
	return &growthv1.WeeklyFocus{
		Items:   items,
		Summary: resp.Summary,
		Themes:  toWeeklyFocusThemesProto(resp.Themes),
	}, nil
}

func toWeeklyFocusThemesProto(themes []*biz.WeeklyFocusTheme) []*growthv1.WeeklyFocusTheme {
	result := make([]*growthv1.WeeklyFocusTheme, 0, len(themes))
	for _, t := range themes {
		result = append(result, &growthv1.WeeklyFocusTheme{
			Title:                     t.Title,
			Reason:                    t.Reason,
			Source:                    t.Source,
			SourceLabel:               t.SourceLabel,
			FocusTags:                 t.FocusTags,
			TopicCodes:                t.TopicCodes,
			RelatedQuestionSets:       t.RelatedQuestionSets,
			DominantArchivePhase:      t.DominantArchivePhase,
			DominantArchivePhaseLabel: t.DominantArchivePhaseLabel,
			OccurrenceCount:           t.OccurrenceCount,
			InterviewOccurrenceCount:  t.InterviewOccurrenceCount,
			Suggestions:               t.Suggestions,
		})
	}
	return result
}

// SyncStudyLog 同步学习记录
func (s *GrowthService) SyncStudyLog(ctx context.Context, req *growthv1.SyncStudyLogRequest) (*growthv1.StudyLog, error) {
	// FIX G1: proto 字段为秒，biz 字段为分钟，需做单位转换
	durationSeconds := req.GetDurationSeconds()
	durationMinutes := durationSeconds / 60

	// 如果没有指定日期，使用今天
	dateKey := strings.TrimSpace(req.GetDateKey())
	if dateKey == "" {
		dateKey = time.Now().Format("2006-01-02")
	}

	// 如果没有指定 action，使用默认值
	action := strings.TrimSpace(req.GetAction())
	if action == "" {
		action = "study"
	}

	log := &biz.StudyLog{
		UserID:           req.GetUserId(),
		Action:           action,
		RefID:            req.GetRefId(),
		DurationMinutes:  durationMinutes,
		DateKey:          dateKey,
		PlanID:           req.GetPlanId(),
		Summary:          strings.TrimSpace(req.GetSummary()),
		FocusTaskTitle:   strings.TrimSpace(req.GetFocusTaskTitle()),
		CompletedCount:   req.GetCompletedCount(),
		SkippedCount:     req.GetSkippedCount(),
		CompletedTitles:  req.GetCompletedTitles(),
		SkippedTitles:    req.GetSkippedTitles(),
		LatestActionText: strings.TrimSpace(req.GetLatestActionText()),
	}
	saved, err := s.uc.SyncStudyLog(ctx, log)
	if err != nil {
		return nil, err
	}
	return &growthv1.StudyLog{
		Id:               saved.ID,
		UserId:           saved.UserID,
		Action:           saved.Action,
		RefId:            saved.RefID,
		CreatedAt:        timestamppb.New(saved.CreatedAt),
		DateKey:          saved.DateKey,
		Summary:          saved.Summary,
		FocusTaskTitle:   saved.FocusTaskTitle,
		CompletedCount:   saved.CompletedCount,
		SkippedCount:     saved.SkippedCount,
		CompletedTitles:  saved.CompletedTitles,
		SkippedTitles:    saved.SkippedTitles,
		LatestActionText: saved.LatestActionText,
	}, nil
}
