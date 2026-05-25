// Package logger 提供基于Zap的结构化日志功能
// 支持不同日志级别、JSON/Console 格式、日志轮转和性能优化
package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	instance *zap.Logger
	once     sync.Once
)

// Config 日志配置
type Config struct {
	Level      string // debug, info, warn, error
	Mode       string // development, production
	Format     string // console, json（为空时根据 Mode 推断）
	OutputPath string // 日志输出路径，为空时输出到stdout

	// 日志轮转配置（仅 OutputPath 为文件路径时生效）
	MaxSizeMB  int // 单个文件最大 MB，默认 100
	MaxBackups int // 最多保留旧文件数，默认 5
	MaxDays    int // 最多保留天数，默认 30
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

	// 根据 Format 选择 encoder：production 默认 JSON，development 默认 Console
	useJSON := cfg.Format == "json" || (cfg.Format == "" && cfg.Mode == "production")
	var encoder zapcore.Encoder
	if useJSON {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 设置输出
	var writeSyncer zapcore.WriteSyncer
	if cfg.OutputPath != "" {
		writeSyncer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.OutputPath,
			MaxSize:    defaultInt(cfg.MaxSizeMB, 100),
			MaxBackups: defaultInt(cfg.MaxBackups, 5),
			MaxAge:     defaultInt(cfg.MaxDays, 30),
			Compress:   true,
		})
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

func defaultInt(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
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
