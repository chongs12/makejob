package data

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"makejob/app/realtime/internal/biz"
	"makejob/app/realtime/internal/conf"
)

// ========== 二进制帧协议常量 ==========

const (
	// 帧头固定 12 字节
	frameHeaderSize = 12

	// 消息类型
	msgTypeFullClientRequest = 0x01 // 客户端请求
	msgTypeServerResponse    = 0x09 // 服务端响应
	msgTypeControl           = 100  // 控制事件
	msgTypeASR               = 501  // ASR 语音识别
	msgTypeChat              = 502  // Chat 对话
	msgTypeTTS               = 503  // TTS 语音合成

	// 序列化方式
	serializationJSON    = 1
	serializationProto   = 2
	serializationFlatBuf = 3

	// 压缩方式
	compressionNone = 0
	compressionGzip = 1

	// Flag 位
	flagPositiveSequence = 0x01

	// FrameHeader 中 byte0 的值：version=1, header_size=12 → (1<<4)|12 = 0x1C
	frameHeaderByte0 = 0x1C
)

// frameHeader 二进制帧头结构
//
// 布局（12 字节）:
//
//	[0]  version(4bit) | header_size(4bit) → 0x1C
//	[1]  message_type（完整字节）
//	[2]  flags
//	[3]  serialization(4bit) | compression(4bit)
//	[4-7]  reserved
//	[8-10] payload_size（24bit 大端序）
//	[11]   reserved
type frameHeader struct {
	MessageType   uint8
	Flags         uint8
	Serialization uint8
	Compression   uint8
	PayloadSize   uint32
}

// encode 将帧头编码为 12 字节
func (h *frameHeader) encode() []byte {
	buf := make([]byte, frameHeaderSize)
	buf[0] = frameHeaderByte0
	buf[1] = h.MessageType
	buf[2] = h.Flags
	buf[3] = (h.Serialization << 4) | (h.Compression & 0x0F)
	// bytes 4-7 为 reserved，保持 0
	// payload_size 写入 bytes 8-10（24bit 大端序）
	payloadSizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(payloadSizeBytes, h.PayloadSize)
	copy(buf[8:11], payloadSizeBytes[1:4])
	return buf
}

// decodeFrameHeader 从 12 字节解码帧头
func decodeFrameHeader(data []byte) (*frameHeader, error) {
	if len(data) < frameHeaderSize {
		return nil, fmt.Errorf("帧头数据不足: 需要 %d 字节，实际 %d 字节", frameHeaderSize, len(data))
	}
	h := &frameHeader{
		MessageType:   data[1],
		Flags:         data[2],
		Serialization: (data[3] >> 4) & 0x0F,
		Compression:   data[3] & 0x0F,
	}
	// 从 bytes 8-10 读取 24bit payload size
	payloadSizeBytes := []byte{0, data[8], data[9], data[10]}
	h.PayloadSize = binary.BigEndian.Uint32(payloadSizeBytes)
	return h, nil
}

// volcEvent 火山引擎服务端事件
type volcEvent struct {
	Header  *frameHeader
	Payload []byte
}

// GetMessageType 返回事件消息类型
func (e *volcEvent) GetMessageType() int {
	return int(e.Header.MessageType)
}

// GetPayload 返回事件载荷
func (e *volcEvent) GetPayload() []byte {
	return e.Payload
}

// ========== VolcengineClient 实现 ==========

// volcengineClient 火山引擎 WebSocket 客户端
type volcengineClient struct {
	conn  *websocket.Conn
	appID string
	token string
	wsURL string
}

// NewVolcEngineFactory 创建火山引擎连接工厂函数
func NewVolcEngineFactory(cfg *conf.Volcengine) biz.VolcEngineFactory {
	return func(ctx context.Context) (biz.VolcEngineConn, error) {
		client := &volcengineClient{
			appID: cfg.AppID,
			token: cfg.Token,
			wsURL: cfg.WSUrl,
		}
		if err := client.connect(ctx); err != nil {
			return nil, err
		}
		return client, nil
	}
}

// connect 建立与火山引擎的 WebSocket 连接
func (c *volcengineClient) connect(ctx context.Context) error {
	wsURL := c.wsURL
	if wsURL == "" {
		// 未配置 ws_url 时，通过 API 获取连接地址
		endpoint, err := c.fetchEndpoint(ctx)
		if err != nil {
			return fmt.Errorf("获取火山引擎 WebSocket 端点失败: %w", err)
		}
		wsURL = endpoint
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	header := http.Header{}
	header.Set("X-Api-App-Key", c.appID)
	header.Set("X-Api-Access-Key", c.token)
	header.Set("X-Api-Resource-Id", "volc.bigasr.sauc.duration")
	header.Set("X-Api-Connect-Id", fmt.Sprintf("rt-%d", time.Now().UnixNano()))

	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("火山引擎 WebSocket 连接失败: %w", err)
	}
	c.conn = conn

	// 发送初始化请求
	return c.sendInitRequest()
}

// fetchEndpoint 通过 API 获取火山引擎 WebSocket 端点
func (c *volcengineClient) fetchEndpoint(ctx context.Context) (string, error) {
	apiURL := "https://openspeech.bytedance.com/api/v1/ws/sauc/bigmodel"

	reqBody := map[string]interface{}{
		"app_id":     c.appID,
		"token":      c.token,
		"resource_id": "volc.bigasr.sauc.duration",
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求火山引擎 API 失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code      int    `json:"code"`
		WsURL     string `json:"ws_url"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 API 响应失败: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("火山引擎 API 返回错误: code=%d, message=%s", result.Code, result.Message)
	}
	if result.WsURL == "" {
		return "", fmt.Errorf("火山引擎 API 未返回 ws_url")
	}
	return result.WsURL, nil
}

// sendInitRequest 发送初始化配置请求（全双工模式 + JSON 序列化）
func (c *volcengineClient) sendInitRequest() error {
	initPayload := map[string]interface{}{
		"appkey":    c.appID,
		"cluster":   "volc.bigasr.sauc.duration",
		"request_id": fmt.Sprintf("req-%d", time.Now().UnixNano()),
		"language":   "zh-CN",
	}
	payload, err := json.Marshal(initPayload)
	if err != nil {
		return fmt.Errorf("序列化初始化请求失败: %w", err)
	}

	hdr := &frameHeader{
		MessageType:   msgTypeFullClientRequest,
		Flags:         flagPositiveSequence,
		Serialization: serializationJSON,
		Compression:   compressionNone,
		PayloadSize:   uint32(len(payload)),
	}

	msg := append(hdr.encode(), payload...)
	return c.conn.WriteMessage(websocket.BinaryMessage, msg)
}

// SendAudio 发送音频数据到火山引擎
func (c *volcengineClient) SendAudio(data []byte) error {
	hdr := &frameHeader{
		MessageType:   msgTypeFullClientRequest,
		Flags:         flagPositiveSequence,
		Serialization: serializationJSON,
		Compression:   compressionNone,
		PayloadSize:   uint32(len(data)),
	}

	msg := append(hdr.encode(), data...)
	return c.conn.WriteMessage(websocket.BinaryMessage, msg)
}

// ReadEvent 读取火山引擎服务端事件
func (c *volcengineClient) ReadEvent() (*biz.VolcEvent, error) {
	msgType, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("读取火山引擎消息失败: %w", err)
	}

	if msgType != websocket.BinaryMessage {
		return nil, fmt.Errorf("预期二进制消息，收到类型: %d", msgType)
	}

	if len(data) < frameHeaderSize {
		return nil, fmt.Errorf("消息长度不足: 需要至少 %d 字节", frameHeaderSize)
	}

	hdr, err := decodeFrameHeader(data[:frameHeaderSize])
	if err != nil {
		return nil, fmt.Errorf("解码帧头失败: %w", err)
	}

	payload := data[frameHeaderSize:]
	if hdr.Compression == compressionGzip {
		// 预留 gzip 解压逻辑
		return nil, fmt.Errorf("暂不支持 gzip 压缩")
	}

	return &biz.VolcEvent{
		Type:    int(hdr.MessageType),
		Payload: payload,
	}, nil
}

// InjectContext 注入上下文文本到火山引擎会话
func (c *volcengineClient) InjectContext(text string) error {
	payload := map[string]interface{}{
		"event":   "inject_context",
		"context": text,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化上下文注入请求失败: %w", err)
	}

	hdr := &frameHeader{
		MessageType:   msgTypeControl,
		Flags:         0,
		Serialization: serializationJSON,
		Compression:   compressionNone,
		PayloadSize:   uint32(len(data)),
	}

	msg := append(hdr.encode(), data...)
	return c.conn.WriteMessage(websocket.BinaryMessage, msg)
}

// Close 关闭 WebSocket 连接
func (c *volcengineClient) Close() error {
	if c.conn != nil {
		// 发送关闭帧
		_ = c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		return c.conn.Close()
	}
	return nil
}

