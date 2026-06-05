package service

import (
	"context"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	interviewv1 "makejob/api/makejob/interview/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	"makejob/app/interview/internal/biz"
)

// InterviewService 实现 gRPC InterviewServiceServer
type InterviewService struct {
	interviewv1.UnimplementedInterviewServiceServer
	uc *biz.InterviewUseCase
}

// NewInterviewService 由 Wire 调用
func NewInterviewService(uc *biz.InterviewUseCase) *InterviewService {
	return &InterviewService{uc: uc}
}

func (s *InterviewService) CreateInterview(ctx context.Context, req *interviewv1.CreateInterviewRequest) (*interviewv1.InterviewResponse, error) {
	interview, firstQ, err := s.uc.CreateInterview(ctx, &biz.CreateInterviewRequest{
		UserID:         req.UserId,
		IndustryCode:   req.IndustryCode,
		Difficulty:     req.Difficulty,
		Topics:         req.Topics,
		QuestionCount:  req.QuestionCount,
		InterviewMode:  req.InterviewMode,
		ResumeText:     req.ResumeText,
		JobDescription: req.JobDescription,
		Live2DModelKey: req.Live2DModelKey,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &interviewv1.InterviewResponse{
		Id:        interview.ID,
		Status:    interview.Status,
		CreatedAt: timestamppb.New(interview.CreatedAt),
	}

	if firstQ != nil {
		resp.FirstQuestion = toProtoQuestion(firstQ)
	}

	return resp, nil
}

func (s *InterviewService) GetInterview(ctx context.Context, req *interviewv1.GetInterviewRequest) (*interviewv1.InterviewDetail, error) {
	interview, messages, err := s.uc.GetInterview(ctx, req.InterviewId, req.UserId)
	if err != nil {
		return nil, toGRPCError(err)
	}

	msgs := make([]*interviewv1.InterviewMessage, len(messages))
	for i, m := range messages {
		msgs[i] = &interviewv1.InterviewMessage{
			Id:          m.ID,
			Role:        m.Role,
			Content:     m.Content,
			MessageType: m.MessageType,
			CreatedAt:   timestamppb.New(m.CreatedAt),
		}
	}

	return &interviewv1.InterviewDetail{
		Id:            interview.ID,
		UserId:        interview.UserID,
		IndustryCode:  interview.IndustryCode,
		Status:        interview.Status,
		InterviewMode: interview.InterviewMode,
		Messages:      msgs,
		CreatedAt:     timestamppb.New(interview.CreatedAt),
	}, nil
}

func (s *InterviewService) ListInterviews(ctx context.Context, req *interviewv1.ListInterviewsRequest) (*interviewv1.ListInterviewsResponse, error) {
	page, pageSize := int32(1), int32(20)
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}

	interviews, total, err := s.uc.ListInterviews(ctx, req.UserId, page, pageSize)
	if err != nil {
		return nil, toGRPCError(err)
	}

	items := make([]*interviewv1.InterviewResponse, len(interviews))
	for i, iv := range interviews {
		items[i] = &interviewv1.InterviewResponse{
			Id:        iv.ID,
			Status:    iv.Status,
			CreatedAt: timestamppb.New(iv.CreatedAt),
		}
	}

	return &interviewv1.ListInterviewsResponse{
		Interviews: items,
		PageResult: &sharedv1.PageResult{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

func (s *InterviewService) SubmitAnswer(ctx context.Context, req *interviewv1.SubmitAnswerRequest) (*interviewv1.AnswerFeedback, error) {
	feedback, nextQ, err := s.uc.SubmitAnswer(ctx, req.InterviewId, req.UserId, req.QuestionIndex, req.Answer)
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &interviewv1.AnswerFeedback{
		Score:       feedback.Score,
		IsCorrect:   feedback.IsCorrect,
		Feedback:    feedback.Feedback,
		KeyPoints:   feedback.KeyPoints,
		Suggestions: feedback.Suggestions,
		FollowUp:    feedback.FollowUp,
	}

	if nextQ != nil {
		resp.NextQuestion = toProtoQuestion(nextQ)
	}

	return resp, nil
}

func (s *InterviewService) GetInterviewStats(ctx context.Context, req *interviewv1.UserIDRequest) (*interviewv1.InterviewStats, error) {
	stats, err := s.uc.GetInterviewStats(ctx, req.UserId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &interviewv1.InterviewStats{
		TotalInterviews: stats.TotalInterviews,
		AvgScore:        stats.AvgScore,
	}, nil
}

// --- 辅助函数 ---

func toProtoQuestion(q *biz.InterviewQuestion) *interviewv1.InterviewQuestion {
	if q == nil {
		return nil
	}
	return &interviewv1.InterviewQuestion{
		Question:   q.Question,
		Topic:      q.Topic,
		Difficulty: q.Difficulty,
		Type:       q.Type,
		Hints:      q.Hints,
		Language:   q.Language,
	}
}

func toGRPCError(err error) error {
	// Kratos errors 会被 Kratos gRPC transport 自动转换
	// 对于非 Kratos 错误，包装为 Internal
	if kratosErr.FromError(err) != nil {
		return err
	}
	return kratosErr.InternalServer("INTERNAL", err.Error())
}
