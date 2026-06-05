package service

import (
	"context"

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
	}, nil
}

func (s *GrowthService) GetWeeklyFocus(ctx context.Context, req *growthv1.UserIDRequest) (*growthv1.WeeklyFocus, error) {
	focusItems, err := s.uc.GetWeeklyFocus(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	items := make([]*growthv1.FocusItem, len(focusItems))
	for i, fi := range focusItems {
		items[i] = &growthv1.FocusItem{
			Topic:      fi.Topic,
			Source:     fi.Source,
			Weight:     fi.Weight,
			Suggestion: fi.Suggestion,
		}
	}
	return &growthv1.WeeklyFocus{
		Items: items,
	}, nil
}

func (s *GrowthService) SyncStudyLog(ctx context.Context, req *growthv1.SyncStudyLogRequest) (*growthv1.StudyLog, error) {
	log, err := s.uc.SyncStudyLog(ctx, req.UserId, req.Action, req.RefId, req.DurationSeconds)
	if err != nil {
		return nil, err
	}
	return &growthv1.StudyLog{
		Id:        log.ID,
		UserId:    log.UserID,
		Action:    log.Action,
		RefId:     log.RefID,
		CreatedAt: timestamppb.New(log.CreatedAt),
	}, nil
}
