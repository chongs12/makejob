// Package executor 提供代码执行能力，通过 Piston API 运行用户代码。
package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultPistonEndpoint = "http://localhost:2000/api/v2/execute"

// PistonRequest 表示发送给 Piston API 的执行请求。
type PistonRequest struct {
	Language string         `json:"language"`
	Version  string         `json:"version"`
	Files    []PistonFile   `json:"files"`
	Stdin    string         `json:"stdin,omitempty"`
}

// PistonFile 表示提交给 Piston 的源代码文件。
type PistonFile struct {
	Name    string `json:"name,omitempty"`
	Content string `json:"content"`
}

// PistonResponse 表示 Piston API 返回的执行结果。
type PistonResponse struct {
	Run PistonRunResult `json:"run"`
}

// PistonRunResult 表示代码运行的输出。
type PistonRunResult struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Code   int    `json:"code"`
	Signal string `json:"signal"`
}

// CodeResult 表示经过处理后的代码执行结果。
type CodeResult struct {
	Output string `json:"output"`
	Passed bool   `json:"passed"`
}

// PistonClient 封装与 Piston API 的交互。
type PistonClient struct {
	endpoint   string
	httpClient *http.Client
}

// NewPistonClient 创建 Piston 客户端实例。
func NewPistonClient(endpoint string, timeoutSec int) *PistonClient {
	if endpoint == "" {
		endpoint = defaultPistonEndpoint
	}
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &PistonClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

// Execute 提交代码到 Piston 并返回执行结果。
func (c *PistonClient) Execute(ctx context.Context, language, code string) (*CodeResult, error) {
	return c.ExecuteWithInput(ctx, language, code, "")
}

// ExecuteWithInput 提交代码与标准输入到 Piston 并返回执行结果。
func (c *PistonClient) ExecuteWithInput(ctx context.Context, language, code string, stdin string) (*CodeResult, error) {
	langID, filename, err := resolveLanguage(language)
	if err != nil {
		return nil, err
	}

	reqBody := PistonRequest{
		Language: langID,
		Version:  "*",
		Files: []PistonFile{
			{Name: filename, Content: code},
		},
		Stdin: stdin,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal piston request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create piston request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("piston request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read piston response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("piston returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var pistonResp PistonResponse
	if err := json.Unmarshal(respBody, &pistonResp); err != nil {
		return nil, fmt.Errorf("unmarshal piston response: %w", err)
	}

	return buildCodeResult(pistonResp.Run), nil
}

// buildCodeResult 将 Piston 的原始输出转换为统一的 CodeResult。
func buildCodeResult(run PistonRunResult) *CodeResult {
	stdout := run.Stdout
	stderr := run.Stderr

	output := ""
	if stdout != "" {
		output = stdout
	}
	if stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += stderr
	}
	if output == "" {
		if run.Code == 0 {
			output = "程序执行完成，无输出"
		} else {
			output = fmt.Sprintf("进程退出，退出码: %d", run.Code)
		}
	}

	return &CodeResult{
		Output: output,
		Passed: run.Code == 0,
	}
}

// resolveLanguage 将前端语言标识映射为 Piston 语言标识和默认文件名。
func resolveLanguage(language string) (langID, filename string, err error) {
	switch language {
	case "go":
		return "go", "main.go", nil
	case "python":
		return "python", "main.py", nil
	case "javascript":
		return "javascript", "main.js", nil
	case "java":
		return "java", "Main.java", nil
	case "cpp", "c++":
		return "c++", "main.cpp", nil
	default:
		return "", "", fmt.Errorf("不支持的编程语言: %s", language)
	}
}
