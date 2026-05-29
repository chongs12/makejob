package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	aiRuntime "makejob-backend/internal/ai/runtime"
	"makejob-backend/internal/config"
	"makejob-backend/internal/model"
	"makejob-backend/internal/mq"
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

// scraperImportTaskProcessor 定义爬虫导入消息消费需要的最小处理能力。
type scraperImportTaskProcessor interface {
	ProcessImportTask(ctx context.Context, taskID uint) error
}

// adminQuestionPipelineTaskProcessor 定义题目流水线消息消费需要的最小处理能力。
type adminQuestionPipelineTaskProcessor interface {
	ProcessQuestionPipelineTask(ctx context.Context, taskID uint) error
}

// planFeedbackDiagnosisTaskProcessor 定义学习任务反馈诊断消息消费需要的最小处理能力。
type planFeedbackDiagnosisTaskProcessor interface {
	ProcessTaskFeedbackDiagnosisTask(ctx context.Context, asyncTaskID uint) error
}

// planGenerateTaskProcessor 定义学习计划生成消息消费需要的最小处理能力。
type planGenerateTaskProcessor interface {
	ProcessPlanGenerateTask(ctx context.Context, asyncTaskID uint) error
}

// interviewResumeParseTaskProcessor 定义简历解析消息消费需要的最小处理能力。
type interviewResumeParseTaskProcessor interface {
	ProcessInterviewResumeParseTask(ctx context.Context, asyncTaskID uint) error
}

// interviewReportTaskProcessor 定义面试报告消息消费需要的最小处理能力。
type interviewReportTaskProcessor interface {
	ProcessInterviewReportTask(ctx context.Context, asyncTaskID uint) error
}

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
	planService := buildWorkerPlanService(db, cfg)
	interviewService := buildWorkerInterviewService(db, cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.RabbitMQ.Enabled {
		if err := runWorkerMQ(ctx, cfg, scraperService, adminService, planService, interviewService); err == nil || err == context.Canceled {
			applogger.Info("worker exited")
			return
		} else {
			applogger.Warn("RabbitMQ worker mode unavailable, fallback to polling mode", zap.Error(err))
		}
	}

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

// buildWorkerPlanService 构造 worker 处理学习计划生成与反馈诊断所需的最小学习计划服务依赖集合。
func buildWorkerPlanService(db *gorm.DB, cfg *config.Config) service.PlanService {
	planRepo := repository.NewPlanRepository(db)
	taskRepo := repository.NewPlanTaskRepository(db)
	feedbackRepo := repository.NewPlanTaskFeedbackRepository(db)
	diagnosisRepo := repository.NewPlanTaskDiagnosisRepository(db)
	archiveRepo := repository.NewLearningArchiveRepository(db)
	industryRepo := repository.NewIndustryRepository(db)
	adminConfigRepo := repository.NewAdminConfigRepository(db)
	promptRepo := repository.NewPromptTemplateRepository(db)
	aiCallLogRepo := repository.NewAICallLogRepository(db)
	asyncTaskRepo := repository.NewAsyncTaskRepository(db)
	runtimeBuilder := aiRuntime.NewBuilder(adminConfigRepo, promptRepo, industryRepo, aiCallLogRepo, cfg.AIRuntimeDefaults())
	aiClient := aiRuntime.NewRuntimeManager(runtimeBuilder).BuildDynamicClient()
	return service.NewPlanService(
		planRepo,
		taskRepo,
		aiClient.PlanAgent,
		archiveRepo,
		nil,
		feedbackRepo,
		diagnosisRepo,
		aiClient.QuizAnalyzer,
		industryRepo,
		service.AsyncDispatchOption{AsyncTaskRepo: asyncTaskRepo},
	)
}

// buildWorkerInterviewService 构造 worker 处理面试简历解析和报告生成所需的服务依赖。
func buildWorkerInterviewService(db *gorm.DB, cfg *config.Config) service.InterviewService {
	interviewRepo := repository.NewInterviewRepository(db)
	interviewMessageRepo := repository.NewInterviewMessageRepository(db)
	codingRepo := repository.NewInterviewCodingAttemptRepository(db)
	archiveRepo := repository.NewLearningArchiveRepository(db)
	industryRepo := repository.NewIndustryRepository(db)
	adminConfigRepo := repository.NewAdminConfigRepository(db)
	promptRepo := repository.NewPromptTemplateRepository(db)
	aiCallLogRepo := repository.NewAICallLogRepository(db)
	asyncTaskRepo := repository.NewAsyncTaskRepository(db)
	runtimeBuilder := aiRuntime.NewBuilder(adminConfigRepo, promptRepo, industryRepo, aiCallLogRepo, cfg.AIRuntimeDefaults())
	aiClient := aiRuntime.NewRuntimeManager(runtimeBuilder).BuildDynamicClient()
	return service.NewInterviewService(
		interviewRepo,
		interviewMessageRepo,
		codingRepo,
		archiveRepo,
		aiClient.InterviewAgent,
		aiClient.QuizAnalyzer,
		industryRepo,
		service.RealtimeInterviewServiceOption{Enabled: cfg.Volcengine.Realtime.Enabled},
		aiClient.ResumeParser,
		service.AsyncDispatchOption{AsyncTaskRepo: asyncTaskRepo},
	)
}

// runWorkerMQ 使用 RabbitMQ 持续消费异步任务，覆盖学习诊断、爬虫导入、题目流水线和面试异步后处理任务。
func runWorkerMQ(ctx context.Context, cfg *config.Config, scraperService service.ScraperService, adminService service.AdminService, planService service.PlanService, interviewService service.InterviewService) error {
	handlers, err := buildWorkerTaskHandlers(scraperService, adminService, planService, interviewService)
	if err != nil {
		return err
	}
	consumer := mq.NewConsumer(cfg.RabbitMQ, mq.DefaultQueueSpecs(), handlers)
	return consumer.Start(ctx)
}

// buildWorkerTaskHandlers 组装 worker 需要注册到 RabbitMQ 消费器的任务处理器集合。
func buildWorkerTaskHandlers(scraperService service.ScraperService, adminService service.AdminService, planService service.PlanService, interviewService service.InterviewService) (map[string]mq.TaskHandler, error) {
	scraperProcessor, ok := scraperService.(scraperImportTaskProcessor)
	if !ok {
		return nil, fmt.Errorf("scraper service does not implement MQ import processor")
	}
	adminProcessor, ok := adminService.(adminQuestionPipelineTaskProcessor)
	if !ok {
		return nil, fmt.Errorf("admin service does not implement MQ pipeline processor")
	}
	planProcessor, ok := planService.(planFeedbackDiagnosisTaskProcessor)
	if !ok {
		return nil, fmt.Errorf("plan service does not implement MQ diagnosis processor")
	}
	planGenerateProcessor, ok := planService.(planGenerateTaskProcessor)
	if !ok {
		return nil, fmt.Errorf("plan service does not implement MQ generate processor")
	}
	resumeProcessor, ok := interviewService.(interviewResumeParseTaskProcessor)
	if !ok {
		return nil, fmt.Errorf("interview service does not implement MQ resume processor")
	}
	reportProcessor, ok := interviewService.(interviewReportTaskProcessor)
	if !ok {
		return nil, fmt.Errorf("interview service does not implement MQ report processor")
	}

	return map[string]mq.TaskHandler{
		"makejob.async.scraper.import.questions": mq.TaskHandlerFunc(func(ctx context.Context, message mq.TaskMessage) error {
			return processScraperImportTaskMessage(ctx, scraperProcessor, message)
		}),
		"makejob.async.admin.question.pipeline.build": mq.TaskHandlerFunc(func(ctx context.Context, message mq.TaskMessage) error {
			return processAdminQuestionPipelineTaskMessage(ctx, adminProcessor, message)
		}),
		"makejob.async.plan.feedback.diagnosis": mq.TaskHandlerFunc(func(ctx context.Context, message mq.TaskMessage) error {
			return processPlanFeedbackDiagnosisTaskMessage(ctx, planProcessor, message)
		}),
		"makejob.async.plan.generate": mq.TaskHandlerFunc(func(ctx context.Context, message mq.TaskMessage) error {
			return processPlanGenerateTaskMessage(ctx, planGenerateProcessor, message)
		}),
		"makejob.async.interview.resume.parse": mq.TaskHandlerFunc(func(ctx context.Context, message mq.TaskMessage) error {
			return processInterviewResumeParseTaskMessage(ctx, resumeProcessor, message)
		}),
		"makejob.async.interview.report.generate": mq.TaskHandlerFunc(func(ctx context.Context, message mq.TaskMessage) error {
			return processInterviewReportTaskMessage(ctx, reportProcessor, message)
		}),
	}, nil
}

// processScraperImportTaskMessage 解析爬虫导入消息并调用对应服务完成消费。
func processScraperImportTaskMessage(ctx context.Context, processor scraperImportTaskProcessor, message mq.TaskMessage) error {
	var payload mq.ScraperImportPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return fmt.Errorf("解析爬虫导入消息失败: %w", err)
	}
	return processor.ProcessImportTask(ctx, payload.ScraperTaskID)
}

// processAdminQuestionPipelineTaskMessage 解析题目流水线消息并调用对应服务完成消费。
func processAdminQuestionPipelineTaskMessage(ctx context.Context, processor adminQuestionPipelineTaskProcessor, message mq.TaskMessage) error {
	var payload mq.AdminQuestionPipelinePayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return fmt.Errorf("解析题目流水线消息失败: %w", err)
	}
	return processor.ProcessQuestionPipelineTask(ctx, payload.ScraperTaskID)
}

// processPlanFeedbackDiagnosisTaskMessage 解析学习任务反馈诊断消息并调用对应服务完成消费。
func processPlanFeedbackDiagnosisTaskMessage(ctx context.Context, processor planFeedbackDiagnosisTaskProcessor, message mq.TaskMessage) error {
	if message.TaskID == 0 {
		return fmt.Errorf("训练反馈诊断消息缺少 async task id")
	}
	return processor.ProcessTaskFeedbackDiagnosisTask(ctx, message.TaskID)
}

// processPlanGenerateTaskMessage 解析学习计划生成异步消息并调用对应服务完成消费。
func processPlanGenerateTaskMessage(ctx context.Context, processor planGenerateTaskProcessor, message mq.TaskMessage) error {
	if message.TaskID == 0 {
		return fmt.Errorf("学习计划生成消息缺少 async task id")
	}
	return processor.ProcessPlanGenerateTask(ctx, message.TaskID)
}

// processInterviewResumeParseTaskMessage 解析简历异步消息并调用对应服务完成消费。
func processInterviewResumeParseTaskMessage(ctx context.Context, processor interviewResumeParseTaskProcessor, message mq.TaskMessage) error {
	if message.TaskID == 0 {
		return fmt.Errorf("简历解析消息缺少 async task id")
	}
	return processor.ProcessInterviewResumeParseTask(ctx, message.TaskID)
}

// processInterviewReportTaskMessage 解析面试报告异步消息并调用对应服务完成消费。
func processInterviewReportTaskMessage(ctx context.Context, processor interviewReportTaskProcessor, message mq.TaskMessage) error {
	if message.TaskID == 0 {
		return fmt.Errorf("面试报告消息缺少 async task id")
	}
	return processor.ProcessInterviewReportTask(ctx, message.TaskID)
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
