package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/service"
)

// ListAICallLogs 获取 AI 调用日志列表。
func (h *AdminHandler) ListAICallLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	result, err := h.adminService.ListAICallLogs(c.Request.Context(), &service.ListAICallLogsRequest{
		Page:     page,
		PageSize: pageSize,
		Scene:    c.Query("scene"),
		Source:   c.Query("source"),
		Status:   c.Query("status"),
		TraceID:  c.Query("trace_id"),
	})
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取 AI 调用日志失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}
