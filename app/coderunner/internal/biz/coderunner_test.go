package biz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

// mockPistonExecutor 模拟 Piston 执行器
type mockPistonExecutor struct {
	executeFunc func(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error)
}

func (m *mockPistonExecutor) Execute(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error) {
	return m.executeFunc(ctx, input)
}

func TestCodeRunnerUseCase_Execute_NoTestCases(t *testing.T) {
	mock := &mockPistonExecutor{
		executeFunc: func(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error) {
			return &ExecuteOutput{
				Success:         true,
				Stdout:          "hello\n",
				ExitCode:        0,
				ExecutionTimeMs: 100,
			}, nil
		},
	}
	uc := NewCodeRunnerUseCase(mock, log.DefaultLogger)

	output, err := uc.Execute(context.Background(), &ExecuteInput{
		Language: "go",
		Code:     `package main; import "fmt"; func main() { fmt.Println("hello") }`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Error("expected success=true")
	}
	if output.Stdout != "hello\n" {
		t.Errorf("expected stdout='hello\\n', got '%s'", output.Stdout)
	}
}

func TestCodeRunnerUseCase_Execute_WithTestCases_AllPass(t *testing.T) {
	mock := &mockPistonExecutor{
		executeFunc: func(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error) {
			return &ExecuteOutput{
				Success:         true,
				Stdout:          input.Stdin,
				ExitCode:        0,
				ExecutionTimeMs: 50,
			}, nil
		},
	}
	uc := NewCodeRunnerUseCase(mock, log.DefaultLogger)

	output, err := uc.Execute(context.Background(), &ExecuteInput{
		Language: "go",
		Code:     "echo code",
		TestCases: []TestCaseInput{
			{Input: "3", ExpectedOutput: "3"},
			{Input: "5", ExpectedOutput: "5"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Success {
		t.Error("expected success=true (all tests passed)")
	}
	if output.PassedCount != 2 {
		t.Errorf("expected passed_count=2, got %d", output.PassedCount)
	}
	if output.TotalCount != 2 {
		t.Errorf("expected total_count=2, got %d", output.TotalCount)
	}
}

func TestCodeRunnerUseCase_Execute_WithTestCases_PartialFail(t *testing.T) {
	callCount := 0
	mock := &mockPistonExecutor{
		executeFunc: func(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error) {
			callCount++
			// 第一次返回正确结果，第二次返回错误结果
			if callCount == 1 {
				return &ExecuteOutput{Success: true, Stdout: "3", ExitCode: 0}, nil
			}
			return &ExecuteOutput{Success: true, Stdout: "wrong", ExitCode: 0}, nil
		},
	}
	uc := NewCodeRunnerUseCase(mock, log.DefaultLogger)

	output, err := uc.Execute(context.Background(), &ExecuteInput{
		Language: "python",
		Code:     "echo code",
		TestCases: []TestCaseInput{
			{Input: "3", ExpectedOutput: "3"},
			{Input: "5", ExpectedOutput: "5"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Success {
		t.Error("expected success=false (partial fail)")
	}
	if output.PassedCount != 1 {
		t.Errorf("expected passed_count=1, got %d", output.PassedCount)
	}
}

func TestCodeRunnerUseCase_Execute_ExecutorError(t *testing.T) {
	mock := &mockPistonExecutor{
		executeFunc: func(ctx context.Context, input *ExecuteInput) (*ExecuteOutput, error) {
			return nil, ErrPistonUnavailable
		},
	}
	uc := NewCodeRunnerUseCase(mock, log.DefaultLogger)

	_, err := uc.Execute(context.Background(), &ExecuteInput{
		Language: "go",
		Code:     "code",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrPistonUnavailable {
		t.Errorf("expected ErrPistonUnavailable, got %v", err)
	}
}
