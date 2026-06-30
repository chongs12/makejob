package biz

import (
	"encoding/json"
	"strings"
)

const (
	EvaluationModeAnalysisOnly = "analysis_only"
	EvaluationModeTestcase     = "testcase"
)

// JudgeConfig 描述编程题的统一判题配置（对齐单体 QuestionJudgeConfig）
type JudgeConfig struct {
	EvaluationMode   string              `json:"evaluation_mode"`
	DefaultLanguage  string              `json:"default_language"`
	AllowedLanguages []string            `json:"allowed_languages"`
	StarterCode      string              `json:"starter_code"`
	PublicTestCases  []JudgeTestCase     `json:"public_test_cases"`
	HiddenTestCases  []JudgeTestCase     `json:"hidden_test_cases"`
	ReferenceAnswers []ReferenceSolution `json:"reference_solutions"`
	TimeLimitMS      int                 `json:"time_limit_ms"`
	MemoryLimitMB    int                 `json:"memory_limit_mb"`
}

// JudgeTestCase 描述一条输入输出型测试用例
type JudgeTestCase struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	Description    string `json:"description,omitempty"`
}

// ReferenceSolution 描述一份参考实现
type ReferenceSolution struct {
	Language    string `json:"language"`
	Title       string `json:"title,omitempty"`
	Code        string `json:"code"`
	Explanation string `json:"explanation,omitempty"`
}

// ParseJudgeConfig 解析编程题判题配置，支持 judge_config_json 和 test_cases_json 双回退
func ParseJudgeConfig(judgeConfigJSON, testCasesJSON, questionType string) *JudgeConfig {
	if questionType != "code" {
		return nil
	}

	// 优先使用 judge_config_json（完整配置：公开/隐藏用例分组）
	var config JudgeConfig
	if strings.TrimSpace(judgeConfigJSON) != "" {
		if json.Unmarshal([]byte(judgeConfigJSON), &config) == nil {
			if len(config.PublicTestCases) > 0 || len(config.HiddenTestCases) > 0 {
				return normalizeJudgeConfig(&config)
			}
		}
	}

	// 回退：从 test_cases_json 构造兼容配置（旧数据）
	if strings.TrimSpace(testCasesJSON) != "" {
		var flatCases []struct {
			Input          string `json:"input"`
			ExpectedOutput string `json:"expected_output"`
		}
		if json.Unmarshal([]byte(testCasesJSON), &flatCases) == nil && len(flatCases) > 0 {
			cases := make([]JudgeTestCase, 0, len(flatCases))
			for _, fc := range flatCases {
				cases = append(cases, JudgeTestCase{
					Input:          fc.Input,
					ExpectedOutput: fc.ExpectedOutput,
				})
			}
			return normalizeJudgeConfig(&JudgeConfig{
				EvaluationMode:  EvaluationModeTestcase,
				PublicTestCases: cases,
				HiddenTestCases: cases,
			})
		}
	}

	// 最终回退：analysis_only 模式
	return normalizeJudgeConfig(&JudgeConfig{
		EvaluationMode: EvaluationModeAnalysisOnly,
	})
}

// normalizeJudgeConfig 规范化判题配置，填充默认值
func normalizeJudgeConfig(config *JudgeConfig) *JudgeConfig {
	if config == nil {
		config = &JudgeConfig{}
	}
	config.EvaluationMode = normalizeEvaluationMode(config.EvaluationMode)
	config.DefaultLanguage = normalizeJudgeLanguage(config.DefaultLanguage)
	config.AllowedLanguages = normalizeJudgeLanguages(config.AllowedLanguages, config.DefaultLanguage)
	config.StarterCode = strings.TrimSpace(config.StarterCode)
	config.PublicTestCases = normalizeTestCases(config.PublicTestCases)
	config.HiddenTestCases = normalizeTestCases(config.HiddenTestCases)
	if config.TimeLimitMS <= 0 {
		config.TimeLimitMS = 2000
	}
	if config.MemoryLimitMB <= 0 {
		config.MemoryLimitMB = 128
	}
	if len(config.AllowedLanguages) == 0 && config.DefaultLanguage != "" {
		config.AllowedLanguages = []string{config.DefaultLanguage}
	}
	return config
}

func normalizeEvaluationMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case EvaluationModeTestcase:
		return EvaluationModeTestcase
	default:
		return EvaluationModeAnalysisOnly
	}
}

func normalizeJudgeLanguage(raw string) string {
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

func normalizeJudgeLanguages(values []string, defaultLanguage string) []string {
	result := make([]string, 0, len(values)+1)
	seen := make(map[string]struct{}, len(values)+1)
	add := func(v string) {
		v = normalizeJudgeLanguage(v)
		if v == "" {
			return
		}
		if _, exists := seen[v]; exists {
			return
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	if defaultLanguage != "" {
		add(defaultLanguage)
	}
	for _, v := range values {
		add(v)
	}
	return result
}

func normalizeTestCases(cases []JudgeTestCase) []JudgeTestCase {
	result := make([]JudgeTestCase, 0, len(cases))
	for _, c := range cases {
		c.Input = strings.TrimSpace(c.Input)
		c.ExpectedOutput = strings.TrimSpace(c.ExpectedOutput)
		c.Description = strings.TrimSpace(c.Description)
		if c.Input != "" || c.ExpectedOutput != "" {
			result = append(result, c)
		}
	}
	return result
}

// SelectTestCases 根据场景选择需要执行的测试用例
// usePublic=true: RunCode 场景，返回公开用例（最多3条）
// usePublic=false: SubmitAnswer 场景，返回隐藏用例
func SelectTestCases(config *JudgeConfig, usePublic bool) []JudgeTestCase {
	if config == nil {
		return nil
	}
	if !usePublic {
		return config.HiddenTestCases
	}
	cases := config.PublicTestCases
	if len(cases) > 3 {
		return append([]JudgeTestCase(nil), cases[:3]...)
	}
	return append([]JudgeTestCase(nil), cases...)
}

// NormalizeJudgeOutputText 统一清理判题对比使用的输出文本
func NormalizeJudgeOutputText(raw string) string {
	trimmed := strings.ReplaceAll(raw, "\r\n", "\n")
	trimmed = strings.ReplaceAll(trimmed, "\r", "\n")
	return strings.TrimSpace(trimmed)
}

// ResolveEvaluationMode 返回最终判题模式
func ResolveEvaluationMode(config *JudgeConfig) string {
	if config == nil {
		return EvaluationModeAnalysisOnly
	}
	return config.EvaluationMode
}

// ResolveJudgeLanguage 解析最终生效的语言，支持请求语言 → 题目默认 → 白名单校验
func ResolveJudgeLanguage(requested string, config *JudgeConfig) string {
	language := normalizeJudgeLanguage(requested)
	if language == "" && config != nil {
		language = config.DefaultLanguage
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
