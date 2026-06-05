package logger

import (
	"os"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewZapLogger 创建 Zap 日志器并适配 Kratos log 接口
// 通过环境变量 LOG_LEVEL 控制日志级别: debug, info, warn, error
// 通过环境变量 LOG_OUTPUT 控制输出目标: stdout, stderr
func NewZapLogger() log.Logger {
	// 解析日志级别
	level := zapcore.InfoLevel
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		switch strings.ToLower(lvl) {
		case "debug":
			level = zapcore.DebugLevel
		case "warn":
			level = zapcore.WarnLevel
		case "error":
			level = zapcore.ErrorLevel
		}
	}

	// 解析输出目标
	output := os.Stdout
	if out := os.Getenv("LOG_OUTPUT"); strings.ToLower(out) == "stderr" {
		output = os.Stderr
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(output),
		level,
	)

	zl := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2))
	return &kratosZapLogger{logger: zl}
}

// kratosZapLogger 适配 Kratos log.Logger 接口
type kratosZapLogger struct {
	logger *zap.Logger
}

func (l *kratosZapLogger) Log(level log.Level, keyvals ...interface{}) error {
	var zapLevel zapcore.Level
	switch level {
	case log.LevelDebug:
		zapLevel = zapcore.DebugLevel
	case log.LevelInfo:
		zapLevel = zapcore.InfoLevel
	case log.LevelWarn:
		zapLevel = zapcore.WarnLevel
	case log.LevelError:
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	fields := make([]zap.Field, 0, len(keyvals)/2)
	for i := 0; i < len(keyvals)-1; i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, zap.Any(key, keyvals[i+1]))
	}

	l.logger.Log(zapLevel, "", fields...)
	return nil
}
