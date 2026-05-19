package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
	"makejob-backend/internal/tts"
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
	if resp.Live2DDirective != nil {
		t.Fatalf("expected nil live2d directive without configured service, got %#v", resp.Live2DDirective)
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
			"current_plan_title":    "Go 强化计划",
			"focused_task_title":    "并发复习",
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

// TestCompanionServiceChatSkipsSlowLive2DDirective 验证 Live2D 指令生成过慢时不会阻塞主回复。
func TestCompanionServiceChatSkipsSlowLive2DDirective(t *testing.T) {
	t.Parallel()

	originalTimeout := live2DDirectiveTimeout
	live2DDirectiveTimeout = 20 * time.Millisecond
	t.Cleanup(func() {
		live2DDirectiveTimeout = originalTimeout
	})

	svc := NewCompanionService(
		stubCompanionAgent{
			reply: ai.CompanionResponse{
				Content: "先把主回复返回给前端。",
				Emotion: "neutral",
				Action:  "idle",
			},
		},
		stubLive2DDirectiveService{blockUntilDone: true},
	)

	startedAt := time.Now()
	resp, err := svc.Chat(context.Background(), 1, &CompanionChatRequest{
		Message:        "继续吧",
		Live2DModelKey: "db:1",
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 300*time.Millisecond {
		t.Fatalf("expected chat to return quickly, got %s", elapsed)
	}
	if resp.Live2DDirective != nil {
		t.Fatalf("expected slow live2d directive to be skipped, got %#v", resp.Live2DDirective)
	}
}

// TestCompanionServiceChatIncludesTTSAudio 验证陪伴聊天在 TTS 可用时会把音频资源地址一并返回给前端。
func TestCompanionServiceChatIncludesTTSAudio(t *testing.T) {
	t.Parallel()

	svc := NewCompanionService(
		stubCompanionAgent{
			reply: ai.CompanionResponse{
				Content: "今天先把并发模型的核心概念过一遍。",
				Emotion: "steady",
				Action:  "nod",
			},
		},
		stubCompanionTTSProvider{
			result: tts.SynthesizeResult{
				AudioURL:   "/static/mock/companion_tts.mp3",
				Duration:   2.4,
				Format:     "mp3",
				SampleRate: 24000,
			},
		},
	)

	resp, err := svc.Chat(context.Background(), 1, &CompanionChatRequest{
		Message: "继续今天的复习计划",
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if resp.AudioURL != "/static/mock/companion_tts.mp3" {
		t.Fatalf("expected audio url to be returned, got %#v", resp)
	}
	if resp.AudioFormat != "mp3" || resp.AudioSampleRate != 24000 {
		t.Fatalf("unexpected tts audio metadata: %#v", resp)
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

// stubLive2DDirectiveService 模拟 Live2D 指令服务，便于验证超时兜底是否生效。
type stubLive2DDirectiveService struct {
	blockUntilDone bool
}

// stubCompanionTTSProvider 模拟陪伴 TTS Provider，便于验证响应里是否透出音频信息。
type stubCompanionTTSProvider struct {
	result tts.SynthesizeResult
	err    error
}

// Synthesize 返回测试预设的语音合成结果。
func (s stubCompanionTTSProvider) Synthesize(context.Context, tts.SynthesizeRequest) (tts.SynthesizeResult, error) {
	return s.result, s.err
}

// ListVoices 满足接口要求，当前测试不依赖音色列表。
func (s stubCompanionTTSProvider) ListVoices(context.Context, string) ([]tts.Voice, error) {
	return nil, nil
}

// GetVoice 满足接口要求，当前测试不依赖音色详情。
func (s stubCompanionTTSProvider) GetVoice(context.Context, string) (tts.Voice, error) {
	return tts.Voice{}, nil
}

// GetSupportedEngines 返回一个稳定引擎标识，供陪伴服务构造合成请求。
func (s stubCompanionTTSProvider) GetSupportedEngines() []string {
	return []string{"mock"}
}

// ResolveActiveManifest 返回最小可用 manifest，满足陪伴服务的前置校验。
func (s stubLive2DDirectiveService) ResolveActiveManifest(context.Context, string, string) (*ai.Live2DManifest, error) {
	return &ai.Live2DManifest{
		ModelKey:  "db:1",
		ModelName: "test",
		Scene:     model.Live2DSceneCompanion,
	}, nil
}

// GenerateDirective 按测试需要阻塞到上下文结束，模拟慢速 LLM 指令链路。
func (s stubLive2DDirectiveService) GenerateDirective(ctx context.Context, req ai.Live2DDirectiveContext) (*ai.Live2DDirective, error) {
	if !s.blockUntilDone {
		return &ai.Live2DDirective{Reply: req.AssistantReply}, nil
	}
	<-ctx.Done()
	return nil, ctx.Err()
}
