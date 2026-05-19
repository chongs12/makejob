package service

import (
	"context"
	"testing"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
)

// TestAdminServiceGetAIConfigsIncludesPresets 验证 AI 配置页响应会携带预设列表和当前生效预设 ID。
func TestAdminServiceGetAIConfigsIncludesPresets(t *testing.T) {
	presetJSON, err := serializeAIPresetConfigs(buildValidTestRuntimeConfig("preset-model"))
	if err != nil {
		t.Fatalf("serialize preset config: %v", err)
	}

	svc := &adminService{
		adminConfigRepo: &adminServiceConfigRepoStub{
			items: []model.AdminConfig{
				{ConfigKey: ai.ConfigKeyModel, ConfigValue: "runtime-model"},
			},
		},
		aiPresetRepo: &adminServicePresetRepoStub{
			presets: []model.AIPreset{
				{
					BaseModel:  model.BaseModel{ID: 7},
					Name:       "常用配置",
					ConfigJSON: presetJSON,
					IsActive:   true,
				},
			},
		},
		baseAIConfig: ai.DefaultRuntimeConfig(),
	}

	response, err := svc.GetAIConfigs(context.Background())
	if err != nil {
		t.Fatalf("GetAIConfigs returned error: %v", err)
	}
	if response.ActivePresetID == nil || *response.ActivePresetID != 7 {
		t.Fatalf("expected active preset id 7, got %#v", response.ActivePresetID)
	}
	if len(response.Presets) != 1 || response.Presets[0].Name != "常用配置" {
		t.Fatalf("expected preset list to contain 常用配置, got %#v", response.Presets)
	}
}

// TestAdminServiceCreateAIPresetStoresSnapshot 验证创建预设时会保存完整 AI 配置快照。
func TestAdminServiceCreateAIPresetStoresSnapshot(t *testing.T) {
	presetRepo := &adminServicePresetRepoStub{}
	svc := &adminService{
		aiPresetRepo: presetRepo,
		baseAIConfig: ai.DefaultRuntimeConfig(),
	}

	preset, err := svc.CreateAIPreset(context.Background(), &CreateAIPresetRequest{
		Name:    "面试高配",
		Configs: buildValidTestRuntimeConfig("interview-plus"),
	})
	if err != nil {
		t.Fatalf("CreateAIPreset returned error: %v", err)
	}
	if preset.ID == 0 {
		t.Fatalf("expected created preset to have id")
	}
	if len(presetRepo.presets) != 1 {
		t.Fatalf("expected one preset to be stored, got %d", len(presetRepo.presets))
	}

	configs, err := parseAIPresetConfigs(presetRepo.presets[0].ConfigJSON)
	if err != nil {
		t.Fatalf("parse stored preset config: %v", err)
	}
	if configs[ai.ConfigKeyModel] != "interview-plus" {
		t.Fatalf("expected stored model interview-plus, got %q", configs[ai.ConfigKeyModel])
	}
}

// TestAdminServiceApplyAIPresetUpdatesRuntime 验证应用预设会覆盖当前运行配置并更新唯一活动预设。
func TestAdminServiceApplyAIPresetUpdatesRuntime(t *testing.T) {
	configJSON1, err := serializeAIPresetConfigs(buildValidTestRuntimeConfig("model-a"))
	if err != nil {
		t.Fatalf("serialize preset A: %v", err)
	}
	configJSON2, err := serializeAIPresetConfigs(buildValidTestRuntimeConfig("model-b"))
	if err != nil {
		t.Fatalf("serialize preset B: %v", err)
	}

	configRepo := &adminServiceConfigRepoStub{}
	presetRepo := &adminServicePresetRepoStub{
		presets: []model.AIPreset{
			{BaseModel: model.BaseModel{ID: 1}, Name: "A", ConfigJSON: configJSON1, IsActive: true},
			{BaseModel: model.BaseModel{ID: 2}, Name: "B", ConfigJSON: configJSON2, IsActive: false},
		},
	}
	svc := &adminService{
		adminConfigRepo: configRepo,
		aiPresetRepo:    presetRepo,
		baseAIConfig:    ai.DefaultRuntimeConfig(),
	}

	response, err := svc.ApplyAIPreset(context.Background(), 2)
	if err != nil {
		t.Fatalf("ApplyAIPreset returned error: %v", err)
	}
	if response.ActivePresetID == nil || *response.ActivePresetID != 2 {
		t.Fatalf("expected active preset id 2 after apply, got %#v", response.ActivePresetID)
	}
	if got := configRepo.values[ai.ConfigKeyModel]; got != "model-b" {
		t.Fatalf("expected runtime model to become model-b, got %q", got)
	}
	activePreset, err := presetRepo.GetActive(context.Background())
	if err != nil {
		t.Fatalf("GetActive returned error: %v", err)
	}
	if activePreset == nil || activePreset.ID != 2 {
		t.Fatalf("expected preset 2 to become active, got %#v", activePreset)
	}
}

// TestAdminServiceDeleteAIPresetRejectsActive 验证当前活动预设不可直接删除。
func TestAdminServiceDeleteAIPresetRejectsActive(t *testing.T) {
	configJSON, err := serializeAIPresetConfigs(buildValidTestRuntimeConfig("model-a"))
	if err != nil {
		t.Fatalf("serialize preset config: %v", err)
	}

	svc := &adminService{
		aiPresetRepo: &adminServicePresetRepoStub{
			presets: []model.AIPreset{
				{BaseModel: model.BaseModel{ID: 1}, Name: "A", ConfigJSON: configJSON, IsActive: true},
			},
		},
		baseAIConfig: ai.DefaultRuntimeConfig(),
	}

	err = svc.DeleteAIPreset(context.Background(), 1)
	if err == nil {
		t.Fatalf("expected deleting active preset to fail")
	}
	businessErr, ok := err.(*common.BusinessError)
	if !ok || businessErr.Code != common.CodeBadRequest {
		t.Fatalf("expected bad request business error, got %#v", err)
	}
}

// TestAdminServiceUpdateAIConfigsSyncsActivePreset 验证直接保存运行配置时会同步更新当前活动预设快照。
func TestAdminServiceUpdateAIConfigsSyncsActivePreset(t *testing.T) {
	oldConfigJSON, err := serializeAIPresetConfigs(buildValidTestRuntimeConfig("old-model"))
	if err != nil {
		t.Fatalf("serialize old preset config: %v", err)
	}

	configRepo := &adminServiceConfigRepoStub{}
	presetRepo := &adminServicePresetRepoStub{
		presets: []model.AIPreset{
			{BaseModel: model.BaseModel{ID: 3}, Name: "活动预设", ConfigJSON: oldConfigJSON, IsActive: true},
		},
	}
	svc := &adminService{
		adminConfigRepo: configRepo,
		aiPresetRepo:    presetRepo,
		baseAIConfig:    ai.DefaultRuntimeConfig(),
	}

	err = svc.UpdateAIConfigs(context.Background(), map[string]string{
		ai.ConfigKeyProvider: string(ai.ProviderTypeEino),
		ai.ConfigKeyModel:    "new-model",
		ai.ConfigKeyAPIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("UpdateAIConfigs returned error: %v", err)
	}
	if got := configRepo.values[ai.ConfigKeyModel]; got != "new-model" {
		t.Fatalf("expected runtime model new-model, got %q", got)
	}

	activePreset, err := presetRepo.GetActive(context.Background())
	if err != nil {
		t.Fatalf("GetActive returned error: %v", err)
	}
	configs, err := parseAIPresetConfigs(activePreset.ConfigJSON)
	if err != nil {
		t.Fatalf("parse synced preset config: %v", err)
	}
	if configs[ai.ConfigKeyModel] != "new-model" {
		t.Fatalf("expected active preset snapshot new-model, got %q", configs[ai.ConfigKeyModel])
	}
}

// adminServiceConfigRepoStub 模拟 AI 配置仓库，便于断言写回的运行配置内容。
type adminServiceConfigRepoStub struct {
	items   []model.AdminConfig
	values  map[string]string
	upserts int
}

// List 返回当前保存的 AI 配置项。
func (s *adminServiceConfigRepoStub) List(context.Context) ([]model.AdminConfig, error) {
	return append([]model.AdminConfig(nil), s.items...), nil
}

// GetByKey 按键返回单个 AI 配置项。
func (s *adminServiceConfigRepoStub) GetByKey(_ context.Context, key string) (*model.AdminConfig, error) {
	for i := range s.items {
		if s.items[i].ConfigKey == key {
			itemCopy := s.items[i]
			return &itemCopy, nil
		}
	}
	return nil, nil
}

// Upsert 写入单个 AI 配置项。
func (s *adminServiceConfigRepoStub) Upsert(_ context.Context, config *model.AdminConfig) error {
	s.upserts++
	if s.values == nil {
		s.values = make(map[string]string)
	}
	s.values[config.ConfigKey] = config.ConfigValue

	replaced := false
	for i := range s.items {
		if s.items[i].ConfigKey == config.ConfigKey {
			s.items[i] = *config
			replaced = true
			break
		}
	}
	if !replaced {
		s.items = append(s.items, *config)
	}
	return nil
}

// BatchUpsert 批量写入 AI 配置项。
func (s *adminServiceConfigRepoStub) BatchUpsert(ctx context.Context, configs []model.AdminConfig) error {
	for i := range configs {
		if err := s.Upsert(ctx, &configs[i]); err != nil {
			return err
		}
	}
	return nil
}

// adminServicePresetRepoStub 模拟 AI 预设仓库，便于测试活动预设切换和快照同步。
type adminServicePresetRepoStub struct {
	presets []model.AIPreset
	nextID  uint
}

// List 返回当前所有 AI 预设。
func (s *adminServicePresetRepoStub) List(context.Context) ([]model.AIPreset, error) {
	return append([]model.AIPreset(nil), s.presets...), nil
}

// GetByID 按 ID 返回 AI 预设。
func (s *adminServicePresetRepoStub) GetByID(_ context.Context, id uint) (*model.AIPreset, error) {
	for i := range s.presets {
		if s.presets[i].ID == id {
			presetCopy := s.presets[i]
			return &presetCopy, nil
		}
	}
	return nil, nil
}

// GetByName 按名称返回 AI 预设。
func (s *adminServicePresetRepoStub) GetByName(_ context.Context, name string) (*model.AIPreset, error) {
	for i := range s.presets {
		if s.presets[i].Name == name {
			presetCopy := s.presets[i]
			return &presetCopy, nil
		}
	}
	return nil, nil
}

// GetActive 返回当前活动 AI 预设。
func (s *adminServicePresetRepoStub) GetActive(context.Context) (*model.AIPreset, error) {
	for i := range s.presets {
		if s.presets[i].IsActive {
			presetCopy := s.presets[i]
			return &presetCopy, nil
		}
	}
	return nil, nil
}

// Create 创建一条新的 AI 预设记录。
func (s *adminServicePresetRepoStub) Create(_ context.Context, preset *model.AIPreset) error {
	if s.nextID == 0 {
		s.nextID = uint(len(s.presets) + 1)
	}
	presetCopy := *preset
	presetCopy.ID = s.nextID
	s.nextID++
	*preset = presetCopy
	s.presets = append(s.presets, presetCopy)
	return nil
}

// Update 更新指定 AI 预设记录。
func (s *adminServicePresetRepoStub) Update(_ context.Context, preset *model.AIPreset) error {
	for i := range s.presets {
		if s.presets[i].ID == preset.ID {
			s.presets[i] = *preset
			return nil
		}
	}
	return nil
}

// Delete 删除指定 AI 预设记录。
func (s *adminServicePresetRepoStub) Delete(_ context.Context, id uint) error {
	filtered := make([]model.AIPreset, 0, len(s.presets))
	for _, preset := range s.presets {
		if preset.ID == id {
			continue
		}
		filtered = append(filtered, preset)
	}
	s.presets = filtered
	return nil
}

// SetActive 切换当前唯一生效的 AI 预设。
func (s *adminServicePresetRepoStub) SetActive(_ context.Context, id uint) error {
	for i := range s.presets {
		s.presets[i].IsActive = s.presets[i].ID == id
	}
	return nil
}

// ClearActive 清空当前活动 AI 预设标记。
func (s *adminServicePresetRepoStub) ClearActive(context.Context) error {
	for i := range s.presets {
		s.presets[i].IsActive = false
	}
	return nil
}

// buildValidTestRuntimeConfig 返回一份满足当前 runtime 校验要求的最小测试配置。
func buildValidTestRuntimeConfig(modelName string) map[string]string {
	return map[string]string{
		ai.ConfigKeyProvider: string(ai.ProviderTypeEino),
		ai.ConfigKeyModel:    modelName,
		ai.ConfigKeyAPIKey:   "test-key",
	}
}
