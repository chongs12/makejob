package service

import (
	"context"
	"strings"
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

// TestNormalizeCompanionMessagesInjectsContext 验证陪伴上下文会被整理成 system 消息并注入到对话历史前面。
func TestNormalizeCompanionMessagesInjectsContext(t *testing.T) {
	t.Parallel()

	messages := normalizeCompanionMessages(&CompanionChatRequest{
		Messages: []ai.Message{
			{Role: "user", Content: "继续今天的任务"},
		},
		Context: map[string]any{
			"current_plan_title":  "Go 强化计划",
			"focused_task_title":  "并发复习",
			"completed_today_count": 1,
		},
	})
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages after context injection, got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Fatalf("expected first message role system, got %s", messages[0].Role)
	}
	if !strings.Contains(messages[0].Content, "current_plan_title") || !strings.Contains(messages[0].Content, "focused_task_title") {
		t.Fatalf("expected context message to contain context keys, got %q", messages[0].Content)
	}
	if messages[1].Role != "user" || messages[1].Content != "继续今天的任务" {
		t.Fatalf("unexpected user message: %#v", messages[1])
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
