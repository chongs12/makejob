package service

import (
	"context"
	"testing"

	"makejob-backend/internal/ai"
)

// TestSanitizeCompanionReplyRemovesThinkBlock 验证陪伴回复会过滤模型返回的深度思考标签。
func TestSanitizeCompanionReplyRemovesThinkBlock(t *testing.T) {
	t.Parallel()

	got := sanitizeCompanionReply("<think>\n分析用户情绪\n</think>\n\n辛苦了。先休息一下也没关系的。")
	want := "辛苦了。先休息一下也没关系的。"
	if got != want {
		t.Fatalf("unexpected sanitized reply: got %q want %q", got, want)
	}
}

// TestCompanionServiceChatSanitizesReply 验证陪伴聊天接口最终返回给前端的内容不会携带思维链。
func TestCompanionServiceChatSanitizesReply(t *testing.T) {
	t.Parallel()

	svc := NewCompanionService(stubCompanionAgent{
		reply: ai.CompanionResponse{
			Content: "<think>这里是模型推理</think>\n\n现在先歇一会儿，我陪你缓缓。",
			Emotion: "encouraging",
			Action:  "nod",
		},
	})

	resp, err := svc.Chat(context.Background(), 1, &CompanionChatRequest{
		Message: "今天好累",
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if resp.Content != "现在先歇一会儿，我陪你缓缓。" {
		t.Fatalf("unexpected content: %#v", resp)
	}
	if resp.Reply != resp.Content {
		t.Fatalf("expected reply alias to match content, got %#v", resp)
	}
}

// stubCompanionAgent 为陪伴服务测试提供最小 Agent 实现。
type stubCompanionAgent struct {
	reply ai.CompanionResponse
	err   error
}

// Chat 返回预置陪伴回复。
func (s stubCompanionAgent) Chat(context.Context, []ai.Message, string) (ai.CompanionResponse, error) {
	return s.reply, s.err
}

// GetGreeting 满足接口要求，当前测试无需使用。
func (s stubCompanionAgent) GetGreeting(context.Context, ai.UserProfile, string) (ai.CompanionResponse, error) {
	return ai.CompanionResponse{}, nil
}

// GetEncouragement 满足接口要求，当前测试无需使用。
func (s stubCompanionAgent) GetEncouragement(context.Context, string) (ai.CompanionResponse, error) {
	return ai.CompanionResponse{}, nil
}
