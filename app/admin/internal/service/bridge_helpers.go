package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

// joinNonEmptyTags 统一清洗并拼接标签列表。
func joinNonEmptyTags(tags []string) string {
	filtered := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			filtered = append(filtered, tag)
		}
	}
	return strings.Join(filtered, ",")
}

// metadataJSONFromStringMap 将字符串字典稳定序列化为后台持久化所需的 JSON 文本。
func metadataJSONFromStringMap(input map[string]string) (string, error) {
	if len(input) == 0 {
		return "", nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal metadata failed: %w", err)
	}
	return string(data), nil
}
