package biz

import (
	"context"
	"encoding/json"
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
	KnowledgeTopics []string // 知识点专项面试的自定义知识点，供后续出题与报告使用
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

	// 构造首题 prompt：按面试类型选内联 prompt 或 DB 模板
	var promptText string
	switch req.InterviewType {
	case "knowledge":
		promptText = buildKnowledgeQuestionPrompt(req.Topics, req.Difficulty, "0")
	case "job":
		promptText = buildJobQuestionPrompt(req.ResumeText, req.JobDescription, req.Difficulty, "0")
	default:
		tpl, err := uc.promptRepo.GetActiveTemplate(ctx, scene)
		if err != nil {
			return nil, ErrPromptRenderFailed
		}
		promptText = RenderPrompt(tpl.TemplateContent, map[string]string{
			"industry_code":   req.IndustryCode,
			"difficulty":      req.Difficulty,
			"resume_text":     req.ResumeText,
			"job_description": req.JobDescription,
			"question_index":  "0",
		})
	}

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
		uc.logger.Warnf("StartInterview LLM 响应解析失败，使用本地兜底: interview_id=%d err=%v", req.InterviewID, err)
		return buildLocalStartResponse(req), nil
	}

	// LLM 可能返回合法 JSON 但 question 字段为空/缺失，此时首题为空、不会落库，面试会卡在“无题可答”状态。
	// 降级到本地兜底题，保证非实时面试始终有首题（实时面试不走 StartInterview，不受影响）。
	if strings.TrimSpace(result.Question) == "" {
		uc.logger.Warnf("StartInterview LLM 返回空题目，使用本地兜底: interview_id=%d type=%s topics=%v", req.InterviewID, req.InterviewType, req.Topics)
		return buildLocalStartResponse(req), nil
	}

	// 创建会话
	sessionID := uuid.NewString()
	session := &interviewSessionState{
		SessionID:      sessionID,
		InterviewID:    req.InterviewID,
		IndustryCode:   req.IndustryCode,
		Difficulty:     req.Difficulty,
		QuestionCount:  req.QuestionCount,
		ResumeText:     req.ResumeText,
		JobDescription: req.JobDescription,
		InterviewMode:  req.InterviewMode,
		KnowledgeTopics: req.Topics,
		CurrentIndex:   0,
		HasStarted:     true,
		CreatedAt:      time.Now(),
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

	// 添加用户答案到历史
	session.History = append(session.History, Message{
		Role:    "user",
		Content: req.Answer,
	})

	// 构造评估 prompt（专用：明确要求 0-100 打分 + 关键点 + 建议），不再依赖通用出题模板，
	// 避免出题合同里 score 无描述、无 key_points/suggestions 字段导致 LLM 漏返回评分。
	promptText := buildEvaluatePrompt(req.Answer, session.Difficulty)
	if req.RAGContext != "" {
		promptText += "\n参考资料（评估时可参考）：\n" + req.RAGContext
	}
	schema := interviewEvaluateSchema()
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

	result, err := parseStructuredJSON[InterviewEvaluateResult](llmCtx, uc.llm, cfg, resp.Content, schema)
	if err != nil {
		uc.logger.Warnf("EvaluateAnswer LLM 响应解析失败，使用本地兜底: err=%v", err)
		return buildLocalEvaluateResponse(req.Answer), nil
	}

	// LLM 可能不打分（score=0）或漏字段，用本地兜底补齐，保证前端总能拿到完整结构化反馈
	fallback := buildLocalEvaluateResponse(req.Answer)
	if result.Score <= 0 {
		result.Score = fallback.Score
	}
	if strings.TrimSpace(result.Feedback) == "" {
		result.Feedback = fallback.Feedback
	}
	if len(result.KeyPoints) == 0 {
		result.KeyPoints = fallback.KeyPoints
	}
	if strings.TrimSpace(result.Suggestions) == "" {
		result.Suggestions = fallback.Suggestions
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
		Score:       result.Score,
		IsCorrect:   result.Score >= 60,
		Feedback:    result.Feedback,
		KeyPoints:   result.KeyPoints,
		Suggestions: result.Suggestions,
		FollowUp:    "",
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
	Topics        []string // 知识点专项面试的自定义知识点列表
	InterviewType string   // knowledge | job，决定出题 prompt 分支
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

// GenerateKnowledgeReport 知识点专项面试报告生成（不依赖 session，基于完整对话历史）。
// 推理模型偶发返回空内容，故最多重试 3 次；全部失败时用本地兜底报告，保证不落 report_failed。
func (uc *InterviewSessionUseCase) GenerateKnowledgeReport(ctx context.Context, req *GenerateKnowledgeReportRequest) (*GenerateKnowledgeReportResponse, error) {
	if len(req.History) == 0 {
		return nil, kratosErr.BadRequest("EMPTY_HISTORY", "对话历史不能为空")
	}

	const scene = "interview_agent"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	prompt := uc.buildKnowledgeReportPrompt(req.KnowledgeTopics, req.Difficulty, req.TotalQuestions)
	schema := knowledgeReportSchema()
	messages := []Message{{Role: "system", Content: buildJSONContractPrompt(prompt, schema)}}
	messages = append(messages, req.History...)

	var result KnowledgeReportResult
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		llmCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		resp, chatErr := uc.llm.Chat(llmCtx, messages, cfg)
		uc.saveLog(ctx, scene, cfg.Model, resp, chatErr, time.Since(start).Milliseconds())
		if chatErr == nil && resp != nil {
			parsed, perr := parseStructuredJSON[KnowledgeReportResult](llmCtx, uc.llm, cfg, resp.Content, schema)
			cancel()
			if perr == nil && parsed.OverallScore > 0 {
				result = parsed
				lastErr = nil
				break
			}
			lastErr = perr
		} else {
			cancel()
			lastErr = chatErr
		}
		uc.logger.Warnf("GenerateKnowledgeReport attempt %d 失败，重试: %v", attempt+1, lastErr)
	}

	if lastErr != nil {
		uc.logger.Warnf("GenerateKnowledgeReport LLM 全部失败，使用本地兜底报告: %v", lastErr)
		result = buildLocalKnowledgeReport(req.History, req.KnowledgeTopics, req.TotalQuestions)
	}

	// 后端定级，确保评级规则一致，不依赖 LLM 自由发挥
	rating := classifyKnowledgeRating(result.OverallScore)
	result.Rating = rating

	reportBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal knowledge report failed: %w", err)
	}

	return &GenerateKnowledgeReportResponse{
		ReportJSON:   string(reportBytes),
		OverallScore: result.OverallScore,
		Rating:       rating,
	}, nil
}

// buildKnowledgeReportPrompt 构造知识点专项报告生成 prompt。
// 对话历史作为 messages 传入（标准做法），不再嵌进 system prompt，避免重复占 token、干扰模型。
func (uc *InterviewSessionUseCase) buildKnowledgeReportPrompt(topics []string, difficulty string, totalQuestions int32) string {
	var sb strings.Builder
	sb.WriteString("你是一位严谨的知识点考核评估专家。请根据对话历史中候选人围绕自定义知识点的问答表现，生成一份学习型评估报告。\n\n")
	sb.WriteString(fmt.Sprintf("考核知识点：%s\n目标难度：%s\n题目数量：%d\n\n", strings.Join(topics, "、"), difficulty, totalQuestions))
	sb.WriteString("请严格按 JSON 合同输出报告，要求：\n")
	sb.WriteString("1. overall_score: 总体评分 0-100；rating: 优秀(>=85)/良好(>=70)/合格(>=60)/薄弱(<60)。\n")
	sb.WriteString("2. conclusion: 一句话总结该知识点掌握现状。\n")
	sb.WriteString("3. question_reviews: 逐题复盘，含用户作答原文、得分、错误点/遗漏点/亮点、标准答案、核心得分点。\n")
	sb.WriteString("4. dimension_scores: 四个固定维度（知识点基础掌握度/知识点应用落地能力/知识延伸与深度/答题精准度与严谨度）各打分 0-100 并给评语。\n")
	sb.WriteString("5. mastered_points: 本次完全掌握的知识点。\n")
	sb.WriteString("6. blind_spots: 知识盲区，level 必须是 完全不会/一知半解/容易混淆/答题不严谨 之一。\n")
	sb.WriteString("7. study_suggestions: 针对薄弱点的补强学习建议。\n")
	sb.WriteString("8. next_quiz_topics: 下一轮专项刷题方向及原因。\n")
	return sb.String()
}

// buildKnowledgeQuestionPrompt 构造知识点专项出题 prompt（内联，不依赖 DB 模板）。
func buildKnowledgeQuestionPrompt(topics []string, difficulty, questionIndex string) string {
	return fmt.Sprintf("你是一位严谨的技术考核官，正在对候选人进行知识点专项考核。\n\n"+
		"考核知识点：%s\n难度：%s\n当前题号：%s\n\n"+
		"出题要求：\n"+
		"1. 围绕上述知识点出题，每道题聚焦一个具体知识点或其易错点。\n"+
		"2. 题目类型在 technical/behavioral/coding 中选择，coding 题需给出可运行的题目描述。\n"+
		"3. 由浅入深，覆盖基础概念、应用场景、进阶边界与易错点。\n"+
		"4. topic 字段必须填写本题对应的具体知识点名称。\n",
		strings.Join(topics, "、"), difficulty, questionIndex)
}

// buildEvaluatePrompt 构造答案评估 prompt，明确要求 0-100 打分、关键点与改进建议。
func buildEvaluatePrompt(answer, difficulty string) string {
	return fmt.Sprintf("你是一位严谨的技术面试官，现在需要对候选人刚才的回答进行评估打分。\n\n"+
		"候选人回答：%s\n难度：%s\n\n"+
		"评估要求：\n"+
		"1. 根据回答的准确性、完整性、技术深度给出 0-100 的整数评分（<60 不合格，60-75 合格，76-85 良好，86-100 优秀）。\n"+
		"2. is_correct 表示回答是否基本正确（评分 >=60 时为 true）。\n"+
		"3. feedback 给出具体评价，先肯定亮点，再指出不足。\n"+
		"4. key_points 列出回答中涉及或应当涉及的关键知识点（2-5 条）。\n"+
		"5. suggestions 给出可执行的改进建议。\n",
		answer, difficulty)
}

// classifyKnowledgeRating 按总分映射评级，保证一致性。
func classifyKnowledgeRating(score float64) string {
	switch {
	case score >= 85:
		return "优秀"
	case score >= 70:
		return "良好"
	case score >= 60:
		return "合格"
	default:
		return "薄弱"
	}
}

// GenerateKnowledgeReportRequest 知识点专项报告生成请求
type GenerateKnowledgeReportRequest struct {
	History         []Message
	KnowledgeTopics []string
	Difficulty      string
	TotalQuestions  int32
}

// GenerateKnowledgeReportResponse 知识点专项报告生成响应
type GenerateKnowledgeReportResponse struct {
	ReportJSON   string
	OverallScore float64
	Rating       string
}

// KnowledgeReportResult 知识点专项报告 LLM 结构化输出结果
type KnowledgeReportResult struct {
	OverallScore     float64                    `json:"overall_score"`
	Rating           string                     `json:"rating"`
	Conclusion       string                     `json:"conclusion"`
	BasicInfo        KnowledgeReportBasicInfo   `json:"basic_info"`
	QuestionReviews  []KnowledgeQuestionReview  `json:"question_reviews"`
	DimensionScores  []KnowledgeDimensionScore  `json:"dimension_scores"`
	MasteredPoints   []string                   `json:"mastered_points"`
	BlindSpots       []KnowledgeBlindSpot       `json:"blind_spots"`
	StudySuggestions []KnowledgeStudySuggestion `json:"study_suggestions"`
	NextQuizTopics   []KnowledgeNextQuizTopic   `json:"next_quiz_topics"`
}

// KnowledgeReportBasicInfo 报告基础信息
type KnowledgeReportBasicInfo struct {
	KnowledgeTopics []string `json:"knowledge_topics"`
	QuestionType    string   `json:"question_type"`
	DurationSeconds int32    `json:"duration_seconds"`
	TotalQuestions  int32    `json:"total_questions"`
	CorrectCount    int32    `json:"correct_count"`
	Accuracy        float64  `json:"accuracy"`
}

// KnowledgeQuestionReview 逐题复盘
type KnowledgeQuestionReview struct {
	QuestionIndex  int32    `json:"question_index"`
	Question       string   `json:"question"`
	UserAnswer     string   `json:"user_answer"`
	Score          float64  `json:"score"`
	MaxScore       float64  `json:"max_score"`
	Errors         []string `json:"errors"`
	Omissions      []string `json:"omissions"`
	Highlights     []string `json:"highlights"`
	StandardAnswer string   `json:"standard_answer"`
	KeyPoints      []string `json:"key_points"`
}

// KnowledgeDimensionScore 维度评分
type KnowledgeDimensionScore struct {
	Dimension string  `json:"dimension"`
	Score     float64 `json:"score"`
	Comment   string  `json:"comment"`
}

// KnowledgeBlindSpot 知识盲区
type KnowledgeBlindSpot struct {
	Topic  string `json:"topic"`
	Level  string `json:"level"`
	Detail string `json:"detail"`
}

// KnowledgeStudySuggestion 补强学习建议
type KnowledgeStudySuggestion struct {
	Focus  string `json:"focus"`
	Detail string `json:"detail"`
}

// KnowledgeNextQuizTopic 二次考核出题建议
type KnowledgeNextQuizTopic struct {
	Topic  string `json:"topic"`
	Reason string `json:"reason"`
}

// GenerateJobReport 岗位求职面试报告生成（不依赖 session，基于完整对话历史 + 简历画像 + JD）。
// 求职型报告必须由 LLM 生成，失败直接返回错误，不做兜底降级。
func (uc *InterviewSessionUseCase) GenerateJobReport(ctx context.Context, req *GenerateJobReportRequest) (*GenerateJobReportResponse, error) {
	if len(req.History) == 0 {
		return nil, kratosErr.BadRequest("EMPTY_HISTORY", "对话历史不能为空")
	}

	const scene = "interview_agent"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	prompt := uc.buildJobReportPrompt(req)
	schema := jobReportSchema()
	messages := []Message{{Role: "system", Content: buildJSONContractPrompt(prompt, schema)}}
	messages = append(messages, req.History...)

	llmCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	resp, err := uc.llm.Chat(llmCtx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, err
	}

	result, err := parseStructuredJSON[JobReportResult](llmCtx, uc.llm, cfg, resp.Content, schema)
	if err != nil {
		return nil, err
	}

	// 后端按固定权重加权算总分，覆盖 LLM 综合，确保一致性
	overallScore := computeJobOverallScore(result.DimensionScores)
	result.OverallScore = overallScore
	result.Rating = classifyKnowledgeRating(overallScore) // 复用评级逻辑

	reportBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal job report failed: %w", err)
	}

	return &GenerateJobReportResponse{
		ReportJSON:         string(reportBytes),
		OverallScore:       overallScore,
		Rating:             result.Rating,
		HireRecommendation: result.HireRecommendation,
	}, nil
}

// buildJobReportPrompt 构造岗位求职报告生成 prompt。
func (uc *InterviewSessionUseCase) buildJobReportPrompt(req *GenerateJobReportRequest) string {
	var sb strings.Builder
	sb.WriteString("你是一位资深的企业招聘面试评估专家。请根据候选人简历、目标岗位要求与完整面试对话记录，生成一份求职型面试评估报告。\n\n")
	sb.WriteString(fmt.Sprintf("目标岗位 JD：\n%s\n\n", strings.TrimSpace(req.JobDescription)))
	if strings.TrimSpace(req.ResumeParsedJSON) != "" {
		sb.WriteString(fmt.Sprintf("简历结构化画像（JSON）：\n%s\n\n", req.ResumeParsedJSON))
	}
	if strings.TrimSpace(req.ResumeText) != "" {
		sb.WriteString(fmt.Sprintf("简历原文：\n%s\n\n", req.ResumeText))
	}
	sb.WriteString(fmt.Sprintf("行业方向：%s  目标难度：%s  题目数量：%d\n\n", req.IndustryCode, req.Difficulty, req.TotalQuestions))
	sb.WriteString("完整面试对话记录：\n")
	for _, msg := range req.History {
		role := "候选人"
		if msg.Role == "assistant" {
			role = "面试官"
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", role, msg.Content))
	}
	sb.WriteString("\n请严格按 JSON 合同输出报告，要求：\n")
	sb.WriteString("1. overall_score: 总体评分 0-100（后端会按维度权重重新加权计算，此处给出你的综合判断即可）；rating: 优秀(>=85)/良好(>=70)/合格(>=60)/薄弱(<60)。\n")
	sb.WriteString("2. hire_recommendation: 录用建议，建议录用/建议复试考察/人才储备/不予录用 之一。\n")
	sb.WriteString("3. basic_info: 面试档案基础信息，candidate_name 从简历提取（无则填'候选人'）。\n")
	sb.WriteString("4. jd_match_overview: 简历与岗位 JD 的匹配总览，含核心匹配项、缺失项、硬性条件达标、简历优势与硬伤。\n")
	sb.WriteString("5. question_reviews: 逐题复盘，含面试亮点、回答漏洞、踩坑点、职场面试禁忌点。\n")
	sb.WriteString("6. dimension_scores: 六大维度评分（0-100）+ 详细优缺点解读，维度名与权重必须严格按合同：岗位硬技能匹配度(0.35)、简历项目真实性&含金量(0.25)、逻辑思维与表达能力(0.15)、求职动机与岗位认知(0.10)、职业素养与稳定性(0.10)、综合面试印象(0.05)。\n")
	sb.WriteString("7. core_advantages: 核心求职优势，可直接用于优化自我介绍与应答话术。\n")
	sb.WriteString("8. weaknesses_risks: 面试短板与求职风险，level 必须是 致命 或 轻微 之一，注明对录用的影响。\n")
	sb.WriteString("9. hire_decision: 最终面试决策，含 decision 与核心依据 rationale。\n")
	sb.WriteString("10. optimization_plan: 针对性面试优化方案，aspect 从 话术优化/项目包装/短板补强/高频追问应对/简历优化 中选择。\n")
	sb.WriteString("11. next_round_questions: 下一轮复试预测题库，含考点与难度。\n")
	return sb.String()
}

// buildJobQuestionPrompt 构造岗位求职出题 prompt（内联，不依赖 DB 模板）。
func buildJobQuestionPrompt(resumeText, jobDescription, difficulty, questionIndex string) string {
	var sb strings.Builder
	sb.WriteString("你是一位资深的企业面试官，正在对候选人进行岗位求职面试。请结合候选人简历与目标岗位要求出题。\n\n")
	sb.WriteString(fmt.Sprintf("目标难度：%s  当前题号：%s\n\n", difficulty, questionIndex))
	if strings.TrimSpace(jobDescription) != "" {
		sb.WriteString(fmt.Sprintf("目标岗位 JD：\n%s\n\n", jobDescription))
	}
	if strings.TrimSpace(resumeText) != "" {
		sb.WriteString(fmt.Sprintf("候选人简历：\n%s\n\n", resumeText))
	}
	sb.WriteString("出题要求：\n")
	sb.WriteString("1. 围绕岗位 JD 的核心能力要求与简历经历出题，模拟真实企业面试（技术面/HR面/综合面）。\n")
	sb.WriteString("2. 追问简历项目的真实性、个人贡献、成果量化，挖掘注水与含金量。\n")
	sb.WriteString("3. 由浅入深，覆盖硬技能、项目经历、逻辑表达、求职动机、职业素养等维度。\n")
	sb.WriteString("4. topic 字段填写本题考察的维度（如'岗位硬技能匹配度'、'项目含金量'、'逻辑表达'）。\n")
	return sb.String()
}

// computeJobOverallScore 按固定权重加权计算岗位面试总分，归一化容错缺失维度。
func computeJobOverallScore(dimensions []JobDimensionScore) float64 {
	if len(dimensions) == 0 {
		return 0
	}
	var weightedSum, weightSum float64
	for _, d := range dimensions {
		w := jobDimensionWeight(d.Dimension)
		weightedSum += d.Score * w
		weightSum += w
	}
	if weightSum <= 0 {
		return 0
	}
	return weightedSum / weightSum
}

// jobDimensionWeight 返回维度的固定权重，未知维度返回 0。
func jobDimensionWeight(dimension string) float64 {
	switch strings.TrimSpace(dimension) {
	case "岗位硬技能匹配度":
		return 0.35
	case "简历项目真实性&含金量":
		return 0.25
	case "逻辑思维与表达能力":
		return 0.15
	case "求职动机与岗位认知":
		return 0.10
	case "职业素养与稳定性":
		return 0.10
	case "综合面试印象":
		return 0.05
	default:
		return 0
	}
}

// GenerateJobReportRequest 岗位求职报告生成请求
type GenerateJobReportRequest struct {
	History          []Message
	ResumeText       string
	ResumeParsedJSON string
	JobDescription   string
	IndustryCode     string
	Difficulty       string
	TotalQuestions   int32
}

// GenerateJobReportResponse 岗位求职报告生成响应
type GenerateJobReportResponse struct {
	ReportJSON         string
	OverallScore       float64
	Rating             string
	HireRecommendation string
}

// JobReportResult 岗位求职报告 LLM 结构化输出结果
type JobReportResult struct {
	OverallScore       float64                `json:"overall_score"`
	Rating             string                 `json:"rating"`
	HireRecommendation string                 `json:"hire_recommendation"`
	BasicInfo          JobReportBasicInfo     `json:"basic_info"`
	JDMatchOverview    JobJDMatchOverview     `json:"jd_match_overview"`
	QuestionReviews    []JobQuestionReview    `json:"question_reviews"`
	DimensionScores    []JobDimensionScore    `json:"dimension_scores"`
	CoreAdvantages     []string               `json:"core_advantages"`
	WeaknessesRisks    []JobWeaknessRisk      `json:"weaknesses_risks"`
	HireDecision       JobHireDecision        `json:"hire_decision"`
	OptimizationPlan   []JobOptimizationItem  `json:"optimization_plan"`
	NextRoundQuestions []JobNextRoundQuestion `json:"next_round_questions"`
}

// JobReportBasicInfo 面试档案基础信息
type JobReportBasicInfo struct {
	CandidateName   string  `json:"candidate_name"`
	TargetPosition  string  `json:"target_position"`
	InterviewType   string  `json:"interview_type"`
	DurationSeconds int32   `json:"duration_seconds"`
	TotalQuestions  int32   `json:"total_questions"`
	OverallScore    float64 `json:"overall_score"`
	Rating          string  `json:"rating"`
}

// JobJDMatchOverview 简历&JD匹配总览
type JobJDMatchOverview struct {
	MatchedItems        []string `json:"matched_items"`
	MissingItems        []string `json:"missing_items"`
	HardRequirementsMet bool     `json:"hard_requirements_met"`
	ResumeHighlights    []string `json:"resume_highlights"`
	ResumeHardWounds    []string `json:"resume_hard_wounds"`
}

// JobQuestionReview 逐题复盘
type JobQuestionReview struct {
	QuestionIndex int32    `json:"question_index"`
	Question      string   `json:"question"`
	UserAnswer    string   `json:"user_answer"`
	Score         float64  `json:"score"`
	MaxScore      float64  `json:"max_score"`
	Highlights    []string `json:"highlights"`
	Loopholes     []string `json:"loopholes"`
	Pitfalls      []string `json:"pitfalls"`
	Taboos        []string `json:"taboos"`
}

// JobDimensionScore 维度评分
type JobDimensionScore struct {
	Dimension string  `json:"dimension"`
	Score     float64 `json:"score"`
	Weight    float64 `json:"weight"`
	Comment   string  `json:"comment"`
}

// JobWeaknessRisk 短板风险
type JobWeaknessRisk struct {
	Item   string `json:"item"`
	Level  string `json:"level"`
	Impact string `json:"impact"`
}

// JobHireDecision 最终面试决策
type JobHireDecision struct {
	Decision  string `json:"decision"`
	Rationale string `json:"rationale"`
}

// JobOptimizationItem 面试优化方案
type JobOptimizationItem struct {
	Aspect string `json:"aspect"`
	Detail string `json:"detail"`
}

// JobNextRoundQuestion 复试预测题
type JobNextRoundQuestion struct {
	Question   string `json:"question"`
	Focus      string `json:"focus"`
	Difficulty string `json:"difficulty"`
}
