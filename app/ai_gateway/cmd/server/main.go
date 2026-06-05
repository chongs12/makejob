package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/ai_gateway/internal/biz"
	"makejob/app/ai_gateway/internal/conf"
	"makejob/app/ai_gateway/internal/data"
	"makejob/app/ai_gateway/internal/server"
	"makejob/app/ai_gateway/internal/service"
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
	configRepo := data.NewAIConfigRepo(db)
	promptRepo := data.NewPromptRepo(db)
	callLogRepo := data.NewCallLogRepo(db)

	// data 层：ARK LLM 客户端
	llmClient := data.NewArkLLMClient(bc.ARK)

	// biz 层：各场景业务用例
	interviewUC := biz.NewInterviewAgentUseCase(configRepo, promptRepo, callLogRepo, llmClient, logger)
	planUC := biz.NewPlanAgentUseCase(configRepo, promptRepo, callLogRepo, llmClient, logger)
	companionUC := biz.NewCompanionAgentUseCase(configRepo, promptRepo, callLogRepo, llmClient, logger)
	quizUC := biz.NewQuizAnalyzerUseCase(configRepo, promptRepo, callLogRepo, llmClient, logger)
	resumeUC := biz.NewResumeParserUseCase(configRepo, promptRepo, callLogRepo, llmClient, logger)
	live2dUC := biz.NewLive2DDirectorUseCase(configRepo, promptRepo, callLogRepo, llmClient, logger)

	// service 层：gRPC 服务实现
	aiGatewayService := service.NewAIGatewayService(
		interviewUC, planUC, companionUC, quizUC, resumeUC, live2dUC,
	)

	// auth 拦截器
	authInterceptor := auth.NewInterceptor(bc.JWT.Secret)

	// server 层：gRPC 服务器
	gs := server.NewGRPCServer(bc.Server, aiGatewayService, authInterceptor, logger)

	// 组装 Kratos app
	app := kratos.New(
		kratos.Name("makejob.ai_gateway"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs),
	)

	cleanup := func() {
		// 清理资源
	}

	return app, cleanup, nil
}
