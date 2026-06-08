package service

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"
	adminv1 "makejob/api/makejob/admin/v1"
	"makejob/app/admin/internal/biz"
)

// asyncTaskRepoStub 为异步任务回写测试提供最小仓库桩。
type asyncTaskRepoStub struct {
	biz.AdminRepo
	task        *biz.ScraperTaskRecord
	updatedTask *biz.ScraperTaskRecord
}

// GetScraperTask 返回预置任务。
func (r *asyncTaskRepoStub) GetScraperTask(_ context.Context, _ uint64) (*biz.ScraperTaskRecord, error) {
	clone := *r.task
	return &clone, nil
}

// UpdateScraperTask 记录最新的任务状态。
func (r *asyncTaskRepoStub) UpdateScraperTask(_ context.Context, task *biz.ScraperTaskRecord) error {
	clone := *task
	r.updatedTask = &clone
	r.task = &clone
	return nil
}

// asyncTaskPublisherStub 记录 Admin 侧异步消息投递行为。
type asyncTaskPublisherStub struct {
	lastPipelineTaskID uint64
	lastScraperTaskID  uint64
	lastScraperPayload []byte
}

// PublishQuestionPipelineBuild 记录题目流水线重试投递。
func (p *asyncTaskPublisherStub) PublishQuestionPipelineBuild(_ context.Context, taskID uint64, _ *adminv1.GenerateQuestionPipelineRequest) error {
	p.lastPipelineTaskID = taskID
	return nil
}

// PublishScraperImport 记录 scraper 导入重试投递。
func (p *asyncTaskPublisherStub) PublishScraperImport(_ context.Context, taskID uint64, payload []byte) error {
	p.lastScraperTaskID = taskID
	p.lastScraperPayload = append([]byte(nil), payload...)
	return nil
}

// TestUpdateQuestionPipelineTaskAcceptsScraperImport 验证 question 服务可以回写 scraper 导入任务状态。
func TestUpdateQuestionPipelineTaskAcceptsScraperImport(t *testing.T) {
	repo := &asyncTaskRepoStub{
		task: &biz.ScraperTaskRecord{
			ID:       21,
			TaskType: "import_questions",
			Status:   "pending",
		},
	}
	svc := NewAdminService(biz.NewAdminUseCase(repo, nil, nil, nil, nil), &asyncTaskPublisherStub{})

	resp, err := svc.UpdateQuestionPipelineTask(context.Background(), &adminv1.UpdateQuestionPipelineTaskRequest{
		TaskId:        21,
		Status:        "running",
		QuestionCount: 3,
		ImportedCount: 1,
		StartedAt:     timestamppb.Now(),
	})
	if err != nil {
		t.Fatalf("UpdateQuestionPipelineTask returned error: %v", err)
	}
	if !resp.GetApplied() {
		t.Fatalf("expected task update to be applied")
	}
	if repo.updatedTask == nil {
		t.Fatalf("expected repo to persist updated task")
	}
	if repo.updatedTask.Status != "running" {
		t.Fatalf("expected status=running, got %s", repo.updatedTask.Status)
	}
	if repo.updatedTask.ImportedCount != 1 {
		t.Fatalf("expected imported_count=1, got %d", repo.updatedTask.ImportedCount)
	}
}

// TestRetryScraperTaskRequeuesScraperImport 验证失败的 scraper 导入任务会按原 payload 重新入队。
func TestRetryScraperTaskRequeuesScraperImport(t *testing.T) {
	repo := &asyncTaskRepoStub{
		task: &biz.ScraperTaskRecord{
			ID:          34,
			TaskType:    "import_questions",
			Status:      "failed",
			PayloadJSON: `{"task_id":34,"industry_code":"backend","questions":[{"title":"Go GC"}]}`,
		},
	}
	publisher := &asyncTaskPublisherStub{}
	svc := NewAdminService(biz.NewAdminUseCase(repo, nil, nil, nil, nil), publisher)

	resp, err := svc.RetryScraperTask(context.Background(), &adminv1.RetryScraperTaskRequest{Id: 34})
	if err != nil {
		t.Fatalf("RetryScraperTask returned error: %v", err)
	}
	if resp.GetStatus() != "pending" {
		t.Fatalf("expected status=pending, got %s", resp.GetStatus())
	}
	if publisher.lastScraperTaskID != 34 {
		t.Fatalf("expected scraper task 34 to be requeued, got %d", publisher.lastScraperTaskID)
	}
	if string(publisher.lastScraperPayload) != repo.task.PayloadJSON {
		t.Fatalf("expected payload to be replayed verbatim, got %s", string(publisher.lastScraperPayload))
	}
}
