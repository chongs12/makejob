package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	aiv1 "makejob/api/makejob/ai/v1"
	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/conf"
)

const correctThreshold = 0.6

// aiServiceClient 实现 biz.AIServiceClient 接口
// 通过 gRPC 调用 AI 服务
type aiServiceClient struct {
	client aiv1.AIServiceClient
	conn   *grpc.ClientConn
}

// NewAIServiceClient 创建 AI 服务客户端（由 Wire 调用）
func NewAIServiceClient(cfg *conf.AI) (biz.AIServiceClient, error) {
	conn, err := grpc.Dial(cfg.ServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial AI service at %s: %w", cfg.ServiceAddr, err)
	}
	return &aiServiceClient{
		client: aiv1.NewAIServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *aiServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// InterviewAgent 调用 AI Gateway 生成下一题或评估当前回答。
func (c *aiServiceClient) InterviewAgent(ctx context.Context, req *biz.InterviewAgentRequest) (*biz.InterviewAgentResponse, error) {
	// 转换 biz 历史消息为 proto 消息
	history := make([]*aiv1.Message, 0, len(req.History))
	for _, msg := range req.History {
		history = append(history, &aiv1.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	resp, err := c.client.InterviewAgent(ctx, &aiv1.InterviewAgentRequest{
		InterviewId:    req.InterviewID,
		IndustryCode:   req.IndustryCode,
		Difficulty:     req.Difficulty,
		History:        history,
		UserAnswer:     req.UserAnswer,
		QuestionIndex:  req.QuestionIndex,
		ResumeText:     req.ResumeText,
		JobDescription: req.JobDesc,
	})
	if err != nil {
		return nil, fmt.Errorf("InterviewAgent gRPC call failed: %w", err)
	}

	result := &biz.InterviewAgentResponse{
		ShouldEnd: resp.ShouldEnd,
		Live2DDirective: &biz.Live2DDirective{
			Emotion: resp.Live2DEmotion,
			Action:  resp.Live2DAction,
		},
	}
	if resp.Question != "" {
		result.Question = &biz.InterviewQuestion{
			Question:        resp.Question,
			Topic:           resp.Topic,
			Difficulty:      resp.Difficulty,
			Type:            resp.Type,
			Hints:           resp.Hints,
			Live2DDirective: result.Live2DDirective,
		}
	}
	if resp.Feedback != "" || resp.Score > 0 {
		result.Feedback = &biz.AnswerFeedback{
			Score:       resp.Score,
			Feedback:    resp.Feedback,
			IsCorrect:   resp.Score > correctThreshold,
			KeyPoints:   nil,
			Suggestions: "",
		}
	}
	return result, nil
}

// QuizAnalyzer 调用 AI Gateway 评估问答或代码答案。
func (c *aiServiceClient) QuizAnalyzer(ctx context.Context, req *biz.QuizAnalyzerRequest) (*biz.QuizAnalyzerResponse, error) {
	resp, err := c.client.QuizAnalyzer(ctx, &aiv1.QuizAnalyzerRequest{
		Question:   req.Question,
		Answer:     req.Answer,
		Topic:      req.Topic,
		Difficulty: req.Difficulty,
	})
	if err != nil {
		return nil, fmt.Errorf("QuizAnalyzer gRPC call failed: %w", err)
	}

	return &biz.QuizAnalyzerResponse{
		Score:         resp.Score,
		IsCorrect:     resp.IsCorrect,
		Feedback:      resp.Feedback,
		KeyPoints:     resp.KeyPoints,
		Suggestions:   resp.Suggestions,
		CorrectAnswer: resp.CorrectAnswer,
	}, nil
}

// ResumeParser 调用 AI Gateway 解析简历文本。
func (c *aiServiceClient) ResumeParser(ctx context.Context, req *biz.ResumeParserRequest) (*biz.ResumeParserResponse, error) {
	resp, err := c.client.ResumeParser(ctx, &aiv1.ResumeParserRequest{
		ResumeText: req.ResumeText,
	})
	if err != nil {
		return nil, fmt.Errorf("ResumeParser gRPC call failed: %w", err)
	}

	return &biz.ResumeParserResponse{
		Skills:      resp.Skills,
		Experience:  resp.Experience,
		Education:   resp.Education,
		Projects:    resp.Projects,
		Summary:     resp.Summary,
		WeakSignals: resp.WeakSignals,
	}, nil
}

// StartInterview 开始面试会话（对齐单体 InterviewAgent.StartInterview）
func (c *aiServiceClient) StartInterview(ctx context.Context, req *biz.StartInterviewRequest) (*biz.StartInterviewResponse, error) {
	resp, err := c.client.StartInterview(ctx, &aiv1.StartInterviewRequest{
		InterviewId:   req.InterviewID,
		IndustryCode:  req.IndustryCode,
		Difficulty:    req.Difficulty,
		QuestionCount: req.QuestionCount,
		ResumeText:    req.ResumeText,
		JobDescription: req.JobDescription,
		InterviewMode: req.InterviewMode,
	})
	if err != nil {
		return nil, fmt.Errorf("StartInterview gRPC call failed: %w", err)
	}

	return &biz.StartInterviewResponse{
		SessionID:  resp.SessionId,
		Question:   resp.Question,
		Topic:      resp.Topic,
		Difficulty: resp.Difficulty,
		Type:       resp.Type,
		Hints:      resp.Hints,
	}, nil
}

// EvaluateAnswer 评估用户答案（对齐单体 InterviewAgent.EvaluateAnswer）
func (c *aiServiceClient) EvaluateAnswer(ctx context.Context, req *biz.EvaluateAnswerRequest) (*biz.EvaluateAnswerResponse, error) {
	resp, err := c.client.EvaluateAnswer(ctx, &aiv1.EvaluateAnswerRequest{
		SessionId:     req.SessionId,
		QuestionIndex: req.QuestionIndex,
		Answer:        req.Answer,
		RagContext:    req.RAGContext,
	})
	if err != nil {
		return nil, fmt.Errorf("EvaluateAnswer gRPC call failed: %w", err)
	}

	return &biz.EvaluateAnswerResponse{
		Score:      resp.Score,
		IsCorrect:  resp.IsCorrect,
		Feedback:   resp.Feedback,
		KeyPoints:  resp.KeyPoints,
		Suggestions: resp.Suggestions,
		FollowUp:   resp.FollowUp,
	}, nil
}

// GetNextQuestionSession 获取下一道题（对齐单体 InterviewAgent.GetNextQuestion + EnhanceQuestionPrompt）
func (c *aiServiceClient) GetNextQuestionSession(ctx context.Context, req *biz.GetNextQuestionSessionRequest) (*biz.GetNextQuestionSessionResponse, error) {
	resp, err := c.client.GetNextQuestionSession(ctx, &aiv1.GetNextQuestionSessionRequest{
		SessionId:  req.SessionId,
		RagContext: req.RAGContext,
	})
	if err != nil {
		return nil, fmt.Errorf("GetNextQuestionSession gRPC call failed: %w", err)
	}

	return &biz.GetNextQuestionSessionResponse{
		Question:   resp.Question,
		Topic:      resp.Topic,
		Difficulty: resp.Difficulty,
		Type:       resp.Type,
		Hints:      resp.Hints,
		HasNext:    resp.HasNext,
	}, nil
}

// GenerateInterviewReport 生成面试报告（对齐单体 InterviewAgent.GenerateReport）
func (c *aiServiceClient) GenerateInterviewReport(ctx context.Context, req *biz.GenerateInterviewReportRequest) (*biz.GenerateInterviewReportResponse, error) {
	resp, err := c.client.GenerateInterviewReport(ctx, &aiv1.GenerateInterviewReportRequest{
		SessionId: req.SessionId,
	})
	if err != nil {
		return nil, fmt.Errorf("GenerateInterviewReport gRPC call failed: %w", err)
	}

	return &biz.GenerateInterviewReportResponse{
		OverallScore:    resp.OverallScore,
		Summary:         resp.Summary,
		DimensionScores: resp.DimensionScores,
		Strengths:       resp.Strengths,
		Weaknesses:      resp.Weaknesses,
		Suggestions:     resp.Suggestions,
		AiFeedback:      resp.AiFeedback,
	}, nil
}

// EndInterviewSession 结束面试会话（对齐单体 InterviewAgent.EndInterview）
func (c *aiServiceClient) EndInterviewSession(ctx context.Context, req *biz.EndInterviewSessionRequest) (*biz.EndInterviewSessionResponse, error) {
	resp, err := c.client.EndInterviewSession(ctx, &aiv1.EndInterviewSessionRequest{
		SessionId: req.SessionId,
	})
	if err != nil {
		return nil, fmt.Errorf("EndInterviewSession gRPC call failed: %w", err)
	}

	return &biz.EndInterviewSessionResponse{
		Success: resp.Success,
	}, nil
}
