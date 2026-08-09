package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	adminv1 "makejob/api/makejob/admin/v1"
	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/conf"
	"makejob/app/question/internal/data"
	"makejob/app/question/internal/server"
	"makejob/app/question/internal/service"
	"makejob/pkg/auth"
	mlog "makejob/pkg/logger"
	"makejob/pkg/mq"
)

var flagConf string

func main() {
	// FIX: 将init()中的flag注册移到main()开头（禁止使用init()函数）
	flag.StringVar(&flagConf, "conf", "configs/config.yaml", "config path")
	flag.Parse()

	logger := mlog.NewZapLogger("makejob.question")
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

// wireApp 手动组装题目服务依赖，并为异步 MQ 回写准备受保护 Admin RPC 所需凭证。
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

	// 创建 CodeRunner gRPC 客户端
	codeRunner, err := data.NewCodeRunnerClient(bc.AI)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CodeRunner client: %w", err)
	}

	// 创建题目生成 AI 客户端
	questionGenerator, err := data.NewQuestionGeneratorClient(bc.AI)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create question generator client: %w", err)
	}

	// 创建考试和题集仓储
	examRepo := data.NewExamRepo(db)
	questionSetRepo := data.NewQuestionSetRepo(db)

	// 创建 learning_archive gRPC 客户端（替代本地 LearningArchiveRepo）
	archiveServiceToken, err := buildServiceAccessToken(bc.JWT.Secret)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create learning_archive service token: %w", err)
	}
	learningArchiveClient, err := data.NewLearningArchiveClient(bc.DependentServices, archiveServiceToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create learning_archive client: %w", err)
	}

	adminConn, err := grpc.Dial(bc.AI.AdminAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create admin client: %w", err)
	}
	adminClient := adminv1.NewAdminServiceClient(adminConn)
	adminAccessToken, err := buildAdminAccessToken(bc.JWT.Secret)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create admin access token: %w", err)
	}

	uc := biz.NewQuestionUseCase(
		questionRepo, recordRepo, favoriteRepo, noteRepo,
		categoryRepo, industryRepo, quizAnalyzer,
		codeRunner, examRepo, questionSetRepo, questionGenerator,
		learningArchiveClient,
	)

	// 创建 MQ 发布器用于 RAG 同步
	mqPublisher, err := mq.NewPublisher(bc.MQ.URL, bc.MQ.Exchange)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create MQ publisher: %w", err)
	}
	ragSyncPub := data.NewRAGSyncPublisher(mqPublisher)
	uc.SetRAGSyncPublisher(ragSyncPub)

	svc := service.NewQuestionService(uc, bc.JWT.Secret)
	authInterceptor := auth.NewInterceptor(bc.JWT.Secret)
	gs := server.NewGRPCServer(bc.Server, svc, authInterceptor, logger)

	// MQ 消费者（处理题目流水线和 scraper 导入任务）
	mqConsumer, err := server.NewMQConsumer(bc.MQ.URL, uc, adminClient, adminAccessToken, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create MQ consumer: %w", err)
	}

	app := kratos.New(
		kratos.Name("makejob.question"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs, mqConsumer),
	)

	// Collect closable resources
	type closer interface{ Close() error }
	var closers []closer
	if c, ok := quizAnalyzer.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := codeRunner.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := questionGenerator.(closer); ok {
		closers = append(closers, c)
	}
	if c, ok := learningArchiveClient.(closer); ok {
		closers = append(closers, c)
	}
	closers = append(closers, adminConn)
	closers = append(closers, mqPublisher)

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

// buildAdminAccessToken 为题目服务生成内部 Admin 回写令牌，供异步 MQ 任务调用受保护的管理 RPC。
func buildAdminAccessToken(jwtSecret string) (string, error) {
	return auth.GenerateToken(0, "question-service@internal", "admin", jwtSecret, 24*time.Hour)
}

// buildServiceAccessToken 为题目服务生成内部服务令牌，供调用 learning_archive 等受保护的 gRPC 服务。
func buildServiceAccessToken(jwtSecret string) (string, error) {
	return auth.GenerateToken(0, "question-service@internal", "service", jwtSecret, 24*time.Hour)
}
