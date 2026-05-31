// Package handler 提供HTTP请求处理层
package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/asr"
	"makejob-backend/internal/common"
	appconfig "makejob-backend/internal/config"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/rag"
	realtimevolc "makejob-backend/internal/realtime/volcengine"
	"makejob-backend/internal/service"
	"makejob-backend/internal/tts"
	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
)

// InterviewHandler 面试处理器
type InterviewHandler struct {
	interviewService service.InterviewService
	ttsSceneService  service.SceneTTSService
	ttsProvider      tts.TTSProvider
	asrProvider      asr.ASRProvider
	realtimeConfig   appconfig.VolcRealtimeDialogConfig
	ragService       *rag.InterviewRAGService
}

// NewInterviewHandler 创建面试处理器实例
func NewInterviewHandler(
	svc service.InterviewService,
	ttsSceneService service.SceneTTSService,
	ttsProvider tts.TTSProvider,
	asrProvider asr.ASRProvider,
	realtimeConfig appconfig.VolcRealtimeDialogConfig,
	ragService *rag.InterviewRAGService,
) *InterviewHandler {
	return &InterviewHandler{
		interviewService: svc,
		ttsSceneService:  ttsSceneService,
		ttsProvider:      ttsProvider,
		asrProvider:      asrProvider,
		realtimeConfig:   realtimeConfig,
		ragService:       ragService,
	}
}

// RegisterRoutes 注册面试相关路由
func (h *InterviewHandler) RegisterRoutes(protected *gin.RouterGroup) {
	interviews := protected.Group("/interviews")
	{
		interviews.POST("", h.CreateInterview)
		interviews.GET("", h.ListInterviews)
		interviews.GET("/:id", h.GetInterview)
		interviews.POST("/:id/answer", h.SubmitAnswer)
		interviews.GET("/:id/next", h.GetNextQuestion)
		interviews.POST("/:id/finish", h.FinishInterview)
		interviews.GET("/:id/report", h.GetReport)
		interviews.GET("/:id/ws", h.WebSocket)
	}
}

// CreateInterview 创建面试会话
// @Summary 创建面试会话
// @Description 创建新的模拟面试会话
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.CreateInterviewRequest true "面试配置"
// @Success 200 {object} common.Response{data=service.InterviewResponse}
// @Failure 400 {object} common.Response
// @Router /api/interviews [post]
func (h *InterviewHandler) CreateInterview(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	var req service.CreateInterviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.interviewService.CreateInterview(c.Request.Context(), userID, &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建面试失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// ListInterviews 获取面试列表
// @Summary 获取面试列表
// @Description 获取当前用户的面试历史列表
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} common.Response{data=common.PageResult}
// @Failure 401 {object} common.Response
// @Router /api/interviews [get]
func (h *InterviewHandler) ListInterviews(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	pageParam := common.ReadPageParam(c)

	result, err := h.interviewService.ListInterviews(c.Request.Context(), userID, pageParam.Page, pageParam.PageSize)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取面试列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// GetInterview 获取面试详情
// @Summary 获取面试详情
// @Description 获取指定面试的详细信息
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "面试ID"
// @Success 200 {object} common.Response{data=service.InterviewDetailResponse}
// @Failure 404 {object} common.Response
// @Router /api/interviews/{id} [get]
func (h *InterviewHandler) GetInterview(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	interviewID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的面试ID")
		return
	}

	resp, err := h.interviewService.GetInterview(c.Request.Context(), userID, uint(interviewID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取面试详情失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// SubmitAnswer 提交回答
// @Summary 提交面试回答
// @Description 提交当前题目的回答，并由系统直接推进到下一题
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "面试ID"
// @Param request body service.InterviewAnswerRequest true "回答内容"
// @Success 200 {object} common.Response{data=service.InterviewAnswerResponse}
// @Failure 400 {object} common.Response
// @Router /api/interviews/{id}/answer [post]
func (h *InterviewHandler) SubmitAnswer(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	interviewID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的面试ID")
		return
	}

	var req service.InterviewAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.interviewService.SubmitAnswer(c.Request.Context(), userID, uint(interviewID), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "提交回答失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// GetNextQuestion 获取下一题
// @Summary 获取下一道面试题
// @Description 手动获取下一道面试题目
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "面试ID"
// @Success 200 {object} common.Response{data=service.NextQuestionResponse}
// @Failure 400 {object} common.Response
// @Router /api/interviews/{id}/next [get]
func (h *InterviewHandler) GetNextQuestion(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	interviewID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的面试ID")
		return
	}

	resp, err := h.interviewService.GetNextQuestion(c.Request.Context(), userID, uint(interviewID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取下一题失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// FinishInterview 结束面试
// @Summary 结束面试
// @Description 结束面试并生成报告
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "面试ID"
// @Success 200 {object} common.Response{data=service.InterviewReportResponse}
// @Failure 400 {object} common.Response
// @Router /api/interviews/{id}/finish [post]
func (h *InterviewHandler) FinishInterview(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	interviewID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的面试ID")
		return
	}

	resp, err := h.interviewService.FinishInterview(c.Request.Context(), userID, uint(interviewID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "结束面试失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// GetReport 获取面试报告
// @Summary 获取面试报告
// @Description 获取已结束面试的详细报告
// @Tags 面试
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "面试ID"
// @Success 200 {object} common.Response{data=service.InterviewReportResponse}
// @Failure 404 {object} common.Response
// @Router /api/interviews/{id}/report [get]
func (h *InterviewHandler) GetReport(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	interviewID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的面试ID")
		return
	}

	resp, err := h.interviewService.GetReport(c.Request.Context(), userID, uint(interviewID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取面试报告失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// WebSocket消息类型定义。
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
	WSMessageTypeAudioStart                 WSMessageType = "audio_start"
	WSMessageTypeAudioChunk                 WSMessageType = "audio_chunk"
	WSMessageTypeAudioEnd                   WSMessageType = "audio_end"
	WSMessageTypePing                       WSMessageType = "ping"
	WSMessageTypeError                      WSMessageType = "error"
	WSMessageTypeFinished                   WSMessageType = "finished"
)

// WSMessage 描述服务端推送给前端的统一 WebSocket 事件。
type WSMessage struct {
	Type        WSMessageType `json:"type"`
	Content     string        `json:"content,omitempty"`
	Data        interface{}   `json:"data,omitempty"`
	Timestamp   int64         `json:"timestamp"`
	TraceID     string        `json:"trace_id,omitempty"`
	InterviewID uint          `json:"interview_id,omitempty"`
}

// wsClientMessage 描述前端发给后端的 WebSocket 事件。
type wsClientMessage struct {
	Type    WSMessageType    `json:"type"`
	Content string           `json:"content,omitempty"`
	Data    *json.RawMessage `json:"data,omitempty"`
}

// summarizeWSClientMessage 为前端上行 WebSocket 消息生成一条适合写日志的摘要。
func summarizeWSClientMessage(msg wsClientMessage) []zap.Field {
	fields := []zap.Field{
		zap.String("type", string(msg.Type)),
		zap.Int("content_length", len(strings.TrimSpace(msg.Content))),
	}
	if msg.Data == nil || len(*msg.Data) == 0 {
		return fields
	}

	fields = append(fields, zap.Int("data_bytes", len(*msg.Data)))
	if msg.Type != WSMessageTypeAudioChunk {
		return fields
	}

	var payload wsAudioChunkPayload
	if err := json.Unmarshal(*msg.Data, &payload); err != nil {
		return append(fields, zap.String("audio_payload", "unmarshal_failed"))
	}
	return append(fields, zap.Int("audio_base64_length", len(strings.TrimSpace(payload.AudioBase64))))
}

// wsInterviewStatePayload 描述当前会话阶段状态。
type wsInterviewStatePayload struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Mode    string `json:"mode,omitempty"`
}

// wsAudioStartPayload 描述一轮语音识别的启动参数。
type wsAudioStartPayload struct {
	Language string `json:"language"`
	Engine   string `json:"engine,omitempty"`
}

// wsAudioChunkPayload 描述浏览器上传的一段 PCM 音频。
type wsAudioChunkPayload struct {
	AudioBase64 string `json:"audio_base64"`
}

// wsASRPayload 描述流式语音识别文本片段。
type wsASRPayload struct {
	Text       string  `json:"text"`
	IsFinal    bool    `json:"is_final"`
	Confidence float64 `json:"confidence"`
}

// wsQuestionPayload 描述当前题目和题号。
type wsQuestionPayload struct {
	Question        string              `json:"question"`
	QuestionNo      int                 `json:"question_no"`
	Type            string              `json:"type"`
	Hints           string              `json:"hints,omitempty"`
	Language        string              `json:"language,omitempty"`
	StarterCode     string              `json:"starter_code,omitempty"`
	EditorMode      string              `json:"editor_mode,omitempty"`
	EvaluationMode  string              `json:"evaluation_mode,omitempty"`
	Live2DDirective *ai.Live2DDirective `json:"live2d_directive,omitempty"`
}

// wsTTSAudioPayload 描述面试官当前播报文本对应的语音资源。
type wsTTSAudioPayload struct {
	Kind       string  `json:"kind"`
	Text       string  `json:"text"`
	AudioURL   string  `json:"audio_url"`
	Duration   float64 `json:"duration"`
	Format     string  `json:"format"`
	SampleRate int     `json:"sample_rate"`
}

// wsAssistantTranscriptPayload 描述实时模型当前播报中的字幕文本片段。
type wsAssistantTranscriptPayload struct {
	Text       string `json:"text"`
	IsFinal    bool   `json:"is_final"`
	QuestionID string `json:"question_id,omitempty"`
	ReplyID    string `json:"reply_id,omitempty"`
}

// wsAssistantAudioChunkPayload 描述实时模型返回的一段 PCM 音频块。
type wsAssistantAudioChunkPayload struct {
	AudioBase64 string `json:"audio_base64"`
	Format      string `json:"format"`
	SampleRate  int    `json:"sample_rate"`
}

// wsAssistantTurnPayload 描述一轮面试官播报结束后的最终文本和题目元数据。
type wsAssistantTurnPayload struct {
	Text            string              `json:"text"`
	QuestionNo      int                 `json:"question_no"`
	IsQuestion      bool                `json:"is_question"`
	Live2DDirective *ai.Live2DDirective `json:"live2d_directive,omitempty"`
}

// wsLive2DExpressionPayload 描述前端应切换到的表情状态。
type wsLive2DExpressionPayload struct {
	Emotion            string                       `json:"emotion"`
	Action             string                       `json:"action"`
	Source             string                       `json:"source"`
	ExpressionMix      []ai.Live2DExpressionLayer   `json:"expression_mix,omitempty"`
	ParameterOverrides []ai.Live2DParameterOverride `json:"parameter_overrides,omitempty"`
	Intensity          float64                      `json:"intensity,omitempty"`
	DurationMS         int                          `json:"duration_ms,omitempty"`
	MouthOpen          *float64                     `json:"mouth_open,omitempty"`
}

// wsInterviewSession 管理单个面试连接的实时状态与写入串行化。
type wsInterviewSession struct {
	handler          *InterviewHandler
	conn             *websocket.Conn
	userID           uint
	interviewID      uint
	live2DModelKey   string
	traceID          string
	realtimeMode     bool
	writeMu          sync.Mutex
	asrMu            sync.Mutex
	asrStream        asr.StreamSession
	asrLanguage      string
	asrEngine        string
	latestTranscript string
	realtimeClient   *realtimevolc.Client
	realtimeContext  *service.RealtimeInterviewContext
	realtimeMu       sync.Mutex
	realtimeTurn     realtimeTurnState
	ragService       *rag.InterviewRAGService
}

type realtimeTurnState struct {
	liveText            string
	replyText           string
	questionID          string
	replyID             string
	audioEnded          bool
	textEnded           bool
	userFinalText       string
	userAudioChunkCount int
}

// WebSocket upgrader配置。
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WebSocket 提供实时面试通信链路，并补齐题目、语音和表情事件。
// @Summary WebSocket实时面试通信
// @Description 用于面试过程中的实时消息推送，支持发送答案、语音片段和接收AI回复
// @Tags 面试
// @Security Bearer
// @Param id path int true "面试ID"
// @Router /api/interviews/{id}/ws [get]
func (h *InterviewHandler) WebSocket(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, common.Response{
			Code:    common.CodeUnauthorized,
			Message: "未登录",
		})
		return
	}

	interviewID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.Response{
			Code:    common.CodeBadRequest,
			Message: "无效的面试ID",
		})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		applogger.Error("WebSocket升级失败", zap.Error(err))
		return
	}
	applogger.Info("interview websocket connected",
		zap.Uint("user_id", userID),
		zap.Uint64("interview_id", interviewID),
	)

	session := &wsInterviewSession{
		handler:     h,
		conn:        conn,
		userID:      userID,
		interviewID: uint(interviewID),
		traceID:     uuid.NewString(),
		asrLanguage: "zh-CN",
		ragService:  h.ragService,
	}
	defer session.close()

	session.send(WSMessage{
		Type:    WSMessageTypeConnected,
		Content: "实时面试连接已建立",
	})
	session.sendState("ready", "面试会话已连接，正在恢复当前题目。")

	session.bootstrap()

	for {
		var msg wsClientMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				applogger.Error("WebSocket读取错误", zap.Error(err), zap.String("trace_id", session.traceID))
			}
			return
		}
		if msg.Type != WSMessageTypeAudioChunk {
			fields := append([]zap.Field{
				zap.String("trace_id", session.traceID),
				zap.Uint("interview_id", session.interviewID),
			}, summarizeWSClientMessage(msg)...)
			applogger.Info("interview websocket client message received", fields...)
		}

		switch msg.Type {
		case WSMessageTypeUserAnswer:
			session.handleUserAnswer(strings.TrimSpace(msg.Content))
		case WSMessageTypeAudioStart:
			session.handleAudioStart(msg.Data)
		case WSMessageTypeAudioChunk:
			session.handleAudioChunk(msg.Data)
		case WSMessageTypeAudioEnd:
			session.handleAudioEnd()
		case WSMessageTypePing:
			session.sendState("ready", "心跳已收到，实时链路保持活跃。")
		default:
			session.sendError("未知的消息类型: " + string(msg.Type))
		}
	}
}

// bootstrap 恢复会话当前状态，并补发未完成题目和初始表情。
func (s *wsInterviewSession) bootstrap() {
	isRealtimeInterview, err := s.handler.interviewService.IsRealtimeInterview(context.Background(), s.userID, s.interviewID)
	if err != nil {
		s.sendError("恢复面试模式失败: " + err.Error())
		return
	}
	s.realtimeMode = isRealtimeInterview
	applogger.Info("interview websocket bootstrap mode resolved",
		zap.String("trace_id", s.traceID),
		zap.Uint("user_id", s.userID),
		zap.Uint("interview_id", s.interviewID),
		zap.Bool("realtime_mode", s.realtimeMode),
	)
	if s.realtimeMode {
		s.bootstrapRealtime()
		return
	}

	detail, err := s.handler.interviewService.GetInterview(context.Background(), s.userID, s.interviewID)
	if err != nil {
		s.sendError("恢复面试详情失败: " + err.Error())
		return
	}

	s.send(WSMessage{
		Type: WSMessageTypeSessionReady,
		Data: wsInterviewStatePayload{
			Status:  detail.Status,
			Message: "当前面试文本链路已就绪。",
			Mode:    "http",
		},
	})
	s.live2DModelKey = strings.TrimSpace(detail.Live2DModelKey)
	s.sendDirective(nil)

	question, questionNo, ok := resolveCurrentInterviewQuestion(detail)
	if !ok {
		if detail.Status == "completed" {
			s.sendState("finished", "本场面试已结束，可直接查看报告。")
			s.send(WSMessage{
				Type:    WSMessageTypeFinished,
				Content: "面试已完成",
			})
			return
		}
		s.sendState("ready", "当前没有待回答题目，可继续作答或结束面试。")
		return
	}

	s.sendQuestion(question, questionNo)
}

// handleUserAnswer 处理用户提交的文本答案，并在成功后直接推进下一题。
func (s *wsInterviewSession) handleUserAnswer(answer string) {
	if s.realtimeMode {
		s.handleRealtimeUserAnswer(answer)
		return
	}

	if answer == "" {
		s.sendError("回答内容不能为空")
		return
	}

	s.closeASRSession()
	s.sendState("thinking", "AI 正在整理你的回答并准备下一题。")

	resp, err := s.handler.interviewService.SubmitAnswer(context.Background(), s.userID, s.interviewID, &service.InterviewAnswerRequest{
		Answer: answer,
	})
	if err != nil {
		s.sendError(err.Error())
		return
	}

	if resp.NextQuestion != nil {
		s.sendQuestion(*resp.NextQuestion, 0)
	}

	if resp.IsFinished {
		s.sendState("finished", "本场面试已完成，可以生成最终报告。")
		s.sendDirective(nil)
		s.send(WSMessage{
			Type:    WSMessageTypeFinished,
			Content: "面试已完成",
		})
		return
	}

	s.sendState("ready", "下一题已准备好，可继续作答。")
}

// handleAudioStart 启动一轮流式语音识别，并把文本片段持续推送给前端。
func (s *wsInterviewSession) handleAudioStart(rawData *json.RawMessage) {
	if s.realtimeMode {
		s.handleRealtimeAudioStart(rawData)
		return
	}

	if s.handler.asrProvider == nil {
		s.sendError("ASR 服务未配置，当前无法使用语音识别。")
		return
	}

	payload := wsAudioStartPayload{
		Language: "zh-CN",
	}
	if rawData != nil && len(*rawData) > 0 {
		if err := json.Unmarshal(*rawData, &payload); err != nil {
			s.sendError("语音识别启动参数错误: " + err.Error())
			return
		}
	}
	if strings.TrimSpace(payload.Language) == "" {
		payload.Language = "zh-CN"
	}

	s.closeASRSession()

	stream, err := s.handler.asrProvider.StartStream(context.Background(), strings.TrimSpace(payload.Engine), strings.TrimSpace(payload.Language))
	if err != nil {
		s.sendError("启动语音识别失败: " + err.Error())
		return
	}

	s.asrMu.Lock()
	s.asrStream = stream
	s.asrLanguage = payload.Language
	s.asrEngine = payload.Engine
	s.latestTranscript = ""
	s.asrMu.Unlock()

	s.sendState("listening", "正在实时识别你的回答，请继续说。")
	go s.consumeASRResults(stream)
}

// handleAudioChunk 接收并转发浏览器上传的 PCM 音频块。
func (s *wsInterviewSession) handleAudioChunk(rawData *json.RawMessage) {
	if s.realtimeMode {
		s.handleRealtimeAudioChunk(rawData)
		return
	}

	if rawData == nil || len(*rawData) == 0 {
		s.sendError("缺少语音音频数据")
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

	s.asrMu.Lock()
	stream := s.asrStream
	s.asrMu.Unlock()
	if stream == nil {
		s.sendError("语音识别会话尚未启动")
		return
	}
	if err := stream.SendAudio(chunk); err != nil {
		s.sendError("发送语音音频失败: " + err.Error())
		return
	}
}

// handleAudioEnd 结束当前识别会话，并在拿到有效转写后自动提交本轮语音回答。
func (s *wsInterviewSession) handleAudioEnd() {
	if s.realtimeMode {
		s.handleRealtimeAudioEnd()
		return
	}

	s.asrMu.Lock()
	recognizedText := strings.TrimSpace(s.latestTranscript)
	s.asrMu.Unlock()
	s.closeASRSession()

	if recognizedText == "" {
		s.sendState("ready", "本轮未识别到有效回答，请重新开始语音作答或手动输入。")
		return
	}

	s.send(WSMessage{
		Type:    WSMessageTypeUserAnswer,
		Content: recognizedText,
	})
	s.handleUserAnswer(recognizedText)
}

// consumeASRResults 持续消费 ASR 结果并推送 partial/final 事件。
func (s *wsInterviewSession) consumeASRResults(stream asr.StreamSession) {
	for result := range stream.ReceiveText() {
		if strings.TrimSpace(result.Text) == "" {
			continue
		}

		msgType := WSMessageTypeASRPartial
		s.asrMu.Lock()
		s.latestTranscript = result.Text
		s.asrMu.Unlock()
		if result.IsFinal {
			msgType = WSMessageTypeASRFinal
		}

		s.send(WSMessage{
			Type:    msgType,
			Content: result.Text,
			Data: wsASRPayload{
				Text:       result.Text,
				IsFinal:    result.IsFinal,
				Confidence: result.Confidence,
			},
		})
	}
}

// sendQuestion 推送当前题目、面试状态、表情以及对应语音。
func (s *wsInterviewSession) sendQuestion(question ai.InterviewQuestion, questionNo int) {
	s.sendDirective(question.Live2DDirective)
	s.send(WSMessage{
		Type:    WSMessageTypeAIQuestion,
		Content: question.Question,
		Data: wsQuestionPayload{
			Question:        question.Question,
			QuestionNo:      questionNo,
			Type:            firstNonEmpty(question.Type, "technical"),
			Hints:           question.Hints,
			Language:        question.Language,
			StarterCode:     question.StarterCode,
			EditorMode:      question.EditorMode,
			EvaluationMode:  question.EvaluationMode,
			Live2DDirective: question.Live2DDirective,
		},
	})
	s.sendState("speaking", "面试官正在播报当前题目。")
	s.synthesizeSpeech(question.Question, "question")
}

// synthesizeSpeech 为当前文本生成语音资源，并在成功后推送给前端播放。
func (s *wsInterviewSession) synthesizeSpeech(text string, kind string) {
	if strings.TrimSpace(text) == "" {
		return
	}

	if s.handler.ttsSceneService != nil {
		result, err := s.handler.ttsSceneService.SynthesizeForScene(context.Background(), service.SceneTTSRequest{
			Scene:          "interview",
			Live2DModelKey: s.live2DModelKey,
			Text:           text,
		})
		if err == nil {
			s.send(WSMessage{
				Type: WSMessageTypeTTSAudio,
				Data: wsTTSAudioPayload{
					Kind:       kind,
					Text:       text,
					AudioURL:   result.AudioURL,
					Duration:   result.Duration,
					Format:     result.Format,
					SampleRate: result.SampleRate,
				},
			})
			return
		}
		applogger.Warn("interview scene tts failed and will fallback",
			zap.String("trace_id", s.traceID),
			zap.String("live2d_model_key", s.live2DModelKey),
			zap.String("kind", kind),
			zap.Error(err),
		)
	}

	if s.handler.ttsProvider == nil {
		if s.handler.ttsProvider == nil {
			applogger.Warn("interview tts skipped because provider is nil",
				zap.String("trace_id", s.traceID),
				zap.String("kind", kind),
			)
		}
		return
	}

	engine := resolveTTSEngine(s.handler.ttsProvider)
	applogger.Info("interview tts requested",
		zap.String("trace_id", s.traceID),
		zap.String("kind", kind),
		zap.String("engine", engine),
		zap.Int("text_length", len([]rune(strings.TrimSpace(text)))),
	)
	result, err := s.handler.ttsProvider.Synthesize(context.Background(), tts.SynthesizeRequest{
		Text:   text,
		Engine: engine,
	})
	if err != nil {
		s.sendState("ready", "TTS 当前不可用，已自动回退到文本模式。")
		applogger.Warn("interview tts failed", zap.Error(err), zap.String("trace_id", s.traceID))
		return
	}

	s.send(WSMessage{
		Type: WSMessageTypeTTSAudio,
		Data: wsTTSAudioPayload{
			Kind:       kind,
			Text:       text,
			AudioURL:   result.AudioURL,
			Duration:   result.Duration,
			Format:     result.Format,
			SampleRate: result.SampleRate,
		},
	})
	applogger.Info("interview tts pushed to client",
		zap.String("trace_id", s.traceID),
		zap.String("kind", kind),
		zap.String("format", result.Format),
		zap.Float64("duration", result.Duration),
	)
}

// resolveTTSEngine 返回当前 TTS Provider 首选的引擎标识。
func resolveTTSEngine(provider tts.TTSProvider) string {
	if provider == nil {
		return ""
	}
	supportedEngines := provider.GetSupportedEngines()
	if len(supportedEngines) == 0 {
		return ""
	}
	return strings.TrimSpace(supportedEngines[0])
}

// sendDirective 推送当前应展示的 Live2D 结构化状态，缺省时回退到中性待机。
func (s *wsInterviewSession) sendDirective(directive *ai.Live2DDirective) {
	payload := wsLive2DExpressionPayload{
		Emotion: "neutral",
		Action:  "idle",
		Source:  "fallback",
	}
	if directive != nil {
		if strings.TrimSpace(directive.Emotion) != "" {
			payload.Emotion = strings.TrimSpace(directive.Emotion)
		}
		if strings.TrimSpace(directive.Action) != "" {
			payload.Action = strings.TrimSpace(directive.Action)
		}
		if strings.TrimSpace(directive.Source) != "" {
			payload.Source = strings.TrimSpace(directive.Source)
		}
		payload.ExpressionMix = directive.ExpressionMix
		payload.ParameterOverrides = directive.ParameterOverrides
		payload.Intensity = directive.Intensity
		payload.DurationMS = directive.DurationMS
		payload.MouthOpen = directive.MouthOpen
	}
	s.send(WSMessage{
		Type: WSMessageTypeLive2DExpression,
		Data: payload,
	})
}

// sendState 推送当前面试链路所处的阶段状态。
func (s *wsInterviewSession) sendState(status string, message string) {
	mode := "http"
	if s.realtimeMode {
		mode = "realtime"
	}
	s.send(WSMessage{
		Type:    WSMessageTypeInterviewState,
		Content: message,
		Data: wsInterviewStatePayload{
			Status:  status,
			Message: message,
			Mode:    mode,
		},
	})
}

// sendError 推送错误事件，并在服务端记录对应 trace 信息。
func (s *wsInterviewSession) sendError(message string) {
	s.send(WSMessage{
		Type:    WSMessageTypeError,
		Content: message,
	})
	applogger.Warn("interview websocket error", zap.String("trace_id", s.traceID), zap.String("message", message))
}

// send 统一写入 WebSocket 事件，并保证 trace_id 与 interview_id 始终透传。
func (s *wsInterviewSession) send(msg WSMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().Unix()
	}
	if msg.TraceID == "" {
		msg.TraceID = s.traceID
	}
	if msg.InterviewID == 0 {
		msg.InterviewID = s.interviewID
	}

	if err := s.conn.WriteJSON(msg); err != nil {
		applogger.Error("WebSocket写入错误", zap.Error(err), zap.String("trace_id", s.traceID))
		return err
	}
	return nil
}

// close 释放当前连接占用的识别会话和底层 WebSocket。
func (s *wsInterviewSession) close() {
	if s.realtimeClient != nil {
		_ = s.realtimeClient.Close()
		s.realtimeClient = nil
	}
	s.closeASRSession()
	_ = s.conn.Close()
	applogger.Info("interview websocket closed",
		zap.String("trace_id", s.traceID),
		zap.Uint("user_id", s.userID),
		zap.Uint("interview_id", s.interviewID),
	)
}

// closeASRSession 关闭当前流式识别会话，避免同一连接残留旧流。
func (s *wsInterviewSession) closeASRSession() {
	s.asrMu.Lock()
	defer s.asrMu.Unlock()

	if s.asrStream == nil {
		return
	}
	if err := s.asrStream.Close(); err != nil {
		applogger.Warn("close interview asr stream failed", zap.Error(err), zap.String("trace_id", s.traceID))
	}
	s.asrStream = nil
}

// resolveCurrentInterviewQuestion 从历史消息中恢复当前仍待回答的题目。
func resolveCurrentInterviewQuestion(detail *service.InterviewDetailResponse) (ai.InterviewQuestion, int, bool) {
	if detail == nil || detail.Status != "ongoing" {
		return ai.InterviewQuestion{}, 0, false
	}

	if detail.CurrentQuestion != nil {
		answerCount := 0
		for _, item := range detail.Messages {
			if item.Role == "user" {
				answerCount++
			}
		}
		return *detail.CurrentQuestion, answerCount + 1, true
	}

	answerCount := 0
	for _, item := range detail.Messages {
		if item.Role == "user" {
			answerCount++
		}
	}
	if answerCount >= detail.TotalQuestions {
		return ai.InterviewQuestion{}, 0, false
	}

	for i := len(detail.Messages) - 1; i >= 0; i-- {
		item := detail.Messages[i]
		if item.Role != "ai" || item.MessageType != "text" || strings.TrimSpace(item.Content) == "" {
			continue
		}

		return ai.InterviewQuestion{
			Question: item.Content,
			Type:     "technical",
		}, answerCount + 1, true
	}

	return ai.InterviewQuestion{}, 0, false
}

// firstNonEmpty 返回第一个非空字符串，用于兜底事件字段值。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
