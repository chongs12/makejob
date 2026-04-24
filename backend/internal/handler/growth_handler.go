// Package handler 提供 HTTP 请求处理层
package handler

import (
	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/service"
)

// GrowthHandler 负责成长档案相关接口的请求分发。
type GrowthHandler struct {
	growthService service.GrowthService
}

// NewGrowthHandler 创建成长档案处理器实例。
func NewGrowthHandler(growthService service.GrowthService) *GrowthHandler {
	return &GrowthHandler{
		growthService: growthService,
	}
}

// RegisterRoutes 注册成长档案相关受保护路由。
func (h *GrowthHandler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.PUT("/user/study-logs/daily", h.SyncDailyStudyLog)
	protected.GET("/user/growth-summary", h.GetGrowthSummary)
}

// SyncDailyStudyLog 接收前端当天学习摘要并同步到服务端。
func (h *GrowthHandler) SyncDailyStudyLog(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	var req service.SyncStudyLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.growthService.SyncStudyLog(c.Request.Context(), userID, &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "同步学习日志失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// GetGrowthSummary 返回成长档案首页需要的聚合概览数据。
func (h *GrowthHandler) GetGrowthSummary(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	resp, err := h.growthService.GetGrowthSummary(c.Request.Context(), userID)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取成长档案失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}
