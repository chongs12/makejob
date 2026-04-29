package service

import (
	"context"
	"strings"
	"testing"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/scraper"
)

type scraperTaskRepositoryStub struct {
	task        *model.ScraperTask
	list        []model.ScraperTask
	total       int64
	createdTask *model.ScraperTask
	lastFilter  scraper.TaskListFilter
}

// Create 模拟创建任务记录，并保留最近一次入队任务，便于其他服务测试校验任务初始化结果。
func (s *scraperTaskRepositoryStub) Create(_ context.Context, task *model.ScraperTask) error {
	if task != nil {
		clone := *task
		s.createdTask = &clone
	}
	return nil
}

// Update 模拟更新任务记录，并将最新状态回写到内存任务对象。
func (s *scraperTaskRepositoryStub) Update(_ context.Context, task *model.ScraperTask) error {
	if s.task != nil && task != nil {
		*s.task = *task
	}
	return nil
}

// List 模拟任务列表查询，并记录最后一次筛选条件，便于验证服务层是否正确透传过滤参数。
func (s *scraperTaskRepositoryStub) List(_ context.Context, page, pageSize int, filter scraper.TaskListFilter) ([]model.ScraperTask, int64, error) {
	s.lastFilter = filter
	return s.list, s.total, nil
}

// GetByID 模拟按 ID 读取任务详情。
func (s *scraperTaskRepositoryStub) GetByID(_ context.Context, _ uint) (*model.ScraperTask, error) {
	return s.task, nil
}

// ClaimNextPending 模拟领取任务，本测试不覆盖 worker 轮询逻辑。
func (s *scraperTaskRepositoryStub) ClaimNextPending(_ context.Context, _ string) (*model.ScraperTask, error) {
	return nil, nil
}

type scraperIndustryRepositoryStub struct {
	industry *model.Industry
}

// List 模拟行业列表查询，本测试场景不依赖列表结果。
func (s *scraperIndustryRepositoryStub) List(_ context.Context) ([]model.Industry, error) {
	return nil, nil
}

// GetByID 模拟按 ID 读取行业，本测试场景不依赖该能力。
func (s *scraperIndustryRepositoryStub) GetByID(_ context.Context, _ uint) (*model.Industry, error) {
	return nil, nil
}

// Create 模拟创建行业，本测试场景不依赖该能力。
func (s *scraperIndustryRepositoryStub) Create(_ context.Context, _ *model.Industry) error {
	return nil
}

// Update 模拟更新行业，本测试场景不依赖该能力。
func (s *scraperIndustryRepositoryStub) Update(_ context.Context, _ *model.Industry) error {
	return nil
}

// GetByCode 模拟按编码读取行业，用于校验异步导入任务入队前的行业存在性。
func (s *scraperIndustryRepositoryStub) GetByCode(_ context.Context, _ string) (*model.Industry, error) {
	return s.industry, nil
}

// TestBuildScraperImportTaskPayloadRequiresQuestions 验证异步导入任务在入队前必须至少携带一题。
func TestBuildScraperImportTaskPayloadRequiresQuestions(t *testing.T) {
	t.Parallel()

	_, err := buildScraperImportTaskPayload(scraper.ImportRequest{
		IndustryCode: "go",
	})
	if err == nil || !strings.Contains(err.Error(), "至少需要一题") {
		t.Fatalf("expected question validation error, got %v", err)
	}
}

// TestResolveScraperTaskSourceByURL 验证任务来源会根据 URL 域名自动归一化，便于后续后台聚合查看。
func TestResolveScraperTaskSourceByURL(t *testing.T) {
	t.Parallel()

	if source := resolveScraperTaskSource("https://www.nowcoder.com/discuss/123"); source != scraper.SourceNiuke {
		t.Fatalf("expected niuke source, got %s", source)
	}
	if source := resolveScraperTaskSource("https://example.com/article"); source != "manual" {
		t.Fatalf("expected manual fallback source, got %s", source)
	}
}

// TestRetryTaskResetsFailedImportTask 验证失败的异步导入任务重试时会被重置回 pending，并清空上次执行残留。
func TestRetryTaskResetsFailedImportTask(t *testing.T) {
	t.Parallel()

	task := &model.ScraperTask{
		BaseModel:     model.BaseModel{ID: 9},
		TaskType:      scraper.TaskTypeImportQuestions,
		Status:        scraper.TaskStatusFailed,
		ImportedCount: 3,
		ResultJSON:    `{"success_count":3}`,
		ErrorMsg:      "mock failure",
	}
	svc := &scraperService{
		scraperRepo: &scraperTaskRepositoryStub{task: task},
	}

	retried, err := svc.RetryTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("RetryTask returned error: %v", err)
	}
	if retried.Status != scraper.TaskStatusPending {
		t.Fatalf("expected pending status, got %#v", retried)
	}
	if retried.ErrorMsg != "" || retried.ResultJSON != "" || retried.ImportedCount != 0 {
		t.Fatalf("expected retry state to be cleared, got %#v", retried)
	}
}

// TestCreateImportTaskRequiresExistingIndustry 验证异步导入任务在入队前会校验行业存在，避免无效任务进入队列。
func TestCreateImportTaskRequiresExistingIndustry(t *testing.T) {
	t.Parallel()

	svc := &scraperService{
		scraperRepo:  &scraperTaskRepositoryStub{},
		industryRepo: &scraperIndustryRepositoryStub{},
	}

	_, err := svc.CreateImportTask(context.Background(), scraper.ImportRequest{
		IndustryCode: "missing",
		Questions: []scraper.CleanedQuestion{
			{Title: "goroutine 调度", Content: "解释 GMP 调度模型"},
		},
	})
	if err == nil {
		t.Fatal("expected CreateImportTask to fail when industry is missing")
	}
	businessErr, ok := err.(*common.BusinessError)
	if !ok || businessErr.Code != common.CodeNotFound {
		t.Fatalf("expected not found business error, got %T %v", err, err)
	}
}

// TestListTasksTrimsFilter 验证任务列表筛选条件会在服务层被裁剪后再传给仓储层，避免前端空格造成筛选失效。
func TestListTasksTrimsFilter(t *testing.T) {
	t.Parallel()

	repo := &scraperTaskRepositoryStub{
		list: []model.ScraperTask{
			{BaseModel: model.BaseModel{ID: 1}, Status: scraper.TaskStatusFailed},
		},
		total: 1,
	}
	svc := &scraperService{scraperRepo: repo}

	result, err := svc.ListTasks(context.Background(), 1, 10, scraper.TaskListFilter{
		Status:   " failed ",
		TaskType: " import_questions ",
	})
	if err != nil {
		t.Fatalf("ListTasks returned error: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected total 1, got %d", result.Total)
	}
	if repo.lastFilter.Status != scraper.TaskStatusFailed || repo.lastFilter.TaskType != scraper.TaskTypeImportQuestions {
		t.Fatalf("expected trimmed filter, got %#v", repo.lastFilter)
	}
}

// TestGetTaskReturnsDetail 验证任务详情会显式带出异步任务载荷和结果，便于后台任务页直接排查。
func TestGetTaskReturnsDetail(t *testing.T) {
	t.Parallel()

	task := &model.ScraperTask{
		BaseModel:   model.BaseModel{ID: 12},
		TaskType:    scraper.TaskTypeImportQuestions,
		Status:      scraper.TaskStatusImported,
		SourceURL:   "manual://question-import",
		Source:      "manual",
		PayloadJSON: `{"industry_code":"go","questions":[{"title":"goroutine"}]}`,
		ResultJSON:  `{"total_count":1,"success_count":1}`,
	}
	svc := &scraperService{
		scraperRepo: &scraperTaskRepositoryStub{task: task},
	}

	detail, err := svc.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask returned error: %v", err)
	}
	if detail == nil {
		t.Fatal("expected task detail, got nil")
	}
	if detail.PayloadJSON != task.PayloadJSON || detail.ResultJSON != task.ResultJSON {
		t.Fatalf("expected payload and result to be exposed, got %#v", detail)
	}
	if detail.ID != task.ID || detail.TaskType != task.TaskType {
		t.Fatalf("expected basic fields to be preserved, got %#v", detail)
	}
}

// TestRetryTaskAllowsQuestionPipelineBuild 验证题目流水线生成任务失败后也允许直接重试，便于后台任务台统一重投。
func TestRetryTaskAllowsQuestionPipelineBuild(t *testing.T) {
	t.Parallel()

	task := &model.ScraperTask{
		BaseModel: model.BaseModel{ID: 21},
		TaskType:  scraper.TaskTypeQuestionPipelineBuild,
		Status:    scraper.TaskStatusFailed,
		ErrorMsg:  "mock failure",
	}
	svc := &scraperService{
		scraperRepo: &scraperTaskRepositoryStub{task: task},
	}

	retried, err := svc.RetryTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("RetryTask returned error: %v", err)
	}
	if retried.Status != scraper.TaskStatusPending {
		t.Fatalf("expected pending status, got %#v", retried)
	}
}
