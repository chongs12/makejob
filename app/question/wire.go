//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/conf"
	"makejob/app/question/internal/data"
	"makejob/app/question/internal/server"
	"makejob/app/question/internal/service"
	"makejob/pkg/auth"
)

var providerSet = wire.NewSet(
	data.NewData,
	data.NewQuestionRepo,
	biz.NewQuestionUseCase,
	service.NewQuestionService,
	server.NewGRPCServer,
	auth.NewInterceptor,
)

func wireApp(*conf.Bootstrap, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(providerSet))
}
