package biz

import (
	"context"
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
}

// TTSClient 语音合成客户端接口
type TTSClient interface {
	// Synthesize 调用 TTS 服务合成语音，返回结构化音频结果
	Synthesize(ctx context.Context, text, voice string) (*TTSAudio, error)
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
	UserID       uint64
	Message      string
	ContextType  string
	RecentTopics []string
}

// CompanionAgentResponse AI Gateway CompanionAgent 响应
type CompanionAgentResponse struct {
	Reply       string
	Emotion     string
	Suggestions []string
}

// Live2DDirectorRequest AI Gateway Live2DDirector 请求
type Live2DDirectorRequest struct {
	Context     string
	EmotionHint string
	ReplyText   string
}

// Live2DDirectiveResponse AI Gateway Live2DDirector 响应
type Live2DDirectiveResponse struct {
	Emotion     string
	Action      string
	Reply       string
	MotionKey   string
	MotionGroup string
	DurationMs  int32
}

// ChatResult 完整对话结果
type ChatResult struct {
	Reply       string
	Emotion     string
	Action      string
	Suggestions []string
	AudioURL    string
}

// TTSAudio 表示语音合成结果，兼容二进制音频和过渡 URL 两种返回形式
type TTSAudio struct {
	AudioData []byte
	AudioURL  string
}

// ---------- 业务用例 ----------

// CompanionUseCase 陪伴助手业务用例
type CompanionUseCase struct {
	repo      CompanionRepo
	aiClient  CompanionClient
	ttsClient TTSClient
	ttsVoice  string
}

// NewCompanionUseCase 创建陪伴助手业务用例
func NewCompanionUseCase(repo CompanionRepo, aiClient CompanionClient, ttsClient TTSClient, ttsVoice string) *CompanionUseCase {
	return &CompanionUseCase{
		repo:      repo,
		aiClient:  aiClient,
		ttsClient: ttsClient,
		ttsVoice:  ttsVoice,
	}
}

// Chat 处理陪伴对话的完整流程
func (uc *CompanionUseCase) Chat(ctx context.Context, userID uint64, message, contextType string) (*ChatResult, error) {
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

	// 2. 构建 recentTopics 供 AI 参考
	var recentTopics []string
	if session.LastTopic != "" {
		recentTopics = []string{session.LastTopic}
	}

	// 3. 调用 AI Gateway.CompanionAgent
	aiResp, err := uc.aiClient.CompanionAgent(ctx, &CompanionAgentRequest{
		UserID:       userID,
		Message:      message,
		ContextType:  contextType,
		RecentTopics: recentTopics,
	})
	if err != nil {
		return nil, kratosErr.InternalServer("COMPANION_AGENT_FAILED", "陪伴对话 AI 调用失败").WithCause(err)
	}

	// 4. 调用 AI Gateway.Live2DDirector（可选，失败降级）
	live2dEmotion := aiResp.Emotion
	live2dAction := ""
	live2dResp, err := uc.aiClient.Live2DDirector(ctx, &Live2DDirectorRequest{
		Context:     contextType,
		EmotionHint: aiResp.Emotion,
		ReplyText:   aiResp.Reply,
	})
	if err == nil && live2dResp != nil {
		live2dEmotion = live2dResp.Emotion
		live2dAction = live2dResp.Action
	}

	// 5. 调用 TTS（可选，失败跳过）
	var audioURL string
	if uc.ttsClient != nil {
		voice := uc.ttsVoice
		if voice == "" {
			voice = "zh_female_shuangkuaisisi_moon_bigtts"
		}
		audioResult, synthErr := uc.ttsClient.Synthesize(ctx, aiResp.Reply, voice)
		if synthErr == nil && audioResult != nil {
			audioURL = audioResult.AudioURL
		}
	}

	// 6. 更新 session
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

	// 7. 返回完整响应
	return &ChatResult{
		Reply:       aiResp.Reply,
		Emotion:     live2dEmotion,
		Action:      live2dAction,
		Suggestions: aiResp.Suggestions,
		AudioURL:    audioURL,
	}, nil
}

// GetCompanionState 查询用户陪伴状态
func (uc *CompanionUseCase) GetCompanionState(ctx context.Context, userID uint64) (*CompanionSession, error) {
	session, err := uc.repo.GetSession(ctx, userID)
	if err != nil {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

// SynthesizeSpeech 执行语音合成并返回结构化音频结果
func (uc *CompanionUseCase) SynthesizeSpeech(ctx context.Context, text, voice string) (*TTSAudio, error) {
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
