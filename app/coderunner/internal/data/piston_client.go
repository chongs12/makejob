package data

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	"makejob/app/coderunner/internal/biz"
)

// 语言版本映射表
var languageVersions = map[string]string{
	"go":         "1.21.0",
	"python":     "3.11.0",
	"javascript": "18.15.0",
	"java":       "17.0.0",
	"cpp":        "17.0.0",
}

// pistonExecuteRequest Piston API 请求体
type pistonExecuteRequest struct {
	Language string   `json:"language"`
	Version  string   `json:"version"`
	Files    []pistonFile  `json:"files"`
	Stdin    string   `json:"stdin,omitempty"`
	Timeout  int      `json:"timeout,omitempty"`
}

// pistonFile Piston 文件结构
type pistonFile struct {
	Name    string `json:"name,omitempty"`
	Content string `json:"content"`
}

// pistonExecuteResponse Piston API 响应体
type pistonExecuteResponse struct {
	Run pistonRunResult `json:"run"`
}

// pistonRunResult Piston 运行结果
type pistonRunResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Code     int    `json:"code"`
	Signal   string `json:"signal"`
	Output   string `json:"output"`
}

// pistonClient Piston HTTP 客户端，实现 biz.PistonExecutor 接口
type pistonClient struct {
	httpClient *http.Client
	endpoint   string
	logger     log.Logger
}

// NewPistonClient 创建 Piston HTTP 客户端
func NewPistonClient(endpoint string, timeoutMs int, logger log.Logger) biz.PistonExecutor {
	return &pistonClient{
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutMs) * time.Millisecond,
		},
		endpoint: strings.TrimRight(endpoint, "/"),
		logger:   logger,
	}
}

// Execute 调用 Piston API 执行代码
func (c *pistonClient) Execute(ctx context.Context, input *biz.ExecuteInput) (*biz.ExecuteOutput, error) {
	// 检查语言支持
	_, ok := languageVersions[input.Language]
	if !ok {
		return nil, biz.ErrUnsupportedLanguage
	}

	// 构造请求体（version 使用 "*" 通配，让 Piston 使用已安装的版本）
	reqBody := pistonExecuteRequest{
		Language: input.Language,
		Version:  "*",
		Files: []pistonFile{
			{Content: input.Code},
		},
		Stdin: input.Stdin,
	}

	if input.TimeoutMs > 0 {
		reqBody.Timeout = int(input.TimeoutMs)
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.InternalServer("PISTON_REQUEST_BUILD_FAILED", "序列化请求失败")
	}

	// 发起 HTTP 请求
	url := c.endpoint + "/api/v2/execute"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.InternalServer("PISTON_REQUEST_BUILD_FAILED", "创建请求失败")
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	executionTimeMs := time.Since(start).Milliseconds()
	if err != nil {
		return nil, biz.ErrPistonUnavailable
	}
	defer resp.Body.Close()

	// 读取响应
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, biz.ErrPistonUnavailable
	}

	if resp.StatusCode != http.StatusOK {
		_ = respBytes
		return nil, biz.ErrPistonUnavailable
	}

	// 解析响应
	var pistonResp pistonExecuteResponse
	if err := json.Unmarshal(respBytes, &pistonResp); err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.InternalServer("PISTON_RESPONSE_PARSE_FAILED", "解析响应失败")
	}

	run := pistonResp.Run

	// SIGKILL 表示执行超时
	if run.Signal == "SIGKILL" {
		return nil, biz.ErrExecutionTimeout
	}

	success := run.Code == 0 && run.Signal == ""

	return &biz.ExecuteOutput{
		Success:         success,
		Stdout:          run.Stdout,
		Stderr:          run.Stderr,
		ExitCode:        run.Code,
		ExecutionTimeMs: executionTimeMs,
	}, nil
}
