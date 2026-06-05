package data

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"makejob/app/admin/internal/biz"
	"makejob/app/admin/internal/data/model"
)

// TestListAICallLogsAppliesLegacyFilters 验证 AI 日志列表会透传单体已有的状态、trace_id 与 task_id 筛选条件。
func TestListAICallLogsAppliesLegacyFilters(t *testing.T) {
	db := newAICallLogTestDB(t)
	repo := NewAdminRepo(db).(*adminRepo)

	taskID := uint(42)
	otherTaskID := uint(7)
	seeded := []model.AICallLog{
		{
			TraceID:    "trace-hit",
			TaskID:     &taskID,
			Source:     "admin_debug",
			Scene:      "quiz",
			ModelName:  "model-a",
			Status:     "success",
			AgentType:  "quiz",
			LatencyMs:  10,
			TokensUsed: 12,
		},
		{
			TraceID:    "trace-miss",
			TaskID:     &otherTaskID,
			Source:     "admin_debug",
			Scene:      "quiz",
			ModelName:  "model-b",
			Status:     "failed",
			AgentType:  "quiz",
			LatencyMs:  20,
			TokensUsed: 24,
		},
	}
	for i := range seeded {
		seeded[i].CreatedAt = time.Now().Add(time.Duration(i) * time.Minute)
		if err := db.Create(&seeded[i]).Error; err != nil {
			t.Fatalf("failed to seed ai call log: %v", err)
		}
	}

	logs, total, err := repo.ListAICallLogs(context.Background(), biz.AICallLogListFilter{
		Page:     1,
		PageSize: 10,
		Status:   "success",
		TraceID:  "trace-hit",
		TaskID:   &taskID,
	})
	if err != nil {
		t.Fatalf("ListAICallLogs returned error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Model != "model-a" {
		t.Fatalf("expected filtered log model model-a, got %q", logs[0].Model)
	}
}

// newAICallLogTestDB 创建 AI 日志仓储测试使用的最小 SQLite 数据库。
func newAICallLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(&model.AICallLog{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}
	return db
}
