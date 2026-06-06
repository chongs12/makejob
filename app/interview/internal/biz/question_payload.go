package biz

import "encoding/json"

// storedQuestionPayload 表示持久化到消息内容中的题目结构化快照。
type storedQuestionPayload struct {
	Question    string `json:"question"`
	Topic       string `json:"topic,omitempty"`
	Difficulty  string `json:"difficulty,omitempty"`
	Type        string `json:"type,omitempty"`
	Hints       string `json:"hints,omitempty"`
	Language    string `json:"language,omitempty"`
	StarterCode string `json:"starter_code,omitempty"`
	EditorMode  string `json:"editor_mode,omitempty"`
	EvalMode    string `json:"evaluation_mode,omitempty"`
}

// EncodeQuestionContent 将题目结构序列化为可存储的消息内容。
func EncodeQuestionContent(question *InterviewQuestion) string {
	if question == nil {
		return ""
	}
	payload := storedQuestionPayload{
		Question:    question.Question,
		Topic:       question.Topic,
		Difficulty:  question.Difficulty,
		Type:        question.Type,
		Hints:       question.Hints,
		Language:    question.Language,
		StarterCode: question.StarterCode,
		EditorMode:  question.EditorMode,
		EvalMode:    question.EvalMode,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return question.Question
	}
	return string(data)
}

// DecodeQuestionContent 从消息内容中恢复题目结构，兼容旧的纯文本题目。
func DecodeQuestionContent(content string) *InterviewQuestion {
	if content == "" {
		return nil
	}
	var payload storedQuestionPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return &InterviewQuestion{Question: content}
	}
	if payload.Question == "" {
		return &InterviewQuestion{Question: content}
	}
	return &InterviewQuestion{
		Question:    payload.Question,
		Topic:       payload.Topic,
		Difficulty:  payload.Difficulty,
		Type:        payload.Type,
		Hints:       payload.Hints,
		Language:    payload.Language,
		StarterCode: payload.StarterCode,
		EditorMode:  payload.EditorMode,
		EvalMode:    payload.EvalMode,
	}
}

// NormalizeMessageContent 返回适合展示和发给 AI 的消息正文。
func NormalizeMessageContent(msg *InterviewMessage) string {
	if msg == nil {
		return ""
	}
	if msg.Role != "assistant" {
		return msg.Content
	}
	question := DecodeQuestionContent(msg.Content)
	if question == nil || question.Question == "" {
		return msg.Content
	}
	return question.Question
}
