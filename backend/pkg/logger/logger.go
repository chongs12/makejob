// Package logger 提供基于Zap的结构化日志功能
// 支持不同日志级别、日志轮转和性能优化
package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	instance *zap.Logger
	once     sync.Once
)

// Config 日志配置
type Config struct {
	Level      string // debug, info, warn, error
	Mode       string // development, production
	OutputPath string // 日志输出路径，为空时输出到stdout
}

// Init 初始化全局日志实例
func Init(cfg Config) error {
	var err error
	once.Do(func() {
		instance, err = New(cfg)
	})
	return err
}

// Get 获取全局日志实例
func Get() *zap.Logger {
	if instance == nil {
		// 如果未初始化，返回一个默认的development logger
		Init(Config{
			Level: "debug",
			Mode:  "development",
		})
	}
	return instance
}

// New 创建新的日志实例
func New(cfg Config) (*zap.Logger, error) {
	level := parseLevel(cfg.Level)

	var encoderConfig zapcore.EncoderConfig
	if cfg.Mode == "production" {
		encoderConfig = zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "timestamp"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}

	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	// 设置输出
	var writeSyncer zapcore.WriteSyncer
	if cfg.OutputPath != "" {
		file, err := os.OpenFile(cfg.OutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		writeSyncer = zapcore.AddSync(file)
	} else {
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	core := zapcore.NewCore(
		encoder,
		writeSyncer,
		level,
	)

	// 添加调用者信息
	options := []zap.Option{
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	}

	logger := zap.New(core, options...)
	return logger, nil
}

// parseLevel 解析日志级别字符串
func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// Debug 输出Debug级别日志
func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

// Info 输出Info级别日志
func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

// Warn 输出Warn级别日志
func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

// Error 输出Error级别日志
func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

// Fatal 输出Fatal级别日志并退出程序
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}

// With 创建带有附加字段的日志实例
func With(fields ...zap.Field) *zap.Logger {
	return Get().With(fields...)
}

// Sync 刷新日志缓冲区
func Sync() error {
	if instance != nil {
		return instance.Sync()
	}
	return nil
}
