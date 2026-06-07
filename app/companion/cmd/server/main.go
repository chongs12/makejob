package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/companion/internal/biz"
	"makejob/app/companion/internal/conf"
	"makejob/app/companion/internal/data"
	"makejob/app/companion/internal/server"
	"makejob/app/companion/internal/service"
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
	companionRepo := data.NewCompanionRepo(db)

	// data 层：AI 客户端
	aiClient, err := data.NewCompanionAIClient(bc.AI)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	// data 层：TTS 客户端
	ttsClient := data.NewTTsClient(bc.TTS)

	// biz 层：业务用例
	ttsVoice := ""
	if bc.TTS != nil {
		ttsVoice = bc.TTS.Voice
	}
	companionUseCase := biz.NewCompanionUseCase(companionRepo, aiClient, ttsClient, ttsVoice)

	// service 层：gRPC 服务实现
	companionService := service.NewCompanionService(companionUseCase)

	// auth 拦截器
	authInterceptor := auth.NewInterceptor(bc.JWT.Secret)

	// server 层：gRPC 服务器
	gs := server.NewGRPCServer(bc.Server, companionService, authInterceptor, logger)

	// 组装 Kratos app
	app := kratos.New(
		kratos.Name("makejob.companion"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs),
	)

	cleanup := func() {
		if closer, ok := aiClient.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}

	return app, cleanup, nil
}
