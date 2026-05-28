package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"makejob-backend/internal/ai"
	realtimevolc "makejob-backend/internal/realtime/volcengine"
	"makejob-backend/internal/service"
	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
)

type realtimeSentencePayload struct {
	TTSType    string `json:"tts_type"`
	Text       string `json:"text"`
	QuestionID string `json:"question_id"`
	ReplyID    string `json:"reply_id"`
}

type realtimeChatPayload struct {
	Content    string `json:"content"`
	QuestionID string `json:"question_id"`
	ReplyID    string `json:"reply_id"`
}

type realtimeChatQueryConfirmedPayload struct {
	QuestionID string `json:"question_id"`
}

type realtimeASRResponsePayload struct {
	Results []struct {
		Text      string `json:"text"`
		IsInterim bool   `json:"is_interim"`
	} `json:"results"`
}

// bootstrapRealtime 恢复实时语音面试上下文并建立火山实时会话。
func (s *wsInterviewSession) bootstrapRealtime() {
	applogger.Info("bootstrap realtime interview session",
		zap.String("trace_id", s.traceID),
		zap.Uint("user_id", s.userID),
		zap.Uint("interview_id", s.interviewID),
	)
	ctx, err := s.handler.interviewService.GetRealtimeContext(context.Background(), s.userID, s.interviewID)
	if err != nil {
		s.sendError("恢复实时面试上下文失败: " + err.Error())
		return
	}
	s.realtimeContext = ctx
	s.live2DModelKey = strings.TrimSpace(ctx.Live2DModelKey)

	s.send(WSMessage{
		Type: WSMessageTypeSessionReady,
		Data: wsInterviewStatePayload{
			Status:  "ready",
			Message: "实时面试链路已就绪。",
			Mode:    "realtime",
		},
	})
	s.sendDirective(nil)

	client, err := realtimevolc.NewClient(s.handler.realtimeConfig)
	if err != nil {
		s.sendState("error", "实时语音模型配置无效，当前无法启动端到端对话。")
		s.sendError("实时语音模型未正确配置: " + err.Error())
		return
	}

	dialogID, err := client.Start(context.Background(), realtimevolc.StartOptions{
		SessionID:       s.traceID,
		DialogID:        ctx.DialogID,
		BotName:         s.handler.realtimeConfig.BotName,
		SystemRole:      s.buildRealtimeSystemRole(ctx),
		SpeakingStyle:   s.handler.realtimeConfig.SpeakingStyle,
		CharacterPrompt: s.handler.realtimeConfig.CharacterPrompt,
		LocationCity:    s.handler.realtimeConfig.LocationCity,
		InputMode:       s.handler.realtimeConfig.InputMode,
		RecvTimeout:     s.handler.realtimeConfig.RecvTimeout,
		Speaker:         s.handler.realtimeConfig.Speaker,
	})
	if err != nil {
		s.sendState("error", "实时语音会话启动失败，请检查火山实时语音配置和凭证。")
		s.sendError("启动实时语音会话失败: " + err.Error())
		return
	}
	applogger.Info("realtime interview session started",
		zap.String("trace_id", s.traceID),
		zap.Uint("interview_id", s.interviewID),
		zap.String("dialog_id", dialogID),
		zap.Bool("restored_dialog", strings.TrimSpace(ctx.DialogID) != ""),
	)

	s.realtimeClient = client
	if strings.TrimSpace(dialogID) != "" && dialogID != ctx.DialogID {
		if err := s.handler.interviewService.BindRealtimeDialog(context.Background(), s.userID, s.interviewID, dialogID); err != nil {
			applogger.Warn("bind realtime dialog id failed",
				zap.String("trace_id", s.traceID),
				zap.Uint("interview_id", s.interviewID),
				zap.Error(err),
			)
		}
	}

	go s.consumeRealtimeEvents(client)

	if ctx.HasStarted {
		s.sendState("ready", "已恢复当前实时面试，可直接继续作答。")
		return
	}

	s.sendState("speaking", "面试官正在准备第一题。")
	kickoffPrompt := s.buildRealtimeKickoffPrompt(ctx)
	applogger.Info("realtime interview kickoff prompt sent",
		zap.String("trace_id", s.traceID),
		zap.Uint("interview_id", s.interviewID),
		zap.String("prompt", kickoffPrompt),
	)
	if err := client.SendTextQuery(kickoffPrompt); err != nil {
		s.sendError("启动第一题失败: " + err.Error())
	}
}

// handleRealtimeUserAnswer 处理手动输入的文本回答，并交给实时模型继续当前轮次。
func (s *wsInterviewSession) handleRealtimeUserAnswer(answer string) {
	if strings.TrimSpace(answer) == "" {
		s.sendError("回答内容不能为空")
		return
	}
	if s.realtimeClient == nil {
		s.sendError("实时语音会话尚未建立")
		return
	}

	if err := s.handler.interviewService.AppendRealtimeUserAnswer(context.Background(), s.userID, s.interviewID, answer); err != nil {
		s.sendError("写入实时回答失败: " + err.Error())
		return
	}
	s.send(WSMessage{
		Type:    WSMessageTypeUserAnswer,
		Content: strings.TrimSpace(answer),
	})
	s.sendState("thinking", "AI 面试官正在整理你的回答。")
	if err := s.realtimeClient.SendTextQuery(answer); err != nil {
		s.sendError("发送实时文本回答失败: " + err.Error())
	}
}

// handleRealtimeAudioStart 标记用户开始一轮按键说话，前端后续会持续发送音频块。
func (s *wsInterviewSession) handleRealtimeAudioStart(_ *json.RawMessage) {
	if s.realtimeClient == nil {
		s.sendError("实时语音会话尚未建立")
		return
	}
	s.realtimeMu.Lock()
	s.realtimeTurn.userAudioChunkCount = 0
	s.realtimeMu.Unlock()
	s.sendState("listening", "正在收听你的回答，请继续说。")
}

// handleRealtimeAudioChunk 解码浏览器上传的 PCM 音频，并转发给火山实时模型。
func (s *wsInterviewSession) handleRealtimeAudioChunk(rawData *json.RawMessage) {
	if rawData == nil || len(*rawData) == 0 {
		s.sendError("缺少语音音频数据")
		return
	}
	if s.realtimeClient == nil {
		s.sendError("实时语音会话尚未建立")
		return
	}

	var payload wsAudioChunkPayload
	if err := json.Unmarshal(*rawData, &payload); err != nil {
		s.sendError("语音音频数据解析失败: " + err.Error())
		return
	}
	chunk, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload.AudioBase64))
	if err != nil {
		s.sendError("语音音频数据解码失败: " + err.Error())
		return
	}
	if err := s.realtimeClient.SendAudio(chunk); err != nil {
		s.sendError("发送实时语音音频失败: " + err.Error())
		return
	}

	s.realtimeMu.Lock()
	s.realtimeTurn.userAudioChunkCount++
	currentChunkCount := s.realtimeTurn.userAudioChunkCount
	s.realtimeMu.Unlock()

	if currentChunkCount == 1 {
		applogger.Info("realtime interview first user audio chunk forwarded",
			zap.String("trace_id", s.traceID),
			zap.Uint("interview_id", s.interviewID),
			zap.Int("chunk_bytes", len(chunk)),
		)
	}
}

// handleRealtimeAudioEnd 在按键说话结束后切换为等待识别结果状态。
func (s *wsInterviewSession) handleRealtimeAudioEnd() {
	if s.realtimeClient == nil {
		s.sendError("实时语音会话尚未建立")
		return
	}
	s.realtimeMu.Lock()
	audioChunkCount := s.realtimeTurn.userAudioChunkCount
	s.realtimeMu.Unlock()
	if audioChunkCount <= 0 {
		s.sendState("ready", "本轮没有采集到有效语音，请点击“开始语音回答”后重新作答。")
		return
	}
	if err := s.realtimeClient.SendEndASR(); err != nil {
		s.sendError("结束本轮实时语音输入失败: " + err.Error())
		return
	}
	applogger.Info("realtime interview user audio turn ended",
		zap.String("trace_id", s.traceID),
		zap.Uint("interview_id", s.interviewID),
		zap.Int("audio_chunk_count", audioChunkCount),
	)
	s.sendState("thinking", "已结束收音，面试官正在整理你的回答。")
}

// consumeRealtimeEvents 持续消费火山实时模型事件，并转发为前端可消费的 JSON 协议。
func (s *wsInterviewSession) consumeRealtimeEvents(client *realtimevolc.Client) {
	for event := range client.Events() {
		switch event.ID {
		case realtimevolc.EventASRInfo:
			s.finalizeRealtimeAssistantTurn(true)
			s.send(WSMessage{Type: WSMessageTypeBargeIn})
			s.sendState("listening", "检测到你开始说话，已切到收听状态。")
		case realtimevolc.EventASRResponse:
			s.handleRealtimeASRResponse(event.Payload)
		case realtimevolc.EventASREnded:
			s.handleRealtimeASREnded()
		case realtimevolc.EventTTSSentenceStart:
			s.handleRealtimeTTSSentenceStart(event.Payload)
		case realtimevolc.EventTTSSentenceEnd:
			s.handleRealtimeTTSSentenceEnd(event.Payload)
		case realtimevolc.EventTTSResponse:
			s.handleRealtimeTTSAudio(event.Payload)
		case realtimevolc.EventTTSEnded:
			s.markRealtimeTurnAudioEnded()
		case realtimevolc.EventChatTextQueryConfirmed:
			s.handleRealtimeChatQueryConfirmed(event.Payload)
		case realtimevolc.EventChatResponse:
			s.handleRealtimeChatResponse(event.Payload)
		case realtimevolc.EventChatEnded:
			s.markRealtimeTurnTextEnded()
		case realtimevolc.EventSessionFinished:
			s.sendState("ready", "实时会话已结束。")
		}
	}
}

// handleRealtimeASRResponse 把实时识别文本片段同步给前端，并缓存最终结果用于落库。
func (s *wsInterviewSession) handleRealtimeASRResponse(payload []byte) {
	var response realtimeASRResponsePayload
	if err := json.Unmarshal(payload, &response); err != nil {
		return
	}
	if len(response.Results) == 0 {
		return
	}

	lastResult := response.Results[len(response.Results)-1]
	text := strings.TrimSpace(lastResult.Text)
	if text == "" {
		return
	}

	s.realtimeMu.Lock()
	s.realtimeTurn.userFinalText = text
	s.realtimeMu.Unlock()

	msgType := WSMessageTypeASRPartial
	if !lastResult.IsInterim {
		msgType = WSMessageTypeASRFinal
	}
	s.send(WSMessage{
		Type:    msgType,
		Content: text,
		Data: wsASRPayload{
			Text:       text,
			IsFinal:    !lastResult.IsInterim,
			Confidence: 1,
		},
	})
}

// handleRealtimeASREnded 在实时模型确认用户说话结束后，把最终文本写入消息仓库并等待模型回复。
func (s *wsInterviewSession) handleRealtimeASREnded() {
	s.realtimeMu.Lock()
	text := strings.TrimSpace(s.realtimeTurn.userFinalText)
	s.realtimeTurn.userFinalText = ""
	s.realtimeTurn.userAudioChunkCount = 0
	s.realtimeMu.Unlock()

	if text == "" {
		s.sendState("ready", "本轮未识别到有效回答，请重新开始。")
		return
	}
	if err := s.handler.interviewService.AppendRealtimeUserAnswer(context.Background(), s.userID, s.interviewID, text); err != nil {
		s.sendError("写入实时语音回答失败: " + err.Error())
		return
	}
	s.send(WSMessage{
		Type:    WSMessageTypeUserAnswer,
		Content: text,
	})
	s.sendState("thinking", "AI 面试官正在继续当前对话。")
}

// handleRealtimeTTSSentenceStart 同步模型当前即将播报的一段字幕文本。
func (s *wsInterviewSession) handleRealtimeTTSSentenceStart(payload []byte) {
	var response realtimeSentencePayload
	if err := json.Unmarshal(payload, &response); err != nil {
		return
	}

	text := strings.TrimSpace(response.Text)
	if text == "" {
		return
	}

	s.realtimeMu.Lock()
	if response.QuestionID != "" {
		s.realtimeTurn.questionID = response.QuestionID
	}
	if response.ReplyID != "" {
		s.realtimeTurn.replyID = response.ReplyID
	}
	s.realtimeTurn.liveText = appendRealtimeTextChunk(s.realtimeTurn.liveText, text)
	currentText := s.realtimeTurn.liveText
	s.realtimeMu.Unlock()

	applogger.Info("realtime assistant tts sentence started",
		zap.String("trace_id", s.traceID),
		zap.Uint("interview_id", s.interviewID),
		zap.String("text", text),
		zap.String("question_id", response.QuestionID),
		zap.String("reply_id", response.ReplyID),
	)

	s.sendState("speaking", "面试官正在播报。")
	s.send(WSMessage{
		Type:    WSMessageTypeAssistantTranscriptPartial,
		Content: currentText,
		Data: wsAssistantTranscriptPayload{
			Text:       currentText,
			IsFinal:    false,
			QuestionID: response.QuestionID,
			ReplyID:    response.ReplyID,
		},
	})
}

// handleRealtimeTTSSentenceEnd 标记当前播报句子的字幕已结束，便于没有 ChatEnded 事件时也能完成一轮收口。
func (s *wsInterviewSession) handleRealtimeTTSSentenceEnd(payload []byte) {
	var response realtimeSentencePayload
	if err := json.Unmarshal(payload, &response); err == nil {
		s.realtimeMu.Lock()
		if response.QuestionID != "" {
			s.realtimeTurn.questionID = response.QuestionID
		}
		if response.ReplyID != "" {
			s.realtimeTurn.replyID = response.ReplyID
		}
		s.realtimeMu.Unlock()
	}

	s.realtimeMu.Lock()
	if strings.TrimSpace(s.realtimeTurn.replyText) == "" && strings.TrimSpace(s.realtimeTurn.liveText) != "" {
		s.realtimeTurn.textEnded = true
	}
	s.realtimeMu.Unlock()
	s.finalizeRealtimeAssistantTurn(false)
}

// handleRealtimeTTSAudio 将实时返回的 PCM 音频块转成 base64 后透传给前端播放器。
func (s *wsInterviewSession) handleRealtimeTTSAudio(audio []byte) {
	if len(audio) == 0 {
		return
	}
	s.send(WSMessage{
		Type: WSMessageTypeAssistantAudioChunk,
		Data: wsAssistantAudioChunkPayload{
			AudioBase64: base64.StdEncoding.EncodeToString(audio),
			Format:      s.handler.realtimeConfig.TTSFormat,
			SampleRate:  s.handler.realtimeConfig.TTSSampleRate,
		},
	})
}

// handleRealtimeChatResponse 缓存模型本轮最终回复文本，供音频播报结束后统一落库。
func (s *wsInterviewSession) handleRealtimeChatResponse(payload []byte) {
	var response realtimeChatPayload
	if err := json.Unmarshal(payload, &response); err != nil {
		return
	}
	text := strings.TrimSpace(response.Content)
	if text == "" {
		return
	}

	applogger.Info("realtime assistant chat response received",
		zap.String("trace_id", s.traceID),
		zap.Uint("interview_id", s.interviewID),
		zap.String("text", text),
		zap.String("question_id", response.QuestionID),
		zap.String("reply_id", response.ReplyID),
	)

	s.realtimeMu.Lock()
	s.realtimeTurn.replyText = appendRealtimeTextChunk(s.realtimeTurn.replyText, text)
	if response.QuestionID != "" {
		s.realtimeTurn.questionID = response.QuestionID
	}
	if response.ReplyID != "" {
		s.realtimeTurn.replyID = response.ReplyID
	}
	if s.realtimeTurn.liveText == "" {
		currentText := s.realtimeTurn.replyText
		s.realtimeMu.Unlock()
		s.send(WSMessage{
			Type:    WSMessageTypeAssistantTranscriptPartial,
			Content: currentText,
			Data: wsAssistantTranscriptPayload{
				Text:       currentText,
				IsFinal:    false,
				QuestionID: response.QuestionID,
				ReplyID:    response.ReplyID,
			},
		})
		return
	}
	s.realtimeMu.Unlock()
}

// handleRealtimeChatQueryConfirmed 记录服务端确认的 question_id，便于排查首轮文本 query 是否真正进入模型链路。
func (s *wsInterviewSession) handleRealtimeChatQueryConfirmed(payload []byte) {
	var response realtimeChatQueryConfirmedPayload
	if err := json.Unmarshal(payload, &response); err != nil {
		return
	}
	if strings.TrimSpace(response.QuestionID) == "" {
		return
	}

	s.realtimeMu.Lock()
	s.realtimeTurn.questionID = response.QuestionID
	s.realtimeMu.Unlock()

	applogger.Info("realtime chat text query confirmed",
		zap.String("trace_id", s.traceID),
		zap.Uint("interview_id", s.interviewID),
		zap.String("question_id", response.QuestionID),
	)
}

// markRealtimeTurnAudioEnded 标记当前播报音频已结束，并尝试落地当前面试官回复。
func (s *wsInterviewSession) markRealtimeTurnAudioEnded() {
	s.realtimeMu.Lock()
	s.realtimeTurn.audioEnded = true
	if strings.TrimSpace(s.realtimeTurn.replyText) == "" && strings.TrimSpace(s.realtimeTurn.liveText) != "" {
		s.realtimeTurn.textEnded = true
	}
	s.realtimeMu.Unlock()
	s.finalizeRealtimeAssistantTurn(false)
}

// markRealtimeTurnTextEnded 标记当前模型文本回复已结束，并尝试落地当前面试官回复。
func (s *wsInterviewSession) markRealtimeTurnTextEnded() {
	s.realtimeMu.Lock()
	s.realtimeTurn.textEnded = true
	questionID := s.realtimeTurn.questionID
	replyID := s.realtimeTurn.replyID
	s.realtimeMu.Unlock()
	applogger.Info("realtime assistant chat stream ended",
		zap.String("trace_id", s.traceID),
		zap.Uint("interview_id", s.interviewID),
		zap.String("question_id", questionID),
		zap.String("reply_id", replyID),
	)
	s.finalizeRealtimeAssistantTurn(false)
}

// finalizeRealtimeAssistantTurn 在一轮播报结束或被用户打断时，把最终回复写入持久化并推送最终事件。
func (s *wsInterviewSession) finalizeRealtimeAssistantTurn(force bool) {
	s.realtimeMu.Lock()
	if !force {
		if !s.realtimeTurn.audioEnded {
			s.realtimeMu.Unlock()
			return
		}
		// 如果已经收到了 chat 文本流，则继续等待 ChatEnded；否则允许仅凭 TTS 字幕 + TTSEnded 收口。
		if strings.TrimSpace(s.realtimeTurn.replyText) != "" && !s.realtimeTurn.textEnded {
			s.realtimeMu.Unlock()
			return
		}
	}

	finalText := strings.TrimSpace(s.realtimeTurn.replyText)
	if finalText == "" {
		finalText = strings.TrimSpace(s.realtimeTurn.liveText)
	}
	questionID := s.realtimeTurn.questionID
	replyID := s.realtimeTurn.replyID
	s.realtimeTurn = realtimeTurnState{}
	s.realtimeMu.Unlock()

	if finalText == "" {
		return
	}
	applogger.Info("realtime assistant turn finalized",
		zap.String("trace_id", s.traceID),
		zap.Uint("interview_id", s.interviewID),
		zap.String("final_text", finalText),
	)

	question, questionNo, finished, err := s.handler.interviewService.AppendRealtimeAssistantReply(context.Background(), s.userID, s.interviewID, finalText)
	if err != nil {
		s.sendError("保存实时面试官回复失败: " + err.Error())
		return
	}

	if finished {
		_, finishErr := s.handler.interviewService.FinishInterview(context.Background(), s.userID, s.interviewID)
		if finishErr != nil {
			applogger.Warn("auto-finish resume_driven interview failed",
				zap.String("trace_id", s.traceID),
				zap.Uint("interview_id", s.interviewID),
				zap.Error(finishErr),
			)
		}
		s.send(WSMessage{
			Type:    WSMessageTypeFinished,
			Content: "面试已结束，正在生成报告。",
		})
	}

	s.send(WSMessage{
		Type:    WSMessageTypeAssistantTranscriptFinal,
		Content: finalText,
		Data: wsAssistantTranscriptPayload{
			Text:       finalText,
			IsFinal:    true,
			QuestionID: questionID,
			ReplyID:    replyID,
		},
	})

	payload := wsAssistantTurnPayload{
		Text:       finalText,
		QuestionNo: questionNo,
		IsQuestion: question != nil,
	}
	if question != nil {
		payload.Live2DDirective = question.Live2DDirective
		s.sendDirective(question.Live2DDirective)
	}
	s.send(WSMessage{
		Type:    WSMessageTypeAssistantTurnFinished,
		Content: finalText,
		Data:    payload,
	})

	s.sendState("ready", "面试官播报完成，可继续作答。")
}

// buildRealtimeSystemRole 构造实时模型整场面试要遵守的固定系统提示词。
func (s *wsInterviewSession) buildRealtimeSystemRole(ctx *service.RealtimeInterviewContext) string {
	if ctx != nil && ctx.InterviewMode == "resume_driven" {
		return buildResumeDrivenSystemPrompt(ctx.ResumeProfile, safeRealtimeIndustryCode(ctx))
	}

	topics := "通用技术能力"
	if ctx != nil && len(ctx.Topics) > 0 {
		topics = strings.Join(ctx.Topics, "、")
	}

	lines := []string{
		s.handler.realtimeConfig.SystemRole,
		fmt.Sprintf("你正在进行一场中文技术模拟面试，目标方向是 %s。", firstNonEmpty(safeRealtimeIndustryCode(ctx), "通用方向")),
		fmt.Sprintf("整场面试共 %d 题，目标难度为 %s，优先覆盖这些主题：%s。", safeRealtimeQuestionCount(ctx), safeRealtimeDifficulty(ctx), topics),
	}

	if ctx != nil && len(ctx.WeakTopics) > 0 {
		lines = append(lines, fmt.Sprintf("用户近期高频薄弱点：%s。至少 1-2 道题目围绕这些薄弱点出题，帮助用户验证是否已克服。", strings.Join(ctx.WeakTopics, "、")))
	}
	lines = append(lines,
		"你必须一次只问一个问题，用户回答后先给一句简短反馈，再自然进入下一题。",
		"到最后一题回答完成后，请只给简短总结，不要继续追问。",
		"请始终使用自然口语中文，不要输出 Markdown、列表标题或代码块。",
	)
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// buildResumeDrivenSystemPrompt 根据简历画像生成简历驱动面试模式的完整系统提示词。
func buildResumeDrivenSystemPrompt(profile *ai.ResumeProfile, industryCode string) string {
	industryLabel := firstNonEmpty(industryCode, "通用方向")

	var sb strings.Builder
	fmt.Fprintf(&sb, "你是一位专业、敏锐、耐心的技术面试官。\n你正在主持一场针对 %s 的面试。\n\n", industryLabel)

	sb.WriteString("## 面试总原则\n")
	sb.WriteString("1. 自然对话，而非问答列表：不要提及\"第X题\"、\"共N题\"等计数语言，不要像考试一样按顺序提问，要像真正的面试官一样进行对话。\n")
	sb.WriteString("2. 阶段驱动，而非计数驱动：整场面试按阶段推进（破冰→项目深挖→技术基础→开放题→结束），每个阶段根据候选人回答质量动态决定深度和轮数。\n")
	sb.WriteString("3. 简历即线索，追问即核心：简历中提到的每段经历、每项技术都必须被追问验证，而不是泛泛而谈。追问深度取决于候选人回答的真实性。\n")
	sb.WriteString("4. 一次只问一个问题：等候选人回答完毕后，根据回答质量决定是追问细节还是切换话题。\n\n")

	sb.WriteString("## 面试阶段规划\n\n")

	sb.WriteString("### 阶段 1：破冰与自我介绍（1 轮）\n")
	sb.WriteString("- 用一句友好的开场白提及候选人的核心经历，然后请候选人做自我介绍。\n")
	sb.WriteString("- 观察候选人的表达结构、逻辑性、重点选择。\n\n")

	sb.WriteString("### 阶段 2：项目深挖与真实性验证（3-5 轮追问）\n")
	sb.WriteString("- 从简历中最核心的项目切入，依次追问：\n")
	sb.WriteString("  - 项目的背景和你的具体职责\n")
	sb.WriteString("  - 技术选型的原因（为什么用X而不用Y）\n")
	sb.WriteString("  - 遇到的最大技术挑战及解决方案\n")
	sb.WriteString("  - 项目成果的量化指标（性能提升、用户量、稳定性等）\n")
	sb.WriteString("- 如果候选人回答模糊或回避细节，追问具体实现，验证是否真正参与。\n")
	sb.WriteString("- 如果回答清晰有深度，可以快速过渡到下一个项目或技术点。\n\n")

	sb.WriteString("### 阶段 3：技术基础的情景化考察（2-3 轮）\n")
	sb.WriteString("- 结合候选人简历中的技术栈，设计情景化问题而非纯八股文。\n")
	sb.WriteString("- 例如：\"你在项目中用了Redis缓存，能说说你们的缓存失效策略是怎么设计的吗？遇到过缓存穿透的问题吗？\"\n")
	sb.WriteString("- 追问方向：原理理解 → 实际应用场景 → 边界条件和异常处理。\n\n")

	sb.WriteString("### 阶段 4：工程素养与开放题（1-2 轮）\n")
	sb.WriteString("- 问一个开放性问题，考察候选人的工程思维和学习能力。\n")
	sb.WriteString("- 例如：\"如果让你重新做这个项目，你会在架构上做哪些改变？\"或\"你最近关注的技术趋势是什么？\"\n")
	sb.WriteString("- 观察候选人的思考深度、技术视野、自我反思能力。\n\n")

	sb.WriteString("### 阶段 5：结束与候选人提问（1 轮）\n")
	sb.WriteString("- 简要总结面试亮点，然后问候选人：\"你有什么想问我的吗？\"\n")
	sb.WriteString("- 无论候选人是否提问，都友好结束面试。\n\n")

	sb.WriteString("## 追问决策引擎\n")
	sb.WriteString("- 回答具体且有深度 → 给予肯定，快速进入下一个话题\n")
	sb.WriteString("- 回答模糊但方向正确 → 追问细节，引导候选人展开\n")
	sb.WriteString("- 回答明显错误 → 温和指出，给候选人补充机会\n")
	sb.WriteString("- 回答过于简短 → 追问\"能再具体说说吗？\"或\"能举个例子吗？\"\n")
	sb.WriteString("- 回答明显背诵痕迹 → 追问实际应用和变体场景\n\n")

	sb.WriteString("## 绝对禁止的行为\n")
	sb.WriteString("- 禁止提及\"第X题\"、\"共N题\"、\"让我们进入下一题\"等考试化语言\n")
	sb.WriteString("- 禁止一次性抛出多个问题\n")
	sb.WriteString("- 禁止跳过自我介绍阶段直接问技术题\n")
	sb.WriteString("- 禁止忽略简历内容而问泛泛的八股文\n")
	sb.WriteString("- 禁止在候选人回答后不给任何反馈就直接问下一个问题\n")
	sb.WriteString("- 禁止使用 Markdown、列表标题或代码块格式\n\n")

	if profile != nil {
		sb.WriteString("## 简历数据\n")
		if s := strings.TrimSpace(profile.Summary); s != "" {
			fmt.Fprintf(&sb, "候选人背景：%s\n", s)
		}
		if len(profile.Skills) > 0 {
			fmt.Fprintf(&sb, "核心技术栈：%s\n", strings.Join(profile.Skills, "、"))
		}
		if len(profile.Projects) > 0 {
			fmt.Fprintf(&sb, "重点项目经历：%s\n", strings.Join(profile.Projects, "；"))
		}
		if len(profile.Strengths) > 0 {
			fmt.Fprintf(&sb, "简历优势：%s\n", strings.Join(profile.Strengths, "、"))
		}
		if len(profile.WeakSignals) > 0 {
			fmt.Fprintf(&sb, "简历薄弱信号：%s（请重点追问验证）\n", strings.Join(profile.WeakSignals, "、"))
		}
	}

	return sb.String()
}

// buildRealtimeKickoffPrompt 生成进入第一题前主动唤起模型开场的文本指令。
func (s *wsInterviewSession) buildRealtimeKickoffPrompt(ctx *service.RealtimeInterviewContext) string {
	if ctx != nil && ctx.InterviewMode == "resume_driven" && ctx.ResumeProfile != nil {
		summary := strings.TrimSpace(ctx.ResumeProfile.Summary)
		if summary != "" {
			return fmt.Sprintf("现在开始这场基于候选人简历的技术面试。候选人背景：%s。请用一句友好的开场白提及候选人的核心经历，然后请候选人做自我介绍。", summary)
		}
		return "现在开始这场基于候选人简历的技术面试。请用一句友好的开场白，然后请候选人做自我介绍。"
	}
	return fmt.Sprintf("现在开始这场中文技术面试。请先用一句简短开场白，然后直接提出第 1 道问题。整场共 %d 题。", safeRealtimeQuestionCount(ctx))
}

func safeRealtimeIndustryCode(ctx *service.RealtimeInterviewContext) string {
	if ctx == nil {
		return ""
	}
	return strings.TrimSpace(ctx.IndustryCode)
}

func safeRealtimeDifficulty(ctx *service.RealtimeInterviewContext) string {
	if ctx == nil || strings.TrimSpace(ctx.Difficulty) == "" {
		return "mixed"
	}
	return strings.TrimSpace(ctx.Difficulty)
}

func safeRealtimeQuestionCount(ctx *service.RealtimeInterviewContext) int {
	if ctx == nil || ctx.TotalQuestions <= 0 {
		return 5
	}
	return ctx.TotalQuestions
}

func appendRealtimeTextChunk(current string, next string) string {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if next == "" {
		return current
	}
	if current == "" {
		return next
	}
	if strings.Contains(current, next) {
		return current
	}
	return current + next
}
