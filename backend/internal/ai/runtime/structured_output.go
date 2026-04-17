package runtime

import (
	"context"
	"fmt"
	"strings"

	"makejob-backend/internal/ai"
)

// callStructuredJSON 执行结构化输出调用，并在首次解析失败时尝试一次 JSON 修复。
func callStructuredJSON[T any](ctx context.Context, provider ai.AIProvider, messages []ai.Message, schema string) (T, string, error) {
	var zero T

	response, err := provider.Chat(ctx, messages)
	if err != nil {
		return zero, "", err
	}

	payload, decodeErr := decodeJSONPayload[T](response)
	if decodeErr == nil {
		return payload, response, nil
	}

	repairedResponse, repairErr := provider.Chat(ctx, buildJSONRepairMessages(schema, response))
	if repairErr != nil {
		return zero, response, fmt.Errorf("decode json payload: %w; repair request failed: %v", decodeErr, repairErr)
	}

	repairedPayload, repairedDecodeErr := decodeJSONPayload[T](repairedResponse)
	if repairedDecodeErr != nil {
		return zero, buildStructuredTrace(response, repairedResponse), fmt.Errorf("decode json payload: %w; repair decode failed: %v", decodeErr, repairedDecodeErr)
	}

	return repairedPayload, buildStructuredTrace(response, repairedResponse), nil
}

// buildJSONRepairMessages 构造结构化输出修复请求。
func buildJSONRepairMessages(schema string, raw string) []ai.Message {
	return []ai.Message{
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
			Role: "user",
			Content: fmt.Sprintf("目标 JSON 结构：\n%s\n\n待修复内容：\n%s", strings.TrimSpace(schema), strings.TrimSpace(raw)),
		},
	}
}

// buildStructuredTrace 拼接原始响应与修复响应，便于日志排查。
func buildStructuredTrace(initial string, repaired string) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(initial) != "" {
		parts = append(parts, "[initial_response]\n"+strings.TrimSpace(initial))
	}
	if strings.TrimSpace(repaired) != "" {
		parts = append(parts, "[repair_response]\n"+strings.TrimSpace(repaired))
	}
	return strings.Join(parts, "\n\n")
}
