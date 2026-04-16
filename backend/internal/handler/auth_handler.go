// Package handler 提供HTTP请求处理层
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/service"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authService service.AuthService
}

// NewAuthHandler 创建认证处理器实例
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// RegisterRoutes 注册公开路由（无需认证）
func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	// 注册路由（r已经是/auth组）
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)
	r.POST("/refresh", h.RefreshToken)
}

// RegisterProtectedRoutes 注册需要认证的路由
func (h *AuthHandler) RegisterProtectedRoutes(r *gin.RouterGroup) {
	// 兼容旧前端调用
	r.GET("/auth/me", h.GetProfile)

	// 用户相关路由
	user := r.Group("/user")
	{
		user.GET("/profile", h.GetProfile)
		user.PUT("/profile", h.UpdateProfile)
	}
}

// Register 用户注册
// @Summary 用户注册
// @Description 创建新用户账号
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body service.RegisterRequest true "注册信息"
// @Success 200 {object} common.Response{data=service.RegisterResponse}
// @Failure 400 {object} common.Response
// @Router /api/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req service.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "注册失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// Login 用户登录
// @Summary 用户登录
// @Description 使用邮箱和密码登录
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body service.LoginRequest true "登录信息"
// @Success 200 {object} common.Response{data=service.LoginResponse}
// @Failure 401 {object} common.Response
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "登录失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// RefreshToken 刷新令牌
// @Summary 刷新访问令牌
// @Description 使用刷新令牌获取新的访问令牌
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body map[string]string true "刷新令牌"
// @Success 200 {object} common.Response{data=service.TokenResponse}
// @Failure 401 {object} common.Response
// @Router /api/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken     string `json:"refresh_token"`
		RefreshTokenAlt  string `json:"refreshToken"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	refreshToken := req.RefreshToken
	if refreshToken == "" {
		refreshToken = req.RefreshTokenAlt
	}
	if refreshToken == "" {
		common.BadRequest(c, "刷新令牌不能为空")
		return
	}

	resp, err := h.authService.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "刷新令牌失败: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

// GetProfile 获取用户资料
// @Summary 获取用户资料
// @Description 获取当前登录用户的详细资料
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} common.Response{data=service.UserProfile}
// @Failure 401 {object} common.Response
// @Router /api/user/profile [get]
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	profile, err := h.authService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "获取用户资料失败: "+err.Error())
		}
		return
	}

	common.Success(c, profile)
}

// UpdateProfile 更新用户资料
// @Summary 更新用户资料
// @Description 更新当前登录用户的资料信息
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body service.UpdateProfileRequest true "更新信息"
// @Success 200 {object} common.Response
// @Failure 400 {object} common.Response
// @Router /api/user/profile [put]
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "未登录")
		return
	}

	var req service.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.authService.UpdateProfile(c.Request.Context(), userID, &req); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "更新用户资料失败: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "更新成功", nil)
}

// GetMe 获取当前登录用户信息（兼容旧接口）
// @Summary 获取当前用户信息
// @Description 获取当前登录用户的基本信息
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} common.Response
// @Router /api/me [get]
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, idExists := middleware.GetUserID(c)
	role, roleExists := middleware.GetRole(c)
	username, nameExists := middleware.GetUsername(c)

	if !idExists || !roleExists || !nameExists {
		common.Unauthorized(c, "未登录")
		return
	}

	common.Success(c, gin.H{
		"user_id":  userID,
		"username": username,
		"role":     role,
	})
}

// Logout 用户登出（预留接口，可用于令牌黑名单）
// @Summary 用户登出
// @Description 用户登出（预留）
// @Tags 认证
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} common.Response
// @Router /api/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// TODO: 实现令牌黑名单机制
	common.SuccessWithMessage(c, "登出成功", nil)
}

// ChangePassword 修改密码（预留接口）
// @Summary 修改密码
// @Description 修改当前用户密码
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body map[string]string true "密码信息"
// @Success 200 {object} common.Response
// @Router /api/user/password [put]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	// TODO: 实现修改密码功能
	c.JSON(http.StatusNotImplemented, common.Response{
		Code:    common.CodeInternalError,
		Message: "功能开发中",
	})
}
