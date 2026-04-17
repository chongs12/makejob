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

	return &CompanionChatResponse{
		Content: reply.Content,
		Reply:   reply.Content,
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
