package runtime

import (
	"context"
	"testing"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

// TestRuntimeManagerRebuildsClientWhenConfigChanges 验证运行时配置变更后会重建并切换到新的客户端。
func TestRuntimeManagerRebuildsClientWhenConfigChanges(t *testing.T) {
	t.Parallel()

	configRepo := &runtimeAdminConfigRepoStub{
		items: []model.AdminConfig{
			{ConfigKey: ai.ConfigKeyModel, ConfigValue: "model-a"},
		},
	}
	manager := NewRuntimeManager(NewBuilder(configRepo, nil, nil, nil, ai.DefaultRuntimeConfig()))

	firstModel := manager.CurrentClient(context.Background()).GetModelName()
	if firstModel != "model-a" {
		t.Fatalf("expected first model model-a, got %q", firstModel)
	}

	configRepo.items = []model.AdminConfig{
		{ConfigKey: ai.ConfigKeyModel, ConfigValue: "model-b"},
	}
	secondModel := manager.CurrentClient(context.Background()).GetModelName()
	if secondModel != "model-b" {
		t.Fatalf("expected second model model-b, got %q", secondModel)
	}
}

// TestDynamicInterviewAgentKeepsSessionAcrossConfigSwitch 验证切换配置后已创建的面试会话仍可继续推进。
func TestDynamicInterviewAgentKeepsSessionAcrossConfigSwitch(t *testing.T) {
	t.Parallel()

	configRepo := &runtimeAdminConfigRepoStub{
		items: []model.AdminConfig{
			{ConfigKey: ai.ConfigKeyModel, ConfigValue: "model-a"},
		},
	}
	client := NewRuntimeManager(NewBuilder(configRepo, nil, nil, nil, ai.DefaultRuntimeConfig())).BuildDynamicClient()

	sessionID, _, err := client.InterviewAgent.StartInterview(context.Background(), ai.InterviewConfig{
		IndustryCode:  "go",
		Difficulty:    "mixed",
		QuestionCount: 3,
	})
	if err != nil {
		t.Fatalf("StartInterview returned error: %v", err)
	}

	configRepo.items = []model.AdminConfig{
		{ConfigKey: ai.ConfigKeyModel, ConfigValue: "model-b"},
	}
	question, hasNext, err := client.InterviewAgent.GetNextQuestion(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetNextQuestion returned error after config switch: %v", err)
	}
	if !hasNext {
		t.Fatalf("expected next question to be available")
	}
	if question.Question == "" {
		t.Fatalf("expected next question content to be populated")
	}
}

// runtimeAdminConfigRepoStub 模拟后台 AI 配置仓库，便于测试 runtime 热切换。
type runtimeAdminConfigRepoStub struct {
	items []model.AdminConfig
}

// List 返回当前内存中的 AI 配置列表。
func (s *runtimeAdminConfigRepoStub) List(context.Context) ([]model.AdminConfig, error) {
	return append([]model.AdminConfig(nil), s.items...), nil
}

// GetByKey 按键返回单个 AI 配置项。
func (s *runtimeAdminConfigRepoStub) GetByKey(_ context.Context, key string) (*model.AdminConfig, error) {
	for i := range s.items {
		if s.items[i].ConfigKey == key {
			itemCopy := s.items[i]
			return &itemCopy, nil
		}
	}
	return nil, nil
}

// Upsert 满足仓库接口，当前测试场景不需要实际写入。
func (s *runtimeAdminConfigRepoStub) Upsert(context.Context, *model.AdminConfig) error {
	return nil
}

// BatchUpsert 满足仓库接口，当前测试场景不需要实际写入。
func (s *runtimeAdminConfigRepoStub) BatchUpsert(context.Context, []model.AdminConfig) error {
	return nil
}

var _ repository.AdminConfigRepository = (*runtimeAdminConfigRepoStub)(nil)
