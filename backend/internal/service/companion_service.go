package service

import (
	"context"
	"strings"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/common"
)

type CompanionChatRequest struct {
	Message     string         `json:"message"`
	Messages    []ai.Message   `json:"messages"`
	UserEmotion string         `json:"user_emotion"`
	Context     map[string]any `json:"context"`
}

type CompanionChatResponse struct {
	Content string `json:"content"`
	Reply   string `json:"reply"`
	Emotion string `json:"emotion"`
	Mood    string `json:"mood"`
	Action  string `json:"action"`
}

type CompanionService interface {
	Chat(ctx context.Context, userID uint, req *CompanionChatRequest) (*CompanionChatResponse, error)
}

type companionService struct {
	companionAgent ai.CompanionAgent
}

func NewCompanionService(companionAgent ai.CompanionAgent) CompanionService {
	return &companionService{
		companionAgent: companionAgent,
	}
}

func (s *companionService) Chat(ctx context.Context, userID uint, req *CompanionChatRequest) (*CompanionChatResponse, error) {
	if req == nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "请求不能为空")
	}

	messages := normalizeCompanionMessages(req)
	if len(messages) == 0 {
		return nil, common.NewBusinessError(common.CodeBadRequest, "消息不能为空")
	}

	userEmotion := strings.TrimSpace(req.UserEmotion)
	if userEmotion == "" {
		userEmotion = "neutral"
	}

	reply, err := s.companionAgent.Chat(ctx, messages, userEmotion)
	if err != nil {
		return nil, err
	}

	replyContent := sanitizeCompanionReply(reply.Content)
	return &CompanionChatResponse{
		Content: replyContent,
		Reply:   replyContent,
		Emotion: reply.Emotion,
		Mood:    reply.Emotion,
		Action:  reply.Action,
	}, nil
}

func normalizeCompanionMessages(req *CompanionChatRequest) []ai.Message {
	if len(req.Messages) > 0 {
		messages := make([]ai.Message, 0, len(req.Messages))
		for _, item := range req.Messages {
			if strings.TrimSpace(item.Content) == "" {
				continue
			}

			role := strings.TrimSpace(item.Role)
			if role == "" {
				role = "user"
			}

			messages = append(messages, ai.Message{
				Role:    role,
				Content: strings.TrimSpace(item.Content),
			})
		}
		return messages
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		return nil
	}

	return []ai.Message{{
		Role:    "user",
		Content: message,
	}}
}

// sanitizeCompanionReply 清理陪伴回复中的思维链标签，避免直接显示在 Live2D 对话框中。
func sanitizeCompanionReply(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	type blockMarker struct {
		start string
		end   string
	}

	blocks := []blockMarker{
		{start: "<think>", end: "</think>"},
		{start: "<reasoning>", end: "</reasoning>"},
	}

	lowered := strings.ToLower(content)
	for _, block := range blocks {
		for {
			start := strings.Index(lowered, block.start)
			if start < 0 {
				break
			}
			end := strings.Index(lowered[start+len(block.start):], block.end)
			if end < 0 {
				content = strings.TrimSpace(content[:start])
				lowered = strings.ToLower(content)
				break
			}

			realEnd := start + len(block.start) + end + len(block.end)
			content = content[:start] + content[realEnd:]
			lowered = strings.ToLower(content)
		}
	}

	content = strings.ReplaceAll(content, "<think>", "")
	content = strings.ReplaceAll(content, "</think>", "")
	content = strings.ReplaceAll(content, "<reasoning>", "")
	content = strings.ReplaceAll(content, "</reasoning>", "")

	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	previousBlank := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if previousBlank {
				continue
			}
			previousBlank = true
			filtered = append(filtered, "")
			continue
		}

		previousBlank = false
		filtered = append(filtered, trimmed)
	}

	return strings.TrimSpace(strings.Join(filtered, "\n"))
}
