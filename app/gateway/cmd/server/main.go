package main

import (
	"flag"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/gateway/internal/conf"
	"makejob/app/gateway/internal/proxy"
	mlog "makejob/pkg/logger"
)

var flagConf string

func init() {
	flag.StringVar(&flagConf, "conf", "configs/config.yaml", "config path")
}

func main() {
	flag.Parse()

	logger := mlog.NewZapLogger()
	log.SetLogger(logger)

	bc, err := conf.Load(flagConf)
	if err != nil {
		log.Errorf("failed to load config: %v", err)
		os.Exit(1)
	}

	// 创建网关
	gw, err := proxy.NewGateway(bc)
	if err != nil {
		log.Errorf("failed to create gateway: %v", err)
		os.Exit(1)
	}
	defer gw.Close()

	// 创建 Gin 引擎
	r := gin.Default()

	// CORS 中间件
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 注册路由
	gw.RegisterRoutes(r)

	// 启动 HTTP 服务器
	addr := bc.Server.HTTP.Addr
	log.Infof("gateway listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Errorf("failed to run gateway: %v", err)
		os.Exit(1)
	}
}
