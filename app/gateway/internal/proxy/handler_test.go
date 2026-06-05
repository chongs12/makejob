package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"makejob/pkg/auth"
)

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
