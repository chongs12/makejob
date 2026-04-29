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
	TaskID   *uint  `json:"task_id,omitempty"`
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
		TaskID:   req.TaskID,
	}
	logs, total, err := s.aiCallLogRepo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	if params.Page <= 0 {
		params.Page = 1
	}
	pageParam := common.PageParam{Page: params.Page, PageSize: params.PageSize}
	pageParam.Normalize()

	return common.NewPageResult(logs, total, pageParam), nil
}

// GetAICallLog 返回单条 AI 调用日志详情，供任务页展开查看 prompt、消息和模型原始输出。
func (s *adminService) GetAICallLog(ctx context.Context, id uint) (*model.AICallLog, error) {
	if id == 0 {
		return nil, common.NewBusinessError(common.CodeBadRequest, "ai call log id is required")
	}
	if s.aiCallLogRepo == nil {
		return nil, common.NewBusinessError(common.CodeInternalError, "ai call log repository is unavailable")
	}

	log, err := s.aiCallLogRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if log == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "ai call log not found")
	}

	return log, nil
}

// recordAICallLog 将调试结果写入 AI 调用日志表。
func (s *adminService) recordAICallLog(ctx context.Context, req *AIDebugRequest, resp *AIDebugResponse) {
	if s.aiCallLogRepo == nil || req == nil || resp == nil {
		return
	}

	log := &model.AICallLog{
		TraceID:            strings.TrimSpace(resp.TraceID),
		TaskID:             req.TaskID,
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

// fillAIDebugResponseModelOutput 尝试按 trace_id 回填调试结果中的模型原始输出，避免流式告警缺失 raw_output。
func (s *adminService) fillAIDebugResponseModelOutput(ctx context.Context, resp *AIDebugResponse) {
	if s.aiCallLogRepo == nil || resp == nil {
		return
	}
	if strings.TrimSpace(resp.ModelOutput) != "" || strings.TrimSpace(resp.TraceID) == "" {
		return
	}

	log, err := s.aiCallLogRepo.GetLatestByTraceID(ctx, resp.TraceID)
	if err != nil || log == nil {
		return
	}
	resp.ModelOutput = strings.TrimSpace(log.ModelOutput)
	if strings.TrimSpace(resp.ModelError) == "" {
		resp.ModelError = strings.TrimSpace(log.ModelError)
	}
	if strings.TrimSpace(resp.Provider) == "" {
		resp.Provider = strings.TrimSpace(log.Provider)
	}
	if strings.TrimSpace(resp.Model) == "" {
		resp.Model = strings.TrimSpace(log.Model)
	}
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
