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
      "estimated_hours": 2
    }
  ],
  "summary": "计划总结"
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
