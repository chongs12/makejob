package main

import (
	"flag"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/gateway/internal/conf"
	"makejob/app/gateway/internal/proxy"
	"makejob/pkg/live2dassets"
	mlog "makejob/pkg/logger"
)

var flagConf string

// main 启动 Gateway HTTP 服务，并挂载前端可直接访问的 Live2D 静态资源目录。
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

	// 创建网关
	gw, err := proxy.NewGateway(bc)
	if err != nil {
		log.Errorf("failed to create gateway: %v", err)
		os.Exit(1)
	}
	defer gw.Close()

	// 创建 Gin 引擎
	r := gin.Default()
	if assetsDir, err := live2dassets.EnsureAssetsDir(); err == nil && assetsDir != "" {
		r.StaticFS(live2dassets.MountPath, gin.Dir(assetsDir, false))
	} else if err != nil {
		log.Warnf("failed to mount live2d assets dir: %v", err)
	}

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

	// 响应包装中间件：将所有 JSON 响应包成 { code, message, data } 格式
	r.Use(proxy.WrapResponseMiddleware())

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
