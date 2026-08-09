package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/user/internal/biz"
	"makejob/app/user/internal/conf"
	"makejob/app/user/internal/data"
	"makejob/app/user/internal/server"
	"makejob/app/user/internal/service"
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
	logger := mlog.NewZapLogger("makejob.user")
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

	// data 层：Redis 客户端
	rdb := data.NewRedisClient(bc.Data.Redis)
	tokenBlacklist := data.NewTokenBlacklist(rdb)

	// data 层：仓库实现
	userRepo := data.NewUserRepo(db)

	// biz 层：业务用例
	userUseCase := biz.NewUserUseCase(userRepo)

	// service 层：gRPC 服务实现
	userService := service.NewUserService(userUseCase, bc.JWT, tokenBlacklist, logger)

	// auth 拦截器（FIX B1: 注入黑名单检查器）
	authInterceptor := auth.NewInterceptor(bc.JWT.Secret,
		auth.WithBlacklistChecker(tokenBlacklist),
		auth.WithLogger(logger),
	)

	// server 层：gRPC 服务器
	gs := server.NewGRPCServer(bc.Server, userService, authInterceptor, logger)

	// 组装 Kratos app
	app := kratos.New(
		kratos.Name("makejob.user"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs),
	)

	cleanup := func() {
		if err := rdb.Close(); err != nil {
			log.Errorf("failed to close redis: %v", err)
		}
	}

	return app, cleanup, nil
}
