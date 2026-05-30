package eino

import (
	"context"
	"testing"
)

func TestNewEmbedder_Validation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		apiKey  string
		model   string
		baseURL string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "空apiKey",
			apiKey:  "",
			model:   "doubao-embedding-large-text-240915",
			baseURL: "",
			wantErr: true,
			errMsg:  "apiKey不能为空",
		},
		{
			name:    "空model",
			apiKey:  "test-key",
			model:   "",
			baseURL: "",
			wantErr: true,
			errMsg:  "model不能为空",
		},
		{
			name:    "空白字符apiKey",
			apiKey:  "   ",
			model:   "doubao-embedding-large-text-240915",
			baseURL: "",
			wantErr: true,
			errMsg:  "apiKey不能为空",
		},
		{
			name:    "空白字符model",
			apiKey:  "test-key",
			model:   "   ",
			baseURL: "",
			wantErr: true,
			errMsg:  "model不能为空",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEmbedder(ctx, tt.apiKey, tt.model, tt.baseURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewEmbedder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("NewEmbedder() error = %v, want contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
