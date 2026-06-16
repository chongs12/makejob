package data

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"makejob/app/realtime/internal/biz"
	"makejob/app/realtime/internal/conf"
)

// 火山实时语音事件常量（对齐单体 realtime/volcengine/client.go）

const (
	defaultRealtimeDialogAppKey = "PlgvMymc7f3tQnJ6"

	EventStartConnection  int32 = 1
	EventFinishConnection int32 = 2
	EventStartSession     int32 = 100
	EventFinishSession    int32 = 102
	EventTaskRequest      int32 = 200
	EventSayHello         int32 = 300
	EventEndASR           int32 = 400
	EventChatTextQuery    int32 = 501
	EventChatTTSText      int32 = 500
	EventChatRAGText      int32 = 502

	EventConnectionStarted int32 = 50
	EventSessionStarted    int32 = 150
	EventSessionFinished   int32 = 152
	EventSessionFailed     int32 = 153
	EventTTSSentenceStart  int32 = 350
	EventTTSSentenceEnd    int32 = 351
	EventTTSResponse       int32 = 352
	EventTTSEnded          int32 = 359
	EventASRInfo           int32 = 450
	EventASRResponse       int32 = 451
	EventASREnded          int32 = 459
	EventChatResponse      int32 = 550
	EventChatTextQueryConfirmed int32 = 553
	EventChatEnded         int32 = 559
)

// startSessionPayload 启动会话时的配置载荷
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

// volcengineClient 封装火山端到端实时语音 WebSocket 会话（对齐单体 Client）
type volcengineClient struct {
	cfg       *conf.Volcengine
	conn      *websocket.Conn
	sessionID string
	dialogID  string
	events    chan biz.VolcEvent
	writeMu   sync.Mutex
	closeOnce sync.Once
}

// NewVolcEngineFactory 创建火山引擎连接工厂函数（对齐单体 NewClient + Start 流程）
func NewVolcEngineFactory(cfg *conf.Volcengine) biz.VolcEngineFactory {
	return func(ctx context.Context) (biz.VolcEngineConn, error) {
		client, err := newVolcengineClient(cfg)
		if err != nil {
			return nil, err
		}
		if err := client.connect(ctx); err != nil {
			return nil, err
		}
		return client, nil
	}
}

// NewVolcEngineSessionFactory 创建火山引擎会话工厂函数（支持 StartOptions）
func NewVolcEngineSessionFactory(cfg *conf.Volcengine) biz.VolcEngineSessionFactory {
	return func(ctx context.Context, opts biz.VolcStartOptions) (biz.VolcEngineSessionConn, error) {
		client, err := newVolcengineClient(cfg)
		if err != nil {
			return nil, err
		}
		dialogID, err := client.Start(ctx, opts)
		if err != nil {
			return nil, err
		}
		client.dialogID = dialogID
		return client, nil
	}
}

// VolcEngineSessionConn 火山引擎会话连接接口（扩展 VolcEngineConn）
type VolcEngineSessionConn interface {
	biz.VolcEngineConn
	SendTextQuery(text string) error
	SendSayHello(text string) error
	SendEndASR() error
	SendChatTTSText(content string) error
	SendChatRAGText(externalRAG string) error
	DialogID() string
	Events() <-chan biz.VolcEvent
}

// newVolcengineClient 创建火山引擎客户端实例
func newVolcengineClient(cfg *conf.Volcengine) (*volcengineClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("volcengine config is nil")
	}
	if !cfg.Enabled {
		return nil, fmt.Errorf("realtime dialog is disabled; set volcengine.enabled=true")
	}
	if strings.TrimSpace(cfg.AppID) == "" {
		return nil, fmt.Errorf("realtime dialog config missing app_id")
	}
	if strings.TrimSpace(cfg.AccessToken) == "" {
		return nil, fmt.Errorf("realtime dialog config missing access_token")
	}

	return &volcengineClient{
		cfg:    cfg,
		events: make(chan biz.VolcEvent, 128),
	}, nil
}

// connect 建立 WebSocket 连接并完成握手（简化版，兼容旧接口）
func (c *volcengineClient) connect(ctx context.Context) error {
	return c.StartWithDefaults(ctx)
}

// StartWithDefaults 使用默认配置启动会话
func (c *volcengineClient) StartWithDefaults(ctx context.Context) error {
	_, err := c.Start(ctx, biz.VolcStartOptions{})
	return err
}

// Start 完成建连、鉴权和 StartSession，并返回服务端生成的 dialog_id（对齐单体 Client.Start）
func (c *volcengineClient) Start(ctx context.Context, opts biz.VolcStartOptions) (string, error) {
	baseURL := strings.TrimSpace(c.cfg.BaseURL)
	if baseURL == "" {
		baseURL = "wss://openspeech.bytedance.com/api/v3/realtime/dialogue"
	}
	appKey := defaultRealtimeDialogAppKey
	resourceID := strings.TrimSpace(c.cfg.ResourceID)
	if resourceID == "" {
		resourceID = "volc.speech.dialog"
	}

	dialer := websocket.Dialer{}
	headers := http.Header{
		"X-Api-Resource-Id": []string{resourceID},
		"X-Api-Access-Key":  []string{c.cfg.AccessToken},
		"X-Api-App-Key":     []string{appKey},
		"X-Api-App-ID":      []string{c.cfg.AppID},
		"X-Api-Connect-Id":  []string{uuid.NewString()},
	}

	conn, _, err := dialer.DialContext(ctx, baseURL, headers)
	if err != nil {
		return "", fmt.Errorf("dial realtime dialog websocket failed: %w", err)
	}
	c.conn = conn
	c.sessionID = strings.TrimSpace(opts.SessionID)
	if c.sessionID == "" {
		c.sessionID = uuid.NewString()
	}

	// 发送 StartConnection
	if err := c.sendFullJSON(EventStartConnection, "", map[string]interface{}{}); err != nil {
		return "", err
	}
	if _, err := c.readHandshakeMessage(EventConnectionStarted); err != nil {
		return "", err
	}

	// 构建 StartSession 载荷
	inputMode := firstNonEmpty(opts.InputMode, c.cfg.InputMode, "push_to_talk")
	audioFormat := firstNonEmpty(c.cfg.AudioFormat, "pcm")
	sampleRate := firstPositive(c.cfg.SampleRate, 16000)
	ttsFormat := firstNonEmpty(c.cfg.TTSFormat, "pcm_s16le")
	ttsSampleRate := firstPositive(c.cfg.TTSSampleRate, 24000)
	speaker := firstNonEmpty(opts.Speaker, c.cfg.Speaker, "zh_female_vv_jupiter_bigtts")
	botName := firstNonEmpty(opts.BotName, c.cfg.BotName, "Ariu")
	systemRole := firstNonEmpty(opts.SystemRole, c.cfg.SystemRole, "你是一位专业、耐心、会逐题推进的中文技术面试官。")
	speakingStyle := firstNonEmpty(opts.SpeakingStyle, c.cfg.SpeakingStyle, "请用口语化中文进行简洁播报，每次只问一个问题。")
	characterPrompt := firstNonEmpty(opts.CharacterPrompt, c.cfg.CharacterPrompt)
	locationCity := firstNonEmpty(opts.LocationCity, c.cfg.LocationCity)
	recvTimeout := firstPositive(opts.RecvTimeout, c.cfg.RecvTimeout, 120)

	payload := startSessionPayload{
		ASR: asrPayload{
			AudioInfo: audioInfo{
				Format:     audioFormat,
				SampleRate: sampleRate,
				Channel:    1,
			},
			Extra: map[string]interface{}{
				"end_smooth_window_ms": 1500,
			},
		},
		TTS: ttsPayload{
			Speaker: speaker,
			AudioConfig: audioConfig{
				Channel:    1,
				Format:     ttsFormat,
				SampleRate: ttsSampleRate,
			},
		},
		Dialog: dialogPayload{
			DialogID:          strings.TrimSpace(opts.DialogID),
			BotName:           botName,
			SystemRole:        systemRole,
			SpeakingStyle:     speakingStyle,
			CharacterManifest: strings.TrimSpace(characterPrompt),
			Location: &locationInfo{
				City: locationCity,
			},
			Extra: map[string]interface{}{
				"strict_audit":           false,
				"input_mod":              inputMode,
				"recv_timeout":           recvTimeout,
				"enable_user_query_exit": false,
				"model":                  "1.2.1.1",
			},
		},
	}

	// 发送 StartSession
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

// Events 返回事件流通道
func (c *volcengineClient) Events() <-chan biz.VolcEvent {
	return c.events
}

// DialogID 返回服务端分配的 dialog_id
func (c *volcengineClient) DialogID() string {
	return c.dialogID
}

// SendAudio 向实时语音会话发送一段 PCM 音频块。
func (c *volcengineClient) SendAudio(chunk []byte) error {
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

// SendTextQuery 向实时语音会话发送一段文本输入。
func (c *volcengineClient) SendTextQuery(text string) error {
	payload := chatTextQueryPayload{
		Content: strings.TrimSpace(text),
	}
	return c.sendFullJSON(EventChatTextQuery, c.sessionID, payload)
}

// SendSayHello 主动唤起模型先播报一段内容。
func (c *volcengineClient) SendSayHello(text string) error {
	payload := sayHelloPayload{
		Content: strings.TrimSpace(text),
	}
	return c.sendFullJSON(EventSayHello, c.sessionID, payload)
}

// SendEndASR 在 push_to_talk 模式下显式结束当前一轮用户语音输入。
func (c *volcengineClient) SendEndASR() error {
	return c.sendFullJSON(EventEndASR, c.sessionID, map[string]interface{}{})
}

// SendChatTTSText 向实时模型发送安抚话术（事件500）。
func (c *volcengineClient) SendChatTTSText(content string) error {
	payload := chatTTSTextPayload{
		Start:   true,
		Content: strings.TrimSpace(content),
		End:     true,
	}
	return c.sendFullJSON(EventChatTTSText, c.sessionID, payload)
}

// SendChatRAGText 向实时模型注入外部RAG数据（事件502）。
func (c *volcengineClient) SendChatRAGText(externalRAG string) error {
	payload := chatRAGTextPayload{
		ExternalRAG: externalRAG,
	}
	return c.sendFullJSON(EventChatRAGText, c.sessionID, payload)
}

// InjectContext 注入上下文文本（兼容旧接口，内部使用 SendChatRAGText）
func (c *volcengineClient) InjectContext(text string) error {
	return c.SendChatRAGText(text)
}

// ReadEvent 读取火山引擎服务端事件（兼容旧接口，从 events 通道读取）
func (c *volcengineClient) ReadEvent() (*biz.VolcEvent, error) {
	event, ok := <-c.events
	if !ok {
		return nil, fmt.Errorf("realtime event channel closed")
	}
	return &event, nil
}

// Close 结束当前实时语音会话并释放底层连接。
func (c *volcengineClient) Close() error {
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

// readLoop 持续消费服务端事件。
func (c *volcengineClient) readLoop() {
	defer c.Close()

	for {
		_, frame, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		msg, err := unmarshalProtocolMessage(frame)
		if err != nil {
			continue
		}

		c.events <- biz.VolcEvent{
			Type:    int(msg.Event),
			Payload: append([]byte(nil), msg.Payload...),
		}
	}
}

// readHandshakeMessage 在 readLoop 启动前同步读取一条握手响应。
func (c *volcengineClient) readHandshakeMessage(expectedEvent int32) (*protocolMessage, error) {
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
func (c *volcengineClient) sendFullJSON(event int32, sessionID string, payload interface{}) error {
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

// writeBinary 串行写入底层 WebSocket。
func (c *volcengineClient) writeBinary(frame []byte) error {
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

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// firstPositive 返回第一个大于零的整数。
func firstPositive(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}
