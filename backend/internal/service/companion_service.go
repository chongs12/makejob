package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/tts"
	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
)

type CompanionChatRequest struct {
	Message        string         `json:"message"`
	Messages       []ai.Message   `json:"messages"`
	UserEmotion    string         `json:"user_emotion"`
	Context        map[string]any `json:"context"`
	Live2DModelKey string         `json:"live2d_model_key"`
}

type CompanionChatResponse struct {
	Content         string              `json:"content"`
	Reply           string              `json:"reply"`
	Emotion         string              `json:"emotion"`
	Mood            string              `json:"mood"`
	Action          string              `json:"action"`
	AudioURL        string              `json:"audio_url,omitempty"`
	AudioDuration   float64             `json:"audio_duration,omitempty"`
	AudioFormat     string              `json:"audio_format,omitempty"`
	AudioSampleRate int                 `json:"audio_sample_rate,omitempty"`
	Live2DDirective *ai.Live2DDirective `json:"live2d_directive,omitempty"`
}

type CompanionService interface {
	Chat(ctx context.Context, userID uint, req *CompanionChatRequest) (*CompanionChatResponse, error)
}

type companionService struct {
	companionAgent  ai.CompanionAgent
	live2dDirective Live2DDirectiveService
	ttsSceneService SceneTTSService
	ttsProvider     tts.TTSProvider
}

// NewCompanionService 创建陪伴服务，并按传入依赖自动接入 Live2D 指令和 TTS 能力。
func NewCompanionService(companionAgent ai.CompanionAgent, dependencies ...any) CompanionService {
	var directiveService Live2DDirectiveService
	var ttsSceneService SceneTTSService
	var ttsProvider tts.TTSProvider
	for _, dependency := range dependencies {
		switch typedDependency := dependency.(type) {
		case Live2DDirectiveService:
			directiveService = typedDependency
		case SceneTTSService:
			ttsSceneService = typedDependency
		case tts.TTSProvider:
			ttsProvider = typedDependency
		}
	}
	return &companionService{
		companionAgent:  companionAgent,
		live2dDirective: directiveService,
		ttsSceneService: ttsSceneService,
		ttsProvider:     ttsProvider,
	}
}

// Chat 生成陪伴回复，并在有选中模型时追加结构化 Live2D 控制指令。
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
	var directive *ai.Live2DDirective
	if s.live2dDirective != nil && strings.TrimSpace(req.Live2DModelKey) != "" {
		manifest, err := s.live2dDirective.ResolveActiveManifest(ctx, model.Live2DSceneCompanion, req.Live2DModelKey)
		if err == nil && manifest != nil {
			directiveCtx, cancel := context.WithTimeout(ctx, live2DDirectiveTimeout)
			directive, _ = s.live2dDirective.GenerateDirective(directiveCtx, ai.Live2DDirectiveContext{
				Scene:          model.Live2DSceneCompanion,
				Model:          *manifest,
				UserMessage:    latestCompanionUserMessage(messages),
				AssistantReply: replyContent,
				UserEmotion:    userEmotion,
				RecentMessages: messages,
			})
			cancel()
		}
	}
	ttsAudio := synthesizeCompanionSpeech(ctx, s.ttsSceneService, s.ttsProvider, replyContent, req.Live2DModelKey)
	return &CompanionChatResponse{
		Content:         replyContent,
		Reply:           replyContent,
		Emotion:         reply.Emotion,
		Mood:            reply.Emotion,
		Action:          reply.Action,
		AudioURL:        ttsAudio.AudioURL,
		AudioDuration:   ttsAudio.Duration,
		AudioFormat:     ttsAudio.Format,
		AudioSampleRate: ttsAudio.SampleRate,
		Live2DDirective: directive,
	}, nil
}

// synthesizeCompanionSpeech 为陪伴回复生成可选语音资源，失败时静默降级到纯文本。
func synthesizeCompanionSpeech(ctx context.Context, sceneService SceneTTSService, provider tts.TTSProvider, text string, live2DModelKey string) tts.SynthesizeResult {
	if strings.TrimSpace(text) == "" {
		return tts.SynthesizeResult{}
	}

	if sceneService != nil {
		result, err := sceneService.SynthesizeForScene(ctx, SceneTTSRequest{
			Scene:          model.Live2DSceneCompanion,
			Live2DModelKey: strings.TrimSpace(live2DModelKey),
			Text:           text,
		})
		if err == nil {
			return result
		}
		applogger.Warn("companion scene tts failed and will fallback",
			zap.String("scene", model.Live2DSceneCompanion),
			zap.String("live2d_model_key", strings.TrimSpace(live2DModelKey)),
			zap.Error(err),
		)
	}

	if provider == nil {
		return tts.SynthesizeResult{}
	}

	result, err := provider.Synthesize(ctx, tts.SynthesizeRequest{
		Text:   text,
		Engine: resolveCompanionTTSEngine(provider),
	})
	if err != nil {
		return tts.SynthesizeResult{}
	}

	return result
}

// resolveCompanionTTSEngine 返回当前 TTS Provider 优先使用的引擎标识。
func resolveCompanionTTSEngine(provider tts.TTSProvider) string {
	if provider == nil {
		return ""
	}

	supportedEngines := provider.GetSupportedEngines()
	if len(supportedEngines) == 0 {
		return ""
	}

	return strings.TrimSpace(supportedEngines[0])
}

// normalizeCompanionMessages 统一整理陪伴对话历史，并在有上下文时自动注入一条 system 消息。
func normalizeCompanionMessages(req *CompanionChatRequest) []ai.Message {
	contextMessage := buildCompanionContextMessage(req.Context)
	if len(req.Messages) > 0 {
		messages := make([]ai.Message, 0, len(req.Messages)+1)
		if contextMessage != "" {
			messages = append(messages, ai.Message{
				Role:    "system",
				Content: contextMessage,
			})
		}
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

	messages := make([]ai.Message, 0, 2)
	if contextMessage != "" {
		messages = append(messages, ai.Message{
			Role:    "system",
			Content: contextMessage,
		})
	}
	messages = append(messages, ai.Message{
		Role:    "user",
		Content: message,
	})

	return messages
}

// buildCompanionContextMessage 将前端传入的陪伴上下文整理成一条稳定的系统消息，帮助模型回复更贴近当前计划与任务。
func buildCompanionContextMessage(contextMap map[string]any) string {
	if len(contextMap) == 0 {
		return ""
	}

	keys := make([]string, 0, len(contextMap))
	for key := range contextMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys)+1)
	lines = append(lines, "当前学习陪伴上下文：")
	for _, key := range keys {
		valueText := strings.TrimSpace(fmt.Sprint(contextMap[key]))
		if valueText == "" || valueText == "[]" || valueText == "<nil>" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", key, valueText))
	}

	if len(lines) == 1 {
		return ""
	}

	return strings.Join(lines, "\n")
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

// latestCompanionUserMessage 返回最近一条用户消息，供 Live2D 指令提示词使用。
func latestCompanionUserMessage(messages []ai.Message) string {
	for index := len(messages) - 1; index >= 0; index-- {
		item := messages[index]
		if strings.EqualFold(strings.TrimSpace(item.Role), "user") && strings.TrimSpace(item.Content) != "" {
			return strings.TrimSpace(item.Content)
		}
	}
	return ""
}
