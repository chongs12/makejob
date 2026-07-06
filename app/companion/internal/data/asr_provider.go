package data

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"makejob/app/companion/internal/biz"
)

const (
	volcASRURL          = "wss://openspeech.bytedance.com/api/v2/asr"
	volcASRDefaultLang  = "zh-CN"
	volcASRSegSize      = 160000 // 5s at 16kHz
)

// 官方 demo 定义的协议常量（pre-shifted byte values）
var (
	fullClientHeader  = []byte{0x11, 0x10, 0x11, 0x00} // version=1, header=1, type=full_client, flags=none, serial=json, compress=gzip
	audioOnlyHeader   = []byte{0x11, 0x20, 0x11, 0x00} // type=audio_only, flags=none
	lastAudioHeader   = []byte{0x11, 0x22, 0x11, 0x00} // type=audio_only, flags=neg_seq
)

type volcASRProvider struct {
	appID       string
	accessToken string
	cluster     string
	language    string
}

func NewVolcengineASRProvider(appID, accessToken, cluster, language string) biz.ASRProvider {
	if cluster == "" {
		cluster = "volcengine_streaming_common"
	}
	if language == "" {
		language = volcASRDefaultLang
	}
	return &volcASRProvider{
		appID:       appID,
		accessToken: accessToken,
		cluster:     cluster,
		language:    language,
	}
}

func (p *volcASRProvider) GetSupportedEngines() []string {
	return []string{"volcengine"}
}

func (p *volcASRProvider) Recognize(ctx context.Context, req biz.ASRRequest) (*biz.ASRResult, error) {
	if len(req.AudioData) == 0 {
		return nil, fmt.Errorf("asr: empty audio data")
	}

	language := req.Language
	if strings.TrimSpace(language) == "" {
		language = p.language
	}

	// 1. 建立 WebSocket 连接（使用官方 demo 的鉴权格式）
	authHeader := http.Header{"Authorization": []string{fmt.Sprintf("Bearer;%s", p.accessToken)}}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, volcASRURL, authHeader)
	if err != nil {
		return nil, fmt.Errorf("connect volcengine asr: %w", err)
	}
	defer conn.Close()
	logASR("websocket connected")

	// 2. 发送 full client request（gzip 压缩）
	requestJSON := p.buildRequestJSON(language)
	compressedReq := gzipCompress(requestJSON)
	fullMsg := buildFrame(fullClientHeader, compressedReq)
	if err := conn.WriteMessage(websocket.BinaryMessage, fullMsg); err != nil {
		return nil, fmt.Errorf("send full client request: %w", err)
	}
	logASR(fmt.Sprintf("sent full client request: %d bytes", len(fullMsg)))

	// 读取服务端响应
	resp, err := readASRResponse(conn)
	if err != nil {
		return nil, err
	}
	logASR(fmt.Sprintf("full client response: code=%d message=%s", resp.Code, resp.Message))

	// 3. 分块发送音频数据（gzip 压缩）
	audioData := req.AudioData
	for sent := 0; sent < len(audioData); sent += volcASRSegSize {
		isLast := sent+volcASRSegSize >= len(audioData)
		chunk := audioData[sent:]
		if !isLast {
			chunk = audioData[sent : sent+volcASRSegSize]
		}

		header := audioOnlyHeader
		if isLast {
			header = lastAudioHeader
		}

		compressedChunk := gzipCompress(chunk)
		audioMsg := buildFrame(header, compressedChunk)
		if err := conn.WriteMessage(websocket.BinaryMessage, audioMsg); err != nil {
			return nil, fmt.Errorf("send audio chunk: %w", err)
		}

		resp, err = readASRResponse(conn)
		if err != nil {
			return nil, err
		}
		logASR(fmt.Sprintf("audio response: code=%d, text=%s", resp.Code, extractText(resp)))
	}

	// 4. 返回最终结果
	text := extractText(resp)
	if text == "" {
		return nil, fmt.Errorf("volcengine asr returned empty text")
	}

	duration := float64(len(req.AudioData)) / float64(16000*2)
	if duration < 0.5 {
		duration = 0.5
	}

	confidence := 0.0
	if len(resp.Results) > 0 {
		confidence = float64(resp.Results[0].Confidence)
	}

	return &biz.ASRResult{
		Text:       text,
		Confidence: confidence,
		Duration:   duration,
		Language:   language,
	}, nil
}

// buildRequestJSON 构造 ASR 请求 JSON（与官方 demo constructRequest 一致）。
func (p *volcASRProvider) buildRequestJSON(language string) []byte {
	req := map[string]interface{}{
		"app": map[string]interface{}{
			"appid":   p.appID,
			"cluster": p.cluster,
			"token":   p.accessToken,
		},
		"user": map[string]interface{}{
			"uid": "companion_asr",
		},
		"request": map[string]interface{}{
			"reqid":       uuid.NewString(),
			"nbest":       1,
			"workflow":    "audio_in,resample,partition,vad,fe,decode",
			"result_type": "full",
			"sequence":    1,
		},
		"audio": map[string]interface{}{
			"format": "pcm",
			"codec":  "raw",
			"rate":   16000,
			"bits":   16,
			"channel": 1,
			"language": language,
		},
	}
	data, _ := json.Marshal(req)
	return data
}

// buildFrame 构造二进制帧：header + payloadSize(4B) + payload。
func buildFrame(header []byte, payload []byte) []byte {
	frame := make([]byte, 0, len(header)+4+len(payload))
	frame = append(frame, header...)
	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, uint32(len(payload)))
	frame = append(frame, sizeBytes...)
	frame = append(frame, payload...)
	return frame
}

// gzipCompress gzip 压缩数据。
func gzipCompress(data []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}

// gzipDecompress gzip 解压数据。
func gzipDecompress(data []byte) []byte {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return data
	}
	defer r.Close()
	out, _ := io.ReadAll(r)
	return out
}

// asrResponse 火山引擎 ASR 响应结构。
type asrResponse struct {
	ReqID    string `json:"reqid"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Sequence int    `json:"sequence"`
	Results  []struct {
		Text       string `json:"text"`
		Confidence int    `json:"confidence"`
	} `json:"result,omitempty"`
}

// readASRResponse 读取并解析服务端响应（与官方 demo parseResponse 一致）。
func readASRResponse(conn *websocket.Conn) (*asrResponse, error) {
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("read asr response: %w", err)
	}

	if len(msg) < 4 {
		return nil, fmt.Errorf("asr response too short: %d bytes", len(msg))
	}

	headerSize := int(msg[0] & 0x0F)
	messageType := msg[1] >> 4
	compression := msg[2] & 0x0F

	// 跳过 header，取 payload 部分
	payload := msg[headerSize*4:]

	switch messageType {
	case 0x09: // SERVER_FULL_RESPONSE
		if len(payload) < 4 {
			return nil, fmt.Errorf("full response payload too short")
		}
		payloadMsg := payload[4:] // 跳过 payload size
		if compression == 0x01 {
			payloadMsg = gzipDecompress(payloadMsg)
		}
		var resp asrResponse
		if err := json.Unmarshal(payloadMsg, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal full response: %w", err)
		}
		return &resp, nil

	case 0x0B: // SERVER_ACK
		if len(payload) >= 8 {
			payloadMsg := payload[8:] // 跳过 seq(4B) + payloadSize(4B)
			if compression == 0x01 {
				payloadMsg = gzipDecompress(payloadMsg)
			}
			var resp asrResponse
			if err := json.Unmarshal(payloadMsg, &resp); err == nil {
				return &resp, nil
			}
		}
		return &asrResponse{Code: 1000, Message: "ack"}, nil

	case 0x0F: // SERVER_ERROR_RESPONSE
		if len(payload) < 8 {
			return nil, fmt.Errorf("error response too short")
		}
		code := int32(binary.BigEndian.Uint32(payload[:4]))
		payloadSize := int(binary.BigEndian.Uint32(payload[4:8]))
		payloadMsg := payload[8:]
		if len(payloadMsg) > payloadSize {
			payloadMsg = payloadMsg[:payloadSize]
		}
		if compression == 0x01 {
			payloadMsg = gzipDecompress(payloadMsg)
		}
		return nil, fmt.Errorf("volcengine asr error: code=%d message=%s", code, string(payloadMsg))

	default:
		return nil, fmt.Errorf("unexpected message type: %d", messageType)
	}
}

func extractText(resp *asrResponse) string {
	if resp == nil || len(resp.Results) == 0 {
		return ""
	}
	return strings.TrimSpace(resp.Results[0].Text)
}

func logASR(msg string) {
	f, err := os.OpenFile("asr_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[ASR] %s\n", msg)
}
