package bridge

import (
	"fmt"

	"makejob-backend/internal/ai"
	aiRuntime "makejob-backend/internal/ai/runtime"
)

// NormalizeAIConfigs 复用单体运行时规则统一清洗并校验一份完整 AI 配置快照。
func NormalizeAIConfigs(configs map[string]string) (map[string]string, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("configs cannot be empty")
	}

	normalized := ai.NormalizeRuntimeConfig(configs)
	if err := aiRuntime.ValidateRuntimeConfig(normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}
