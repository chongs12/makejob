package data

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
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

// openaiLLMClient 封装 OpenAI 兼容接口的 Chat Completions 调用。
// 请求格式与 arkLLMClient 一致，区别在于配置来源和默认超时。
type openaiLLMClient struct {
	apiKey  string
	baseURL string
	model   string
	timeout time.Duration
	client  *http.Client
}

// NewOpenAILLMClient 创建 OpenAI 兼容 LLM 客户端。
func NewOpenAILLMClient(cfg *conf.Fallback) biz.LLMClient {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &openaiLLMClient{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Chat 调用 OpenAI 兼容 Chat Completions，支持从 ExtraParamsJSON 读取运行时覆盖。
func (c *openaiLLMClient) Chat(ctx context.Context, messages []biz.Message, config *biz.AIConfig) (resp *biz.LLMResponse, err error) {
	// openai.chat span：备用 LLM 调用，用入参 ctx 创建以继承上游 trace。
	ctx, span := otel.Tracer("makejob.ai").Start(ctx, "openai.chat",
		trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	if err = ctx.Err(); err != nil {
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", llmFallbackErrorMessage(err, 0))
	}

	apiKey := c.apiKey
	baseURL := strings.TrimRight(c.baseURL, "/")
	model := c.model

	if override := resolveAIConfigRuntimeValue(config, "ai_api_key"); override != "" {
		apiKey = override
	}
	if override := resolveAIConfigRuntimeValue(config, "ai_base_url"); override != "" {
		baseURL = strings.TrimRight(override, "/")
	}
	if config != nil && config.Model != "" {
		model = config.Model
	}
	span.SetAttributes(
		attribute.String("llm.provider", "openai_compatible"),
		attribute.String("llm.model", model),
		attribute.Int("llm.message_count", len(messages)),
		attribute.Bool("llm.is_fallback", true),
	)

	reqMessages := make([]chatMessage, 0, len(messages))
	for _, m := range messages {
		reqMessages = append(reqMessages, chatMessage{Role: m.Role, Content: m.Content})
	}

	reqBody := chatRequest{Model: model, Messages: reqMessages}
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

	effectiveTimeout := c.timeout
	if configTimeout := effectiveLLMRequestTimeout(0, config); configTimeout > 0 {
		effectiveTimeout = configTimeout
	}
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
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", llmFallbackErrorMessage(err, effectiveTimeout))
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", "读取 LLM 响应失败")
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", "调用备用模型 API 返回非 200 状态")
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, kratoserr.InternalServer("LLM_RESPONSE_PARSE_FAILED", "解析 LLM 响应失败")
	}

	if chatResp.Error != nil {
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", "调用备用模型 API 返回错误")
	}

	if len(chatResp.Choices) == 0 {
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", "调用备用模型 API 未返回内容")
	}

	// 推理模型可能把输出放在 reasoning_content 而 content 为空：优先 content，空则回退 reasoning_content。
	content := chatResp.Choices[0].Message.Content
	if strings.TrimSpace(content) == "" {
		content = chatResp.Choices[0].Message.ReasoningContent
	}
	if strings.TrimSpace(content) == "" {
		return nil, kratoserr.ServiceUnavailable("LLM_CALL_FAILED", "备用模型返回空内容（content 与 reasoning_content 均为空），请检查模型配置/额度/max_tokens")
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

// llmFallbackErrorMessage 将底层错误收敛为备用模型专用的用户可读文案。
func llmFallbackErrorMessage(err error, timeout time.Duration) string {
	switch {
	case stderrors.Is(err, context.DeadlineExceeded):
		if timeout > 0 {
			return fmt.Sprintf("备用模型调用超时（%d秒）", int(timeout/time.Second))
		}
		return "备用模型调用超时"
	case stderrors.Is(err, context.Canceled):
		return "备用模型调用已取消"
	default:
		return "调用备用模型 API 失败"
	}
}
