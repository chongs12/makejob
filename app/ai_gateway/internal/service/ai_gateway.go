package service

import (
	"context"

	aiv1 "makejob/api/makejob/ai/v1"
	"makejob/app/ai_gateway/internal/biz"
)

// AIGatewayService AI 网关 gRPC 服务实现，聚合全部 AI 场景用例
type AIGatewayService struct {
	aiv1.UnimplementedAIServiceServer
	interviewUC  *biz.InterviewAgentUseCase
	planUC       *biz.PlanAgentUseCase
	companionUC  *biz.CompanionAgentUseCase
	quizUC       *biz.QuizAnalyzerUseCase
	resumeUC     *biz.ResumeParserUseCase
	live2dUC     *biz.Live2DDirectorUseCase
}

// NewAIGatewayService 创建 AI 网关 gRPC 服务
func NewAIGatewayService(
	interviewUC *biz.InterviewAgentUseCase,
	planUC *biz.PlanAgentUseCase,
	companionUC *biz.CompanionAgentUseCase,
	quizUC *biz.QuizAnalyzerUseCase,
	resumeUC *biz.ResumeParserUseCase,
	live2dUC *biz.Live2DDirectorUseCase,
) *AIGatewayService {
	return &AIGatewayService{
		interviewUC: interviewUC,
		planUC:      planUC,
		companionUC: companionUC,
		quizUC:      quizUC,
		resumeUC:    resumeUC,
		live2dUC:    live2dUC,
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

// CompanionAgent AI 陪伴聊天 handler
func (s *AIGatewayService) CompanionAgent(ctx context.Context, req *aiv1.CompanionAgentRequest) (*aiv1.CompanionAgentResponse, error) {
	result, err := s.companionUC.Chat(
		ctx,
		req.Message,
		req.ContextType,
		req.RecentTopics,
	)
	if err != nil {
		return nil, err
	}
	return &aiv1.CompanionAgentResponse{
		Reply:       result.Reply,
		Emotion:     result.Emotion,
		Suggestions: result.Suggestions,
	}, nil
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

// ResumeParser 简历解析 handler
func (s *AIGatewayService) ResumeParser(ctx context.Context, req *aiv1.ResumeParserRequest) (*aiv1.ResumeParserResponse, error) {
	result, err := s.resumeUC.Parse(ctx, req.ResumeText)
	if err != nil {
		return nil, err
	}
	return &aiv1.ResumeParserResponse{
		Skills:     result.Skills,
		Experience: result.Experience,
		Education:  result.Education,
		Projects:   result.Projects,
		Summary:    result.Summary,
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
	return &aiv1.Live2DDirectiveResponse{
		Emotion:     result.Emotion,
		Action:      result.Action,
		Reply:       result.Reply,
		MotionKey:   result.MotionKey,
		MotionGroup: result.MotionGroup,
		DurationMs:  result.DurationMs,
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
			Title:          t.Title,
			Description:    t.Description,
			Phase:          t.Phase,
			OrderIndex:     t.OrderIndex,
			EstimatedHours: t.EstimatedHours,
		})
	}
	return result
}
