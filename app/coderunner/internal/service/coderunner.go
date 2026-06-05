package service

import (
	"context"

	coderunnerv1 "makejob/api/makejob/coderunner/v1"
	"makejob/app/coderunner/internal/biz"
)

// CodeRunnerService 代码运行 gRPC 服务实现
type CodeRunnerService struct {
	coderunnerv1.UnimplementedCodeRunnerServiceServer
	uc *biz.CodeRunnerUseCase
}

// NewCodeRunnerService 创建代码运行 gRPC 服务
func NewCodeRunnerService(uc *biz.CodeRunnerUseCase) *CodeRunnerService {
	return &CodeRunnerService{uc: uc}
}

// Execute 执行代码，支持单次执行和批量测试用例
func (s *CodeRunnerService) Execute(ctx context.Context, req *coderunnerv1.ExecuteRequest) (*coderunnerv1.ExecuteResponse, error) {
	// 转换 proto 测试用例为 biz 结构
	testCases := make([]biz.TestCaseInput, 0, len(req.TestCases))
	for _, tc := range req.TestCases {
		testCases = append(testCases, biz.TestCaseInput{
			Input:          tc.Input,
			ExpectedOutput: tc.ExpectedOutput,
		})
	}

	input := &biz.ExecuteInput{
		Language:  req.Language,
		Code:      req.Code,
		Stdin:     req.Stdin,
		TestCases: testCases,
		TimeoutMs: req.TimeoutMs,
	}

	output, err := s.uc.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	return toProtoExecuteResponse(output), nil
}

// ListLanguages 查询支持的编程语言列表
func (s *CodeRunnerService) ListLanguages(ctx context.Context, req *coderunnerv1.ListLanguagesRequest) (*coderunnerv1.ListLanguagesResponse, error) {
	return &coderunnerv1.ListLanguagesResponse{
		Languages: []*coderunnerv1.LanguageInfo{
			{Name: "go", Version: "1.21"},
			{Name: "python", Version: "3.11"},
			{Name: "javascript", Version: "18"},
			{Name: "java", Version: "17"},
			{Name: "cpp", Version: "17"},
		},
	}, nil
}

// toProtoExecuteResponse 将 biz 执行结果转换为 proto 响应
func toProtoExecuteResponse(output *biz.ExecuteOutput) *coderunnerv1.ExecuteResponse {
	testResults := make([]*coderunnerv1.TestResult, 0, len(output.TestResults))
	for _, tr := range output.TestResults {
		testResults = append(testResults, &coderunnerv1.TestResult{
			Input:          tr.Input,
			ExpectedOutput: tr.Expected,
			ActualOutput:   tr.Actual,
			Passed:         tr.Passed,
		})
	}

	return &coderunnerv1.ExecuteResponse{
		Success:         output.Success,
		Stdout:          output.Stdout,
		Stderr:          output.Stderr,
		ExitCode:        int32(output.ExitCode),
		ExecutionTimeMs: output.ExecutionTimeMs,
		TestResults:     testResults,
		PassedCount:     output.PassedCount,
		TotalCount:      output.TotalCount,
		// FIX: 填充proto新增的error字段
		Error:           output.Error,
	}
}
