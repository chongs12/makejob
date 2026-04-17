package volcengine

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"makejob-backend/internal/asr"
	appconfig "makejob-backend/internal/config"
)

const (
	defaultASRURL            = "wss://openspeech.bytedance.com/api/v2/asr"
	defaultASRCluster        = "volcengine_streaming_common"
	defaultASRFormat         = "wav"
	defaultASRSampleRate     = 16000
	defaultASRLanguage       = "zh-CN"
	defaultChunkSize         = 3200
	protocolVersion          = 0x1
	headerSize               = 0x1
	messageTypeFullClient    = 0x1
	messageTypeAudioOnly     = 0x2
	messageTypeFullServer    = 0x9
	messageTypeErrorResponse = 0xF
	flagPositiveSequence     = 0x1
	flagNegativeSequence     = 0x2
	serializationNone        = 0x0
	serializationJSON        = 0x1
	compressionGzip          = 0x1
)

// Provider 封装火山云 ASR WebSocket 协议实现。
type Provider struct {
	baseURL     string
	appID       string
	accessToken string
	cluster     string
	resourceID  string
	workflow    string
	audioFormat string
	sampleRate  int
	language    string
	dialer      *websocket.Dialer
}

// sessionConfig 描述一次识别会话的配置。
type sessionConfig struct {
	language   string
	audioFmt   string
	sampleRate int
	requestID  string
}

// streamSession 管理火山云流式识别会话。
type streamSession struct {
	conn       *websocket.Conn
	resultChan chan asr.StreamResult
	closeOnce  sync.Once
	writeMu    sync.Mutex
	closed     chan struct{}
	readErr    error
}

// requestPayload 描述火山云 ASR 初始请求体。
type requestPayload struct {
	App struct {
		AppID      string `json:"appid"`
		Cluster    string `json:"cluster"`
		Token      string `json:"token"`
		ResourceID string `json:"resource_id,omitempty"`
	} `json:"app"`
	User struct {
		UID string `json:"uid"`
	} `json:"user"`
	Request struct {
		ReqID      string `json:"reqid"`
		Workflow   string `json:"workflow,omitempty"`
		Sequence   int    `json:"sequence"`
		Nbest      int    `json:"nbest,omitempty"`
		ShowUtter  bool   `json:"show_utterances,omitempty"`
		ResultType string `json:"result_type,omitempty"`
	} `json:"request"`
	Audio struct {
		Format     string `json:"format"`
		SampleRate int    `json:"sample_rate"`
		Language   string `json:"language,omitempty"`
	} `json:"audio"`
}

// responsePayload 描述火山云 ASR 返回结构。
type responsePayload struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	ReqID    string `json:"reqid"`
	Sequence int    `json:"sequence"`
	Result   []struct {
		Text       string  `json:"text"`
		Confidence float64 `json:"confidence"`
		Utterances []struct {
			Text string `json:"text"`
		} `json:"utterances"`
	} `json:"result"`
}

// NewProvider 根据火山云配置创建真实 ASR Provider。
func NewProvider(cfg appconfig.VolcengineConfig) (*Provider, error) {
	baseURL := strings.TrimSpace(cfg.ASR.BaseURL)
	if baseURL == "" {
		baseURL = defaultASRURL
	}
	appID := strings.TrimSpace(cfg.ASR.AppID)
	accessToken := strings.TrimSpace(cfg.ASR.AccessToken)
	if appID == "" || accessToken == "" {
		return nil, fmt.Errorf("volcengine asr config missing app_id or access_token")
	}

	cluster := strings.TrimSpace(cfg.ASR.Cluster)
	if cluster == "" {
		cluster = defaultASRCluster
	}
	audioFormat := strings.TrimSpace(cfg.ASR.AudioFormat)
	if audioFormat == "" {
		audioFormat = defaultASRFormat
	}
	sampleRate := cfg.ASR.SampleRate
	if sampleRate <= 0 {
		sampleRate = defaultASRSampleRate
	}
	language := strings.TrimSpace(cfg.ASR.Language)
	if language == "" {
		language = defaultASRLanguage
	}

	return &Provider{
		baseURL:     baseURL,
		appID:       appID,
		accessToken: accessToken,
		cluster:     cluster,
		resourceID:  strings.TrimSpace(cfg.ASR.ResourceID),
		workflow:    strings.TrimSpace(cfg.ASR.Workflow),
		audioFormat: audioFormat,
		sampleRate:  sampleRate,
		language:    language,
		dialer:      websocket.DefaultDialer,
	}, nil
}

// Recognize 发送完整音频并等待最终识别结果。
func (p *Provider) Recognize(ctx context.Context, req asr.RecognizeRequest) (asr.RecognizeResult, error) {
	cfg := p.buildSessionConfig(req.Engine, req.Language)
	conn, err := p.openConnection(ctx, cfg)
	if err != nil {
		return asr.RecognizeResult{}, err
	}
	defer conn.Close()

	for offset := 0; offset < len(req.AudioData); offset += defaultChunkSize {
		end := offset + defaultChunkSize
		if end > len(req.AudioData) {
			end = len(req.AudioData)
		}
		if err := writeConnFrame(conn, buildAudioFrame(req.AudioData[offset:end], false)); err != nil {
			return asr.RecognizeResult{}, err
		}
	}
	if err := writeConnFrame(conn, buildAudioFrame(nil, true)); err != nil {
		return asr.RecognizeResult{}, err
	}

	for {
		select {
		case <-ctx.Done():
			return asr.RecognizeResult{}, ctx.Err()
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			return asr.RecognizeResult{}, fmt.Errorf("read volcengine asr response: %w", err)
		}

		payload, isFinal, err := decodeServerFrame(message)
		if err != nil {
			return asr.RecognizeResult{}, err
		}
		if payload.Code != 0 {
			return asr.RecognizeResult{}, fmt.Errorf("volcengine asr failed: code=%d message=%s", payload.Code, strings.TrimSpace(payload.Message))
		}
		if !isFinal {
			continue
		}

		return asr.RecognizeResult{
			Text:       extractResultText(payload),
			Confidence: extractConfidence(payload),
			Duration:   estimateDurationSeconds(req.AudioData, req.SampleRate),
			Language:   chooseLanguage(req.Language, p.language),
		}, nil
	}
}

// StartStream 创建真实火山云流式识别会话。
func (p *Provider) StartStream(ctx context.Context, engine string, language string) (asr.StreamSession, error) {
	cfg := p.buildSessionConfig(engine, language)
	conn, err := p.openConnection(ctx, cfg)
	if err != nil {
		return nil, err
	}

	session := &streamSession{
		conn:       conn,
		resultChan: make(chan asr.StreamResult, 16),
		closed:     make(chan struct{}),
	}
	go session.readLoop()
	return session, nil
}

// openConnection 建立并初始化火山云识别连接。
func (p *Provider) openConnection(ctx context.Context, cfg sessionConfig) (*websocket.Conn, error) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer; "+p.accessToken)

	conn, _, err := p.dialer.DialContext(ctx, p.baseURL, headers)
	if err != nil {
		return nil, fmt.Errorf("connect volcengine asr websocket: %w", err)
	}
	if err := writeConnFrame(conn, buildFullClientFrame(p.buildHandshakePayload(cfg))); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// GetSupportedEngines 返回当前 Provider 支持的引擎列表。
func (p *Provider) GetSupportedEngines() []string {
	return []string{"volcengine"}
}

// buildSessionConfig 组装本次识别会话参数。
func (p *Provider) buildSessionConfig(engine string, language string) sessionConfig {
	_ = engine
	return sessionConfig{
		language:   chooseLanguage(language, p.language),
		audioFmt:   p.audioFormat,
		sampleRate: p.sampleRate,
		requestID:  uuid.NewString(),
	}
}

// buildHandshakePayload 构造流式识别初始化请求。
func (p *Provider) buildHandshakePayload(cfg sessionConfig) requestPayload {
	payload := requestPayload{}
	payload.App.AppID = p.appID
	payload.App.Cluster = p.cluster
	payload.App.Token = p.accessToken
	payload.App.ResourceID = p.resourceID
	payload.User.UID = uuid.NewString()
	payload.Request.ReqID = cfg.requestID
	payload.Request.Sequence = 1
	payload.Request.Workflow = p.workflow
	payload.Request.Nbest = 1
	payload.Request.ShowUtter = true
	payload.Request.ResultType = "single"
	payload.Audio.Format = cfg.audioFmt
	payload.Audio.SampleRate = cfg.sampleRate
	payload.Audio.Language = cfg.language
	return payload
}

// SendAudio 向火山云流式会话发送音频数据。
func (s *streamSession) SendAudio(data []byte) error {
	select {
	case <-s.closed:
		return context.Canceled
	default:
	}
	if len(data) == 0 {
		return nil
	}

	for offset := 0; offset < len(data); offset += defaultChunkSize {
		end := offset + defaultChunkSize
		if end > len(data) {
			end = len(data)
		}
		if err := s.writeFrame(buildAudioFrame(data[offset:end], false)); err != nil {
			return err
		}
	}
	return nil
}

// ReceiveText 返回识别结果通道。
func (s *streamSession) ReceiveText() <-chan asr.StreamResult {
	return s.resultChan
}

// Close 发送结束帧并关闭连接。
func (s *streamSession) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		if err := s.writeFrame(buildAudioFrame(nil, true)); err != nil {
			closeErr = err
		}
		close(s.closed)
		if err := s.conn.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}

// readLoop 持续读取火山云返回并转成统一结果。
func (s *streamSession) readLoop() {
	defer close(s.resultChan)

	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			s.readErr = err
			return
		}

		payload, isFinal, err := decodeServerFrame(message)
		if err != nil {
			s.readErr = err
			return
		}
		if payload.Code != 0 {
			s.readErr = fmt.Errorf("volcengine asr failed: code=%d message=%s", payload.Code, strings.TrimSpace(payload.Message))
			return
		}

		result := asr.StreamResult{
			Text:       extractResultText(payload),
			IsFinal:    isFinal,
			Confidence: extractConfidence(payload),
		}

		select {
		case s.resultChan <- result:
		case <-s.closed:
			return
		}
		if isFinal {
			return
		}
	}
}

// writeFrame 发送协议帧。
func (s *streamSession) writeFrame(frame []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return writeConnFrame(s.conn, frame)
}

// buildFullClientFrame 构造首个握手请求帧。
func buildFullClientFrame(payload requestPayload) []byte {
	raw, _ := json.Marshal(payload)
	compressed := gzipData(raw)

	header := []byte{
		(protocolVersion << 4) | headerSize,
		(messageTypeFullClient << 4) | flagPositiveSequence,
		(serializationJSON << 4) | compressionGzip,
		0,
	}
	return appendFramePayload(header, compressed)
}

// buildAudioFrame 构造音频数据帧。
func buildAudioFrame(audio []byte, isFinal bool) []byte {
	flag := flagPositiveSequence
	if isFinal {
		flag = flagNegativeSequence
	}
	compressed := gzipData(audio)
	header := []byte{
		(protocolVersion << 4) | headerSize,
		byte((messageTypeAudioOnly << 4) | flag),
		(serializationNone << 4) | compressionGzip,
		0,
	}
	return appendFramePayload(header, compressed)
}

// appendFramePayload 将长度前缀和负载写入协议帧。
func appendFramePayload(header []byte, payload []byte) []byte {
	frame := make([]byte, 0, len(header)+4+len(payload))
	frame = append(frame, header...)
	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, uint32(len(payload)))
	frame = append(frame, sizeBytes...)
	frame = append(frame, payload...)
	return frame
}

// decodeServerFrame 解析火山云返回帧。
func decodeServerFrame(frame []byte) (responsePayload, bool, error) {
	var payload responsePayload
	if len(frame) < 8 {
		return payload, false, fmt.Errorf("invalid volcengine asr frame length")
	}

	messageType := frame[1] >> 4
	flag := frame[1] & 0x0F
	compression := frame[2] & 0x0F
	bodyLength := binary.BigEndian.Uint32(frame[4:8])
	if len(frame) < int(8+bodyLength) {
		return payload, false, fmt.Errorf("invalid volcengine asr frame payload length")
	}
	body := frame[8 : 8+bodyLength]

	if compression == compressionGzip {
		var err error
		body, err = gunzipData(body)
		if err != nil {
			return payload, false, err
		}
	}
	if messageType == messageTypeErrorResponse {
		return payload, true, fmt.Errorf("volcengine asr error frame: %s", strings.TrimSpace(string(body)))
	}
	if messageType != messageTypeFullServer {
		return payload, false, fmt.Errorf("unexpected volcengine asr message type: %d", messageType)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return payload, false, fmt.Errorf("decode volcengine asr payload: %w", err)
	}
	return payload, flag == flagNegativeSequence || payload.Sequence < 0, nil
}

// extractResultText 提取识别文本。
func extractResultText(payload responsePayload) string {
	if len(payload.Result) == 0 {
		return ""
	}
	if text := strings.TrimSpace(payload.Result[0].Text); text != "" {
		return text
	}
	if len(payload.Result[0].Utterances) > 0 {
		return strings.TrimSpace(payload.Result[0].Utterances[0].Text)
	}
	return ""
}

// extractConfidence 提取识别置信度。
func extractConfidence(payload responsePayload) float64 {
	if len(payload.Result) == 0 {
		return 0
	}
	return payload.Result[0].Confidence
}

// chooseLanguage 选择最终使用的语言参数。
func chooseLanguage(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

// estimateDurationSeconds 根据音频字节和采样率估算时长。
func estimateDurationSeconds(audio []byte, sampleRate int) float64 {
	if sampleRate <= 0 {
		sampleRate = defaultASRSampleRate
	}
	if len(audio) == 0 {
		return 0
	}
	return float64(len(audio)) / float64(sampleRate*2)
}

// gzipData 压缩协议负载。
func gzipData(raw []byte) []byte {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, _ = writer.Write(raw)
	_ = writer.Close()
	return buffer.Bytes()
}

// gunzipData 解压协议负载。
func gunzipData(raw []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("gunzip volcengine asr payload: %w", err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read gunzip volcengine asr payload: %w", err)
	}
	return body, nil
}

// writeConnFrame 向 WebSocket 连接写入一帧二进制消息。
func writeConnFrame(conn *websocket.Conn, frame []byte) error {
	if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		return fmt.Errorf("write volcengine asr frame: %w", err)
	}
	return nil
}
