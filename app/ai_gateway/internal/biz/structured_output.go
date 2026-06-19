package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// buildJSONContractPrompt 在基础 Prompt 末尾追加结构化输出约束，要求模型仅返回 JSON。
// 对齐单体 runtime 的 buildXxxSystemPrompt 做法：种子 Prompt 本身是对话式的，
// 需显式追加 JSON 合同，模型才会返回可解析的结构化结果。
func buildJSONContractPrompt(basePrompt, schema string) string {
	return strings.TrimSpace(basePrompt) + "\n\n" +
		"你正在执行结构化输出任务。你必须只返回一个 JSON 对象，不要返回 Markdown、解释、代码块或额外文字。\n" +
		"JSON 结构如下：\n" + strings.TrimSpace(schema)
}

// extractJSONObject 从模型输出中提取 JSON 对象主体，兼容 ```json 围栏与前后说明文字。
// 直接移植单体 runtime.extractJSONObject。
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

// decodeJSONPayload 提取并解析模型输出为目标结构。
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

// buildJSONRepairMessages 构造一次性 JSON 修复请求，把散文输出重写为目标结构。
// 移植单体 runtime.buildJSONRepairMessages。
func buildJSONRepairMessages(schema, raw string) []Message {
	return []Message{
		{
			Role: "system",
			Content: strings.TrimSpace(`你是一个 JSON 修复助手。
你的任务不是回答原问题，而是把给定内容重写成一个合法 JSON 对象。
你必须严格遵守以下规则：
1. 只能输出一个 JSON 对象。
2. 首字符必须是 {，末字符必须是 }。
3. 不要输出 Markdown、解释、前后缀、注释或代码块。
4. 不要凭空扩展字段，只保留能从原内容中稳定提取的信息。`),
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("目标 JSON 结构：\n%s\n\n待修复内容：\n%s", strings.TrimSpace(schema), strings.TrimSpace(raw)),
		},
	}
}

// parseStructuredJSON 解析模型结构化输出，首解析失败时发起一次修复重试，全部失败返回 ErrParseFailed。
func parseStructuredJSON[T any](ctx context.Context, llm LLMClient, cfg *AIConfig, raw, schema string) (T, error) {
	if payload, err := decodeJSONPayload[T](raw); err == nil {
		return payload, nil
	}

	repaired, repairErr := llm.Chat(ctx, buildJSONRepairMessages(schema, raw), cfg)
	if repairErr != nil || repaired == nil {
		var zero T
		return zero, ErrParseFailed
	}
	repairedPayload, repairedErr := decodeJSONPayload[T](repaired.Content)
	if repairedErr != nil {
		var zero T
		return zero, ErrParseFailed
	}
	return repairedPayload, nil
}

// ---------- 各场景 JSON 合同 ----------

// interviewResultSchema 面试出题/评分结构化输出合同。
func interviewResultSchema() string {
	return `{
  "question": "题目正文，生成新题时必填",
  "topic": "知识点主题",
  "difficulty": "easy|medium|hard",
  "type": "technical|behavioral|coding",
  "hints": "简短提示，可为空字符串",
  "feedback": "对上一题回答的评价，无则空字符串",
  "score": 0,
  "should_end": false,
  "live2d_emotion": "情绪标签，可为空字符串",
  "live2d_action": "动作标签，可为空字符串"
}`
}

// planResultSchema 学习计划结构化输出合同。
func planResultSchema() string {
	return `{
  "plan_title": "计划标题",
  "tasks": [
    {
      "title": "任务标题",
      "description": "任务说明",
      "phase": "阶段名",
      "order_index": 1,
      "estimated_hours": 2,
      "task_type": "study|practice|interview|review",
      "phase_goal": "阶段目标",
      "day_number": 1,
      "duration_minutes": 60,
      "priority": "high|medium|low"
    }
  ],
  "summary": "计划总结",
  "phase": "foundation|drill|review|mock",
  "phase_goal": "阶段目标",
  "duration_days": 30
}`
}

// quizResultSchema 答题分析结构化输出合同。
func quizResultSchema() string {
	return `{
  "score": 0,
  "is_correct": true,
  "feedback": "总体评价",
  "key_points": ["关键点1", "关键点2"],
  "suggestions": "改进建议",
  "correct_answer": "参考答案"
}`
}

// codeAnalysisSchema 代码分析结构化输出合同。
func codeAnalysisSchema() string {
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

// codingDiagnosisSchema 编程面试诊断结构化输出合同。
func codingDiagnosisSchema() string {
	return `{
  "score": 78,
  "mistake_tags": ["状态定义不清"],
  "strength_tags": ["愿意主动验证思路"],
  "evidence": ["多次运行后仍集中修改同一逻辑分支"],
  "suggestions": ["补练边界条件和状态设计"],
  "process_summary": "能够持续迭代，但调试路径还不够稳定。"
}`
}

// buildCodeAnalysisSystemPrompt 构造代码分析系统提示词。
func buildCodeAnalysisSystemPrompt() string {
	return `请分析这道题的答案，并严格返回 JSON，不要输出 Markdown 或额外解释。JSON 结构如下：
{
  "is_correct": true,
  "score": 0,
  "feedback": "整体评价",
  "issues": ["问题1", "问题2"],
  "improvements": ["改进建议1", "改进建议2"],
  "mistake_tags": ["错因标签1"],
  "strength_tags": ["优势标签1"],
  "time_complexity": "O(n)",
  "space_complexity": "O(1)"
}
要求：
1. score 必须在 0 到 100 之间。
2. 只评价答案本身是否正确、完整、高质量。不要评价题目描述中与答案无关的内容。
3. is_correct 应基于答案是否正确回应了题目的核心要求来判定。
4. issues 和 improvements 至少各返回 1 条。
5. mistake_tags 要尽量具体，不要只写"基础不好"这类空泛表述。`
}

// buildCodingDiagnosisSystemPrompt 构造编程面试诊断系统提示词。
func buildCodingDiagnosisSystemPrompt() string {
	return `请根据题目、最终代码和过程事件分析候选人的编程面试表现，并严格返回 JSON，不要输出 Markdown 或额外解释。JSON 结构如下：
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
4. suggestions 给出可执行的补强动作。`
}
