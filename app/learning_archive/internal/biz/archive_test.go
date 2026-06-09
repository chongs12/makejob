package biz

import (
	"context"
	"testing"
	"time"
)

// archiveRepoKey 是测试仓储的幂等索引键。
type archiveRepoKey struct {
	userID      uint64
	interviewID uint64
	sourceType  string
	sourceRef   string
}

// archiveRepoStub 提供学习档案用例测试所需的最小仓储行为。
type archiveRepoStub struct {
	existingBySource map[archiveRepoKey]*ArchiveEntry
	createCalls      int
	batchCreateCalls int
	hasMarker        bool
	batchEntries     []*ArchiveEntry
}

// Create 记录单条创建调用，并回填伪造主键。
func (r *archiveRepoStub) Create(_ context.Context, entry *ArchiveEntry) error {
	r.createCalls++
	entry.ID = 99
	entry.CreatedAt = time.Unix(1700000000, 0)
	return nil
}

// BatchCreate 记录批量写入内容。
func (r *archiveRepoStub) BatchCreate(_ context.Context, entries []*ArchiveEntry) (int, error) {
	r.batchCreateCalls++
	r.batchEntries = append([]*ArchiveEntry(nil), entries...)
	return len(entries), nil
}

// ListByUser 返回空列表，当前测试不依赖该查询。
func (r *archiveRepoStub) ListByUser(context.Context, uint64, int32) ([]*ArchiveEntry, error) {
	return nil, nil
}

// GetWeakTopics 返回空结果，当前测试不依赖该查询。
func (r *archiveRepoStub) GetWeakTopics(context.Context, uint64) ([]string, error) {
	return nil, nil
}

// GetFocusSignals 返回空结果，当前测试不依赖该查询。
func (r *archiveRepoStub) GetFocusSignals(context.Context, uint64) ([]*FocusSignal, error) {
	return nil, nil
}

// Transaction 直接在测试上下文中执行闭包。
func (r *archiveRepoStub) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

// GetBySource 按幂等键返回预置条目。
func (r *archiveRepoStub) GetBySource(_ context.Context, userID, interviewID uint64, sourceType, sourceRef string) (*ArchiveEntry, error) {
	if r.existingBySource == nil {
		return nil, nil
	}
	return r.existingBySource[archiveRepoKey{
		userID:      userID,
		interviewID: interviewID,
		sourceType:  sourceType,
		sourceRef:   sourceRef,
	}], nil
}

// HasInterviewFinishedMarker 返回预置的处理标记状态。
func (r *archiveRepoStub) HasInterviewFinishedMarker(context.Context, uint64, uint64) (bool, error) {
	return r.hasMarker, nil
}

// archivePublisherStub 记录 archive.written 事件的发布次数。
type archivePublisherStub struct {
	publishCalls int
}

// PublishArchiveWritten 记录发布调用。
func (p *archivePublisherStub) PublishArchiveWritten(context.Context, uint64, string, uint64, []string, []string) error {
	p.publishCalls++
	return nil
}

// TestArchiveUseCaseWriteEntryReturnsExistingRecord 验证幂等命中时返回已有记录而非空成功。
func TestArchiveUseCaseWriteEntryReturnsExistingRecord(t *testing.T) {
	existing := &ArchiveEntry{
		ID:          7,
		UserID:      8,
		SourceType:  "interview_coding",
		SourceRef:   "42",
		InterviewID: 42,
		OccurredAt:  time.Unix(1700000100, 0),
		CreatedAt:   time.Unix(1700000200, 0),
	}
	repo := &archiveRepoStub{
		existingBySource: map[archiveRepoKey]*ArchiveEntry{
			{userID: 8, interviewID: 42, sourceType: "interview_coding", sourceRef: "42"}: existing,
		},
	}
	uc := NewArchiveUseCase(repo, nil)
	entry := &ArchiveEntry{
		UserID:      8,
		SourceType:  "interview_coding",
		InterviewID: 42,
	}

	if err := uc.WriteEntry(context.Background(), entry); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("expected duplicate write to skip create, got %d create calls", repo.createCalls)
	}
	if entry.ID != existing.ID || !entry.CreatedAt.Equal(existing.CreatedAt) || entry.SourceRef != "42" {
		t.Fatalf("expected existing record to be returned, got %#v", entry)
	}
}

// TestArchiveUseCaseHandleInterviewFinishedWithoutTopics 验证空 topic 事件只写处理标记且不发布事件。
func TestArchiveUseCaseHandleInterviewFinishedWithoutTopics(t *testing.T) {
	repo := &archiveRepoStub{}
	publisher := &archivePublisherStub{}
	uc := NewArchiveUseCase(repo, publisher)

	if err := uc.HandleInterviewFinished(context.Background(), 21, 9, 88, nil, nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.batchCreateCalls != 1 {
		t.Fatalf("expected one batch create, got %d", repo.batchCreateCalls)
	}
	if len(repo.batchEntries) != 1 {
		t.Fatalf("expected only marker entry, got %d entries", len(repo.batchEntries))
	}
	marker := repo.batchEntries[0]
	if marker.SourceType != ArchiveSourceTypeInterviewFinishedMarker || marker.SourceRef != archiveSourceRefProcessed {
		t.Fatalf("unexpected marker entry: %#v", marker)
	}
	if publisher.publishCalls != 0 {
		t.Fatalf("expected no archive.written publish, got %d", publisher.publishCalls)
	}
}
