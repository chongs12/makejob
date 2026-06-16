package biz

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// WSMessageType WebSocket 消息类型（对齐单体 handler/interview_handler.go）
type WSMessageType string

const (
	WSMessageTypeConnected                  WSMessageType = "connected"
	WSMessageTypeSessionReady               WSMessageType = "session_ready"
	WSMessageTypeInterviewState             WSMessageType = "interview_state"
	WSMessageTypeUserAnswer                 WSMessageType = "user_answer"
	WSMessageTypeAIQuestion                 WSMessageType = "ai_question"
	WSMessageTypeASRPartial                 WSMessageType = "asr_partial"
	WSMessageTypeASRFinal                   WSMessageType = "asr_final"
	WSMessageTypeTTSAudio                   WSMessageType = "tts_audio"
	WSMessageTypeLive2DExpression           WSMessageType = "live2d_expression"
	WSMessageTypeAssistantTranscriptPartial WSMessageType = "assistant_transcript_partial"
	WSMessageTypeAssistantTranscriptFinal   WSMessageType = "assistant_transcript_final"
	WSMessageTypeAssistantAudioChunk        WSMessageType = "assistant_audio_chunk"
	WSMessageTypeAssistantTurnFinished      WSMessageType = "assistant_turn_finished"
	WSMessageTypeBargeIn                    WSMessageType = "barge_in"
	WSMessageTypeFinished                   WSMessageType = "finished"
	WSMessageTypeError                      WSMessageType = "error"
)

// WSMessage 统一 WebSocket 消息结构（对齐单体 WSMessage）
type WSMessage struct {
	Type        WSMessageType `json:"type"`
	Content     string        `json:"content,omitempty"`
	Data        interface{}   `json:"data,omitempty"`
	Timestamp   int64         `json:"timestamp"`
	TraceID     string        `json:"trace_id,omitempty"`
	InterviewID uint64        `json:"interview_id,omitempty"`
}

// wsInterviewStatePayload 会话状态载荷
type wsInterviewStatePayload struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Mode    string `json:"mode,omitempty"`
}

// wsASRPayload ASR 识别结果载荷
type wsASRPayload struct {
	Text       string  `json:"text"`
	IsFinal    bool    `json:"is_final"`
	Confidence float64 `json:"confidence"`
}

// wsAssistantTranscriptPayload 字幕文本载荷
type wsAssistantTranscriptPayload struct {
	Text       string `json:"text"`
	IsFinal    bool   `json:"is_final"`
	QuestionID string `json:"question_id,omitempty"`
	ReplyID    string `json:"reply_id,omitempty"`
}

// wsAssistantAudioChunkPayload 音频块载荷
type wsAssistantAudioChunkPayload struct {
	AudioBase64 string `json:"audio_base64"`
	Format      string `json:"format"`
	SampleRate  int    `json:"sample_rate"`
}

// wsAssistantTurnPayload 轮次完成载荷
type wsAssistantTurnPayload struct {
	Text       string `json:"text"`
	QuestionNo int    `json:"question_no"`
	IsQuestion bool   `json:"is_question"`
}

// wsSender 封装 WebSocket 消息发送，对齐单体 WSMessage 协议
type wsSender struct {
	conn        *websocket.Conn
	traceID     string
	interviewID uint64
}

// newWSSender 创建消息发送器
func newWSSender(conn *websocket.Conn, interviewID uint64) *wsSender {
	return &wsSender{
		conn:        conn,
		traceID:     fmt.Sprintf("rt-%d", time.Now().UnixNano()),
		interviewID: interviewID,
	}
}

// send 发送统一格式的 WebSocket 消息（对齐单体 send 方法）
func (s *wsSender) send(msg WSMessage) error {
	msg.Timestamp = time.Now().UnixMilli()
	msg.TraceID = s.traceID
	msg.InterviewID = s.interviewID
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return s.conn.WriteMessage(websocket.TextMessage, data)
}

// sendConnected 发送连接成功事件
func (s *wsSender) sendConnected() error {
	return s.send(WSMessage{
		Type:    WSMessageTypeConnected,
		Content: "实时面试链路已连接。",
	})
}

// sendSessionReady 发送会话就绪事件
func (s *wsSender) sendSessionReady() error {
	return s.send(WSMessage{
		Type: WSMessageTypeSessionReady,
		Data: wsInterviewStatePayload{
			Status:  "ready",
			Message: "实时面试链路已就绪。",
			Mode:    "realtime",
		},
	})
}

// sendState 发送状态变更事件（对齐单体 sendState）
func (s *wsSender) sendState(status, message string) error {
	return s.send(WSMessage{
		Type:    WSMessageTypeInterviewState,
		Content: message,
		Data: wsInterviewStatePayload{
			Status:  status,
			Message: message,
			Mode:    "realtime",
		},
	})
}

// sendError 发送错误事件（对齐单体 sendError）
func (s *wsSender) sendError(message string) error {
	return s.send(WSMessage{
		Type:    WSMessageTypeError,
		Content: message,
	})
}

// sendBargeIn 发送打断事件
func (s *wsSender) sendBargeIn() error {
	return s.send(WSMessage{Type: WSMessageTypeBargeIn})
}

// sendUserAnswer 发送用户回答事件
func (s *wsSender) sendUserAnswer(content string) error {
	return s.send(WSMessage{
		Type:    WSMessageTypeUserAnswer,
		Content: content,
	})
}

// sendASRPartial 发送 ASR 部分识别事件
func (s *wsSender) sendASRPartial(text string) error {
	return s.send(WSMessage{
		Type:    WSMessageTypeASRPartial,
		Content: text,
		Data: wsASRPayload{
			Text:       text,
			IsFinal:    false,
			Confidence: 1,
		},
	})
}

// sendASRFinal 发送 ASR 最终识别事件
func (s *wsSender) sendASRFinal(text string) error {
	return s.send(WSMessage{
		Type:    WSMessageTypeASRFinal,
		Content: text,
		Data: wsASRPayload{
			Text:       text,
			IsFinal:    true,
			Confidence: 1,
		},
	})
}

// sendAssistantTranscriptPartial 发送字幕部分事件
func (s *wsSender) sendAssistantTranscriptPartial(text, questionID, replyID string) error {
	return s.send(WSMessage{
		Type:    WSMessageTypeAssistantTranscriptPartial,
		Content: text,
		Data: wsAssistantTranscriptPayload{
			Text:       text,
			IsFinal:    false,
			QuestionID: questionID,
			ReplyID:    replyID,
		},
	})
}

// sendAssistantTranscriptFinal 发送字幕最终事件
func (s *wsSender) sendAssistantTranscriptFinal(text, questionID, replyID string) error {
	return s.send(WSMessage{
		Type:    WSMessageTypeAssistantTranscriptFinal,
		Content: text,
		Data: wsAssistantTranscriptPayload{
			Text:       text,
			IsFinal:    true,
			QuestionID: questionID,
			ReplyID:    replyID,
		},
	})
}

// sendAssistantAudioChunk 发送音频块事件（base64 编码，对齐单体）
func (s *wsSender) sendAssistantAudioChunk(audio []byte, format string, sampleRate int) error {
	return s.send(WSMessage{
		Type: WSMessageTypeAssistantAudioChunk,
		Data: wsAssistantAudioChunkPayload{
			AudioBase64: base64.StdEncoding.EncodeToString(audio),
			Format:      format,
			SampleRate:  sampleRate,
		},
	})
}

// sendAssistantTurnFinished 发送轮次完成事件
func (s *wsSender) sendAssistantTurnFinished(text string, questionNo int, isQuestion bool) error {
	return s.send(WSMessage{
		Type:    WSMessageTypeAssistantTurnFinished,
		Content: text,
		Data: wsAssistantTurnPayload{
			Text:       text,
			QuestionNo: questionNo,
			IsQuestion: isQuestion,
		},
	})
}

// sendFinished 发送面试结束事件
func (s *wsSender) sendFinished(content string) error {
	return s.send(WSMessage{
		Type:    WSMessageTypeFinished,
		Content: content,
	})
}

// sendBinary 发送原始二进制帧（仅用于透传火山引擎原始音频，一般不直接调用）
func (s *wsSender) sendBinary(data []byte) error {
	return s.conn.WriteMessage(websocket.BinaryMessage, data)
}
