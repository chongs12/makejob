package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/gorilla/websocket"
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

// RealtimeContext 实时面试上下文
type RealtimeContext struct {
	Resume               string
	JD                   string
	Industry             string
	QuestionCount        int32
	Difficulty           string
	CurrentQuestionIndex int32
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
	UserID       uint64
	RTCtx        *RealtimeContext
	ClientConn   *websocket.Conn
	VolcConn     VolcEngineConn
	Cancel       context.CancelFunc
	LastActivity time.Time
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
	repo        RealtimeRepo
	interview   InterviewClient
	rag         RAGClient
	smgr        *SessionManager
	volcFactory VolcEngineFactory
	log         *log.Helper
}

// NewRealtimeUseCase 创建实时会话业务用例
func NewRealtimeUseCase(repo RealtimeRepo, interview InterviewClient, rag RAGClient, smgr *SessionManager, volcFactory VolcEngineFactory, logger log.Logger) *RealtimeUseCase {
	return &RealtimeUseCase{
		repo:        repo,
		interview:   interview,
		rag:         rag,
		smgr:        smgr,
		volcFactory: volcFactory,
		log:         log.NewHelper(logger),
	}
}

// InitSession 初始化实时面试会话，创建持久化记录并返回会话信息
func (uc *RealtimeUseCase) InitSession(ctx context.Context, interviewID, userID uint64) (*Session, error) {
	// 验证面试是否为实时模式
	isRealtime, err := uc.interview.IsRealtimeInterview(ctx, interviewID)
	if err != nil {
		return nil, errors.InternalServer("INTERVIEW_CHECK_FAILED", "检查面试模式失败: "+err.Error())
	}
	if !isRealtime {
		return nil, ErrInterviewNotRealtime
	}

	// 创建会话记录
	session := &Session{
		ID:          fmt.Sprintf("rt_%d_%d", interviewID, time.Now().UnixNano()),
		InterviewID: interviewID,
		UserID:      userID,
		Status:      "pending",
	}
	if err := uc.repo.CreateSession(ctx, session); err != nil {
		return nil, errors.InternalServer("CREATE_SESSION_FAILED", "创建会话失败: "+err.Error())
	}
	return session, nil
}

// GetSession 查询会话状态（优先从内存管理器获取活跃会话）
func (uc *RealtimeUseCase) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	// 优先查询活跃会话
	if rt, ok := uc.smgr.Load(sessionID); ok {
		return &Session{
			ID:          rt.SessionID,
			InterviewID: rt.InterviewID,
			UserID:      rt.UserID,
			Status:      "active",
		}, nil
	}
	// 回退到持久化存储
	return uc.repo.GetSession(ctx, sessionID)
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

// EndSession 结束指定会话，触发清理流程
func (uc *RealtimeUseCase) EndSession(ctx context.Context, sessionID string) error {
	session, ok := uc.smgr.Load(sessionID)
	if !ok {
		return ErrSessionNotFound
	}
	session.Cancel()
	uc.smgr.Delete(sessionID)
	// 更新持久化状态
	if err := uc.repo.UpdateSessionStatus(ctx, sessionID, "ended"); err != nil {
		uc.log.Errorf("更新会话状态失败: %v", err)
	}
	return nil
}

// prepareSession 优先复用预创建的 pending 会话，没有可用会话时回退为即时创建。
func (uc *RealtimeUseCase) prepareSession(ctx context.Context, interviewID, userID uint64, sessionID string) (*Session, error) {
	if sessionID != "" {
		session, err := uc.repo.GetSession(ctx, sessionID)
		if err == nil && session.InterviewID == interviewID && session.UserID == userID && session.Status == "pending" {
			if updateErr := uc.repo.UpdateSessionStatus(ctx, session.ID, "active"); updateErr == nil {
				session.Status = "active"
				return session, nil
			}
		}
	}
	session := &Session{
		ID:          fmt.Sprintf("rt_%d_%d", interviewID, time.Now().UnixNano()),
		InterviewID: interviewID,
		UserID:      userID,
		Status:      "active",
	}
	if err := uc.repo.CreateSession(ctx, session); err != nil {
		return nil, errors.InternalServer("CREATE_SESSION_FAILED", "创建会话失败: "+err.Error())
	}
	return session, nil
}

// HandleSession 处理实时 WebSocket 会话，桥接客户端音频流与火山引擎语音服务
func (uc *RealtimeUseCase) HandleSession(ctx context.Context, interviewID uint64, userID uint64, sessionID string, clientConn *websocket.Conn) {
	defer clientConn.Close()

	// 1. 获取实时面试上下文
	rtCtx, err := uc.interview.GetRealtimeContext(ctx, interviewID)
	if err != nil {
		uc.log.Errorf("获取实时面试上下文失败: interview_id=%d, err=%v", interviewID, err)
		_ = clientConn.WriteMessage(websocket.TextMessage,
			[]byte(`{"error":"获取面试上下文失败"}`))
		return
	}

	// 2. 复用预创建会话，若没有则回退为即时创建。
	session, err := uc.prepareSession(ctx, interviewID, userID, sessionID)
	if err != nil {
		uc.log.Errorf("准备实时会话失败: %v", err)
		_ = clientConn.WriteMessage(websocket.TextMessage,
			[]byte(`{"error":"创建会话失败"}`))
		return
	}

	// 3. 绑定实时对话 ID 到 Interview 服务
	if err := uc.interview.BindRealtimeDialog(ctx, interviewID, session.ID); err != nil {
		uc.log.Errorf("绑定实时对话失败: interview_id=%d, err=%v", interviewID, err)
		_ = clientConn.WriteMessage(websocket.TextMessage,
			[]byte(`{"error":"绑定对话失败"}`))
		return
	}

	// 4. 连接火山引擎 WebSocket
	volcConn, err := uc.volcFactory(ctx)
	if err != nil {
		uc.log.Errorf("连接火山引擎失败: %v", err)
		_ = clientConn.WriteMessage(websocket.TextMessage,
			[]byte(`{"error":"连接语音服务失败"}`))
		return
	}
	defer volcConn.Close()

	// 5. 创建会话上下文，确保所有 goroutine 可被取消
	sessionCtx, sessionCancel := context.WithCancel(ctx)
	defer sessionCancel()
	go func() {
		<-sessionCtx.Done()
		_ = clientConn.Close()
		_ = volcConn.Close()
	}()

	// 6. 注册到会话管理器
	rtSession := &RealtimeSession{
		SessionID:    session.ID,
		InterviewID:  interviewID,
		UserID:       userID,
		RTCtx:        rtCtx,
		ClientConn:   clientConn,
		VolcConn:     volcConn,
		Cancel:       sessionCancel,
		LastActivity: time.Now(),
	}
	uc.smgr.Store(session.ID, rtSession)
	defer uc.smgr.Delete(session.ID)
	defer func() {
		_ = uc.repo.UpdateSessionStatus(context.Background(), session.ID, "ended")
		if err := uc.interview.FinishInterview(context.Background(), interviewID); err != nil {
			uc.log.Warnf("结束实时面试失败: interview_id=%d err=%v", interviewID, err)
		}
	}()

	// 7. 启动 3 个 goroutine 并等待任一退出
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		uc.clientToVolc(sessionCtx, rtSession)
	}()
	go func() {
		defer wg.Done()
		uc.volcToClient(sessionCtx, rtSession)
	}()
	go func() {
		defer wg.Done()
		uc.ragInjector(sessionCtx, rtSession)
	}()

	// 等待任一 goroutine 退出（会话结束）
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

// volcToClient 读取火山引擎事件，处理后转发给客户端
func (uc *RealtimeUseCase) volcToClient(ctx context.Context, session *RealtimeSession) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		event, err := session.VolcConn.ReadEvent()
		if err != nil {
			uc.log.Errorf("读取火山引擎事件失败: %v", err)
			session.Cancel()
			return
		}

		session.LastActivity = time.Now()

		switch event.Type {
		case 501: // ASR 语音识别结果
			_ = session.ClientConn.WriteMessage(websocket.TextMessage,
				uc.buildASRMessage(event.Payload))
			// 保存 ASR 识别的用户回答
			uc.handleASREvent(ctx, session, event.Payload)
		case 502: // Chat 对话结果
			uc.handleChatEvent(ctx, session, event.Payload)
		case 503: // TTS 语音合成结果
			_ = session.ClientConn.WriteMessage(websocket.BinaryMessage, event.Payload)
		case 100: // Control 控制事件
			_ = session.ClientConn.WriteMessage(websocket.TextMessage, event.Payload)
		}
	}
}

// ragInjector 定时调用 RAG 检索注入上下文，每 30 秒执行一次
func (uc *RealtimeUseCase) ragInjector(ctx context.Context, session *RealtimeSession) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			uc.injectRAGForSession(ctx, session)
		}
	}
}

// injectRAGForSession 为单次 RAG 注入执行检索和上下文写入
func (uc *RealtimeUseCase) injectRAGForSession(ctx context.Context, session *RealtimeSession) {
	query := fmt.Sprintf("%s %s 面试", session.RTCtx.Industry, session.RTCtx.Difficulty)
	docs, err := uc.rag.Retrieve(ctx, query, 3)
	if err != nil {
		uc.log.Errorf("RAG 检索失败: session_id=%s, err=%v", session.SessionID, err)
		return
	}
	if len(docs) == 0 {
		return
	}

	var sb strings.Builder
	sb.WriteString("参考资料：")
	for _, doc := range docs {
		sb.WriteString("\n- ")
		sb.WriteString(doc.Content)
	}

	if err := session.VolcConn.InjectContext(sb.String()); err != nil {
		uc.log.Errorf("注入 RAG 上下文失败: session_id=%s, err=%v", session.SessionID, err)
	}
}

// handleClientControl 处理客户端控制指令（如结束面试）
func (uc *RealtimeUseCase) handleClientControl(_ context.Context, session *RealtimeSession, payload []byte) {
	var msg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		uc.log.Warnf("解析客户端控制消息失败: %v", err)
		return
	}
	if msg.Type == "end_interview" {
		uc.log.Infof("客户端请求结束面试: session_id=%s", session.SessionID)
		session.Cancel()
	}
}

// handleASREvent 处理 ASR 语音识别事件，保存用户回答
func (uc *RealtimeUseCase) handleASREvent(ctx context.Context, session *RealtimeSession, payload []byte) {
	var asrResult struct {
		Text    string `json:"text"`
		IsFinal bool   `json:"is_final"`
	}
	if err := json.Unmarshal(payload, &asrResult); err != nil {
		uc.log.Errorf("解析 ASR 事件失败: %v", err)
		return
	}
	// 只保存最终识别结果
	if asrResult.IsFinal && asrResult.Text != "" {
		if err := uc.interview.AppendRealtimeUserAnswer(ctx, session.InterviewID, asrResult.Text); err != nil {
			uc.log.Errorf("保存用户回答失败: %v", err)
		}
	}
}

// handleChatEvent 处理火山引擎对话事件，保存 AI 回复
func (uc *RealtimeUseCase) handleChatEvent(ctx context.Context, session *RealtimeSession, payload []byte) {
	// 转发对话结果给客户端
	_ = session.ClientConn.WriteMessage(websocket.TextMessage,
		uc.buildChatMessage(payload))

	// 解析 AI 回复并保存
	var chatResult struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(payload, &chatResult); err == nil && chatResult.Text != "" {
		meta, err := uc.interview.AppendRealtimeAssistantReply(ctx, session.InterviewID, chatResult.Text)
		if err != nil {
			uc.log.Errorf("保存 AI 回复失败: %v", err)
			return
		}
		if meta != nil && meta.IsLastQuestion {
			session.Cancel()
		}
	}
}

// buildASRMessage 构建 ASR 结果消息
func (uc *RealtimeUseCase) buildASRMessage(payload []byte) []byte {
	msg := fmt.Sprintf(`{"type":"asr","data":%s}`, string(payload))
	return []byte(msg)
}

// buildChatMessage 构建 Chat 结果消息
func (uc *RealtimeUseCase) buildChatMessage(payload []byte) []byte {
	msg := fmt.Sprintf(`{"type":"chat","data":%s}`, string(payload))
	return []byte(msg)
}
