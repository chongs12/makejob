package bridge

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"makejob-backend/internal/live2dassets"
	"makejob-backend/internal/middleware"
)

// RegisterGatewayRoutes 以单体行为为准，将 legacy HTTP 路由完整挂载到 gateway。
func (r *Runtime) RegisterGatewayRoutes(engine *gin.Engine, optionalAuthMiddleware gin.HandlerFunc, authMiddleware gin.HandlerFunc, adminMiddleware gin.HandlerFunc) {
	r.registerLegacyPublicRoutes(engine, optionalAuthMiddleware)
	r.registerLegacyProtectedRoutes(engine, authMiddleware)
	r.registerLegacyAdminRoutes(engine, authMiddleware, adminMiddleware)

	if r.live2DAssetsDir != "" {
		engine.StaticFS(live2dassets.MountPath, gin.Dir(r.live2DAssetsDir, false))
		return
	}

	engine.GET(live2dassets.MountPath+"/*filepath", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "live2d assets directory is not ready"})
	})
}

// registerLegacyPublicRoutes 挂载单体公开路由，并保留 OptionalAuth 语义。
func (r *Runtime) registerLegacyPublicRoutes(engine *gin.Engine, optionalAuthMiddleware gin.HandlerFunc) {
	api := engine.Group("/api")
	if r.authHandler != nil {
		auth := api.Group("/auth")
		r.authHandler.RegisterRoutes(auth, nil)
	}

	public := api.Group("")
	public.Use(resolveOptionalAuthMiddleware(optionalAuthMiddleware))
	if r.questionHandler != nil {
		r.questionHandler.RegisterRoutes(public, nil)
	}
	if r.communityHandler != nil {
		r.communityHandler.RegisterRoutes(public, nil)
	}
	if r.live2DHandler != nil {
		r.live2DHandler.RegisterRoutes(public)
	}
}

// registerLegacyProtectedRoutes 挂载单体受保护路由，并保留原始 Auth 语义。
func (r *Runtime) registerLegacyProtectedRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc) {
	protected := engine.Group("/api")
	protected.Use(resolveAuthMiddleware(authMiddleware))
	if r.authHandler != nil {
		r.authHandler.RegisterProtectedRoutes(protected)
	}
	if r.membershipHandler != nil {
		r.membershipHandler.RegisterRoutes(protected)
	}
	if r.interviewHandler != nil {
		r.interviewHandler.RegisterRoutes(protected)
	}
	if r.planHandler != nil {
		r.planHandler.RegisterRoutes(protected)
	}
	if r.companionHandler != nil {
		r.companionHandler.RegisterRoutes(protected)
	}
	if r.growthHandler != nil {
		r.growthHandler.RegisterRoutes(protected)
	}
	if r.questionHandler != nil {
		r.questionHandler.RegisterRoutes(nil, protected)
	}
	if r.communityHandler != nil {
		r.communityHandler.RegisterRoutes(nil, protected)
	}
}

// registerLegacyAdminRoutes 挂载单体后台路由，并复用原始 Casbin 权限校验。
func (r *Runtime) registerLegacyAdminRoutes(engine *gin.Engine, authMiddleware gin.HandlerFunc, adminMiddleware gin.HandlerFunc) {
	admin := engine.Group("/api/admin")
	admin.Use(resolveAuthMiddleware(authMiddleware))
	if adminMiddleware != nil {
		admin.Use(adminMiddleware)
	}
	admin.Use(middleware.Casbin())
	if r.adminHandler != nil {
		r.adminHandler.RegisterRoutes(admin)
	}
	if r.scraperHandler != nil {
		r.scraperHandler.RegisterRoutes(admin)
	}
	if r.adminRAGHandler != nil {
		r.adminRAGHandler.RegisterRoutes(admin)
	}
	if r.ragDocHandler != nil {
		r.ragDocHandler.RegisterRoutes(admin)
	}
}

// resolveOptionalAuthMiddleware 返回可复用的可选认证中间件，缺省时回退到单体实现。
func resolveOptionalAuthMiddleware(optionalAuthMiddleware gin.HandlerFunc) gin.HandlerFunc {
	if optionalAuthMiddleware != nil {
		return optionalAuthMiddleware
	}
	return middleware.OptionalAuth()
}

// resolveAuthMiddleware 返回可复用的强制认证中间件，缺省时回退到单体实现。
func resolveAuthMiddleware(authMiddleware gin.HandlerFunc) gin.HandlerFunc {
	if authMiddleware != nil {
		return authMiddleware
	}
	return middleware.Auth()
}
