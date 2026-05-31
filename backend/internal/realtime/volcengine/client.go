package volcengine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	appconfig "makejob-backend/internal/config"
	applogger "makejob-backend/pkg/logger"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// defaultRealtimeDialogAppKey 是火山实时语音网关要求的固定 AppKey。
	defaultRealtimeDialogAppKey = "PlgvMymc7f3tQnJ6"
	// EventStartConnection 表示初始化底层实时语音连接。
	EventStartConnection int32 = 1
	// EventFinishConnection 表示释放底层实时语音连接。
	EventFinishConnection int32 = 2
	// EventStartSession 表示启动一轮实时语音会话。
	EventStartSession int32 = 100
	// EventFinishSession 表示结束当前实时语音会话。
	EventFinishSession int32 = 102
	// EventTaskRequest 表示上传一段用户音频。
	EventTaskRequest int32 = 200
	// EventSayHello 表示主动唤起模型先说一段开场白。
	EventSayHello int32 = 300
	// EventEndASR 表示在按键说话模式下显式告诉服务端本轮用户音频已结束。
	EventEndASR int32 = 400
	// EventChatTextQuery 表示向实时模型发送一段文本输入。
	EventChatTextQuery int32 = 501
	// EventChatTTSText 表示向实时模型发送安抚话术（用于RAG检索期间）。
	EventChatTTSText int32 = 500
	// EventChatRAGText 表示向实时模型注入外部RAG数据。
	EventChatRAGText int32 = 502

	// EventConnectionStarted 表示连接建立成功。
	EventConnectionStarted int32 = 50
	// EventSessionStarted 表示实时会话建立成功。
	EventSessionStarted int32 = 150
	// EventSessionFinished 表示当前实时会话已结束。
	EventSessionFinished int32 = 152
	// EventSessionFailed 表示当前实时会话创建失败。
	EventSessionFailed int32 = 153
	// EventTTSSentenceStart 表示一段待播报文本开始合成。
	EventTTSSentenceStart int32 = 350
	// EventTTSSentenceEnd 表示当前字幕句子播报完毕。
	EventTTSSentenceEnd int32 = 351
	// EventTTSResponse 表示一段音频数据块。
	EventTTSResponse int32 = 352
	// EventTTSEnded 表示本轮播报音频结束。
	EventTTSEnded int32 = 359
	// EventASRInfo 表示识别到用户首字，可用于打断播报。
	EventASRInfo int32 = 450
	// EventASRResponse 表示用户语音识别文本片段。
	EventASRResponse int32 = 451
	// EventASREnded 表示用户语音输入结束。
	EventASREnded int32 = 459
	// EventChatResponse 表示模型回复的文本内容。
	EventChatResponse int32 = 550
	// EventChatTextQueryConfirmed 表示服务端已确认接收到文本 query。
	EventChatTextQueryConfirmed int32 = 553
	// EventChatEnded 表示模型当前文本回复结束。
	EventChatEnded int32 = 559
)

type startSessionPayload struct {
	ASR    asrPayload    `json:"asr"`
	TTS    ttsPayload    `json:"tts"`
	Dialog dialogPayload `json:"dialog"`
}

type asrPayload struct {
	AudioInfo audioInfo              `json:"audio_info"`
	Extra     map[string]interface{} `json:"extra"`
}

type audioInfo struct {
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
	Channel    int    `json:"channel"`
}

type ttsPayload struct {
	Speaker     string      `json:"speaker"`
	AudioConfig audioConfig `json:"audio_config"`
}

type audioConfig struct {
	Channel    int    `json:"channel"`
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
}

type dialogPayload struct {
	DialogID          string                 `json:"dialog_id"`
	BotName           string                 `json:"bot_name"`
	SystemRole        string                 `json:"system_role"`
	SpeakingStyle     string                 `json:"speaking_style"`
	CharacterManifest string                 `json:"character_manifest,omitempty"`
	Location          *locationInfo          `json:"location,omitempty"`
	Extra             map[string]interface{} `json:"extra"`
}

type locationInfo struct {
	City string `json:"city"`
}

type chatTextQueryPayload struct {
	Content string `json:"content"`
}

type sayHelloPayload struct {
	Content string `json:"content"`
}

type chatTTSTextPayload struct {
	Start   bool   `json:"start"`
	Content string `json:"content"`
	End     bool   `json:"end"`
}

type chatRAGTextPayload struct {
	ExternalRAG string `json:"external_rag"`
}

// ClientEvent 描述火山实时语音网关返回的一条事件。
type ClientEvent struct {
	ID        int32
	SessionID string
	ConnectID string
	ErrorCode uint32
	Payload   []byte
}

// StartOptions 描述启动一轮实时语音会话时所需的会话配置。
type StartOptions struct {
	SessionID       string
	DialogID        string
	BotName         string
	SystemRole      string
	SpeakingStyle   string
	CharacterPrompt string
	LocationCity    string
	InputMode       string
	RecvTimeout     int
	Speaker         string
}

// Client 封装火山端到端实时语音 WebSocket 会话。
type Client struct {
	cfg       appconfig.VolcRealtimeDialogConfig
	conn      *websocket.Conn
	sessionID string
	events    chan ClientEvent
	writeMu   sync.Mutex
	closeOnce sync.Once
}

// NewClient 根据项目配置创建实时语音客户端。
func NewClient(cfg appconfig.VolcRealtimeDialogConfig) (*Client, error) {
	normalized := cfg
	if err := validateRealtimeDialogConfig(&normalized); err != nil {
		return nil, err
	}
	if strings.TrimSpace(normalized.BaseURL) == "" {
		normalized.BaseURL = "wss://openspeech.bytedance.com/api/v3/realtime/dialogue"
	}
	normalized.AppKey = defaultRealtimeDialogAppKey
	if strings.TrimSpace(normalized.ResourceID) == "" {
		normalized.ResourceID = "volc.speech.dialog"
	}
	if strings.TrimSpace(normalized.InputMode) == "" {
		normalized.InputMode = "push_to_talk"
	}
	if strings.TrimSpace(normalized.AudioFormat) == "" {
		normalized.AudioFormat = "pcm"
	}
	if normalized.SampleRate <= 0 {
		normalized.SampleRate = 16000
	}
	if strings.TrimSpace(normalized.TTSFormat) == "" {
		normalized.TTSFormat = "pcm_s16le"
	}
	if normalized.TTSSampleRate <= 0 {
		normalized.TTSSampleRate = 24000
	}
	if strings.TrimSpace(normalized.Speaker) == "" {
		normalized.Speaker = "zh_female_vv_jupiter_bigtts"
	}
	if strings.TrimSpace(normalized.BotName) == "" {
		normalized.BotName = "Ariu"
	}
	if strings.TrimSpace(normalized.SystemRole) == "" {
		normalized.SystemRole = "你是一位专业、耐心、会逐题推进的中文技术面试官。"
	}
	if strings.TrimSpace(normalized.SpeakingStyle) == "" {
		normalized.SpeakingStyle = "请用口语化中文进行简洁播报，每次只问一个问题。"
	}
	if normalized.RecvTimeout <= 0 {
		normalized.RecvTimeout = 120
	}

	return &Client{
		cfg:    normalized,
		events: make(chan ClientEvent, 128),
	}, nil
}

// Events 返回客户端内部读取到的实时语音服务端事件流。
func (c *Client) Events() <-chan ClientEvent {
	return c.events
}

// validateRealtimeDialogConfig 校验实时语音面试所需配置，并补齐固定协议常量。
func validateRealtimeDialogConfig(cfg *appconfig.VolcRealtimeDialogConfig) error {
	if cfg == nil {
		return fmt.Errorf("realtime dialog config is nil")
	}
	if !cfg.Enabled {
		return fmt.Errorf("realtime dialog is disabled; set volcengine.realtime.enabled=true")
	}
	if strings.TrimSpace(cfg.AppID) == "" {
		return fmt.Errorf("realtime dialog config missing app_id; set volcengine.realtime.app_id")
	}
	if strings.TrimSpace(cfg.AccessToken) == "" {
		return fmt.Errorf("realtime dialog config missing access_token; set volcengine.realtime.access_token")
	}
	cfg.AppKey = defaultRealtimeDialogAppKey
	return nil
}

// Start 完成建连、鉴权和 StartSession，并返回服务端生成的 dialog_id。
func (c *Client) Start(ctx context.Context, options StartOptions) (string, error) {
	dialer := websocket.Dialer{}
	headers := http.Header{
		"X-Api-Resource-Id": []string{c.cfg.ResourceID},
		"X-Api-Access-Key":  []string{c.cfg.AccessToken},
		"X-Api-App-Key":     []string{c.cfg.AppKey},
		"X-Api-App-ID":      []string{c.cfg.AppID},
		"X-Api-Connect-Id":  []string{uuid.NewString()},
	}

	conn, _, err := dialer.DialContext(ctx, c.cfg.BaseURL, headers)
	if err != nil {
		return "", fmt.Errorf("dial realtime dialog websocket failed: %w", err)
	}
	c.conn = conn
	c.sessionID = strings.TrimSpace(options.SessionID)
	if c.sessionID == "" {
		c.sessionID = uuid.NewString()
	}

	if err := c.sendFullJSON(EventStartConnection, "", map[string]interface{}{}); err != nil {
		return "", err
	}
	started, err := c.readHandshakeMessage(EventConnectionStarted)
	if err != nil {
		return "", err
	}
	_ = started

	payload := startSessionPayload{
		ASR: asrPayload{
			AudioInfo: audioInfo{
				Format:     c.cfg.AudioFormat,
				SampleRate: c.cfg.SampleRate,
				Channel:    1,
			},
			Extra: map[string]interface{}{
				"end_smooth_window_ms": 1500,
			},
		},
		TTS: ttsPayload{
			Speaker: firstNonEmptyRealtimeString(options.Speaker, c.cfg.Speaker),
			AudioConfig: audioConfig{
				Channel:    1,
				Format:     c.cfg.TTSFormat,
				SampleRate: c.cfg.TTSSampleRate,
			},
		},
		Dialog: dialogPayload{
			DialogID:          strings.TrimSpace(options.DialogID),
			BotName:           firstNonEmptyRealtimeString(options.BotName, c.cfg.BotName),
			SystemRole:        firstNonEmptyRealtimeString(options.SystemRole, c.cfg.SystemRole),
			SpeakingStyle:     firstNonEmptyRealtimeString(options.SpeakingStyle, c.cfg.SpeakingStyle),
			CharacterManifest: strings.TrimSpace(firstNonEmptyRealtimeString(options.CharacterPrompt, c.cfg.CharacterPrompt)),
			Location: &locationInfo{
				City: firstNonEmptyRealtimeString(options.LocationCity, c.cfg.LocationCity),
			},
			Extra: map[string]interface{}{
				"strict_audit":           false,
				"input_mod":              firstNonEmptyRealtimeString(options.InputMode, c.cfg.InputMode),
				"recv_timeout":           firstNonEmptyRealtimeInt(options.RecvTimeout, c.cfg.RecvTimeout),
				"enable_user_query_exit": false,
				"model":                  "1.2.1.1",
			},
		},
	}
	if payload.Dialog.CharacterManifest == "" {
		payload.Dialog.CharacterManifest = ""
	}

	if err := c.sendFullJSON(EventStartSession, c.sessionID, payload); err != nil {
		return "", err
	}
	msg, err := c.readHandshakeMessage(EventSessionStarted)
	if err != nil {
		return "", err
	}

	dialogID := ""
	if len(msg.Payload) > 0 {
		var response struct {
			DialogID string `json:"dialog_id"`
		}
		if err := json.Unmarshal(msg.Payload, &response); err == nil {
			dialogID = strings.TrimSpace(response.DialogID)
		}
	}

	go c.readLoop()
	return dialogID, nil
}

// SendAudio 向实时语音会话发送一段 PCM 音频块。
func (c *Client) SendAudio(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	frame, err := marshalProtocolMessage(protocolMessage{
		MessageType:   messageTypeAudioClient,
		Flag:          messageFlagWithEvent,
		Serialization: serializationRaw,
		Event:         EventTaskRequest,
		SessionID:     c.sessionID,
		Payload:       chunk,
	})
	if err != nil {
		return fmt.Errorf("marshal realtime audio chunk failed: %w", err)
	}
	return c.writeBinary(frame)
}

// SendTextQuery 向实时语音会话发送一段文本输入，让模型继续当前多轮面试。
func (c *Client) SendTextQuery(text string) error {
	payload := chatTextQueryPayload{
		Content: strings.TrimSpace(text),
	}
	return c.sendFullJSON(EventChatTextQuery, c.sessionID, payload)
}

// SendSayHello 主动唤起模型先播报一段内容，适合在实时语音会话刚开始时出第一题。
func (c *Client) SendSayHello(text string) error {
	payload := sayHelloPayload{
		Content: strings.TrimSpace(text),
	}
	return c.sendFullJSON(EventSayHello, c.sessionID, payload)
}

// SendEndASR 在 push_to_talk 模式下显式结束当前一轮用户语音输入。
func (c *Client) SendEndASR() error {
	return c.sendFullJSON(EventEndASR, c.sessionID, map[string]interface{}{})
}

// SendChatTTSText 向实时模型发送安抚话术（事件500）。
// 用于RAG检索期间播放安抚语音，避免用户等待沉默。
func (c *Client) SendChatTTSText(content string) error {
	payload := chatTTSTextPayload{
		Start:   true,
		Content: strings.TrimSpace(content),
		End:     true,
	}
	return c.sendFullJSON(EventChatTTSText, c.sessionID, payload)
}

// SendChatRAGText 向实时模型注入外部RAG数据（事件502）。
// externalRAG 格式: [{"title":"...","content":"..."}]
// 模型会自动总结和口语化改写RAG内容后输出音频。
func (c *Client) SendChatRAGText(externalRAG string) error {
	payload := chatRAGTextPayload{
		ExternalRAG: externalRAG,
	}
	return c.sendFullJSON(EventChatRAGText, c.sessionID, payload)
}

// Close 结束当前实时语音会话并释放底层连接。
func (c *Client) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		if c.conn == nil {
			close(c.events)
			return
		}

		_ = c.sendFullJSON(EventFinishSession, c.sessionID, map[string]interface{}{})
		_ = c.sendFullJSON(EventFinishConnection, "", map[string]interface{}{})
		closeErr = c.conn.Close()
		close(c.events)
	})
	return closeErr
}

// readLoop 持续消费服务端事件并转发给上层会话编排器。
func (c *Client) readLoop() {
	defer c.Close()

	for {
		_, frame, err := c.conn.ReadMessage()
		if err != nil {
			applogger.Warn("realtime websocket read message failed", zap.Error(err))
			return
		}

		msg, err := unmarshalProtocolMessage(frame)
		if err != nil {
			applogger.Warn("realtime websocket unmarshal failed",
				zap.Error(err),
				zap.Int("frame_length", len(frame)),
			)
			continue
		}
		applogger.Info("realtime websocket event received",
			zap.Int32("event", msg.Event),
			zap.String("session_id", msg.SessionID),
			zap.String("connect_id", msg.ConnectID),
			zap.Int32("sequence", msg.Sequence),
			zap.Int("payload_size", len(msg.Payload)),
			zap.Uint32("error_code", msg.ErrorCode),
			zap.Uint8("message_type", msg.MessageType),
			zap.Uint8("flag", msg.Flag),
		)

		c.events <- ClientEvent{
			ID:        msg.Event,
			SessionID: msg.SessionID,
			ConnectID: msg.ConnectID,
			ErrorCode: msg.ErrorCode,
			Payload:   append([]byte(nil), msg.Payload...),
		}
	}
}

// readHandshakeMessage 在 readLoop 启动前同步读取一条握手响应。
func (c *Client) readHandshakeMessage(expectedEvent int32) (*protocolMessage, error) {
	for {
		_, frame, err := c.conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("read realtime handshake message failed: %w", err)
		}
		msg, err := unmarshalProtocolMessage(frame)
		if err != nil {
			return nil, err
		}
		if msg.MessageType == messageTypeError {
			return nil, fmt.Errorf("realtime dialog returned error code=%d body=%s", msg.ErrorCode, string(msg.Payload))
		}
		if msg.Event == expectedEvent {
			return msg, nil
		}
		if msg.Event == EventSessionFailed {
			return nil, fmt.Errorf("realtime session failed: %s", strings.TrimSpace(string(msg.Payload)))
		}
	}
}

// sendFullJSON 发送一条带 JSON payload 的完整客户端事件。
func (c *Client) sendFullJSON(event int32, sessionID string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal realtime json payload failed: %w", err)
	}
	frame, err := marshalProtocolMessage(protocolMessage{
		MessageType:   messageTypeFullClient,
		Flag:          messageFlagWithEvent,
		Serialization: serializationJSON,
		Event:         event,
		SessionID:     sessionID,
		Payload:       body,
	})
	if err != nil {
		return fmt.Errorf("marshal realtime json frame failed: %w", err)
	}
	return c.writeBinary(frame)
}

// writeBinary 串行写入底层 WebSocket，避免文本和音频事件交叉污染。
func (c *Client) writeBinary(frame []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("realtime websocket is not connected")
	}
	if err := c.conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		return fmt.Errorf("write realtime websocket frame failed: %w", err)
	}
	return nil
}

// firstNonEmptyRealtimeString 返回第一个非空字符串。
func firstNonEmptyRealtimeString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// firstNonEmptyRealtimeInt 返回第一个大于零的整数。
func firstNonEmptyRealtimeInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
