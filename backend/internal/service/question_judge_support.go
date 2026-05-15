package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"makejob-backend/internal/model"
)

const (
	// QuestionEvaluationModeAnalysisOnly 表示仅做 AI/解析点评，不做确定性用例判题。
	QuestionEvaluationModeAnalysisOnly = "analysis_only"
	// QuestionEvaluationModeTestcase 表示通过测试用例对代码进行确定性判题。
	QuestionEvaluationModeTestcase = "testcase"
)

// QuestionJudgeConfig 描述编程题的统一判题配置。
type QuestionJudgeConfig struct {
	EvaluationMode   string                      `json:"evaluation_mode"`
	DefaultLanguage  string                      `json:"default_language"`
	AllowedLanguages []string                    `json:"allowed_languages"`
	StarterCode      string                      `json:"starter_code"`
	PublicTestCases  []QuestionTestCase          `json:"public_test_cases"`
	HiddenTestCases  []QuestionTestCase          `json:"hidden_test_cases"`
	ReferenceAnswers []QuestionReferenceSolution `json:"reference_solutions"`
	TimeLimitMS      int                         `json:"time_limit_ms"`
	MemoryLimitMB    int                         `json:"memory_limit_mb"`
}

// QuestionTestCase 描述一条输入输出型测试用例。
type QuestionTestCase struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	Description    string `json:"description,omitempty"`
}

// QuestionReferenceSolution 描述一份可用于教学和兜底的参考实现。
type QuestionReferenceSolution struct {
	Language    string `json:"language"`
	Title       string `json:"title,omitempty"`
	Code        string `json:"code"`
	Explanation string `json:"explanation,omitempty"`
}

// QuestionTestCaseResult 描述单条测试用例的执行结果。
type QuestionTestCaseResult struct {
	Index          int    `json:"index"`
	Description    string `json:"description,omitempty"`
	Input          string `json:"input,omitempty"`
	ExpectedOutput string `json:"expected_output,omitempty"`
	ActualOutput   string `json:"actual_output,omitempty"`
	Passed         bool   `json:"passed"`
	ErrorOutput    string `json:"error_output,omitempty"`
}

// QuestionJudgeSummary 描述一次代码运行或提交后的用例汇总结果。
type QuestionJudgeSummary struct {
	Mode          string                   `json:"mode"`
	PassedCount   int                      `json:"passed_count"`
	TotalCount    int                      `json:"total_count"`
	AllPassed     bool                     `json:"all_passed"`
	CaseResults   []QuestionTestCaseResult `json:"case_results,omitempty"`
	TimeLimitMS   int                      `json:"time_limit_ms,omitempty"`
	MemoryLimitMB int                      `json:"memory_limit_mb,omitempty"`
}

// normalizeQuestionJudgeConfig 规范化编程题判题配置，并兼容旧题默认值。
func normalizeQuestionJudgeConfig(value *QuestionJudgeConfig, question *model.Question) *QuestionJudgeConfig {
	if question == nil || !question.IsCode() {
		return nil
	}

	config := &QuestionJudgeConfig{}
	if value != nil {
		*config = *value
	}

	config.EvaluationMode = normalizeQuestionEvaluationMode(config.EvaluationMode)
	config.DefaultLanguage = normalizeQuestionJudgeLanguage(config.DefaultLanguage)
	config.AllowedLanguages = normalizeQuestionJudgeLanguages(config.AllowedLanguages, config.DefaultLanguage)
	config.StarterCode = strings.TrimSpace(config.StarterCode)
	config.PublicTestCases = normalizeQuestionTestCases(config.PublicTestCases)
	config.HiddenTestCases = normalizeQuestionTestCases(config.HiddenTestCases)
	config.ReferenceAnswers = normalizeQuestionReferenceSolutions(config.ReferenceAnswers)
	if config.TimeLimitMS <= 0 {
		config.TimeLimitMS = 2000
	}
	if config.MemoryLimitMB <= 0 {
		config.MemoryLimitMB = 128
	}
	if config.DefaultLanguage == "" {
		config.DefaultLanguage = detectQuestionLanguage(question)
	}
	if len(config.AllowedLanguages) == 0 {
		config.AllowedLanguages = []string{config.DefaultLanguage}
	}
	return config
}

// buildFallbackQuestionJudgeConfig 为旧编程题构造最小兼容判题配置。
func buildFallbackQuestionJudgeConfig(question *model.Question) *QuestionJudgeConfig {
	if question == nil || !question.IsCode() {
		return nil
	}

	return normalizeQuestionJudgeConfig(&QuestionJudgeConfig{
		EvaluationMode:   QuestionEvaluationModeAnalysisOnly,
		DefaultLanguage:  detectQuestionLanguage(question),
		AllowedLanguages: []string{detectQuestionLanguage(question)},
	}, question)
}

// parseQuestionJudgeConfig 解析编程题判题配置，并在缺失时回退到旧模式兼容配置。
func parseQuestionJudgeConfig(raw string, question *model.Question) *QuestionJudgeConfig {
	if question == nil || !question.IsCode() {
		return nil
	}

	var config QuestionJudgeConfig
	if strings.TrimSpace(raw) != "" && json.Unmarshal([]byte(raw), &config) == nil {
		return normalizeQuestionJudgeConfig(&config, question)
	}
	return buildFallbackQuestionJudgeConfig(question)
}

// marshalQuestionJudgeConfig 将判题配置写回数据库字段，并在非编程题场景下忽略内容。
func marshalQuestionJudgeConfig(value *QuestionJudgeConfig, question *model.Question) (string, error) {
	if question == nil || !question.IsCode() {
		return "", nil
	}

	normalized := normalizeQuestionJudgeConfig(value, question)
	if normalized == nil {
		return "", nil
	}
	content, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// validateQuestionJudgeConfig 校验编程题判题配置是否满足当前判题模式要求。
func validateQuestionJudgeConfig(question *model.Question, config *QuestionJudgeConfig) error {
	if question == nil || !question.IsCode() {
		return nil
	}

	if config == nil {
		return fmt.Errorf("编程题缺少判题配置")
	}
	if config.EvaluationMode == QuestionEvaluationModeTestcase {
		if config.DefaultLanguage == "" {
			return fmt.Errorf("测试用例判题模式必须提供默认语言")
		}
		if len(config.PublicTestCases) == 0 {
			return fmt.Errorf("测试用例判题模式至少需要一条公开样例")
		}
		if len(config.HiddenTestCases) == 0 {
			return fmt.Errorf("测试用例判题模式至少需要一条隐藏测试用例")
		}
		if len(config.ReferenceAnswers) == 0 {
			return fmt.Errorf("测试用例判题模式至少需要一份参考实现")
		}
	}
	return nil
}

// normalizeQuestionEvaluationMode 将判题模式收敛为已知枚举。
func normalizeQuestionEvaluationMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case QuestionEvaluationModeTestcase:
		return QuestionEvaluationModeTestcase
	default:
		return QuestionEvaluationModeAnalysisOnly
	}
}

// normalizeQuestionJudgeLanguage 统一清理判题配置中的语言字段。
func normalizeQuestionJudgeLanguage(raw string) string {
	language := strings.ToLower(strings.TrimSpace(raw))
	switch language {
	case "golang":
		return "go"
	case "c++":
		return "cpp"
	default:
		return language
	}
}

// normalizeQuestionJudgeLanguages 清理支持语言列表，并保证默认语言可用。
func normalizeQuestionJudgeLanguages(values []string, defaultLanguage string) []string {
	result := make([]string, 0, len(values)+1)
	seen := make(map[string]struct{}, len(values)+1)
	appendLanguage := func(value string) {
		normalized := normalizeQuestionJudgeLanguage(value)
		if normalized == "" {
			return
		}
		if _, exists := seen[normalized]; exists {
			return
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	for _, value := range values {
		appendLanguage(value)
	}
	appendLanguage(defaultLanguage)
	return result
}

// normalizeQuestionTestCases 清理测试用例中的空值与空白。
func normalizeQuestionTestCases(values []QuestionTestCase) []QuestionTestCase {
	result := make([]QuestionTestCase, 0, len(values))
	for _, item := range values {
		normalized := QuestionTestCase{
			Input:          strings.TrimSpace(item.Input),
			ExpectedOutput: strings.TrimSpace(item.ExpectedOutput),
			Description:    strings.TrimSpace(item.Description),
		}
		if normalized.Input == "" && normalized.ExpectedOutput == "" {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

// normalizeQuestionReferenceSolutions 清理参考实现列表中的空值与语言别名。
func normalizeQuestionReferenceSolutions(values []QuestionReferenceSolution) []QuestionReferenceSolution {
	result := make([]QuestionReferenceSolution, 0, len(values))
	for _, item := range values {
		normalized := QuestionReferenceSolution{
			Language:    normalizeQuestionJudgeLanguage(item.Language),
			Title:       strings.TrimSpace(item.Title),
			Code:        strings.TrimSpace(item.Code),
			Explanation: strings.TrimSpace(item.Explanation),
		}
		if normalized.Language == "" || normalized.Code == "" {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

// parseQuestionJudgeConfigPayload 将任意动态 JSON 值解析为判题配置对象，供导入链路复用。
func parseQuestionJudgeConfigPayload(value any) *QuestionJudgeConfig {
	if value == nil {
		return nil
	}

	switch typed := value.(type) {
	case *QuestionJudgeConfig:
		return typed
	case QuestionJudgeConfig:
		config := typed
		return &config
	case map[string]any:
		return parseQuestionJudgeConfigMapPayload(typed)
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		var config QuestionJudgeConfig
		if json.Unmarshal([]byte(typed), &config) == nil {
			return &config
		}
		return parseQuestionJudgeConfigTextPayload(typed)
	default:
		payload, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		var config QuestionJudgeConfig
		if json.Unmarshal(payload, &config) == nil {
			if normalized := parseQuestionJudgeConfigMapPayload(mustDecodeQuestionJudgeConfigMap(payload)); normalized != nil {
				return normalized
			}
			return &config
		}
	}

	return nil
}

// parseQuestionJudgeConfigMapPayload 解析对象型判题配置，并兼容 solution_code 等历史字段别名。
func parseQuestionJudgeConfigMapPayload(value map[string]any) *QuestionJudgeConfig {
	if len(value) == 0 {
		return nil
	}

	normalized := cloneQuestionJudgeConfigMap(value)
	normalizeQuestionJudgeReferenceAnswerAliases(normalized)

	payload, err := json.Marshal(normalized)
	if err != nil {
		return nil
	}

	var config QuestionJudgeConfig
	if json.Unmarshal(payload, &config) != nil {
		return nil
	}
	return &config
}

// mustDecodeQuestionJudgeConfigMap 将 JSON 内容尽力回解为对象，失败时返回空 map 供上层走原始回退。
func mustDecodeQuestionJudgeConfigMap(payload []byte) map[string]any {
	var value map[string]any
	if json.Unmarshal(payload, &value) != nil {
		return map[string]any{}
	}
	return value
}

// cloneQuestionJudgeConfigMap 深拷贝一份对象型判题配置，避免在原始动态值上原地改写。
func cloneQuestionJudgeConfigMap(value map[string]any) map[string]any {
	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = cloneQuestionJudgeConfigValue(item)
	}
	return cloned
}

// cloneQuestionJudgeConfigValue 深拷贝判题配置中的任意值，兼容对象和数组嵌套。
func cloneQuestionJudgeConfigValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneQuestionJudgeConfigMap(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, cloneQuestionJudgeConfigValue(item))
		}
		return items
	default:
		return value
	}
}

// normalizeQuestionJudgeReferenceAnswerAliases 将 reference_solutions 中的 solution_code 等别名统一收敛到 code。
func normalizeQuestionJudgeReferenceAnswerAliases(value map[string]any) {
	rawItems, ok := value["reference_solutions"]
	if !ok || rawItems == nil {
		return
	}

	items, ok := rawItems.([]any)
	if !ok {
		return
	}

	for index, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}

		if _, exists := item["code"]; exists {
			items[index] = item
			continue
		}
		if code, exists := item["solution_code"]; exists {
			item["code"] = code
		}
		items[index] = item
	}
	value["reference_solutions"] = items
}

// parseQuestionJudgeConfigTextPayload 解析 YAML-like/Markdown-like 的判题配置文本，尽量保留编程题关键结构。
func parseQuestionJudgeConfigTextPayload(raw string) *QuestionJudgeConfig {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 0 {
		return nil
	}

	config := &QuestionJudgeConfig{}
	currentSection := ""
	currentBlockField := ""
	currentCase := QuestionTestCase{}
	currentReference := QuestionReferenceSolution{}
	hasAnyField := false

	flushCase := func() {
		if strings.TrimSpace(currentCase.Input) == "" && strings.TrimSpace(currentCase.ExpectedOutput) == "" && strings.TrimSpace(currentCase.Description) == "" {
			currentCase = QuestionTestCase{}
			return
		}
		switch currentSection {
		case "public_test_cases":
			config.PublicTestCases = append(config.PublicTestCases, currentCase)
		case "hidden_test_cases":
			config.HiddenTestCases = append(config.HiddenTestCases, currentCase)
		}
		currentCase = QuestionTestCase{}
	}
	flushReference := func() {
		if strings.TrimSpace(currentReference.Language) == "" && strings.TrimSpace(currentReference.Code) == "" && strings.TrimSpace(currentReference.Explanation) == "" {
			currentReference = QuestionReferenceSolution{}
			return
		}
		config.ReferenceAnswers = append(config.ReferenceAnswers, currentReference)
		currentReference = QuestionReferenceSolution{}
	}
	appendBlockValue := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		switch currentBlockField {
		case "starter_code":
			config.StarterCode = appendQuestionJudgeConfigTextBlock(config.StarterCode, value)
			hasAnyField = true
		case "reference_code":
			currentReference.Code = appendQuestionJudgeConfigTextBlock(currentReference.Code, value)
			hasAnyField = true
		case "reference_explanation":
			currentReference.Explanation = appendQuestionJudgeConfigTextBlock(currentReference.Explanation, value)
			hasAnyField = true
		}
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" {
			continue
		}

		normalizedLine := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if key, value, ok := splitQuestionPipelineKeyValue(normalizedLine); ok {
			field := normalizeQuestionPipelineFieldKey(key)
			currentBlockField = ""

			switch field {
			case "judgeconfig", "judge_config", "judge-config", "judgeconfigraw", "判题配置", "测评配置", "测试配置":
				currentSection = "judge_config"
			case "evaluationmode":
				config.EvaluationMode = strings.TrimSpace(value)
				hasAnyField = true
			case "defaultlanguage":
				config.DefaultLanguage = strings.TrimSpace(value)
				hasAnyField = true
			case "allowedlanguages":
				config.AllowedLanguages = parseQuestionJudgeConfigLanguageList(value)
				hasAnyField = true
			case "timelimitms":
				if parsed, err := parseQuestionJudgeConfigInteger(value); err == nil {
					config.TimeLimitMS = parsed
					hasAnyField = true
				}
			case "memorylimitmb":
				if parsed, err := parseQuestionJudgeConfigInteger(value); err == nil {
					config.MemoryLimitMB = parsed
					hasAnyField = true
				}
			case "startercode":
				if value == "|" {
					currentSection = "starter_code"
					currentBlockField = "starter_code"
				} else {
					config.StarterCode = appendQuestionJudgeConfigTextBlock(config.StarterCode, value)
					hasAnyField = true
				}
			case "publictestcases":
				flushCase()
				flushReference()
				currentSection = "public_test_cases"
			case "hiddentestcases":
				flushCase()
				flushReference()
				currentSection = "hidden_test_cases"
			case "referencesolutions":
				flushCase()
				flushReference()
				currentSection = "reference_solutions"
			case "input":
				if currentSection == "public_test_cases" || currentSection == "hidden_test_cases" {
					if strings.TrimSpace(currentCase.Input) != "" || strings.TrimSpace(currentCase.ExpectedOutput) != "" || strings.TrimSpace(currentCase.Description) != "" {
						flushCase()
					}
					currentCase.Input = strings.TrimSpace(value)
					hasAnyField = true
				}
			case "expectedoutput":
				if currentSection == "public_test_cases" || currentSection == "hidden_test_cases" {
					currentCase.ExpectedOutput = strings.TrimSpace(value)
					hasAnyField = true
				}
			case "description":
				if currentSection == "public_test_cases" || currentSection == "hidden_test_cases" {
					currentCase.Description = strings.TrimSpace(value)
					hasAnyField = true
				}
			case "language":
				if currentSection == "reference_solutions" {
					if strings.TrimSpace(currentReference.Language) != "" || strings.TrimSpace(currentReference.Code) != "" || strings.TrimSpace(currentReference.Explanation) != "" {
						flushReference()
					}
					currentReference.Language = strings.TrimSpace(value)
					hasAnyField = true
				}
			case "title":
				if currentSection == "reference_solutions" {
					currentReference.Title = strings.TrimSpace(value)
					hasAnyField = true
				}
			case "code":
				if currentSection == "reference_solutions" {
					if value == "|" {
						currentBlockField = "reference_code"
					} else {
						currentReference.Code = appendQuestionJudgeConfigTextBlock(currentReference.Code, value)
						hasAnyField = true
					}
				}
			case "explanation":
				if currentSection == "reference_solutions" {
					if value == "|" {
						currentBlockField = "reference_explanation"
					} else {
						currentReference.Explanation = appendQuestionJudgeConfigTextBlock(currentReference.Explanation, value)
						hasAnyField = true
					}
				}
			}
			continue
		}

		if field, ok := extractQuestionJudgeConfigTextHeader(normalizedLine); ok {
			currentBlockField = ""
			switch field {
			case "startercode":
				currentSection = "starter_code"
				currentBlockField = "starter_code"
			case "publictestcases":
				flushCase()
				currentSection = "public_test_cases"
			case "hiddentestcases":
				flushCase()
				currentSection = "hidden_test_cases"
			case "referencesolutions":
				flushReference()
				currentSection = "reference_solutions"
			}
			continue
		}

		if currentBlockField != "" {
			appendBlockValue(normalizedLine)
			continue
		}

		switch currentSection {
		case "starter_code":
			appendBlockValue(normalizedLine)
		case "public_test_cases", "hidden_test_cases":
			if strings.TrimSpace(currentCase.Description) == "" {
				currentCase.Description = normalizedLine
				hasAnyField = true
			}
		case "reference_solutions":
			if strings.TrimSpace(currentReference.Explanation) == "" {
				currentReference.Explanation = normalizedLine
				hasAnyField = true
			} else {
				currentReference.Explanation = appendQuestionJudgeConfigTextBlock(currentReference.Explanation, normalizedLine)
				hasAnyField = true
			}
		}
	}

	flushCase()
	flushReference()
	if !hasAnyField {
		return nil
	}
	return config
}

// extractQuestionJudgeConfigTextHeader 识别仅包含字段头的文本行，兼容 judge_config 的 YAML 风格块。
func extractQuestionJudgeConfigTextHeader(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}
	if !(strings.HasSuffix(line, ":") || strings.HasSuffix(line, "：")) {
		return "", false
	}
	field := normalizeQuestionPipelineFieldKey(strings.TrimSuffix(strings.TrimSuffix(line, ":"), "："))
	switch field {
	case "startercode", "publictestcases", "hiddentestcases", "referencesolutions":
		return field, true
	default:
		return "", false
	}
}

// parseQuestionJudgeConfigLanguageList 解析判题配置中的语言列表，兼容 JSON 数组与逗号分隔文本。
func parseQuestionJudgeConfigLanguageList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var values []string
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		if err := json.Unmarshal([]byte(raw), &values); err == nil {
			return values
		}
		raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]"))
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ';' || r == '；'
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, `"'`))
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

// parseQuestionJudgeConfigInteger 解析判题配置中的整数值，兼容被引号包裹的文本。
func parseQuestionJudgeConfigInteger(raw string) (int, error) {
	raw = strings.TrimSpace(strings.Trim(raw, `"'`))
	return strconv.Atoi(raw)
}

// appendQuestionJudgeConfigTextBlock 追加多行文本块，并保持换行结构。
func appendQuestionJudgeConfigTextBlock(current string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return strings.TrimSpace(current)
	}
	if strings.TrimSpace(current) == "" {
		return value
	}
	return strings.TrimSpace(current + "\n" + value)
}

// resolveQuestionEvaluationMode 返回当前题目的最终判题模式。
func resolveQuestionEvaluationMode(question *model.Question, config *QuestionJudgeConfig) string {
	if question == nil || !question.IsCode() {
		return ""
	}
	if config == nil {
		return QuestionEvaluationModeAnalysisOnly
	}
	return config.EvaluationMode
}

// resolveQuestionAnswerLanguage 解析答题分析默认语言，优先取判题配置。
func resolveQuestionAnswerLanguage(question *model.Question, config *QuestionJudgeConfig) string {
	if config != nil && strings.TrimSpace(config.DefaultLanguage) != "" {
		return config.DefaultLanguage
	}
	return detectQuestionLanguage(question)
}

// resolveRequestedQuestionLanguage 解析运行代码时最终生效的语言，并回退到题目默认语言。
func resolveRequestedQuestionLanguage(requested string, question *model.Question, config *QuestionJudgeConfig) string {
	language := normalizeQuestionJudgeLanguage(requested)
	if language == "" {
		language = resolveQuestionAnswerLanguage(question, config)
	}
	if config == nil || len(config.AllowedLanguages) == 0 {
		return language
	}
	for _, allowed := range config.AllowedLanguages {
		if allowed == language {
			return language
		}
	}
	return config.DefaultLanguage
}

// selectQuestionTestCases 根据运行或提交场景选择需要执行的测试用例集合。
func selectQuestionTestCases(config *QuestionJudgeConfig, usePublicCases bool) []QuestionTestCase {
	if config == nil {
		return nil
	}

	if !usePublicCases {
		return config.HiddenTestCases
	}

	testCases := config.PublicTestCases
	if len(testCases) > 3 {
		return append([]QuestionTestCase(nil), testCases[:3]...)
	}
	return append([]QuestionTestCase(nil), testCases...)
}

// evaluateQuestionTestCases 逐条执行题目的公开或隐藏测试用例。
func (s *questionService) evaluateQuestionTestCases(
	ctx context.Context,
	question *model.Question,
	config *QuestionJudgeConfig,
	code string,
	usePublicCases bool,
) (*QuestionJudgeSummary, error) {
	if s == nil || s.codeExecutor == nil {
		return nil, fmt.Errorf("代码执行器未配置")
	}
	if question == nil || config == nil {
		return nil, fmt.Errorf("编程题判题配置不能为空")
	}

	testCases := selectQuestionTestCases(config, usePublicCases)

	summary := &QuestionJudgeSummary{
		Mode:          config.EvaluationMode,
		TotalCount:    len(testCases),
		TimeLimitMS:   config.TimeLimitMS,
		MemoryLimitMB: config.MemoryLimitMB,
	}
	for index, testCase := range testCases {
		result, err := s.codeExecutor.ExecuteWithInput(ctx, config.DefaultLanguage, code, testCase.Input)
		caseResult := QuestionTestCaseResult{
			Index:          index + 1,
			Description:    testCase.Description,
			Passed:         false,
			ExpectedOutput: testCase.ExpectedOutput,
		}
		if usePublicCases {
			caseResult.Input = testCase.Input
		}
		if err != nil {
			caseResult.ErrorOutput = err.Error()
			summary.CaseResults = append(summary.CaseResults, caseResult)
			continue
		}

		caseResult.ActualOutput = normalizeQuestionJudgeOutputText(result.Output)
		caseResult.Passed = result.Passed && caseResult.ActualOutput == normalizeQuestionJudgeOutputText(testCase.ExpectedOutput)
		if caseResult.Passed {
			summary.PassedCount++
		}
		summary.CaseResults = append(summary.CaseResults, caseResult)
	}
	summary.AllPassed = summary.TotalCount > 0 && summary.PassedCount == summary.TotalCount
	return summary, nil
}

// buildQuestionJudgeOutput 将用例执行结果汇总成可展示的运行文本。
func buildQuestionJudgeOutput(summary *QuestionJudgeSummary) string {
	if summary == nil {
		return "暂无判题结果"
	}
	parts := []string{
		fmt.Sprintf("通过 %d/%d 条测试用例", summary.PassedCount, summary.TotalCount),
	}
	for _, item := range summary.CaseResults {
		status := "失败"
		if item.Passed {
			status = "通过"
		}
		line := fmt.Sprintf("用例 #%d：%s", item.Index, status)
		if strings.TrimSpace(item.Description) != "" {
			line += " - " + item.Description
		}
		if strings.TrimSpace(item.ErrorOutput) != "" {
			line += "，错误：" + item.ErrorOutput
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, "\n")
}

// buildQuestionJudgeExplanation 将判题汇总结果转换为面向用户的简短说明。
func buildQuestionJudgeExplanation(summary *QuestionJudgeSummary) string {
	if summary == nil {
		return "暂无判题说明。"
	}
	if summary.AllPassed {
		return fmt.Sprintf("隐藏测试用例全部通过，共 %d/%d 条。", summary.PassedCount, summary.TotalCount)
	}
	return fmt.Sprintf("隐藏测试用例通过 %d/%d 条，建议优先检查边界条件、输出格式和核心逻辑。", summary.PassedCount, summary.TotalCount)
}

// normalizeQuestionJudgeOutputText 统一清理判题对比使用的输出文本。
func normalizeQuestionJudgeOutputText(raw string) string {
	trimmed := strings.ReplaceAll(raw, "\r\n", "\n")
	trimmed = strings.ReplaceAll(trimmed, "\r", "\n")
	return strings.TrimSpace(trimmed)
}
