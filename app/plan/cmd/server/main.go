package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/plan/internal/biz"
	"makejob/app/plan/internal/conf"
	"makejob/app/plan/internal/data"
	"makejob/app/plan/internal/server"
	"makejob/app/plan/internal/service"
	"makejob/pkg/auth"
	mlog "makejob/pkg/logger"
)

var flagConf string

func main() {
	flag.StringVar(&flagConf, "conf", "configs/config.yaml", "config path, eg: -conf configs/config.yaml")
	flag.Parse()

	// 初始化日志
	logger := mlog.NewZapLogger()
	log.SetLogger(logger)

	// 加载配置
	bc, err := conf.Load(flagConf)
	if err != nil {
		log.Errorf("failed to load config: %v", err)
		os.Exit(1)
	}

	// 手动组装依赖
	app, cleanup, err := wireApp(bc, logger)
	if err != nil {
		log.Errorf("failed to wire app: %v", err)
		os.Exit(1)
	}
	defer cleanup()

	// 启动应用
	if err := app.Run(); err != nil {
		log.Errorf("failed to run app: %v", err)
		os.Exit(1)
	}
}

// wireApp 手动组装依赖
func wireApp(bc *conf.Bootstrap, logger log.Logger) (*kratos.App, func(), error) {
	// data 层：数据库连接
	db, err := data.NewData(bc.Data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// data 层：仓库实现
	planRepo := data.NewPlanRepo(db)
	taskRepo := data.NewTaskRepo(db)
	feedbackRepo := data.NewTaskFeedbackRepo(db)
	adjustmentRepo := data.NewPlanAdjustmentRepo(db)
	industryRepo := data.NewIndustryRepo(db)

	// data 层：MQ 发布者
	publisher, err := data.NewMQPublisher(bc.MQ)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create MQ publisher: %w", err)
	}

	// data 层：AI 服务客户端
	aiClient, err := data.NewPlanAgentClient(bc.AI)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	// data 层：诊断分析客户端
	diagClient, err := data.NewDiagnosisClient(bc.AI)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create diagnosis client: %w", err)
	}

	// data 层：learning_archive gRPC 客户端
	archiveServiceToken, err := auth.GenerateToken(0, "plan-service@internal", "service", bc.JWT.Secret, 24*time.Hour)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create archive service token: %w", err)
	}
	archiveClient, err := data.NewLearningArchiveClient(bc.DependentServices, archiveServiceToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create learning_archive client: %w", err)
	}

	// biz 层：业务用例
	planUseCase := biz.NewPlanUseCase(planRepo, taskRepo, feedbackRepo, adjustmentRepo, industryRepo, aiClient, diagClient, publisher, archiveClient, logger)

	// service 层：gRPC 服务实现
	planService := service.NewPlanService(planUseCase)

	// auth 拦截器
	authInterceptor := auth.NewInterceptor(bc.JWT.Secret)

	// server 层：gRPC 服务器
	gs := server.NewGRPCServer(bc.Server, planService, authInterceptor, logger)

	// server 层：MQ 消费者
	mqConsumer, err := server.NewMQConsumer(bc.MQ, planUseCase)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create MQ consumer: %w", err)
	}

	// 组装 Kratos app
	app := kratos.New(
		kratos.Name("makejob.plan"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs, mqConsumer),
	)

	// 收集可关闭资源
	type closer interface{ Close() error }
	var closers []closer
	if c, ok := aiClient.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := diagClient.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := archiveClient.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := publisher.(closer); ok {
		closers = append(closers, c)
	}

	cleanup := func() {
		for _, c := range closers {
			if err := c.Close(); err != nil {
				log.Errorf("failed to close resource: %v", err)
			}
		}
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}

	return app, cleanup, nil
}
