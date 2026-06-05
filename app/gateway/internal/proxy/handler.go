package proxy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"makejob-backend/bridge"
	adminv1 "makejob/api/makejob/admin/v1"
	communityv1 "makejob/api/makejob/community/v1"
	growthv1 "makejob/api/makejob/growth/v1"
	interviewv1 "makejob/api/makejob/interview/v1"
	questionv1 "makejob/api/makejob/question/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	userv1 "makejob/api/makejob/user/v1"
	"makejob/app/gateway/internal/conf"
	"makejob/pkg/auth"
)

// Gateway HTTP → gRPC 代理
type Gateway struct {
	conns           []*grpc.ClientConn
	userClient      userv1.UserServiceClient
	questionClient  questionv1.QuestionServiceClient
	interviewClient interviewv1.InterviewServiceClient
	growthClient    growthv1.GrowthServiceClient
	adminClient     adminv1.AdminServiceClient
	communityClient communityv1.CommunityServiceClient
	backendBridge   *bridge.Runtime
	db              *gorm.DB
	jwtSecret       string
}

// legacyResponse 复刻原单体统一响应结构，供 bridge 模式下的鉴权错误复用。
type legacyResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

const (
	legacyCodeUnauthorized = 401
	legacyCodeTokenExpired = 4011
	legacyCodeTokenInvalid = 4012
)

// NewGateway 创建网关实例
func NewGateway(cfg *conf.Bootstrap) (*Gateway, error) {
	gw := &Gateway{jwtSecret: cfg.JWT.Secret}

	if cfg.Data != nil && cfg.Data.Database != nil && strings.TrimSpace(cfg.Data.Database.Source) != "" {
		db, err := gorm.Open(postgres.Open(cfg.Data.Database.Source), &gorm.Config{})
		if err != nil {
			return nil, err
		}
		backendBridge, err := bridge.NewRuntime(db)
		if err != nil {
			return nil, err
		}
		gw.db = db
		gw.backendBridge = backendBridge
	}

	type clientSetup struct {
		name  string
		setup func(addr string) error
	}
	setups := []clientSetup{
		{"user", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.userClient = userv1.NewUserServiceClient(conn)
			return nil
		}},
		{"question", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.questionClient = questionv1.NewQuestionServiceClient(conn)
			return nil
		}},
		{"interview", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.interviewClient = interviewv1.NewInterviewServiceClient(conn)
			return nil
		}},
		{"growth", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.growthClient = growthv1.NewGrowthServiceClient(conn)
			return nil
		}},
		{"admin", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.adminClient = adminv1.NewAdminServiceClient(conn)
			return nil
		}},
		{"community", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.communityClient = communityv1.NewCommunityServiceClient(conn)
			return nil
		}},
	}

	for _, s := range setups {
		if svc, ok := cfg.Services[s.name]; ok && svc.Addr != "" {
			if err := s.setup(svc.Addr); err != nil {
				gw.Close()
				return nil, err
			}
		}
	}

	return gw, nil
}

// Close 关闭所有连接
func (gw *Gateway) Close() {
	for _, conn := range gw.conns {
		conn.Close()
	}
	if gw.db != nil {
		if sqlDB, err := gw.db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
}

// grpcErrorToHTTP 将 gRPC 状态码映射为 HTTP 状态码
func grpcErrorToHTTP(err error) (int, string) {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusInternalServerError, "internal error"
	}
	switch st.Code() {
	case codes.NotFound:
		return http.StatusNotFound, st.Message()
	case codes.InvalidArgument, codes.FailedPrecondition:
		return http.StatusBadRequest, st.Message()
	case codes.Unauthenticated:
		return http.StatusUnauthorized, st.Message()
	case codes.PermissionDenied:
		return http.StatusForbidden, st.Message()
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict, st.Message()
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests, st.Message()
	default:
		return http.StatusInternalServerError, "internal error"
	}
}

// parseID 解析路径参数中的 ID，无效时返回错误响应
func parseID(c *gin.Context, param string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + param})
		return 0, false
	}
	return id, true
}

// getUserID 从上下文中获取用户 ID，缺失时返回未认证错误
func getUserID(c *gin.Context) (uint64, bool) {
	val, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return 0, false
	}
	switch typedValue := val.(type) {
	case uint64:
		return typedValue, true
	case uint:
		return uint64(typedValue), true
	case int:
		if typedValue > 0 {
			return uint64(typedValue), true
		}
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
	return 0, false
}

// grpcErr 处理 gRPC 错误并返回 HTTP 响应
func grpcErr(c *gin.Context, err error) {
	code, msg := grpcErrorToHTTP(err)
	c.JSON(code, gin.H{"error": msg})
}

// RegisterRoutes 注册 HTTP 路由（对齐 backend 单体路由）
func (gw *Gateway) RegisterRoutes(r *gin.Engine) {
	gw.registerSystemRoutes(r)

	if gw.backendBridge != nil {
		gw.backendBridge.RegisterGatewayRoutes(r, gw.LegacyOptionalJWTMiddleware(), gw.LegacyAuthMiddleware(), nil)
		return
	}

	api := r.Group("/api")

	// ========== 公开接口（无需认证，对齐 backend OptionalAuth） ==========
	public := api.Group("")
	{
		// 认证
		if gw.userClient != nil {
			public.POST("/auth/register", gw.handleRegister)
			public.POST("/auth/login", gw.handleLogin)
			public.POST("/auth/refresh", gw.handleRefreshToken)
		}
		// 题库公开接口
		if gw.questionClient != nil {
			public.GET("/questions", gw.handleListQuestions)
			public.GET("/questions/:id", gw.handleGetQuestion)
			public.GET("/industries", gw.handleListIndustries)
			public.GET("/categories", gw.handleListCategories)
		}
		// 社区公开接口
		if gw.communityClient != nil {
			public.GET("/community/posts", gw.handleListPosts)
			public.GET("/community/posts/:id", gw.handleGetPost)
			public.GET("/community/posts/:id/comments", gw.handleListComments)
		}
	}

	// ========== 需要认证的接口 ==========
	protected := api.Group("")
	protected.Use(gw.JWTMiddleware())
	{
		// --- 用户 ---
		if gw.userClient != nil {
			protected.GET("/auth/me", gw.handleGetProfile)
			protected.GET("/user/profile", gw.handleGetProfile)
			protected.PUT("/user/profile", gw.handleUpdateProfile)
		}

		// --- 题库（需认证部分） ---
		if gw.questionClient != nil {
			protected.POST("/questions/:id/submit", gw.handleSubmitAnswer)
			protected.POST("/questions/:id/run", gw.handleRunCode)
			protected.POST("/questions/:id/favorite", gw.handleToggleFavorite)
			protected.GET("/user/favorites", gw.handleListFavorites)
			protected.GET("/user/wrong-questions", gw.handleGetWrongQuestions)
			protected.GET("/user/notes", gw.handleListNotes)
			protected.POST("/user/notes", gw.handleCreateNote)
			protected.PUT("/user/notes/:id", gw.handleUpdateNote)
			protected.DELETE("/user/notes/:id", gw.handleDeleteNote)
			protected.GET("/user/practice-stats", gw.handleGetPracticeStats)
			protected.GET("/user/practice-recommendations", gw.handleGetPracticeRecommendations)
			protected.POST("/exams/random", gw.handleGetRandomExam)
		}

		// --- 面试 ---
		if gw.interviewClient != nil {
			protected.POST("/interviews", gw.handleCreateInterview)
			protected.GET("/interviews", gw.handleListInterviews)
			protected.GET("/interviews/:id", gw.handleGetInterview)
			protected.POST("/interviews/:id/answer", gw.handleSubmitInterviewAnswer)
			protected.GET("/interviews/:id/next", gw.handleGetNextQuestion)
			protected.POST("/interviews/:id/finish", gw.handleFinishInterview)
			protected.GET("/interviews/:id/report", gw.handleGetReport)
			protected.POST("/interviews/:id/coding", gw.handleSubmitCodingAnswer)
		}

		// --- 学习计划 & 成长 ---
		if gw.growthClient != nil {
			protected.POST("/plans", gw.handleCreatePlan)
			protected.GET("/plans/current", gw.handleGetCurrentPlan)
			protected.GET("/plans/:id", gw.handleGetPlan)
			protected.PUT("/plans/:id/tasks/:taskId", gw.handleUpdateTaskStatus)
			protected.POST("/plans/:id/tasks/:taskId/feedback", gw.handleSubmitTaskFeedback)
			protected.PUT("/user/study-logs/daily", gw.handleSyncStudyLog)
			protected.GET("/user/growth-summary", gw.handleGetGrowthSummary)
			protected.GET("/user/weekly-focus", gw.handleGetWeeklyFocus)
			protected.POST("/companion/chat", gw.handleCompanionChat)
		}

		// --- 社区（需认证部分） ---
		if gw.communityClient != nil {
			protected.POST("/community/posts", gw.handleCreatePost)
			protected.PUT("/community/posts/:id", gw.handleUpdatePost)
			protected.DELETE("/community/posts/:id", gw.handleDeletePost)
			protected.POST("/community/posts/:id/comments", gw.handleCreateComment)
			protected.POST("/community/posts/:id/like", gw.handleToggleLike)
		}

		// --- 管理后台 ---
		if gw.adminClient != nil {
			admin := protected.Group("/admin")
			admin.Use(gw.AdminMiddleware())
			{
				// 仪表盘
				admin.GET("/dashboard", gw.handleAdminGetDashboard)

				// 用户管理
				admin.GET("/users", gw.handleAdminListUsers)
				admin.PUT("/users/:id/role", gw.handleAdminUpdateUserRole)
				admin.PUT("/users/:id/disable", gw.handleAdminDisableUser)

				// 题库管理
				admin.GET("/questions", gw.handleAdminListQuestions)
				admin.POST("/questions", gw.handleAdminCreateQuestion)
				admin.PUT("/questions/:id", gw.handleAdminUpdateQuestion)
				admin.DELETE("/questions/:id", gw.handleAdminDeleteQuestion)
				admin.POST("/questions/import", gw.handleAdminBatchImportQuestions)
				admin.GET("/questions/tag-taxonomy", gw.handleAdminGetQuestionTagTaxonomy)

				// 题目流水线
				admin.POST("/question-pipeline/generate", gw.handleAdminGenerateQuestionPipeline)
				admin.POST("/question-pipeline/generate/async", gw.handleAdminGenerateQuestionPipelineAsync)
				admin.POST("/question-pipeline/import", gw.handleAdminImportQuestionPipeline)

				// 分类管理
				admin.GET("/categories", gw.handleAdminListCategories)
				admin.POST("/categories", gw.handleAdminCreateCategory)
				admin.PUT("/categories/:id", gw.handleAdminUpdateCategory)
				admin.DELETE("/categories/:id", gw.handleAdminDeleteCategory)

				// 行业管理
				admin.GET("/industries", gw.handleAdminListIndustries)
				admin.POST("/industries", gw.handleAdminCreateIndustry)
				admin.PUT("/industries/:id", gw.handleAdminUpdateIndustry)

				// Prompt 模板
				admin.GET("/prompt-templates", gw.handleAdminListPromptTemplates)
				admin.POST("/prompt-templates", gw.handleAdminSavePromptTemplate)
				admin.POST("/prompts", gw.handleAdminCreatePrompt)
				admin.PUT("/prompts/:id", gw.handleAdminUpdatePrompt)
				admin.DELETE("/prompts/:id", gw.handleAdminDeletePrompt)
				admin.POST("/prompts/test-render", gw.handleAdminTestRenderPrompt)

				// AI 配置
				admin.GET("/ai-configs", gw.handleAdminGetAIConfigs)
				admin.PUT("/ai-configs", gw.handleAdminUpdateAIConfigs)

				// AI 预设
				admin.GET("/ai-config-presets", gw.handleAdminListAIPresets)
				admin.POST("/ai-config-presets", gw.handleAdminCreateAIPreset)
				admin.PUT("/ai-config-presets/:id", gw.handleAdminUpdateAIPreset)
				admin.DELETE("/ai-config-presets/:id", gw.handleAdminDeleteAIPreset)
				admin.POST("/ai-config-presets/:id/apply", gw.handleAdminApplyAIPreset)

				// AI 调试 & 日志
				admin.POST("/ai/debug", gw.handleAdminDebugAI)
				admin.GET("/ai-call-logs", gw.handleAdminListAICallLogs)
				admin.GET("/ai-call-logs/:id", gw.handleAdminGetAICallLog)

				// Live2D 管理
				admin.GET("/live2d-models", gw.handleAdminListLive2DModels)
				admin.POST("/live2d-models", gw.handleAdminCreateLive2DModel)
				admin.PUT("/live2d-models/:id", gw.handleAdminUpdateLive2DModel)
				admin.DELETE("/live2d-models/:id", gw.handleAdminDeleteLive2DModel)
				admin.POST("/live2d-models/import", gw.handleAdminImportLive2DPackage)
				admin.POST("/live2d-models/backgrounds/import", gw.handleAdminImportLive2DBackground)

				// TTS 管理
				admin.GET("/tts-configs", gw.handleAdminListTTSConfigs)
				admin.POST("/tts-configs", gw.handleAdminCreateTTSConfig)
				admin.PUT("/tts-configs/:id", gw.handleAdminUpdateTTSConfig)
				admin.DELETE("/tts-configs/:id", gw.handleAdminDeleteTTSConfig)
				admin.PUT("/tts-configs/defaults", gw.handleAdminUpdateTTSSceneDefaults)

				// RAG 配置
				admin.GET("/rag-configs", gw.handleAdminGetRAGConfigs)
				admin.PUT("/rag-configs", gw.handleAdminUpdateRAGConfigs)
				admin.POST("/rag-configs/test", gw.handleAdminTestRAGConnection)

				// RAG 索引
				admin.POST("/rag/index-all", gw.handleAdminIndexAllQuestions)
				admin.POST("/rag/index", gw.handleAdminIndexQuestions)
				admin.DELETE("/rag/index", gw.handleAdminDeleteRAGIndex)
				admin.GET("/rag/search", gw.handleAdminSearchRAGQuestions)

				// RAG 文档
				admin.GET("/rag-documents", gw.handleAdminListRAGDocuments)
				admin.GET("/rag-documents/stats", gw.handleAdminGetRAGDocumentStats)
				admin.GET("/rag-documents/:id", gw.handleAdminGetRAGDocument)
				admin.POST("/rag-documents", gw.handleAdminCreateRAGDocument)
				admin.PUT("/rag-documents/:id", gw.handleAdminUpdateRAGDocument)
				admin.DELETE("/rag-documents/:id", gw.handleAdminDeleteRAGDocument)
				admin.POST("/rag-documents/batch-import", gw.handleAdminBatchImportRAGDocuments)
				admin.POST("/rag-documents/sync", gw.handleAdminSyncRAGDocuments)
				admin.POST("/rag-documents/sync-all", gw.handleAdminSyncAllPendingRAGDocuments)

				// 面经爬虫
				admin.GET("/scraper/sources", gw.handleAdminGetScraperSources)
				admin.POST("/scraper/search", gw.handleAdminScraperSearch)
				admin.POST("/scraper/fetch", gw.handleAdminScraperFetch)
				admin.POST("/scraper/clean", gw.handleAdminScraperClean)
				admin.POST("/scraper/import", gw.handleAdminScraperImport)
				admin.POST("/scraper/import/async", gw.handleAdminScraperImportAsync)
				admin.GET("/scraper/tasks", gw.handleAdminListScraperTasks)
				admin.GET("/scraper/tasks/:id", gw.handleAdminGetScraperTask)
				admin.POST("/scraper/tasks/:id/retry", gw.handleAdminRetryScraperTask)

				// 系统配置
				admin.GET("/configs/:key", gw.handleAdminGetConfig)
				admin.PUT("/configs/:key", gw.handleAdminSetConfig)
			}
		}
	}
}

// ========== 中间件 ==========

// JWTMiddleware JWT 认证中间件
func (gw *Gateway) JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractAccessToken(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		claims, err := auth.ParseToken(tokenString, gw.jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		injectIdentityContext(c, claims)
		c.Next()
	}
}

// OptionalJWTMiddleware 在保留匿名访问的同时尽量补齐已登录用户上下文。
func (gw *Gateway) OptionalJWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractAccessToken(c.Request)
		if err == nil && tokenString != "" {
			if claims, parseErr := auth.ParseToken(tokenString, gw.jwtSecret); parseErr == nil {
				injectIdentityContext(c, claims)
			}
		}
		c.Next()
	}
}

// LegacyAuthMiddleware 复用 gateway 的 JWT 配置，但保持单体鉴权失败时的响应格式。
func (gw *Gateway) LegacyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractAccessToken(c.Request)
		if err != nil {
			legacyUnauthorized(c, err.Error())
			c.Abort()
			return
		}

		claims, err := auth.ParseToken(tokenString, gw.jwtSecret)
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				legacyError(c, http.StatusOK, legacyCodeTokenExpired, "令牌已过期", nil)
			} else {
				legacyError(c, http.StatusOK, legacyCodeTokenInvalid, "无效的令牌", nil)
			}
			c.Abort()
			return
		}

		injectIdentityContext(c, claims)
		c.Next()
	}
}

// LegacyOptionalJWTMiddleware 在公开接口上保留单体 OptionalAuth 的透传行为。
func (gw *Gateway) LegacyOptionalJWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractAccessToken(c.Request)
		if err == nil && tokenString != "" {
			if claims, parseErr := auth.ParseToken(tokenString, gw.jwtSecret); parseErr == nil {
				injectIdentityContext(c, claims)
			}
		}
		c.Next()
	}
}

// AdminMiddleware 检查用户是否具有管理员角色
func (gw *Gateway) AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role.(string) != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// extractAccessToken 统一提取 Bearer Token，并兼容 WebSocket 握手时通过 Query 透传 token。
func extractAccessToken(r *http.Request) (string, error) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader != "" {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return "", errors.New("Authorization格式错误，应为Bearer {token}")
		}
		if strings.TrimSpace(tokenString) == "" {
			return "", errors.New("Authorization格式错误，应为Bearer {token}")
		}
		return strings.TrimSpace(tokenString), nil
	}

	if isWebSocketUpgradeRequest(r) {
		for _, key := range []string{"token", "access_token"} {
			if token := strings.TrimSpace(r.URL.Query().Get(key)); token != "" {
				return token, nil
			}
		}
	}

	return "", errors.New("缺少Authorization请求头")
}

// isWebSocketUpgradeRequest 判断当前请求是否为 WebSocket 升级请求。
func isWebSocketUpgradeRequest(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket")
}

// injectIdentityContext 将网关解析出的 JWT 声明写回 Gin 上下文，并兼容 legacy handler 依赖的 user_id 类型。
func injectIdentityContext(c *gin.Context, claims *auth.Claims) {
	userID := safeLegacyUserID(claims.UserID)
	c.Set("user_id", userID)
	c.Set("role", claims.Role)
	c.Set("email", claims.Email)

	ctx := context.WithValue(c.Request.Context(), auth.ContextKeyUserID, claims.UserID)
	c.Request = c.Request.WithContext(ctx)
}

// safeLegacyUserID 将 JWT 中的 uint64 用户 ID 安全收敛为 legacy handler 可读取的 uint。
func safeLegacyUserID(userID uint64) uint {
	maxUint := ^uint(0)
	if uint64(maxUint) < userID {
		return maxUint
	}
	return uint(userID)
}

// registerSystemRoutes 注册与单体保持一致的基础健康检查与重定向路由。
func (gw *Gateway) registerSystemRoutes(engine *gin.Engine) {
	engine.GET("/api/health", gw.handleHealthLiveness)
	engine.GET("/api/health/ready", gw.handleHealthReadiness)
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	engine.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/api/health")
	})
}

// handleHealthLiveness 返回轻量级存活探针，响应结构与单体保持一致。
func (gw *Gateway) handleHealthLiveness(c *gin.Context) {
	legacySuccess(c, map[string]any{
		"status":    "ok",
		"version":   "",
		"timestamp": time.Now().Unix(),
	})
}

// handleHealthReadiness 检查 gateway 侧 bridge 复用数据库是否可达，保持与单体一致的 ready 语义。
func (gw *Gateway) handleHealthReadiness(c *gin.Context) {
	checks := map[string]string{
		"database": "not configured",
		"redis":    "not configured",
	}

	if gw.db != nil {
		sqlDB, err := gw.db.DB()
		if err != nil {
			checks["database"] = "error: " + err.Error()
			legacyError(c, http.StatusServiceUnavailable, http.StatusInternalServerError, "service not ready", gin.H{"checks": checks})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			checks["database"] = "unreachable: " + err.Error()
			legacyError(c, http.StatusServiceUnavailable, http.StatusInternalServerError, "service not ready", gin.H{"checks": checks})
			return
		}
		checks["database"] = "ok"
	}

	legacySuccess(c, map[string]any{
		"status": "ok",
		"checks": checks,
	})
}

// legacyUnauthorized 以单体相同结构返回 401 未授权响应。
func legacyUnauthorized(c *gin.Context, message string) {
	legacyError(c, http.StatusUnauthorized, legacyCodeUnauthorized, message, nil)
}

// legacyError 以单体相同结构返回错误响应。
func legacyError(c *gin.Context, httpStatus int, code int, message string, data interface{}) {
	c.JSON(httpStatus, legacyResponse{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// legacySuccess 以单体相同结构返回成功响应。
func legacySuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, legacyResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// ========== User 代理 ==========

func (gw *Gateway) handleRegister(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username, email and password are required"})
		return
	}
	if !strings.Contains(req.Email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email format"})
		return
	}
	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 6 characters"})
		return
	}
	resp, err := gw.userClient.Register(c.Request.Context(), &userv1.RegisterRequest{
		Username: req.Username, Email: req.Email, Password: req.Password,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.userClient.Login(c.Request.Context(), &userv1.LoginRequest{
		Email: req.Email, Password: req.Password,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleRefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.userClient.RefreshToken(c.Request.Context(), &userv1.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetProfile(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.userClient.GetProfile(c.Request.Context(), &userv1.UserIDRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleUpdateProfile(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.userClient.UpdateProfile(c.Request.Context(), &userv1.UpdateProfileRequest{
		UserId: userID, Username: req.Username, Avatar: req.Avatar,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetMembershipStatus(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.userClient.GetMembershipStatus(c.Request.Context(), &userv1.UserIDRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleUpgradeMembership(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Plan string `json:"plan"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.userClient.UpgradeMembership(c.Request.Context(), &userv1.UpgradeRequest{
		UserId: userID, Plan: req.Plan,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ========== Question 代理 ==========

func (gw *Gateway) handleListQuestions(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	var categoryID uint64
	if cid := c.Query("category_id"); cid != "" {
		categoryID, _ = strconv.ParseUint(cid, 10, 64)
	}
	resp, err := gw.questionClient.ListQuestions(c.Request.Context(), &questionv1.ListQuestionsRequest{
		IndustryCode: c.Query("industry_code"),
		Difficulty:   c.Query("difficulty"),
		CategoryId:   categoryID,
		Keyword:      c.Query("keyword"),
		Page:         &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetQuestion(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.questionClient.GetQuestion(c.Request.Context(), &questionv1.GetQuestionRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleListIndustries(c *gin.Context) {
	resp, err := gw.questionClient.ListIndustries(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleListCategories(c *gin.Context) {
	resp, err := gw.questionClient.ListCategories(c.Request.Context(), &questionv1.ListCategoriesRequest{
		IndustryCode: c.Query("industry_code"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleSubmitAnswer(c *gin.Context) {
	questionID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Answer   string `json:"answer"`
		Language string `json:"language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.questionClient.SubmitAnswer(c.Request.Context(), &questionv1.SubmitAnswerRequest{
		QuestionId: questionID, UserId: userID, Answer: req.Answer, Language: req.Language,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleRunCode(c *gin.Context) {
	questionID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Language string `json:"language"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.questionClient.RunCode(c.Request.Context(), &questionv1.RunCodeRequest{
		QuestionId: questionID, Language: req.Language, Code: req.Code,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleToggleFavorite(c *gin.Context) {
	questionID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	// 先尝试创建，如果已存在则删除（toggle 语义）
	_, err := gw.questionClient.CreateFavorite(c.Request.Context(), &questionv1.CreateFavoriteRequest{
		UserId: userID, QuestionId: questionID,
	})
	if err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.AlreadyExists {
			_, err = gw.questionClient.DeleteFavorite(c.Request.Context(), &questionv1.DeleteFavoriteRequest{
				UserId: userID, QuestionId: questionID,
			})
			if err != nil {
				grpcErr(c, err)
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "removed"})
			return
		}
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "added"})
}

func (gw *Gateway) handleListFavorites(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.questionClient.ListFavorites(c.Request.Context(), &questionv1.ListFavoritesRequest{
		UserId: userID,
		Page:   &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetWrongQuestions(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.questionClient.GetWrongQuestions(c.Request.Context(), &questionv1.WrongQuestionRequest{
		UserId:       userID,
		IndustryCode: c.Query("industry_code"),
		Page:         &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleListNotes(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	var questionID uint64
	if qid := c.Query("question_id"); qid != "" {
		questionID, _ = strconv.ParseUint(qid, 10, 64)
	}
	resp, err := gw.questionClient.ListNotes(c.Request.Context(), &questionv1.ListNotesRequest{
		UserId:     userID,
		QuestionId: questionID,
		Page:       &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleCreateNote(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		QuestionID uint64 `json:"question_id"`
		Content    string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.questionClient.CreateNote(c.Request.Context(), &questionv1.CreateNoteRequest{
		UserId: userID, QuestionId: req.QuestionID, Content: req.Content,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleUpdateNote(c *gin.Context) {
	noteID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.questionClient.UpdateNote(c.Request.Context(), &questionv1.UpdateNoteRequest{
		Id: noteID, UserId: userID, Content: req.Content,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleDeleteNote(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "delete note not yet implemented in proto"})
}

func (gw *Gateway) handleGetPracticeStats(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.questionClient.GetUserPracticeStats(c.Request.Context(), &questionv1.UserIDRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetPracticeRecommendations(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.questionClient.GetPracticeRecommendations(c.Request.Context(), &questionv1.UserIDRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetRandomExam(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		IndustryCode  string   `json:"industry_code"`
		QuestionCount int32    `json:"question_count"`
		Categories    []string `json:"categories"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.questionClient.GetRandomExam(c.Request.Context(), &questionv1.RandomExamRequest{
		UserId: userID, IndustryCode: req.IndustryCode,
		QuestionCount: req.QuestionCount, Categories: req.Categories,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ========== Interview 代理 ==========

func (gw *Gateway) handleCreateInterview(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		IndustryCode   string   `json:"industry_code"`
		Difficulty     string   `json:"difficulty"`
		Topics         []string `json:"topics"`
		QuestionCount  int32    `json:"question_count"`
		InterviewMode  string   `json:"interview_mode"`
		ResumeText     string   `json:"resume_text"`
		JobDescription string   `json:"job_description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.interviewClient.CreateInterview(c.Request.Context(), &interviewv1.CreateInterviewRequest{
		UserId: userID, IndustryCode: req.IndustryCode, Difficulty: req.Difficulty,
		Topics: req.Topics, QuestionCount: req.QuestionCount, InterviewMode: req.InterviewMode,
		ResumeText: req.ResumeText, JobDescription: req.JobDescription,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleListInterviews(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.interviewClient.ListInterviews(c.Request.Context(), &interviewv1.ListInterviewsRequest{
		UserId: userID,
		Page:   &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetInterview(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.interviewClient.GetInterview(c.Request.Context(), &interviewv1.GetInterviewRequest{
		InterviewId: id, UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleSubmitInterviewAnswer(c *gin.Context) {
	interviewID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		QuestionIndex int32  `json:"question_index"`
		Answer        string `json:"answer"`
		Language      string `json:"language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.interviewClient.SubmitAnswer(c.Request.Context(), &interviewv1.SubmitAnswerRequest{
		InterviewId: interviewID, UserId: userID,
		QuestionIndex: req.QuestionIndex, Answer: req.Answer, Language: req.Language,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetNextQuestion(c *gin.Context) {
	interviewID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	currentIndex, _ := strconv.ParseInt(c.DefaultQuery("current_index", "0"), 10, 32)
	resp, err := gw.interviewClient.GetNextQuestion(c.Request.Context(), &interviewv1.GetNextQuestionRequest{
		InterviewId: interviewID, UserId: userID, CurrentIndex: int32(currentIndex),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleFinishInterview(c *gin.Context) {
	interviewID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.interviewClient.FinishInterview(c.Request.Context(), &interviewv1.FinishInterviewRequest{
		InterviewId: interviewID, UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetReport(c *gin.Context) {
	interviewID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.interviewClient.GetReport(c.Request.Context(), &interviewv1.GetReportRequest{
		InterviewId: interviewID, UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleSubmitCodingAnswer(c *gin.Context) {
	interviewID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		QuestionIndex int32  `json:"question_index"`
		Language      string `json:"language"`
		Code          string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.interviewClient.SubmitCodingAnswer(c.Request.Context(), &interviewv1.SubmitCodingRequest{
		InterviewId: interviewID, UserId: userID,
		QuestionIndex: req.QuestionIndex, Language: req.Language, Code: req.Code,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ========== Growth 代理 ==========

func (gw *Gateway) handleGetGrowthSummary(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.growthClient.GetGrowthSummary(c.Request.Context(), &growthv1.UserIDRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetWeeklyFocus(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.growthClient.GetWeeklyFocus(c.Request.Context(), &growthv1.UserIDRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleSyncStudyLog(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Action          string `json:"action"`
		RefID           uint64 `json:"ref_id"`
		DurationSeconds int32  `json:"duration_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.growthClient.SyncStudyLog(c.Request.Context(), &growthv1.SyncStudyLogRequest{
		UserId: userID, Action: req.Action, RefId: req.RefID, DurationSeconds: req.DurationSeconds,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleCreatePlan(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		IndustryCode string `json:"industry_code"`
		Goal         string `json:"goal"`
		DailyHours   int32  `json:"daily_hours"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.growthClient.CreatePlan(c.Request.Context(), &growthv1.CreatePlanRequest{
		UserId: userID, IndustryCode: req.IndustryCode, Goal: req.Goal, DailyHours: req.DailyHours,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetCurrentPlan(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	// GetPlan with plan_id=0 means "get current/latest plan"
	resp, err := gw.growthClient.GetPlan(c.Request.Context(), &growthv1.GetPlanRequest{
		PlanId: 0, UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetPlan(c *gin.Context) {
	planID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.growthClient.GetPlan(c.Request.Context(), &growthv1.GetPlanRequest{
		PlanId: planID, UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleUpdateTaskStatus(c *gin.Context) {
	taskID, ok := parseID(c, "taskId")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.growthClient.UpdateTaskStatus(c.Request.Context(), &growthv1.UpdateTaskStatusRequest{
		TaskId: taskID, UserId: userID, Status: req.Status,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleSubmitTaskFeedback(c *gin.Context) {
	taskID, ok := parseID(c, "taskId")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Content          string `json:"content"`
		DifficultyRating int32  `json:"difficulty_rating"`
		ConfidenceRating int32  `json:"confidence_rating"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.growthClient.SubmitTaskFeedback(c.Request.Context(), &growthv1.SubmitFeedbackRequest{
		TaskId: taskID, UserId: userID, Content: req.Content,
		DifficultyRating: req.DifficultyRating, ConfidenceRating: req.ConfidenceRating,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleCompanionChat(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Message     string `json:"message"`
		ContextType string `json:"context_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.growthClient.Chat(c.Request.Context(), &growthv1.CompanionChatRequest{
		UserId: userID, Message: req.Message, ContextType: req.ContextType,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ========== Community 代理 ==========

func (gw *Gateway) handleListPosts(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.communityClient.ListPosts(c.Request.Context(), &communityv1.ListPostsRequest{
		Category: c.Query("category"),
		Page:     &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetPost(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.communityClient.GetPost(c.Request.Context(), &communityv1.GetPostRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleCreatePost(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	resp, err := gw.communityClient.CreatePost(c.Request.Context(), &communityv1.CreatePostRequest{
		AuthorId: userID, Title: req.Title, Content: req.Content, Category: req.Category,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleUpdatePost(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.communityClient.UpdatePost(c.Request.Context(), &communityv1.UpdatePostRequest{
		Id: id, AuthorId: userID, Title: req.Title, Content: req.Content,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleDeletePost(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	_, err := gw.communityClient.DeletePost(c.Request.Context(), &communityv1.DeletePostRequest{Id: id, AuthorId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleListComments(c *gin.Context) {
	postID, ok := parseID(c, "id")
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.communityClient.ListComments(c.Request.Context(), &communityv1.ListCommentsRequest{
		PostId: postID,
		Page:   &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleCreateComment(c *gin.Context) {
	postID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.communityClient.CreateComment(c.Request.Context(), &communityv1.CreateCommentRequest{
		PostId: postID, AuthorId: userID, Content: req.Content,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleToggleLike(c *gin.Context) {
	postID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.communityClient.ToggleLike(c.Request.Context(), &communityv1.ToggleLikeRequest{
		UserId: userID, PostId: postID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ========== Admin 代理 ==========

func (gw *Gateway) handleAdminListUsers(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.adminClient.ListUsers(c.Request.Context(), &adminv1.ListUsersRequest{
		Keyword: c.Query("keyword"),
		Page:    &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateUserRole(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateUserRole(c.Request.Context(), &adminv1.UpdateUserRoleRequest{
		UserId: id, Role: req.Role,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminListAIPresets(c *gin.Context) {
	resp, err := gw.adminClient.ListAIPresets(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminSaveAIPreset(c *gin.Context) {
	var req struct {
		ID       uint64            `json:"id"`
		Name     string            `json:"name"`
		Provider string            `json:"provider"`
		Model    string            `json:"model"`
		Params   map[string]string `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.SaveAIPreset(c.Request.Context(), &adminv1.SaveAIPresetRequest{
		Id: req.ID, Name: req.Name, Provider: req.Provider, Model: req.Model, Params: req.Params,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminListPromptTemplates(c *gin.Context) {
	resp, err := gw.adminClient.ListPromptTemplates(c.Request.Context(), &adminv1.ListPromptTemplatesRequest{
		IndustryCode: c.Query("industry_code"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminSavePromptTemplate(c *gin.Context) {
	var req struct {
		ID           uint64 `json:"id"`
		Name         string `json:"name"`
		IndustryCode string `json:"industry_code"`
		TemplateType string `json:"template_type"`
		Content      string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.SavePromptTemplate(c.Request.Context(), &adminv1.SavePromptTemplateRequest{
		Id: req.ID, Name: req.Name, IndustryCode: req.IndustryCode,
		TemplateType: req.TemplateType, Content: req.Content,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminGetConfig(c *gin.Context) {
	key := c.Param("key")
	resp, err := gw.adminClient.GetAdminConfig(c.Request.Context(), &adminv1.GetAdminConfigRequest{Key: key})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminSetConfig(c *gin.Context) {
	key := c.Param("key")
	var req struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.SetAdminConfig(c.Request.Context(), &adminv1.SetAdminConfigRequest{
		Key: key, Value: req.Value,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDebugAI(c *gin.Context) {
	var req struct {
		AgentType string            `json:"agent_type"`
		Prompt    string            `json:"prompt"`
		Params    map[string]string `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.DebugAI(c.Request.Context(), &adminv1.DebugAIRequest{
		AgentType: req.AgentType, Prompt: req.Prompt, Params: req.Params,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleAdminListAICallLogs 在无 bridge 直挂时，将单体后台已有的日志筛选参数透传给 admin gRPC。
func (gw *Gateway) handleAdminListAICallLogs(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)

	var taskID uint64
	if rawTaskID := strings.TrimSpace(c.Query("task_id")); rawTaskID != "" {
		parsedTaskID, err := strconv.ParseUint(rawTaskID, 10, 64)
		if err != nil || parsedTaskID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id"})
			return
		}
		taskID = parsedTaskID
	}
	resp, err := gw.adminClient.ListAICallLogs(c.Request.Context(), &adminv1.ListAICallLogsRequest{
		Page:      &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
		AgentType: c.Query("agent_type"),
		Scene:     c.Query("scene"),
		Source:    c.Query("source"),
		Status:    c.Query("status"),
		TraceId:   c.Query("trace_id"),
		TaskId:    taskID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: 仪表盘 ====================

func (gw *Gateway) handleAdminGetDashboard(c *gin.Context) {
	resp, err := gw.adminClient.GetDashboard(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: 用户管理 ====================

func (gw *Gateway) handleAdminDisableUser(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DisableUser(c.Request.Context(), &adminv1.DisableUserRequest{UserId: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// ==================== Admin: 题库管理 ====================

func (gw *Gateway) handleAdminListQuestions(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	var categoryID uint64
	if cid := c.Query("category_id"); cid != "" {
		categoryID, _ = strconv.ParseUint(cid, 10, 64)
	}
	resp, err := gw.adminClient.AdminListQuestions(c.Request.Context(), &adminv1.AdminListQuestionsRequest{
		Page:         &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
		Keyword:      c.Query("keyword"),
		Difficulty:   c.Query("difficulty"),
		CategoryId:   categoryID,
		IndustryCode: c.Query("industry_code"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminCreateQuestion(c *gin.Context) {
	var req struct {
		CategoryID         uint64 `json:"category_id"`
		IndustryID         uint64 `json:"industry_id"`
		Type               string `json:"type"`
		Difficulty         string `json:"difficulty"`
		Title              string `json:"title"`
		Content            string `json:"content"`
		OptionsJSON        string `json:"options_json"`
		Answer             string `json:"answer"`
		Explanation        string `json:"explanation"`
		SolutionJSON       string `json:"solution_json"`
		JudgeConfigJSON    string `json:"judge_config_json"`
		AnswerTemplateJSON string `json:"answer_template_json"`
		Tags               string `json:"tags"`
		IsActive           bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateQuestion(c.Request.Context(), &adminv1.CreateQuestionRequest{
		CategoryId: req.CategoryID, IndustryId: req.IndustryID, Type: req.Type,
		Difficulty: req.Difficulty, Title: req.Title, Content: req.Content,
		OptionsJson: req.OptionsJSON, Answer: req.Answer, Explanation: req.Explanation,
		SolutionJson: req.SolutionJSON, JudgeConfigJson: req.JudgeConfigJSON,
		AnswerTemplateJson: req.AnswerTemplateJSON, Tags: req.Tags, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateQuestion(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		CategoryID         uint64 `json:"category_id"`
		IndustryID         uint64 `json:"industry_id"`
		Type               string `json:"type"`
		Difficulty         string `json:"difficulty"`
		Title              string `json:"title"`
		Content            string `json:"content"`
		OptionsJSON        string `json:"options_json"`
		Answer             string `json:"answer"`
		Explanation        string `json:"explanation"`
		SolutionJSON       string `json:"solution_json"`
		JudgeConfigJSON    string `json:"judge_config_json"`
		AnswerTemplateJSON string `json:"answer_template_json"`
		Tags               string `json:"tags"`
		IsActive           *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateQuestion(c.Request.Context(), &adminv1.UpdateQuestionRequest{
		Id: id, CategoryId: req.CategoryID, IndustryId: req.IndustryID,
		Type: req.Type, Difficulty: req.Difficulty, Title: req.Title,
		Content: req.Content, OptionsJson: req.OptionsJSON, Answer: req.Answer,
		Explanation: req.Explanation, SolutionJson: req.SolutionJSON,
		JudgeConfigJson: req.JudgeConfigJSON, AnswerTemplateJson: req.AnswerTemplateJSON,
		Tags: req.Tags, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDeleteQuestion(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeleteQuestion(c.Request.Context(), &adminv1.DeleteQuestionRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleAdminBatchImportQuestions(c *gin.Context) {
	var req struct {
		IndustryCode string `json:"industry_code"`
		Questions    []struct {
			CategoryName       string `json:"category_name"`
			Type               string `json:"type"`
			Difficulty         string `json:"difficulty"`
			Title              string `json:"title"`
			Content            string `json:"content"`
			OptionsJSON        string `json:"options_json"`
			Answer             string `json:"answer"`
			Explanation        string `json:"explanation"`
			SolutionJSON       string `json:"solution_json"`
			JudgeConfigJSON    string `json:"judge_config_json"`
			AnswerTemplateJSON string `json:"answer_template_json"`
			Tags               string `json:"tags"`
		} `json:"questions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items := make([]*adminv1.ImportQuestionItem, len(req.Questions))
	for i, q := range req.Questions {
		items[i] = &adminv1.ImportQuestionItem{
			CategoryName: q.CategoryName, Type: q.Type, Difficulty: q.Difficulty,
			Title: q.Title, Content: q.Content, OptionsJson: q.OptionsJSON,
			Answer: q.Answer, Explanation: q.Explanation, SolutionJson: q.SolutionJSON,
			JudgeConfigJson: q.JudgeConfigJSON, AnswerTemplateJson: q.AnswerTemplateJSON, Tags: q.Tags,
		}
	}
	resp, err := gw.adminClient.BatchImportQuestions(c.Request.Context(), &adminv1.BatchImportQuestionsRequest{
		IndustryCode: req.IndustryCode, Questions: items,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminGetQuestionTagTaxonomy(c *gin.Context) {
	resp, err := gw.adminClient.GetQuestionTagTaxonomy(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: 题目流水线 ====================

func (gw *Gateway) handleAdminGenerateQuestionPipeline(c *gin.Context) {
	var req struct {
		IndustryCode     string   `json:"industry_code"`
		Requirement      string   `json:"requirement"`
		AgentPrompt      string   `json:"agent_prompt"`
		GenerationMode   string   `json:"generation_mode"`
		CandidateCount   int32    `json:"candidate_count"`
		IncludeScraped   bool     `json:"include_scraped"`
		IncludeGenerated bool     `json:"include_generated"`
		Sources          []string `json:"sources"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.GenerateQuestionPipeline(c.Request.Context(), &adminv1.GenerateQuestionPipelineRequest{
		IndustryCode: req.IndustryCode, Requirement: req.Requirement,
		AgentPrompt: req.AgentPrompt, GenerationMode: req.GenerationMode,
		CandidateCount: req.CandidateCount, IncludeScraped: req.IncludeScraped,
		IncludeGenerated: req.IncludeGenerated, Sources: req.Sources,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminGenerateQuestionPipelineAsync(c *gin.Context) {
	var req struct {
		IndustryCode     string   `json:"industry_code"`
		Requirement      string   `json:"requirement"`
		AgentPrompt      string   `json:"agent_prompt"`
		GenerationMode   string   `json:"generation_mode"`
		CandidateCount   int32    `json:"candidate_count"`
		IncludeScraped   bool     `json:"include_scraped"`
		IncludeGenerated bool     `json:"include_generated"`
		Sources          []string `json:"sources"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.GenerateQuestionPipelineAsync(c.Request.Context(), &adminv1.GenerateQuestionPipelineRequest{
		IndustryCode: req.IndustryCode, Requirement: req.Requirement,
		AgentPrompt: req.AgentPrompt, GenerationMode: req.GenerationMode,
		CandidateCount: req.CandidateCount, IncludeScraped: req.IncludeScraped,
		IncludeGenerated: req.IncludeGenerated, Sources: req.Sources,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminImportQuestionPipeline(c *gin.Context) {
	var req struct {
		IndustryCode string `json:"industry_code"`
		Cards        []struct {
			Title       string   `json:"title"`
			Content     string   `json:"content"`
			Type        string   `json:"type"`
			Difficulty  string   `json:"difficulty"`
			Category    string   `json:"category"`
			Answer      string   `json:"answer"`
			Explanation string   `json:"explanation"`
			Tags        []string `json:"tags"`
		} `json:"cards"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cards := make([]*adminv1.PipelineCard, len(req.Cards))
	for i, c := range req.Cards {
		cards[i] = &adminv1.PipelineCard{
			Title: c.Title, Content: c.Content, Type: c.Type,
			Difficulty: c.Difficulty, Category: c.Category,
			Answer: c.Answer, Explanation: c.Explanation, Tags: c.Tags,
		}
	}
	resp, err := gw.adminClient.ImportQuestionPipeline(c.Request.Context(), &adminv1.ImportQuestionPipelineRequest{
		IndustryCode: req.IndustryCode, Cards: cards,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: 分类管理 ====================

func (gw *Gateway) handleAdminListCategories(c *gin.Context) {
	resp, err := gw.adminClient.AdminListCategories(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminCreateCategory(c *gin.Context) {
	var req struct {
		IndustryID  uint64 `json:"industry_id"`
		Name        string `json:"name"`
		ParentID    uint64 `json:"parent_id"`
		SortOrder   int32  `json:"sort_order"`
		Icon        string `json:"icon"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateCategory(c.Request.Context(), &adminv1.CreateCategoryRequest{
		IndustryId: req.IndustryID, Name: req.Name, ParentId: req.ParentID,
		SortOrder: req.SortOrder, Icon: req.Icon, Description: req.Description,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateCategory(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		IndustryID  uint64 `json:"industry_id"`
		Name        string `json:"name"`
		ParentID    uint64 `json:"parent_id"`
		SortOrder   int32  `json:"sort_order"`
		Icon        string `json:"icon"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateCategory(c.Request.Context(), &adminv1.UpdateCategoryRequest{
		Id: id, IndustryId: req.IndustryID, Name: req.Name, ParentId: req.ParentID,
		SortOrder: req.SortOrder, Icon: req.Icon, Description: req.Description,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDeleteCategory(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeleteCategory(c.Request.Context(), &adminv1.DeleteCategoryRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ==================== Admin: 行业管理 ====================

func (gw *Gateway) handleAdminListIndustries(c *gin.Context) {
	resp, err := gw.adminClient.AdminListIndustries(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminCreateIndustry(c *gin.Context) {
	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		SortOrder   int32  `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateIndustry(c.Request.Context(), &adminv1.CreateIndustryRequest{
		Code: req.Code, Name: req.Name, Description: req.Description,
		Icon: req.Icon, SortOrder: req.SortOrder,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateIndustry(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		IsActive    *bool  `json:"is_active"`
		SortOrder   int32  `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateIndustry(c.Request.Context(), &adminv1.UpdateIndustryRequest{
		Id: id, Code: req.Code, Name: req.Name, Description: req.Description,
		Icon: req.Icon, IsActive: req.IsActive, SortOrder: req.SortOrder,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// ==================== Admin: Prompt 模板 ====================

func (gw *Gateway) handleAdminCreatePrompt(c *gin.Context) {
	var req struct {
		IndustryID      uint64 `json:"industry_id"`
		Name            string `json:"name"`
		Scene           string `json:"scene"`
		TemplateContent string `json:"template_content"`
		Variables       string `json:"variables"`
		IsActive        bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreatePrompt(c.Request.Context(), &adminv1.CreatePromptRequest{
		IndustryId: req.IndustryID, Name: req.Name, Scene: req.Scene,
		TemplateContent: req.TemplateContent, Variables: req.Variables, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdatePrompt(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		IndustryID      uint64 `json:"industry_id"`
		Name            string `json:"name"`
		Scene           string `json:"scene"`
		TemplateContent string `json:"template_content"`
		Variables       string `json:"variables"`
		IsActive        *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdatePrompt(c.Request.Context(), &adminv1.UpdatePromptRequest{
		Id: id, IndustryId: req.IndustryID, Name: req.Name, Scene: req.Scene,
		TemplateContent: req.TemplateContent, Variables: req.Variables, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDeletePrompt(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeletePrompt(c.Request.Context(), &adminv1.DeletePromptRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleAdminTestRenderPrompt(c *gin.Context) {
	var req struct {
		AgentType string            `json:"agent_type"`
		Prompt    string            `json:"prompt"`
		Params    map[string]string `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.TestRenderPrompt(c.Request.Context(), &adminv1.TestRenderPromptRequest{
		AgentType: req.AgentType, Prompt: req.Prompt, Params: req.Params,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: AI 配置 ====================

func (gw *Gateway) handleAdminGetAIConfigs(c *gin.Context) {
	resp, err := gw.adminClient.GetAIConfigs(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleAdminUpdateAIConfigs 在无 bridge 直挂时转发 AI 配置更新请求，由 admin gRPC 复用单体验证规则。
func (gw *Gateway) handleAdminUpdateAIConfigs(c *gin.Context) {
	var req struct {
		Configs map[string]string `json:"configs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateAIConfigs(c.Request.Context(), &adminv1.UpdateAIConfigsRequest{Configs: req.Configs})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// ==================== Admin: AI 预设 ====================

func (gw *Gateway) handleAdminCreateAIPreset(c *gin.Context) {
	var req struct {
		Name    string            `json:"name"`
		Configs map[string]string `json:"configs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateAIPreset(c.Request.Context(), &adminv1.CreateAIPresetRequest{
		Name: req.Name, Configs: req.Configs,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateAIPreset(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name    string            `json:"name"`
		Configs map[string]string `json:"configs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.UpdateAIPreset(c.Request.Context(), &adminv1.UpdateAIPresetRequest{
		Id: id, Name: req.Name, Configs: req.Configs,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminDeleteAIPreset(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeleteAIPreset(c.Request.Context(), &adminv1.DeleteAIPresetRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleAdminApplyAIPreset(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.adminClient.ApplyAIPreset(c.Request.Context(), &adminv1.ApplyAIPresetRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: AI 日志 ====================

func (gw *Gateway) handleAdminGetAICallLog(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.adminClient.GetAICallLog(c.Request.Context(), &adminv1.GetAICallLogRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: Live2D 管理 ====================

func (gw *Gateway) handleAdminListLive2DModels(c *gin.Context) {
	resp, err := gw.adminClient.ListLive2DModels(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminCreateLive2DModel(c *gin.Context) {
	var req struct {
		Name         string `json:"name"`
		IndustryID   uint64 `json:"industry_id"`
		Scene        string `json:"scene"`
		ModelURL     string `json:"model_url"`
		ThumbnailURL string `json:"thumbnail_url"`
		ConfigJSON   string `json:"config_json"`
		TTSConfigID  uint64 `json:"tts_config_id"`
		IsActive     bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateLive2DModel(c.Request.Context(), &adminv1.CreateLive2DModelRequest{
		Name: req.Name, IndustryId: req.IndustryID, Scene: req.Scene,
		ModelUrl: req.ModelURL, ThumbnailUrl: req.ThumbnailURL,
		ConfigJson: req.ConfigJSON, TtsConfigId: req.TTSConfigID, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateLive2DModel(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name         string `json:"name"`
		IndustryID   uint64 `json:"industry_id"`
		Scene        string `json:"scene"`
		ModelURL     string `json:"model_url"`
		ThumbnailURL string `json:"thumbnail_url"`
		ConfigJSON   string `json:"config_json"`
		TTSConfigID  uint64 `json:"tts_config_id"`
		IsActive     *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateLive2DModel(c.Request.Context(), &adminv1.UpdateLive2DModelRequest{
		Id: id, Name: req.Name, IndustryId: req.IndustryID, Scene: req.Scene,
		ModelUrl: req.ModelURL, ThumbnailUrl: req.ThumbnailURL,
		ConfigJson: req.ConfigJSON, TtsConfigId: req.TTSConfigID, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDeleteLive2DModel(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeleteLive2DModel(c.Request.Context(), &adminv1.DeleteLive2DModelRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleAdminImportLive2DPackage(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()
	// 读取文件内容
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}
	resp, err := gw.adminClient.ImportLive2DPackage(c.Request.Context(), &adminv1.ImportLive2DPackageRequest{
		FileContent: fileBytes,
		Filename:    c.PostForm("filename"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminImportLive2DBackground(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}
	resp, err := gw.adminClient.ImportLive2DBackground(c.Request.Context(), &adminv1.ImportLive2DBackgroundRequest{
		FileContent: fileBytes,
		Filename:    c.PostForm("filename"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: TTS 管理 ====================

func (gw *Gateway) handleAdminListTTSConfigs(c *gin.Context) {
	resp, err := gw.adminClient.ListTTSConfigs(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminCreateTTSConfig(c *gin.Context) {
	var req struct {
		Name           string `json:"name"`
		Engine         string `json:"engine"`
		VoiceID        string `json:"voice_id"`
		AuthConfigJSON string `json:"auth_config_json"`
		ParamsJSON     string `json:"params_json"`
		IsActive       bool   `json:"is_active"`
		SortOrder      int32  `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateTTSConfig(c.Request.Context(), &adminv1.CreateTTSConfigRequest{
		Name: req.Name, Engine: req.Engine, VoiceId: req.VoiceID,
		AuthConfigJson: req.AuthConfigJSON, ParamsJson: req.ParamsJSON,
		IsActive: req.IsActive, SortOrder: req.SortOrder,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateTTSConfig(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name           string `json:"name"`
		Engine         string `json:"engine"`
		VoiceID        string `json:"voice_id"`
		AuthConfigJSON string `json:"auth_config_json"`
		ParamsJSON     string `json:"params_json"`
		IsActive       *bool  `json:"is_active"`
		SortOrder      int32  `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateTTSConfig(c.Request.Context(), &adminv1.UpdateTTSConfigRequest{
		Id: id, Name: req.Name, Engine: req.Engine, VoiceId: req.VoiceID,
		AuthConfigJson: req.AuthConfigJSON, ParamsJson: req.ParamsJSON,
		IsActive: req.IsActive, SortOrder: req.SortOrder,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDeleteTTSConfig(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeleteTTSConfig(c.Request.Context(), &adminv1.DeleteTTSConfigRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleAdminUpdateTTSSceneDefaults(c *gin.Context) {
	var req struct {
		DefaultBindings map[string]uint64 `json:"default_bindings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateTTSSceneDefaults(c.Request.Context(), &adminv1.UpdateTTSSceneDefaultsRequest{
		DefaultBindings: req.DefaultBindings,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// ==================== Admin: RAG 配置 ====================

func (gw *Gateway) handleAdminGetRAGConfigs(c *gin.Context) {
	resp, err := gw.adminClient.GetRAGConfigs(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateRAGConfigs(c *gin.Context) {
	var req struct {
		Configs map[string]string `json:"configs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateRAGConfigs(c.Request.Context(), &adminv1.UpdateRAGConfigsRequest{Configs: req.Configs})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminTestRAGConnection(c *gin.Context) {
	resp, err := gw.adminClient.TestRAGConnection(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: RAG 索引 ====================

func (gw *Gateway) handleAdminIndexAllQuestions(c *gin.Context) {
	var req struct {
		IndustryID uint64 `json:"industry_id"`
	}
	c.ShouldBindJSON(&req)
	resp, err := gw.adminClient.IndexAllQuestions(c.Request.Context(), &adminv1.IndexAllQuestionsRequest{IndustryId: req.IndustryID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminIndexQuestions(c *gin.Context) {
	var req struct {
		QuestionIDs []uint64 `json:"question_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.IndexQuestions(c.Request.Context(), &adminv1.IndexQuestionsRequest{QuestionIds: req.QuestionIDs})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminDeleteRAGIndex(c *gin.Context) {
	var req struct {
		QuestionIDs []uint64 `json:"question_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.DeleteRAGIndex(c.Request.Context(), &adminv1.DeleteRAGIndexRequest{QuestionIds: req.QuestionIDs})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminSearchRAGQuestions(c *gin.Context) {
	query := c.Query("query")
	topK, _ := strconv.ParseInt(c.DefaultQuery("top_k", "5"), 10, 32)
	resp, err := gw.adminClient.SearchRAGQuestions(c.Request.Context(), &adminv1.SearchRAGQuestionsRequest{
		Query: query, TopK: int32(topK),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: RAG 文档 ====================

func (gw *Gateway) handleAdminListRAGDocuments(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.adminClient.ListRAGDocuments(c.Request.Context(), &adminv1.ListRAGDocumentsRequest{
		Page:       &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
		Collection: c.Query("collection"),
		DocType:    c.Query("doc_type"),
		Keyword:    c.Query("keyword"),
		SyncStatus: c.Query("sync_status"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminGetRAGDocumentStats(c *gin.Context) {
	resp, err := gw.adminClient.GetRAGDocumentStats(c.Request.Context(), &adminv1.GetRAGDocumentStatsRequest{
		Collection: c.Query("collection"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminGetRAGDocument(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.adminClient.GetRAGDocument(c.Request.Context(), &adminv1.GetRAGDocumentRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminCreateRAGDocument(c *gin.Context) {
	var req struct {
		Collection string            `json:"collection"`
		DocType    string            `json:"doc_type"`
		Title      string            `json:"title"`
		Content    string            `json:"content"`
		Metadata   map[string]string `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateRAGDocument(c.Request.Context(), &adminv1.CreateRAGDocumentRequest{
		Collection: req.Collection, DocType: req.DocType,
		Title: req.Title, Content: req.Content, Metadata: req.Metadata,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateRAGDocument(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Collection string            `json:"collection"`
		DocType    string            `json:"doc_type"`
		Title      string            `json:"title"`
		Content    string            `json:"content"`
		Metadata   map[string]string `json:"metadata"`
		IsActive   *bool             `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateRAGDocument(c.Request.Context(), &adminv1.UpdateRAGDocumentRequest{
		Id: id, Collection: req.Collection, DocType: req.DocType,
		Title: req.Title, Content: req.Content, Metadata: req.Metadata, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDeleteRAGDocument(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeleteRAGDocument(c.Request.Context(), &adminv1.DeleteRAGDocumentRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleAdminBatchImportRAGDocuments(c *gin.Context) {
	var req struct {
		Collection string `json:"collection"`
		DocType    string `json:"doc_type"`
		Documents  []struct {
			Title    string            `json:"title"`
			Content  string            `json:"content"`
			Metadata map[string]string `json:"metadata"`
		} `json:"documents"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	docs := make([]*adminv1.BatchImportDocItem, len(req.Documents))
	for i, d := range req.Documents {
		docs[i] = &adminv1.BatchImportDocItem{
			Title: d.Title, Content: d.Content, Metadata: d.Metadata,
		}
	}
	resp, err := gw.adminClient.BatchImportRAGDocuments(c.Request.Context(), &adminv1.BatchImportRAGDocumentsRequest{
		Collection: req.Collection, DocType: req.DocType, Documents: docs,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminSyncRAGDocuments(c *gin.Context) {
	var req struct {
		IDs []uint64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.SyncRAGDocumentsToVectorDB(c.Request.Context(), &adminv1.SyncRAGDocumentsRequest{Ids: req.IDs})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "synced"})
}

func (gw *Gateway) handleAdminSyncAllPendingRAGDocuments(c *gin.Context) {
	_, err := gw.adminClient.SyncAllPendingRAGDocuments(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "synced"})
}

// ==================== Admin: 面经爬虫 ====================

func (gw *Gateway) handleAdminGetScraperSources(c *gin.Context) {
	resp, err := gw.adminClient.GetScraperSources(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminScraperSearch(c *gin.Context) {
	var req struct {
		Keyword  string `json:"keyword"`
		Source   string `json:"source"`
		Page     int32  `json:"page"`
		PageSize int32  `json:"page_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.ScraperSearch(c.Request.Context(), &adminv1.ScraperSearchRequest{
		Keyword: req.Keyword, Source: req.Source, Page: req.Page, PageSize: req.PageSize,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminScraperFetch(c *gin.Context) {
	var req struct {
		URL    string `json:"url"`
		Source string `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.ScraperFetch(c.Request.Context(), &adminv1.ScraperFetchRequest{
		Url: req.URL, Source: req.Source,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminScraperClean(c *gin.Context) {
	var req struct {
		Content      string `json:"content"`
		IndustryCode string `json:"industry_code"`
		Source       string `json:"source"`
		SourceURL    string `json:"source_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.ScraperClean(c.Request.Context(), &adminv1.ScraperCleanRequest{
		Content: req.Content, IndustryCode: req.IndustryCode,
		Source: req.Source, SourceUrl: req.SourceURL,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminScraperImport(c *gin.Context) {
	var req struct {
		IndustryCode string `json:"industry_code"`
		Questions    []struct {
			CategoryName string `json:"category_name"`
			Type         string `json:"type"`
			Difficulty   string `json:"difficulty"`
			Title        string `json:"title"`
			Content      string `json:"content"`
			Answer       string `json:"answer"`
			Explanation  string `json:"explanation"`
			Tags         string `json:"tags"`
		} `json:"questions"`
		SourceURL   string `json:"source_url"`
		SourceTitle string `json:"source_title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items := make([]*adminv1.ScraperCleanedQuestion, len(req.Questions))
	for i, q := range req.Questions {
		items[i] = &adminv1.ScraperCleanedQuestion{
			CategoryName: q.CategoryName, Type: q.Type, Difficulty: q.Difficulty,
			Title: q.Title, Content: q.Content, Answer: q.Answer,
			Explanation: q.Explanation, Tags: q.Tags,
		}
	}
	resp, err := gw.adminClient.ScraperImport(c.Request.Context(), &adminv1.ScraperImportRequest{
		IndustryCode: req.IndustryCode, Questions: items,
		SourceUrl: req.SourceURL, SourceTitle: req.SourceTitle,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminScraperImportAsync(c *gin.Context) {
	var req struct {
		IndustryCode string `json:"industry_code"`
		Questions    []struct {
			CategoryName string `json:"category_name"`
			Type         string `json:"type"`
			Difficulty   string `json:"difficulty"`
			Title        string `json:"title"`
			Content      string `json:"content"`
			Answer       string `json:"answer"`
			Explanation  string `json:"explanation"`
			Tags         string `json:"tags"`
		} `json:"questions"`
		SourceURL   string `json:"source_url"`
		SourceTitle string `json:"source_title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items := make([]*adminv1.ScraperCleanedQuestion, len(req.Questions))
	for i, q := range req.Questions {
		items[i] = &adminv1.ScraperCleanedQuestion{
			CategoryName: q.CategoryName, Type: q.Type, Difficulty: q.Difficulty,
			Title: q.Title, Content: q.Content, Answer: q.Answer,
			Explanation: q.Explanation, Tags: q.Tags,
		}
	}
	resp, err := gw.adminClient.ScraperImportAsync(c.Request.Context(), &adminv1.ScraperImportRequest{
		IndustryCode: req.IndustryCode, Questions: items,
		SourceUrl: req.SourceURL, SourceTitle: req.SourceTitle,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminListScraperTasks(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.adminClient.ListScraperTasks(c.Request.Context(), &adminv1.ListScraperTasksRequest{
		Page:     &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
		Status:   c.Query("status"),
		TaskType: c.Query("task_type"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminGetScraperTask(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.adminClient.GetScraperTask(c.Request.Context(), &adminv1.GetScraperTaskRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminRetryScraperTask(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.adminClient.RetryScraperTask(c.Request.Context(), &adminv1.RetryScraperTaskRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
