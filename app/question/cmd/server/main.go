package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/conf"
	"makejob/app/question/internal/data"
	"makejob/app/question/internal/server"
	"makejob/app/question/internal/service"
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

	questionRepo := data.NewQuestionRepo(db)
	recordRepo := data.NewRecordRepo(db)
	favoriteRepo := data.NewFavoriteRepo(db)
	noteRepo := data.NewNoteRepo(db)
	categoryRepo := data.NewCategoryRepo(db)
	industryRepo := data.NewIndustryRepo(db)
	quizAnalyzer, err := data.NewQuizAnalyzerClient(bc.AI)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	uc := biz.NewQuestionUseCase(questionRepo, recordRepo, favoriteRepo, noteRepo, categoryRepo, industryRepo, quizAnalyzer)
	svc := service.NewQuestionService(uc)
	authInterceptor := auth.NewInterceptor(bc.JWT.Secret)
	gs := server.NewGRPCServer(bc.Server, svc, authInterceptor, logger)

	app := kratos.New(
		kratos.Name("makejob.question"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs),
	)

	// Collect closable resources
	type closer interface{ Close() error }
	var closers []closer
	if c, ok := quizAnalyzer.(closer); ok {
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
