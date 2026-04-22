package service

import (
	"context"
	"strings"

	aiRuntime "makejob-backend/internal/ai/runtime"
	"makejob-backend/internal/common"
)

// AIDebugRequest 复用 runtime 层的调试请求结构。
type AIDebugRequest = aiRuntime.DebugRequest

// AIDebugResponse 复用 runtime 层的调试响应结构。
type AIDebugResponse = aiRuntime.DebugResponse

// DebugAIRuntime 执行一次后台 AI 调试请求。
func (s *adminService) DebugAIRuntime(ctx context.Context, req *AIDebugRequest) (*AIDebugResponse, error) {
	if req == nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "debug request cannot be empty")
	}
	if strings.TrimSpace(req.Scene) == "" {
		return nil, common.NewBusinessError(common.CodeBadRequest, "scene is required")
	}

	debugger := aiRuntime.NewDebugger(
		s.adminConfigRepo,
		s.promptRepo,
		s.industryRepo,
		s.baseAIConfig,
	)
	result, err := debugger.Run(ctx, *req)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, err.Error())
	}

	s.recordAICallLog(ctx, req, result)
	s.fillAIDebugResponseModelOutput(ctx, result)
	return result, nil
}
