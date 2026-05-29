// Package service 提供业务逻辑层实现
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
)

const realtimeInterviewSessionPrefix = "realtime_dialog:"

// RealtimeInterviewServiceOption 描述面试服务是否启用实时语音面试模式。
type RealtimeInterviewServiceOption struct {
	Enabled bool
}

// RealtimeInterviewContext 描述实时语音面试恢复会话所需的业务上下文。
type RealtimeInterviewContext struct {
	InterviewID           uint
	IndustryCode          string
	Live2DModelKey        string
	TotalQuestions        int
	AskedQuestionCount    int
	AnsweredQuestionCount int
	Difficulty            string
	Topics                []string
	WeakTopics            []string
	InterviewMode         string
	ResumeProfile         *ai.ResumeProfile
	DialogID              string
	HasStarted            bool
}

type realtimeInterviewMetadata struct {
	Mode             string   `json:"mode"`
	InterviewMode    string   `json:"interview_mode"`
	ResumeProfileJSON string `json:"resume_profile_json,omitempty"`
	Difficulty       string   `json:"difficulty"`
	Topics           []string `json:"topics"`
	QuestionCount    int      `json:"question_count"`
}

// toStorageValue 将实时面试元数据序列化到复用字段，便于刷新后恢复配置。
func (m realtimeInterviewMetadata) toStorageValue() string {
	payload, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(payload)
}

// buildRealtimeInterviewMetadata 把创建请求转换为实时面试元数据。
func buildRealtimeInterviewMetadata(req *CreateInterviewRequest) realtimeInterviewMetadata {
	metadata := realtimeInterviewMetadata{
		Mode:          "realtime_dialog",
		InterviewMode: "general",
		Difficulty:    "mixed",
		Topics:        []string{},
		QuestionCount: 5,
	}
	if req == nil {
		return metadata
	}

	if firstNonEmptyInterviewString(req.InterviewMode, "") == "resume_driven" {
		metadata.InterviewMode = "resume_driven"
		metadata.Difficulty = "mixed"
		metadata.QuestionCount = 20
		metadata.Topics = append([]string(nil), req.Topics...)
		return metadata
	}

	metadata.Difficulty = firstNonEmptyInterviewString(req.Difficulty, metadata.Difficulty)
	metadata.Topics = append([]string(nil), req.Topics...)
	if req.QuestionCount > 0 {
		metadata.QuestionCount = req.QuestionCount
	}
	return metadata
}

// parseRealtimeInterviewMetadata 从持久化字段恢复实时面试创建配置。
func parseRealtimeInterviewMetadata(raw string, fallbackCount int) realtimeInterviewMetadata {
	metadata := realtimeInterviewMetadata{
		Mode:          "realtime_dialog",
		InterviewMode: "general",
		Difficulty:    "mixed",
		Topics:        []string{},
		QuestionCount: fallbackCount,
	}
	if metadata.QuestionCount <= 0 {
		metadata.QuestionCount = 5
	}
	if strings.TrimSpace(raw) == "" {
		return metadata
	}
	_ = json.Unmarshal([]byte(raw), &metadata)
	if strings.TrimSpace(metadata.Mode) == "" {
		metadata.Mode = "realtime_dialog"
	}
	if strings.TrimSpace(metadata.InterviewMode) == "" {
		metadata.InterviewMode = "general"
	}
	if strings.TrimSpace(metadata.Difficulty) == "" {
		metadata.Difficulty = "mixed"
	}
	if metadata.QuestionCount <= 0 {
		metadata.QuestionCount = fallbackCount
	}
	if metadata.QuestionCount <= 0 {
		metadata.QuestionCount = 5
	}
	return metadata
}

// encodeRealtimeDialogID 为实时语音面试持久化一个稳定的 dialog_id 标识。
func encodeRealtimeDialogID(dialogID string) string {
	return realtimeInterviewSessionPrefix + strings.TrimSpace(dialogID)
}

// decodeRealtimeDialogID 从持久化的 AI 会话字段中提取实时语音 dialog_id。
func decodeRealtimeDialogID(raw string) string {
	if !strings.HasPrefix(strings.TrimSpace(raw), realtimeInterviewSessionPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), realtimeInterviewSessionPrefix))
}

// isRealtimeInterviewSessionID 判断当前 AI 会话字段是否来自实时语音面试链路。
func isRealtimeInterviewSessionID(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), realtimeInterviewSessionPrefix)
}

// ensureRealtimeInterviewReady 校验实时面试是否已经进入可交互状态，避免 preparing 阶段提前进入实时问答。
func ensureRealtimeInterviewReady(interview *model.MockInterview) error {
	if interview == nil {
		return common.NewBusinessError(common.CodeNotFound, "面试记录不存在")
	}
	if interview.IsPreparing() {
		return common.NewBusinessError(common.CodeBadRequest, "简历解析中，请稍后开始面试")
	}
	if !interview.IsOngoing() {
		return common.NewBusinessError(common.CodeBadRequest, "面试已结束")
	}
	return nil
}

// GetRealtimeContext 返回恢复实时语音面试会话所需的上下文数据。
func (s *interviewService) GetRealtimeContext(ctx context.Context, userID, interviewID uint) (*RealtimeInterviewContext, error) {
	interview, messages, err := s.loadInterviewWithMessages(ctx, userID, interviewID)
	if err != nil {
		return nil, err
	}
	if err := ensureRealtimeInterviewReady(interview); err != nil {
		return nil, err
	}

	metadata := parseRealtimeInterviewMetadata(interview.AIFeedback, interview.TotalQuestions)
	askedQuestionCount := 0
	for _, item := range messages {
		if item.Role == model.MessageRoleAI && item.MessageType == model.MessageTypeText {
			askedQuestionCount++
		}
	}

	var resumeProfile *ai.ResumeProfile
	if strings.TrimSpace(metadata.ResumeProfileJSON) != "" {
		var rp ai.ResumeProfile
		if jsonErr := json.Unmarshal([]byte(metadata.ResumeProfileJSON), &rp); jsonErr == nil {
			resumeProfile = &rp
		}
	}

	return &RealtimeInterviewContext{
		InterviewID:           interview.ID,
		IndustryCode:          s.resolveInterviewIndustryCode(ctx, interview.IndustryID),
		Live2DModelKey:        strings.TrimSpace(interview.Live2DModelKey),
		TotalQuestions:        maxRealtimeInterviewInt(interview.TotalQuestions, metadata.QuestionCount),
		AskedQuestionCount:    askedQuestionCount,
		AnsweredQuestionCount: countAnsweredInterviewQuestions(messages),
		Difficulty:            metadata.Difficulty,
		Topics:                append([]string(nil), metadata.Topics...),
		WeakTopics:            s.resolveUserWeakTopicsForInterview(ctx, userID),
		InterviewMode:         firstNonEmptyInterviewString(metadata.InterviewMode, "general"),
		ResumeProfile:         resumeProfile,
		DialogID:              decodeRealtimeDialogID(interview.AISessionID),
		HasStarted:            askedQuestionCount > 0,
	}, nil
}

// BindRealtimeDialog 在实时语音会话启动后把火山返回的 dialog_id 持久化到面试记录。
func (s *interviewService) BindRealtimeDialog(ctx context.Context, userID, interviewID uint, dialogID string) error {
	interview, err := s.interviewRepo.GetByID(ctx, interviewID)
	if err != nil {
		return err
	}
	if interview == nil {
		return common.NewBusinessError(common.CodeNotFound, "面试记录不存在")
	}
	if interview.UserID != userID {
		return common.NewBusinessError(common.CodeForbidden, "无权访问该面试记录")
	}

	interview.AISessionID = encodeRealtimeDialogID(dialogID)
	return s.interviewRepo.Update(ctx, interview)
}

// AppendRealtimeUserAnswer 将实时语音识别出的最终回答写入当前面试消息流。
func (s *interviewService) AppendRealtimeUserAnswer(ctx context.Context, userID, interviewID uint, answer string) error {
	interview, messages, err := s.loadInterviewWithMessages(ctx, userID, interviewID)
	if err != nil {
		return err
	}
	if err := ensureRealtimeInterviewReady(interview); err != nil {
		return err
	}

	currentQuestion := resolveCurrentQuestionFromStoredMessages(messages)
	questionType := ""
	if currentQuestion != nil {
		questionType = currentQuestion.Type
	}

	answerMsg := buildInterviewAnswerMessage(interviewID, questionType, answer, "", "")
	return s.interviewMessageRepo.Create(ctx, answerMsg)
}

// isRealtimeInterviewClosingReply 检测面试官回复是否包含结束信号。
func isRealtimeInterviewClosingReply(text string) bool {
	closingPhrases := []string{
		"我的问题基本就是这些",
		"面试到这里",
		"面试就到这里",
		"今天的面试到这里",
		"我的问题就到这里",
		"你有什么想问我的吗",
		"你有什么想问的吗",
		"有什么想问我的",
	}
	for _, phrase := range closingPhrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

// AppendRealtimeAssistantReply 将实时模型最终回复写入消息流，并为当前题目补齐 Live2D 指令元数据。
// 返回值中的 bool 表示面试是否已结束（仅 resume_driven 模式下通过结束信号检测触发）。
func (s *interviewService) AppendRealtimeAssistantReply(ctx context.Context, userID, interviewID uint, reply string) (*ai.InterviewQuestion, int, bool, error) {
	interview, messages, err := s.loadInterviewWithMessages(ctx, userID, interviewID)
	if err != nil {
		return nil, 0, false, err
	}
	if err := ensureRealtimeInterviewReady(interview); err != nil {
		return nil, 0, false, err
	}

	metadata := parseRealtimeInterviewMetadata(interview.AIFeedback, interview.TotalQuestions)
	questionNo := 0
	for _, item := range messages {
		if item.Role == model.MessageRoleAI && item.MessageType == model.MessageTypeText {
			questionNo++
		}
	}

	isResumeDriven := strings.TrimSpace(metadata.InterviewMode) == "resume_driven"

	if !isResumeDriven && countAnsweredInterviewQuestions(messages) >= maxRealtimeInterviewInt(interview.TotalQuestions, metadata.QuestionCount) {
		msg := &model.InterviewMessage{
			InterviewID: interviewID,
			Role:        model.MessageRoleAI,
			Content:     strings.TrimSpace(reply),
			MessageType: model.MessageTypeText,
		}
		if err := s.interviewMessageRepo.Create(ctx, msg); err != nil {
			return nil, 0, false, err
		}
		return nil, questionNo, false, nil
	}

	// resume_driven 模式：检测结束信号
	finished := false
	if isResumeDriven && isRealtimeInterviewClosingReply(reply) {
		finished = true
	}

	question := ai.InterviewQuestion{
		Question:   strings.TrimSpace(reply),
		Topic:      resolveRealtimeQuestionTopic(metadata, questionNo),
		Difficulty: resolveRealtimeQuestionDifficulty(metadata.Difficulty, questionNo),
		Type:       "technical",
	}
	s.decorateInterviewQuestionWithLive2D(ctx, interview, &question, resolveCurrentQuestionFromStoredMessages(messages))

	questionMsg, err := buildInterviewQuestionMessage(interviewID, question)
	if err != nil {
		return nil, 0, false, err
	}
	if err := s.interviewMessageRepo.Create(ctx, questionMsg); err != nil {
		return nil, 0, false, err
	}
	return &question, questionNo + 1, finished, nil
}

// finishRealtimeInterview 在实时语音模式下用持久化问答记录生成兜底面试报告。
func (s *interviewService) finishRealtimeInterview(ctx context.Context, userID uint, interview *model.MockInterview) (*InterviewReportResponse, error) {
	if interview == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "面试记录不存在")
	}
	if interview.UserID != userID {
		return nil, common.NewBusinessError(common.CodeForbidden, "无权访问该面试记录")
	}

	messages, err := s.interviewMessageRepo.ListByInterview(ctx, interview.ID)
	if err != nil {
		return nil, err
	}

	report := buildRealtimeInterviewReport(messages, interview.TotalQuestions)
	if err := s.enrichInterviewReportWithCoding(ctx, interview, &report); err != nil {
		return nil, fmt.Errorf("生成编程题诊断失败: %w", err)
	}
	reportJSON, err := serializeInterviewReport(report)
	if err != nil {
		return nil, fmt.Errorf("序列化面试报告失败: %w", err)
	}

	now := time.Now()
	if err := s.persistLearningArchiveEntries(ctx, interview, report, now); err != nil {
		return nil, err
	}

	interview.Status = model.InterviewStatusCompleted
	interview.Score = report.OverallScore
	interview.EndedAt = &now
	interview.ReportJSON = reportJSON
	interview.AIFeedback = buildInterviewReportSummary(report)
	if err := s.interviewRepo.Update(ctx, interview); err != nil {
		return nil, err
	}

	var duration int64
	if interview.StartedAt != nil {
		duration = int64(now.Sub(*interview.StartedAt).Seconds())
	}
	return &InterviewReportResponse{
		InterviewID: interview.ID,
		Report:      &report,
		Duration:    duration,
		CompletedAt: now,
	}, nil
}

// loadInterviewWithMessages 为实时语音链路一次性加载面试记录与历史消息。
func (s *interviewService) loadInterviewWithMessages(ctx context.Context, userID, interviewID uint) (*model.MockInterview, []model.InterviewMessage, error) {
	interview, err := s.interviewRepo.GetByID(ctx, interviewID)
	if err != nil {
		return nil, nil, err
	}
	if interview == nil {
		return nil, nil, common.NewBusinessError(common.CodeNotFound, "面试记录不存在")
	}
	if interview.UserID != userID {
		return nil, nil, common.NewBusinessError(common.CodeForbidden, "无权访问该面试记录")
	}

	messages, err := s.interviewMessageRepo.ListByInterview(ctx, interviewID)
	if err != nil {
		return nil, nil, err
	}
	return interview, messages, nil
}

// resolveRealtimeQuestionDifficulty 为实时面试补齐一个稳定的题目难度标签。
func resolveRealtimeQuestionDifficulty(raw string, questionIndex int) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "easy", "medium", "hard":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		switch {
		case questionIndex == 0:
			return "easy"
		case questionIndex%3 == 2:
			return "hard"
		default:
			return "medium"
		}
	}
}

// resolveRealtimeQuestionTopic 为实时面试题目补齐当前展示所需的主题标签。
func resolveRealtimeQuestionTopic(metadata realtimeInterviewMetadata, questionIndex int) string {
	if len(metadata.Topics) == 0 {
		return "技术面试"
	}
	return strings.TrimSpace(metadata.Topics[questionIndex%len(metadata.Topics)])
}

// buildRealtimeInterviewReport 基于实时面试消息流生成一个可用的兜底报告。
func buildRealtimeInterviewReport(messages []model.InterviewMessage, totalQuestions int) ai.InterviewReport {
	dimensionScores := map[string]float64{}
	dimensionCounts := map[string]int{}
	answeredCount := 0
	correctCount := 0
	var totalScore float64
	currentTopic := "综合表达"

	for _, item := range messages {
		switch item.Role {
		case model.MessageRoleAI:
			if question := parseInterviewQuestionMetadata(item.MetadataJSON); question != nil && strings.TrimSpace(question.Topic) != "" {
				currentTopic = strings.TrimSpace(question.Topic)
			}
		case model.MessageRoleUser:
			answer := strings.TrimSpace(item.Content)
			if answer == "" {
				continue
			}
			score := estimateRealtimeAnswerScore(answer)
			totalScore += score
			answeredCount++
			if score >= 60 {
				correctCount++
			}
			dimensionScores[currentTopic] += score
			dimensionCounts[currentTopic]++
		}
	}

	for topic, score := range dimensionScores {
		count := dimensionCounts[topic]
		if count <= 0 {
			continue
		}
		dimensionScores[topic] = roundRealtimeScore(score / float64(count))
	}

	overallScore := 0.0
	if answeredCount > 0 {
		overallScore = roundRealtimeScore(totalScore / float64(answeredCount))
	}

	strengths, weaknesses := summarizeRealtimeInterviewPerformance(dimensionScores, answeredCount)
	suggestions := []string{
		"回答时先给结论，再补原理、例子和边界条件。",
		"遇到问题先拆场景和约束，避免直接堆概念。",
		"复盘每轮表达是否覆盖了性能、稳定性和工程取舍。",
	}
	if answeredCount == 0 {
		suggestions = []string{"先至少完成一轮有效作答，再生成更有参考价值的面试报告。"}
	}

	return ai.InterviewReport{
		OverallScore:    overallScore,
		TotalQuestions:  maxRealtimeInterviewInt(totalQuestions, answeredCount),
		CorrectCount:    correctCount,
		DimensionScores: dimensionScores,
		Strengths:       strengths,
		Weaknesses:      weaknesses,
		Suggestions:     suggestions,
		Summary: fmt.Sprintf(
			"实时语音面试共记录 %d 轮有效回答，综合得分 %.0f 分。建议继续围绕低分主题做口语化复盘与工程化表达训练。",
			answeredCount,
			overallScore,
		),
	}
}

// estimateRealtimeAnswerScore 根据回答长度和结构化痕迹给出一个稳定的兜底分数。
func estimateRealtimeAnswerScore(answer string) float64 {
	runes := []rune(strings.TrimSpace(answer))
	score := 48.0
	switch {
	case len(runes) >= 220:
		score = 88
	case len(runes) >= 140:
		score = 78
	case len(runes) >= 80:
		score = 68
	case len(runes) >= 30:
		score = 58
	}

	for _, marker := range []string{"例如", "比如", "原理", "场景", "边界", "复杂度", "稳定性", "取舍"} {
		if strings.Contains(answer, marker) {
			score += 2
		}
	}
	if score > 100 {
		return 100
	}
	return score
}

// summarizeRealtimeInterviewPerformance 为实时语音面试兜底报告提炼优势和风险。
func summarizeRealtimeInterviewPerformance(dimensionScores map[string]float64, answeredCount int) ([]string, []string) {
	strengths := []string{}
	weaknesses := []string{}
	for topic, score := range dimensionScores {
		switch {
		case score >= 80:
			strengths = append(strengths, fmt.Sprintf("%s 表达完整，具备较强的结构化说明能力。", topic))
		case score < 65:
			weaknesses = append(weaknesses, fmt.Sprintf("%s 的回答仍偏概念化，缺少原理或工程细节支撑。", topic))
		}
	}
	if answeredCount == 0 {
		return []string{"已完成实时语音链路初始化。"}, []string{"当前没有足够的有效回答，暂时无法判断真实面试水平。"}
	}
	if len(strengths) == 0 {
		strengths = append(strengths, "能够完成基本口语作答，具备继续加强的表达基础。")
	}
	if len(weaknesses) == 0 {
		weaknesses = append(weaknesses, "整体发挥相对均衡，下一步应重点提升回答深度和追问应对能力。")
	}
	return strengths, weaknesses
}

// roundRealtimeScore 对实时面试兜底评分做两位小数规整。
func roundRealtimeScore(score float64) float64 {
	return float64(int(score*100+0.5)) / 100
}

// maxRealtimeInterviewInt 返回两个整数中的较大值，避免实时链路读取到空题量时退化为零。
func maxRealtimeInterviewInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
