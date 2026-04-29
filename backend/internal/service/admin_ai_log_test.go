package service

import (
	"context"
	"testing"

	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

// aiCallLogRepositoryStub 模拟 AI 调用日志仓库，供服务层校验筛选参数与写库行为。
type aiCallLogRepositoryStub struct {
	listParams repository.AICallLogListParams
	createdLog *model.AICallLog
	logByID    *model.AICallLog
}

// Create 记录最近一次写入的 AI 调用日志，便于断言任务关联字段是否落库。
func (s *aiCallLogRepositoryStub) Create(_ context.Context, log *model.AICallLog) error {
	if log == nil {
		s.createdLog = nil
		return nil
	}
	copyLog := *log
	s.createdLog = &copyLog
	return nil
}

// List 记录最近一次查询参数，便于断言 task_id 等筛选条件是否正确透传。
func (s *aiCallLogRepositoryStub) List(_ context.Context, params repository.AICallLogListParams) ([]model.AICallLog, int64, error) {
	s.listParams = params
	return []model.AICallLog{}, 0, nil
}

// GetByID 模拟按主键读取 AI 调用日志详情，供服务层校验详情查询链路。
func (s *aiCallLogRepositoryStub) GetByID(_ context.Context, _ uint) (*model.AICallLog, error) {
	return s.logByID, nil
}

// GetLatestByTraceID 模拟按 trace_id 回查日志，本测试场景不依赖该能力。
func (s *aiCallLogRepositoryStub) GetLatestByTraceID(_ context.Context, _ string) (*model.AICallLog, error) {
	return nil, nil
}

// TestResolveAIDebugTaskIDPrefersRequestValue 验证 AI 调试任务 ID 会优先采用显式请求值，而不是上下文中的旧值。
func TestResolveAIDebugTaskIDPrefersRequestValue(t *testing.T) {
	t.Parallel()

	ctx := withAsyncTaskID(context.Background(), 42)
	requestTaskID := uint(7)
	resolved := resolveAIDebugTaskID(ctx, &requestTaskID)
	if resolved == nil || *resolved != 7 {
		t.Fatalf("expected request task id to win, got %#v", resolved)
	}
}

// TestListAICallLogsForwardsTaskIDFilter 验证 AI 日志列表查询会把 task_id 筛选条件透传到仓库层。
func TestListAICallLogsForwardsTaskIDFilter(t *testing.T) {
	t.Parallel()

	repo := &aiCallLogRepositoryStub{}
	svc := &adminService{aiCallLogRepo: repo}
	taskID := uint(99)

	_, err := svc.ListAICallLogs(context.Background(), &ListAICallLogsRequest{
		Page:   1,
		TaskID: &taskID,
	})
	if err != nil {
		t.Fatalf("ListAICallLogs returned error: %v", err)
	}
	if repo.listParams.TaskID == nil || *repo.listParams.TaskID != taskID {
		t.Fatalf("expected task_id filter to be forwarded, got %#v", repo.listParams.TaskID)
	}
}

// TestGetAICallLogReturnsDetail 验证 AI 日志详情查询会返回仓库中的完整日志对象。
func TestGetAICallLogReturnsDetail(t *testing.T) {
	t.Parallel()

	repo := &aiCallLogRepositoryStub{
		logByID: &model.AICallLog{
			BaseModel:      model.BaseModel{ID: 9},
			TraceID:        "trace-detail",
			RenderedPrompt: "prompt body",
			ModelOutput:    "model output",
		},
	}
	svc := &adminService{aiCallLogRepo: repo}

	log, err := svc.GetAICallLog(context.Background(), 9)
	if err != nil {
		t.Fatalf("GetAICallLog returned error: %v", err)
	}
	if log == nil || log.ID != 9 || log.TraceID != "trace-detail" {
		t.Fatalf("expected ai call log detail, got %#v", log)
	}
}

// TestRecordAICallLogPersistsTaskID 验证写入 AI 调用日志时会保留关联异步任务 ID，便于任务页反查模型调用。
func TestRecordAICallLogPersistsTaskID(t *testing.T) {
	t.Parallel()

	repo := &aiCallLogRepositoryStub{}
	svc := &adminService{aiCallLogRepo: repo}
	taskID := uint(123)

	svc.recordAICallLog(context.Background(), &AIDebugRequest{
		TaskID:    &taskID,
		Scene:     model.PromptSceneInterview,
		UserInput: "生成一张题卡",
	}, &AIDebugResponse{
		TraceID:        "trace-1",
		Scene:          model.PromptSceneInterview,
		PromptSource:   "template_custom",
		RenderedPrompt: "prompt",
		Provider:       "openai",
		Model:          "gpt-test",
		ModelOutput:    "ok",
	})
	if repo.createdLog == nil {
		t.Fatal("expected ai call log to be persisted")
	}
	if repo.createdLog.TaskID == nil || *repo.createdLog.TaskID != taskID {
		t.Fatalf("expected persisted task_id %d, got %#v", taskID, repo.createdLog.TaskID)
	}
}
