package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/realtime/internal/biz"
	"makejob/app/realtime/internal/conf"
	"makejob/app/realtime/internal/data"
	"makejob/app/realtime/internal/server"
	"makejob/app/realtime/internal/service"
	"makejob/pkg/auth"
	mlog "makejob/pkg/logger"
)

var flagConf string

func main() {
	// FIX: 将init()中的flag注册移到main()开头（禁止使用init()函数）
	flag.StringVar(&flagConf, "conf", "configs/config.yaml", "config path, eg: -conf configs/config.yaml")
	flag.Parse()

	// 初始化日志
	logger := mlog.NewZapLogger("makejob.realtime")
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

// wireApp 手动组装所有依赖
func wireApp(bc *conf.Bootstrap, logger log.Logger) (*kratos.App, func(), error) {
	// data 层：下游服务客户端
	interviewClient, interviewCloser, err := data.NewInterviewClient(bc.DependentServices)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create interview client: %w", err)
	}

	ragClient, ragCloser, err := data.NewRAGClient(bc.DependentServices)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create RAG client: %w", err)
	}

	// data 层：火山引擎连接工厂
	volcFactory := data.NewVolcEngineFactory(bc.Volcengine)
	volcSessionFactory := data.NewVolcEngineSessionFactory(bc.Volcengine)

	// biz 层：会话管理器和业务用例（对齐单体：纯内存管理，不依赖数据库）
	sessionManager := biz.NewSessionManager()
	realtimeUseCase := biz.NewRealtimeUseCase(
		interviewClient,
		ragClient,
		sessionManager,
		volcFactory,
		volcSessionFactory,
		bc.Volcengine,
		logger,
	)

	// service 层：gRPC 服务实现
	realtimeService := service.NewRealtimeService(realtimeUseCase, "ws", bc.Server.HTTP.Addr)

	// auth 拦截器
	authInterceptor := auth.NewInterceptor(bc.JWT.Secret)

	// server 层：gRPC 服务器
	gs := server.NewGRPCServer(bc.Server, realtimeService, authInterceptor, logger)

	// server 层：HTTP/WebSocket 服务器
	hs := server.NewHTTPServer(bc.Server, realtimeUseCase, bc.JWT.Secret, logger)

	// 组装 Kratos app
	app := kratos.New(
		kratos.Name("makejob.realtime"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs, hs),
	)

	cleanup := func() {
		if interviewCloser != nil {
			_ = interviewCloser.Close()
		}
		if ragCloser != nil {
			_ = ragCloser.Close()
		}
	}

	return app, cleanup, nil
}
