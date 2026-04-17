package handler

import (
	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/service"
)

type CompanionHandler struct {
	companionService service.CompanionService
}

func NewCompanionHandler(companionService service.CompanionService) *CompanionHandler {
	return &CompanionHandler{
		companionService: companionService,
	}
}

func (h *CompanionHandler) RegisterRoutes(protected *gin.RouterGroup) {
	companion := protected.Group("/companion")
	{
		companion.POST("/chat", h.Chat)
	}
}

func (h *CompanionHandler) Chat(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	var req service.CompanionChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.companionService.Chat(c.Request.Context(), userID, &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "陪伴聊天失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}
