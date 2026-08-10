package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v2/log"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"makejob/app/gateway/internal/conf"
	"makejob/app/gateway/internal/proxy"
	"makejob/pkg/live2dassets"
	mlog "makejob/pkg/logger"
	"makejob/pkg/telemetry"
)

var flagConf string

// main 启动 Gateway HTTP 服务，并挂载前端可直接访问的 Live2D 静态资源目录。
func main() {
	// FIX: 将init()中的flag注册移到main()开头（禁止使用init()函数）
	flag.StringVar(&flagConf, "conf", "configs/config.yaml", "config path")
	flag.Parse()

	logger := mlog.NewZapLogger("makejob.gateway")
	log.SetLogger(logger)

	bc, err := conf.Load(flagConf)
	if err != nil {
		log.Errorf("failed to load config: %v", err)
		os.Exit(1)
	}

	// telemetry.Init：OTel tracer/metrics + :6060 可观测性 HTTP server。
	// 必须在 proxy.NewGateway 之前，让下游 gRPC 客户端的 otelgrpc 拦截器拿到全局 TracerProvider。
	telCleanup, err := telemetry.Init(telemetry.Config{
		OTLPEndpoint: bc.Telemetry.OTLPEndpoint,
		ServiceName:  bc.Telemetry.ServiceName,
		SampleRatio:  bc.Telemetry.SampleRatio,
		HTTPPort:     bc.Telemetry.HTTPPort,
	})
	if err != nil {
		log.Errorf("failed to init telemetry: %v", err)
		os.Exit(1)
	}
	defer telCleanup()

	// 创建网关
	gw, err := proxy.NewGateway(bc)
	if err != nil {
		telCleanup() // os.Exit 不跑 defer
		log.Errorf("failed to create gateway: %v", err)
		os.Exit(1)
	}
	defer gw.Close()

	// 创建 Gin 引擎（gin.New 不带默认 Logger/Recovery，手动注册以接入 kratos log + otelgin）
	r := gin.New()
	if assetsDir, err := live2dassets.EnsureAssetsDir(); err == nil && assetsDir != "" {
		r.StaticFS(live2dassets.MountPath, gin.Dir(assetsDir, false))
	} else if err != nil {
		log.Warnf("failed to mount live2d assets dir: %v", err)
	}

	// otelgin 中间件（最外层）：创建 root span，前端无 OTel 埋点，gateway 是 W3C trace 的起点。
	r.Use(otelgin.Middleware("makejob.gateway"))
	// panic 恢复（替代 gin.Default 自带的 Recovery）
	r.Use(gin.Recovery())
	// access log 走 kratos log，带 otelgin span 的 trace_id（替代 gin.Default 自带的 Logger）
	r.Use(proxy.GinLoggerMiddleware())

	// 业务 HTTP 指标（http_requests_total / http_request_duration_seconds）
	r.Use(proxy.HTTPMetricsMiddleware())

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

	// 优雅停机：r.Run 改为 http.Server + signal handling，K8s 滚动更新时等待 in-flight 请求完成。
	addr := bc.Server.HTTP.Addr
	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		log.Infof("gateway listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("failed to run gateway: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Infof("gateway shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("gateway forced shutdown: %v", err)
	}
	log.Infof("gateway exited")
}
