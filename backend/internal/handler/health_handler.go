package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"makejob-backend/internal/common"
	"makejob-backend/internal/metrics"
)

// HealthHandler 提供健康检查和 Prometheus 指标端点。
type HealthHandler struct {
	db      *gorm.DB
	rdb     *redis.Client
	version string
}

// NewHealthHandler 创建健康检查处理器。
// db 和 rdb 可以为 nil，对应检查会跳过。
func NewHealthHandler(db *gorm.DB, rdb *redis.Client, version string) *HealthHandler {
	return &HealthHandler{db: db, rdb: rdb, version: version}
}

// RegisterRoutes 注册健康检查和指标路由。
func (h *HealthHandler) RegisterRoutes(r *gin.Engine) {
	r.GET("/api/health", h.Liveness)
	r.GET("/api/health/ready", h.Readiness)
	r.GET("/metrics", h.Metrics)
}

// Liveness 轻量级存活探针，始终返回 200。
func (h *HealthHandler) Liveness(c *gin.Context) {
	common.Success(c, gin.H{
		"status":    "ok",
		"version":   h.version,
		"timestamp": time.Now().Unix(),
	})
}

// Readiness 就绪探针，检查 DB 和 Redis 连通性。
// 任一依赖不可用则返回 503。
func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	checks := make(map[string]string)
	allOK := true

	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil {
			checks["database"] = "error: " + err.Error()
			allOK = false
		} else if err := sqlDB.PingContext(ctx); err != nil {
			checks["database"] = "unreachable: " + err.Error()
			allOK = false
		} else {
			checks["database"] = "ok"
		}
	} else {
		checks["database"] = "not configured"
	}

	if h.rdb != nil {
		if err := h.rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = "unreachable: " + err.Error()
			allOK = false
		} else {
			checks["redis"] = "ok"
		}
	} else {
		checks["redis"] = "not configured"
	}

	if !allOK {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    common.CodeInternalError,
			"message": "service not ready",
			"checks":  checks,
		})
		return
	}

	common.Success(c, gin.H{
		"status": "ok",
		"checks": checks,
	})
}

// Metrics 返回 Prometheus scrape 端点。
func (h *HealthHandler) Metrics(c *gin.Context) {
	metrics.Handler().ServeHTTP(c.Writer, c.Request)
}
