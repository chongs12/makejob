package biz

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// interviewSessionState 面试会话状态（对齐单体 interviewSessionState）
type interviewSessionState struct {
	SessionID       string
	InterviewID     uint64
	IndustryCode    string
	Difficulty      string
	QuestionCount   int32
	ResumeText      string
	JobDescription  string
	InterviewMode   string
	CurrentIndex    int32
	History         []Message
	Questions       []InterviewSessionQuestion
	Feedbacks       []InterviewSessionFeedback
	HasStarted      bool
	CreatedAt       time.Time
}

// InterviewSessionQuestion 会话中的题目记录
type InterviewSessionQuestion struct {
	Index    int32
	Question string
	Topic    string
	Difficulty string
	Type     string
	Hints    string
}

// InterviewSessionFeedback 会话中的反馈记录
type InterviewSessionFeedback struct {
	Index    int32
	Score    float64
	Feedback string
}

// InterviewSessionUseCase 面试会话用例（对齐单体 InterviewAgent 接口）
type InterviewSessionUseCase struct {
	configRepo  AIConfigRepo
	promptRepo  PromptRepo
	callLogRepo CallLogRepo
	llm         LLMClient
	logger      log.Helper
	sessions    sync.Map // sessionID -> *interviewSessionState
}

// NewInterviewSessionUseCase 创建面试会话用例
func NewInterviewSessionUseCase(configRepo AIConfigRepo, promptRepo PromptRepo, callLogRepo CallLogRepo, llm LLMClient, logger log.Logger) *InterviewSessionUseCase {
	return &InterviewSessionUseCase{
		configRepo:  configRepo,
		promptRepo:  promptRepo,
		callLogRepo: callLogRepo,
		llm:         llm,
		logger:      *log.NewHelper(logger),
	}
}

// StartInterview 开始面试并生成首题（对齐单体 InterviewAgent.StartInterview）
func (uc *InterviewSessionUseCase) StartInterview(ctx context.Context, req *StartInterviewRequest) (*StartInterviewResponse, error) {
	const scene = "interview_agent"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	tpl, err := uc.promptRepo.GetActiveTemplate(ctx, scene)
	if err != nil {
		return nil, ErrPromptRenderFailed
	}

	// 构造首题 prompt
	promptText := RenderPrompt(tpl.TemplateContent, map[string]string{
		"industry_code":   req.IndustryCode,
		"difficulty":      req.Difficulty,
		"resume_text":     req.ResumeText,
		"job_description": req.JobDescription,
		"question_index":  "0",
	})

	schema := interviewResultSchema()
	messages := []Message{{Role: "system", Content: buildJSONContractPrompt(promptText, schema)}}

	llmCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	resp, err := uc.llm.Chat(llmCtx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		uc.logger.Warnf("LLM 出题失败，使用本地兜底: %v", err)
		return buildLocalStartResponse(req), nil
	}

	result, err := parseStructuredJSON[InterviewResult](llmCtx, uc.llm, cfg, resp.Content, schema)
	if err != nil {
		return nil, err
	}

	// 创建会话
	sessionID := uuid.NewString()
	session := &interviewSessionState{
		SessionID:     sessionID,
		InterviewID:   req.InterviewID,
		IndustryCode:  req.IndustryCode,
		Difficulty:    req.Difficulty,
		QuestionCount: req.QuestionCount,
		ResumeText:    req.ResumeText,
		JobDescription: req.JobDescription,
		InterviewMode: req.InterviewMode,
		CurrentIndex:  0,
		HasStarted:    true,
		CreatedAt:     time.Now(),
	}

	// 记录首题
	session.Questions = append(session.Questions, InterviewSessionQuestion{
		Index:    0,
		Question: result.Question,
		Topic:    result.Topic,
		Difficulty: result.Difficulty,
		Type:     result.Type,
		Hints:    result.Hints,
	})

	// 添加首题到历史
	session.History = append(session.History, Message{
		Role:    "assistant",
		Content: result.Question,
	})

	uc.sessions.Store(sessionID, session)

	return &StartInterviewResponse{
		SessionID:  sessionID,
		Question:   result.Question,
		Topic:      result.Topic,
		Difficulty: result.Difficulty,
		Type:       result.Type,
		Hints:      result.Hints,
	}, nil
}

// EvaluateAnswer 评估用户答案（对齐单体 InterviewAgent.EvaluateAnswer）
func (uc *InterviewSessionUseCase) EvaluateAnswer(ctx context.Context, req *EvaluateAnswerRequest) (*EvaluateAnswerResponse, error) {
	session, err := uc.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}

	const scene = "interview_agent"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	tpl, err := uc.promptRepo.GetActiveTemplate(ctx, scene)
	if err != nil {
		return nil, ErrPromptRenderFailed
	}

	// 添加用户答案到历史
	session.History = append(session.History, Message{
		Role:    "user",
		Content: req.Answer,
	})

	// 构造评估 prompt
	vars := map[string]string{
		"industry_code":   session.IndustryCode,
		"difficulty":      session.Difficulty,
		"user_answer":     req.Answer,
		"resume_text":     session.ResumeText,
		"job_description": session.JobDescription,
		"question_index":  fmt.Sprintf("%d", req.QuestionIndex),
	}
	if req.RAGContext != "" {
		vars["rag_context"] = req.RAGContext
	}
	promptText := RenderPrompt(tpl.TemplateContent, vars)

	schema := interviewResultSchema()
	messages := []Message{{Role: "system", Content: buildJSONContractPrompt(promptText, schema)}}
	messages = append(messages, session.History...)

	llmCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	resp, err := uc.llm.Chat(llmCtx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		uc.logger.Warnf("LLM 评分失败，使用本地兜底: %v", err)
		return buildLocalEvaluateResponse(req.Answer), nil
	}

	result, err := parseStructuredJSON[InterviewResult](llmCtx, uc.llm, cfg, resp.Content, schema)
	if err != nil {
		return nil, err
	}

	// 记录反馈
	session.Feedbacks = append(session.Feedbacks, InterviewSessionFeedback{
		Index:    req.QuestionIndex,
		Score:    result.Score,
		Feedback: result.Feedback,
	})

	// 更新历史
	if result.Feedback != "" {
		session.History = append(session.History, Message{
			Role:    "assistant",
			Content: result.Feedback,
		})
	}

	return &EvaluateAnswerResponse{
		Score:      result.Score,
		IsCorrect:  result.Score > 0.6,
		Feedback:   result.Feedback,
		KeyPoints:  nil,
		Suggestions: "",
		FollowUp:   "",
	}, nil
}

// GetNextQuestion 获取下一道题（对齐单体 InterviewAgent.GetNextQuestion）
func (uc *InterviewSessionUseCase) GetNextQuestion(ctx context.Context, req *GetNextQuestionSessionRequest) (*GetNextQuestionSessionResponse, error) {
	session, err := uc.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}

	const scene = "interview_agent"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	tpl, err := uc.promptRepo.GetActiveTemplate(ctx, scene)
	if err != nil {
		return nil, ErrPromptRenderFailed
	}

	nextIndex := session.CurrentIndex + 1

	// 构造下一题 prompt
	vars := map[string]string{
		"industry_code":   session.IndustryCode,
		"difficulty":      session.Difficulty,
		"resume_text":     session.ResumeText,
		"job_description": session.JobDescription,
		"question_index":  fmt.Sprintf("%d", nextIndex),
	}
	if req.RAGContext != "" {
		vars["rag_context"] = req.RAGContext
	}
	promptText := RenderPrompt(tpl.TemplateContent, vars)

	schema := interviewResultSchema()
	messages := []Message{{Role: "system", Content: buildJSONContractPrompt(promptText, schema)}}
	messages = append(messages, session.History...)

	llmCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	resp, err := uc.llm.Chat(llmCtx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, ErrLLMCallFailed
	}

	result, err := parseStructuredJSON[InterviewResult](llmCtx, uc.llm, cfg, resp.Content, schema)
	if err != nil {
		return nil, err
	}

	// 更新会话状态
	session.CurrentIndex = nextIndex
	session.Questions = append(session.Questions, InterviewSessionQuestion{
		Index:    nextIndex,
		Question: result.Question,
		Topic:    result.Topic,
		Difficulty: result.Difficulty,
		Type:     result.Type,
		Hints:    result.Hints,
	})
	session.History = append(session.History, Message{
		Role:    "assistant",
		Content: result.Question,
	})

	hasNext := nextIndex < session.QuestionCount-1

	return &GetNextQuestionSessionResponse{
		Question:   result.Question,
		Topic:      result.Topic,
		Difficulty: result.Difficulty,
		Type:       result.Type,
		Hints:      result.Hints,
		HasNext:    hasNext,
	}, nil
}

// GenerateReport 生成面试报告（对齐单体 InterviewAgent.GenerateReport）
func (uc *InterviewSessionUseCase) GenerateReport(ctx context.Context, req *GenerateInterviewReportRequest) (*GenerateInterviewReportResponse, error) {
	session, err := uc.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}

	const scene = "interview_agent"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	// 构造报告生成 prompt
	reportPrompt := uc.buildReportPrompt(session)
	schema := interviewReportSchema()
	messages := []Message{{Role: "system", Content: buildJSONContractPrompt(reportPrompt, schema)}}
	messages = append(messages, session.History...)

	llmCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	resp, err := uc.llm.Chat(llmCtx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		uc.logger.Warnf("LLM 报告生成失败，使用本地兜底: %v", err)
		return buildLocalReportResponse(session), nil
	}

	result, err := parseStructuredJSON[InterviewReportResult](llmCtx, uc.llm, cfg, resp.Content, schema)
	if err != nil {
		return nil, err
	}

	// 构造维度分数
	dimensionScores := make(map[string]float64)
	for _, feedback := range session.Feedbacks {
		dimensionScores[fmt.Sprintf("question_%d", feedback.Index)] = feedback.Score
	}

	return &GenerateInterviewReportResponse{
		OverallScore:    result.OverallScore,
		Summary:         result.Summary,
		DimensionScores: dimensionScores,
		Strengths:       result.Strengths,
		Weaknesses:      result.Weaknesses,
		Suggestions:     result.Suggestions,
		AiFeedback:      result.Summary,
	}, nil
}

// EndInterviewSession 结束面试会话（对齐单体 InterviewAgent.EndInterview）
func (uc *InterviewSessionUseCase) EndInterviewSession(ctx context.Context, req *EndInterviewSessionRequest) (*EndInterviewSessionResponse, error) {
	uc.sessions.Delete(req.SessionId)
	return &EndInterviewSessionResponse{Success: true}, nil
}

// getSession 获取会话状态
func (uc *InterviewSessionUseCase) getSession(sessionID string) (*interviewSessionState, error) {
	val, ok := uc.sessions.Load(sessionID)
	if !ok {
		return nil, kratosErr.NotFound("SESSION_NOT_FOUND", "面试会话不存在或已过期，请重新开始面试")
	}
	return val.(*interviewSessionState), nil
}

// buildReportPrompt 构造报告生成 prompt
func (uc *InterviewSessionUseCase) buildReportPrompt(session *interviewSessionState) string {
	var sb strings.Builder
	sb.WriteString("你是一位专业的技术面试评估专家。请根据以下面试记录生成一份详细的面试评估报告。\n\n")
	sb.WriteString(fmt.Sprintf("面试信息：\n- 行业：%s\n- 难度：%s\n- 题目数量：%d\n\n",
		session.IndustryCode, session.Difficulty, len(session.Questions)))

	sb.WriteString("面试记录：\n")
	for i, q := range session.Questions {
		sb.WriteString(fmt.Sprintf("\n第 %d 题：%s\n", i+1, q.Question))
		sb.WriteString(fmt.Sprintf("主题：%s, 难度：%s, 类型：%s\n", q.Topic, q.Difficulty, q.Type))
		if i < len(session.Feedbacks) {
			f := session.Feedbacks[i]
			sb.WriteString(fmt.Sprintf("评分：%.1f, 反馈：%s\n", f.Score, f.Feedback))
		}
	}

	sb.WriteString("\n请生成面试评估报告，包括：\n")
	sb.WriteString("1. overall_score: 总体评分 (0-100)\n")
	sb.WriteString("2. summary: 总体评价摘要\n")
	sb.WriteString("3. strengths: 优势列表\n")
	sb.WriteString("4. weaknesses: 不足列表\n")
	sb.WriteString("5. suggestions: 改进建议列表\n")

	return sb.String()
}

// InterviewReportResult 面试报告结果
type InterviewReportResult struct {
	OverallScore float64  `json:"overall_score"`
	Summary      string   `json:"summary"`
	Strengths    []string `json:"strengths"`
	Weaknesses   []string `json:"weaknesses"`
	Suggestions  []string `json:"suggestions"`
}

// interviewReportSchema 面试报告 JSON 合同
func interviewReportSchema() string {
	return `{
  "overall_score": 75.0,
  "summary": "面试总体评价",
  "strengths": ["优势1", "优势2"],
  "weaknesses": ["不足1", "不足2"],
  "suggestions": ["建议1", "建议2"]
}`
}

// GenerateReportFromHistory 从对话历史生成面试报告（不依赖 session，供实时面试使用）
func (uc *InterviewSessionUseCase) GenerateReportFromHistory(ctx context.Context, req *GenerateReportFromHistoryRequest) (*GenerateInterviewReportResponse, error) {
	if len(req.History) == 0 {
		return nil, kratosErr.BadRequest("EMPTY_HISTORY", "对话历史不能为空")
	}

	const scene = "interview_agent"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	// 构造报告 prompt
	prompt := uc.buildReportPromptFromHistory(req)
	schema := interviewReportSchema()
	messages := []Message{{Role: "system", Content: buildJSONContractPrompt(prompt, schema)}}
	messages = append(messages, req.History...)

	llmCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	resp, err := uc.llm.Chat(llmCtx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, err
	}

	result, err := parseStructuredJSON[InterviewReportResult](llmCtx, uc.llm, cfg, resp.Content, schema)
	if err != nil {
		return nil, err
	}

	return &GenerateInterviewReportResponse{
		OverallScore: result.OverallScore,
		Summary:      result.Summary,
		Strengths:    result.Strengths,
		Weaknesses:   result.Weaknesses,
		Suggestions:  result.Suggestions,
	}, nil
}

// buildReportPromptFromHistory 从对话历史构造报告生成 prompt
func (uc *InterviewSessionUseCase) buildReportPromptFromHistory(req *GenerateReportFromHistoryRequest) string {
	var sb strings.Builder
	sb.WriteString("你是一位专业的技术面试评估专家。请根据以下完整面试对话记录生成一份详细的面试评估报告。\n\n")
	sb.WriteString(fmt.Sprintf("面试信息：\n- 行业方向：%s\n- 目标难度：%s\n- 题目数量：%d\n\n",
		req.IndustryCode, req.Difficulty, req.TotalQuestions))
	sb.WriteString("完整面试对话记录：\n")
	for _, msg := range req.History {
		role := "候选人"
		if msg.Role == "assistant" {
			role = "面试官"
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", role, msg.Content))
	}
	sb.WriteString("\n请综合分析候选人的整场面试表现，生成评估报告，包括：\n")
	sb.WriteString("1. overall_score: 总体评分 (0-100)\n")
	sb.WriteString("2. summary: 总体评价摘要（200字以内）\n")
	sb.WriteString("3. strengths: 优势列表\n")
	sb.WriteString("4. weaknesses: 不足列表\n")
	sb.WriteString("5. suggestions: 改进建议列表\n")
	return sb.String()
}

// StartInterviewRequest 开始面试请求
type StartInterviewRequest struct {
	InterviewID   uint64
	IndustryCode  string
	Difficulty    string
	QuestionCount int32
	ResumeText    string
	JobDescription string
	InterviewMode string
}

// StartInterviewResponse 开始面试响应
type StartInterviewResponse struct {
	SessionID  string
	Question   string
	Topic      string
	Difficulty string
	Type       string
	Hints      string
}

// EvaluateAnswerRequest 评估答案请求
type EvaluateAnswerRequest struct {
	SessionId     string
	QuestionIndex int32
	Answer        string
	RAGContext    string // RAG 检索到的参考知识
}

// EvaluateAnswerResponse 评估答案响应
type EvaluateAnswerResponse struct {
	Score      float64
	IsCorrect  bool
	Feedback   string
	KeyPoints  []string
	Suggestions string
	FollowUp   string
}

// GetNextQuestionSessionRequest 获取下一题请求
type GetNextQuestionSessionRequest struct {
	SessionId  string
	RAGContext string // RAG 检索到的参考知识
}

// GetNextQuestionSessionResponse 获取下一题响应
type GetNextQuestionSessionResponse struct {
	Question   string
	Topic      string
	Difficulty string
	Type       string
	Hints      string
	HasNext    bool
}

// GenerateInterviewReportRequest 生成报告请求
type GenerateInterviewReportRequest struct {
	SessionId string
}

// GenerateInterviewReportResponse 生成报告响应
type GenerateInterviewReportResponse struct {
	OverallScore    float64
	Summary         string
	DimensionScores map[string]float64
	Strengths       []string
	Weaknesses      []string
	Suggestions     []string
	AiFeedback      string
}

// GenerateReportFromHistoryRequest 从对话历史生成报告请求（不依赖 session）
type GenerateReportFromHistoryRequest struct {
	History        []Message
	IndustryCode   string
	Difficulty     string
	TotalQuestions int32
}

// EndInterviewSessionRequest 结束会话请求
type EndInterviewSessionRequest struct {
	SessionId string
}

// EndInterviewSessionResponse 结束会话响应
type EndInterviewSessionResponse struct {
	Success bool
}

// saveLog 记录 LLM 调用日志
func (uc *InterviewSessionUseCase) saveLog(ctx context.Context, scene, model string, resp *LLMResponse, callErr error, latencyMs int64) {
	logEntry := &AICallLog{Scene: scene, Model: model, LatencyMs: latencyMs}
	if resp != nil {
		logEntry.InputTokens = resp.InputTokens
		logEntry.OutputTokens = resp.OutputTokens
	}
	if callErr != nil {
		logEntry.Status = "error"
		logEntry.ErrorMsg = callErr.Error()
	} else {
		logEntry.Status = "success"
	}
	if err := uc.callLogRepo.Create(ctx, logEntry); err != nil {
		uc.logger.Warnf("写入AI调用日志失败: %v", err)
	}
}
