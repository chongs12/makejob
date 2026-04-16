// Package handler 提供HTTP请求处理层
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/service"
)

// PlanHandler 学习计划处理器
type PlanHandler struct {
	planService service.PlanService
}

// NewPlanHandler 创建学习计划处理器实例
func NewPlanHandler(planService service.PlanService) *PlanHandler {
	return &PlanHandler{
		planService: planService,
	}
}

// RegisterRoutes 注册学习计划相关路由
func (h *PlanHandler) RegisterRoutes(protected *gin.RouterGroup) {
	plans := protected.Group("/plans")
	{
		plans.POST("", h.GeneratePlan)                      // 生成学习计划
		plans.GET("/current", h.GetCurrentPlan)             // 获取当前计划
		plans.GET("", h.ListPlans)                          // 计划列表
		plans.GET("/:id", h.GetPlan)                        // 计划详情
		plans.PUT("/:id/tasks/:taskId", h.UpdateTaskStatus) // 更新任务状态
		plans.POST("/:id/adjust", h.AdjustPlan)             // 动态调整计划
		plans.GET("/:id/progress", h.GetProgress)           // 进度统计
	}
}

// GeneratePlan 生成学习计划
// @Summary 生成学习计划
// @Description 根据用户配置生成个性化学习计划
// @Tags 学习计划
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.GeneratePlanRequest true "计划配置"
// @Success 200 {object} common.Response{data=service.PlanDetailResponse}
// @Failure 400 {object} common.Response
// @Router /api/plans [post]
func (h *PlanHandler) GeneratePlan(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	var req service.GeneratePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.planService.GeneratePlan(c.Request.Context(), userID, &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "生成学习计划失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// GetCurrentPlan 获取当前学习计划
// @Summary 获取当前学习计划
// @Description 获取用户当前进行中的学习计划
// @Tags 学习计划
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} common.Response{data=service.PlanDetailResponse}
// @Failure 404 {object} common.Response
// @Router /api/plans/current [get]
func (h *PlanHandler) GetCurrentPlan(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	resp, err := h.planService.GetCurrentPlan(c.Request.Context(), userID)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取当前计划失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// ListPlans 获取学习计划列表
// @Summary 获取学习计划列表
// @Description 获取用户的所有学习计划列表
// @Tags 学习计划
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页数量，默认10"
// @Success 200 {object} common.Response{data=common.PageResult}
// @Failure 401 {object} common.Response
// @Router /api/plans [get]
func (h *PlanHandler) ListPlans(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	result, err := h.planService.ListPlans(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取计划列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// GetPlan 获取学习计划详情
// @Summary 获取学习计划详情
// @Description 获取指定学习计划的详细信息
// @Tags 学习计划
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "计划ID"
// @Success 200 {object} common.Response{data=service.PlanDetailResponse}
// @Failure 404 {object} common.Response
// @Router /api/plans/{id} [get]
func (h *PlanHandler) GetPlan(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	planID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的计划ID")
		return
	}

	resp, err := h.planService.GetPlan(c.Request.Context(), userID, uint(planID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取计划详情失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// UpdateTaskStatus 更新任务状态
// @Summary 更新任务状态
// @Description 更新学习计划中指定任务的状态
// @Tags 学习计划
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "计划ID"
// @Param taskId path int true "任务ID"
// @Param request body service.UpdateTaskStatusRequest true "状态信息"
// @Success 200 {object} common.Response
// @Failure 400 {object} common.Response
// @Router /api/plans/{id}/tasks/{taskId} [put]
func (h *PlanHandler) UpdateTaskStatus(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	planID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的计划ID")
		return
	}

	taskID, err := strconv.ParseUint(c.Param("taskId"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的任务ID")
		return
	}

	var req service.UpdateTaskStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.planService.UpdateTaskStatus(c.Request.Context(), userID, uint(planID), uint(taskID), &req); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新任务状态失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "更新成功", nil)
}

// AdjustPlan 动态调整学习计划
// @Summary 动态调整学习计划
// @Description 根据学习进度动态调整学习计划
// @Tags 学习计划
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "计划ID"
// @Success 200 {object} common.Response{data=service.PlanDetailResponse}
// @Failure 400 {object} common.Response
// @Router /api/plans/{id}/adjust [post]
func (h *PlanHandler) AdjustPlan(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	planID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的计划ID")
		return
	}

	resp, err := h.planService.AdjustPlan(c.Request.Context(), userID, uint(planID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "调整学习计划失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// GetProgress 获取学习进度统计
// @Summary 获取学习进度统计
// @Description 获取学习计划的进度统计信息
// @Tags 学习计划
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "计划ID"
// @Success 200 {object} common.Response{data=service.PlanProgressResponse}
// @Failure 404 {object} common.Response
// @Router /api/plans/{id}/progress [get]
func (h *PlanHandler) GetProgress(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	planID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的计划ID")
		return
	}

	resp, err := h.planService.GetProgress(c.Request.Context(), userID, uint(planID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取学习进度失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}
