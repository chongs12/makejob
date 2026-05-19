package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"makejob-backend/internal/config"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
	scraperimpl "makejob-backend/internal/scraper"
	scraperMock "makejob-backend/internal/scraper/mock"
	"makejob-backend/internal/service"
	applogger "makejob-backend/pkg/logger"
)

const (
	workerVersion         = "1.0.0"
	defaultPollInterval   = 3 * time.Second
	defaultIdleSleepDelay = 1200 * time.Millisecond
)

// main 启动单体内任务执行器进程，当前负责消费异步导入与题目流水线生成任务。
func main() {
	cfg := config.GetConfig()

	if err := initWorkerLogger(cfg); err != nil {
		fmt.Printf("init worker logger failed: %v\n", err)
		os.Exit(1)
	}
	defer applogger.Sync()

	applogger.Info("starting makejob worker", zap.String("version", workerVersion))

	db, err := model.InitDB(&cfg.Database)
	if err != nil {
		applogger.Fatal("worker database init failed", zap.Error(err))
	}
	defer model.CloseDB()

	if err := model.AutoMigrate(db); err != nil {
		applogger.Fatal("worker auto migrate failed", zap.Error(err))
	}

	scraperService := buildWorkerScraperService(db)
	adminService := buildWorkerAdminService(db, cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := runWorkerLoop(ctx, scraperService, adminService); err != nil && err != context.Canceled {
		applogger.Fatal("worker loop exited with error", zap.Error(err))
	}

	applogger.Info("worker exited")
}

// initWorkerLogger 初始化 worker 使用的日志配置，保持与主服务一致的日志等级策略。
func initWorkerLogger(cfg *config.Config) error {
	logConfig := applogger.Config{
		Level: "info",
		Mode:  cfg.Server.Mode,
	}
	if cfg.Server.Mode == "debug" {
		logConfig.Level = "debug"
	}
	return applogger.Init(logConfig)
}

// buildWorkerScraperService 构造 worker 运行所需的异步导入任务服务依赖集合。
func buildWorkerScraperService(db *gorm.DB) service.ScraperService {
	scraperRepo := repository.NewScraperTaskRepository(db)
	industryRepo := repository.NewIndustryRepository(db)
	categoryRepo := repository.NewAdminCategoryRepository(db)
	questionRepo := repository.NewAdminQuestionRepository(db)
	return service.NewScraperService(nil, nil, scraperRepo, industryRepo, categoryRepo, questionRepo)
}

// buildWorkerAdminService 构造 worker 执行题目流水线生成任务所需的最小后台服务依赖集合。
func buildWorkerAdminService(db *gorm.DB, cfg *config.Config) service.AdminService {
	industryRepo := repository.NewIndustryRepository(db)
	adminCategoryRepo := repository.NewAdminCategoryRepository(db)
	adminConfigRepo := repository.NewAdminConfigRepository(db)
	aiPresetRepo := repository.NewAIPresetRepository(db)
	promptRepo := repository.NewPromptTemplateRepository(db)
	aiCallLogRepo := repository.NewAICallLogRepository(db)
	scraperTaskRepo := repository.NewScraperTaskRepository(db)
	scraperProvider := scraperMock.NewMockScraperProvider()
	questionCleaner := scraperimpl.NewMockCleaner()

	return service.NewAdminService(
		nil,
		nil,
		industryRepo,
		adminCategoryRepo,
		promptRepo,
		adminConfigRepo,
		aiPresetRepo,
		aiCallLogRepo,
		nil,
		nil,
		nil,
		scraperTaskRepo,
		scraperProvider,
		questionCleaner,
		cfg.AIRuntimeDefaults(),
	)
}

// runWorkerLoop 持续轮询并消费待执行任务，空闲时短暂休眠，有任务时尽快继续拉取下一条。
func runWorkerLoop(ctx context.Context, scraperService service.ScraperService, adminService service.AdminService) error {
	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if task, handled, err := scraperService.RunNextPendingTask(ctx); handled || err != nil {
			if err != nil {
				logWorkerTaskFailure(task, err)
			}
			if handled {
				logWorkerTaskHandled(task, err == nil)
				continue
			}
		}

		if task, handled, err := adminService.RunNextPendingQuestionPipelineTask(ctx); handled || err != nil {
			if err != nil {
				logWorkerTaskFailure(task, err)
			}
			if handled {
				logWorkerTaskHandled(task, err == nil)
				continue
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(defaultIdleSleepDelay):
		case <-ticker.C:
		}
	}
}

// logWorkerTaskHandled 记录任务被 worker 执行后的关键信息，便于后续按任务类型与状态排查。
func logWorkerTaskHandled(task *model.ScraperTask, succeeded bool) {
	if task == nil {
		return
	}

	applogger.Info("worker handled async task",
		zap.Uint("task_id", task.ID),
		zap.String("task_type", task.TaskType),
		zap.String("status", task.Status),
		zap.Bool("succeeded", succeeded),
	)
}

// logWorkerTaskFailure 记录任务执行失败日志，保证任务表与进程日志都保留同一失败上下文。
func logWorkerTaskFailure(task *model.ScraperTask, err error) {
	if task == nil {
		applogger.Warn("worker execute task failed", zap.Error(err))
		return
	}

	applogger.Warn("worker execute async task failed",
		zap.Uint("task_id", task.ID),
		zap.String("task_type", task.TaskType),
		zap.Error(err),
	)
}
