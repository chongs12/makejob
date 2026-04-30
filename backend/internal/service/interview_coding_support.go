// Package service 提供业务逻辑层实现
package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

// InterviewCodingProcessEvent 表示前端提交的编程题过程事件。
type InterviewCodingProcessEvent struct {
	Type        string                 `json:"type"`
	TimestampMS int64                  `json:"timestamp_ms"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
}

// buildInterviewQuestionMessage 构造带题目元数据的 AI 题目消息记录。
func buildInterviewQuestionMessage(interviewID uint, question ai.InterviewQuestion) (*model.InterviewMessage, error) {
	metadataJSON, err := serializeInterviewQuestionMetadata(question)
	if err != nil {
		return nil, err
	}

	return &model.InterviewMessage{
		InterviewID:  interviewID,
		Role:         model.MessageRoleAI,
		Content:      question.Question,
		MessageType:  model.MessageTypeText,
		MetadataJSON: metadataJSON,
	}, nil
}

// buildInterviewAnswerMessage 根据题目类型构造用户提交的面试消息记录。
func buildInterviewAnswerMessage(interviewID uint, questionType string, answer string, finalCode string, language string) *model.InterviewMessage {
	content := strings.TrimSpace(answer)
	messageType := model.MessageTypeText

	if strings.EqualFold(strings.TrimSpace(questionType), "coding") {
		messageType = model.MessageTypeCode
		if content == "" {
			content = fmt.Sprintf("已提交 %s 代码，共 %d 行。", defaultInterviewCodeLanguage(language), countCodeLines(finalCode))
		}
	}

	return &model.InterviewMessage{
		InterviewID: interviewID,
		Role:        model.MessageRoleUser,
		Content:     content,
		MessageType: messageType,
	}
}

// serializeInterviewQuestionMetadata 将题目结构序列化到消息元数据字段。
func serializeInterviewQuestionMetadata(question ai.InterviewQuestion) (string, error) {
	payload, err := json.Marshal(question)
	if err != nil {
		return "", fmt.Errorf("序列化面试题元数据失败: %w", err)
	}
	return string(payload), nil
}

// parseInterviewQuestionMetadata 从消息元数据中解析题目结构，失败时返回空值。
func parseInterviewQuestionMetadata(raw string) *ai.InterviewQuestion {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var question ai.InterviewQuestion
	if err := json.Unmarshal([]byte(raw), &question); err != nil {
		return nil
	}
	if strings.TrimSpace(question.Question) == "" {
		return nil
	}
	return &question
}

// resolveCurrentInterviewQuestionFromMessages 从消息列表恢复当前待回答题目。
func resolveCurrentInterviewQuestionFromMessages(messages []InterviewMessageResponse, totalQuestions int, status string) *ai.InterviewQuestion {
	if status != model.InterviewStatusOngoing {
		return nil
	}

	answeredCount := 0
	for _, item := range messages {
		if item.Role == model.MessageRoleUser {
			answeredCount++
		}
	}
	if answeredCount >= totalQuestions {
		return nil
	}

	for index := len(messages) - 1; index >= 0; index-- {
		item := messages[index]
		if item.Role != model.MessageRoleAI || item.MessageType != model.MessageTypeText {
			continue
		}
		if item.Question != nil {
			return item.Question
		}
		return &ai.InterviewQuestion{
			Question: item.Content,
			Type:     "technical",
		}
	}
	return nil
}

// buildCodingAttemptFromRequest 将当前编程题提交内容转换为作答记录模型。
func buildCodingAttemptFromRequest(
	interviewID uint,
	userID uint,
	questionIndex int,
	question ai.InterviewQuestion,
	req *InterviewAnswerRequest,
) (*model.InterviewCodingAttempt, []model.InterviewCodingEvent, error) {
	if req == nil {
		return nil, nil, fmt.Errorf("编程题提交参数不能为空")
	}

	attempt := &model.InterviewCodingAttempt{
		InterviewID:        interviewID,
		UserID:             userID,
		QuestionIndex:      questionIndex,
		QuestionPrompt:     strings.TrimSpace(question.Question),
		QuestionTopic:      strings.TrimSpace(question.Topic),
		QuestionType:       firstNonEmptyInterviewString(strings.TrimSpace(question.Type), strings.TrimSpace(req.QuestionType)),
		QuestionDifficulty: strings.TrimSpace(question.Difficulty),
		Language:           defaultInterviewCodeLanguage(firstNonEmptyInterviewString(strings.TrimSpace(req.Language), strings.TrimSpace(question.Language))),
		StarterCode:        strings.TrimSpace(question.StarterCode),
		FinalCode:          strings.TrimSpace(req.FinalCode),
		FinalAnswer:        strings.TrimSpace(req.Answer),
	}

	processEvents, err := convertInterviewProcessEvents(req.ProcessEvents)
	if err != nil {
		return nil, nil, err
	}
	if len(processEvents) == 0 {
		processEvents = append(processEvents, model.InterviewCodingEvent{
			EventType: model.InterviewCodingEventTypeSubmitCode,
			EventTSMS: time.Now().UnixMilli(),
		})
	}

	return attempt, processEvents, nil
}

// convertInterviewProcessEvents 将前端事件数组转换为可入库的模型事件。
func convertInterviewProcessEvents(events []InterviewCodingProcessEvent) ([]model.InterviewCodingEvent, error) {
	result := make([]model.InterviewCodingEvent, 0, len(events))
	for index, event := range events {
		payloadJSON, err := json.Marshal(event.Payload)
		if err != nil {
			return nil, fmt.Errorf("序列化编程题过程事件失败: %w", err)
		}

		eventTSMS := event.TimestampMS
		if eventTSMS <= 0 {
			eventTSMS = time.Now().UnixMilli()
		}

		result = append(result, model.InterviewCodingEvent{
			Sequence:    index,
			EventType:   strings.TrimSpace(event.Type),
			EventTSMS:   eventTSMS,
			PayloadJSON: string(payloadJSON),
		})
	}
	return result, nil
}

// convertAttemptEventsToAI 将数据库事件转换为诊断输入所需的 AI 事件结构。
func convertAttemptEventsToAI(events []model.InterviewCodingEvent) []ai.CodingProcessEvent {
	result := make([]ai.CodingProcessEvent, 0, len(events))
	for _, event := range events {
		payload := map[string]interface{}{}
		if strings.TrimSpace(event.PayloadJSON) != "" {
			_ = json.Unmarshal([]byte(event.PayloadJSON), &payload)
		}

		result = append(result, ai.CodingProcessEvent{
			Type:        strings.TrimSpace(event.EventType),
			TimestampMS: event.EventTSMS,
			Payload:     payload,
		})
	}
	return result
}

// buildLocalCodingDiagnosis 在 AI 诊断不可用时生成本地兜底编程诊断。
func buildLocalCodingDiagnosis(attempt *model.InterviewCodingAttempt, events []ai.CodingProcessEvent) ai.CodingQuestionDiagnosis {
	diagnosis := ai.CodingQuestionDiagnosis{
		QuestionIndex: attempt.QuestionIndex,
		Language:      defaultInterviewCodeLanguage(attempt.Language),
		Score:         68,
		MistakeTags:   []string{},
		StrengthTags:  []string{},
		Evidence:      []string{},
		Suggestions:   []string{},
	}

	runCount := 0
	errorRunCount := 0
	idleCount := 0
	lastSnapshotLength := 0

	for _, event := range events {
		switch strings.TrimSpace(event.Type) {
		case model.InterviewCodingEventTypeRunCode:
			runCount++
		case model.InterviewCodingEventTypeRunResult:
			if hasInterviewRunError(event.Payload) {
				errorRunCount++
			}
		case model.InterviewCodingEventTypeIdleTimeout:
			idleCount++
		case model.InterviewCodingEventTypeCodeSnapshot:
			if code, ok := event.Payload["code"].(string); ok {
				lastSnapshotLength = len(strings.TrimSpace(code))
			}
		}
	}

	if strings.TrimSpace(attempt.FinalCode) == "" {
		diagnosis.Score = 45
		diagnosis.MistakeTags = append(diagnosis.MistakeTags, "代码实现不完整")
		diagnosis.Evidence = append(diagnosis.Evidence, "最终提交中缺少完整代码实现。")
		diagnosis.Suggestions = append(diagnosis.Suggestions, "先按函数骨架补齐主流程，再逐步验证边界条件。")
	} else {
		diagnosis.StrengthTags = append(diagnosis.StrengthTags, "能够完成基本编码提交")
	}

	if errorRunCount >= 2 {
		diagnosis.Score -= 8
		diagnosis.MistakeTags = append(diagnosis.MistakeTags, "调试路径混乱")
		diagnosis.Evidence = append(diagnosis.Evidence, fmt.Sprintf("记录到 %d 次带错误的运行结果。", errorRunCount))
		diagnosis.Suggestions = append(diagnosis.Suggestions, "先写最小可运行版本，再按一类错误一类错误地收敛。")
	}

	if idleCount > 0 {
		diagnosis.MistakeTags = appendUniqueStrings(diagnosis.MistakeTags, "边界条件生疏")
		diagnosis.Evidence = append(diagnosis.Evidence, fmt.Sprintf("过程里出现 %d 次较长停顿，说明关键分支处理还不够稳定。", idleCount))
		diagnosis.Suggestions = appendUniqueStrings(diagnosis.Suggestions, "把常见边界条件先列清单，再对照代码逐项检查。")
	}

	if runCount == 0 {
		diagnosis.MistakeTags = appendUniqueStrings(diagnosis.MistakeTags, "复杂度意识薄弱")
		diagnosis.Evidence = append(diagnosis.Evidence, "提交前没有记录到运行验证过程。")
		diagnosis.Suggestions = appendUniqueStrings(diagnosis.Suggestions, "每次编码后至少做一次自测，确认主流程和边界输入。")
	} else {
		diagnosis.StrengthTags = appendUniqueStrings(diagnosis.StrengthTags, "愿意主动验证思路")
	}

	if lastSnapshotLength > 0 && countCodeLines(attempt.FinalCode) >= 8 {
		diagnosis.StrengthTags = appendUniqueStrings(diagnosis.StrengthTags, "能够持续迭代代码版本")
	}

	if len(diagnosis.MistakeTags) == 0 {
		diagnosis.MistakeTags = append(diagnosis.MistakeTags, "状态定义不清")
		diagnosis.Evidence = append(diagnosis.Evidence, "过程数据未显示出稳定的状态拆解或验证节奏。")
		diagnosis.Suggestions = append(diagnosis.Suggestions, "写代码前先把状态、输入输出和边界分支拆开。")
	}

	if len(diagnosis.Suggestions) == 0 {
		diagnosis.Suggestions = append(diagnosis.Suggestions, "补练同类题，重点复盘状态设计和边界处理。")
	}

	if diagnosis.Score < 0 {
		diagnosis.Score = 0
	}
	if diagnosis.Score > 100 {
		diagnosis.Score = 100
	}
	diagnosis.ProcessSummary = buildCodingProcessSummary(events, runCount, errorRunCount, idleCount)
	return diagnosis
}

// buildCodingProcessSummary 生成可展示的编程题过程摘要。
func buildCodingProcessSummary(events []ai.CodingProcessEvent, runCount int, errorRunCount int, idleCount int) string {
	snapshotCount := 0
	for _, event := range events {
		if strings.TrimSpace(event.Type) == model.InterviewCodingEventTypeCodeSnapshot {
			snapshotCount++
		}
	}

	return fmt.Sprintf(
		"本题共记录 %d 条过程事件，其中代码快照 %d 次、运行 %d 次、报错运行 %d 次、长停顿 %d 次。",
		len(events),
		snapshotCount,
		runCount,
		errorRunCount,
		idleCount,
	)
}

// hasInterviewRunError 判断一次运行结果事件是否包含错误信号。
func hasInterviewRunError(payload map[string]interface{}) bool {
	if payload == nil {
		return false
	}

	if hasError, ok := payload["has_error"].(bool); ok {
		return hasError
	}
	for _, key := range []string{"error", "error_message", "stderr"} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

// appendUniqueStrings 向字符串切片追加去重后的非空值。
func appendUniqueStrings(values []string, next ...string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values)+len(next))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	for _, value := range next {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// defaultInterviewCodeLanguage 为编程题提供稳定的默认语言值。
func defaultInterviewCodeLanguage(language string) string {
	if strings.TrimSpace(language) == "" {
		return "go"
	}
	return strings.TrimSpace(language)
}

// countCodeLines 统计代码文本的有效行数，便于生成用户侧提示。
func countCodeLines(code string) int {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

// firstNonEmptyInterviewString 返回第一个非空字符串。
func firstNonEmptyInterviewString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
