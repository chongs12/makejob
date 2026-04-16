// Package main 是MakeJob后端服务的入口程序
// 负责初始化所有组件并启动HTTP服务器
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"makejob-backend/internal/ai/mock"
	"makejob-backend/internal/common"
	"makejob-backend/internal/config"
	"makejob-backend/internal/handler"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
	"makejob-backend/internal/scraper"
	scraperMock "makejob-backend/internal/scraper/mock"
	"makejob-backend/internal/service"
	applogger "makejob-backend/pkg/logger"
)

const (
	// Version 应用版本号
	Version = "1.0.0"
)

// AppDependencies 应用依赖容器
type AppDependencies struct {
	UserRepo             repository.UserRepository
	MembershipRepo       repository.MembershipRepository
	InterviewRepo        repository.InterviewRepository
	InterviewMessageRepo repository.InterviewMessageRepository
	QuestionRepo         repository.QuestionRepository
	CategoryRepo         repository.CategoryRepository
	RecordRepo           repository.QuestionRecordRepository
	FavoriteRepo         repository.FavoriteRepository
	NoteRepo             repository.NoteRepository
	PlanRepo             repository.PlanRepository
	PlanTaskRepo         repository.PlanTaskRepository
	AuthService          service.AuthService
	MembershipService    service.MembershipService
	InterviewService     service.InterviewService
	QuestionService      service.QuestionService
	PlanService          service.PlanService
	CasbinService        service.CasbinService
	AuthHandler          *handler.AuthHandler
	MembershipHandler    *handler.MembershipHandler
	InterviewHandler     *handler.InterviewHandler
	QuestionHandler      *handler.QuestionHandler
	PlanHandler          *handler.PlanHandler
	AdminHandler         *handler.AdminHandler
	ScraperHandler       *handler.ScraperHandler
	CommunityHandler     *handler.CommunityHandler
}

func main() {
	// 加载配置
	cfg := config.GetConfig()

	// 初始化日志
	if err := initLogger(cfg); err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer applogger.Sync()

	applogger.Info("MakeJob后端服务启动中...",
		zap.String("version", Version))

	// 初始化数据库连接（优雅降级）
	db, dbErr := model.InitDB(&cfg.Database)
	if dbErr != nil {
		applogger.Warn("数据库连接失败，服务将以降级模式运行",
			zap.Any("error", dbErr))
	} else {
		applogger.Info("数据库连接成功")

		// 自动迁移数据库表结构
		if err := model.AutoMigrate(db); err != nil {
			applogger.Warn("数据库迁移失败",
				zap.Any("error", err))
		} else {
			applogger.Info("数据库表迁移完成")

			// 插入种子数据（首次启动）
			if err := model.SeedData(db); err != nil {
				applogger.Warn("种子数据插入失败",
					zap.Any("error", err))
			}
			if err := model.EnsureAdminUser(db, &cfg.AdminBootstrap); err != nil {
				applogger.Warn("admin bootstrap failed",
					zap.Any("error", err))
			}
		}

		// 在应用退出时关闭数据库连接
		defer model.CloseDB()
	}

	// 初始化Redis连接（优雅降级）
	rdb, redisErr := initRedis(cfg)
	if redisErr != nil {
		applogger.Warn("Redis连接失败，服务将以降级模式运行",
			zap.Any("error", redisErr))
	} else {
		applogger.Info("Redis连接成功")
		defer rdb.Close()
	}

	// 初始化依赖注入
	deps := initDependencies(db, cfg)

	// 设置Gin运行模式
	gin.SetMode(cfg.Server.Mode)

	// 创建Gin引擎
	r := gin.New()

	// 注册全局中间件
	r.Use(middleware.Logger())    // 请求日志
	r.Use(middleware.CORS())      // 跨域处理
	r.Use(middleware.RateLimit()) // 限流保护
	r.Use(gin.Recovery())         // panic恢复

	// 注册路由
	registerRoutes(r, deps)

	// 创建HTTP服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: r,
	}

	// 在后台启动服务器
	go func() {
		applogger.Info("HTTP服务器启动",
			zap.Int("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			applogger.Fatal("服务器启动失败",
				zap.Any("error", err))
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	applogger.Info("正在关闭服务器...")

	// 设置关闭超时时间
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		applogger.Error("服务器关闭失败",
			zap.Any("error", err))
	}

	applogger.Info("服务器已安全关闭")
}

// initDependencies 初始化依赖注入
func initDependencies(db *gorm.DB, cfg *config.Config) *AppDependencies {
	deps := &AppDependencies{}

	if db != nil {
		// Repository层
		deps.UserRepo = repository.NewUserRepository(db)
		deps.MembershipRepo = repository.NewMembershipRepository(db)
		deps.InterviewRepo = repository.NewInterviewRepository(db)
		deps.InterviewMessageRepo = repository.NewInterviewMessageRepository(db)
		deps.QuestionRepo = repository.NewQuestionRepository(db)
		deps.CategoryRepo = repository.NewCategoryRepository(db)
		deps.RecordRepo = repository.NewQuestionRecordRepository(db)
		deps.FavoriteRepo = repository.NewFavoriteRepository(db)
		deps.NoteRepo = repository.NewNoteRepository(db)
		deps.PlanRepo = repository.NewPlanRepository(db)
		deps.PlanTaskRepo = repository.NewPlanTaskRepository(db)
		communityRepo := repository.NewCommunityRepository(db)

		// AI Provider初始化（使用Mock）
		aiProvider := mock.NewAIProvider("mock", nil)
		interviewAgent := mock.NewInterviewAgent(aiProvider)
		quizAnalyzer := mock.NewQuizAnalyzer(aiProvider)
		planAgent := mock.NewPlanAgent(aiProvider)

		// 行业仓库（共用）
		industryRepo := repository.NewIndustryRepository(db)

		// Service层
		deps.AuthService = service.NewAuthService(deps.UserRepo, cfg)
		deps.MembershipService = service.NewMembershipService(deps.MembershipRepo, deps.UserRepo)
		deps.InterviewService = service.NewInterviewService(
			deps.InterviewRepo,
			deps.InterviewMessageRepo,
			interviewAgent,
			industryRepo,
		)
		deps.QuestionService = service.NewQuestionService(
			deps.QuestionRepo,
			deps.CategoryRepo,
			deps.RecordRepo,
			deps.FavoriteRepo,
			deps.NoteRepo,
			quizAnalyzer,
		)
		deps.PlanService = service.NewPlanService(
			deps.PlanRepo,
			deps.PlanTaskRepo,
			planAgent,
		)
		communityService := service.NewCommunityService(communityRepo, deps.UserRepo)

		// Handler层
		deps.AuthHandler = handler.NewAuthHandler(deps.AuthService)
		deps.MembershipHandler = handler.NewMembershipHandler(deps.MembershipService)
		deps.InterviewHandler = handler.NewInterviewHandler(deps.InterviewService)
		deps.QuestionHandler = handler.NewQuestionHandler(deps.QuestionService)
		deps.PlanHandler = handler.NewPlanHandler(deps.PlanService)
		deps.CommunityHandler = handler.NewCommunityHandler(communityService)

		// Admin相关依赖初始化
		adminUserRepo := repository.NewAdminUserRepository(db)
		adminQuestionRepo := repository.NewAdminQuestionRepository(db)
		adminCategoryRepo := repository.NewAdminCategoryRepository(db)
		promptRepo := repository.NewPromptTemplateRepository(db)
		adminConfigRepo := repository.NewAdminConfigRepository(db)
		live2DRepo := repository.NewLive2DModelRepository(db)
		ttsRepo := repository.NewTTSConfigRepository(db)
		mockInterviewRepo := repository.NewMockInterviewRepository(db)
		scraperTaskRepo := repository.NewScraperTaskRepository(db)

		// Scraper初始化
		scraperProvider := scraperMock.NewMockScraperProvider()
		scraperCleaner := scraper.NewMockCleaner()

		adminService := service.NewAdminService(
			adminUserRepo,
			adminQuestionRepo,
			industryRepo,
			adminCategoryRepo,
			promptRepo,
			adminConfigRepo,
			live2DRepo,
			ttsRepo,
			mockInterviewRepo,
		)
		deps.AdminHandler = handler.NewAdminHandler(adminService)

		// Scraper服务初始化
		scraperService := service.NewScraperService(
			scraperProvider,
			scraperCleaner,
			scraperTaskRepo,
			industryRepo,
			adminCategoryRepo,
			adminQuestionRepo,
		)
		deps.ScraperHandler = handler.NewScraperHandler(scraperService)

		applogger.Info("依赖注入初始化完成")
	}

	// 初始化Casbin权限服务
	casbinService, err := service.NewCasbinService(cfg)
	if err != nil {
		applogger.Warn("Casbin权限服务初始化失败，权限检查将被禁用",
			zap.Any("error", err))
	} else {
		deps.CasbinService = casbinService
		applogger.Info("Casbin权限服务初始化成功")
	}

	return deps
}

// initLogger 初始化日志系统
func initLogger(cfg *config.Config) error {
	logConfig := applogger.Config{
		Level: "info",
		Mode:  cfg.Server.Mode,
	}

	if cfg.Server.Mode == "debug" {
		logConfig.Level = "debug"
	}

	return applogger.Init(logConfig)
}

// initRedis 初始化Redis连接
// 如果连接失败，返回错误但不中断程序
func initRedis(cfg *config.Config) (*redis.Client, error) {
	if cfg.Redis.Host == "" {
		return nil, fmt.Errorf("Redis配置不完整")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	return rdb, nil
}

// registerRoutes 注册所有路由
func registerRoutes(r *gin.Engine, deps *AppDependencies) {
	// 健康检查端点
	r.GET("/api/health", func(c *gin.Context) {
		common.Success(c, gin.H{
			"status":    "ok",
			"version":   Version,
			"timestamp": time.Now().Unix(),
		})
	})

	// 根路径重定向到健康检查
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/api/health")
	})

	// API路由组
	api := r.Group("/api")
	{
		// 公开路由（无需认证）
		if deps.AuthHandler != nil {
			auth := api.Group("/auth")
			deps.AuthHandler.RegisterRoutes(auth, nil)
		}

		// 题库公开路由（可匿名访问）
		if deps.QuestionHandler != nil {
			public := api.Group("")
			public.Use(middleware.OptionalAuth())
			deps.QuestionHandler.RegisterRoutes(public, nil)
			if deps.CommunityHandler != nil {
				deps.CommunityHandler.RegisterRoutes(public, nil)
			}
		}

		// 需要认证的路由
		if deps.AuthHandler != nil {
			protected := api.Group("")
			protected.Use(middleware.Auth())
			deps.AuthHandler.RegisterProtectedRoutes(protected)

			// 会员相关路由（需要认证）
			if deps.MembershipHandler != nil {
				deps.MembershipHandler.RegisterRoutes(protected)
			}

			// TODO: Task后续集成 - 刷题路由（需要认证+会员检查）
			// practice := protected.Group("/practice")
			// practice.Use(middleware.MembershipCheck(deps.MembershipService))
			// { ... }

			// 面试路由（需要认证）
			if deps.InterviewHandler != nil {
				deps.InterviewHandler.RegisterRoutes(protected)
			}

			// 学习计划路由（需要认证）
			if deps.PlanHandler != nil {
				deps.PlanHandler.RegisterRoutes(protected)
			}

			// 题库认证路由（需要认证）
			if deps.QuestionHandler != nil {
				deps.QuestionHandler.RegisterRoutes(nil, protected)
			}
			if deps.CommunityHandler != nil {
				deps.CommunityHandler.RegisterRoutes(nil, protected)
			}
		}

		// 管理员路由（需要认证+权限检查）
		admin := api.Group("/admin")
		admin.Use(middleware.Auth())
		if deps.CasbinService != nil {
			admin.Use(middleware.Casbin())
		}
		{
			// 注册管理员路由
			if deps.AdminHandler != nil {
				deps.AdminHandler.RegisterRoutes(admin)
			}

			// 注册Scraper路由
			if deps.ScraperHandler != nil {
				deps.ScraperHandler.RegisterRoutes(admin)
			}
		}
	}
}
