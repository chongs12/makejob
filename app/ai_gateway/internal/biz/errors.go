package biz

import (
	"github.com/go-kratos/kratos/v2/errors"
)

// ErrAIConfigNotFound AI 配置未找到
var ErrAIConfigNotFound = errors.NotFound("AI_CONFIG_NOT_FOUND", "AI配置未找到")

// ErrLLMCallFailed 大模型调用失败
var ErrLLMCallFailed = errors.ServiceUnavailable("LLM_CALL_FAILED", "大模型调用失败")

// ErrPromptRenderFailed Prompt 渲染失败
var ErrPromptRenderFailed = errors.InternalServer("PROMPT_RENDER_FAILED", "Prompt渲染失败")

// ErrParseFailed 结果解析失败
var ErrParseFailed = errors.InternalServer("PARSE_FAILED", "结果解析失败")
