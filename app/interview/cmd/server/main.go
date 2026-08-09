package main

import (
	"flag"
	"fmt"
	"os"
	"time"

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
	logger := mlog.NewZapLogger("makejob.interview")
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

	archiveServiceToken, err := auth.GenerateToken(0, "interview-service@internal", "service", bc.JWT.Secret, 24*time.Hour)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create archive service token: %w", err)
	}
	archiveClient, err := data.NewLearningArchiveClient(bc.Archive, archiveServiceToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Archive client: %w", err)
	}

	membershipClient, err := data.NewMembershipClient(bc.Membership, archiveServiceToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Membership client: %w", err)
	}

	// 行业数据直接查本地 DB（行业是静态字典表，各服务本地都有副本）
	industryRepo := data.NewIndustryRepo(db)

	ragClient, err := data.NewRAGClient(bc.RAG)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create RAG client: %w", err)
	}

	codeRunnerClient, err := data.NewCodeRunnerClient(bc.CodeRunner)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CodeRunner client: %w", err)
	}

	// biz 层：业务用例
	timeoutMinutes := 40
	if bc.Interview != nil && bc.Interview.TimeoutMinutes > 0 {
		timeoutMinutes = bc.Interview.TimeoutMinutes
	}
	interviewUseCase := biz.NewInterviewUseCase(
		interviewRepo,
		aiClient,
		archiveClient,
		industryRepo,
		membershipClient,
		ragClient,
		codeRunnerClient,
		reportRepo,
		publisher,
		logger,
		timeoutMinutes,
	)

	// companion 客户端（TTS 预热）：配置存在时装配并注入用例，缺省则不预热，不影响主流程。
	var companionClient biz.TTSPrewarmClient
	if bc.Companion != nil && bc.Companion.ServiceAddr != "" {
		cc, err := data.NewCompanionClient(bc.Companion, archiveServiceToken)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create Companion client: %w", err)
		}
		companionClient = cc
		interviewUseCase.SetTTSPrewarmClient(cc)
	}

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
	if c, ok := membershipClient.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := companionClient.(closer); ok {
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
