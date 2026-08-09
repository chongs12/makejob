package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	coderunnerv1 "makejob/api/makejob/coderunner/v1"
	"makejob/app/interview/internal/biz"
	"makejob/app/interview/internal/conf"
	"makejob/pkg/middleware"
)

// codeRunnerClient 实现 biz.CodeRunnerClient 接口，通过 gRPC 调用代码执行服务
type codeRunnerClient struct {
	client    coderunnerv1.CodeRunnerServiceClient
	conn      *grpc.ClientConn
	timeoutMs int32
}

// NewCodeRunnerClient 创建代码执行服务客户端
func NewCodeRunnerClient(cfg *conf.CodeRunner) (biz.CodeRunnerClient, error) {
	conn, err := grpc.NewClient(cfg.ServiceAddr, middleware.CommonDialOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CodeRunner service at %s: %w", cfg.ServiceAddr, err)
	}
	timeoutMs := int32(cfg.TimeoutMs)
	if timeoutMs <= 0 {
		timeoutMs = 10000
	}
	return &codeRunnerClient{
		client:    coderunnerv1.NewCodeRunnerServiceClient(conn),
		conn:      conn,
		timeoutMs: timeoutMs,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *codeRunnerClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Execute 提交代码并执行测试用例
func (c *codeRunnerClient) Execute(ctx context.Context, language, code string, testCases []biz.CodeTestCase) (*biz.CodeRunnerResult, error) {
	// 转换测试用例格式
	tc := make([]*coderunnerv1.TestCase, len(testCases))
	for i, t := range testCases {
		tc[i] = &coderunnerv1.TestCase{
			Input:          t.Input,
			ExpectedOutput: t.ExpectedOutput,
		}
	}

	req := &coderunnerv1.ExecuteRequest{
		Language:  language,
		Code:      code,
		TestCases: tc,
		TimeoutMs: c.timeoutMs,
	}

	resp, err := c.client.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("CodeRunner gRPC call failed: %w", err)
	}

	return &biz.CodeRunnerResult{
		Success:         resp.Success,
		Stdout:          resp.Stdout,
		Stderr:          resp.Stderr,
		PassedCount:     resp.PassedCount,
		TotalCount:      resp.TotalCount,
		ExecutionTimeMs: resp.ExecutionTimeMs,
	}, nil
}
