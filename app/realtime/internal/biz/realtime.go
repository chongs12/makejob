package biz

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/gorilla/websocket"

	"makejob/app/realtime/internal/conf"
)

// ========== 错误定义 ==========

var (
	ErrInterviewNotRealtime = errors.New(400, "INTERVIEW_NOT_REALTIME", "面试不是实时模式")
	ErrSessionNotFound      = errors.New(404, "SESSION_NOT_FOUND", "会话不存在")
	ErrSessionNotActive     = errors.New(400, "SESSION_NOT_ACTIVE", "会话不活跃")
	ErrVolcConnection       = errors.New(500, "VOLC_CONNECTION_ERROR", "火山引擎连接失败")
)

// ========== 领域实体 ==========

// Session 实时会话持久化实体
type Session struct {
	ID          string `gorm:"primaryKey;size:64"`
	InterviewID uint64 `gorm:"index"`
	UserID      uint64 `gorm:"index"`
	Status      string `gorm:"size:20;not null;default:'active'"`
}

// RealtimeContext 实时面试上下文（对齐单体 RealtimeInterviewContext）
type RealtimeContext struct {
	InterviewID           uint64
	IndustryCode          string
	Live2DModelKey        string
	TotalQuestions        int
	AskedQuestionCount    int
	AnsweredQuestionCount int
	Difficulty            string
	Topics                []string
	WeakTopics            []string
	InterviewMode         string
	ResumeProfile         *ResumeProfile
	DialogID              string
	HasStarted            bool
	CurrentTopic          string
}

// ResumeProfile 简历画像（对齐单体 ai.ResumeProfile）
type ResumeProfile struct {
	Summary     string   `json:"summary"`
	Skills      []string `json:"skills"`
	Projects    []string `json:"projects"`
	Strengths   []string `json:"strengths"`
	WeakSignals []string `json:"weak_signals"`
}

// NextQuestionMeta 下一题元数据
type NextQuestionMeta struct {
	QuestionIndex  int32
	IsLastQuestion bool
}

// RAGDocument RAG 检索文档
type RAGDocument struct {
	ID      string
	Content string
	Score   float64
}

// RealtimeSession 运行时会话状态（非持久化）
type RealtimeSession struct {
	SessionID    string
	InterviewID  uint64
	userID       uint64
	RTCtx        *RealtimeContext
	ClientConn   *websocket.Conn
	VolcConn     VolcEngineConn
	Cancel       context.CancelFunc
	LastActivity time.Time
	mu           sync.Mutex
	turnState    realtimeTurnState
	sender       *wsSender
	ctx          context.Context // 保存 HTTP 请求 context（含 token），供 gRPC 调用透传
}

// ========== 仓库接口 ==========

// RealtimeRepo 实时会话仓库接口，data 层必须实现
type RealtimeRepo interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	UpdateSessionStatus(ctx context.Context, sessionID string, status string) error
}

// ========== 外部服务客户端接口 ==========

// InterviewClient Interview 服务客户端接口
type InterviewClient interface {
	IsRealtimeInterview(ctx context.Context, interviewID uint64) (bool, error)
	GetRealtimeContext(ctx context.Context, interviewID uint64) (*RealtimeContext, error)
	BindRealtimeDialog(ctx context.Context, interviewID uint64, dialogID string) error
	AppendRealtimeUserAnswer(ctx context.Context, interviewID uint64, content string) error
	AppendRealtimeAssistantReply(ctx context.Context, interviewID uint64, content string) (*NextQuestionMeta, error)
	FinishInterview(ctx context.Context, interviewID uint64) error
}

// RAGClient RAG 服务客户端接口
type RAGClient interface {
	Retrieve(ctx context.Context, query string, topK int32) ([]*RAGDocument, error)
}

// VolcEngineConn 火山引擎 WebSocket 连接抽象
type VolcEngineConn interface {
	SendAudio(data []byte) error
	ReadEvent() (*VolcEvent, error)
	InjectContext(text string) error
	Close() error
}

// VolcEvent 火山引擎事件
type VolcEvent struct {
	Type    int
	Payload []byte
}

// VolcEngineFactory 火山引擎连接工厂函数类型
type VolcEngineFactory func(ctx context.Context) (VolcEngineConn, error)

// VolcStartOptions 启动实时语音会话的配置
type VolcStartOptions struct {
	SessionID       string
	DialogID        string
	BotName         string
	SystemRole      string
	SpeakingStyle   string
	CharacterPrompt string
	LocationCity    string
	InputMode       string
	RecvTimeout     int
	Speaker         string
}

// VolcEngineSessionConn 火山引擎会话连接接口（扩展 VolcEngineConn）
type VolcEngineSessionConn interface {
	VolcEngineConn
	SendTextQuery(text string) error
	SendSayHello(text string) error
	SendEndASR() error
	SendChatTTSText(content string) error
	SendChatRAGText(externalRAG string) error
	DialogID() string
	Events() <-chan VolcEvent
}

// VolcEngineSessionFactory 火山引擎会话工厂函数类型
type VolcEngineSessionFactory func(ctx context.Context, opts VolcStartOptions) (VolcEngineSessionConn, error)

// SessionManager 会话管理器，管理所有活跃的实时会话
type SessionManager struct {
	sessions sync.Map
}

// NewSessionManager 创建会话管理器
func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

// Store 存储活跃会话
func (m *SessionManager) Store(sessionID string, session *RealtimeSession) {
	m.sessions.Store(sessionID, session)
}

// Load 加载活跃会话
func (m *SessionManager) Load(sessionID string) (*RealtimeSession, bool) {
	val, ok := m.sessions.Load(sessionID)
	if !ok {
		return nil, false
	}
	return val.(*RealtimeSession), true
}

// Delete 删除会话
func (m *SessionManager) Delete(sessionID string) {
	m.sessions.Delete(sessionID)
}

// RealtimeUseCase 实时会话业务用例
type RealtimeUseCase struct {
	interview         InterviewClient
	rag               RAGClient
	smgr              *SessionManager
	volcFactory       VolcEngineFactory
	volcSessionFactory VolcEngineSessionFactory
	volcCfg           *conf.Volcengine
	log               *log.Helper
}

// NewRealtimeUseCase 创建实时会话业务用例
func NewRealtimeUseCase(interview InterviewClient, rag RAGClient, smgr *SessionManager, volcFactory VolcEngineFactory, volcSessionFactory VolcEngineSessionFactory, volcCfg *conf.Volcengine, logger log.Logger) *RealtimeUseCase {
	return &RealtimeUseCase{
		interview:          interview,
		rag:                rag,
		smgr:               smgr,
		volcFactory:        volcFactory,
		volcSessionFactory: volcSessionFactory,
		volcCfg:            volcCfg,
		log:                log.NewHelper(logger),
	}
}

// InitSession 初始化实时面试会话（对齐单体：纯内存管理，不落库）
func (uc *RealtimeUseCase) InitSession(_ context.Context, interviewID, userID uint64) (*Session, error) {
	session := &Session{
		ID:          fmt.Sprintf("rt_%d_%d", interviewID, time.Now().UnixNano()),
		InterviewID: interviewID,
		UserID:      userID,
		Status:      "pending",
	}
	return session, nil
}

// GetSession 查询会话状态（对齐单体：仅从内存管理器获取）
func (uc *RealtimeUseCase) GetSession(_ context.Context, sessionID string) (*Session, error) {
	if rt, ok := uc.smgr.Load(sessionID); ok {
		return &Session{
			ID:          rt.SessionID,
			InterviewID: rt.InterviewID,
			UserID:      rt.userID,
			Status:      "active",
		}, nil
	}
	return nil, ErrSessionNotFound
}

// InjectRAGContext 向指定会话注入 RAG 上下文
func (uc *RealtimeUseCase) InjectRAGContext(ctx context.Context, sessionID, ragContext string) error {
	session, ok := uc.smgr.Load(sessionID)
	if !ok {
		return ErrSessionNotFound
	}
	if err := session.VolcConn.InjectContext(ragContext); err != nil {
		return errors.InternalServer("INJECT_RAG_FAILED", "注入 RAG 上下文失败: "+err.Error())
	}
	return nil
}

// EndSession 结束指定会话，触发清理流程（对齐单体：纯内存管理）
func (uc *RealtimeUseCase) EndSession(_ context.Context, sessionID string) error {
	session, ok := uc.smgr.Load(sessionID)
	if !ok {
		return ErrSessionNotFound
	}
	session.Cancel()
	uc.smgr.Delete(sessionID)
	return nil
}

// prepareSession 创建内存会话（对齐单体：不落库，纯内存管理）
func (uc *RealtimeUseCase) prepareSession(_ context.Context, interviewID, userID uint64, _ string) (*Session, error) {
	session := &Session{
		ID:          fmt.Sprintf("rt_%d_%d", interviewID, time.Now().UnixNano()),
		InterviewID: interviewID,
		UserID:      userID,
		Status:      "active",
	}
	return session, nil
}

// HandleSession 处理实时 WebSocket 会话，桥接客户端音频流与火山引擎语音服务
func (uc *RealtimeUseCase) HandleSession(ctx context.Context, interviewID uint64, userID uint64, sessionID string, clientConn *websocket.Conn) {
	defer clientConn.Close()

	sender := newWSSender(clientConn, interviewID)

	// 0. 发送连接成功事件（对齐单体）
	_ = sender.sendConnected()

	// 1. 获取实时面试上下文
	rtCtx, err := uc.interview.GetRealtimeContext(ctx, interviewID)
	if err != nil {
		uc.log.Errorf("获取实时面试上下文失败: interview_id=%d, err=%v", interviewID, err)
		_ = sender.sendError("获取面试上下文失败: " + err.Error())
		return
	}

	// 2. 复用预创建会话，若没有则回退为即时创建。
	session, err := uc.prepareSession(ctx, interviewID, userID, sessionID)
	if err != nil {
		uc.log.Errorf("准备实时会话失败: %v", err)
		_ = sender.sendError("创建会话失败")
		return
	}

	// 3. 绑定实时对话 ID 到 Interview 服务
	if err := uc.interview.BindRealtimeDialog(ctx, interviewID, session.ID); err != nil {
		uc.log.Errorf("绑定实时对话失败: interview_id=%d, err=%v", interviewID, err)
		_ = sender.sendError("绑定对话失败")
		return
	}

	// 4. 构建系统提示词并连接火山引擎（对齐单体 bootstrapRealtime）
	systemRole := uc.buildRealtimeSystemRole(rtCtx, uc.volcCfg)
	kickoffPrompt := uc.buildRealtimeKickoffPrompt(rtCtx)

	// 使用 session-based factory 启动火山引擎会话（含系统提示词注入）
	volcSessionConn, err := uc.volcSessionFactory(ctx, VolcStartOptions{
		SessionID:       session.ID,
		DialogID:        rtCtx.DialogID,
		BotName:         uc.volcCfg.BotName,
		SystemRole:      systemRole,
		SpeakingStyle:   uc.volcCfg.SpeakingStyle,
		CharacterPrompt: uc.volcCfg.CharacterPrompt,
		LocationCity:    uc.volcCfg.LocationCity,
		InputMode:       uc.volcCfg.InputMode,
		RecvTimeout:     uc.volcCfg.RecvTimeout,
		Speaker:         uc.volcCfg.Speaker,
	})
	if err != nil {
		uc.log.Errorf("连接火山引擎失败: %v", err)
		_ = sender.sendState("error", "实时语音会话启动失败，请检查火山实时语音配置和凭证。")
		_ = sender.sendError("连接语音服务失败")
		return
	}
	volcConn := volcSessionConn.(VolcEngineConn)
	dialogID := volcSessionConn.DialogID()
	defer volcConn.Close()

	// 绑定 dialog ID
	if strings.TrimSpace(dialogID) != "" && dialogID != rtCtx.DialogID {
		if bindErr := uc.interview.BindRealtimeDialog(ctx, interviewID, dialogID); bindErr != nil {
			uc.log.Warnf("绑定 dialog ID 失败: %v", bindErr)
		}
	}

	// 5. 发送会话就绪事件（对齐单体）
	_ = sender.sendSessionReady()

	// 6. 创建会话上下文，确保所有 goroutine 可被取消
	sessionCtx, sessionCancel := context.WithCancel(ctx)
	defer sessionCancel()
	go func() {
		<-sessionCtx.Done()
		_ = clientConn.Close()
		_ = volcConn.Close()
	}()

	// 7. 注册到会话管理器，附加 sender 和 ctx
	rtSession := &RealtimeSession{
		SessionID:    session.ID,
		InterviewID:  interviewID,
		userID:       userID,
		RTCtx:        rtCtx,
		ClientConn:   clientConn,
		VolcConn:     volcConn,
		Cancel:       sessionCancel,
		LastActivity: time.Now(),
		sender:       sender,
		ctx:          ctx,
	}
	uc.smgr.Store(session.ID, rtSession)
	defer uc.smgr.Delete(session.ID)
	defer func() {
		if err := uc.interview.FinishInterview(ctx, interviewID); err != nil {
			uc.log.Warnf("结束实时面试失败: interview_id=%d err=%v", interviewID, err)
		}
	}()

	// 8. 启动事件消费 goroutine
	go func() {
		if sessionConn, ok := volcConn.(VolcEngineSessionConn); ok {
			for event := range sessionConn.Events() {
				uc.handleVolcEvent(sessionCtx, rtSession, event)
			}
		}
	}()

	// 9. 发送开场白（对齐单体 bootstrapRealtime）
	if rtCtx.HasStarted {
		_ = sender.sendState("ready", "已恢复当前实时面试，可直接继续作答。")
	} else {
		_ = sender.sendState("speaking", "面试官正在准备第一题。")
		if sessionConn, ok := volcConn.(VolcEngineSessionConn); ok {
			if err := sessionConn.SendTextQuery(kickoffPrompt); err != nil {
				uc.log.Errorf("发送开场白失败: %v", err)
			}
		}
	}

	// 10. 启动客户端音频转发和 RAG 注入 goroutine
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		uc.clientToVolc(sessionCtx, rtSession)
	}()

	wg.Wait()
	uc.log.Infof("实时会话结束: session_id=%s", session.ID)
}

// clientToVolc 读取客户端音频帧，转发到火山引擎
func (uc *RealtimeUseCase) clientToVolc(ctx context.Context, session *RealtimeSession) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgType, data, err := session.ClientConn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				uc.log.Infof("客户端正常断开: session_id=%s", session.SessionID)
			} else {
				uc.log.Errorf("读取客户端消息失败: session_id=%s, err=%v", session.SessionID, err)
			}
			session.Cancel()
			return
		}

		session.LastActivity = time.Now()

		switch msgType {
		case websocket.BinaryMessage:
			// 音频数据转发到火山引擎
			if err := session.VolcConn.SendAudio(data); err != nil {
				uc.log.Errorf("发送音频到火山引擎失败: %v", err)
				session.Cancel()
				return
			}
		case websocket.TextMessage:
			// 客户端控制指令
			uc.handleClientControl(ctx, session, data)
		}
	}
}

// injectRAGForSession 为单次 RAG 注入执行检索和上下文写入（对齐单体 injectRAGContext）
func (uc *RealtimeUseCase) injectRAGForSession(ctx context.Context, session *RealtimeSession, userAnswer string) {
	query := strings.TrimSpace(userAnswer + " " + session.RTCtx.IndustryCode)
	if query == "" {
		query = fmt.Sprintf("%s %s 面试", session.RTCtx.IndustryCode, session.RTCtx.Difficulty)
	}
	docs, err := uc.rag.Retrieve(ctx, query, 3)
	if err != nil {
		uc.log.Errorf("RAG 检索失败: session_id=%s, err=%v", session.SessionID, err)
		return
	}
	if len(docs) == 0 {
		return
	}

	// 格式化为 external_rag 格式
	var ragItems []map[string]string
	for _, doc := range docs {
		ragItems = append(ragItems, map[string]string{
			"title":   "参考知识",
			"content": doc.Content,
		})
	}
	ragJSON, _ := json.Marshal(ragItems)
	if string(ragJSON) == "[]" {
		return
	}

	// 使用扩展接口发送安抚话术 + RAG 数据
	if extConn, ok := session.VolcConn.(interface {
		SendChatTTSText(string) error
		SendChatRAGText(string) error
	}); ok {
		// 发送安抚话术
		if err := extConn.SendChatTTSText("让我想想..."); err != nil {
			uc.log.Warnf("发送安抚话术失败: %v", err)
		}
		// 注入 RAG 数据
		if err := extConn.SendChatRAGText(string(ragJSON)); err != nil {
			uc.log.Errorf("注入 RAG 数据失败: %v", err)
		}
	} else {
		// 回退到旧接口
		if err := session.VolcConn.InjectContext(string(ragJSON)); err != nil {
			uc.log.Errorf("注入 RAG 上下文失败: session_id=%s, err=%v", session.SessionID, err)
		}
	}
}

// realtimeTurnState 实时面试单轮状态
type realtimeTurnState struct {
	userFinalText string
	liveText      string
	replyText     string
	questionID    string
	replyID       string
	audioEnded    bool
	textEnded     bool
}

// handleASRResponseEvent 处理 ASR 识别文本片段（事件451）
func (uc *RealtimeUseCase) handleASRResponseEvent(_ context.Context, session *RealtimeSession, payload []byte) {
	var response struct {
		Results []struct {
			Text      string `json:"text"`
			IsInterim bool   `json:"is_interim"`
		} `json:"results"`
	}
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

	// 缓存最终识别结果
	session.mu.Lock()
	session.turnState.userFinalText = text
	session.mu.Unlock()

	if lastResult.IsInterim {
		_ = session.sender.sendASRPartial(text)
	} else {
		_ = session.sender.sendASRFinal(text)
	}
}

// handleASREndedEvent 处理 ASR 识别结束（事件459），保存用户回答并触发 RAG 注入
func (uc *RealtimeUseCase) handleASREndedEvent(ctx context.Context, session *RealtimeSession) {
	session.mu.Lock()
	text := strings.TrimSpace(session.turnState.userFinalText)
	session.turnState.userFinalText = ""
	session.mu.Unlock()

	if text == "" {
		_ = session.sender.sendState("ready", "本轮未识别到有效回答，请重新开始。")
		return
	}

	if err := uc.interview.AppendRealtimeUserAnswer(ctx, session.InterviewID, text); err != nil {
		uc.log.Errorf("保存用户回答失败: %v", err)
	}
	_ = session.sender.sendUserAnswer(text)

	// 事件驱动 RAG 注入（对齐单体 injectRAGContext：ASR 结束后、AI 回复前注入参考知识）
	// 异步执行，不阻塞事件循环；设置 3 秒超时避免 RAG 慢响应影响语音流畅度
	go func() {
		ragCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		uc.injectRAGForSession(ragCtx, session, text)
	}()

	_ = session.sender.sendState("thinking", "AI 面试官正在整理你的回答。")
}

// handleTTSSentenceStartEvent 处理 TTS 字幕开始（事件350）
func (uc *RealtimeUseCase) handleTTSSentenceStartEvent(session *RealtimeSession, payload []byte) {
	var response struct {
		Text       string `json:"text"`
		QuestionID string `json:"question_id"`
		ReplyID    string `json:"reply_id"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return
	}
	text := strings.TrimSpace(response.Text)
	if text == "" {
		return
	}

	session.mu.Lock()
	if response.QuestionID != "" {
		session.turnState.questionID = response.QuestionID
	}
	if response.ReplyID != "" {
		session.turnState.replyID = response.ReplyID
	}
	session.turnState.liveText = appendRealtimeTextChunk(session.turnState.liveText, text)
	currentText := session.turnState.liveText
	session.mu.Unlock()

	_ = session.sender.sendAssistantTranscriptPartial(currentText, session.turnState.questionID, session.turnState.replyID)
}

// handleTTSSentenceEndEvent 处理 TTS 字幕结束（事件351）
func (uc *RealtimeUseCase) handleTTSSentenceEndEvent(session *RealtimeSession, payload []byte) {
	var response struct {
		QuestionID string `json:"question_id"`
		ReplyID    string `json:"reply_id"`
	}
	if err := json.Unmarshal(payload, &response); err == nil {
		session.mu.Lock()
		if response.QuestionID != "" {
			session.turnState.questionID = response.QuestionID
		}
		if response.ReplyID != "" {
			session.turnState.replyID = response.ReplyID
		}
		session.mu.Unlock()
	}

	session.mu.Lock()
	if strings.TrimSpace(session.turnState.replyText) == "" && strings.TrimSpace(session.turnState.liveText) != "" {
		session.turnState.textEnded = true
	}
	session.mu.Unlock()
	uc.finalizeRealtimeAssistantTurn(session, false)
}

// handleTTSAudioEvent 处理 TTS 音频数据块（事件352），base64 编码后发送（对齐单体）
func (uc *RealtimeUseCase) handleTTSAudioEvent(session *RealtimeSession, payload []byte) {
	if len(payload) == 0 {
		return
	}
	ttsFormat := firstNonEmpty(uc.volcCfg.TTSFormat, "pcm_s16le")
	ttsSampleRate := uc.volcCfg.TTSSampleRate
	if ttsSampleRate <= 0 {
		ttsSampleRate = 24000
	}
	_ = session.sender.sendAssistantAudioChunk(payload, ttsFormat, ttsSampleRate)
}

// handleTTSEndedEvent 处理 TTS 播报结束（事件359）
func (uc *RealtimeUseCase) handleTTSEndedEvent(ctx context.Context, session *RealtimeSession) {
	session.mu.Lock()
	session.turnState.audioEnded = true
	if strings.TrimSpace(session.turnState.replyText) == "" && strings.TrimSpace(session.turnState.liveText) != "" {
		session.turnState.textEnded = true
	}
	session.mu.Unlock()
	uc.finalizeRealtimeAssistantTurn(session, false)
}

// handleChatQueryConfirmedEvent 处理 Chat query 确认（事件553）
func (uc *RealtimeUseCase) handleChatQueryConfirmedEvent(session *RealtimeSession, payload []byte) {
	var response struct {
		QuestionID string `json:"question_id"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return
	}
	if strings.TrimSpace(response.QuestionID) == "" {
		return
	}
	session.mu.Lock()
	session.turnState.questionID = response.QuestionID
	session.mu.Unlock()
}

// handleChatResponseEvent 处理 Chat 文本回复（事件550）
func (uc *RealtimeUseCase) handleChatResponseEvent(session *RealtimeSession, payload []byte) {
	var response struct {
		Content    string `json:"content"`
		QuestionID string `json:"question_id"`
		ReplyID    string `json:"reply_id"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return
	}
	text := strings.TrimSpace(response.Content)
	if text == "" {
		return
	}

	session.mu.Lock()
	session.turnState.replyText = appendRealtimeTextChunk(session.turnState.replyText, text)
	if response.QuestionID != "" {
		session.turnState.questionID = response.QuestionID
	}
	if response.ReplyID != "" {
		session.turnState.replyID = response.ReplyID
	}
	// 如果没有 TTS 字幕流，直接推送部分文本
	if session.turnState.liveText == "" {
		currentText := session.turnState.replyText
		session.mu.Unlock()
		_ = session.sender.sendAssistantTranscriptPartial(currentText, session.turnState.questionID, session.turnState.replyID)
		return
	}
	session.mu.Unlock()
}

// handleChatEndedEvent 处理 Chat 回复结束（事件559）
func (uc *RealtimeUseCase) handleChatEndedEvent(ctx context.Context, session *RealtimeSession) {
	session.mu.Lock()
	session.turnState.textEnded = true
	session.mu.Unlock()
	uc.finalizeRealtimeAssistantTurn(session, false)
}

// finalizeRealtimeAssistantTurn 在一轮播报结束或被用户打断时，把最终回复写入持久化（对齐单体 finalizeRealtimeAssistantTurn）
func (uc *RealtimeUseCase) finalizeRealtimeAssistantTurn(session *RealtimeSession, force bool) {
	session.mu.Lock()
	if !force {
		if !session.turnState.audioEnded {
			session.mu.Unlock()
			return
		}
		if strings.TrimSpace(session.turnState.replyText) != "" && !session.turnState.textEnded {
			session.mu.Unlock()
			return
		}
	}

	finalText := strings.TrimSpace(session.turnState.replyText)
	if finalText == "" {
		finalText = strings.TrimSpace(session.turnState.liveText)
	}
	questionID := session.turnState.questionID
	replyID := session.turnState.replyID
	session.turnState = realtimeTurnState{}
	session.mu.Unlock()

	if finalText == "" {
		return
	}

	// 保存 AI 回复（使用 session.ctx，包含 token）
	meta, err := uc.interview.AppendRealtimeAssistantReply(session.ctx, session.InterviewID, finalText)
	if err != nil {
		uc.log.Errorf("保存 AI 回复失败: %v", err)
		return
	}

	// 推送最终字幕（对齐单体）
	_ = session.sender.sendAssistantTranscriptFinal(finalText, questionID, replyID)

	// 推送轮次完成（对齐单体）
	_ = session.sender.sendAssistantTurnFinished(finalText, 0, meta != nil)

	// 如果是最后一题，结束面试
	if meta != nil && meta.IsLastQuestion {
		_ = session.sender.sendFinished("面试已结束，正在生成报告。")
		session.Cancel()
	}

	_ = session.sender.sendState("ready", "面试官播报完成，可继续作答。")
}

// appendRealtimeTextChunk 拼接实时文本块（对齐单体 appendRealtimeTextChunk）
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

// handleClientControl 处理客户端控制指令（音频转发、结束面试等）
func (uc *RealtimeUseCase) handleClientControl(_ context.Context, session *RealtimeSession, payload []byte) {
	var msg struct {
		Type string `json:"type"`
		Data *struct {
			AudioBase64 string `json:"audio_base64"`
		} `json:"data,omitempty"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		uc.log.Warnf("解析客户端控制消息失败: %v", err)
		return
	}

	switch msg.Type {
	case "audio_start":
		uc.log.Infof("客户端开始录音: session_id=%s", session.SessionID)

	case "audio_chunk":
		// 解码 base64 音频并转发给火山引擎
		if msg.Data == nil || msg.Data.AudioBase64 == "" {
			return
		}
		audioBytes, err := base64.StdEncoding.DecodeString(msg.Data.AudioBase64)
		if err != nil {
			uc.log.Warnf("解码音频数据失败: session_id=%s, err=%v", session.SessionID, err)
			return
		}
		if err := session.VolcConn.SendAudio(audioBytes); err != nil {
			uc.log.Errorf("转发音频到火山引擎失败: session_id=%s, err=%v", session.SessionID, err)
		}

	case "audio_end":
		// 通知火山引擎用户语音输入结束（push_to_talk 模式需要显式 EndASR）
		if extConn, ok := session.VolcConn.(VolcEngineSessionConn); ok {
			if err := extConn.SendEndASR(); err != nil {
				uc.log.Warnf("发送 EndASR 失败: session_id=%s, err=%v", session.SessionID, err)
			}
		}

	case "end_interview":
		uc.log.Infof("客户端请求结束面试: session_id=%s", session.SessionID)
		session.Cancel()
	}
}

// handleVolcEvent 处理火山引擎事件（对齐单体 consumeRealtimeEvents）
func (uc *RealtimeUseCase) handleVolcEvent(ctx context.Context, session *RealtimeSession, event VolcEvent) {
	const (
		EventSessionFinished        = 152
		EventTTSSentenceStart       = 350
		EventTTSSentenceEnd         = 351
		EventTTSResponse            = 352
		EventTTSEnded               = 359
		EventASRInfo                = 450
		EventASRResponse            = 451
		EventASREnded               = 459
		EventChatResponse           = 550
		EventChatTextQueryConfirmed = 553
		EventChatEnded              = 559
	)

	switch event.Type {
	case EventASRInfo:
		_ = session.sender.sendBargeIn()
		_ = session.sender.sendState("listening", "检测到你开始说话，已切到收听状态。")
	case EventASRResponse:
		uc.handleASRResponseEvent(ctx, session, event.Payload)
	case EventASREnded:
		uc.handleASREndedEvent(ctx, session)
	case EventTTSSentenceStart:
		uc.handleTTSSentenceStartEvent(session, event.Payload)
	case EventTTSSentenceEnd:
		uc.handleTTSSentenceEndEvent(session, event.Payload)
	case EventTTSResponse:
		uc.handleTTSAudioEvent(session, event.Payload)
	case EventTTSEnded:
		uc.handleTTSEndedEvent(ctx, session)
	case EventChatTextQueryConfirmed:
		uc.handleChatQueryConfirmedEvent(session, event.Payload)
	case EventChatResponse:
		uc.handleChatResponseEvent(session, event.Payload)
	case EventChatEnded:
		uc.handleChatEndedEvent(ctx, session)
	case EventSessionFinished:
		_ = session.sender.sendState("ready", "实时会话已结束。")
		session.Cancel()
	}
}

// buildRealtimeSystemRole 构造实时模型整场面试要遵守的固定系统提示词（对齐单体）
func (uc *RealtimeUseCase) buildRealtimeSystemRole(ctx *RealtimeContext, cfg *conf.Volcengine) string {
	if ctx != nil && ctx.InterviewMode == "resume_driven" && ctx.ResumeProfile != nil {
		return buildResumeDrivenSystemPrompt(ctx.ResumeProfile, safeStr(ctx.IndustryCode))
	}

	topics := "通用技术能力"
	if ctx != nil && len(ctx.Topics) > 0 {
		topics = strings.Join(ctx.Topics, "、")
	}

	lines := []string{
		cfg.SystemRole,
		fmt.Sprintf("你正在进行一场中文技术模拟面试，目标方向是 %s。", firstNonEmpty(safeStr(ctx.IndustryCode), "通用方向")),
		fmt.Sprintf("整场面试共 %d 题，目标难度为 %s，优先覆盖这些主题：%s。", safeQuestionCount(ctx), safeDifficulty(ctx), topics),
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

// buildResumeDrivenSystemPrompt 根据简历画像生成简历驱动面试模式的完整系统提示词（对齐单体）
func buildResumeDrivenSystemPrompt(profile *ResumeProfile, industryCode string) string {
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
	sb.WriteString("- 例如：\"如果让你重新做这个项目，你会在架构上做哪些改变？\"或\"你最近关注的技术趋势是什么？\"\n\n")
	sb.WriteString("### 阶段 5：结束与候选人提问（1 轮）\n")
	sb.WriteString("- 简要总结面试亮点，然后问候选人：\"你有什么想问我的吗？\"\n\n")

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

// buildRealtimeKickoffPrompt 生成进入第一题前主动唤起模型开场的文本指令（对齐单体）
func (uc *RealtimeUseCase) buildRealtimeKickoffPrompt(ctx *RealtimeContext) string {
	if ctx != nil && ctx.InterviewMode == "resume_driven" && ctx.ResumeProfile != nil {
		summary := strings.TrimSpace(ctx.ResumeProfile.Summary)
		if summary != "" {
			return fmt.Sprintf("现在开始这场基于候选人简历的技术面试。候选人背景：%s。请用一句友好的开场白提及候选人的核心经历，然后请候选人做自我介绍。", summary)
		}
		return "现在开始这场基于候选人简历的技术面试。请用一句友好的开场白，然后请候选人做自我介绍。"
	}
	return fmt.Sprintf("现在开始这场中文技术面试。请先用一句简短开场白，然后直接提出第 1 道问题。整场共 %d 题。", safeQuestionCount(ctx))
}

func safeStr(s string) string {
	return strings.TrimSpace(s)
}

func safeDifficulty(ctx *RealtimeContext) string {
	if ctx == nil || strings.TrimSpace(ctx.Difficulty) == "" {
		return "mixed"
	}
	return strings.TrimSpace(ctx.Difficulty)
}

func safeQuestionCount(ctx *RealtimeContext) int {
	if ctx == nil || ctx.TotalQuestions <= 0 {
		return 5
	}
	return ctx.TotalQuestions
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
