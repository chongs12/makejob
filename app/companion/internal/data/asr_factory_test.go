package data

import (
	"testing"

	"makejob/app/companion/internal/biz"
)

// TestNewASRProviderFromConfigRecord_Volcengine 验证从配置记录创建火山引擎 ASR Provider。
func TestNewASRProviderFromConfigRecord_Volcengine(t *testing.T) {
	record := &biz.ASRConfig{
		Engine:         "volcengine",
		AuthConfigJSON: `{"app_id":"test-app","access_token":"test-token"}`,
	}

	provider, err := NewASRProviderFromConfigRecord(record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}

	engines := provider.GetSupportedEngines()
	found := false
	for _, e := range engines {
		if e == "volcengine" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'volcengine' in engines, got %v", engines)
	}
}

// TestNewASRProviderFromConfigRecord_MissingAuth 验证缺少鉴权配置时返回错误。
func TestNewASRProviderFromConfigRecord_MissingAuth(t *testing.T) {
	record := &biz.ASRConfig{
		Engine:         "volcengine",
		AuthConfigJSON: `{"app_id":"test-app"}`,
	}

	_, err := NewASRProviderFromConfigRecord(record)
	if err == nil {
		t.Fatal("expected error for missing access_token")
	}
}

// TestNewASRProviderFromConfigRecord_UnsupportedEngine 验证不支持的引擎返回错误。
func TestNewASRProviderFromConfigRecord_UnsupportedEngine(t *testing.T) {
	record := &biz.ASRConfig{
		Engine:         "unsupported",
		AuthConfigJSON: `{}`,
	}

	_, err := NewASRProviderFromConfigRecord(record)
	if err == nil {
		t.Fatal("expected error for unsupported engine")
	}
}

// TestNewASRProviderFromConfigRecord_NilRecord 验证空记录返回错误。
func TestNewASRProviderFromConfigRecord_NilRecord(t *testing.T) {
	_, err := NewASRProviderFromConfigRecord(nil)
	if err == nil {
		t.Fatal("expected error for nil record")
	}
}

// TestNewASRClientAdapter 验证适配器正确包装 provider。
func TestNewASRClientAdapter(t *testing.T) {
	mockProvider := biz.NewMockASRProvider()
	client := NewASRClientAdapter(mockProvider)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// TestNewASRClientAdapter_NilProvider 验证 nil provider 返回 nil client。
func TestNewASRClientAdapter_NilProvider(t *testing.T) {
	client := NewASRClientAdapter(nil)
	if client != nil {
		t.Fatal("expected nil client for nil provider")
	}
}
