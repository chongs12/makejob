// Package handler 提供HTTP请求处理层
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/service"
)

// MembershipHandler 会员处理器
type MembershipHandler struct {
	membershipService service.MembershipService
}

// NewMembershipHandler 创建会员处理器实例
func NewMembershipHandler(membershipService service.MembershipService) *MembershipHandler {
	return &MembershipHandler{
		membershipService: membershipService,
	}
}

// RegisterRoutes 注册会员相关路由
func (h *MembershipHandler) RegisterRoutes(r *gin.RouterGroup) {
	membership := r.Group("/membership")
	{
		// 会员方案
		membership.GET("/plans", h.GetPlans)

		// 会员状态
		membership.GET("/status", h.GetStatus)

		// 订单相关
		membership.POST("/orders", h.CreateOrder)
		membership.GET("/orders", h.ListOrders)
		membership.GET("/orders/:id", h.GetOrder)

		// Mock支付回调
		membership.POST("/callback", h.MockPayCallback)
	}
}

// GetPlans 获取会员方案列表
// @Summary 获取会员方案列表
// @Description 获取所有可用的会员订阅方案
// @Tags 会员
// @Accept json
// @Produce json
// @Success 200 {object} common.Response{data=[]service.MembershipPlanResponse}
// @Router /api/membership/plans [get]
func (h *MembershipHandler) GetPlans(c *gin.Context) {
	plans, err := h.membershipService.GetPlans(c.Request.Context())
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取会员方案失败: "+err.Error())
		}
		return
	}

	common.Success(c, plans)
}

// GetStatus 获取当前会员状态
// @Summary 获取当前会员状态
// @Description 获取当前登录用户的会员状态和使用情况
// @Tags 会员
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} common.Response{data=service.MembershipStatusResponse}
// @Failure 401 {object} common.Response
// @Router /api/membership/status [get]
func (h *MembershipHandler) GetStatus(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	status, err := h.membershipService.GetMembershipStatus(c.Request.Context(), userID)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取会员状态失败: "+err.Error())
		}
		return
	}

	common.Success(c, status)
}

// CreateOrder 创建订单
// @Summary 创建会员订单
// @Description 创建一个新的会员订阅订单
// @Tags 会员
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.CreateOrderRequest true "订单信息"
// @Success 200 {object} common.Response{data=service.OrderResponse}
// @Failure 400 {object} common.Response
// @Failure 401 {object} common.Response
// @Router /api/membership/orders [post]
func (h *MembershipHandler) CreateOrder(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	var req service.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	order, err := h.membershipService.CreateOrder(c.Request.Context(), userID, &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "创建订单失败: "+err.Error())
		}
		return
	}

	common.Success(c, order)
}

// GetOrder 获取订单详情
// @Summary 获取订单详情
// @Description 根据ID获取订单详情
// @Tags 会员
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "订单ID"
// @Success 200 {object} common.Response{data=service.OrderResponse}
// @Failure 401 {object} common.Response
// @Failure 403 {object} common.Response
// @Failure 404 {object} common.Response
// @Router /api/membership/orders/{id} [get]
func (h *MembershipHandler) GetOrder(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	// 解析订单ID
	orderIDStr := c.Param("id")
	orderID, err := strconv.ParseUint(orderIDStr, 10, 32)
	if err != nil {
		common.BadRequest(c, "无效的订单ID")
		return
	}

	order, err := h.membershipService.GetOrder(c.Request.Context(), userID, uint(orderID))
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			if businessErr.Code == common.CodeNotFound {
				common.NotFound(c, businessErr.Message)
			} else if businessErr.Code == common.CodeForbidden {
				common.Forbidden(c, businessErr.Message)
			} else {
				common.Error(c, businessErr.Code, businessErr.Message)
			}
		} else {
			common.InternalError(c, "获取订单失败: "+err.Error())
		}
		return
	}

	common.Success(c, order)
}

// ListOrders 获取订单列表
// @Summary 获取订单列表
// @Description 获取当前用户的所有订单
// @Tags 会员
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} common.Response{data=common.PageResult}
// @Failure 401 {object} common.Response
// @Router /api/membership/orders [get]
func (h *MembershipHandler) ListOrders(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	// 统一读取并规范化分页参数，避免列表接口出现不一致的默认值。
	pageParam := common.ReadPageParam(c)

	result, err := h.membershipService.ListOrders(c.Request.Context(), userID, pageParam.Page, pageParam.PageSize)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取订单列表失败: "+err.Error())
		}
		return
	}

	common.Success(c, result)
}

// MockPayCallback Mock支付回调
// @Summary Mock支付回调
// @Description 模拟支付成功回调，用于测试
// @Tags 会员
// @Accept json
// @Produce json
// @Param request body MockPayCallbackRequest true "回调信息"
// @Success 200 {object} common.Response{data=service.OrderResponse}
// @Failure 400 {object} common.Response
// @Failure 404 {object} common.Response
// @Router /api/membership/callback [post]
func (h *MembershipHandler) MockPayCallback(c *gin.Context) {
	var req MockPayCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if req.OrderNo == "" {
		common.BadRequest(c, "订单号不能为空")
		return
	}

	order, err := h.membershipService.MockPayCallback(c.Request.Context(), req.OrderNo)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			if businessErr.Code == common.CodeNotFound {
				common.NotFound(c, businessErr.Message)
			} else {
				common.Error(c, businessErr.Code, businessErr.Message)
			}
		} else {
			common.InternalError(c, "处理支付回调失败: "+err.Error())
		}
		return
	}

	common.Success(c, order)
}

// MockPayCallbackRequest Mock支付回调请求
type MockPayCallbackRequest struct {
	OrderNo string `json:"order_no" binding:"required"`
}
