package data

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	kratoserr "github.com/go-kratos/kratos/v2/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"makejob/app/ai_gateway/internal/biz"
	"makejob/app/ai_gateway/internal/conf"
)

const defaultLLMRequestTimeout = 120 * time.Second

// arkLLMClient 封装 ARK Chat Completions 调用。
type arkLLMClient struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewArkLLMClient 创建 ARK LLM 客户端。
func NewArkLLMClient(cfg *conf.ARK) biz.LLMClient {
	return &arkLLMClient{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		client: &http.Client{
			Timeout: defaultLLMRequestTimeout,
		},
	}
}

// chatRequest 表示 ARK Chat Completions 请求体。
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Thinking    *chatThinking `json:"thinking,omitempty"`
}

// chatThinking 表示 ARK 深度思考开关，type 取值 enabled/disabled/auto。
type chatThinking struct {
	Type string `json:"type"`
}

// chatMessage 表示 ARK Chat Completions 单条消息。
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse 表示 ARK Chat Completions 响应体。
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat 调用 ARK Chat Completions，并按统一错误语义返回结果。
func (c *arkLLMClient) Chat(ctx context.Context, messages []biz.Message, config *biz.AIConfig) (resp *biz.LLMResponse, err error) {
	// ark.chat span：记录 LLM 调用（provider/model/thinking/usage），用入参 ctx 创建以继承上游 RPC trace。
	ctx, span := otel.Tracer("makejob.ai").Start(ctx, "ark.chat",
		trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	if err = ctx.Err(); err != nil {
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", llmCallErrorMessage(err, 0))
	}

	model := c.model
	apiKey := c.apiKey
	baseURL := strings.TrimRight(c.baseURL, "/")
	if config != nil && config.Model != "" {
		model = config.Model
	}
	span.SetAttributes(
		attribute.String("llm.provider", "volcengine_ark"),
		attribute.String("llm.model", model),
		attribute.Int("llm.message_count", len(messages)),
	)
	if thinking := resolveThinkingOverride(config); thinking != nil {
		span.SetAttributes(attribute.String("llm.thinking_mode", thinking.Type))
	}
	if override := resolveAIConfigRuntimeValue(config, "ai_api_key"); override != "" {
		apiKey = override
	}
	if override := resolveAIConfigRuntimeValue(config, "ai_base_url"); override != "" {
		baseURL = strings.TrimRight(override, "/")
	}

	reqMessages := make([]chatMessage, 0, len(messages))
	for _, m := range messages {
		reqMessages = append(reqMessages, chatMessage{Role: m.Role, Content: m.Content})
	}

	reqBody := chatRequest{Model: model, Messages: reqMessages}
	reqBody.Thinking = resolveThinkingOverride(config)
	if config != nil {
		if config.Temperature > 0 {
			temperature := config.Temperature
			reqBody.Temperature = &temperature
		}
		if config.MaxTokens > 0 {
			maxTokens := config.MaxTokens
			reqBody.MaxTokens = &maxTokens
		}
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, kratoserr.InternalServer("LLM_REQUEST_BUILD_FAILED", "序列化 LLM 请求失败")
	}

	effectiveTimeout := effectiveLLMRequestTimeout(c.client.Timeout, config)
	requestCtx, cancel := buildLLMRequestContext(ctx, effectiveTimeout)
	defer cancel()

	url := baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, kratoserr.InternalServer("LLM_REQUEST_BUILD_FAILED", "创建 LLM 请求失败")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	httpResp, err := c.client.Do(req)
	if err != nil {
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", llmCallErrorMessage(err, effectiveTimeout))
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", "读取 LLM 响应失败")
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", "调用模型 API 返回非 200 状态")
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, kratoserr.InternalServer("LLM_RESPONSE_PARSE_FAILED", "解析 LLM 响应失败")
	}

	if chatResp.Error != nil {
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", "调用模型 API 返回错误")
	}

	if len(chatResp.Choices) == 0 {
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", "调用模型 API 未返回内容")
	}

	// 推理模型（如 DeepSeek-R1/V4）可能把输出放在 reasoning_content 而 content 为空：优先 content，空则回退 reasoning_content。
	content := chatResp.Choices[0].Message.Content
	if strings.TrimSpace(content) == "" {
		content = chatResp.Choices[0].Message.ReasoningContent
	}
	if strings.TrimSpace(content) == "" {
		finishReason := chatResp.Choices[0].FinishReason
		// finish_reason=stop 且 0 token：常见于消息序列以 assistant 结尾时推理模型认为"该轮已结束、无需生成"；
		// finish_reason=length：max_tokens 不足，推理阶段就把预算耗尽、content 未产出。
		// 一并带出，便于定位是消息结尾角色问题还是 max_tokens 配置问题。
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", fmt.Sprintf("模型返回空内容（content 与 reasoning_content 均为空，finish_reason=%s），请检查消息结尾角色/模型配置/max_tokens", finishReason))
	}

	resp = &biz.LLMResponse{
		Content:      content,
		InputTokens:  chatResp.Usage.PromptTokens,
		OutputTokens: chatResp.Usage.CompletionTokens,
	}
	span.SetAttributes(
		attribute.Int("gen_ai.usage.prompt_tokens", resp.InputTokens),
		attribute.Int("gen_ai.usage.completion_tokens", resp.OutputTokens),
		attribute.Int("gen_ai.usage.total_tokens", resp.InputTokens+resp.OutputTokens),
	)
	return resp, nil
}

// resolveThinkingOverride 将抽象的 ai_thinking_mode 配置翻译为 ARK thinking 字段：
// disabled/enabled 时显式下发，default 或未知取值时不发任何字段（跟随厂商默认，对不支持的模型零风险）。
func resolveThinkingOverride(config *biz.AIConfig) *chatThinking {
	switch strings.ToLower(resolveAIConfigRuntimeValue(config, "ai_thinking_mode")) {
	case "disabled":
		return &chatThinking{Type: "disabled"}
	case "enabled":
		return &chatThinking{Type: "enabled"}
	default:
		return nil
	}
}

// resolveAIConfigRuntimeValue 从 ExtraParamsJSON 提取运行时配置。
func resolveAIConfigRuntimeValue(config *biz.AIConfig, key string) string {
	if config == nil || strings.TrimSpace(config.ExtraParamsJSON) == "" || strings.TrimSpace(key) == "" {
		return ""
	}
	values := make(map[string]string)
	if err := json.Unmarshal([]byte(config.ExtraParamsJSON), &values); err != nil {
		return ""
	}
	return strings.TrimSpace(values[key])
}

// buildLLMRequestContext 为模型调用创建独立超时上下文，避免继承上游隐式 deadline。
func buildLLMRequestContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), timeout)
}

// effectiveLLMRequestTimeout 解析模型调用超时，优先使用后台 AI 配置。
func effectiveLLMRequestTimeout(fallback time.Duration, config *biz.AIConfig) time.Duration {
	if timeout := parseLLMRequestTimeout(config); timeout > 0 {
		return timeout
	}
	if fallback > 0 {
		return fallback
	}
	return defaultLLMRequestTimeout
}

// parseLLMRequestTimeout 读取 ai_timeout_seconds 并转换为 time.Duration。
func parseLLMRequestTimeout(config *biz.AIConfig) time.Duration {
	rawTimeout := strings.TrimSpace(resolveAIConfigRuntimeValue(config, "ai_timeout_seconds"))
	if rawTimeout == "" {
		return 0
	}
	seconds, err := strconv.Atoi(rawTimeout)
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// llmCallErrorMessage 将底层 context/HTTP 错误收敛为稳定的用户可读文案。
func llmCallErrorMessage(err error, timeout time.Duration) string {
	switch {
	case stderrors.Is(err, context.DeadlineExceeded):
		if timeout > 0 {
			return fmt.Sprintf("模型调用超时（%d秒）", int(timeout/time.Second))
		}
		return "模型调用超时"
	case stderrors.Is(err, context.Canceled):
		return "模型调用已取消"
	default:
		return "调用模型 API 失败"
	}
}
