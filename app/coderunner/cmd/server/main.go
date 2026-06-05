package main

import (
	"flag"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/coderunner/internal/biz"
	"makejob/app/coderunner/internal/conf"
	"makejob/app/coderunner/internal/data"
	"makejob/app/coderunner/internal/server"
	"makejob/app/coderunner/internal/service"
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
	// data 层：创建 Piston HTTP 客户端
	pistonClient := data.NewPistonClient(bc.Piston.Endpoint, bc.Piston.TimeoutMs, logger)

	// biz 层：业务用例
	coderunnerUseCase := biz.NewCodeRunnerUseCase(pistonClient, logger)

	// service 层：gRPC 服务实现
	coderunnerService := service.NewCodeRunnerService(coderunnerUseCase)

	// server 层：gRPC 服务器
	gs := server.NewGRPCServer(bc.Server, coderunnerService, logger)

	// 组装 Kratos app
	app := kratos.New(
		kratos.Name("makejob.coderunner"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs),
	)

	cleanup := func() {
		// 清理资源
	}

	return app, cleanup, nil
}
