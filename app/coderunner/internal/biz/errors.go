package biz

import (
	"github.com/go-kratos/kratos/v2/errors"
)

// ErrUnsupportedLanguage 不支持的编程语言
var ErrUnsupportedLanguage = errors.BadRequest("UNSUPPORTED_LANGUAGE", "不支持的编程语言")

// ErrExecutionTimeout 代码执行超时
var ErrExecutionTimeout = errors.GatewayTimeout("EXECUTION_TIMEOUT", "代码执行超时")

// ErrPistonUnavailable 代码执行引擎不可用
var ErrPistonUnavailable = errors.ServiceUnavailable("PISTON_UNAVAILABLE", "代码执行引擎不可用")
