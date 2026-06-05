package service

import (
	"makejob-backend/bridge"
	adminv1 "makejob/api/makejob/admin/v1"
)

// mergeAdminConfigItems 将数据库中的后台配置覆盖到基础默认值上，保持配置读取语义与单体一致。
func mergeAdminConfigItems(items []*adminv1.AdminConfigItem, base map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(items))
	for key, value := range base {
		merged[key] = value
	}
	for _, item := range items {
		if item == nil || item.Key == "" {
			continue
		}
		merged[item.Key] = item.Value
	}
	return merged
}

// normalizeAdminAIConfigs 通过 bridge 复用单体配置规则，统一清洗并校验后台提交的 AI 配置。
func normalizeAdminAIConfigs(configs map[string]string) (map[string]string, error) {
	return bridge.NormalizeAIConfigs(configs)
}
