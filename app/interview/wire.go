//go:build wireinject
// +build wireinject

// Wire 依赖注入定义文件
// 此文件仅在 wire 命令运行时编译，用于自动生成 wire_gen.go
// 运行方式: wire ./app/interview/...

package main

import (
	"github.com/google/wire"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/conf"
	"makejob/app/interview/internal/data"
	"makejob/app/interview/internal/server"
	"makejob/app/interview/internal/service"
	"makejob/pkg/auth"
)

// providerSet 定义 Interview 服务的完整依赖图
var providerSet = wire.NewSet(
	// 配置提取
	wire.FieldsOf(new(*conf.Bootstrap), "AI", "Industry"),

	// data 层
	data.NewData,
	data.NewInterviewRepo,
	data.NewAIServiceClient,
	data.NewLearningArchiveClient,
	data.NewIndustryClient,

	// biz 层
	biz.NewInterviewUseCase,

	// service 层
	service.NewInterviewService,

	// server 层
	server.NewGRPCServer,
	server.NewMQConsumer,

	// auth
	auth.NewInterceptor,
)

// wireApp 由 Wire 自动生成依赖注入代码
func wireApp(*conf.Bootstrap, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		providerSet,
	))
}
