package bridge

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/handler"
	"makejob-backend/internal/service"
)

// stubGrowthService 为 bridge 路由测试提供最小成长服务桩。
type stubGrowthService struct{}

// SyncStudyLog 返回固定响应，满足 GrowthService 接口。
func (stubGrowthService) SyncStudyLog(_ context.Context, _ uint, _ *service.SyncStudyLogRequest) (*service.StudyLogResponse, error) {
	return &service.StudyLogResponse{}, nil
}

// GetGrowthSummary 返回固定响应，满足 GrowthService 接口。
func (stubGrowthService) GetGrowthSummary(_ context.Context, _ uint) (*service.GrowthSummaryResponse, error) {
	return &service.GrowthSummaryResponse{}, nil
}

// GetWeeklyFocus 返回固定响应，满足 GrowthService 接口。
func (stubGrowthService) GetWeeklyFocus(_ context.Context, _ uint) (*service.WeeklyFocusResponse, error) {
	return &service.WeeklyFocusResponse{}, nil
}

// stubCompanionService 为 bridge 路由测试提供最小陪伴服务桩。
type stubCompanionService struct{}

// Chat 返回固定响应，满足 CompanionService 接口。
func (stubCompanionService) Chat(_ context.Context, _ uint, _ *service.CompanionChatRequest) (*service.CompanionChatResponse, error) {
	return &service.CompanionChatResponse{}, nil
}

// TestRegisterGatewayRoutesIncludesGrowthAndCompanion 验证 bridge 会完整挂载原单体的 growth 与 companion 路由。
func TestRegisterGatewayRoutesIncludesGrowthAndCompanion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	runtime := &Runtime{
		growthHandler:    handler.NewGrowthHandler(stubGrowthService{}),
		companionHandler: handler.NewCompanionHandler(stubCompanionService{}),
	}

	engine := gin.New()
	runtime.RegisterGatewayRoutes(engine, nil, nil, nil)

	expectedRoutes := map[string]string{
		http.MethodPut + " /api/user/study-logs/daily":  http.MethodPut,
		http.MethodGet + " /api/user/growth-summary":    http.MethodGet,
		http.MethodGet + " /api/user/weekly-focus":      http.MethodGet,
		http.MethodPost + " /api/companion/chat":        http.MethodPost,
	}

	registered := make(map[string]struct{})
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = struct{}{}
	}

	for route := range expectedRoutes {
		if _, exists := registered[route]; !exists {
			t.Fatalf("expected route %s to be registered", route)
		}
	}
}
