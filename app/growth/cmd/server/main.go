package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/growth/internal/biz"
	"makejob/app/growth/internal/conf"
	"makejob/app/growth/internal/data"
	"makejob/app/growth/internal/server"
	"makejob/app/growth/internal/service"
	"makejob/pkg/auth"
	mlog "makejob/pkg/logger"
)

var flagConf string

func main() {
	// FIX: 将init()中的flag注册移到main()开头（禁止使用init()函数）
	flag.StringVar(&flagConf, "conf", "configs/config.yaml", "config path, eg: -conf configs/config.yaml")
	flag.Parse()

	// 初始化日志
	logger := mlog.NewZapLogger("makejob.growth")
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
	growthRepo := data.NewGrowthRepo(db)

	// data 层：下游服务客户端（返回 biz 接口类型）
	questionClient, err := data.NewQuestionClient(bc.DepServices)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create question client: %w", err)
	}
	planClient, err := data.NewPlanClient(bc.DepServices)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create plan client: %w", err)
	}
	archiveClient, err := data.NewArchiveClient(bc.DepServices)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create archive client: %w", err)
	}
	interviewClient, err := data.NewInterviewClient(bc.DepServices)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create interview client: %w", err)
	}

	// biz 层：业务用例
	growthUseCase := biz.NewGrowthUseCase(
		growthRepo,
		questionClient,
		planClient,
		archiveClient,
		interviewClient,
		logger,
	)

	// service 层：gRPC 服务实现
	growthService := service.NewGrowthService(growthUseCase)

	// auth 拦截器
	authInterceptor := auth.NewInterceptor(bc.JWT.Secret)

	// server 层：gRPC 服务器
	gs := server.NewGRPCServer(bc.Server, growthService, authInterceptor, logger)

	// 组装 Kratos app
	app := kratos.New(
		kratos.Name("makejob.growth"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs),
	)

	// 收集所有可关闭资源
	type closer interface{ Close() error }
	var closers []closer
	if c, ok := questionClient.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := planClient.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := archiveClient.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := interviewClient.(closer); ok {
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
