package service

import (
	"encoding/json"
	"strings"

	kratoserr "github.com/go-kratos/kratos/v2/errors"
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
		return "", kratoserr.InternalServer("METADATA_MARSHAL_FAILED", "序列化元数据失败").WithCause(err)
	}
	return string(data), nil
}
