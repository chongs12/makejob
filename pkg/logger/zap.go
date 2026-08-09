package logger

import (
	"os"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewZapLogger 创建 Zap 日志器并适配 Kratos log 接口。
//
// serviceName 注入为每条日志的 service 字段（如 "makejob.interview"），
// 同时包装 tracing.TraceID()/SpanID() 两个 log.Valuer：当通过 log.Context(ctx)
// 输出日志时，Valuer 会从 ctx 中的 OTel span 求值出 trace_id/span_id。
//
// 前提（由 main.go 保证）：
//  1. 调用 log.SetLogger(NewZapLogger(serviceName)) 设为全局 logger；
//  2. otelgrpc 拦截器在 Logging 之前执行（span 已注入 ctx）；
//  3. 日志走 log.Context(ctx).Infof(...) / .Errorw(...)，而非无 ctx 的 log.Infof(...)。
//
// 无 span 时（如启动期日志），TraceID()/SpanID() 的 HasTraceID() 守卫返回空串，
// 不会输出 32 位全零串（避免手动 trace.SpanContextFromContext().TraceID() 的污染问题）。
//
// 环境变量：
//   - LOG_LEVEL: debug, info, warn, error
//   - LOG_OUTPUT: stdout, stderr
func NewZapLogger(serviceName string) log.Logger {
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
	base := &kratosZapLogger{logger: zl}

	// log.With 把 service / trace_id / span_id 作为 prefix 注入每条日志；
	// trace_id/span_id 是 Valuer，会在 log.Context(ctx) 求值时按当前 ctx 的 span 计算。
	return log.With(base,
		"service", serviceName,
		"trace_id", tracing.TraceID(),
		"span_id", tracing.SpanID(),
	)
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
