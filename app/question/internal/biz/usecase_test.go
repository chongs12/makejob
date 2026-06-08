package biz

import (
	"context"
	"testing"
)

// questionRepoStub 为题目用例测试提供最小仓储桩。
type questionRepoStub struct {
	existsByTitle map[string]bool
	created       []*Question
	nextID        uint64
}

// List 返回空结果，满足接口要求。
func (r *questionRepoStub) List(context.Context, *QuestionFilter, int32, int32) ([]*Question, int64, error) {
	return nil, 0, nil
}

// GetByID 返回空结果，满足接口要求。
func (r *questionRepoStub) GetByID(context.Context, uint64) (*Question, error) {
	return nil, nil
}

// Create 记录写入的题目，并分配递增主键。
func (r *questionRepoStub) Create(_ context.Context, question *Question) error {
	r.nextID++
	question.ID = r.nextID
	clone := *question
	r.created = append(r.created, &clone)
	return nil
}

// Update 返回空结果，满足接口要求。
func (r *questionRepoStub) Update(context.Context, *Question) error {
	return nil
}

// Delete 返回空结果，满足接口要求。
func (r *questionRepoStub) Delete(context.Context, uint64) error {
	return nil
}

// Count 返回零值，满足接口要求。
func (r *questionRepoStub) Count(context.Context, *QuestionFilter) (int64, error) {
	return 0, nil
}

// RandomSelect 返回空结果，满足接口要求。
func (r *questionRepoStub) RandomSelect(context.Context, *QuestionFilter, int32) ([]*Question, error) {
	return nil, nil
}

// ExistsByTitleAndIndustry 按测试注入的去重表返回结果。
func (r *questionRepoStub) ExistsByTitleAndIndustry(_ context.Context, title, industryCode string) (bool, error) {
	if r.existsByTitle == nil {
		return false, nil
	}
	return r.existsByTitle[title+"|"+industryCode], nil
}

// ragSyncPublisherStub 记录题目变更事件的发布情况。
type ragSyncPublisherStub struct {
	published []ragSyncCall
}

// ragSyncCall 描述一次 RAG 事件发布调用。
type ragSyncCall struct {
	questionID uint64
	action     string
	content    string
	metadata   map[string]string
}

// PublishQuestionChanged 记录发布参数，供断言校验。
func (p *ragSyncPublisherStub) PublishQuestionChanged(_ context.Context, questionID uint64, action string, content string, metadata map[string]string) error {
	p.published = append(p.published, ragSyncCall{
		questionID: questionID,
		action:     action,
		content:    content,
		metadata:   metadata,
	})
	return nil
}

// generatorStub 返回预设题目结果，供流水线用例测试复用。
type generatorStub struct {
	questions []*Question
}

// GenerateQuestions 返回预设候选题。
func (g *generatorStub) GenerateQuestions(context.Context, *GenerateQuestionsRequest) ([]*Question, error) {
	return g.questions, nil
}

// TestImportQuestionsPublishesRAGSync 验证 scraper 导入成功写库后会发布 RAG create 事件。
func TestImportQuestionsPublishesRAGSync(t *testing.T) {
	repo := &questionRepoStub{nextID: 100}
	pub := &ragSyncPublisherStub{}
	uc := NewQuestionUseCase(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	uc.SetRAGSyncPublisher(pub)

	imported, err := uc.ImportQuestions(context.Background(), []*Question{
		{
			Title:        "Go GC",
			Content:      "解释 Go GC 的三色标记法",
			IndustryCode: "backend",
			Type:         "subjective",
			Difficulty:   "medium",
		},
	})
	if err != nil {
		t.Fatalf("ImportQuestions returned error: %v", err)
	}
	if imported != 1 {
		t.Fatalf("expected imported=1, got %d", imported)
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 RAG event, got %d", len(pub.published))
	}
	if pub.published[0].questionID == 0 {
		t.Fatalf("expected published question id to be assigned")
	}
	if pub.published[0].action != "create" {
		t.Fatalf("expected action=create, got %s", pub.published[0].action)
	}
	if pub.published[0].metadata["title"] != "Go GC" {
		t.Fatalf("expected title metadata to be preserved, got %+v", pub.published[0].metadata)
	}
}

// TestPipelineGenerateQuestionsPublishesRAGSync 验证流水线生成的题目写库后同样会发布 RAG create 事件。
func TestPipelineGenerateQuestionsPublishesRAGSync(t *testing.T) {
	repo := &questionRepoStub{nextID: 7}
	pub := &ragSyncPublisherStub{}
	generator := &generatorStub{
		questions: []*Question{
			{
				Title:        "并发调度",
				Content:      "说明 GMP 模型",
				IndustryCode: "backend",
				Type:         "subjective",
				Difficulty:   "hard",
			},
		},
	}
	uc := NewQuestionUseCase(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, generator)
	uc.SetRAGSyncPublisher(pub)

	created, err := uc.PipelineGenerateQuestions(context.Background(), &GenerateQuestionsRequest{
		IndustryCode:   "backend",
		Requirement:    "生成并发专题题目",
		CandidateCount: 1,
	}, nil)
	if err != nil {
		t.Fatalf("PipelineGenerateQuestions returned error: %v", err)
	}
	if created != 1 {
		t.Fatalf("expected created=1, got %d", created)
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 RAG event, got %d", len(pub.published))
	}
	if pub.published[0].content != "并发调度\n说明 GMP 模型" {
		t.Fatalf("unexpected published content: %s", pub.published[0].content)
	}
}
