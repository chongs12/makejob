package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"makejob/app/admin/internal/biz"
	"makejob/app/admin/internal/conf"
	"makejob/app/admin/internal/data"
	"makejob/app/admin/internal/server"
	"makejob/app/admin/internal/service"
	"makejob/pkg/auth"
	mlog "makejob/pkg/logger"
)

var flagConf string

func main() {
	// FIX: 将init()中的flag注册移到main()开头（禁止使用init()函数）
	flag.StringVar(&flagConf, "conf", "configs/config.yaml", "config path, eg: -conf configs/config.yaml")
	flag.Parse()

	// 初始化日志
	logger := mlog.NewZapLogger("makejob.admin")
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

// wireApp 手动组装依赖，创建下游 gRPC 客户端并注入到业务层
func wireApp(bc *conf.Bootstrap, logger log.Logger) (*kratos.App, func(), error) {
	// data 层：数据库连接
	db, err := data.NewData(bc.Data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// data 层：仓库实现
	adminRepo := data.NewAdminRepo(db)
	publisher, err := data.NewMQPublisher(bc.MQ)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create MQ publisher: %w", err)
	}

	// 创建下游服务 gRPC 连接
	var userConn, questionConn, interviewConn, aiGatewayConn *grpc.ClientConn
	cleanupFns := make([]func(), 0)

	if bc.DependentServices != nil && bc.DependentServices.UserAddr != "" {
		userConn, err = grpc.NewClient(bc.DependentServices.UserAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect user service: %w", err)
		}
		cleanupFns = append(cleanupFns, func() { userConn.Close() })
	}

	if bc.DependentServices != nil && bc.DependentServices.QuestionAddr != "" {
		questionConn, err = grpc.NewClient(bc.DependentServices.QuestionAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect question service: %w", err)
		}
		cleanupFns = append(cleanupFns, func() { questionConn.Close() })
	}

	if bc.DependentServices != nil && bc.DependentServices.InterviewAddr != "" {
		interviewConn, err = grpc.NewClient(bc.DependentServices.InterviewAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect interview service: %w", err)
		}
		cleanupFns = append(cleanupFns, func() { interviewConn.Close() })
	}

	if bc.DependentServices != nil && bc.DependentServices.AIGatewayAddr != "" {
		aiGatewayConn, err = grpc.NewClient(bc.DependentServices.AIGatewayAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect AI gateway service: %w", err)
		}
		cleanupFns = append(cleanupFns, func() { aiGatewayConn.Close() })
	}

	// data 层：创建下游服务客户端
	var userClient biz.UserClient
	if userConn != nil {
		userClient = data.NewUserClient(userConn, logger)
	} else {
		return nil, nil, fmt.Errorf("user service address is required (dependent_services.user_addr)")
	}

	var questionClient biz.QuestionClient
	if questionConn != nil {
		questionClient = data.NewQuestionClient(questionConn, adminRepo, logger)
	} else {
		return nil, nil, fmt.Errorf("question service address is required (dependent_services.question_addr)")
	}

	var interviewClient biz.InterviewClient
	if interviewConn != nil {
		interviewClient = data.NewInterviewClient(interviewConn, logger)
	} else {
		return nil, nil, fmt.Errorf("interview service address is required (dependent_services.interview_addr)")
	}

	// AI Gateway 客户端（可选，但调试功能需要）
	var aiGatewayClient biz.AIGatewayClient
	if aiGatewayConn != nil {
		aiGatewayClient = data.NewAIGatewayClient(aiGatewayConn, logger)
	} else {
		aiGatewayClient = data.NewAIGatewayClientNoop()
	}

	// RAG 服务客户端（可选）
	var ragConn *grpc.ClientConn
	if bc.DependentServices != nil && bc.DependentServices.RAGAddr != "" {
		ragConn, err = grpc.NewClient(bc.DependentServices.RAGAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect RAG service: %w", err)
		}
		cleanupFns = append(cleanupFns, func() { ragConn.Close() })
	}

	// biz 层：业务用例
	adminUseCase := biz.NewAdminUseCase(adminRepo, userClient, questionClient, interviewClient, aiGatewayClient)

	// 注入 RAG 客户端
	if ragConn != nil {
		ragClient := data.NewRAGClient(ragConn, logger)
		adminUseCase.SetRAGClient(ragClient)
	}

	// 注入爬虫依赖（Admin 自有能力）
	scraperProvider := data.NewScraperProvider()
	scraperCleaner := data.NewScraperCleaner()
	adminUseCase.SetScraperDeps(scraperProvider, scraperCleaner, publisher)

	// service 层：gRPC 服务实现
	adminService := service.NewAdminService(adminUseCase, publisher)

	// auth 拦截器
	authInterceptor := auth.NewInterceptor(bc.JWT.Secret)

	// server 层：gRPC 服务器
	gs := server.NewGRPCServer(bc.Server, adminService, authInterceptor, logger)

	// 组装 Kratos app
	app := kratos.New(
		kratos.Name("makejob.admin"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(gs),
	)

	cleanup := func() {
		for _, fn := range cleanupFns {
			fn()
		}
		_ = publisher.Close()
	}

	return app, cleanup, nil
}
