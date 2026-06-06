package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/conf"
	"makejob/app/interview/internal/data"
	"makejob/app/interview/internal/server"
	"makejob/app/interview/internal/service"
	"makejob/pkg/auth"
	mlog "makejob/pkg/logger"
)

var flagConf string

func main() {
	// FIX: 将init()中的flag注册移到main()开头（禁止使用init()函数）
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

	// 手动组装依赖（体现依赖注入模式，可用 Wire 自动化）
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

// wireApp 手动组装依赖（替代 Wire 自动生成）
func wireApp(bc *conf.Bootstrap, logger log.Logger) (*kratos.App, func(), error) {
	// data 层：数据库连接
	db, err := data.NewData(bc.Data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// data 层：各仓库实现
	interviewRepo := data.NewInterviewRepo(db)
	reportRepo := data.NewReportRepo(db)

	// data 层：MQ 发布者
	publisher, err := data.NewMQPublisher(bc.MQ)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create MQ publisher: %w", err)
	}

	// data 层：gRPC 客户端
	aiClient, err := data.NewAIServiceClient(bc.AI)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	archiveClient, err := data.NewLearningArchiveClient(bc.Archive)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Archive client: %w", err)
	}

	industryClient, err := data.NewIndustryClient(bc.Industry)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Industry client: %w", err)
	}

	ragClient, err := data.NewRAGClient(bc.RAG)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create RAG client: %w", err)
	}

	codeRunnerClient, err := data.NewCodeRunnerClient(bc.CodeRunner)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CodeRunner client: %w", err)
	}

	// biz 层：业务用例
	interviewUseCase := biz.NewInterviewUseCase(
		interviewRepo,
		aiClient,
		archiveClient,
		industryClient,
		ragClient,
		codeRunnerClient,
		reportRepo,
		publisher,
		logger,
	)

	// service 层：gRPC 服务实现
	interviewService := service.NewInterviewService(interviewUseCase)

	// auth 拦截器
	authInterceptor := auth.NewInterceptor(bc.JWT.Secret)

	// server 层：gRPC 服务器（Kratos transport）
	gs := server.NewGRPCServer(bc.Server, interviewService, authInterceptor, logger)

	// server 层：MQ 消费者
	mqConsumer, err := server.NewMQConsumer(bc.MQ, interviewUseCase)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create MQ consumer: %w", err)
	}

	// 组装 Kratos app
	app := kratos.New(
		kratos.Name("makejob.interview"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs, mqConsumer),
	)

	// Collect all closable resources for cleanup
	type closer interface{ Close() error }
	var closers []closer
	if c, ok := aiClient.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := archiveClient.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := industryClient.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := ragClient.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := codeRunnerClient.(closer); ok {
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
