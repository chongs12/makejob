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
	}, nil
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
	}, nil
}

// SyncStudyLog 同步学习记录
func (s *GrowthService) SyncStudyLog(ctx context.Context, req *growthv1.SyncStudyLogRequest) (*growthv1.StudyLog, error) {
	// FIX G1: proto 字段为秒，biz 字段为分钟，需做单位转换
	durationSeconds := req.GetDurationSeconds()
	durationMinutes := durationSeconds / 60

	log := &biz.StudyLog{
		UserID:          req.GetUserId(),
		Action:          req.GetAction(),
		RefID:           req.GetRefId(),
		DurationMinutes: durationMinutes,
	}
	saved, err := s.uc.SyncStudyLog(ctx, log)
	if err != nil {
		return nil, err
	}
	return &growthv1.StudyLog{
		Id:        saved.ID,
		UserId:    saved.UserID,
		Action:    saved.Action,
		RefId:     saved.RefID,
		CreatedAt: timestamppb.New(saved.CreatedAt),
	}, nil
}
