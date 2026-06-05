package service

import "testing"

// TestMetadataJSONFromStringMap 验证 metadata 会被真实序列化，而不是被占位内容覆盖。
func TestMetadataJSONFromStringMap(t *testing.T) {
	raw, err := metadataJSONFromStringMap(map[string]string{
		"title": "Go",
		"score": "9",
	})
	if err != nil {
		t.Fatalf("metadataJSONFromStringMap returned error: %v", err)
	}
	if raw == "" || raw == "{}" {
		t.Fatalf("expected serialized metadata, got %q", raw)
	}
}
