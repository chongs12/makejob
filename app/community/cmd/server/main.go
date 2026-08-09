package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/community/internal/biz"
	"makejob/app/community/internal/conf"
	"makejob/app/community/internal/data"
	"makejob/app/community/internal/server"
	"makejob/app/community/internal/service"
	"makejob/pkg/auth"
	mlog "makejob/pkg/logger"
	"makejob/pkg/telemetry"
)

var flagConf string

func main() {
	// FIX: 将init()中的flag注册移到main()开头（禁止使用init()函数）
	flag.StringVar(&flagConf, "conf", "configs/config.yaml", "config path, eg: -conf configs/config.yaml")
	flag.Parse()

	// 初始化日志
	logger := mlog.NewZapLogger("makejob.community")
	log.SetLogger(logger)

	// 加载配置
	bc, err := conf.Load(flagConf)
	if err != nil {
		log.Errorf("failed to load config: %v", err)
		os.Exit(1)
	}

	// 手动组装依赖
	// telemetry.Init：必须在 wireApp 之前，让 otelgrpc 拦截器拿到全局 TracerProvider
	telCleanup, err := telemetry.Init(telemetry.Config{
		OTLPEndpoint: bc.Telemetry.OTLPEndpoint,
		ServiceName:  bc.Telemetry.ServiceName,
		SampleRatio:  bc.Telemetry.SampleRatio,
		HTTPPort:     bc.Telemetry.HTTPPort,
	})
	if err != nil {
		log.Errorf("failed to init telemetry: %v", err)
		os.Exit(1)
	}
	defer telCleanup()
	app, cleanup, err := wireApp(bc, logger)
	if err != nil {
		log.Errorf("failed to wire app: %v", err)
		telCleanup()
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
	communityRepo := data.NewCommunityRepo(db)
	likeRepo := data.NewLikeRepo(db)

	// biz 层：业务用例
	communityUseCase := biz.NewCommunityUseCase(communityRepo, likeRepo)

	// service 层：gRPC 服务实现
	communityService := service.NewCommunityService(communityUseCase)

	// auth 拦截器
	authInterceptor := auth.NewInterceptor(bc.JWT.Secret)

	// server 层：gRPC 服务器
	gs := server.NewGRPCServer(bc.Server, communityService, authInterceptor, logger)

	// 组装 Kratos app
	app := kratos.New(
		kratos.Name("makejob.community"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs),
	)

	cleanup := func() {
		// 清理资源
	}

	return app, cleanup, nil
}
