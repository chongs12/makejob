package main

import (
	"context"
	"flag"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"

	"makejob/app/rag/internal/biz"
	"makejob/app/rag/internal/conf"
	"makejob/app/rag/internal/data"
	"makejob/app/rag/internal/server"
	"makejob/app/rag/internal/service"
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
	ctx := context.Background()

	// data 层：Milvus + Ark 客户端（同时实现 Embedder 和 VectorStore）
	milvusCli, err := data.NewMilvusClient(ctx, bc.RAG, logger)
	if err != nil {
		return nil, nil, err
	}

	// biz 层：业务用例
	retrieveUC := biz.NewRetrieveUseCase(milvusCli, milvusCli, bc.RAG.CollectionName, bc.RAG.TopK, logger)
	indexUC := biz.NewIndexUseCase(milvusCli, milvusCli, bc.RAG.CollectionName, logger)
	syncHandler := biz.NewSyncHandler(milvusCli, milvusCli, bc.RAG.CollectionName, logger)

	// service 层：gRPC 服务实现
	ragService := service.NewRAGService(retrieveUC, indexUC, syncHandler, milvusCli, bc.RAG.CollectionName, bc.RAG.EmbedModel)

	// server 层：gRPC 服务器
	gs := server.NewGRPCServer(bc.Server, ragService, logger)

	// server 层：MQ 消费者
	var mqConsumer *server.MQConsumer
	transports := []transport.Server{gs}

	if bc.MQ != nil && bc.MQ.URL != "" {
		mqConsumer, err = server.NewMQConsumer(bc.MQ.URL, syncHandler, logger)
		if err != nil {
			log.Warnf("MQ 消费者创建失败，将以纯 gRPC 模式启动: %v", err)
		} else {
			transports = append(transports, mqConsumer)
		}
	}

	// 组装 Kratos app
	app := kratos.New(
		kratos.Name("makejob.rag"),
		kratos.Version("1.0.0"),
		kratos.Logger(logger),
		kratos.Server(transports...),
	)

	cleanup := func() {
		if milvusCli != nil {
			if err := milvusCli.Close(); err != nil {
				log.Errorf("关闭 Milvus 客户端失败: %v", err)
			}
		}
	}

	return app, cleanup, nil
}
