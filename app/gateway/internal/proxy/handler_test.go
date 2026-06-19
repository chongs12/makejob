package proxy

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	questionv1 "makejob/api/makejob/question/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	"makejob/pkg/auth"
)

// questionMetadataProbeServer 用于记录题库 RPC 收到的 metadata，验证 Gateway 可选鉴权透传是否生效。
type questionMetadataProbeServer struct {
	questionv1.UnimplementedQuestionServiceServer
	listMetadata   metadata.MD
	detailMetadata metadata.MD
}

// ListQuestions 记录题库列表请求收到的 metadata，供路由透传测试断言。
func (s *questionMetadataProbeServer) ListQuestions(ctx context.Context, _ *questionv1.ListQuestionsRequest) (*questionv1.ListQuestionsResponse, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.listMetadata = md.Copy()
	}
	return &questionv1.ListQuestionsResponse{
		PageResult: &sharedv1.PageResult{Total: 0, Page: 1, PageSize: 20},
	}, nil
}

// GetQuestion 记录题目详情请求收到的 metadata，供详情路由透传测试断言。
func (s *questionMetadataProbeServer) GetQuestion(ctx context.Context, req *questionv1.GetQuestionRequest) (*questionv1.QuestionDetail, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.detailMetadata = md.Copy()
	}
	return &questionv1.QuestionDetail{
		Id:       req.Id,
		Title:    "question",
		Category: &questionv1.CategoryInfo{},
	}, nil
}

// TestJWTMiddlewareInjectsLegacyUserID 验证网关鉴权后会把 user_id 以 legacy handler 可识别的 uint 注入上下文。
func TestJWTMiddlewareInjectsLegacyUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token, err := auth.GenerateToken(12, "tester@example.com", "user", "secret", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}

	gateway := &Gateway{jwtSecret: "secret"}
	engine := gin.New()
	engine.Use(gateway.JWTMiddleware())
	engine.GET("/protected", func(c *gin.Context) {
		value, exists := c.Get("user_id")
		if !exists {
			t.Fatalf("user_id was not injected")
		}
		userID, ok := value.(uint)
		if !ok {
			t.Fatalf("expected uint user_id, got %T", value)
		}
		if userID != 12 {
			t.Fatalf("expected user_id 12, got %d", userID)
		}
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
}

// TestJWTMiddlewareInjectsOutgoingAuthorization 验证网关鉴权后会把 Bearer Token 写入 gRPC 出站上下文，供下游服务透传鉴权。
func TestJWTMiddlewareInjectsOutgoingAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token, err := auth.GenerateToken(12, "tester@example.com", "user", "secret", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}

	gateway := &Gateway{jwtSecret: "secret"}
	engine := gin.New()
	engine.Use(gateway.JWTMiddleware())
	engine.GET("/protected", func(c *gin.Context) {
		md, ok := metadata.FromOutgoingContext(c.Request.Context())
		if !ok {
			t.Fatalf("outgoing metadata was not injected")
		}
		authHeaders := md.Get("authorization")
		if len(authHeaders) != 1 {
			t.Fatalf("expected 1 authorization header, got %d", len(authHeaders))
		}
		if authHeaders[0] != "Bearer "+token {
			t.Fatalf("expected forwarded token, got %q", authHeaders[0])
		}
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
}

// TestRegisterV1RoutesForwardOptionalAuthorization 验证 `/api/v1/questions` 系列公开读接口在已登录时会把 Bearer Token 透传给题库服务。
func TestRegisterV1RoutesForwardOptionalAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token, err := auth.GenerateToken(12, "tester@example.com", "user", "secret", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	probe := &questionMetadataProbeServer{}
	questionv1.RegisterQuestionServiceServer(server, probe)
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Stop()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("DialContext returned error: %v", err)
	}
	defer conn.Close()

	gateway := &Gateway{
		jwtSecret:      "secret",
		questionClient: questionv1.NewQuestionServiceClient(conn),
	}
	engine := gin.New()
	gateway.registerV1Routes(engine)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/questions?page=1&page_size=1", nil)
	listRequest.Header.Set("Authorization", "Bearer "+token)
	listRecorder := httptest.NewRecorder()
	engine.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d", http.StatusOK, listRecorder.Code)
	}
	if got := probe.listMetadata.Get("authorization"); len(got) != 1 || got[0] != "Bearer "+token {
		t.Fatalf("expected list authorization metadata to be forwarded, got %#v", probe.listMetadata)
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/questions/692", nil)
	detailRequest.Header.Set("Authorization", "Bearer "+token)
	detailRecorder := httptest.NewRecorder()
	engine.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("expected detail status %d, got %d", http.StatusOK, detailRecorder.Code)
	}
	if got := probe.detailMetadata.Get("authorization"); len(got) != 1 || got[0] != "Bearer "+token {
		t.Fatalf("expected detail authorization metadata to be forwarded, got %#v", probe.detailMetadata)
	}
}

// TestJWTMiddlewareSupportsWebSocketQueryToken 验证 WebSocket 握手可以通过 Query token 透传身份。
func TestJWTMiddlewareSupportsWebSocketQueryToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token, err := auth.GenerateToken(24, "ws@example.com", "user", "secret", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}

	gateway := &Gateway{jwtSecret: "secret"}
	engine := gin.New()
	engine.Use(gateway.JWTMiddleware())
	engine.GET("/ws", func(c *gin.Context) {
		userID, ok := getUserID(c)
		if !ok {
			t.Fatalf("getUserID returned not authenticated")
		}
		if userID != 24 {
			t.Fatalf("expected user_id 24, got %d", userID)
		}
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/ws?token="+token, nil)
	request.Header.Set("Upgrade", "websocket")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
}

// TestOptionalJWTMiddlewareAllowsAnonymous 验证可选鉴权中间件不会阻断匿名请求。
func TestOptionalJWTMiddlewareAllowsAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gateway := &Gateway{jwtSecret: "secret"}
	engine := gin.New()
	engine.Use(gateway.OptionalJWTMiddleware())
	engine.GET("/public", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/public", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
}

// TestRegisterSystemRoutesMatchesLegacyShape 验证 gateway 基础路由保持与单体一致的健康检查与根路径行为。
func TestRegisterSystemRoutesMatchesLegacyShape(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gateway := &Gateway{}
	engine := gin.New()
	gateway.registerSystemRoutes(engine)

	redirectRecorder := httptest.NewRecorder()
	redirectRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	engine.ServeHTTP(redirectRecorder, redirectRequest)
	if redirectRecorder.Code != http.StatusMovedPermanently {
		t.Fatalf("expected redirect status %d, got %d", http.StatusMovedPermanently, redirectRecorder.Code)
	}

	healthRecorder := httptest.NewRecorder()
	healthRequest := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	engine.ServeHTTP(healthRecorder, healthRequest)
	if healthRecorder.Code != http.StatusOK {
		t.Fatalf("expected health status %d, got %d", http.StatusOK, healthRecorder.Code)
	}

	var response struct {
		Code    int                    `json:"code"`
		Message string                 `json:"message"`
		Data    map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(healthRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal health response: %v", err)
	}
	if response.Code != 0 || response.Message != "success" {
		t.Fatalf("unexpected health response: %+v", response)
	}
	if response.Data["status"] != "ok" {
		t.Fatalf("expected health status ok, got %+v", response.Data)
	}

	readyRecorder := httptest.NewRecorder()
	readyRequest := httptest.NewRequest(http.MethodGet, "/api/health/ready", nil)
	engine.ServeHTTP(readyRecorder, readyRequest)
	if readyRecorder.Code != http.StatusOK {
		t.Fatalf("expected ready status %d, got %d", http.StatusOK, readyRecorder.Code)
	}

	var readyResponse struct {
		Code    int                    `json:"code"`
		Message string                 `json:"message"`
		Data    map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(readyRecorder.Body.Bytes(), &readyResponse); err != nil {
		t.Fatalf("failed to unmarshal ready response: %v", err)
	}
	if readyResponse.Code != 0 || readyResponse.Message != "success" {
		t.Fatalf("unexpected ready response: %+v", readyResponse)
	}

	checksValue, ok := readyResponse.Data["checks"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected ready checks object, got %+v", readyResponse.Data["checks"])
	}
	if checksValue["gateway"] != "ok" {
		t.Fatalf("expected gateway readiness ok, got %+v", checksValue)
	}
}

// TestHandleAdminListAICallLogsRejectsInvalidTaskID 验证无 bridge 代理模式下也会保持单体对 task_id 的参数校验。
func TestHandleAdminListAICallLogsRejectsInvalidTaskID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gateway := &Gateway{}
	engine := gin.New()
	engine.GET("/api/admin/ai-call-logs", gateway.handleAdminListAICallLogs)

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ai-call-logs?task_id=bad", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

// buildTestToken 生成测试用 JWT，便于验证需要鉴权的路由注册是否生效。
func buildTestToken(t *testing.T, userID uint64, role string) string {
	t.Helper()

	token, err := auth.GenerateToken(userID, "tester@example.com", role, "secret", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	return token
}

// TestRegisterRoutesRegistersP64V1Endpoints 验证 P6-4 的 `/api/v1` 新路由已注册，且旧业务前缀不再暴露。
func TestRegisterRoutesRegistersP64V1Endpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gateway := &Gateway{jwtSecret: "secret"}
	engine := gin.New()
	gateway.RegisterRoutes(engine)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/question-sets", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected /api/v1/question-sets status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}

	legacyRequest := httptest.NewRequest(http.MethodGet, "/api/question-sets", nil)
	legacyRecorder := httptest.NewRecorder()
	engine.ServeHTTP(legacyRecorder, legacyRequest)
	if legacyRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected legacy business route status %d, got %d", http.StatusNotFound, legacyRecorder.Code)
	}
}

// TestRegisterRoutesProtectsAdminStream 验证新的 `/admin/...` SSE 路由已经注册，并继续受管理员鉴权保护。
func TestRegisterRoutesProtectsAdminStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gateway := &Gateway{jwtSecret: "secret"}
	engine := gin.New()
	gateway.RegisterRoutes(engine)

	userToken := buildTestToken(t, 1, "user")
	request := httptest.NewRequest(http.MethodGet, "/admin/question-pipeline/generate/stream", nil)
	request.Header.Set("Authorization", "Bearer "+userToken)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected admin stream status %d, got %d", http.StatusForbidden, recorder.Code)
	}

	adminToken := buildTestToken(t, 2, "admin")
	adminRequest := httptest.NewRequest(http.MethodGet, "/admin/question-pipeline/generate/stream", nil)
	adminRequest.Header.Set("Authorization", "Bearer "+adminToken)
	adminRecorder := httptest.NewRecorder()
	engine.ServeHTTP(adminRecorder, adminRequest)
	if adminRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected admin stream unavailable status %d, got %d", http.StatusServiceUnavailable, adminRecorder.Code)
	}

	postRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/question-pipeline/generate/stream", strings.NewReader(`{}`))
	postRequest.Header.Set("Content-Type", "application/json")
	postRequest.Header.Set("Authorization", "Bearer "+adminToken)
	postRecorder := httptest.NewRecorder()
	engine.ServeHTTP(postRecorder, postRequest)
	if postRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected admin POST stream unavailable status %d, got %d", http.StatusServiceUnavailable, postRecorder.Code)
	}
}

// TestWrapResponseMiddlewarePreservesExistingEnvelope 验证已符合 ApiEnvelope 结构的响应不会在中间件中丢失 body。
func TestWrapResponseMiddlewarePreservesExistingEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(WrapResponseMiddleware())
	engine.GET("/enveloped", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"ok": true,
			},
		})
	})

	request := httptest.NewRequest(http.MethodGet, "/enveloped", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response struct {
		Code    int                    `json:"code"`
		Message string                 `json:"message"`
		Data    map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if response.Code != 0 || response.Message != "success" || response.Data["ok"] != true {
		t.Fatalf("unexpected response payload: %+v", response)
	}
}

// TestWrapResponseMiddlewareFlattensPageResult 验证分页响应会被扁平化为前端约定的 {list,total,page,page_size}。
func TestWrapResponseMiddlewareFlattensPageResult(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(WrapResponseMiddleware())
	engine.GET("/api/v1/admin/rag-documents", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"documents": []gin.H{
				{
					"id":         1,
					"title":      "doc",
					"updated_at": gin.H{"seconds": float64(1710000000), "nanos": float64(0)},
				},
			},
			"page_result": gin.H{
				"total":     1,
				"page":      2,
				"page_size": 20,
			},
		})
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/rag-documents", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	var response struct {
		Code int `json:"code"`
		Data struct {
			List     []map[string]interface{} `json:"list"`
			Total    float64                  `json:"total"`
			Page     float64                  `json:"page"`
			PageSize float64                  `json:"page_size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if response.Code != 0 {
		t.Fatalf("expected success code, got %+v", response)
	}
	if len(response.Data.List) != 1 {
		t.Fatalf("expected flattened list with 1 item, got %+v", response.Data)
	}
	if response.Data.Total != 1 || response.Data.Page != 2 || response.Data.PageSize != 20 {
		t.Fatalf("unexpected page payload: %+v", response.Data)
	}
	if _, ok := response.Data.List[0]["updated_at"].(string); !ok {
		t.Fatalf("expected timestamp to be normalized into string, got %#v", response.Data.List[0]["updated_at"])
	}
}

// TestWrapResponseMiddlewareNormalizesAdminQuestions 验证题库管理页会收到已解析的 options/tags/judge_config 字段。
func TestWrapResponseMiddlewareNormalizesAdminQuestions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(WrapResponseMiddleware())
	engine.GET("/api/v1/admin/questions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"questions": []gin.H{
				{
					"id":                   1,
					"title":                "question",
					"options_json":         "[\"A\",\"B\"]",
					"solution_json":        "{\"summary\":\"s\"}",
					"judge_config_json":    "{\"evaluation_mode\":\"analysis_only\"}",
					"answer_template_json": "{\"core_conclusion\":\"c\"}",
					"tags":                 "go, 并发",
				},
			},
			"page_result": gin.H{
				"total":     1,
				"page":      1,
				"page_size": 10,
			},
		})
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/questions", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	var response struct {
		Data struct {
			List []map[string]interface{} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(response.Data.List) != 1 {
		t.Fatalf("expected 1 question item, got %+v", response.Data.List)
	}
	if options, ok := response.Data.List[0]["options"].([]interface{}); !ok || len(options) != 2 {
		t.Fatalf("expected parsed options array, got %#v", response.Data.List[0]["options"])
	}
	if tags, ok := response.Data.List[0]["tags"].([]interface{}); !ok || len(tags) != 2 {
		t.Fatalf("expected parsed tags array, got %#v", response.Data.List[0]["tags"])
	}
	if _, ok := response.Data.List[0]["judge_config"].(map[string]interface{}); !ok {
		t.Fatalf("expected parsed judge_config object, got %#v", response.Data.List[0]["judge_config"])
	}
}

// TestWrapResponseMiddlewareBypassesSSE 验证 SSE 在中间件存在时仍会原样输出，不会被 envelope 包装吞掉。
func TestWrapResponseMiddlewareBypassesSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(WrapResponseMiddleware())
	engine.GET("/stream", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream; charset=utf-8")
		c.Status(http.StatusOK)
		c.Writer.WriteHeaderNow()
		_, _ = c.Writer.Write([]byte("event: ping\ndata: ready\n\n"))
		c.Writer.Flush()
	})

	request := httptest.NewRequest(http.MethodGet, "/stream", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	body := recorder.Body.String()
	if !strings.Contains(body, "event: ping") {
		t.Fatalf("expected SSE payload, got %q", body)
	}
	if strings.Contains(body, "\"code\"") {
		t.Fatalf("expected SSE response to bypass envelope, got %q", body)
	}
}

// TestRegisterRoutesExposesMembershipCallbackWithoutJWT 验证支付回调路由改为公开入口，不再被 JWT 中间件拦截。
func TestRegisterRoutesExposesMembershipCallbackWithoutJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gateway := &Gateway{jwtSecret: "secret"}
	engine := gin.New()
	gateway.RegisterRoutes(engine)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/membership/callback", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected callback route status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
}

// TestRegisterRoutesAddsMistakeTopicAliases 验证 `/api/v1/mistake-topics` 兼容路由已注册并继续受登录保护。
func TestRegisterRoutesAddsMistakeTopicAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gateway := &Gateway{jwtSecret: "secret"}
	engine := gin.New()
	gateway.RegisterRoutes(engine)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/mistake-topics", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected alias route unauthenticated status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}

	token := buildTestToken(t, 1, "user")
	protectedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/mistake-topics", nil)
	protectedRequest.Header.Set("Authorization", "Bearer "+token)
	protectedRecorder := httptest.NewRecorder()
	engine.ServeHTTP(protectedRecorder, protectedRequest)
	if protectedRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected alias route service unavailable status %d, got %d", http.StatusServiceUnavailable, protectedRecorder.Code)
	}
}

// TestRegisterRoutesAddsLive2DPublicRoutes 验证 Live2D 前台公开路由已同时注册到 `/api` 与 `/api/v1`。
func TestRegisterRoutesAddsLive2DPublicRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gateway := &Gateway{jwtSecret: "secret"}
	engine := gin.New()
	gateway.RegisterRoutes(engine)

	legacyRequest := httptest.NewRequest(http.MethodGet, "/api/live2d/models?scene=companion", nil)
	legacyRecorder := httptest.NewRecorder()
	engine.ServeHTTP(legacyRecorder, legacyRequest)
	if legacyRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected legacy live2d route status %d, got %d", http.StatusServiceUnavailable, legacyRecorder.Code)
	}

	v1Request := httptest.NewRequest(http.MethodGet, "/api/v1/live2d/current?scene=interview", nil)
	v1Recorder := httptest.NewRecorder()
	engine.ServeHTTP(v1Recorder, v1Request)
	if v1Recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected v1 live2d route status %d, got %d", http.StatusServiceUnavailable, v1Recorder.Code)
	}
}
