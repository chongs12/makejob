package live2dassets

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// MountPath 是 Live2D 静态资源的统一访问前缀。
	MountPath = "/live2d-assets"
)

var assetDirCandidates = []string{
	"live2d-src",
	filepath.Join("..", "live2d-src"),
	filepath.Join("..", "..", "live2d-src"),
	filepath.Join("..", "..", "..", "live2d-src"),
	filepath.Join("..", "..", "..", "..", "live2d-src"),
}

// ResolveAssetsDir 返回当前进程可用的 Live2D 资源目录。
func ResolveAssetsDir() string {
	for _, candidate := range assetDirCandidates {
		if isExistingDir(candidate) {
			return candidate
		}
	}
	return ""
}

// AssetURL 将相对资源路径转换为可访问的静态 URL。
func AssetURL(relativePath string) string {
	cleaned := strings.TrimSpace(relativePath)
	cleaned = strings.TrimPrefix(cleaned, "/")
	cleaned = filepath.ToSlash(cleaned)
	if cleaned == "" {
		return MountPath
	}
	return MountPath + "/" + cleaned
}

// HasAsset 判断指定相对路径的资源是否存在。
func HasAsset(relativePath string) bool {
	assetDir := ResolveAssetsDir()
	if assetDir == "" {
		return false
	}
	targetPath := filepath.Join(assetDir, filepath.FromSlash(strings.TrimPrefix(strings.TrimSpace(relativePath), "/")))
	info, err := os.Stat(targetPath)
	return err == nil && !info.IsDir()
}

// isExistingDir 判断路径是否存在且为目录。
func isExistingDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
