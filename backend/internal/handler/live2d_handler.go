package handler

import (
	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/service"
)

// Live2DHandler 处理前台 Live2D 模型查询接口。
type Live2DHandler struct {
	live2DService service.Live2DService
}

// NewLive2DHandler 创建前台 Live2D 处理器。
func NewLive2DHandler(live2DService service.Live2DService) *Live2DHandler {
	return &Live2DHandler{
		live2DService: live2DService,
	}
}

// RegisterRoutes 注册前台 Live2D 公开路由。
func (h *Live2DHandler) RegisterRoutes(public *gin.RouterGroup) {
	live2d := public.Group("/live2d")
	{
		live2d.GET("/current", h.GetCurrentModel)
	}
}

// GetCurrentModel 返回当前页面可使用的 Live2D 模型。
func (h *Live2DHandler) GetCurrentModel(c *gin.Context) {
	resp, err := h.live2DService.GetCurrentModel(c.Request.Context(), &service.CurrentLive2DModelRequest{
		Scene:        c.Query("scene"),
		IndustryCode: c.Query("industry_code"),
	})
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取 Live2D 模型失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}
