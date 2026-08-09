package data

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	coderunnerv1 "makejob/api/makejob/coderunner/v1"
	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/conf"
	"makejob/pkg/middleware"
)

// codeRunnerClient 实现 biz.CodeRunnerClient 接口
// 通过 gRPC 调用代码运行服务
type codeRunnerClient struct {
	client coderunnerv1.CodeRunnerServiceClient
	conn   *grpc.ClientConn
}

// NewCodeRunnerClient 创建代码运行服务客户端
func NewCodeRunnerClient(cfg *conf.AI) (biz.CodeRunnerClient, error) {
	conn, err := grpc.NewClient(cfg.CodeRunnerAddr, middleware.CommonDialOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CodeRunner service at %s: %w", cfg.CodeRunnerAddr, err)
	}
	return &codeRunnerClient{
		client: coderunnerv1.NewCodeRunnerServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close 关闭 gRPC 连接
func (c *codeRunnerClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Execute 调用代码运行服务执行代码
func (c *codeRunnerClient) Execute(ctx context.Context, req *biz.CodeRunnerRequest) (*biz.CodeRunnerResponse, error) {
	testCases := make([]*coderunnerv1.TestCase, len(req.TestCases))
	for i, tc := range req.TestCases {
		testCases[i] = &coderunnerv1.TestCase{
			Input:          tc.Input,
			ExpectedOutput: tc.ExpectedOutput,
		}
	}

	resp, err := c.client.Execute(ctx, &coderunnerv1.ExecuteRequest{
		Language:  req.Language,
		Code:      req.Code,
		TestCases: testCases,
		TimeoutMs: req.TimeoutMs,
	})
	if err != nil {
		return nil, fmt.Errorf("CodeRunner gRPC call failed: %w", err)
	}

	return &biz.CodeRunnerResponse{
		Success:         resp.Success,
		Output:          resp.Stdout,
		Error:           resp.Error,
		TestCasesPassed: resp.PassedCount,
		TotalTestCases:  resp.TotalCount,
		ExecutionTimeMs: resp.ExecutionTimeMs,
		TestResults:     toBizTestResults(resp.TestResults),
	}, nil
}

// toBizTestResults 将 proto 测试结果转换为 biz 层结构。
func toBizTestResults(results []*coderunnerv1.TestResult) []biz.CodeTestResult {
	bizResults := make([]biz.CodeTestResult, 0, len(results))
	for _, r := range results {
		bizResults = append(bizResults, biz.CodeTestResult{
			Input:          r.Input,
			ExpectedOutput: r.ExpectedOutput,
			ActualOutput:   r.ActualOutput,
			Passed:         r.Passed,
		})
	}
	return bizResults
}
