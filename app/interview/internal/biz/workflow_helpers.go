package biz

import "strings"

// BuildQuestionMessage 构造一条用于持久化的题目消息。
func BuildQuestionMessage(interviewID uint64, questionIndex int32, question *InterviewQuestion) *InterviewMessage {
	if question == nil {
		return nil
	}
	return &InterviewMessage{
		InterviewID:   interviewID,
		Role:          "assistant",
		Content:       EncodeQuestionContent(question),
		MessageType:   "text",
		QuestionIndex: questionIndex,
	}
}

// NormalizeHistoryMessages 将存储消息转换为适合继续发给 AI 的历史记录。
func NormalizeHistoryMessages(messages []*InterviewMessage) []*InterviewMessage {
	if len(messages) == 0 {
		return nil
	}
	result := make([]*InterviewMessage, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		result = append(result, &InterviewMessage{
			ID:            msg.ID,
			InterviewID:   msg.InterviewID,
			Role:          msg.Role,
			Content:       NormalizeMessageContent(msg),
			MessageType:   msg.MessageType,
			QuestionIndex: msg.QuestionIndex,
			CreatedAt:     msg.CreatedAt,
		})
	}
	return result
}

// ExtractQuestionByIndex 从消息列表中提取指定题号对应的题目。
func ExtractQuestionByIndex(messages []*InterviewMessage, questionIndex int32) *InterviewQuestion {
	for _, msg := range messages {
		if msg == nil || msg.Role != "assistant" || msg.QuestionIndex != questionIndex {
			continue
		}
		question := DecodeQuestionContent(msg.Content)
		if question != nil && strings.TrimSpace(question.Question) != "" {
			return question
		}
	}
	return nil
}

// ResolveCurrentTopic 从历史消息中解析最近一次题目的主题。
func ResolveCurrentTopic(messages []*InterviewMessage) string {
	for index := len(messages) - 1; index >= 0; index-- {
		question := DecodeQuestionContent(messages[index].Content)
		if question != nil && strings.TrimSpace(question.Topic) != "" {
			return strings.TrimSpace(question.Topic)
		}
	}
	return ""
}
