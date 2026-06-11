package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ==================== AI 修复机制 ====================

// repairQuestionPipelineCardResponse 在单卡输出解析失败时，追加一次轻量修复请求。
func (uc *AdminUseCase) repairQuestionPipelineCardResponse(ctx context.Context, raw string, requireCode bool) (*repairResult, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("raw model output is empty")
	}

	// 加载配置
	cfg, err := uc.configRepo.GetActiveConfig(ctx, "question_generator")
	if err != nil {
		return nil, fmt.Errorf("AI 配置加载失败: %w", err)
	}

	prompt := fmt.Sprintf(`你是 MakeJob 的题卡 JSON 修复助手。
你的任务不是重新出题，而是把已有内容整理成唯一合法的 JSON 对象。

严格要求：
1. 只能输出一个 JSON 对象，结构必须是 {"cards":[{...}]}。
2. cards 数组最多只保留 1 张题卡。
3. 不要输出解释、Markdown、代码块、前后缀、注释。
4. 只保留原内容里可以稳定提取的信息，不要凭空扩写。
5. 如果原内容里没有可靠答案，也必须尽量从原文提炼参考答案字段。
6. 如果原内容明显是在生成编程题，优先保留 solution、judge_config、public_test_cases、hidden_test_cases、reference_solutions 等结构化字段。

目标 JSON 结构：
%s

待修复内容：
%s`, buildSingleCardRepairSchema(requireCode), raw)

	// 创建独立的 context
	llmCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	messages := []Message{{Role: "user", Content: prompt}}
	resp, err := uc.llm.Chat(llmCtx, messages, cfg)
	if err != nil {
		return nil, fmt.Errorf("题卡修复调用失败: %w", err)
	}

	// 解析修复结果
	cards := normalizeQuestionPipelineModelCardsFromRaw(resp.Content)
	if len(cards) == 0 {
		return nil, fmt.Errorf("repair cards not found")
	}

	return &repairResult{
		Card:       cards[0],
		TraceOutput: resp.Content,
	}, nil
}

// repairResult 修复结果。
type repairResult struct {
	Card        *QuestionCandidate
	TraceOutput string
}

// ==================== 编程题字段补齐 ====================

// finalizeQuestionPipelineModelCard 补齐编程题必需字段。
func (uc *AdminUseCase) finalizeQuestionPipelineModelCard(ctx context.Context, card *QuestionCandidate, constraints questionPipelineConstraintProfile) (*QuestionCandidate, string, bool) {
	if !constraints.RequireCode && normalizeQuestionPipelineType(card.Type) != "code" {
		return card, "", false
	}

	prepared := buildPreparedCodeCard(card)
	if isCodeCardComplete(prepared) {
		return prepared, "", false
	}

	supplemented, err := uc.supplementCodeCard(ctx, prepared)
	if err != nil {
		return prepared, fmt.Sprintf("编程题结构补齐失败: %v", err), true
	}
	if !isCodeCardComplete(supplemented) {
		return supplemented, "编程题结构补齐后仍缺少完整 judge_config", true
	}
	return supplemented, "", true
}

// supplementCodeCard 基于已有编程题半成品补齐 solution 与 judge_config。
func (uc *AdminUseCase) supplementCodeCard(ctx context.Context, card *QuestionCandidate) (*QuestionCandidate, error) {
	prepared := buildPreparedCodeCard(card)
	payload, err := json.Marshal(map[string]any{
		"cards": []*QuestionCandidate{prepared},
	})
	if err != nil {
		return prepared, err
	}

	// 加载配置
	cfg, err := uc.configRepo.GetActiveConfig(ctx, "question_generator")
	if err != nil {
		return prepared, fmt.Errorf("AI 配置加载失败: %w", err)
	}

	prompt := fmt.Sprintf(`你是 MakeJob 的编程题结构补齐助手。
你的任务不是重新出题，而是在不改变原题核心语义的前提下，把现有编程题整理成可直接导入题库的完整 JSON。

严格要求：
1. 只能输出一个 JSON 对象，结构必须是 {"cards":[{...}]}。
2. cards 数组只能保留 1 张题卡，type 固定为 code。
3. 必须保留原题 title、content、answer 的核心语义，不要换题，不要改成别的考点。
4. answer 只放代码参考答案；solution 只放纯文本思路解析，不要输出 Markdown 标题、代码块或额外说明。
5. judge_config 必须使用 testcase 判题模式，并且完整包含 default_language、allowed_languages、starter_code、public_test_cases、hidden_test_cases、reference_solutions、time_limit_ms、memory_limit_mb。
6. public_test_cases 必须恰好 3 条，供前端"运行代码"直接使用；hidden_test_cases 至少 3 条，供最终提交判题使用。
7. reference_solutions 至少 1 条，且代码要与题干一致；如果原 answer 已经是完整代码，应复用为主要参考实现。
8. 如果原题没有显式测试用例，请根据题干和参考实现补出合理、可执行的测试用例。

目标 JSON 结构：
%s

待补齐题卡：
%s`, buildSingleCardRepairSchema(true), string(payload))

	// 创建独立的 context
	llmCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	messages := []Message{{Role: "user", Content: prompt}}
	resp, err := uc.llm.Chat(llmCtx, messages, cfg)
	if err != nil {
		return prepared, fmt.Errorf("编程题结构补齐调用失败: %w", err)
	}

	// 解析补齐结果
	cards := normalizeQuestionPipelineModelCardsFromRaw(resp.Content)
	if len(cards) == 0 {
		return prepared, fmt.Errorf("supplement cards not found")
	}

	merged := mergeQuestionCandidate(prepared, cards[0])
	return buildPreparedCodeCard(merged), nil
}

// ==================== 辅助函数 ====================

// buildPreparedCodeCard 构建预备的编程题卡。
func buildPreparedCodeCard(card *QuestionCandidate) *QuestionCandidate {
	result := *card
	result.Title = strings.TrimSpace(result.Title)
	result.Content = strings.TrimSpace(result.Content)
	result.Type = "code"
	result.Answer = normalizeCodeField(result.Answer)
	result.Explanation = firstNonEmptyString(strings.TrimSpace(result.Explanation), buildExplanation(result.Title))
	result.Solution = firstNonEmptyString(
		normalizeCodeSolution(result.Solution),
		strings.TrimSpace(result.Explanation),
	)

	// 构建 judge_config 骨架
	if result.JudgeConfig == nil {
		judgeConfig := buildCodeJudgeConfigSkeleton(&result)
		if judgeConfig != nil {
			result.JudgeConfig = judgeConfig
		}
	}

	return &result
}

// isCodeCardComplete 判断编程题卡是否完整。
func isCodeCardComplete(card *QuestionCandidate) bool {
	if strings.TrimSpace(card.Answer) == "" || strings.TrimSpace(normalizeCodeSolution(card.Solution)) == "" {
		return false
	}

	// 解析 judge_config
	if card.JudgeConfig == nil {
		return false
	}

	// 将 JudgeConfig 转换为 map
	judgeConfig, ok := card.JudgeConfig.(map[string]any)
	if !ok {
		// 尝试从 JSON 字符串解析
		jcStr, ok := card.JudgeConfig.(string)
		if !ok {
			return false
		}
		if err := json.Unmarshal([]byte(jcStr), &judgeConfig); err != nil {
			return false
		}
	}

	// 检查必要字段
	evalMode, _ := judgeConfig["evaluation_mode"].(string)
	if evalMode != "testcase" {
		return false
	}

	publicTestCases, _ := judgeConfig["public_test_cases"].([]any)
	hiddenTestCases, _ := judgeConfig["hidden_test_cases"].([]any)
	referenceSolutions, _ := judgeConfig["reference_solutions"].([]any)

	return len(publicTestCases) == 3 && len(hiddenTestCases) > 0 && len(referenceSolutions) > 0
}

// mergeQuestionCandidate 合并两个题卡。
func mergeQuestionCandidate(base, override *QuestionCandidate) *QuestionCandidate {
	merged := *base
	merged.Title = firstNonEmptyString(strings.TrimSpace(override.Title), strings.TrimSpace(base.Title))
	merged.Content = firstNonEmptyString(strings.TrimSpace(override.Content), strings.TrimSpace(base.Content))
	merged.Type = firstNonEmptyString(normalizeQuestionPipelineType(override.Type), normalizeQuestionPipelineType(base.Type))
	merged.Difficulty = firstNonEmptyString(normalizeQuestionPipelineDifficulty(override.Difficulty), normalizeQuestionPipelineDifficulty(base.Difficulty))
	merged.Category = firstNonEmptyString(strings.TrimSpace(override.Category), strings.TrimSpace(base.Category))
	merged.Answer = firstNonEmptyString(strings.TrimSpace(override.Answer), strings.TrimSpace(base.Answer))
	merged.Solution = firstNonEmptyString(strings.TrimSpace(override.Solution), strings.TrimSpace(base.Solution))
	merged.Explanation = firstNonEmptyString(strings.TrimSpace(override.Explanation), strings.TrimSpace(base.Explanation))
	if len(override.Tags) > 0 {
		merged.Tags = append([]string(nil), override.Tags...)
	}
	if override.JudgeConfig != nil {
		merged.JudgeConfig = override.JudgeConfig
	}
	return &merged
}

// normalizeCodeField 规范化代码字段。
func normalizeCodeField(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// 移除代码块标记
	if strings.HasPrefix(raw, "```") {
		if lineEnd := strings.Index(raw, "\n"); lineEnd >= 0 {
			raw = raw[lineEnd+1:]
		} else {
			raw = strings.TrimPrefix(raw, "```")
		}
	}
	raw = strings.TrimSuffix(strings.TrimSpace(raw), "```")
	return strings.TrimSpace(raw)
}

// normalizeCodeSolution 规范化代码解法。
func normalizeCodeSolution(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// 移除 Markdown 标题
	lines := strings.Split(raw, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

// buildExplanation 构建解释文本。
func buildExplanation(title string) string {
	if title == "" {
		return ""
	}
	return fmt.Sprintf("本题考查%s相关知识点，请结合题目要求详细说明核心概念和实现要点。", title)
}

// buildCodeJudgeConfigSkeleton 构建编程题判题配置骨架。
func buildCodeJudgeConfigSkeleton(card *QuestionCandidate) map[string]any {
	if strings.TrimSpace(card.Answer) == "" {
		return nil
	}

	language := detectCardLanguage(card)
	if language == "" || language == "text" {
		language = "go"
	}

	return map[string]any{
		"evaluation_mode":     "testcase",
		"default_language":    language,
		"allowed_languages":   []string{language},
		"starter_code":        "",
		"public_test_cases":   []any{},
		"hidden_test_cases":   []any{},
		"reference_solutions": []any{},
		"time_limit_ms":       5000,
		"memory_limit_mb":     256,
	}
}

// detectCardLanguage 检测题卡的编程语言。
func detectCardLanguage(card *QuestionCandidate) string {
	if card == nil {
		return ""
	}

	// 从 answer 中检测
	answer := strings.ToLower(card.Answer)
	if strings.Contains(answer, "package main") || strings.Contains(answer, "func ") {
		return "go"
	}
	if strings.Contains(answer, "public class") || strings.Contains(answer, "void main") {
		return "java"
	}
	if strings.Contains(answer, "def ") || strings.Contains(answer, "import ") {
		return "python"
	}

	// 从 solution 中检测
	solution := strings.ToLower(card.Solution)
	if strings.Contains(solution, "golang") || strings.Contains(solution, "go语言") {
		return "go"
	}
	if strings.Contains(solution, "java") {
		return "java"
	}
	if strings.Contains(solution, "python") {
		return "python"
	}

	return ""
}

// buildSingleCardRepairSchema 构建单卡修复的 JSON Schema。
func buildSingleCardRepairSchema(requireCode bool) string {
	schema := `{
  "cards": [
    {
      "title": "题卡标题",
      "content": "题卡内容/题干",
      "type": "subjective|code|choice|multi",
      "difficulty": "easy|medium|hard",
      "category": "分类",
      "answer": "参考答案",
      "solution": "思路解析（纯文本）",
      "explanation": "解释说明",
      "tags": ["标签1", "标签2"]
    }
  ]
}`

	if requireCode {
		schema = `{
  "cards": [
    {
      "title": "题卡标题",
      "content": "题卡内容/题干",
      "type": "code",
      "difficulty": "easy|medium|hard",
      "category": "分类",
      "answer": "代码参考答案",
      "solution": "代码思路解析（纯文本）",
      "explanation": "解释说明",
      "tags": ["标签1", "标签2"],
      "judge_config": {
        "evaluation_mode": "testcase",
        "default_language": "go|java|python",
        "allowed_languages": ["go|java|python"],
        "starter_code": "",
        "public_test_cases": [
          {"input": "输入1", "expected_output": "输出1", "description": "描述1"},
          {"input": "输入2", "expected_output": "输出2", "description": "描述2"},
          {"input": "输入3", "expected_output": "输出3", "description": "描述3"}
        ],
        "hidden_test_cases": [
          {"input": "输入1", "expected_output": "输出1"},
          {"input": "输入2", "expected_output": "输出2"},
          {"input": "输入3", "expected_output": "输出3"}
        ],
        "reference_solutions": [
          {"language": "go|java|python", "code": "完整代码"}
        ],
        "time_limit_ms": 5000,
        "memory_limit_mb": 256
      }
    }
  ]
}`
	}

	return schema
}

// normalizeQuestionPipelineModelCardsFromRaw 从原始文本中解析题卡数组。
func normalizeQuestionPipelineModelCardsFromRaw(raw string) []*QuestionCandidate {
	// 先尝试 JSON 解析
	value, err := decodeQuestionPipelineJSONValue(raw)
	if err == nil {
		cards := normalizeQuestionPipelineModelCards(value)
		if len(cards) > 0 {
			return cards
		}
	}

	// 回退到文本解析
	return parseQuestionPipelineCardsText(sanitizeQuestionPipelineModelOutput(raw))
}

// sanitizeQuestionCandidate 清理题卡字段。
func sanitizeQuestionCandidate(card *QuestionCandidate) *QuestionCandidate {
	if card == nil {
		return nil
	}
	result := *card
	result.Title = strings.TrimSpace(result.Title)
	result.Content = strings.TrimSpace(result.Content)
	result.Answer = strings.TrimSpace(result.Answer)
	result.Explanation = strings.TrimSpace(result.Explanation)
	result.Solution = strings.TrimSpace(result.Solution)
	result.Category = strings.TrimSpace(result.Category)
	result.Difficulty = normalizeQuestionPipelineDifficulty(result.Difficulty)
	result.Type = normalizeQuestionPipelineType(result.Type)
	return &result
}

// normalizeCodeCardWithReason 校验编程题完整性。
func normalizeCodeCardWithReason(card *QuestionCandidate) (*QuestionCandidate, string, bool) {
	if card == nil {
		return nil, "题卡为空", false
	}

	// 检查基本字段
	if strings.TrimSpace(card.Title) == "" {
		return card, "缺少标题", false
	}
	if strings.TrimSpace(card.Content) == "" {
		return card, "缺少题干内容", false
	}
	if strings.TrimSpace(card.Answer) == "" {
		return card, "缺少代码参考答案", false
	}

	// 检查 judge_config
	if card.JudgeConfig == nil {
		return card, "缺少 judge_config", false
	}

	// 将 JudgeConfig 转换为 map
	judgeConfig, ok := card.JudgeConfig.(map[string]any)
	if !ok {
		// 尝试从 JSON 字符串解析
		jcStr, ok := card.JudgeConfig.(string)
		if !ok {
			return card, "judge_config 格式错误", false
		}
		if err := json.Unmarshal([]byte(jcStr), &judgeConfig); err != nil {
			return card, "judge_config 格式错误", false
		}
	}

	// 检查 evaluation_mode
	evalMode, _ := judgeConfig["evaluation_mode"].(string)
	if evalMode != "testcase" {
		return card, "evaluation_mode 必须为 testcase", false
	}

	// 检查 public_test_cases
	publicTestCases, ok := judgeConfig["public_test_cases"].([]any)
	if !ok || len(publicTestCases) != 3 {
		return card, fmt.Sprintf("public_test_cases 必须恰好 3 条，当前 %d 条", len(publicTestCases)), false
	}

	// 检查 hidden_test_cases
	hiddenTestCases, ok := judgeConfig["hidden_test_cases"].([]any)
	if !ok || len(hiddenTestCases) == 0 {
		return card, "hidden_test_cases 不能为空", false
	}

	// 检查 reference_solutions
	referenceSolutions, ok := judgeConfig["reference_solutions"].([]any)
	if !ok || len(referenceSolutions) == 0 {
		return card, "reference_solutions 不能为空", false
	}

	return card, "", true
}
