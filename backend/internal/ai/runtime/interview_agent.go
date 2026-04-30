package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

// providerInterviewAgent 基于真实 Provider 驱动面试链路，并保留本地兜底能力。
type providerInterviewAgent struct {
	provider ai.AIProvider
	prompts  *promptResolver
	logger   *aiCallLogRecorder
	sessions sync.Map
}

// interviewSessionState 保存真实面试链路的会话状态。
type interviewSessionState struct {
	SessionID     string
	TraceID       string
	Config        ai.InterviewConfig
	IndustryID    *uint
	PromptDetails resolvedPromptDetails

	Questions []ai.InterviewQuestion
	Answers   []string
	Feedbacks []ai.AnswerFeedback

	StartedAt time.Time
	IsActive  bool
}

// interviewQuestionPayload 定义面试题结构化输出。
type interviewQuestionPayload struct {
	Question       string `json:"question"`
	Topic          string `json:"topic"`
	Difficulty     string `json:"difficulty"`
	Type           string `json:"type"`
	Hints          string `json:"hints"`
	Language       string `json:"language"`
	StarterCode    string `json:"starter_code"`
	EditorMode     string `json:"editor_mode"`
	EvaluationMode string `json:"evaluation_mode"`
}

// interviewFeedbackPayload 定义答案反馈结构化输出。
type interviewFeedbackPayload struct {
	Score       float64  `json:"score"`
	IsCorrect   bool     `json:"is_correct"`
	Feedback    string   `json:"feedback"`
	KeyPoints   []string `json:"key_points"`
	Suggestions string   `json:"suggestions"`
	FollowUp    string   `json:"follow_up"`
}

// interviewReportPayload 定义面试报告结构化输出。
type interviewReportPayload struct {
	OverallScore    float64            `json:"overall_score"`
	TotalQuestions  int                `json:"total_questions"`
	CorrectCount    int                `json:"correct_count"`
	DimensionScores map[string]float64 `json:"dimension_scores"`
	Strengths       []string           `json:"strengths"`
	Weaknesses      []string           `json:"weaknesses"`
	Suggestions     []string           `json:"suggestions"`
	Summary         string             `json:"summary"`
}

// interviewQuestionPayloadSchema 返回面试题结构化输出的 JSON 合同。
func interviewQuestionPayloadSchema() string {
	return `{
  "question": "题目内容",
  "topic": "知识点主题",
  "difficulty": "easy|medium|hard",
  "type": "technical|behavioral|coding",
  "hints": "可选提示",
  "language": "go",
  "starter_code": "package main\n\nfunc solve() {\n\t\n}",
  "editor_mode": "code",
  "evaluation_mode": "manual"
}`
}

// interviewFeedbackPayloadSchema 返回答题反馈结构化输出的 JSON 合同。
func interviewFeedbackPayloadSchema() string {
	return `{
  "score": 85,
  "is_correct": true,
  "feedback": "总体评价",
  "key_points": ["关键点1", "关键点2"],
  "suggestions": "改进建议",
  "follow_up": "追问问题"
}`
}

// interviewReportPayloadSchema 返回面试报告结构化输出的 JSON 合同。
func interviewReportPayloadSchema() string {
	return `{
  "overall_score": 82,
  "total_questions": 5,
  "correct_count": 4,
  "dimension_scores": {
    "并发": 80,
    "网络": 84
  },
  "strengths": ["优势1", "优势2"],
  "weaknesses": ["待加强1", "待加强2"],
  "suggestions": ["建议1", "建议2"],
  "summary": "总结说明"
}`
}

// newInterviewAgent 创建 interview 场景专用 Agent。
func newInterviewAgent(provider ai.AIProvider, prompts *promptResolver, logger *aiCallLogRecorder) ai.InterviewAgent {
	return &providerInterviewAgent{
		provider: provider,
		prompts:  prompts,
		logger:   logger,
	}
}

// StartInterview 开始面试，并生成首题。
func (a *providerInterviewAgent) StartInterview(ctx context.Context, config ai.InterviewConfig) (string, ai.InterviewQuestion, error) {
	if a.shouldUseFallback() {
		return "", ai.InterviewQuestion{}, fmt.Errorf("ai provider is unavailable")
	}

	session := &interviewSessionState{
		SessionID:     uuid.NewString(),
		TraceID:       uuid.NewString(),
		Config:        config,
		IndustryID:    a.resolveIndustryID(ctx, config.IndustryCode),
		PromptDetails: a.resolvePromptDetails(ctx, config),
		StartedAt:     time.Now(),
		IsActive:      true,
	}

	firstQuestion, err := a.generateQuestion(ctx, session, 0)
	if err != nil {
		firstQuestion = buildLocalQuestion(session, 0)
	}

	session.Questions = append(session.Questions, firstQuestion)
	session.Answers = make([]string, config.QuestionCount)
	session.Feedbacks = make([]ai.AnswerFeedback, config.QuestionCount)
	a.sessions.Store(session.SessionID, session)

	return session.SessionID, firstQuestion, nil
}

// EvaluateAnswer 评估当前回答，并记录结果。
func (a *providerInterviewAgent) EvaluateAnswer(ctx context.Context, sessionID string, questionIndex int, answer string) (ai.AnswerFeedback, error) {
	session, ok := a.getSession(sessionID)
	if !ok {
		return ai.AnswerFeedback{}, fmt.Errorf("session not found")
	}

	if err := validateInterviewSession(session, questionIndex); err != nil {
		return ai.AnswerFeedback{}, err
	}

	feedback, err := a.generateFeedback(ctx, session, questionIndex, answer)
	if err != nil {
		feedback = buildLocalFeedback(session.Questions[questionIndex], answer)
	}

	session.Answers[questionIndex] = strings.TrimSpace(answer)
	session.Feedbacks[questionIndex] = feedback
	return feedback, nil
}

// GetNextQuestion 生成下一道面试题。
func (a *providerInterviewAgent) GetNextQuestion(ctx context.Context, sessionID string) (ai.InterviewQuestion, bool, error) {
	session, ok := a.getSession(sessionID)
	if !ok {
		return ai.InterviewQuestion{}, false, fmt.Errorf("session not found")
	}

	if !session.IsActive {
		return ai.InterviewQuestion{}, false, fmt.Errorf("session is not active")
	}

	nextIndex := len(session.Questions)
	if nextIndex >= session.Config.QuestionCount {
		return ai.InterviewQuestion{}, false, nil
	}

	question, err := a.generateQuestion(ctx, session, nextIndex)
	if err != nil {
		question = buildLocalQuestion(session, nextIndex)
	}

	session.Questions = append(session.Questions, question)
	return question, true, nil
}

// GenerateReport 生成整场面试的总结报告。
func (a *providerInterviewAgent) GenerateReport(ctx context.Context, sessionID string) (ai.InterviewReport, error) {
	session, ok := a.getSession(sessionID)
	if !ok {
		return ai.InterviewReport{}, fmt.Errorf("session not found")
	}

	report, err := a.generateReport(ctx, session)
	if err != nil {
		report = buildLocalReport(session)
	}

	return report, nil
}

// EndInterview 结束面试并清理会话。
func (a *providerInterviewAgent) EndInterview(ctx context.Context, sessionID string) error {
	session, ok := a.getSession(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}

	session.IsActive = false
	a.sessions.Delete(sessionID)
	return nil
}

// resolvePrompt 解析 interview 场景系统提示词。
func (a *providerInterviewAgent) resolvePromptDetails(ctx context.Context, config ai.InterviewConfig) resolvedPromptDetails {
	vars := map[string]string{
		"industry_code":  config.IndustryCode,
		"difficulty":     config.Difficulty,
		"topics":         strings.Join(config.Topics, ", "),
		"question_count": intToString(config.QuestionCount),
	}

	if a.prompts == nil {
		return resolvedPromptDetails{
			Prompt: renderPrompt(builtInScenePrompts[model.PromptSceneInterview], vars),
			Source: "built_in",
		}
	}

	return a.prompts.ResolveDetailsByIndustryCode(ctx, model.PromptSceneInterview, config.IndustryCode, vars)
}

// resolveIndustryID 根据行业编码解析行业 ID，供日志落库使用。
func (a *providerInterviewAgent) resolveIndustryID(ctx context.Context, industryCode string) *uint {
	if a.prompts == nil {
		return nil
	}

	return a.prompts.lookupIndustryID(ctx, industryCode)
}

// shouldUseFallback 判断当前是否缺少可用 Provider。
func (a *providerInterviewAgent) shouldUseFallback() bool {
	return a.provider == nil
}

// getSession 获取真实面试会话。
func (a *providerInterviewAgent) getSession(sessionID string) (*interviewSessionState, bool) {
	value, ok := a.sessions.Load(sessionID)
	if !ok {
		return nil, false
	}

	session, ok := value.(*interviewSessionState)
	return session, ok
}

// generateQuestion 调用 Provider 生成结构化面试题。
func (a *providerInterviewAgent) generateQuestion(ctx context.Context, session *interviewSessionState, questionIndex int) (ai.InterviewQuestion, error) {
	userPrompt := buildQuestionUserPrompt(session, questionIndex)
	messages := []ai.Message{
		{
			Role:    "system",
			Content: buildQuestionSystemPrompt(session.PromptDetails.Prompt),
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	startedAt := time.Now()
	payload, response, err := callStructuredJSON[interviewQuestionPayload](ctx, a.provider, messages, interviewQuestionPayloadSchema())
	if err != nil {
		a.recordCall(ctx, session, userPrompt, messages, response, err, startedAt)
		return ai.InterviewQuestion{}, err
	}

	question, err := normalizeQuestionPayload(payload, session, questionIndex)
	a.recordCall(ctx, session, userPrompt, messages, response, err, startedAt)
	if err != nil {
		return ai.InterviewQuestion{}, err
	}

	return question, nil
}

// generateFeedback 调用 Provider 生成结构化答案反馈。
func (a *providerInterviewAgent) generateFeedback(ctx context.Context, session *interviewSessionState, questionIndex int, answer string) (ai.AnswerFeedback, error) {
	userPrompt := buildFeedbackUserPrompt(session, questionIndex, answer)
	messages := []ai.Message{
		{
			Role:    "system",
			Content: buildFeedbackSystemPrompt(session.PromptDetails.Prompt),
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	startedAt := time.Now()
	payload, response, err := callStructuredJSON[interviewFeedbackPayload](ctx, a.provider, messages, interviewFeedbackPayloadSchema())
	if err != nil {
		a.recordCall(ctx, session, strings.TrimSpace(answer), messages, response, err, startedAt)
		return ai.AnswerFeedback{}, err
	}

	feedback := normalizeFeedbackPayload(payload)
	a.recordCall(ctx, session, strings.TrimSpace(answer), messages, response, nil, startedAt)
	return feedback, nil
}

// generateReport 调用 Provider 生成结构化面试报告。
func (a *providerInterviewAgent) generateReport(ctx context.Context, session *interviewSessionState) (ai.InterviewReport, error) {
	userPrompt := buildReportUserPrompt(session)
	messages := []ai.Message{
		{
			Role:    "system",
			Content: buildReportSystemPrompt(session.PromptDetails.Prompt),
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	startedAt := time.Now()
	payload, response, err := callStructuredJSON[interviewReportPayload](ctx, a.provider, messages, interviewReportPayloadSchema())
	if err != nil {
		a.recordCall(ctx, session, userPrompt, messages, response, err, startedAt)
		return ai.InterviewReport{}, err
	}

	report := normalizeReportPayload(payload, session)
	a.recordCall(ctx, session, userPrompt, messages, response, nil, startedAt)
	return report, nil
}

// recordCall 记录一次面试链路的运行时模型调用。
func (a *providerInterviewAgent) recordCall(
	ctx context.Context,
	session *interviewSessionState,
	userInput string,
	messages []ai.Message,
	response string,
	err error,
	startedAt time.Time,
) {
	if a.logger == nil || session == nil {
		return
	}

	a.logger.Record(ctx, runtimeCallLogEntry{
		TraceID:       session.TraceID,
		IndustryID:    session.IndustryID,
		PromptDetails: session.PromptDetails,
		Request:       messages,
		UserInput:     userInput,
		Model:         a.provider.GetModelName(),
		Output:        response,
		Err:           err,
		StartedAt:     startedAt,
	})
}

// buildQuestionSystemPrompt 构建首题/下一题生成的系统提示词。
func buildQuestionSystemPrompt(basePrompt string) string {
	return strings.TrimSpace(basePrompt) + `

你正在执行技术模拟面试题生成任务。你必须只返回一个 JSON 对象，不要返回 Markdown、解释、代码块或额外文字。
JSON 结构如下：
{
  "question": "题目正文",
  "topic": "题目主题",
  "difficulty": "easy|medium|hard",
  "type": "technical|behavioral|coding",
  "hints": "一句简短提示，可为空字符串"
}`
}

// buildQuestionUserPrompt 构建面试题生成的用户提示词。
func buildQuestionUserPrompt(session *interviewSessionState, questionIndex int) string {
	topics := session.Config.Topics
	if len(topics) == 0 {
		topics = defaultTopicsByIndustry(session.Config.IndustryCode)
	}

	return fmt.Sprintf(
		"请生成第 %d/%d 道面试题。\n行业: %s\n难度要求: %s\n候选主题: %s\n已出题列表: %s\n要求题目和已出题不重复，保持循序渐进，并结合真实面试场景。",
		questionIndex+1,
		session.Config.QuestionCount,
		session.Config.IndustryCode,
		resolveQuestionDifficulty(session.Config.Difficulty, questionIndex),
		strings.Join(topics, "、"),
		renderAskedQuestions(session.Questions),
	)
}

// buildFeedbackSystemPrompt 构建答案评分的系统提示词。
func buildFeedbackSystemPrompt(basePrompt string) string {
	return strings.TrimSpace(basePrompt) + `

你正在执行技术面试评分任务。你必须只返回一个 JSON 对象，不要返回 Markdown、解释、代码块或额外文字。
JSON 结构如下：
{
  "score": 0-100 的数字,
  "is_correct": true,
  "feedback": "对回答质量的评价",
  "key_points": ["本题关键点1", "本题关键点2"],
  "suggestions": "明确、可执行的改进建议",
  "follow_up": "可选追问，没有则返回空字符串"
}`
}

// buildFeedbackUserPrompt 构建答案评分的用户提示词。
func buildFeedbackUserPrompt(session *interviewSessionState, questionIndex int, answer string) string {
	question := session.Questions[questionIndex]
	return fmt.Sprintf(
		"请严格评估这道面试题的回答。\n题目: %s\n主题: %s\n难度: %s\n题型: %s\n候选人回答: %s\n请结合岗位场景输出客观评分，并给出关键知识点和改进建议。",
		question.Question,
		question.Topic,
		question.Difficulty,
		question.Type,
		strings.TrimSpace(answer),
	)
}

// buildReportSystemPrompt 构建面试报告的系统提示词。
func buildReportSystemPrompt(basePrompt string) string {
	return strings.TrimSpace(basePrompt) + `

你正在执行技术面试总结任务。你必须只返回一个 JSON 对象，不要返回 Markdown、解释、代码块或额外文字。
JSON 结构如下：
{
  "overall_score": 0-100 的数字,
  "total_questions": 题目总数,
  "correct_count": 回答达标的题目数,
  "dimension_scores": {"主题A": 80, "主题B": 65},
  "strengths": ["优势1", "优势2"],
  "weaknesses": ["不足1", "不足2"],
  "suggestions": ["建议1", "建议2"],
  "summary": "简洁总结"
}`
}

// buildReportUserPrompt 构建面试报告的用户提示词。
func buildReportUserPrompt(session *interviewSessionState) string {
	var history []string
	for index, question := range session.Questions {
		answer := ""
		if index < len(session.Answers) {
			answer = strings.TrimSpace(session.Answers[index])
		}
		feedback := ai.AnswerFeedback{}
		if index < len(session.Feedbacks) {
			feedback = session.Feedbacks[index]
		}
		history = append(history, fmt.Sprintf(
			"第%d题: %s\n回答: %s\n评分: %.0f\n反馈: %s",
			index+1,
			question.Question,
			defaultString(answer, "未作答"),
			feedback.Score,
			defaultString(strings.TrimSpace(feedback.Feedback), "暂无"),
		))
	}

	return fmt.Sprintf(
		"请基于以下整场面试记录生成总结报告。\n行业: %s\n难度: %s\n面试记录:\n%s",
		session.Config.IndustryCode,
		session.Config.Difficulty,
		strings.Join(history, "\n\n"),
	)
}

// decodeJSONPayload 解析模型输出中的 JSON 对象。
func decodeJSONPayload[T any](raw string) (T, error) {
	var payload T
	cleaned := extractJSONObject(raw)
	if cleaned == "" {
		return payload, fmt.Errorf("json payload not found")
	}

	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return payload, err
	}

	return payload, nil
}

// extractJSONObject 提取文本中的 JSON 对象主体。
func extractJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return ""
	}

	return strings.TrimSpace(trimmed[start : end+1])
}

// normalizeQuestionPayload 规范化题目结构，确保字段可用。
func normalizeQuestionPayload(payload interviewQuestionPayload, session *interviewSessionState, questionIndex int) (ai.InterviewQuestion, error) {
	question := ai.InterviewQuestion{
		Question:       strings.TrimSpace(payload.Question),
		Topic:          strings.TrimSpace(payload.Topic),
		Difficulty:     strings.TrimSpace(payload.Difficulty),
		Type:           strings.TrimSpace(payload.Type),
		Hints:          strings.TrimSpace(payload.Hints),
		Language:       strings.TrimSpace(payload.Language),
		StarterCode:    strings.TrimSpace(payload.StarterCode),
		EditorMode:     strings.TrimSpace(payload.EditorMode),
		EvaluationMode: strings.TrimSpace(payload.EvaluationMode),
	}

	if question.Question == "" {
		return ai.InterviewQuestion{}, fmt.Errorf("empty interview question")
	}
	if question.Topic == "" {
		question.Topic = fallbackTopic(session.Config, questionIndex)
	}
	if question.Difficulty == "" {
		question.Difficulty = resolveQuestionDifficulty(session.Config.Difficulty, questionIndex)
	}
	if question.Type == "" {
		question.Type = "technical"
	}
	if question.Type == "coding" {
		if question.Language == "" {
			question.Language = fallbackCodingLanguage(session.Config.IndustryCode)
		}
		if question.StarterCode == "" {
			question.StarterCode = buildFallbackStarterCode(question.Language)
		}
		if question.EditorMode == "" {
			question.EditorMode = "code"
		}
		if question.EvaluationMode == "" {
			question.EvaluationMode = "manual"
		}
	}

	return question, nil
}

// normalizeFeedbackPayload 规范化反馈结构。
func normalizeFeedbackPayload(payload interviewFeedbackPayload) ai.AnswerFeedback {
	score := clampScore(payload.Score)
	isCorrect := payload.IsCorrect
	if !payload.IsCorrect && score >= 60 {
		isCorrect = true
	}

	return ai.AnswerFeedback{
		Score:       score,
		IsCorrect:   isCorrect,
		Feedback:    defaultString(strings.TrimSpace(payload.Feedback), "回答整体可理解，但仍需结合关键知识点继续完善。"),
		KeyPoints:   normalizeStringSlice(payload.KeyPoints),
		Suggestions: defaultString(strings.TrimSpace(payload.Suggestions), "建议补充原理、边界情况和实际应用场景。"),
		FollowUp:    strings.TrimSpace(payload.FollowUp),
	}
}

// normalizeReportPayload 规范化面试报告结构。
func normalizeReportPayload(payload interviewReportPayload, session *interviewSessionState) ai.InterviewReport {
	report := ai.InterviewReport{
		OverallScore:    clampScore(payload.OverallScore),
		TotalQuestions:  payload.TotalQuestions,
		CorrectCount:    payload.CorrectCount,
		DimensionScores: payload.DimensionScores,
		Strengths:       normalizeStringSlice(payload.Strengths),
		Weaknesses:      normalizeStringSlice(payload.Weaknesses),
		Suggestions:     normalizeStringSlice(payload.Suggestions),
		Summary:         strings.TrimSpace(payload.Summary),
	}

	if report.TotalQuestions <= 0 {
		report.TotalQuestions = len(session.Questions)
	}
	if report.DimensionScores == nil {
		report.DimensionScores = map[string]float64{}
	}
	if report.Summary == "" {
		report.Summary = buildLocalReport(session).Summary
	}
	if len(report.Strengths) == 0 && len(report.Weaknesses) == 0 {
		local := buildLocalReport(session)
		report.Strengths = local.Strengths
		report.Weaknesses = local.Weaknesses
		if len(report.Suggestions) == 0 {
			report.Suggestions = local.Suggestions
		}
		if len(report.DimensionScores) == 0 {
			report.DimensionScores = local.DimensionScores
		}
		if report.CorrectCount == 0 {
			report.CorrectCount = local.CorrectCount
		}
	}

	return report
}

// validateInterviewSession 校验当前会话和题目索引是否合法。
func validateInterviewSession(session *interviewSessionState, questionIndex int) error {
	if !session.IsActive {
		return fmt.Errorf("session is not active")
	}
	if questionIndex < 0 || questionIndex >= len(session.Questions) {
		return fmt.Errorf("invalid question index: %d", questionIndex)
	}
	return nil
}

// buildLocalQuestion 在模型不可用时生成本地兜底题目。
func buildLocalQuestion(session *interviewSessionState, questionIndex int) ai.InterviewQuestion {
	topic := fallbackTopic(session.Config, questionIndex)
	difficulty := resolveQuestionDifficulty(session.Config.Difficulty, questionIndex)
	templateIndex := questionIndex % len(localInterviewQuestionTemplates)

	return ai.InterviewQuestion{
		Question:       fmt.Sprintf(localInterviewQuestionTemplates[templateIndex], topic),
		Topic:          topic,
		Difficulty:     difficulty,
		Type:           fallbackQuestionType(questionIndex),
		Hints:          "请从定义、原理、使用场景和常见问题四个角度组织回答。",
		Language:       fallbackCodingLanguage(session.Config.IndustryCode),
		StarterCode:    buildFallbackStarterCode(fallbackCodingLanguage(session.Config.IndustryCode)),
		EditorMode:     "code",
		EvaluationMode: "manual",
	}
}

// buildLocalFeedback 在模型不可用时生成本地兜底评分。
func buildLocalFeedback(question ai.InterviewQuestion, answer string) ai.AnswerFeedback {
	answer = strings.TrimSpace(answer)
	score := 55.0

	switch {
	case len(answer) >= 180:
		score = 88
	case len(answer) >= 100:
		score = 78
	case len(answer) >= 40:
		score = 68
	}

	if strings.Contains(answer, "例如") || strings.Contains(strings.ToLower(answer), "example") {
		score += 4
	}
	if strings.Contains(answer, "原理") || strings.Contains(answer, "why") {
		score += 4
	}

	score = clampScore(score)
	return ai.AnswerFeedback{
		Score:       score,
		IsCorrect:   score >= 60,
		Feedback:    fmt.Sprintf("回答覆盖了“%s”相关内容，但还需要进一步突出核心原理和工程场景。", question.Topic),
		KeyPoints:   []string{question.Topic, "原理说明", "使用场景", "边界情况"},
		Suggestions: "建议按“概念定义 -> 实现原理 -> 使用场景 -> 常见坑点”的顺序组织答案。",
		FollowUp:    "",
	}
}

// buildLocalReport 在模型不可用时聚合本地面试报告。
func buildLocalReport(session *interviewSessionState) ai.InterviewReport {
	dimensionScores := make(map[string]float64)
	dimensionCounts := make(map[string]int)
	var totalScore float64
	var answered int
	var correctCount int

	for index, question := range session.Questions {
		if index >= len(session.Answers) || strings.TrimSpace(session.Answers[index]) == "" {
			continue
		}

		feedback := session.Feedbacks[index]
		totalScore += feedback.Score
		answered++
		if feedback.IsCorrect {
			correctCount++
		}

		dimensionScores[question.Topic] += feedback.Score
		dimensionCounts[question.Topic]++
	}

	for topic, score := range dimensionScores {
		dimensionScores[topic] = roundScore(score / float64(maxInt(dimensionCounts[topic], 1)))
	}

	overallScore := 0.0
	if answered > 0 {
		overallScore = roundScore(totalScore / float64(answered))
	}

	strengths, weaknesses := summarizePerformance(dimensionScores)
	suggestions := []string{
		"优先补强低分主题，避免只记结论不理解原理。",
		"每道题回答时加入真实项目例子，提升说服力。",
		"复盘回答结构，确保先结论后展开，再补充边界情况。",
	}

	return ai.InterviewReport{
		OverallScore:    overallScore,
		TotalQuestions:  len(session.Questions),
		CorrectCount:    correctCount,
		DimensionScores: dimensionScores,
		Strengths:       strengths,
		Weaknesses:      weaknesses,
		Suggestions:     suggestions,
		Summary: fmt.Sprintf(
			"本次模拟面试共回答 %d/%d 题，综合得分 %.0f 分。建议继续围绕薄弱主题做结构化复盘。",
			answered,
			len(session.Questions),
			overallScore,
		),
	}
}

// renderAskedQuestions 将已出题列表渲染为文本。
func renderAskedQuestions(questions []ai.InterviewQuestion) string {
	if len(questions) == 0 {
		return "暂无"
	}

	items := make([]string, 0, len(questions))
	for index, question := range questions {
		items = append(items, fmt.Sprintf("%d. %s", index+1, strings.TrimSpace(question.Question)))
	}
	return strings.Join(items, "；")
}

// fallbackTopic 选择当前题目的兜底主题。
func fallbackTopic(config ai.InterviewConfig, questionIndex int) string {
	if len(config.Topics) > 0 {
		return strings.TrimSpace(config.Topics[questionIndex%len(config.Topics)])
	}

	topics := defaultTopicsByIndustry(config.IndustryCode)
	return topics[questionIndex%len(topics)]
}

// defaultTopicsByIndustry 根据行业返回默认主题。
func defaultTopicsByIndustry(industryCode string) []string {
	switch strings.ToLower(strings.TrimSpace(industryCode)) {
	case "frontend":
		return []string{"JavaScript", "浏览器原理", "性能优化", "工程化"}
	case "java":
		return []string{"JVM", "并发编程", "集合框架", "Spring"}
	case "python":
		return []string{"语言特性", "并发模型", "Web 开发", "性能优化"}
	default:
		return []string{"基础语法", "并发编程", "工程实践", "性能优化"}
	}
}

// resolveQuestionDifficulty 解析 mixed 难度下每题的目标难度。
func resolveQuestionDifficulty(raw string, questionIndex int) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "easy", "medium", "hard":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		switch {
		case questionIndex == 0:
			return "easy"
		case questionIndex%3 == 2:
			return "hard"
		default:
			return "medium"
		}
	}
}

// fallbackQuestionType 给本地题目选择题型。
func fallbackQuestionType(questionIndex int) string {
	switch questionIndex % 4 {
	case 2:
		return "coding"
	case 3:
		return "behavioral"
	default:
		return "technical"
	}
}

// fallbackCodingLanguage 根据当前行业编码返回编程题默认语言。
func fallbackCodingLanguage(industryCode string) string {
	switch strings.ToLower(strings.TrimSpace(industryCode)) {
	case "java":
		return "java"
	case "frontend":
		return "javascript"
	case "python", "ai":
		return "python"
	default:
		return "go"
	}
}

// buildFallbackStarterCode 为编程题提供最小可编辑模板。
func buildFallbackStarterCode(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "java":
		return "class Solution {\n    public int solve(int[] nums) {\n        return 0;\n    }\n}"
	case "javascript", "js", "typescript", "ts":
		return "function solve(nums) {\n  return 0\n}"
	case "python":
		return "def solve(nums):\n    return 0"
	default:
		return "package main\n\nfunc solve(nums []int) int {\n\treturn 0\n}"
	}
}

// summarizePerformance 根据维度分数总结优劣势。
func summarizePerformance(dimensionScores map[string]float64) ([]string, []string) {
	var strengths []string
	var weaknesses []string

	for topic, score := range dimensionScores {
		switch {
		case score >= 80:
			strengths = append(strengths, fmt.Sprintf("%s 表现较强（%.0f 分）", topic, score))
		case score < 65:
			weaknesses = append(weaknesses, fmt.Sprintf("%s 仍需加强（%.0f 分）", topic, score))
		}
	}

	if len(strengths) == 0 {
		strengths = append(strengths, "回答态度稳定，具备继续提升的基础。")
	}
	if len(weaknesses) == 0 {
		weaknesses = append(weaknesses, "整体表现较均衡，建议继续挑战更深层问题。")
	}

	return strengths, weaknesses
}

// normalizeStringSlice 清理字符串数组中的空项。
func normalizeStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// defaultString 返回非空字符串，否则使用兜底值。
func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

// clampScore 将分数限制在 0 到 100 之间。
func clampScore(score float64) float64 {
	switch {
	case score < 0:
		return 0
	case score > 100:
		return 100
	default:
		return roundScore(score)
	}
}

// roundScore 对分数进行两位小数规整。
func roundScore(score float64) float64 {
	return math.Round(score*100) / 100
}

// maxInt 返回两个整数中的较大值。
func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

var localInterviewQuestionTemplates = []string{
	"请解释 %s 的核心概念、常见使用场景以及容易踩坑的地方。",
	"如果你要向初级同学讲清楚 %s，你会怎样从原理、实现和实践三个层面展开？",
	"请结合一个真实项目场景，说明 %s 为什么重要，以及你会如何落地。",
	"针对 %s，请先给出结论，再补充边界情况、性能影响和调试思路。",
}
