package service

import (
	"context"
	"testing"

	adminv1 "makejob/api/makejob/admin/v1"
	"makejob/app/admin/internal/biz"
)

// stubAIPresetRepo 为 AI 预设更新测试提供最小仓库桩。
type stubAIPresetRepo struct {
	biz.AdminRepo
	storedPreset  *biz.AIPreset
	updatedPreset *biz.AIPreset
}

// GetAIPresetByID 返回预设详情，满足 AdminRepo 接口。
func (r *stubAIPresetRepo) GetAIPresetByID(_ context.Context, _ uint64) (*biz.AIPreset, error) {
	return r.storedPreset, nil
}

// UpdateAIPreset 记录更新后的预设，满足 AdminRepo 接口。
func (r *stubAIPresetRepo) UpdateAIPreset(_ context.Context, preset *biz.AIPreset) error {
	clone := *preset
	r.updatedPreset = &clone
	return nil
}

// TestUpdateAIPresetPreservesExistingFields 验证部分更新不会把未传字段错误清空。
func TestUpdateAIPresetPreservesExistingFields(t *testing.T) {
	repo := &stubAIPresetRepo{
		storedPreset: &biz.AIPreset{
			ID:       8,
			Name:     "stable",
			Configs:  map[string]string{"ai_model": "default"},
			IsActive: true,
		},
	}
	svc := NewAdminService(biz.NewAdminUseCase(repo, nil, nil, nil))

	resp, err := svc.UpdateAIPreset(context.Background(), &adminv1.UpdateAIPresetRequest{
		Id:      8,
		Configs: map[string]string{"ai_model": "next"},
	})
	if err != nil {
		t.Fatalf("UpdateAIPreset returned error: %v", err)
	}
	if repo.updatedPreset == nil {
		t.Fatalf("expected repo to receive updated preset")
	}
	if repo.updatedPreset.Name != "stable" {
		t.Fatalf("expected existing name to be preserved, got %q", repo.updatedPreset.Name)
	}
	if repo.updatedPreset.Configs["ai_model"] != "next" {
		t.Fatalf("expected configs to be updated, got %+v", repo.updatedPreset.Configs)
	}
	if resp.GetName() != "stable" {
		t.Fatalf("expected response name to be preserved, got %q", resp.GetName())
	}
}
