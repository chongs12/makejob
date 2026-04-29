package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/service"
)

// ListAICallLogs 获取 AI 调用日志列表。
func (h *AdminHandler) ListAICallLogs(c *gin.Context) {
	pageParam := common.ReadPageParam(c)
	var taskID *uint
	if rawTaskID := strings.TrimSpace(c.Query("task_id")); rawTaskID != "" {
		parsed, err := strconv.ParseUint(rawTaskID, 10, 64)
		if err != nil || parsed == 0 {
			common.BadRequest(c, "task_id 必须是正整数")
			return
		}
		normalized := uint(parsed)
		taskID = &normalized
	}

	result, err := h.adminService.ListAICallLogs(c.Request.Context(), &service.ListAICallLogsRequest{
		Page:     pageParam.Page,
		PageSize: pageParam.PageSize,
		Scene:    c.Query("scene"),
		Source:   c.Query("source"),
		Status:   c.Query("status"),
		TraceID:  c.Query("trace_id"),
		TaskID:   taskID,
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

// GetAICallLog 获取单条 AI 调用日志详情。
func (h *AdminHandler) GetAICallLog(c *gin.Context) {
	id, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id == 0 {
		common.BadRequest(c, "无效的 AI 日志 ID")
		return
	}

	log, err := h.adminService.GetAICallLog(c.Request.Context(), uint(id))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取 AI 调用日志详情失败: "+err.Error())
		}
		return
	}

	common.Success(c, log)
}
