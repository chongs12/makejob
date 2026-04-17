package handler

import (
	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/service"
)

// TestRenderPrompt 调试 prompt 渲染与可选模型试跑。
func (h *AdminHandler) TestRenderPrompt(c *gin.Context) {
	var req service.AIDebugRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.adminService.DebugAIRuntime(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "调试 Prompt 失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}
