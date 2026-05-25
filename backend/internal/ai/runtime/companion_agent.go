package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

// companionAgent 基于 Provider 生成陪伴回复，补齐前端需要的情绪与动作字段。
type companionAgent struct {
	provider ai.AIProvider
	prompts  *promptResolver
	logger   *aiCallLogRecorder
}

// newCompanionAgent 创建陪伴 Agent。
func newCompanionAgent(provider ai.AIProvider, prompts *promptResolver, logger *aiCallLogRecorder) ai.CompanionAgent {
	return &companionAgent{
		provider: provider,
		prompts:  prompts,
		logger:   logger,
	}
}

// Chat 调用真实 Provider 生成陪伴回复，并补齐前端需要的情绪与动作字段。
func (a *companionAgent) Chat(ctx context.Context, messages []ai.Message, userEmotion string) (ai.CompanionResponse, error) {
	if a.prompts != nil {
		prompt := a.prompts.ResolveByIndustryID(ctx, model.PromptSceneCompanion, nil, map[string]string{
			"user_emotion":        userEmotion,
			"latest_user_message": latestUserMessage(messages),
		})
		messages = prependSystemPrompt(messages, prompt)
	}

	if a.provider == nil {
		return ai.CompanionResponse{}, fmt.Errorf("ai provider is unavailable")
	}

	startedAt := time.Now()
	resp, err := a.provider.Chat(ctx, messages)
	output := ""
	usage := ai.TokenUsage{}
	if resp != nil {
		output = resp.Content
		usage = ai.TokenUsage{InputTokens: resp.InputTokens, OutputTokens: resp.OutputTokens}
	}
	a.recordCall(ctx, latestUserMessage(messages), messages, output, err, startedAt, usage)
	if err != nil {
		return ai.CompanionResponse{}, err
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return ai.CompanionResponse{}, fmt.Errorf("empty companion response")
	}

	emotion := normalizeCompanionEmotion(userEmotion)
	return ai.CompanionResponse{
		Content: output,
		Emotion: emotion,
		Action:  companionActionForEmotion(emotion),
	}, nil
}

// GetGreeting 生成无需调用模型的本地欢迎语，避免陪伴首页因 Provider 异常直接报错。
func (a *companionAgent) GetGreeting(ctx context.Context, profile ai.UserProfile, timeOfDay string) (ai.CompanionResponse, error) {
	select {
	case <-ctx.Done():
		return ai.CompanionResponse{}, ctx.Err()
	default:
	}

	content := "你好，今天继续推进你的学习计划。"
	emotion := "happy"
	action := "wave"
	switch strings.ToLower(strings.TrimSpace(timeOfDay)) {
	case "morning":
		content = "早上好，先用一个清晰的小目标打开今天的学习节奏。"
	case "afternoon":
		content = "下午好，保持专注，把今天最重要的一件学习任务收掉。"
		emotion = "encouraging"
		action = "nod"
	case "evening":
		content = "晚上好，适合做复盘和查漏补缺，把今天的收获沉淀下来。"
		emotion = "neutral"
		action = "idle"
	case "night":
		content = "夜深了，注意节奏，优先做轻量复盘，不要透支状态。"
		emotion = "encouraging"
		action = "nod"
	}
	if strings.EqualFold(strings.TrimSpace(profile.Level), "beginner") {
		content += " 先稳住基础，不用追求一步到位。"
	}
	if strings.EqualFold(strings.TrimSpace(profile.Level), "advanced") {
		content += " 今天可以主动挑战一个更难的问题。"
	}
	return ai.CompanionResponse{Content: content, Emotion: emotion, Action: action}, nil
}

// GetEncouragement 生成无需调用模型的本地鼓励语，保证基础交互始终可用。
func (a *companionAgent) GetEncouragement(ctx context.Context, achievement string) (ai.CompanionResponse, error) {
	select {
	case <-ctx.Done():
		return ai.CompanionResponse{}, ctx.Err()
	default:
	}

	achievement = strings.TrimSpace(achievement)
	if achievement == "" {
		achievement = "当前这一步"
	}
	return ai.CompanionResponse{
		Content: achievement + " 做得不错，继续保持这个节奏，不要被短期波动打断。",
		Emotion: "encouraging",
		Action:  "nod",
	}, nil
}

// recordCall 记录一次陪伴链路的运行时模型调用。
func (a *companionAgent) recordCall(ctx context.Context, userInput string, messages []ai.Message, response string, err error, startedAt time.Time, usage ai.TokenUsage) {
	if a.logger == nil {
		return
	}

	a.logger.Record(ctx, runtimeCallLogEntry{
		Request:      messages,
		UserInput:    userInput,
		Model:        a.provider.GetModelName(),
		Output:       response,
		Err:          err,
		StartedAt:    startedAt,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	})
}

// normalizeCompanionEmotion 规范化陪伴场景情绪值，便于前端动作与表情映射保持稳定。
func normalizeCompanionEmotion(userEmotion string) string {
	switch strings.ToLower(strings.TrimSpace(userEmotion)) {
	case "happy", "excited":
		return "happy"
	case "sad", "tired":
		return "encouraging"
	case "frustrated", "confused":
		return "thinking"
	default:
		return "neutral"
	}
}

// companionActionForEmotion 根据情绪选择默认动作，避免陪伴场景出现空动作。
func companionActionForEmotion(emotion string) string {
	switch emotion {
	case "happy":
		return "wave"
	case "encouraging":
		return "nod"
	case "thinking":
		return "thinking"
	default:
		return "idle"
	}
}

// prependSystemPrompt 将系统提示词插入消息列表头部。
func prependSystemPrompt(messages []ai.Message, prompt string) []ai.Message {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return messages
	}

	result := make([]ai.Message, 0, len(messages)+1)
	result = append(result, ai.Message{
		Role:    "system",
		Content: prompt,
	})
	result = append(result, messages...)
	return result
}

// latestUserMessage 从消息列表中提取最近一条用户消息。
func latestUserMessage(messages []ai.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(messages[i].Role), "user") {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}
