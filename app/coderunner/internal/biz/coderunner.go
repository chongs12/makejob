package biz

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
)

// PistonExecutor 代码执行引擎接口，data 层必须实现
type PistonExecutor interface {
	Execute(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error)
}

// TestCaseInput 单条测试用例输入
type TestCaseInput struct {
	Input          string
	ExpectedOutput string
}

// ExecuteInput 代码执行输入参数
type ExecuteInput struct {
	Language   string
	Code       string
	Stdin      string
	TestCases  []TestCaseInput
	TimeoutMs  int32
}

// TestResultOutput 单条测试用例执行结果
type TestResultOutput struct {
	Input    string
	Expected string
	Actual   string
	Passed   bool
}

// ExecuteOutput 代码执行输出结果
type ExecuteOutput struct {
	Success          bool
	Stdout           string
	Stderr           string
	Error            string
	ExitCode         int
	ExecutionTimeMs  int64
	TestResults      []TestResultOutput
	PassedCount      int32
	TotalCount       int32
}

// CodeRunnerUseCase 代码运行业务用例
type CodeRunnerUseCase struct {
	executor PistonExecutor
	logger   log.Logger
}

// NewCodeRunnerUseCase 创建代码运行业务用例
func NewCodeRunnerUseCase(executor PistonExecutor, logger log.Logger) *CodeRunnerUseCase {
	return &CodeRunnerUseCase{
		executor: executor,
		logger:   logger,
	}
}

// Execute 执行代码：若 test_cases 为空则直接执行，否则逐个执行并对比结果
func (uc *CodeRunnerUseCase) Execute(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error) {
	if len(input.TestCases) == 0 {
		return uc.executor.Execute(ctx, input)
	}
	return uc.executeWithTestCases(ctx, input)
}

// executeWithTestCases 逐个执行测试用例并汇总结果
func (uc *CodeRunnerUseCase) executeWithTestCases(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error) {
	var testResults []TestResultOutput
	var passedCount int32
	var totalTimeMs int64

	for _, tc := range input.TestCases {
		singleInput := &ExecuteInput{
			Language:  input.Language,
			Code:      input.Code,
			Stdin:     tc.Input,
			TimeoutMs: input.TimeoutMs,
		}

		output, err := uc.executor.Execute(ctx, singleInput)
		if err != nil {
			return nil, err
		}

		// 清理实际输出末尾换行符后与期望输出对比
		actual := strings.TrimRight(output.Stdout, "\n\r")
		expected := strings.TrimRight(tc.ExpectedOutput, "\n\r")
		passed := actual == expected

		if passed {
			passedCount++
		}

		totalTimeMs += output.ExecutionTimeMs

		testResults = append(testResults, TestResultOutput{
			Input:    tc.Input,
			Expected: tc.ExpectedOutput,
			Actual:   output.Stdout,
			Passed:   passed,
		})
	}

	totalCount := int32(len(input.TestCases))

	return &ExecuteOutput{
		Success:         passedCount == totalCount,
		ExitCode:        0,
		ExecutionTimeMs: totalTimeMs,
		TestResults:     testResults,
		PassedCount:     passedCount,
		TotalCount:      totalCount,
	}, nil
}
