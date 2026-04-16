package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/common"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/service"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)
	r.POST("/refresh", h.RefreshToken)
}

func (h *AuthHandler) RegisterProtectedRoutes(r *gin.RouterGroup) {
	r.GET("/auth/me", h.GetProfile)
	r.POST("/auth/logout", h.Logout)

	user := r.Group("/user")
	{
		user.GET("/profile", h.GetProfile)
		user.PUT("/profile", h.UpdateProfile)
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req service.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "invalid register payload: "+err.Error())
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "register failed: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "invalid login payload: "+err.Error())
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "login failed: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken    string `json:"refresh_token"`
		RefreshTokenAlt string `json:"refreshToken"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "invalid refresh token payload: "+err.Error())
		return
	}

	refreshToken := req.RefreshToken
	if refreshToken == "" {
		refreshToken = req.RefreshTokenAlt
	}
	if refreshToken == "" {
		common.BadRequest(c, "refresh token is required")
		return
	}

	resp, err := h.authService.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "refresh token failed: "+err.Error())
		}
		return
	}

	common.Success(c, resp)
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "login required")
		return
	}

	profile, err := h.authService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "get profile failed: "+err.Error())
		}
		return
	}

	common.Success(c, profile)
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		common.Unauthorized(c, "login required")
		return
	}

	var req service.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "invalid profile payload: "+err.Error())
		return
	}

	if err := h.authService.UpdateProfile(c.Request.Context(), userID, &req); err != nil {
		if businessErr, ok := err.(*common.BusinessError); ok {
			common.Error(c, businessErr.Code, businessErr.Message)
		} else {
			common.InternalError(c, "update profile failed: "+err.Error())
		}
		return
	}

	common.SuccessWithMessage(c, "profile updated", nil)
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, idExists := middleware.GetUserID(c)
	role, roleExists := middleware.GetRole(c)
	username, nameExists := middleware.GetUsername(c)

	if !idExists || !roleExists || !nameExists {
		common.Unauthorized(c, "login required")
		return
	}

	common.Success(c, gin.H{
		"user_id":  userID,
		"username": username,
		"role":     role,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	common.SuccessWithMessage(c, "logout success", nil)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, common.Response{
		Code:    common.CodeInternalError,
		Message: "not implemented",
	})
}
