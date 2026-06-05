package data

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-kratos/kratos/v2/errors"

	"makejob/app/ai_gateway/internal/biz"
	"makejob/app/ai_gateway/internal/conf"
)

// arkLLMClient 基于火山引擎 ARK API 的 LLM 客户端实现
type arkLLMClient struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewArkLLMClient 创建 ARK LLM 客户端
func NewArkLLMClient(cfg *conf.ARK) biz.LLMClient {
	return &arkLLMClient{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// chatRequest ARK Chat Completions 请求体
type chatRequest struct {
	Model    string          `json:"model"`
	Messages []chatMessage   `json:"messages"`
}

// chatMessage 请求中的消息项
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse ARK Chat Completions 响应体
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat 发送对话请求到 ARK API 并返回响应
func (c *arkLLMClient) Chat(ctx context.Context, messages []biz.Message, config *biz.AIConfig) (*biz.LLMResponse, error) {
	model := c.model
	if config != nil && config.Model != "" {
		model = config.Model
	}

	reqMessages := make([]chatMessage, 0, len(messages))
	for _, m := range messages {
		reqMessages = append(reqMessages, chatMessage{Role: m.Role, Content: m.Content})
	}

	reqBody := chatRequest{Model: model, Messages: reqMessages}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.InternalServer("LLM_REQUEST_BUILD_FAILED", "序列化LLM请求失败")
	}

	url := c.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.InternalServer("LLM_REQUEST_BUILD_FAILED", "创建LLM请求失败")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.ServiceUnavailable("LLM_CALL_FAILED", "大模型调用失败")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.ServiceUnavailable("LLM_CALL_FAILED", "读取LLM响应失败")
	}

	if resp.StatusCode != http.StatusOK {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.ServiceUnavailable("LLM_CALL_FAILED", "大模型API返回非200状态")
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.InternalServer("LLM_RESPONSE_PARSE_FAILED", "解析LLM响应失败")
	}

	if chatResp.Error != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.ServiceUnavailable("LLM_CALL_FAILED", "大模型API返回错误")
	}

	if len(chatResp.Choices) == 0 {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.ServiceUnavailable("LLM_CALL_FAILED", "大模型API返回空结果")
	}

	return &biz.LLMResponse{
		Content:      chatResp.Choices[0].Message.Content,
		InputTokens:  chatResp.Usage.PromptTokens,
		OutputTokens: chatResp.Usage.CompletionTokens,
	}, nil
}
