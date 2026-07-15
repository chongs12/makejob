package service

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	aiv1 "makejob/api/makejob/ai/v1"
	"makejob/app/ai_gateway/internal/biz"
)

// AIGatewayService AI 网关 gRPC 服务实现，聚合全部 AI 场景用例
type AIGatewayService struct {
	aiv1.UnimplementedAIServiceServer
	interviewUC        *biz.InterviewAgentUseCase
	interviewSessionUC *biz.InterviewSessionUseCase
	planUC             *biz.PlanAgentUseCase
	companionUC        *biz.CompanionAgentUseCase
	quizUC             *biz.QuizAnalyzerUseCase
	resumeUC           *biz.ResumeParserUseCase
	live2dUC           *biz.Live2DDirectorUseCase
	adminUC            *biz.AdminUseCase
}

// NewAIGatewayService 创建 AI 网关 gRPC 服务
func NewAIGatewayService(
	interviewUC *biz.InterviewAgentUseCase,
	interviewSessionUC *biz.InterviewSessionUseCase,
	planUC *biz.PlanAgentUseCase,
	companionUC *biz.CompanionAgentUseCase,
	quizUC *biz.QuizAnalyzerUseCase,
	resumeUC *biz.ResumeParserUseCase,
	live2dUC *biz.Live2DDirectorUseCase,
	adminUC *biz.AdminUseCase,
) *AIGatewayService {
	return &AIGatewayService{
		interviewUC:        interviewUC,
		interviewSessionUC: interviewSessionUC,
		planUC:             planUC,
		companionUC:        companionUC,
		quizUC:             quizUC,
		resumeUC:           resumeUC,
		live2dUC:           live2dUC,
		adminUC:            adminUC,
	}
}

// InterviewAgent 面试出题/反馈 handler
func (s *AIGatewayService) InterviewAgent(ctx context.Context, req *aiv1.InterviewAgentRequest) (*aiv1.InterviewAgentResponse, error) {
	history := toBizMessages(req.History)
	result, err := s.interviewUC.GenerateQuestion(
		ctx,
		req.IndustryCode,
		req.Difficulty,
		req.UserAnswer,
		req.ResumeText,
		req.JobDescription,
		history,
		req.QuestionIndex,
		req.Topics,
		req.InterviewType,
	)
	if err != nil {
		return nil, err
	}
	return &aiv1.InterviewAgentResponse{
		Question:      result.Question,
		Topic:         result.Topic,
		Difficulty:    result.Difficulty,
		Type:          result.Type,
		Hints:         result.Hints,
		Feedback:      result.Feedback,
		Score:         result.Score,
		ShouldEnd:     result.ShouldEnd,
		Live2DEmotion: result.Live2DEmotion,
		Live2DAction:  result.Live2DAction,
	}, nil
}

// StartInterview 开始面试会话（对齐单体 InterviewAgent.StartInterview）
func (s *AIGatewayService) StartInterview(ctx context.Context, req *aiv1.StartInterviewRequest) (*aiv1.StartInterviewResponse, error) {
	result, err := s.interviewSessionUC.StartInterview(ctx, &biz.StartInterviewRequest{
		InterviewID:    req.InterviewId,
		IndustryCode:   req.IndustryCode,
		Difficulty:     req.Difficulty,
		QuestionCount:  req.QuestionCount,
		ResumeText:     req.ResumeText,
		JobDescription: req.JobDescription,
		InterviewMode:  req.InterviewMode,
		Topics:         req.Topics,
		InterviewType:  req.InterviewType,
	})
	if err != nil {
		return nil, err
	}
	return &aiv1.StartInterviewResponse{
		SessionId:  result.SessionID,
		Question:   result.Question,
		Topic:      result.Topic,
		Difficulty: result.Difficulty,
		Type:       result.Type,
		Hints:      result.Hints,
	}, nil
}

// EvaluateAnswer 评估用户答案（对齐单体 InterviewAgent.EvaluateAnswer + EnhanceFeedbackPrompt）
func (s *AIGatewayService) EvaluateAnswer(ctx context.Context, req *aiv1.EvaluateAnswerRequest) (*aiv1.EvaluateAnswerResponse, error) {
	result, err := s.interviewSessionUC.EvaluateAnswer(ctx, &biz.EvaluateAnswerRequest{
		SessionId:     req.SessionId,
		QuestionIndex: req.QuestionIndex,
		Answer:        req.Answer,
		RAGContext:    req.RagContext,
	})
	if err != nil {
		return nil, err
	}
	return &aiv1.EvaluateAnswerResponse{
		Score:      result.Score,
		IsCorrect:  result.IsCorrect,
		Feedback:   result.Feedback,
		KeyPoints:  result.KeyPoints,
		Suggestions: result.Suggestions,
		FollowUp:   result.FollowUp,
	}, nil
}

// GetNextQuestionSession 获取下一道题（对齐单体 InterviewAgent.GetNextQuestion + EnhanceQuestionPrompt）
func (s *AIGatewayService) GetNextQuestionSession(ctx context.Context, req *aiv1.GetNextQuestionSessionRequest) (*aiv1.GetNextQuestionSessionResponse, error) {
	result, err := s.interviewSessionUC.GetNextQuestion(ctx, &biz.GetNextQuestionSessionRequest{
		SessionId:  req.SessionId,
		RAGContext: req.RagContext,
	})
	if err != nil {
		return nil, err
	}
	return &aiv1.GetNextQuestionSessionResponse{
		Question:   result.Question,
		Topic:      result.Topic,
		Difficulty: result.Difficulty,
		Type:       result.Type,
		Hints:      result.Hints,
		HasNext:    result.HasNext,
	}, nil
}

// GenerateInterviewReport 生成面试报告（对齐单体 InterviewAgent.GenerateReport）
func (s *AIGatewayService) GenerateInterviewReport(ctx context.Context, req *aiv1.GenerateInterviewReportRequest) (*aiv1.GenerateInterviewReportResponse, error) {
	result, err := s.interviewSessionUC.GenerateReport(ctx, &biz.GenerateInterviewReportRequest{
		SessionId: req.SessionId,
	})
	if err != nil {
		return nil, err
	}
	return &aiv1.GenerateInterviewReportResponse{
		OverallScore:    result.OverallScore,
		Summary:         result.Summary,
		DimensionScores: result.DimensionScores,
		Strengths:       result.Strengths,
		Weaknesses:      result.Weaknesses,
		Suggestions:     result.Suggestions,
		AiFeedback:      result.AiFeedback,
	}, nil
}

// GenerateReportFromHistory 从对话历史生成报告（不依赖 session，供实时面试使用）
func (s *AIGatewayService) GenerateReportFromHistory(ctx context.Context, req *aiv1.GenerateReportFromHistoryRequest) (*aiv1.GenerateInterviewReportResponse, error) {
	history := make([]biz.Message, len(req.History))
	for i, m := range req.History {
		history[i] = biz.Message{Role: m.Role, Content: m.Content}
	}
	result, err := s.interviewSessionUC.GenerateReportFromHistory(ctx, &biz.GenerateReportFromHistoryRequest{
		History:        history,
		IndustryCode:   req.IndustryCode,
		Difficulty:     req.Difficulty,
		TotalQuestions: req.TotalQuestions,
	})
	if err != nil {
		return nil, err
	}
	return &aiv1.GenerateInterviewReportResponse{
		OverallScore: result.OverallScore,
		Summary:      result.Summary,
		Strengths:    result.Strengths,
		Weaknesses:   result.Weaknesses,
		Suggestions:  result.Suggestions,
	}, nil
}

// GenerateKnowledgeReport 知识点专项面试报告生成 handler
func (s *AIGatewayService) GenerateKnowledgeReport(ctx context.Context, req *aiv1.GenerateKnowledgeReportRequest) (*aiv1.GenerateKnowledgeReportResponse, error) {
	history := make([]biz.Message, len(req.History))
	for i, m := range req.History {
		history[i] = biz.Message{Role: m.Role, Content: m.Content}
	}
	result, err := s.interviewSessionUC.GenerateKnowledgeReport(ctx, &biz.GenerateKnowledgeReportRequest{
		History:         history,
		KnowledgeTopics: req.KnowledgeTopics,
		Difficulty:      req.Difficulty,
		TotalQuestions:  req.TotalQuestions,
	})
	if err != nil {
		return nil, err
	}
	return &aiv1.GenerateKnowledgeReportResponse{
		ReportJson:   result.ReportJSON,
		OverallScore: result.OverallScore,
		Rating:       result.Rating,
	}, nil
}

// GenerateJobReport 岗位求职面试报告生成 handler
func (s *AIGatewayService) GenerateJobReport(ctx context.Context, req *aiv1.GenerateJobReportRequest) (*aiv1.GenerateJobReportResponse, error) {
	history := make([]biz.Message, len(req.History))
	for i, m := range req.History {
		history[i] = biz.Message{Role: m.Role, Content: m.Content}
	}
	result, err := s.interviewSessionUC.GenerateJobReport(ctx, &biz.GenerateJobReportRequest{
		History:          history,
		ResumeText:       req.ResumeText,
		ResumeParsedJSON: req.ResumeParsedJson,
		JobDescription:   req.JobDescription,
		IndustryCode:     req.IndustryCode,
		Difficulty:       req.Difficulty,
		TotalQuestions:   req.TotalQuestions,
	})
	if err != nil {
		return nil, err
	}
	return &aiv1.GenerateJobReportResponse{
		ReportJson:         result.ReportJSON,
		OverallScore:       result.OverallScore,
		Rating:             result.Rating,
		HireRecommendation: result.HireRecommendation,
	}, nil
}

// EndInterviewSession 结束面试会话（对齐单体 InterviewAgent.EndInterview）
func (s *AIGatewayService) EndInterviewSession(ctx context.Context, req *aiv1.EndInterviewSessionRequest) (*aiv1.EndInterviewSessionResponse, error) {
	result, err := s.interviewSessionUC.EndInterviewSession(ctx, &biz.EndInterviewSessionRequest{
		SessionId: req.SessionId,
	})
	if err != nil {
		return nil, err
	}
	return &aiv1.EndInterviewSessionResponse{
		Success: result.Success,
	}, nil
}

// PlanAgent 学习计划生成 handler
func (s *AIGatewayService) PlanAgent(ctx context.Context, req *aiv1.PlanAgentRequest) (*aiv1.PlanAgentResponse, error) {
	result, err := s.planUC.GeneratePlan(
		ctx,
		req.IndustryCode,
		req.Goal,
		req.DailyHours,
		req.WeakTopics,
		req.RecentActivities,
	)
	if err != nil {
		return nil, err
	}
	return &aiv1.PlanAgentResponse{
		PlanTitle: result.PlanTitle,
		Tasks:     toProtoPlanTasks(result.Tasks),
		Summary:   result.Summary,
	}, nil
}

// AdjustPlan 学习计划调整 handler
func (s *AIGatewayService) AdjustPlan(ctx context.Context, req *aiv1.AdjustPlanRequest) (*aiv1.AdjustPlanResponse, error) {
	result, err := s.planUC.AdjustPlan(
		ctx,
		req.PlanId,
		req.IndustryCode,
		req.CurrentPhase,
		req.GoalDescription,
		req.DailyHours,
		req.CompletedTasks,
		req.WeakTopics,
		req.Performance,
	)
	if err != nil {
		return nil, err
	}
	return &aiv1.AdjustPlanResponse{
		PlanTitle:    result.PlanTitle,
		Tasks:        toProtoPlanTasks(result.Tasks),
		Summary:      result.Summary,
		Phase:        result.Phase,
		PhaseGoal:    result.PhaseGoal,
		DurationDays: result.DurationDays,
	}, nil
}

// GetStudySuggestion 学习建议 handler
func (s *AIGatewayService) GetStudySuggestion(ctx context.Context, req *aiv1.GetStudySuggestionRequest) (*aiv1.GetStudySuggestionResponse, error) {
	suggestion, err := s.planUC.GetStudySuggestion(
		ctx,
		req.IndustryCode,
		req.Level,
		req.GoalDescription,
		req.DailyHours,
		req.DurationDays,
		req.WeakTopics,
		req.StrongTopics,
	)
	if err != nil {
		return nil, err
	}
	return &aiv1.GetStudySuggestionResponse{
		Suggestion: suggestion,
	}, nil
}

// CompanionAgent AI 陪伴聊天 handler
func (s *AIGatewayService) CompanionAgent(ctx context.Context, req *aiv1.CompanionAgentRequest) (*aiv1.CompanionAgentResponse, error) {
	result, err := s.companionUC.Chat(
		ctx,
		req.Message,
		req.ContextType,
		req.GetUsername(),
		req.RecentTopics,
		req.GetConversationStateJson(),
	)
	if err != nil {
		return nil, err
	}

	// 构建 Live2DDirective proto（如果有）
	var live2dDirective *aiv1.Live2DDirective
	if result.Live2DDirective != nil {
		live2dDirective = toProtoLive2DDirective(result.Live2DDirective)
	}

	return &aiv1.CompanionAgentResponse{
		Reply:             result.Reply,
		Emotion:           result.Emotion,
		Suggestions:       result.Suggestions,
		Action:            result.Action,
		SuggestedActions:  toProtoSuggestedActions(result.SuggestedActions),
		Live2DDirective:   live2dDirective,
		InlineTriggers:    toProtoInlineTriggers(result.InlineTriggers),
		Intent:            toProtoIntentInfo(result.Intent),
		PendingAction:     toProtoPendingAction(result.PendingAction),
		ConversationState: toProtoConversationState(result.ConversationState),
	}, nil
}

// toProtoSuggestedActions 将 biz SuggestedActionItem 列表转换为 proto SuggestedAction 列表。
func toProtoSuggestedActions(actions []biz.SuggestedActionItem) []*aiv1.SuggestedAction {
	if len(actions) == 0 {
		return nil
	}
	protos := make([]*aiv1.SuggestedAction, 0, len(actions))
	for _, a := range actions {
		protos = append(protos, &aiv1.SuggestedAction{
			Type:   a.Type,
			Target: a.Target,
			Params: a.Params,
		})
	}
	return protos
}

// toProtoInlineTriggers 将 biz InlineTriggerItem 列表转换为 proto InlineTrigger 列表。
func toProtoInlineTriggers(items []biz.InlineTriggerItem) []*aiv1.InlineTrigger {
	if len(items) == 0 {
		return nil
	}
	protos := make([]*aiv1.InlineTrigger, 0, len(items))
	for _, it := range items {
		protos = append(protos, &aiv1.InlineTrigger{
			Keyword:      it.Keyword,
			ActionType:   it.ActionType,
			Target:       it.Target,
			PositionHint: it.PositionHint,
		})
	}
	return protos
}

// toProtoIntentInfo 将 biz IntentInfo 指针转换为 proto IntentInfo（nullable）。
func toProtoIntentInfo(info *biz.IntentInfo) *aiv1.IntentInfo {
	if info == nil {
		return nil
	}
	return &aiv1.IntentInfo{
		Type:       info.Type,
		Confidence: info.Confidence,
		Stage:      info.Stage,
	}
}

// toProtoPendingAction 将 biz PendingAction 指针转换为 proto PendingAction（nullable）。
func toProtoPendingAction(action *biz.PendingAction) *aiv1.PendingAction {
	if action == nil {
		return nil
	}
	return &aiv1.PendingAction{
		Type:        action.Type,
		Ready:       action.Ready,
		Params:      action.Params,
		MissingInfo: action.MissingInfo,
	}
}

// toProtoConversationState 将 biz ConversationState 指针转换为 proto ConversationState（nullable）。
func toProtoConversationState(state *biz.ConversationState) *aiv1.ConversationState {
	if state == nil {
		return nil
	}
	return &aiv1.ConversationState{
		Phase:           state.Phase,
		CollectedParams: state.CollectedParams,
	}
}

// GetGreeting 本地欢迎语 handler（对齐单体 CompanionAgent.GetGreeting）
func (s *AIGatewayService) GetGreeting(ctx context.Context, req *aiv1.GetGreetingRequest) (*aiv1.GetGreetingResponse, error) {
	result, err := s.companionUC.GetGreeting(ctx, req.GetLevel(), req.GetTimeOfDay())
	if err != nil {
		return nil, err
	}
	return &aiv1.GetGreetingResponse{
		Content: result.Reply,
		Emotion: result.Emotion,
		Action:  result.Action,
	}, nil
}

// GetEncouragement 本地鼓励语 handler（对齐单体 CompanionAgent.GetEncouragement）
func (s *AIGatewayService) GetEncouragement(ctx context.Context, req *aiv1.GetEncouragementRequest) (*aiv1.GetEncouragementResponse, error) {
	result, err := s.companionUC.GetEncouragement(ctx, req.GetAchievement())
	if err != nil {
		return nil, err
	}
	return &aiv1.GetEncouragementResponse{
		Content: result.Reply,
		Emotion: result.Emotion,
		Action:  result.Action,
	}, nil
}

// toProtoLive2DDirective 将 biz Live2DDirectiveResult 转换为 proto Live2DDirective
func toProtoLive2DDirective(result *biz.Live2DDirectiveResult) *aiv1.Live2DDirective {
	expressionMix := make([]*aiv1.Live2DDirectiveExpressionLayer, 0, len(result.ExpressionMix))
	for _, e := range result.ExpressionMix {
		expressionMix = append(expressionMix, &aiv1.Live2DDirectiveExpressionLayer{
			Key:    e.Key,
			Weight: float32(e.Weight),
		})
	}
	parameterOverrides := make([]*aiv1.Live2DDirectiveParameterOverride, 0, len(result.ParameterOverrides))
	for _, p := range result.ParameterOverrides {
		parameterOverrides = append(parameterOverrides, &aiv1.Live2DDirectiveParameterOverride{
			Id:    p.ID,
			Value: float32(p.Value),
		})
	}
	mouthOpen := float32(0)
	if result.MouthOpen != nil {
		mouthOpen = float32(*result.MouthOpen)
	}
	return &aiv1.Live2DDirective{
		Emotion:            result.Emotion,
		Action:             result.Action,
		Reply:              result.Reply,
		MotionKey:          result.MotionKey,
		MotionGroup:        result.MotionGroup,
		MotionPriority:     result.MotionPriority,
		MotionDurationMs:   int32(result.MotionDurationMS),
		Intensity:          float32(result.Intensity),
		DurationMs:         int32(result.DurationMS),
		MouthOpen:          mouthOpen,
		Source:             result.Source,
		ExpressionMix:      expressionMix,
		ParameterOverrides: parameterOverrides,
	}
}

// QuizAnalyzer 答题分析评估 handler
func (s *AIGatewayService) QuizAnalyzer(ctx context.Context, req *aiv1.QuizAnalyzerRequest) (*aiv1.QuizAnalyzerResponse, error) {
	result, err := s.quizUC.Analyze(
		ctx,
		req.Question,
		req.Answer,
		req.Topic,
		req.Difficulty,
		req.QuestionType,
	)
	if err != nil {
		return nil, err
	}
	return &aiv1.QuizAnalyzerResponse{
		Score:         result.Score,
		IsCorrect:     result.IsCorrect,
		Feedback:      result.Feedback,
		KeyPoints:     result.KeyPoints,
		Suggestions:   result.Suggestions,
		CorrectAnswer: result.CorrectAnswer,
	}, nil
}

// AnalyzeCode 代码分析 handler
func (s *AIGatewayService) AnalyzeCode(ctx context.Context, req *aiv1.AnalyzeCodeRequest) (*aiv1.AnalyzeCodeResponse, error) {
	result, err := s.quizUC.AnalyzeCode(
		ctx,
		req.Code,
		req.Language,
		req.Question,
	)
	if err != nil {
		return nil, err
	}
	return &aiv1.AnalyzeCodeResponse{
		IsCorrect:       result.IsCorrect,
		Score:           result.Score,
		Feedback:        result.Feedback,
		Issues:          result.Issues,
		Improvements:    result.Improvements,
		MistakeTags:     result.MistakeTags,
		StrengthTags:    result.StrengthTags,
		TimeComplexity:  result.TimeComplexity,
		SpaceComplexity: result.SpaceComplexity,
	}, nil
}

// DiagnoseInterviewCoding 编程面试诊断 handler
func (s *AIGatewayService) DiagnoseInterviewCoding(ctx context.Context, req *aiv1.DiagnoseInterviewCodingRequest) (*aiv1.DiagnoseInterviewCodingResponse, error) {
	processEvents := make([]biz.CodingProcessEvent, 0, len(req.ProcessEvents))
	for _, event := range req.ProcessEvents {
		processEvents = append(processEvents, biz.CodingProcessEvent{
			Type:        event.Type,
			TimestampMS: event.TimestampMs,
			PayloadJSON: event.PayloadJson,
		})
	}

	result, err := s.quizUC.DiagnoseInterviewCoding(
		ctx,
		req.Question,
		req.Language,
		req.FinalCode,
		req.FinalAnswer,
		processEvents,
	)
	if err != nil {
		return nil, err
	}
	return &aiv1.DiagnoseInterviewCodingResponse{
		Score:          result.Score,
		MistakeTags:    result.MistakeTags,
		StrengthTags:   result.StrengthTags,
		Evidence:       result.Evidence,
		Suggestions:    result.Suggestions,
		ProcessSummary: result.ProcessSummary,
	}, nil
}

// ExplainAnswer 答案解析 handler
func (s *AIGatewayService) ExplainAnswer(ctx context.Context, req *aiv1.ExplainAnswerRequest) (*aiv1.ExplainAnswerResponse, error) {
	explanation, err := s.quizUC.ExplainAnswer(
		ctx,
		req.QuestionTitle,
		req.QuestionContent,
		req.CorrectAnswer,
	)
	if err != nil {
		return nil, err
	}
	return &aiv1.ExplainAnswerResponse{
		Explanation: explanation,
	}, nil
}

// GenerateHint 答题提示 handler
func (s *AIGatewayService) GenerateHint(ctx context.Context, req *aiv1.GenerateHintRequest) (*aiv1.GenerateHintResponse, error) {
	hint, err := s.quizUC.GenerateHint(
		ctx,
		req.QuestionTitle,
		req.QuestionContent,
	)
	if err != nil {
		return nil, err
	}
	return &aiv1.GenerateHintResponse{
		Hint: hint,
	}, nil
}

// ResumeParser 简历解析 handler
func (s *AIGatewayService) ResumeParser(ctx context.Context, req *aiv1.ResumeParserRequest) (*aiv1.ResumeParserResponse, error) {
	result, err := s.resumeUC.Parse(ctx, req.ResumeText, req.JobDescription)
	if err != nil {
		return nil, err
	}
	return &aiv1.ResumeParserResponse{
		Skills:      result.Skills,
		Experience:  result.Experience,
		Education:   result.Education,
		Projects:    result.Projects,
		Summary:     result.Summary,
		WeakSignals: result.WeakSignals,
		Strengths:   result.Strengths,
	}, nil
}

// Live2DDirector Live2D 角色控制指令生成 handler
func (s *AIGatewayService) Live2DDirector(ctx context.Context, req *aiv1.Live2DDirectiveRequest) (*aiv1.Live2DDirectiveResponse, error) {
	result, err := s.live2dUC.GenerateDirective(
		ctx,
		req.Context,
		req.EmotionHint,
		req.ReplyText,
	)
	if err != nil {
		return nil, err
	}
	expressionMix := make([]*aiv1.Live2DDirectiveExpressionLayer, 0, len(result.ExpressionMix))
	for _, e := range result.ExpressionMix {
		expressionMix = append(expressionMix, &aiv1.Live2DDirectiveExpressionLayer{
			Key:    e.Key,
			Weight: float32(e.Weight),
		})
	}
	parameterOverrides := make([]*aiv1.Live2DDirectiveParameterOverride, 0, len(result.ParameterOverrides))
	for _, p := range result.ParameterOverrides {
		parameterOverrides = append(parameterOverrides, &aiv1.Live2DDirectiveParameterOverride{
			Id:    p.ID,
			Value: float32(p.Value),
		})
	}
	mouthOpen := float32(0)
	if result.MouthOpen != nil {
		mouthOpen = float32(*result.MouthOpen)
	}
	return &aiv1.Live2DDirectiveResponse{
		Emotion:            result.Emotion,
		Action:             result.Action,
		Reply:              result.Reply,
		MotionKey:          result.MotionKey,
		MotionGroup:        result.MotionGroup,
		MotionPriority:     result.MotionPriority,
		MotionDurationMs:   int32(result.MotionDurationMS),
		Intensity:          float32(result.Intensity),
		DurationMs:         int32(result.DurationMS),
		MouthOpen:          mouthOpen,
		Source:             result.Source,
		ExpressionMix:      expressionMix,
		ParameterOverrides: parameterOverrides,
	}, nil
}

// toBizMessages 将 proto Message 切片转为 biz Message 切片
func toBizMessages(msgs []*aiv1.Message) []biz.Message {
	result := make([]biz.Message, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, biz.Message{Role: m.Role, Content: m.Content})
	}
	return result
}

// toProtoPlanTasks 将 biz PlanTask 切片转为 proto PlanTask 切片
func toProtoPlanTasks(tasks []biz.PlanTask) []*aiv1.PlanTask {
	result := make([]*aiv1.PlanTask, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, &aiv1.PlanTask{
			Title:           t.Title,
			Description:     t.Description,
			Phase:           t.Phase,
			OrderIndex:      t.OrderIndex,
			EstimatedHours:  t.EstimatedHours,
			TaskType:        t.TaskType,
			PhaseGoal:       t.PhaseGoal,
			DayNumber:       t.DayNumber,
			DurationMinutes: t.DurationMinutes,
			Priority:        t.Priority,
		})
	}
	return result
}

// ==================== Admin 调试 RPC ====================

// RenderPrompt Prompt 渲染预览 handler
func (s *AIGatewayService) RenderPrompt(ctx context.Context, req *aiv1.RenderPromptRequest) (*aiv1.RenderPromptResponse, error) {
	if s.adminUC == nil {
		return nil, status.Error(codes.Unimplemented, "Admin 调试用例未配置")
	}

	variables := req.GetVariables()
	if variables == nil {
		variables = make(map[string]string)
	}

	result, err := s.adminUC.RenderPrompt(ctx, req.GetScene(), req.GetTemplateText(), variables, req.GetRunWithLlm())
	if err != nil {
		return nil, err
	}

	return &aiv1.RenderPromptResponse{
		RenderedPrompt:    result.RenderedPrompt,
		ResolvedVariables: result.ResolvedVariables,
		LlmResponse:       result.LLMResponse,
		Model:             result.Model,
		LatencyMs:         result.LatencyMs,
	}, nil
}

// DebugAI AI 调试 handler
func (s *AIGatewayService) DebugAI(ctx context.Context, req *aiv1.DebugAIRequest) (*aiv1.DebugAIResponse, error) {
	if s.adminUC == nil {
		return nil, status.Error(codes.Unimplemented, "Admin 调试用例未配置")
	}

	params := req.GetParams()
	if params == nil {
		params = make(map[string]string)
	}

	result, err := s.adminUC.DebugAI(ctx, req.GetScene(), req.GetPrompt(), params, req.GetModelOverride())
	if err != nil {
		return nil, err
	}

	return &aiv1.DebugAIResponse{
		RenderedPrompt: result.RenderedPrompt,
		Response:       result.Response,
		Model:          result.Model,
		InputTokens:    int32(result.InputTokens),
		OutputTokens:   int32(result.OutputTokens),
		LatencyMs:      result.LatencyMs,
		Error:          result.Error,
	}, nil
}

// GenerateQuestionCandidates 同步题目候选生成 handler
func (s *AIGatewayService) GenerateQuestionCandidates(ctx context.Context, req *aiv1.GenerateQuestionCandidatesRequest) (*aiv1.GenerateQuestionCandidatesResponse, error) {
	if s.adminUC == nil {
		return nil, status.Error(codes.Unimplemented, "Admin 调试用例未配置")
	}

	candidateCount := req.GetCandidateCount()
	if candidateCount <= 0 {
		candidateCount = 5
	}

	result, err := s.adminUC.GenerateQuestionCandidates(
		ctx,
		req.GetIndustryCode(),
		req.GetRequirement(),
		req.GetAgentPrompt(),
		candidateCount,
		req.GetGenerationMode(),
		req.GetIncludeScraped(),
		req.GetIncludeGenerated(),
		req.GetSources(),
		req.GetIndustryName(),
		req.GetCategories(),
	)
	if err != nil {
		return nil, err
	}

	return &aiv1.GenerateQuestionCandidatesResponse{
		IndustryCode: result.IndustryCode,
		Requirement:  result.Requirement,
		Candidates:   toProtoCandidates(result.Candidates),
		Warnings:     result.Warnings,
	}, nil
}

// GenerateQuestionCandidatesStream 流式题目候选生成 handler
func (s *AIGatewayService) GenerateQuestionCandidatesStream(req *aiv1.GenerateQuestionCandidatesRequest, stream aiv1.AIService_GenerateQuestionCandidatesStreamServer) error {
	if s.adminUC == nil {
		return status.Error(codes.Unimplemented, "Admin 调试用例未配置")
	}

	candidateCount := req.GetCandidateCount()
	if candidateCount <= 0 {
		candidateCount = 5
	}

	// 创建事件回调
	emit := func(event *biz.QuestionPipelineStreamEvent) error {
		protoEvent := &aiv1.QuestionPipelineStreamEvent{
			Event:            event.Event,
			Message:          event.Message,
			TraceId:          event.TraceID,
			RawOutput:        event.RawOutput,
			FailureStage:     event.FailureStage,
			CandidateExcerpt: event.CandidateExcerpt,
			RepairAttempted:  event.RepairAttempted,
			SupplementAttempted: event.SupplementAttempted,
			SlotIndex:        event.SlotIndex,
			RetryIndex:       event.RetryIndex,
		}

		if event.Card != nil {
			protoEvent.Card = toProtoCandidate(event.Card)
		}

		if event.Response != nil {
			protoEvent.Response = &aiv1.GenerateQuestionCandidatesResponse{
				IndustryCode: event.Response.IndustryCode,
				Requirement:  event.Response.Requirement,
				Candidates:   toProtoCandidates(event.Response.Candidates),
				Warnings:     event.Response.Warnings,
			}
		}

		return stream.Send(protoEvent)
	}

	_, err := s.adminUC.GenerateQuestionCandidatesStream(
		stream.Context(),
		req.GetIndustryCode(),
		req.GetRequirement(),
		req.GetAgentPrompt(),
		candidateCount,
		req.GetIndustryName(),
		req.GetCategories(),
		emit,
	)
	return err
}

// toProtoCandidate 将单个 QuestionCandidate 转换为 proto 格式
func toProtoCandidate(c *biz.QuestionCandidate) *aiv1.QuestionCandidate {
	// 将 JudgeConfig 转换为 JSON 字符串
	judgeConfigStr := ""
	if c.JudgeConfig != nil {
		if jcBytes, err := json.Marshal(c.JudgeConfig); err == nil {
			judgeConfigStr = string(jcBytes)
		}
	}

	return &aiv1.QuestionCandidate{
		Id:          c.ID,
		Title:       c.Title,
		Content:     c.Content,
		Type:        c.Type,
		Difficulty:  c.Difficulty,
		Category:    c.Category,
		Answer:      c.Answer,
		Explanation: c.Explanation,
		Tags:        c.Tags,
		SourceType:  c.SourceType,
		Confidence:  c.Confidence,
		Solution:    c.Solution,
		JudgeConfig: judgeConfigStr,
		SourceLabel: c.SourceLabel,
		SourceTitle: c.SourceTitle,
		SourceUrl:   c.SourceURL,
	}
}

// toProtoCandidates 将 QuestionCandidate 切片转换为 proto 格式
func toProtoCandidates(candidates []*biz.QuestionCandidate) []*aiv1.QuestionCandidate {
	result := make([]*aiv1.QuestionCandidate, 0, len(candidates))
	for _, c := range candidates {
		result = append(result, toProtoCandidate(c))
	}
	return result
}
