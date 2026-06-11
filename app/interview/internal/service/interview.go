package service

import (
	"context"
	"strings"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	interviewv1 "makejob/api/makejob/interview/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	"makejob/app/interview/internal/biz"
	"makejob/pkg/auth"
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
	userID := resolveUserID(ctx, req.UserId)
	interview, firstQ, err := s.uc.CreateInterview(ctx, &biz.CreateInterviewRequest{
		UserID:         userID,
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
		Id:         interview.ID,
		InterviewId: interview.ID,
		Status:     interview.Status,
		CreatedAt:  timestamppb.New(interview.CreatedAt),
	}

	if firstQ != nil {
		resp.FirstQuestion = toProtoQuestion(firstQ)
	}

	return resp, nil
}

func (s *InterviewService) GetInterview(ctx context.Context, req *interviewv1.GetInterviewRequest) (*interviewv1.InterviewDetail, error) {
	userID := resolveUserID(ctx, req.UserId)
	interview, messages, err := s.uc.GetInterview(ctx, req.InterviewId, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}

	msgs := make([]*interviewv1.InterviewMessage, len(messages))
	var currentQuestion *interviewv1.InterviewQuestion
	for i, m := range messages {
		msgs[i] = &interviewv1.InterviewMessage{
			Id:          m.ID,
			Role:        m.Role,
			Content:     biz.NormalizeMessageContent(m),
			MessageType: m.MessageType,
			CreatedAt:   timestamppb.New(m.CreatedAt),
		}
		// AI 出题消息携带结构化题目，供前端进行页恢复当前题
		if m.Role == "assistant" {
			if q := biz.DecodeQuestionContent(m.Content); q != nil && q.Question != "" {
				pq := toProtoQuestion(q)
				msgs[i].Question = pq
				currentQuestion = pq
			}
		}
	}

	detail := &interviewv1.InterviewDetail{
		Id:             interview.ID,
		UserId:         interview.UserID,
		IndustryCode:   interview.IndustryCode,
		Status:         interview.Status,
		InterviewMode:  interview.InterviewMode,
		Messages:       msgs,
		Score:          int32(interview.OverallScore),
		TotalQuestions: interview.QuestionCount,
		CurrentQuestion: currentQuestion,
		StartedAt:      timestamppb.New(interview.CreatedAt),
		CreatedAt:      timestamppb.New(interview.CreatedAt),
	}
	if interview.FinishedAt != nil {
		detail.EndedAt = timestamppb.New(*interview.FinishedAt)
	}
	return detail, nil
}

func (s *InterviewService) ListInterviews(ctx context.Context, req *interviewv1.ListInterviewsRequest) (*interviewv1.ListInterviewsResponse, error) {
	page, pageSize := int32(1), int32(20)
	if req.Page != nil {
		page = req.Page.Page
		pageSize = req.Page.PageSize
	}

	interviews, total, err := s.uc.ListInterviews(ctx, resolveUserID(ctx, req.UserId), page, pageSize)
	if err != nil {
		return nil, toGRPCError(err)
	}

	items := make([]*interviewv1.InterviewResponse, len(interviews))
	for i, iv := range interviews {
		item := &interviewv1.InterviewResponse{
			Id:             iv.ID,
			InterviewId:    iv.ID,
			Status:         iv.Status,
			Score:          int32(iv.OverallScore),
			TotalQuestions: iv.QuestionCount,
			StartedAt:      timestamppb.New(iv.CreatedAt),
			CreatedAt:      timestamppb.New(iv.CreatedAt),
		}
		if iv.FinishedAt != nil {
			item.EndedAt = timestamppb.New(*iv.FinishedAt)
		}
		items[i] = item
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
	feedback, nextQ, err := s.uc.SubmitAnswer(ctx, req.InterviewId, resolveUserID(ctx, req.UserId), req.QuestionIndex, req.Answer)
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
	stats, err := s.uc.GetInterviewStats(ctx, resolveUserID(ctx, req.UserId))
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &interviewv1.InterviewStats{
		TotalInterviews: stats.TotalInterviews,
		AvgScore:        stats.AvgScore,
	}, nil
}

// GetAdminInterviewStats 管理后台获取全站面试总量。
func (s *InterviewService) GetAdminInterviewStats(ctx context.Context, _ *interviewv1.GetAdminInterviewStatsRequest) (*interviewv1.AdminInterviewStatsResponse, error) {
	totalInterviews, err := s.uc.GetAdminInterviewStats(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &interviewv1.AdminInterviewStatsResponse{TotalInterviews: totalInterviews}, nil
}

// GetNextQuestion 获取下一道面试题目
func (s *InterviewService) GetNextQuestion(ctx context.Context, req *interviewv1.GetNextQuestionRequest) (*interviewv1.NextQuestionResponse, error) {
	interview, question, err := s.uc.GetNextQuestion(ctx, req.InterviewId, resolveUserID(ctx, req.UserId))
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &interviewv1.NextQuestionResponse{
		Question: toProtoQuestion(question),
		IsLast:   question == nil,
	}
	if interview != nil {
		resp.QuestionNo = interview.CurrentIndex + 1
		resp.IsLast = question == nil || interview.CurrentIndex+1 >= interview.QuestionCount
	}
	return resp, nil
}

// FinishInterview 结束面试并触发报告生成
func (s *InterviewService) FinishInterview(ctx context.Context, req *interviewv1.FinishInterviewRequest) (*interviewv1.InterviewReport, error) {
	err := s.uc.FinishInterview(ctx, req.InterviewId, resolveUserID(ctx, req.UserId))
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &interviewv1.InterviewReport{
		InterviewId: req.InterviewId,
		Status:      "generating",
	}, nil
}

// GetReport 获取面试报告
func (s *InterviewService) GetReport(ctx context.Context, req *interviewv1.GetReportRequest) (*interviewv1.InterviewReport, error) {
	result, err := s.uc.GetReport(ctx, req.InterviewId, resolveUserID(ctx, req.UserId))
	if err != nil {
		return nil, toGRPCError(err)
	}

	// 构造 proto 响应
	report := &interviewv1.InterviewReport{
		InterviewId:     req.InterviewId,
		Status:          result.Status,
		OverallScore:    result.OverallScore,
		TotalQuestions:  result.TotalQuestions,
		CorrectCount:    result.CorrectCount,
		DimensionScores: result.DimensionScores,
		Summary:         result.Summary,
	}
	if result.Strengths != nil {
		report.Strengths = result.Strengths
	}
	if result.Weaknesses != nil {
		report.Weaknesses = result.Weaknesses
	}
	if result.Suggestions != nil {
		report.Suggestions = result.Suggestions
	}

	// 转换编程诊断数据
	if len(result.CodingDiagnostics) > 0 {
		diagnostics := make([]*interviewv1.CodingDiagnosis, len(result.CodingDiagnostics))
		for i, d := range result.CodingDiagnostics {
			diagnostics[i] = &interviewv1.CodingDiagnosis{
				QuestionIndex:   d.QuestionIndex,
				Language:        d.Language,
				Topic:           d.Topic,
				Score:           d.Score,
				MistakeTags:     d.MistakeTags,
				StrengthTags:    d.StrengthTags,
				EvidenceSummary: d.EvidenceSummary,
				Evidence:        splitEvidenceSummary(d.EvidenceSummary),
				ProcessSummary:  d.EvidenceSummary,
				Suggestions:     d.Suggestions,
			}
		}
		report.CodingDiagnostics = diagnostics
	}

	// 对齐前端 InterviewReportResponse 的状态包装字段
	report.TaskStatus = result.Status

	return report, nil
}

// splitEvidenceSummary 将证据摘要文本拆分为前端期望的字符串数组（按换行/分号/中文分号切分）。
func splitEvidenceSummary(summary string) []string {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return []string{}
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '\n' || r == ';' || r == '；'
	})
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return []string{trimmed}
	}
	return result
}

// SubmitCodingAnswer 提交编程题答案
func (s *InterviewService) SubmitCodingAnswer(ctx context.Context, req *interviewv1.SubmitCodingRequest) (*interviewv1.CodingResult, error) {
	result, err := s.uc.SubmitCodingAnswer(ctx, req.InterviewId, resolveUserID(ctx, req.UserId), req.QuestionIndex, req.Language, req.Code)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &interviewv1.CodingResult{
		Passed:          result.Passed,
		TestCasesPassed: result.TestCasesPassed,
		TotalTestCases:  result.TotalTestCases,
		Output:          result.Output,
		Error:           result.ErrorMsg,
		Feedback: &interviewv1.AnswerFeedback{
			Score:       result.AIScore,
			IsCorrect:   result.AIScore >= 60,
			Feedback:    result.AIFeedback,
			KeyPoints:   nil,
			Suggestions: "",
		},
	}, nil
}

// IsRealtimeInterview 查询面试是否为实时模式
func (s *InterviewService) IsRealtimeInterview(ctx context.Context, req *interviewv1.IsRealtimeRequest) (*interviewv1.IsRealtimeResponse, error) {
	isRealtime, err := s.uc.IsRealtimeInterview(ctx, req.InterviewId, resolveUserID(ctx, 0))
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &interviewv1.IsRealtimeResponse{
		IsRealtime: isRealtime,
	}, nil
}

// GetRealtimeContext 获取实时面试上下文
func (s *InterviewService) GetRealtimeContext(ctx context.Context, req *interviewv1.GetRealtimeRequest) (*interviewv1.RealtimeInterviewContext, error) {
	result, err := s.uc.GetRealtimeContext(ctx, req.InterviewId, resolveUserID(ctx, req.UserId))
	if err != nil {
		return nil, toGRPCError(err)
	}

	msgs := make([]*interviewv1.InterviewMessage, len(result.History))
	for i, m := range result.History {
		msgs[i] = &interviewv1.InterviewMessage{
			Id:          m.ID,
			Role:        m.Role,
			Content:     biz.NormalizeMessageContent(m),
			MessageType: m.MessageType,
			CreatedAt:   timestamppb.New(m.CreatedAt),
		}
	}

	return &interviewv1.RealtimeInterviewContext{
		InterviewId:   result.InterviewID,
		IndustryCode:  result.IndustryCode,
		Difficulty:    result.Difficulty,
		History:       msgs,
		CurrentTopic:  result.CurrentTopic,
		QuestionIndex: result.QuestionIndex,
	}, nil
}

// BindRealtimeDialog 绑定实时对话 ID
func (s *InterviewService) BindRealtimeDialog(ctx context.Context, req *interviewv1.BindDialogRequest) (*emptypb.Empty, error) {
	if req.DialogId == "" {
		return nil, toGRPCError(kratosErr.BadRequest("INVALID_DIALOG_ID", "dialog_id 不能为空"))
	}
	if err := s.uc.BindRealtimeDialog(ctx, req.InterviewId, resolveUserID(ctx, 0), req.DialogId); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// AppendRealtimeUserAnswer 追加实时面试用户回答
func (s *InterviewService) AppendRealtimeUserAnswer(ctx context.Context, req *interviewv1.AppendAnswerRequest) (*emptypb.Empty, error) {
	if err := s.uc.AppendRealtimeUserAnswer(ctx, req.InterviewId, resolveUserID(ctx, 0), req.AnswerText); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// AppendRealtimeAssistantReply 追加实时面试 AI 回复
func (s *InterviewService) AppendRealtimeAssistantReply(ctx context.Context, req *interviewv1.AppendReplyRequest) (*interviewv1.AppendReplyResponse, error) {
	shouldEnd, nextQ, err := s.uc.AppendRealtimeAssistantReply(ctx, req.InterviewId, resolveUserID(ctx, 0), req.ReplyText)
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &interviewv1.AppendReplyResponse{
		ShouldEnd: shouldEnd,
	}
	if nextQ != nil {
		resp.NextQuestion = toProtoQuestion(nextQ)
	}
	return resp, nil
}

// --- 辅助函数 ---

func toProtoQuestion(q *biz.InterviewQuestion) *interviewv1.InterviewQuestion {
	if q == nil {
		return nil
	}
	pq := &interviewv1.InterviewQuestion{
		Question:   q.Question,
		Topic:      q.Topic,
		Difficulty: q.Difficulty,
		Type:       q.Type,
		Hints:      q.Hints,
		Language:   q.Language,
	}
	if q.StarterCode != "" {
		pq.StarterCode = q.StarterCode
	}
	if q.EditorMode != "" {
		pq.EditorMode = q.EditorMode
	}
	if q.EvalMode != "" {
		pq.EvaluationMode = q.EvalMode
	}
	return pq
}

// resolveUserID 优先使用认证上下文中的用户 ID，避免信任请求体透传字段。
func resolveUserID(ctx context.Context, requested uint64) uint64 {
	if userID := auth.GetUserIDFromContext(ctx); userID != 0 {
		return userID
	}
	return requested
}

func toGRPCError(err error) error {
	// Kratos errors 会被 Kratos gRPC transport 自动转换
	// 对于非 Kratos 错误，包装为 Internal
	if kratosErr.FromError(err) != nil {
		return err
	}
	return kratosErr.InternalServer("INTERNAL", err.Error())
}
