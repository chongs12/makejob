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

	aiRuntime "makejob-backend/internal/ai/runtime"
	asrfactory "makejob-backend/internal/asr/factory"
	"makejob-backend/internal/common"
	"makejob-backend/internal/config"
	"makejob-backend/internal/handler"
	"makejob-backend/internal/live2dassets"
	"makejob-backend/internal/middleware"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
	"makejob-backend/internal/scraper"
	scraperMock "makejob-backend/internal/scraper/mock"
	"makejob-backend/internal/service"
	ttsfactory "makejob-backend/internal/tts/factory"
	applogger "makejob-backend/pkg/logger"
)

const Version = "1.0.0"

type AppDependencies struct {
	UserRepo              repository.UserRepository
	MembershipRepo        repository.MembershipRepository
	InterviewRepo         repository.InterviewRepository
	InterviewMessageRepo  repository.InterviewMessageRepository
	InterviewCodingRepo   repository.InterviewCodingAttemptRepository
	QuestionRepo          repository.QuestionRepository
	CategoryRepo          repository.CategoryRepository
	RecordRepo            repository.QuestionRecordRepository
	FavoriteRepo          repository.FavoriteRepository
	NoteRepo              repository.NoteRepository
	PlanRepo              repository.PlanRepository
	PlanTaskRepo          repository.PlanTaskRepository
	PlanTaskFeedbackRepo  repository.PlanTaskFeedbackRepository
	PlanTaskDiagnosisRepo repository.PlanTaskDiagnosisRepository
	StudyLogRepo          repository.StudyLogRepository
	LearningArchiveRepo   repository.LearningArchiveRepository

	AuthService       service.AuthService
	MembershipService service.MembershipService
	InterviewService  service.InterviewService
	QuestionService   service.QuestionService
	PlanService       service.PlanService
	CompanionService  service.CompanionService
	GrowthService     service.GrowthService
	Live2DService     service.Live2DService
	CasbinService     service.CasbinService

	AuthHandler       *handler.AuthHandler
	MembershipHandler *handler.MembershipHandler
	InterviewHandler  *handler.InterviewHandler
	QuestionHandler   *handler.QuestionHandler
	PlanHandler       *handler.PlanHandler
	CompanionHandler  *handler.CompanionHandler
	GrowthHandler     *handler.GrowthHandler
	Live2DHandler     *handler.Live2DHandler
	AdminHandler      *handler.AdminHandler
	ScraperHandler    *handler.ScraperHandler
	CommunityHandler  *handler.CommunityHandler
}

func main() {
	cfg := config.GetConfig()

	if err := initLogger(cfg); err != nil {
		fmt.Printf("init logger failed: %v\n", err)
		os.Exit(1)
	}
	defer applogger.Sync()

	applogger.Info("starting makejob backend", zap.String("version", Version))

	db, dbErr := model.InitDB(&cfg.Database)
	if dbErr != nil {
		applogger.Warn("database init failed, continuing without db", zap.Error(dbErr))
	} else {
		applogger.Info("database connected")

		if err := model.AutoMigrate(db); err != nil {
			applogger.Warn("auto migrate failed", zap.Error(err))
		} else {
			applogger.Info("auto migrate finished")
			if err := model.SeedData(db); err != nil {
				applogger.Warn("seed data failed", zap.Error(err))
			}
			if err := model.EnsureAdminUser(db, &cfg.AdminBootstrap); err != nil {
				applogger.Warn("admin bootstrap failed", zap.Error(err))
			}
		}

		defer model.CloseDB()
	}

	rdb, redisErr := initRedis(cfg)
	if redisErr != nil {
		applogger.Warn("redis init failed", zap.Error(redisErr))
	} else {
		applogger.Info("redis connected")
		defer rdb.Close()
	}

	deps := initDependencies(db, cfg)

	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())
	r.Use(middleware.RateLimit())
	r.Use(gin.Recovery())

	registerRoutes(r, deps)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: r,
	}

	go func() {
		applogger.Info("http server listening", zap.Int("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			applogger.Fatal("http server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	applogger.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		applogger.Error("server shutdown failed", zap.Error(err))
		return
	}

	applogger.Info("server exited")
}

// initDependencies 初始化仓库、服务和处理器依赖。
func initDependencies(db *gorm.DB, cfg *config.Config) *AppDependencies {
	deps := &AppDependencies{}
	var industryRepo repository.IndustryRepository
	var live2DRepo repository.Live2DModelRepository

	if db != nil {
		deps.UserRepo = repository.NewUserRepository(db)
		deps.MembershipRepo = repository.NewMembershipRepository(db)
		deps.InterviewRepo = repository.NewInterviewRepository(db)
		deps.InterviewMessageRepo = repository.NewInterviewMessageRepository(db)
		deps.InterviewCodingRepo = repository.NewInterviewCodingAttemptRepository(db)
		deps.QuestionRepo = repository.NewQuestionRepository(db)
		deps.CategoryRepo = repository.NewCategoryRepository(db)
		deps.RecordRepo = repository.NewQuestionRecordRepository(db)
		deps.FavoriteRepo = repository.NewFavoriteRepository(db)
		deps.NoteRepo = repository.NewNoteRepository(db)
		deps.PlanRepo = repository.NewPlanRepository(db)
		deps.PlanTaskRepo = repository.NewPlanTaskRepository(db)
		deps.PlanTaskFeedbackRepo = repository.NewPlanTaskFeedbackRepository(db)
		deps.PlanTaskDiagnosisRepo = repository.NewPlanTaskDiagnosisRepository(db)
		deps.StudyLogRepo = repository.NewStudyLogRepository(db)
		deps.LearningArchiveRepo = repository.NewLearningArchiveRepository(db)

		industryRepo = repository.NewIndustryRepository(db)
		adminConfigRepo := repository.NewAdminConfigRepository(db)
		adminUserRepo := repository.NewAdminUserRepository(db)
		adminQuestionRepo := repository.NewAdminQuestionRepository(db)
		adminCategoryRepo := repository.NewAdminCategoryRepository(db)
		promptRepo := repository.NewPromptTemplateRepository(db)
		aiCallLogRepo := repository.NewAICallLogRepository(db)
		live2DRepo = repository.NewLive2DModelRepository(db)
		ttsRepo := repository.NewTTSConfigRepository(db)
		mockInterviewRepo := repository.NewMockInterviewRepository(db)
		scraperTaskRepo := repository.NewScraperTaskRepository(db)
		aiClient := aiRuntime.NewBuilder(adminConfigRepo, promptRepo, industryRepo, aiCallLogRepo, cfg.AIRuntimeDefaults()).Build(context.Background())
		scraperProvider := scraperMock.NewMockScraperProvider()
		questionCleaner := scraper.NewMockCleaner()
		communityRepo := repository.NewCommunityRepository(db)

		deps.AuthService = service.NewAuthService(deps.UserRepo, cfg)
		deps.MembershipService = service.NewMembershipService(deps.MembershipRepo, deps.UserRepo)
		deps.InterviewService = service.NewInterviewService(
			deps.InterviewRepo,
			deps.InterviewMessageRepo,
			deps.InterviewCodingRepo,
			deps.LearningArchiveRepo,
			aiClient.InterviewAgent,
			aiClient.QuizAnalyzer,
			industryRepo,
		)
		deps.QuestionService = service.NewQuestionService(
			deps.QuestionRepo,
			deps.CategoryRepo,
			deps.RecordRepo,
			deps.FavoriteRepo,
			deps.NoteRepo,
			aiClient.QuizAnalyzer,
			deps.LearningArchiveRepo,
			industryRepo,
		)
		deps.PlanService = service.NewPlanService(
			deps.PlanRepo,
			deps.PlanTaskRepo,
			aiClient.PlanAgent,
			deps.LearningArchiveRepo,
			deps.InterviewRepo,
			deps.PlanTaskFeedbackRepo,
			deps.PlanTaskDiagnosisRepo,
			aiClient.QuizAnalyzer,
			industryRepo,
		)
		deps.CompanionService = service.NewCompanionService(aiClient.CompanionAgent)
		deps.GrowthService = service.NewGrowthService(
			deps.StudyLogRepo,
			deps.RecordRepo,
			deps.InterviewRepo,
			deps.PlanRepo,
			deps.PlanTaskRepo,
			deps.LearningArchiveRepo,
		)
		deps.Live2DService = service.NewLive2DService(live2DRepo, industryRepo)
		communityService := service.NewCommunityService(communityRepo, deps.UserRepo)
		ttsProvider, err := ttsfactory.NewTTSProviderWithConfig("", cfg)
		if err != nil {
			applogger.Warn("interview tts provider not ready", zap.Error(err))
		}
		asrProvider, err := asrfactory.NewASRProviderWithConfig("", cfg)
		if err != nil {
			applogger.Warn("interview asr provider not ready", zap.Error(err))
		}

		deps.AuthHandler = handler.NewAuthHandler(deps.AuthService)
		deps.MembershipHandler = handler.NewMembershipHandler(deps.MembershipService)
		deps.InterviewHandler = handler.NewInterviewHandler(deps.InterviewService, ttsProvider, asrProvider)
		deps.QuestionHandler = handler.NewQuestionHandler(deps.QuestionService)
		deps.PlanHandler = handler.NewPlanHandler(deps.PlanService)
		deps.CompanionHandler = handler.NewCompanionHandler(deps.CompanionService)
		deps.GrowthHandler = handler.NewGrowthHandler(deps.GrowthService)
		deps.Live2DHandler = handler.NewLive2DHandler(deps.Live2DService)
		deps.CommunityHandler = handler.NewCommunityHandler(communityService)

		adminService := service.NewAdminService(
			adminUserRepo,
			adminQuestionRepo,
			industryRepo,
			adminCategoryRepo,
			promptRepo,
			adminConfigRepo,
			aiCallLogRepo,
			live2DRepo,
			ttsRepo,
			mockInterviewRepo,
			scraperTaskRepo,
			scraperProvider,
			questionCleaner,
			cfg.AIRuntimeDefaults(),
		)
		deps.AdminHandler = handler.NewAdminHandler(adminService)

		scraperService := service.NewScraperService(
			scraperProvider,
			questionCleaner,
			scraperTaskRepo,
			industryRepo,
			adminCategoryRepo,
			adminQuestionRepo,
		)
		deps.ScraperHandler = handler.NewScraperHandler(scraperService)
	}

	if deps.Live2DService == nil {
		deps.Live2DService = service.NewLive2DService(live2DRepo, industryRepo)
	}
	if deps.Live2DHandler == nil && deps.Live2DService != nil {
		deps.Live2DHandler = handler.NewLive2DHandler(deps.Live2DService)
	}

	casbinService, err := service.NewCasbinService(cfg)
	if err != nil {
		applogger.Warn("casbin init failed", zap.Error(err))
	} else {
		deps.CasbinService = casbinService
	}

	return deps
}

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

func initRedis(cfg *config.Config) (*redis.Client, error) {
	if cfg.Redis.Host == "" {
		return nil, fmt.Errorf("redis host is empty")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis failed: %w", err)
	}

	return rdb, nil
}

func registerRoutes(r *gin.Engine, deps *AppDependencies) {
	if assetsDir, err := live2dassets.EnsureAssetsDir(); err == nil && assetsDir != "" {
		r.StaticFS(live2dassets.MountPath, gin.Dir(assetsDir, false))
	} else {
		applogger.Warn("live2d assets dir not ready", zap.Error(err))
	}

	r.GET("/api/health", func(c *gin.Context) {
		common.Success(c, gin.H{
			"status":    "ok",
			"version":   Version,
			"timestamp": time.Now().Unix(),
		})
	})

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/api/health")
	})

	api := r.Group("/api")
	{
		if deps.AuthHandler != nil {
			auth := api.Group("/auth")
			deps.AuthHandler.RegisterRoutes(auth, nil)
		}

		public := api.Group("")
		public.Use(middleware.OptionalAuth())
		if deps.QuestionHandler != nil {
			deps.QuestionHandler.RegisterRoutes(public, nil)
		}
		if deps.CommunityHandler != nil {
			deps.CommunityHandler.RegisterRoutes(public, nil)
		}
		if deps.Live2DHandler != nil {
			deps.Live2DHandler.RegisterRoutes(public)
		}

		protected := api.Group("")
		protected.Use(middleware.Auth())
		if deps.AuthHandler != nil {
			deps.AuthHandler.RegisterProtectedRoutes(protected)
		}
		if deps.MembershipHandler != nil {
			deps.MembershipHandler.RegisterRoutes(protected)
		}
		if deps.InterviewHandler != nil {
			deps.InterviewHandler.RegisterRoutes(protected)
		}
		if deps.PlanHandler != nil {
			deps.PlanHandler.RegisterRoutes(protected)
		}
		if deps.CompanionHandler != nil {
			deps.CompanionHandler.RegisterRoutes(protected)
		}
		if deps.GrowthHandler != nil {
			deps.GrowthHandler.RegisterRoutes(protected)
		}
		if deps.QuestionHandler != nil {
			deps.QuestionHandler.RegisterRoutes(nil, protected)
		}
		if deps.CommunityHandler != nil {
			deps.CommunityHandler.RegisterRoutes(nil, protected)
		}

		admin := api.Group("/admin")
		admin.Use(middleware.Auth())
		if deps.CasbinService != nil {
			admin.Use(middleware.Casbin())
		}
		if deps.AdminHandler != nil {
			deps.AdminHandler.RegisterRoutes(admin)
		}
		if deps.ScraperHandler != nil {
			deps.ScraperHandler.RegisterRoutes(admin)
		}
	}
}
