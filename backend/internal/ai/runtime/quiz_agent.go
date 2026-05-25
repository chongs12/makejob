package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

// providerQuizAnalyzer 基于真实 Provider 分析题目答案。
type providerQuizAnalyzer struct {
	provider ai.AIProvider
	prompts  *promptResolver
	logger   *aiCallLogRecorder
}

// quizAnalysisPayload 表示模型返回的答题分析结构。
type quizAnalysisPayload struct {
	IsCorrect       *bool    `json:"is_correct"`
	Score           *float64 `json:"score"`
	Feedback        string   `json:"feedback"`
	Issues          []string `json:"issues"`
	Improvements    []string `json:"improvements"`
	MistakeTags     []string `json:"mistake_tags"`
	StrengthTags    []string `json:"strength_tags"`
	TimeComplexity  string   `json:"time_complexity"`
	SpaceComplexity string   `json:"space_complexity"`
}

// interviewCodingDiagnosisPayload 表示编程面试诊断结构化输出。
type interviewCodingDiagnosisPayload struct {
	Score          *float64 `json:"score"`
	MistakeTags    []string `json:"mistake_tags"`
	StrengthTags   []string `json:"strength_tags"`
	Evidence       []string `json:"evidence"`
	Suggestions    []string `json:"suggestions"`
	ProcessSummary string   `json:"process_summary"`
}

// quizAnalysisPayloadSchema 返回判题分析结构化输出的 JSON 合同。
func quizAnalysisPayloadSchema() string {
	return `{
  "is_correct": true,
  "score": 80,
  "feedback": "总体反馈",
  "issues": ["问题1", "问题2"],
  "improvements": ["改进1", "改进2"],
  "mistake_tags": ["错因标签1"],
  "strength_tags": ["优势标签1"],
  "time_complexity": "O(n)",
  "space_complexity": "O(1)"
}`
}

// interviewCodingDiagnosisPayloadSchema 返回编程面试诊断结构化输出的 JSON 合同。
func interviewCodingDiagnosisPayloadSchema() string {
	return `{
  "score": 78,
  "mistake_tags": ["状态定义不清"],
  "strength_tags": ["愿意主动验证思路"],
  "evidence": ["多次运行后仍集中修改同一逻辑分支"],
  "suggestions": ["补练边界条件和状态设计"],
  "process_summary": "能够持续迭代，但调试路径还不够稳定。"
}`
}

// newQuizAnalyzer 创建题目分析运行时 Agent。
func newQuizAnalyzer(provider ai.AIProvider, prompts *promptResolver, logger *aiCallLogRecorder) ai.QuizAnalyzer {
	return &providerQuizAnalyzer{
		provider: provider,
		prompts:  prompts,
		logger:   logger,
	}
}

// AnalyzeCode 分析代码题或文本题答案。
func (a *providerQuizAnalyzer) AnalyzeCode(ctx context.Context, code string, language string, question string) (ai.CodeAnalysis, error) {
	if a.shouldUseFallback() {
		return ai.CodeAnalysis{}, fmt.Errorf("ai provider is unavailable")
	}

	traceID := uuid.NewString()
	promptDetails := a.resolvePromptDetails(ctx, map[string]string{
		"language": language,
		"question": question,
	})
	userPrompt := buildQuizAnalysisUserPrompt(code, language, question)
	messages := []ai.Message{
		{
			Role:    "system",
			Content: buildQuizAnalysisSystemPrompt(promptDetails.Prompt),
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	startedAt := time.Now()
	payload, response, usage, err := callStructuredJSON[quizAnalysisPayload](ctx, a.provider, messages, quizAnalysisPayloadSchema())
	if err != nil {
		a.recordCall(ctx, traceID, promptDetails, userPrompt, messages, response, err, startedAt, usage.InputTokens, usage.OutputTokens)
		return ai.CodeAnalysis{}, err
	}

	result := normalizeQuizAnalysis(payload, code, language, question)
	a.recordCall(ctx, traceID, promptDetails, userPrompt, messages, response, nil, startedAt, usage.InputTokens, usage.OutputTokens)
	return result, nil
}

// DiagnoseInterviewCoding 基于最终代码与过程事件生成编程面试诊断。
func (a *providerQuizAnalyzer) DiagnoseInterviewCoding(ctx context.Context, input ai.InterviewCodingDiagnosisInput) (ai.CodingQuestionDiagnosis, error) {
	if a.shouldUseFallback() {
		return ai.CodingQuestionDiagnosis{}, fmt.Errorf("ai provider is unavailable")
	}

	traceID := uuid.NewString()
	promptDetails := a.resolvePromptDetails(ctx, map[string]string{
		"language": input.Language,
		"question": input.Question,
	})
	userPrompt, err := buildInterviewCodingDiagnosisUserPrompt(input)
	if err != nil {
		return ai.CodingQuestionDiagnosis{}, err
	}
	messages := []ai.Message{
		{
			Role:    "system",
			Content: buildInterviewCodingDiagnosisSystemPrompt(promptDetails.Prompt),
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	startedAt := time.Now()
	payload, response, usage, err := callStructuredJSON[interviewCodingDiagnosisPayload](ctx, a.provider, messages, interviewCodingDiagnosisPayloadSchema())
	if err != nil {
		a.recordCall(ctx, traceID, promptDetails, userPrompt, messages, response, err, startedAt, usage.InputTokens, usage.OutputTokens)
		return ai.CodingQuestionDiagnosis{}, err
	}

	result := normalizeInterviewCodingDiagnosis(payload, input)
	a.recordCall(ctx, traceID, promptDetails, userPrompt, messages, response, nil, startedAt, usage.InputTokens, usage.OutputTokens)
	return result, nil
}

// ExplainAnswer 生成题目答案解析。
func (a *providerQuizAnalyzer) ExplainAnswer(ctx context.Context, questionTitle string, questionContent string, correctAnswer string) (string, error) {
	if a.shouldUseFallback() {
		return "", fmt.Errorf("ai provider is unavailable")
	}

	traceID := uuid.NewString()
	promptDetails := a.resolvePromptDetails(ctx, map[string]string{
		"question_title":   questionTitle,
		"question_content": questionContent,
		"correct_answer":   correctAnswer,
	})
	userPrompt := buildQuizExplainUserPrompt(questionTitle, questionContent, correctAnswer)
	messages := []ai.Message{
		{
			Role:    "system",
			Content: buildQuizExplainSystemPrompt(promptDetails.Prompt),
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	startedAt := time.Now()
	resp, err := a.provider.Chat(ctx, messages)
	if err != nil {
		a.recordCall(ctx, traceID, promptDetails, userPrompt, messages, "", err, startedAt, 0, 0)
		return "", err
	}

	content := normalizePlainTextResponse(resp.Content)
	if content == "" {
		a.recordCall(ctx, traceID, promptDetails, userPrompt, messages, resp.Content, fmt.Errorf("empty explain answer response"), startedAt, resp.InputTokens, resp.OutputTokens)
		return "", fmt.Errorf("empty explain answer response")
	}

	a.recordCall(ctx, traceID, promptDetails, userPrompt, messages, resp.Content, nil, startedAt, resp.InputTokens, resp.OutputTokens)
	return content, nil
}

// GenerateHint 生成答题提示。
func (a *providerQuizAnalyzer) GenerateHint(ctx context.Context, questionTitle string, questionContent string) (string, error) {
	if a.shouldUseFallback() {
		return "", fmt.Errorf("ai provider is unavailable")
	}

	traceID := uuid.NewString()
	promptDetails := a.resolvePromptDetails(ctx, map[string]string{
		"question_title":   questionTitle,
		"question_content": questionContent,
	})
	userPrompt := buildQuizHintUserPrompt(questionTitle, questionContent)
	messages := []ai.Message{
		{
			Role:    "system",
			Content: buildQuizHintSystemPrompt(promptDetails.Prompt),
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	startedAt := time.Now()
	resp, err := a.provider.Chat(ctx, messages)
	if err != nil {
		a.recordCall(ctx, traceID, promptDetails, userPrompt, messages, "", err, startedAt, 0, 0)
		return "", err
	}

	content := normalizePlainTextResponse(resp.Content)
	if content == "" {
		a.recordCall(ctx, traceID, promptDetails, userPrompt, messages, resp.Content, fmt.Errorf("empty quiz hint response"), startedAt, resp.InputTokens, resp.OutputTokens)
		return "", fmt.Errorf("empty quiz hint response")
	}

	a.recordCall(ctx, traceID, promptDetails, userPrompt, messages, resp.Content, nil, startedAt, resp.InputTokens, resp.OutputTokens)
	return content, nil
}

// shouldUseFallback 判断当前是否缺少可用 Provider。
func (a *providerQuizAnalyzer) shouldUseFallback() bool {
	return a.provider == nil
}

// resolvePromptDetails 解析题目分析场景的 Prompt 明细。
func (a *providerQuizAnalyzer) resolvePromptDetails(ctx context.Context, vars map[string]string) resolvedPromptDetails {
	if a.prompts == nil {
		return resolvedPromptDetails{
			Prompt: renderPrompt(builtInScenePrompts[model.PromptSceneQuiz], vars),
			Source: "built_in",
		}
	}

	return a.prompts.ResolveDetailsByIndustryID(ctx, model.PromptSceneQuiz, nil, vars)
}

// recordCall 记录一次题目分析链路的运行时模型调用。
func (a *providerQuizAnalyzer) recordCall(
	ctx context.Context,
	traceID string,
	promptDetails resolvedPromptDetails,
	userInput string,
	messages []ai.Message,
	response string,
	err error,
	startedAt time.Time,
	inputTokens int,
	outputTokens int,
) {
	if a.logger == nil {
		return
	}

	a.logger.Record(ctx, runtimeCallLogEntry{
		TraceID:       traceID,
		PromptDetails: promptDetails,
		Request:       messages,
		UserInput:     userInput,
		Model:         a.provider.GetModelName(),
		Output:        response,
		Err:           err,
		StartedAt:     startedAt,
		InputTokens:   inputTokens,
		OutputTokens:  outputTokens,
	})
}

// buildQuizAnalysisSystemPrompt 构造答题分析系统提示词。
func buildQuizAnalysisSystemPrompt(basePrompt string) string {
	return mergePrompt(basePrompt, `请分析这道题的答案，并严格返回 JSON，不要输出 Markdown 或额外解释。JSON 结构如下：
{
  “is_correct”: true,
  “score”: 0,
  “feedback”: “整体评价”,
  “issues”: [“问题1”, “问题2”],
  “improvements”: [“改进建议1”, “改进建议2”],
  “mistake_tags”: [“错因标签1”],
  “strength_tags”: [“优势标签1”],
  “time_complexity”: “O(n)”,
  “space_complexity”: “O(1)”
}
要求：
1. score 必须在 0 到 100 之间。
2. 只评价答案本身是否正确、完整、高质量。不要评价题目描述中与答案无关的内容（如岗位要求、面试题目生成、岗位关联等）。
3. is_correct 应基于答案是否正确回应了题目的核心要求来判定。
4. issues 和 improvements 至少各返回 1 条。
5. mistake_tags 要尽量具体，不要只写”基础不好”这类空泛表述。`)
}

// buildQuizAnalysisUserPrompt 构造答题分析请求。
func buildQuizAnalysisUserPrompt(code string, language string, question string) string {
	return fmt.Sprintf(
		"题目（仅参考核心要求，忽略其中与答案评价无关的岗位描述等内容）：\n%s\n\n答案语言：%s\n\n用户答案：\n%s",
		defaultString(strings.TrimSpace(question), "未提供题目"),
		defaultString(strings.TrimSpace(language), "text"),
		defaultString(strings.TrimSpace(code), "未提供答案"),
	)
}

// buildInterviewCodingDiagnosisSystemPrompt 构造编程面试诊断系统提示词。
func buildInterviewCodingDiagnosisSystemPrompt(basePrompt string) string {
	return mergePrompt(basePrompt, `请根据题目、最终代码和过程事件分析候选人的编程面试表现，并严格返回 JSON，不要输出 Markdown 或额外解释。JSON 结构如下：
{
  "score": 0,
  "mistake_tags": ["错因标签1"],
  "strength_tags": ["优势标签1"],
  "evidence": ["证据1", "证据2"],
  "suggestions": ["建议1", "建议2"],
  "process_summary": "过程总结"
}
要求：
1. score 必须在 0 到 100 之间。
2. 错因标签要尽量具体，优先围绕状态定义、边界条件、索引控制、数据结构选择、复杂度意识和调试路径。
3. evidence 必须引用过程中的现象，不要只写空泛判断。
4. suggestions 给出可执行的补强动作。`)
}

// buildInterviewCodingDiagnosisUserPrompt 构造编程面试诊断请求。
func buildInterviewCodingDiagnosisUserPrompt(input ai.InterviewCodingDiagnosisInput) (string, error) {
	processEventsJSON, err := json.Marshal(input.ProcessEvents)
	if err != nil {
		return "", fmt.Errorf("marshal interview coding process events: %w", err)
	}

	return fmt.Sprintf(
		"题目：\n%s\n\n语言：%s\n\n最终代码：\n%s\n\n用户文字说明：\n%s\n\n过程事件(JSON)：\n%s",
		defaultString(strings.TrimSpace(input.Question), "未提供题目"),
		defaultString(strings.TrimSpace(input.Language), "go"),
		defaultString(strings.TrimSpace(input.FinalCode), "未提供最终代码"),
		defaultString(strings.TrimSpace(input.FinalAnswer), "未提供文字说明"),
		defaultString(strings.TrimSpace(string(processEventsJSON)), "[]"),
	), nil
}

// buildQuizExplainSystemPrompt 构造答案解析系统提示词。
func buildQuizExplainSystemPrompt(basePrompt string) string {
	return mergePrompt(basePrompt, "请用简洁、结构化的中文解释标准答案，帮助用户理解关键思路与常见误区。")
}

// buildQuizExplainUserPrompt 构造答案解析请求。
func buildQuizExplainUserPrompt(questionTitle string, questionContent string, correctAnswer string) string {
	return fmt.Sprintf(
		"题目标题：%s\n\n题目内容：\n%s\n\n参考答案：\n%s\n\n请输出面向学习者的解析。",
		defaultString(strings.TrimSpace(questionTitle), "未提供标题"),
		defaultString(strings.TrimSpace(questionContent), "未提供题目内容"),
		defaultString(strings.TrimSpace(correctAnswer), "未提供参考答案"),
	)
}

// buildQuizHintSystemPrompt 构造答题提示系统提示词。
func buildQuizHintSystemPrompt(basePrompt string) string {
	return mergePrompt(basePrompt, "请给出 2 到 4 条循序渐进的提示，不要直接泄露答案。")
}

// normalizeInterviewCodingDiagnosis 规范化编程面试诊断结构。
func normalizeInterviewCodingDiagnosis(payload interviewCodingDiagnosisPayload, input ai.InterviewCodingDiagnosisInput) ai.CodingQuestionDiagnosis {
	score := 0.0
	if payload.Score != nil {
		score = clampScore(*payload.Score)
	}

	processSummary := strings.TrimSpace(payload.ProcessSummary)
	if processSummary == "" {
		processSummary = "本次编程作答已记录，但模型未返回过程总结。"
	}

	return ai.CodingQuestionDiagnosis{
		Language:       defaultString(strings.TrimSpace(input.Language), "go"),
		Score:          score,
		MistakeTags:    normalizeStringSlice(payload.MistakeTags),
		StrengthTags:   normalizeStringSlice(payload.StrengthTags),
		Evidence:       normalizeStringSlice(payload.Evidence),
		Suggestions:    normalizeStringSlice(payload.Suggestions),
		ProcessSummary: processSummary,
	}
}

// buildQuizHintUserPrompt 构造答题提示请求。
func buildQuizHintUserPrompt(questionTitle string, questionContent string) string {
	return fmt.Sprintf(
		"题目标题：%s\n\n题目内容：\n%s\n\n请给出适合学习者推进思路的提示。",
		defaultString(strings.TrimSpace(questionTitle), "未提供标题"),
		defaultString(strings.TrimSpace(questionContent), "未提供题目内容"),
	)
}

// normalizeQuizAnalysis 规范化模型返回的答题分析。
func normalizeQuizAnalysis(payload quizAnalysisPayload, code string, language string, question string) ai.CodeAnalysis {
	result := buildDefaultQuizAnalysis(code, language, question)

	if payload.Score != nil {
		result.Score = clampScore(*payload.Score)
	}
	if payload.IsCorrect != nil {
		result.IsCorrect = *payload.IsCorrect
	} else {
		result.IsCorrect = result.Score >= 60
	}

	if feedback := strings.TrimSpace(payload.Feedback); feedback != "" {
		result.Feedback = feedback
	}
	if issues := normalizeStringSlice(payload.Issues); len(issues) > 0 {
		result.Issues = issues
	}
	if improvements := normalizeStringSlice(payload.Improvements); len(improvements) > 0 {
		result.Improvements = improvements
	}
	if mistakeTags := normalizeStringSlice(payload.MistakeTags); len(mistakeTags) > 0 {
		result.MistakeTags = mistakeTags
	}
	if strengthTags := normalizeStringSlice(payload.StrengthTags); len(strengthTags) > 0 {
		result.StrengthTags = strengthTags
	}
	if complexity := strings.TrimSpace(payload.TimeComplexity); complexity != "" {
		result.TimeComplexity = complexity
	}
	if complexity := strings.TrimSpace(payload.SpaceComplexity); complexity != "" {
		result.SpaceComplexity = complexity
	}

	if payload.Score == nil && payload.IsCorrect != nil {
		if result.IsCorrect {
			result.Score = maxFloat(result.Score, 75)
		} else {
			result.Score = minFloat(result.Score, 59)
		}
	}

	return result
}

// buildDefaultQuizAnalysis 构造本地兜底的答题分析结果。
func buildDefaultQuizAnalysis(code string, language string, question string) ai.CodeAnalysis {
	trimmed := strings.TrimSpace(code)
	isCodeAnswer := looksLikeCodeAnswer(trimmed, language)
	score := 68.0

	switch {
	case trimmed == "":
		score = 0
	case len([]rune(trimmed)) < 30:
		score = 45
	case len([]rune(trimmed)) < 80:
		score = 58
	case len([]rune(trimmed)) > 300:
		score = 78
	}

	if !isCodeAnswer && len([]rune(trimmed)) >= 60 {
		score += 6
	}
	if isCodeAnswer && (strings.Contains(trimmed, "return") || strings.Contains(trimmed, "func ")) {
		score += 6
	}
	score = clampScore(score)

	timeComplexity, spaceComplexity := inferComplexity(question, trimmed)
	issues := []string{"可以进一步补充边界情况、关键判断或复杂度说明。"}
	improvements := []string{"建议按“思路 -> 关键步骤 -> 结果”组织答案，提高可读性。"}
	mistakeTags := []string{"边界条件生疏"}
	strengthTags := []string{"具备基础解题表达"}

	if trimmed == "" {
		issues = []string{"当前没有提供答案，无法判断解题思路是否正确。"}
		improvements = []string{"建议先给出核心思路或伪代码，再逐步完善实现细节。"}
		mistakeTags = []string{"代码实现不完整"}
		strengthTags = []string{}
	}

	if isCodeAnswer {
		issues = []string{"代码答案建议补充边界处理、复杂度说明或关键设计理由。"}
		improvements = []string{"建议补充注释或解释，说明为何这样设计以及复杂度如何。"}
		mistakeTags = []string{"边界条件生疏", "复杂度意识薄弱"}
		strengthTags = []string{"愿意直接给出实现"}
	}

	return ai.CodeAnalysis{
		IsCorrect:       score >= 60,
		Score:           score,
		Feedback:        buildDefaultQuizFeedback(trimmed, isCodeAnswer),
		Issues:          issues,
		Improvements:    improvements,
		MistakeTags:     mistakeTags,
		StrengthTags:    strengthTags,
		TimeComplexity:  timeComplexity,
		SpaceComplexity: spaceComplexity,
	}
}

// buildDefaultQuizFeedback 构造本地兜底的整体评价。
func buildDefaultQuizFeedback(answer string, isCodeAnswer bool) string {
	if strings.TrimSpace(answer) == "" {
		return "当前未提供有效答案，建议先写出核心思路，再补实现。"
	}
	if isCodeAnswer {
		return "答案已经包含代码实现，建议再补充关键思路、边界处理和复杂度说明。"
	}
	return "答案有基本思路，建议进一步补全关键步骤、正确性证明和复杂度分析。"
}

// looksLikeCodeAnswer 判断答案是否更像代码实现。
func looksLikeCodeAnswer(answer string, language string) bool {
	language = strings.ToLower(strings.TrimSpace(language))
	if language != "" && language != "text" && language != "plain" {
		return true
	}

	markers := []string{"func ", "package ", "class ", "public ", "def ", "=>", "{", "};", "return "}
	for _, marker := range markers {
		if strings.Contains(answer, marker) {
			return true
		}
	}
	return false
}

// inferComplexity 根据题目和答案粗略推断复杂度。
func inferComplexity(question string, answer string) (string, string) {
	lowerQuestion := strings.ToLower(question)
	lowerAnswer := strings.ToLower(answer)

	timeComplexity := "待进一步分析"
	spaceComplexity := "待进一步分析"

	switch {
	case strings.Contains(lowerQuestion, "binary") || strings.Contains(lowerQuestion, "二分"):
		timeComplexity = "O(log n)"
	case strings.Count(lowerAnswer, "for") >= 2 || strings.Count(lowerAnswer, "while") >= 2:
		timeComplexity = "O(n^2)"
	case strings.Contains(lowerQuestion, "sort") || strings.Contains(lowerQuestion, "排序"):
		timeComplexity = "O(n log n)"
	case strings.Contains(lowerAnswer, "for") || strings.Contains(lowerAnswer, "while"):
		timeComplexity = "O(n)"
	}

	switch {
	case strings.Contains(lowerAnswer, "map[") || strings.Contains(lowerAnswer, "make([]") || strings.Contains(lowerAnswer, "new("):
		spaceComplexity = "O(n)"
	case strings.Contains(lowerQuestion, "递归") || strings.Contains(lowerAnswer, "recursive") || strings.Contains(lowerAnswer, "recursion"):
		spaceComplexity = "O(h)"
	case timeComplexity != "待进一步分析":
		spaceComplexity = "O(1)"
	}

	return timeComplexity, spaceComplexity
}

// normalizePlainTextResponse 去掉模型返回中的代码块包裹。
func normalizePlainTextResponse(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```markdown")
	trimmed = strings.TrimPrefix(trimmed, "```text")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

// maxFloat 返回两个浮点数中的较大值。
func maxFloat(a float64, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// minFloat 返回两个浮点数中的较小值。
func minFloat(a float64, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
