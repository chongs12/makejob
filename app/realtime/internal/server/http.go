package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"makejob/app/realtime/internal/biz"
	"makejob/app/realtime/internal/conf"
	"makejob/pkg/auth"
)

// activeWSConnections 当前活跃 WebSocket 连接数（由 realtime 服务管理，gateway 不暴露此指标）。
var activeWSConnections = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "active_websocket_connections",
	Help: "Number of active realtime WebSocket interview sessions.",
})

func init() {
	prometheus.MustRegister(activeWSConnections)
}

// httpServer 封装 HTTP/WebSocket 服务器
type httpServer struct {
	server *http.Server
	logger *log.Helper
}

// NewHTTPServer 创建 WebSocket HTTP 服务器
func NewHTTPServer(cfg *conf.Server, uc *biz.RealtimeUseCase, jwtSecret string, logger log.Logger) *httpServer {
	mux := http.NewServeMux()

	upgrader := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		ReadBufferSize:  1024 * 64,
		WriteBufferSize: 1024 * 64,
	}

	helper := log.NewHelper(logger)

	// 注册 WebSocket 路由：/ws/interview/{interview_id}
	mux.HandleFunc("/ws/interview/", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(w, r, uc, jwtSecret, upgrader, helper)
	})

	// 注册健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	addr := ":8008"
	if cfg.HTTP != nil && cfg.HTTP.Addr != "" {
		addr = cfg.HTTP.Addr
	}

	return &httpServer{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		logger: helper,
	}
}

// Start 启动 HTTP 服务器
func (s *httpServer) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("HTTP 服务器监听失败: %w", err)
	}
	s.logger.Infof("HTTP/WebSocket 服务器启动: %s", s.server.Addr)
	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Errorf("HTTP 服务器异常: %v", err)
		}
	}()
	return nil
}

// Stop 停止 HTTP 服务器
func (s *httpServer) Stop(ctx context.Context) error {
	s.logger.Info("HTTP/WebSocket 服务器关闭中...")
	return s.server.Shutdown(ctx)
}

// handleWebSocket 处理 WebSocket 连接升级和路由
func handleWebSocket(w http.ResponseWriter, r *http.Request, uc *biz.RealtimeUseCase, jwtSecret string, upgrader *websocket.Upgrader, logger *log.Helper) {
	// 1. 从路径提取 interview_id
	interviewIDStr := r.URL.Path[len("/ws/interview/"):]
	interviewID, err := strconv.ParseUint(interviewIDStr, 10, 64)
	if err != nil || interviewID == 0 {
		http.Error(w, `{"error":"invalid interview_id"}`, http.StatusBadRequest)
		return
	}

	// 2. 从 query param 获取 token 并验证
	token := r.URL.Query().Get("token")
	if token == "" {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		token = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	if token == "" {
		http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
		return
	}

	claims, err := auth.ParseToken(token, jwtSecret)
	if err != nil {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	userID := claims.UserID

	// 3. 升级为 WebSocket 连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Errorf("WebSocket 升级失败: %v", err)
		return
	}

	// trace：从 HTTP header 提取 traceparent（前端无 OTel 埋点，通常为 root span），
	// 创建 session server span，使 realtime -> interview/rag 的 gRPC 调用能挂在该 span 下。
	propagator := otel.GetTextMapPropagator()
	wsCtx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	wsCtx, span := otel.Tracer("makejob.realtime").Start(wsCtx, "realtime.ws.session",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.Int64("interview.id", int64(interviewID)),
			attribute.Int64("user.id", int64(userID)),
		),
	)
	defer span.End()

	// active_websocket_connections Gauge：连接建立 +1，会话结束 -1
	activeWSConnections.Inc()
	defer activeWSConnections.Dec()

	logger.Infof("WebSocket 连接建立: interview_id=%d, user_id=%d", interviewID, userID)

	// 4. 进入实时会话处理（阻塞直到会话结束）
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	sessionCtx := auth.WithAccessToken(wsCtx, token)
	uc.HandleSession(sessionCtx, interviewID, userID, sessionID, conn)
}
