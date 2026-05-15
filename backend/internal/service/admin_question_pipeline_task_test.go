package service

import (
	"context"
	"testing"

	"makejob-backend/internal/model"
)

// pipelineCategoryRepositoryStub 模拟题目流水线依赖的分类仓库，仅提供载荷入队前的行业分类校验能力。
type pipelineCategoryRepositoryStub struct {
	categories []model.Category
}

// List 模拟读取分类列表，供题目流水线任务创建时校验目标行业下是否已有分类。
func (s *pipelineCategoryRepositoryStub) List(_ context.Context) ([]model.Category, error) {
	return append([]model.Category(nil), s.categories...), nil
}

// GetByID 模拟按 ID 读取分类，当前测试场景不依赖该能力。
func (s *pipelineCategoryRepositoryStub) GetByID(_ context.Context, _ uint) (*model.Category, error) {
	return nil, nil
}

// Create 模拟创建分类，当前测试场景不依赖该能力。
func (s *pipelineCategoryRepositoryStub) Create(_ context.Context, _ *model.Category) error {
	return nil
}

// Update 模拟更新分类，当前测试场景不依赖该能力。
func (s *pipelineCategoryRepositoryStub) Update(_ context.Context, _ *model.Category) error {
	return nil
}

// Delete 模拟删除分类，当前测试场景不依赖该能力。
func (s *pipelineCategoryRepositoryStub) Delete(_ context.Context, _ uint) error {
	return nil
}

// TestBuildQuestionPipelineTaskPayloadNormalizesDefaults 验证异步题目流水线任务入队前会补齐默认逐张模式、数量和抓取开关。
func TestBuildQuestionPipelineTaskPayloadNormalizesDefaults(t *testing.T) {
	t.Parallel()

	payloadJSON, normalized, err := buildQuestionPipelineTaskPayload(&AdminQuestionPipelineGenerateRequest{
		IndustryCode:   " go ",
		Requirement:    " 生成 Go 并发题卡 ",
		GenerationMode: "unexpected",
		CandidateCount: 999,
		Sources:        []string{" niuke ", "", "niuke", "leetcode"},
	})
	if err != nil {
		t.Fatalf("buildQuestionPipelineTaskPayload returned error: %v", err)
	}
	if payloadJSON == "" || normalized == nil {
		t.Fatal("expected payload and normalized request to be returned")
	}
	if normalized.IndustryCode != "go" || normalized.Requirement != "生成 Go 并发题卡" {
		t.Fatalf("expected trimmed request, got %#v", normalized)
	}
	if normalized.GenerationMode != questionPipelineModeDirect {
		t.Fatalf("expected direct mode fallback, got %s", normalized.GenerationMode)
	}
	if normalized.CandidateCount != maxQuestionPipelineCount {
		t.Fatalf("expected normalized candidate count %d, got %d", maxQuestionPipelineCount, normalized.CandidateCount)
	}
	if !normalized.IncludeScraped || !normalized.IncludeGenerated {
		t.Fatalf("expected both include flags to default to true, got %#v", normalized)
	}
	if len(normalized.Sources) != 2 {
		t.Fatalf("expected deduped sources, got %#v", normalized.Sources)
	}
}

// TestCreateQuestionPipelineTaskCreatesPendingTask 验证题目流水线请求入队后会创建 pending 任务，并记录预期候选数量。
func TestCreateQuestionPipelineTaskCreatesPendingTask(t *testing.T) {
	t.Parallel()

	taskRepo := &scraperTaskRepositoryStub{}
	svc := &adminService{
		industryRepo: &scraperIndustryRepositoryStub{
			industry: &model.Industry{
				BaseModel: model.BaseModel{ID: 7},
				Code:      "go",
			},
		},
		adminCategoryRepo: &pipelineCategoryRepositoryStub{
			categories: []model.Category{
				{BaseModel: model.BaseModel{ID: 3}, IndustryID: 7, Name: "Go 并发"},
			},
		},
		scraperTaskRepo: taskRepo,
	}

	task, err := svc.CreateQuestionPipelineTask(context.Background(), &AdminQuestionPipelineGenerateRequest{
		IndustryCode: "go",
		Requirement:  "生成 Go 并发和内存模型面试题",
		Sources:      []string{"niuke"},
	})
	if err != nil {
		t.Fatalf("CreateQuestionPipelineTask returned error: %v", err)
	}
	if task == nil || taskRepo.createdTask == nil {
		t.Fatal("expected pending task to be created")
	}
	if task.TaskType != "question_pipeline_build" || task.Status != "pending" {
		t.Fatalf("expected pending question pipeline task, got %#v", task)
	}
	if task.QuestionCount != defaultQuestionPipelineCount {
		t.Fatalf("expected default candidate count %d, got %d", defaultQuestionPipelineCount, task.QuestionCount)
	}
	if taskRepo.createdTask.PayloadJSON == "" {
		t.Fatal("expected payload json to be persisted")
	}
}
