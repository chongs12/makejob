package biz

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

// BaseModel 所有实体公共基础字段（FIX CP1: 符合全局规范 1.4，支持软删除）
type BaseModel struct {
	ID        uint           `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time      `gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"not null;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// ---------- 错误定义 ----------

var (
	ErrSessionNotFound  = kratosErr.NotFound("COMPANION_SESSION_NOT_FOUND", "陪伴会话不存在")
	ErrAIServiceFailed  = kratosErr.InternalServer("AI_SERVICE_FAILED", "AI 服务调用失败")
	ErrTTSServiceFailed = kratosErr.InternalServer("TTS_SERVICE_FAILED", "语音合成服务调用失败")
	ErrMessageTooLong   = kratosErr.BadRequest("MESSAGE_TOO_LONG", "消息内容过长")
)

// ---------- 仓库接口 ----------

// CompanionRepo 陪伴助手仓库接口，data 层必须实现
type CompanionRepo interface {
	// GetSession 获取用户的陪伴会话
	GetSession(ctx context.Context, userID uint64) (*CompanionSession, error)
	// CreateOrUpdate 创建或更新陪伴会话
	CreateOrUpdate(ctx context.Context, session *CompanionSession) error
}

// ---------- 外部客户端接口 ----------

// CompanionClient AI 服务客户端接口，调用 AI Gateway
type CompanionClient interface {
	// CompanionAgent 调用 AI Gateway 的 CompanionAgent RPC
	CompanionAgent(ctx context.Context, req *CompanionAgentRequest) (*CompanionAgentResponse, error)
	// Live2DDirector 调用 AI Gateway 的 Live2DDirector RPC
	Live2DDirector(ctx context.Context, req *Live2DDirectorRequest) (*Live2DDirectiveResponse, error)
	// GetGreeting 调用 AI Gateway 的 GetGreeting RPC（本地生成，不调用 LLM）
	GetGreeting(ctx context.Context, level, timeOfDay string) (*CompanionAgentResponse, error)
	// GetEncouragement 调用 AI Gateway 的 GetEncouragement RPC（本地生成，不调用 LLM）
	GetEncouragement(ctx context.Context, achievement string) (*CompanionAgentResponse, error)
}

// GrowthClient Growth 服务客户端接口，获取用户学习状态
type GrowthClient interface {
	// GetFocusSignals 获取用户高频薄弱点
	GetFocusSignals(ctx context.Context, userID uint64) ([]FocusSignal, error)
}

// InterviewClient Interview 服务客户端接口，获取最近面试
type InterviewClient interface {
	// ListRecent 获取用户最近面试摘要
	ListRecent(ctx context.Context, userID uint64, limit int32) ([]InterviewBrief, error)
}

// PlanClient Plan 服务客户端接口，获取当前计划
type PlanClient interface {
	// GetCurrentPlan 获取用户当前活跃计划
	GetCurrentPlan(ctx context.Context, userID uint64) (*PlanBrief, error)
}

// FocusSignal 高频薄弱点
type FocusSignal struct {
	Tag                 string
	TopicTitle          string
	OccurrenceCount     int32
	PrimaryQuestionSet  string
	RelatedQuestionSets []string
	RecommendedActions  []string
	Reason              string
}

// SuggestedAction 结构化引导动作，对齐 companion.proto SuggestedAction。
type SuggestedAction struct {
	Type   string
	Target string
	Params string
}

// InlineTriggerItem 字幕行内可点击关键词。
type InlineTriggerItem struct {
	Keyword      string
	ActionType   string
	Target       string
	PositionHint string
}

// IntentInfo LLM 意图识别结果。
type IntentInfo struct {
	Type       string
	Confidence float64
	Stage      string
}

// PendingAction 待执行动作。
type PendingAction struct {
	Type        string
	Ready       bool
	Params      map[string]string
	MissingInfo []string
}

// ConversationState 多轮对话状态。
type ConversationState struct {
	Phase           string
	CollectedParams map[string]string
}

// InterviewBrief 面试摘要
type InterviewBrief struct {
	ID     uint64
	Status string
	Score  int32
}

// PlanBrief 计划摘要
type PlanBrief struct {
	ID             uint64
	Title          string
	Status         string
	TotalTasks     int32
	CompletedTasks int32
	Progress       float64
}

// TTSClient 语音合成客户端接口（简化版，兼容现有实现）
type TTSClient interface {
	// Synthesize 调用 TTS 服务合成语音，返回结构化音频结果
	Synthesize(ctx context.Context, text, voice string) (*TTSAudio, error)
}

// TTSProvider TTS 语音合成供应商接口（对齐单体 tts.TTSProvider）
type TTSProvider interface {
	// Synthesize 将文本合成为语音
	Synthesize(ctx context.Context, req TTSRequest) (*TTSResult, error)
	// GetSupportedEngines 获取支持的引擎列表
	GetSupportedEngines() []string
}

// TTSRequest TTS 合成请求
type TTSRequest struct {
	Text    string
	VoiceID string
	Engine  string
	Speed   float64
	Pitch   float64
	Volume  float64
	Format  string
}

// TTSResult TTS 合成结果
type TTSResult struct {
	AudioURL   string
	AudioData  []byte
	Duration   float64
	Format     string
	SampleRate int
	CharCount  int
}

// ---------- 领域实体 ----------

// CompanionSession 陪伴助手会话领域实体（FIX CP1: 嵌入 BaseModel 支持软删除）
type CompanionSession struct {
	BaseModel
	UserID       uint64    `gorm:"uniqueIndex"`
	LastEmotion  string    `gorm:"size:20"`
	LastTopic    string    `gorm:"size:100"`
	SessionCount int32     `gorm:"default:0"`
	LastChatAt   time.Time `gorm:"autoUpdateTime"`
	MessagesJSON string    `gorm:"type:text"` // 最近 10 条消息 JSON
}

// ChatMessage 对话消息
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ---------- 请求/响应结构体 ----------

// CompanionAgentRequest AI Gateway CompanionAgent 请求
type CompanionAgentRequest struct {
	UserID                uint64
	Message               string
	ContextType           string
	Username              string
	RecentTopics          []string
	ConversationStateJSON string
}

// CompanionAgentResponse AI Gateway CompanionAgent 响应
type CompanionAgentResponse struct {
	Reply             string
	Emotion           string
	Suggestions       []string
	Action            string
	SuggestedActions  []SuggestedAction
	Live2DDirective   *Live2DDirectiveResponse
	InlineTriggers    []InlineTriggerItem
	Intent            *IntentInfo
	PendingAction     *PendingAction
	ConversationState *ConversationState
}

// Live2DDirectorRequest AI Gateway Live2DDirector 请求
type Live2DDirectorRequest struct {
	Context     string
	EmotionHint string
	ReplyText   string
}

// Live2DDirectiveResponse AI Gateway Live2DDirector 响应
type Live2DDirectiveResponse struct {
	Emotion            string
	Action             string
	Reply              string
	MotionKey          string
	MotionGroup        string
	MotionPriority     string
	MotionDurationMS   int
	Intensity          float64
	DurationMS         int
	MouthOpen          *float64
	Source             string
	ExpressionMix      []ExpressionLayer
	ParameterOverrides []ParameterOverride
}

// ExpressionLayer Live2D 表情混合层
type ExpressionLayer struct {
	Key    string
	Weight float64
}

// ParameterOverride Live2D 参数覆盖
type ParameterOverride struct {
	ID    string
	Value float64
}

// ChatResult 完整对话结果
type ChatResult struct {
	Reply             string
	Emotion           string
	Action            string
	Suggestions       []string
	SuggestedActions  []SuggestedAction
	AudioURL          string
	Live2DDirective   *Live2DDirectiveResponse
	InlineTriggers    []InlineTriggerItem
	Intent            *IntentInfo
	PendingAction     *PendingAction
	ConversationState *ConversationState
}

// TTSAudio 表示语音合成结果，兼容二进制音频和过渡 URL 两种返回形式
type TTSAudio struct {
	AudioData []byte
	AudioURL  string
}

// ---------- 业务用例 ----------

// CompanionUseCase 陪伴助手业务用例
type CompanionUseCase struct {
	repo              CompanionRepo
	aiClient          CompanionClient
	ttsClient         TTSClient
	ttsVoice          string
	asrConfigRepo     ASRConfigRepo
	adminConfigRepo   AdminConfigRepo
	asrProviderFactory func(*ASRConfig) (ASRProvider, error)
	growthClient      GrowthClient
	interviewClient   InterviewClient
	planClient        PlanClient
	sceneTTSService   SceneTTSService
}

// NewCompanionUseCase 创建陪伴助手业务用例
func NewCompanionUseCase(
	repo CompanionRepo,
	aiClient CompanionClient,
	ttsClient TTSClient,
	ttsVoice string,
	opts ...CompanionOption,
) *CompanionUseCase {
	uc := &CompanionUseCase{
		repo:      repo,
		aiClient:  aiClient,
		ttsClient: ttsClient,
		ttsVoice:  ttsVoice,
	}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// CompanionOption 陪伴用例可选配置
type CompanionOption func(*CompanionUseCase)

// WithGrowthClient 配置 Growth 服务客户端
func WithGrowthClient(c GrowthClient) CompanionOption {
	return func(uc *CompanionUseCase) { uc.growthClient = c }
}

// WithInterviewClient 配置 Interview 服务客户端
func WithInterviewClient(c InterviewClient) CompanionOption {
	return func(uc *CompanionUseCase) { uc.interviewClient = c }
}

// WithPlanClient 配置 Plan 服务客户端
func WithPlanClient(c PlanClient) CompanionOption {
	return func(uc *CompanionUseCase) { uc.planClient = c }
}

// WithSceneTTSService 配置场景级 TTS 服务
func WithSceneTTSService(s SceneTTSService) CompanionOption {
	return func(uc *CompanionUseCase) { uc.sceneTTSService = s }
}

// WithASRProviderFactory 配置 ASR 供应商工厂函数（从配置记录创建 Provider）
func WithASRProviderFactory(f func(*ASRConfig) (ASRProvider, error)) CompanionOption {
	return func(uc *CompanionUseCase) { uc.asrProviderFactory = f }
}

// WithASRConfigRepo 配置 ASR 配置仓库
func WithASRConfigRepo(r ASRConfigRepo) CompanionOption {
	return func(uc *CompanionUseCase) { uc.asrConfigRepo = r }
}

// WithAdminConfigRepo 配置管理后台配置仓库
func WithAdminConfigRepo(r AdminConfigRepo) CompanionOption {
	return func(uc *CompanionUseCase) { uc.adminConfigRepo = r }
}

// Chat 处理陪伴对话的完整流程。conversationStateJSON 为上一轮对话状态的 JSON 序列化。
func (uc *CompanionUseCase) Chat(ctx context.Context, userID uint64, message, contextType, live2DModelKey string, conversationStateJSON string) (*ChatResult, error) {
	// 1. 加载或创建 companion_session
	session, err := uc.repo.GetSession(ctx, userID)
	if err != nil {
		// FIX CP4: 区分"记录不存在"和"DB 连接失败"
		if err != gorm.ErrRecordNotFound {
			return nil, kratosErr.InternalServer("SESSION_QUERY_FAILED", "查询陪伴会话失败").WithCause(err)
		}
		// 会话不存在则创建新会话
		session = &CompanionSession{
			UserID:       userID,
			LastEmotion:  "neutral",
			SessionCount: 0,
			MessagesJSON: "[]",
		}
	}

	// 2. 服务端上下文注入（对齐单体 enrichRequestContext）
	enrichedMessage := uc.enrichContext(ctx, userID, message)

	// 3. 构建 recentTopics 供 AI 参考
	var recentTopics []string
	if session.LastTopic != "" {
		recentTopics = []string{session.LastTopic}
	}

	// 4. 调用 AI Gateway.CompanionAgent
	aiResp, err := uc.aiClient.CompanionAgent(ctx, &CompanionAgentRequest{
		UserID:                userID,
		Message:               enrichedMessage,
		ContextType:           contextType,
		Username:              "", // TODO: 从用户服务获取用户名
		RecentTopics:          recentTopics,
		ConversationStateJSON: conversationStateJSON,
	})
	if err != nil {
		return nil, kratosErr.InternalServer("COMPANION_AGENT_FAILED", "陪伴对话 AI 调用失败").WithCause(err)
	}

	// 5. 清理思维链标签（对齐单体 sanitizeCompanionReply）
	replyContent := sanitizeCompanionReply(aiResp.Reply)
	if replyContent == "" {
		replyContent = aiResp.Reply
	}

	// 6. 使用 AI Gateway 返回的 action（已从 emotion 推导）
	live2dEmotion := aiResp.Emotion
	live2dAction := aiResp.Action
	live2dResp := aiResp.Live2DDirective

	// 7. 调用 TTS（可选，失败跳过）
	var audioURL string
	if uc.sceneTTSService != nil {
		// 使用场景级 TTS 服务（三级回退）
		result, synthErr := uc.sceneTTSService.SynthesizeForScene(ctx, SceneTTSRequest{
			Scene:          Live2DSceneCompanion,
			Live2DModelKey: live2DModelKey,
			Text:           replyContent,
		})
		if synthErr == nil && result != nil {
			audioURL = result.AudioURL
		} else if synthErr != nil {
			log.Context(ctx).Warnf("scene tts failed: %v", synthErr)
		}
	} else if uc.ttsClient != nil {
		// 降级：直接使用全局 TTS 客户端
		voice := uc.ttsVoice
		if voice == "" {
			voice = "zh_female_shuangkuaisisi_moon_bigtts"
		}
		audioResult, synthErr := uc.ttsClient.Synthesize(ctx, replyContent, voice)
		if synthErr == nil && audioResult != nil {
			audioURL = audioResult.AudioURL
		} else if synthErr != nil {
			log.Context(ctx).Warnf("tts client failed: %v", synthErr)
		}
	}

	// 8. 更新 session
	session.LastEmotion = live2dEmotion
	if len(aiResp.Suggestions) > 0 {
		session.LastTopic = aiResp.Suggestions[0]
	}
	session.SessionCount++
	session.LastChatAt = time.Now()

	if err := uc.repo.CreateOrUpdate(ctx, session); err != nil {
		// FIX CP3: session 更新失败记录日志，不影响主流程返回
		log.Context(ctx).Warnf("更新陪伴会话失败: user_id=%d err=%v", userID, err)
	}

	// 9. 返回完整响应，保证数组字段非 nil 避免前端 .map/.length 崩溃
	suggestions := aiResp.Suggestions
	if suggestions == nil {
		suggestions = []string{}
	}
	return &ChatResult{
		Reply:             replyContent,
		Emotion:           live2dEmotion,
		Action:            live2dAction,
		Suggestions:       suggestions,
		SuggestedActions:  aiResp.SuggestedActions,
		AudioURL:          audioURL,
		Live2DDirective:   live2dResp,
		InlineTriggers:    aiResp.InlineTriggers,
		Intent:            aiResp.Intent,
		PendingAction:     aiResp.PendingAction,
		ConversationState: aiResp.ConversationState,
	}, nil
}

// enrichContext 服务端上下文注入，从 growth/interview/plan 服务查询用户学习状态。
// 对齐单体 companionService.enrichRequestContext 实现。
func (uc *CompanionUseCase) enrichContext(ctx context.Context, userID uint64, message string) string {
	if userID == 0 {
		return message
	}

	parts := make([]string, 0, 4)

	// 查询高频薄弱点与可推荐题集，供 LLM 生成导航建议
	if uc.growthClient != nil {
		signals, err := uc.growthClient.GetFocusSignals(ctx, userID)
		if err == nil && len(signals) > 0 {
			top := signals
			if len(top) > 3 {
				top = top[:3]
			}
			focusLines := make([]string, 0, len(top))
			for _, s := range top {
				line := s.Tag
				if s.TopicTitle != "" {
					line = s.TopicTitle
				}
				if s.Reason != "" {
					line = fmt.Sprintf("%s（%s）", line, s.Reason)
				}
				if s.PrimaryQuestionSet != "" {
					line = fmt.Sprintf("%s；建议题集：%s", line, s.PrimaryQuestionSet)
				}
				if line != "" {
					focusLines = append(focusLines, line)
				}
			}
			if len(focusLines) > 0 {
				parts = append(parts, fmt.Sprintf("当前高频薄弱点与推荐题集：\n- %s", strings.Join(focusLines, "\n- ")))
			}
		}
	}

	// 查询最近面试
	if uc.interviewClient != nil {
		interviews, err := uc.interviewClient.ListRecent(ctx, userID, 3)
		if err == nil && len(interviews) > 0 {
			briefs := make([]string, 0, len(interviews))
			for _, i := range interviews {
				if i.Status == "completed" {
					briefs = append(briefs, fmt.Sprintf("得分%d", i.Score))
				} else {
					briefs = append(briefs, "进行中")
				}
			}
			parts = append(parts, fmt.Sprintf("最近 %d 场面试：%s", len(interviews), strings.Join(briefs, "、")))
		}
	}

	// 查询当前计划
	if uc.planClient != nil {
		plan, err := uc.planClient.GetCurrentPlan(ctx, userID)
		if err == nil && plan != nil {
			progress := int(plan.Progress * 100)
			parts = append(parts, fmt.Sprintf("当前计划：%s（进度 %d%%）", plan.Title, progress))
		}
	}

	// 如果没有上下文信息，直接返回原始消息
	if len(parts) == 0 {
		return message
	}

	// 将上下文信息注入到消息前面
	contextBlock := "[用户学习上下文]\n" + strings.Join(parts, "\n")
	return contextBlock + "\n\n" + message
}

// sanitizeCompanionReply 清理陪伴回复中的思维链标签，避免直接显示在 Live2D 对话框中。
// 对齐单体 runtime.sanitizeCompanionReply 实现。
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

// normalizeCompanionMessages 统一整理陪伴对话历史，并在有上下文时自动注入一条 system 消息。
// 对齐单体 service.normalizeCompanionMessages 实现。
func normalizeCompanionMessages(messages []ChatMessage, contextMap map[string]string) []ChatMessage {
	contextMessage := buildCompanionContextMessage(contextMap)

	if len(messages) > 0 {
		result := make([]ChatMessage, 0, len(messages)+1)
		if contextMessage != "" {
			result = append(result, ChatMessage{
				Role:    "system",
				Content: contextMessage,
			})
		}
		for _, item := range messages {
			if strings.TrimSpace(item.Content) == "" {
				continue
			}
			role := strings.TrimSpace(item.Role)
			if role == "" {
				role = "user"
			}
			result = append(result, ChatMessage{
				Role:    role,
				Content: strings.TrimSpace(item.Content),
			})
		}
		return result
	}

	return messages
}

// buildCompanionContextMessage 将陪伴上下文整理成一条稳定的系统消息。
// 对齐单体 service.buildCompanionContextMessage 实现。
func buildCompanionContextMessage(contextMap map[string]string) string {
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
		valueText := strings.TrimSpace(contextMap[key])
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

// GetCompanionState 查询用户陪伴状态
func (uc *CompanionUseCase) GetCompanionState(ctx context.Context, userID uint64) (*CompanionSession, error) {
	session, err := uc.repo.GetSession(ctx, userID)
	if err != nil {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

// SynthesizeSpeech 执行语音合成并返回结构化音频结果。
// /companion/tts 当前仅面试标准语音链路调用，优先走场景级 TTS（admin 配置：
// live2d 模型绑定 -> 面试场景默认 -> 全局），失败再回退到 config.yaml 的 ttsClient。
func (uc *CompanionUseCase) SynthesizeSpeech(ctx context.Context, text, voice string) (*TTSAudio, error) {
	if uc.sceneTTSService != nil {
		result, err := uc.sceneTTSService.SynthesizeForScene(ctx, SceneTTSRequest{
			Scene: Live2DSceneInterview,
			Text:  text,
		})
		if err == nil && result != nil {
			return &TTSAudio{AudioURL: result.AudioURL, AudioData: result.AudioData}, nil
		}
		if err != nil {
			log.Context(ctx).Warnf("scene tts failed, fallback to ttsClient: %v", err)
		}
	}

	if uc.ttsClient == nil {
		return nil, kratosErr.InternalServer("TTS_NOT_CONFIGURED", "语音合成服务未配置")
	}
	if voice == "" {
		voice = uc.ttsVoice
	}
	audioResult, err := uc.ttsClient.Synthesize(ctx, text, voice)
	if err != nil {
		return nil, kratosErr.InternalServer("TTS_SYNTHESIS_FAILED", "语音合成失败").WithCause(err)
	}
	return audioResult, nil
}

// RecognizeSpeech 执行语音识别并返回结构化结果（每次调用动态读取配置，支持多 provider 降级）
func (uc *CompanionUseCase) RecognizeSpeech(ctx context.Context, req *ASRRequest) (*ASRResult, error) {
	if len(req.AudioData) == 0 {
		return nil, kratosErr.BadRequest("ASR_EMPTY_AUDIO", "音频数据不能为空")
	}

	providers, err := uc.resolveAllASRProviders(ctx)
	if err != nil || len(providers) == 0 {
		return nil, kratosErr.InternalServer("ASR_NOT_CONFIGURED", "语音识别服务未配置")
	}

	// 依次尝试每个 provider，第一个成功就返回
	var lastErr error
	for _, provider := range providers {
		result, err := provider.Recognize(ctx, *req)
		if err == nil {
			return result, nil
		}
		lastErr = err
		log.Context(ctx).Warnf("ASR provider failed, trying next: %v", err)
	}

	return nil, kratosErr.InternalServer("ASR_RECOGNITION_FAILED", "语音识别失败").WithCause(lastErr)
}

// resolveAllASRProviders 按优先级解析所有可用的 ASR Provider 列表。
func (uc *CompanionUseCase) resolveAllASRProviders(ctx context.Context) ([]ASRProvider, error) {
	if uc.asrProviderFactory == nil {
		return nil, fmt.Errorf("asr provider factory not configured")
	}

	var providers []ASRProvider

	// 1. 默认绑定的配置
	if uc.adminConfigRepo != nil {
		if item, err := uc.adminConfigRepo.GetByKey(ctx, ASRDefaultConfigKeyCompanion); err == nil && item != nil && item.ConfigValue != "" {
			var id uint
			fmt.Sscanf(item.ConfigValue, "%d", &id)
			if id > 0 && uc.asrConfigRepo != nil {
				if cfg, err := uc.asrConfigRepo.GetByID(ctx, id); err == nil && cfg != nil && cfg.IsActive {
					if p, err := uc.asrProviderFactory(cfg); err == nil {
						providers = append(providers, p)
					}
				}
			}
		}
	}

	// 2. 数据库中其他启用的配置（排除已添加的）
	if uc.asrConfigRepo != nil {
		if configs, err := uc.asrConfigRepo.List(ctx); err == nil {
			added := make(map[uint]bool)
			for _, p := range providers {
				// 标记已添加的
				_ = p
			}
			for _, cfg := range configs {
				if added[cfg.ID] {
					continue
				}
				if p, err := uc.asrProviderFactory(&cfg); err == nil {
					providers = append(providers, p)
					added[cfg.ID] = true
				}
			}
		}
	}

	// 3. Mock 兜底
	providers = append(providers, NewMockASRProvider())

	return providers, nil
}

// ListASRConfigs 获取所有 ASR 配置
func (uc *CompanionUseCase) ListASRConfigs(ctx context.Context) ([]ASRConfig, error) {
	if uc.asrConfigRepo == nil {
		return nil, nil
	}
	return uc.asrConfigRepo.List(ctx)
}

// CreateASRConfig 创建 ASR 配置
func (uc *CompanionUseCase) CreateASRConfig(ctx context.Context, config *ASRConfig) error {
	if uc.asrConfigRepo == nil {
		return kratosErr.InternalServer("ASR_CONFIG_NOT_AVAILABLE", "ASR 配置管理不可用")
	}
	return uc.asrConfigRepo.Create(ctx, config)
}

// UpdateASRConfig 更新 ASR 配置
func (uc *CompanionUseCase) UpdateASRConfig(ctx context.Context, config *ASRConfig) error {
	if uc.asrConfigRepo == nil {
		return kratosErr.InternalServer("ASR_CONFIG_NOT_AVAILABLE", "ASR 配置管理不可用")
	}
	return uc.asrConfigRepo.Update(ctx, config)
}

// DeleteASRConfig 删除 ASR 配置
func (uc *CompanionUseCase) DeleteASRConfig(ctx context.Context, id uint) error {
	if uc.asrConfigRepo == nil {
		return kratosErr.InternalServer("ASR_CONFIG_NOT_AVAILABLE", "ASR 配置管理不可用")
	}
	return uc.asrConfigRepo.Delete(ctx, id)
}

// GetDefaultASRConfigID 获取全局默认 ASR 配置 ID
func (uc *CompanionUseCase) GetDefaultASRConfigID(ctx context.Context) uint {
	if uc.adminConfigRepo == nil {
		return 0
	}
	item, err := uc.adminConfigRepo.GetByKey(ctx, ASRDefaultConfigKeyCompanion)
	if err != nil || item == nil {
		return 0
	}
	var id uint
	fmt.Sscanf(item.ConfigValue, "%d", &id)
	return id
}

// SetDefaultASRConfigID 设置全局默认 ASR 配置 ID
func (uc *CompanionUseCase) SetDefaultASRConfigID(ctx context.Context, id uint) error {
	if uc.adminConfigRepo == nil {
		return kratosErr.InternalServer("ADMIN_CONFIG_NOT_AVAILABLE", "管理配置不可用")
	}
	value := ""
	if id > 0 {
		value = fmt.Sprintf("%d", id)
	}
	return uc.adminConfigRepo.Upsert(ctx, &AdminConfig{
		ConfigKey:   ASRDefaultConfigKeyCompanion,
		ConfigValue: value,
	})
}
