package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/learning_archive/internal/biz"
	"makejob/app/learning_archive/internal/conf"
	"makejob/app/learning_archive/internal/data"
	"makejob/app/learning_archive/internal/server"
	"makejob/app/learning_archive/internal/service"
	"makejob/pkg/auth"
	mlog "makejob/pkg/logger"
)

var flagConf string

func main() {
	// FIX: 将init()中的flag注册移到main()开头（禁止使用init()函数）
	flag.StringVar(&flagConf, "conf", "configs/config.yaml", "config path")
	flag.Parse()
	logger := mlog.NewZapLogger()
	log.SetLogger(logger)

	bc, err := conf.Load(flagConf)
	if err != nil {
		log.Errorf("failed to load config: %v", err)
		os.Exit(1)
	}

	app, cleanup, err := wireApp(bc, logger)
	if err != nil {
		log.Errorf("failed to wire app: %v", err)
		os.Exit(1)
	}
	defer cleanup()

	if err := app.Run(); err != nil {
		log.Errorf("failed to run app: %v", err)
		os.Exit(1)
	}
}

func wireApp(bc *conf.Bootstrap, logger log.Logger) (*kratos.App, func(), error) {
	db, err := data.NewData(bc.Data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect database: %w", err)
	}

	repo := data.NewArchiveRepo(db)
	uc := biz.NewArchiveUseCase(repo)
	svc := service.NewArchiveService(uc)
	authInterceptor := auth.NewInterceptor(bc.JWT.Secret)
	gs := server.NewGRPCServer(bc.Server, svc, authInterceptor, logger)

	app := kratos.New(
		kratos.Name("makejob.learning_archive"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs),
	)
	cleanup := func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}
	return app, cleanup, nil
}
