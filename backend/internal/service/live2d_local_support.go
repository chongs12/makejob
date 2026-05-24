package service

import (
	"fmt"
	"strings"

	"makejob-backend/internal/live2dassets"
)

const localLive2DModelKeyPrefix = "local:"

// localLive2DModel 描述从 live2d-src 本地资源目录发现的一条可用模型。
type localLive2DModel struct {
	Key          string
	Name         string
	Scene        string
	AssetDir     string
	ModelURL     string
	ThumbnailURL string
}

// buildLocalLive2DModelKey 为本地兜底模型生成稳定的前台选择键。
func buildLocalLive2DModelKey(assetDir string) string {
	return localLive2DModelKeyPrefix + strings.TrimSpace(assetDir)
}

// parseLocalLive2DModelKey 从本地兜底模型键中解析资源目录名。
func parseLocalLive2DModelKey(modelKey string) (string, error) {
	normalized := strings.TrimSpace(modelKey)
	if !strings.HasPrefix(normalized, localLive2DModelKeyPrefix) {
		return "", fmt.Errorf("unsupported local live2d model key")
	}
	assetDir := strings.TrimSpace(strings.TrimPrefix(normalized, localLive2DModelKeyPrefix))
	if assetDir == "" {
		return "", fmt.Errorf("invalid local live2d model key")
	}
	return assetDir, nil
}

// listLocalLive2DModels 扫描 live2d-src 目录并返回当前场景可直接消费的本地模型清单。
func listLocalLive2DModels(scene string) ([]localLive2DModel, error) {
	packages, err := live2dassets.DiscoverLocalModels()
	if err != nil {
		return nil, err
	}

	items := make([]localLive2DModel, 0, len(packages))
	for _, item := range packages {
		if strings.TrimSpace(item.AssetDir) == "" || strings.TrimSpace(item.ModelURL) == "" {
			continue
		}
		items = append(items, localLive2DModel{
			Key:          buildLocalLive2DModelKey(item.AssetDir),
			Name:         strings.TrimSpace(item.Name),
			Scene:        scene,
			AssetDir:     strings.TrimSpace(item.AssetDir),
			ModelURL:     strings.TrimSpace(item.ModelURL),
			ThumbnailURL: strings.TrimSpace(item.ThumbnailURL),
		})
	}
	return items, nil
}

// resolveLocalLive2DModelByKey 根据模型键定位一条本地兜底模型记录。
func resolveLocalLive2DModelByKey(scene string, modelKey string) (*localLive2DModel, error) {
	assetDir, err := parseLocalLive2DModelKey(modelKey)
	if err != nil {
		return nil, err
	}

	items, err := listLocalLive2DModels(scene)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.AssetDir), assetDir) {
			copyValue := item
			return &copyValue, nil
		}
	}
	return nil, fmt.Errorf("local live2d model not found")
}
