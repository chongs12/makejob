package service

import (
	"context"
	"encoding/json"
	"strings"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

// ListAICallLogsRequest AI 调用日志分页查询请求。
type ListAICallLogsRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Scene    string `json:"scene"`
	Source   string `json:"source"`
	Status   string `json:"status"`
	TraceID  string `json:"trace_id"`
}

// ListAICallLogs 查询 AI 调用日志列表。
func (s *adminService) ListAICallLogs(ctx context.Context, req *ListAICallLogsRequest) (*common.PageResult, error) {
	if req == nil {
		req = &ListAICallLogsRequest{}
	}
	if s.aiCallLogRepo == nil {
		return nil, common.NewBusinessError(common.CodeInternalError, "ai call log repository is unavailable")
	}

	params := repository.AICallLogListParams{
		Page:     req.Page,
		PageSize: req.PageSize,
		Scene:    strings.TrimSpace(req.Scene),
		Source:   strings.TrimSpace(req.Source),
		Status:   strings.TrimSpace(req.Status),
		TraceID:  strings.TrimSpace(req.TraceID),
	}
	logs, total, err := s.aiCallLogRepo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}

	return &common.PageResult{
		List:     logs,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

// recordAICallLog 将调试结果写入 AI 调用日志表。
func (s *adminService) recordAICallLog(ctx context.Context, req *AIDebugRequest, resp *AIDebugResponse) {
	if s.aiCallLogRepo == nil || req == nil || resp == nil {
		return
	}

	log := &model.AICallLog{
		TraceID:            strings.TrimSpace(resp.TraceID),
		Source:             model.AICallSourceAdminDebug,
		Scene:              strings.TrimSpace(resp.Scene),
		IndustryID:         req.IndustryID,
		PromptSource:       strings.TrimSpace(resp.PromptSource),
		SelectedPromptID:   resp.SelectedPromptID,
		SelectedPromptName: strings.TrimSpace(resp.SelectedPrompt),
		RenderedPrompt:     strings.TrimSpace(resp.RenderedPrompt),
		RequestMessages:    marshalLogJSON(resp.RequestMessages),
		RuntimeConfig:      marshalLogJSON(resp.RuntimeConfig),
		SceneConfig:        marshalLogJSON(resp.SceneConfig),
		Provider:           strings.TrimSpace(resp.Provider),
		Model:              strings.TrimSpace(resp.Model),
		UserInput:          strings.TrimSpace(req.UserInput),
		ModelOutput:        strings.TrimSpace(resp.ModelOutput),
		ModelError:         strings.TrimSpace(resp.ModelError),
		LatencyMS:          resp.LatencyMS,
		IsSuccess:          strings.TrimSpace(resp.ModelError) == "",
	}

	_ = s.aiCallLogRepo.Create(ctx, log)
}

// marshalLogJSON 将调试结构安全编码为 JSON 字符串。
func marshalLogJSON(value interface{}) string {
	if value == nil {
		return ""
	}

	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}
