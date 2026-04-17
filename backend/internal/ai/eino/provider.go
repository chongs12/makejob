package eino

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	ark "github.com/cloudwego/eino-ext/components/model/ark"
	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"makejob-backend/internal/ai"
)

type Provider struct {
	chatModel einoModel.ToolCallingChatModel
	modelName string
}

func NewProvider(ctx context.Context, config map[string]string) (*Provider, error) {
	normalized := ai.NormalizeRuntimeConfig(config)

	modelName := strings.TrimSpace(normalized[ai.ConfigKeyModel])
	if modelName == "" {
		return nil, fmt.Errorf("eino provider requires %s", ai.ConfigKeyModel)
	}

	apiKey := strings.TrimSpace(normalized[ai.ConfigKeyAPIKey])
	if apiKey == "" {
		return nil, fmt.Errorf("eino provider requires %s", ai.ConfigKeyAPIKey)
	}

	chatModel, err := ark.NewChatModel(ctx, buildChatModelConfig(normalized))
	if err != nil {
		return nil, fmt.Errorf("create ark chat model: %w", err)
	}

	return &Provider{
		chatModel: chatModel,
		modelName: modelName,
	}, nil
}

func (p *Provider) Chat(ctx context.Context, messages []ai.Message) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("messages cannot be empty")
	}

	resp, err := p.chatModel.Generate(ctx, toSchemaMessages(messages))
	if err != nil {
		return "", err
	}

	content := extractMessageText(resp)
	if content == "" {
		return "", fmt.Errorf("eino provider returned empty response")
	}

	return content, nil
}

func (p *Provider) StreamChat(ctx context.Context, messages []ai.Message) (<-chan string, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}

	streamReader, err := p.chatModel.Stream(ctx, toSchemaMessages(messages))
	if err != nil {
		return nil, err
	}

	out := make(chan string)
	go func() {
		defer close(out)
		defer streamReader.Close()

		for {
			msg, recvErr := streamReader.Recv()
			if recvErr != nil {
				if recvErr == io.EOF {
					return
				}
				return
			}

			content := extractMessageText(msg)
			if content == "" {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case out <- content:
			}
		}
	}()

	return out, nil
}

func (p *Provider) GetModelName() string {
	return p.modelName
}

func buildChatModelConfig(config map[string]string) *ark.ChatModelConfig {
	chatConfig := &ark.ChatModelConfig{
		APIKey: strings.TrimSpace(config[ai.ConfigKeyAPIKey]),
		Model:  strings.TrimSpace(config[ai.ConfigKeyModel]),
	}

	if baseURL := strings.TrimSpace(config[ai.ConfigKeyBaseURL]); baseURL != "" {
		chatConfig.BaseURL = baseURL
	}

	if timeoutSeconds := parsePositiveInt(config[ai.ConfigKeyTimeoutSeconds]); timeoutSeconds > 0 {
		timeout := time.Duration(timeoutSeconds) * time.Second
		chatConfig.Timeout = &timeout
	}

	if temperature, ok := parseFloat32(config[ai.ConfigKeyTemperature]); ok {
		chatConfig.Temperature = &temperature
	}

	if topP, ok := parseFloat32(config[ai.ConfigKeyTopP]); ok {
		chatConfig.TopP = &topP
	}

	if maxTokens := parsePositiveInt(config[ai.ConfigKeyMaxTokens]); maxTokens > 0 {
		chatConfig.MaxTokens = &maxTokens
	}

	return chatConfig
}

func toSchemaMessages(messages []ai.Message) []*schema.Message {
	result := make([]*schema.Message, 0, len(messages))
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}

		switch strings.ToLower(strings.TrimSpace(message.Role)) {
		case "system":
			result = append(result, schema.SystemMessage(content))
		case "assistant":
			result = append(result, schema.AssistantMessage(content, nil))
		default:
			result = append(result, schema.UserMessage(content))
		}
	}
	return result
}

func extractMessageText(message *schema.Message) string {
	if message == nil {
		return ""
	}

	if content := strings.TrimSpace(message.Content); content != "" {
		return content
	}

	var parts []string
	for _, part := range message.AssistantGenMultiContent {
		if text := strings.TrimSpace(part.Text); text != "" {
			parts = append(parts, text)
		}
		if part.Reasoning != nil {
			if text := strings.TrimSpace(part.Reasoning.Text); text != "" {
				parts = append(parts, text)
			}
		}
	}

	if reasoning := strings.TrimSpace(message.ReasoningContent); reasoning != "" {
		parts = append(parts, reasoning)
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func parsePositiveInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func parseFloat32(raw string) (float32, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 32)
	if err != nil {
		return 0, false
	}
	return float32(value), true
}
