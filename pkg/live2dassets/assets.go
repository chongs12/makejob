package live2dassets

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
)

const (
	// MountPath 是 Live2D 静态资源的统一访问前缀。
	MountPath = "/live2d-assets"
	// AssetsDirEnv 指定 Live2D 资源目录的环境变量名。
	AssetsDirEnv = "LIVE2D_ASSETS_DIR"
)

// ImportedPackage 描述后台导入后的 Live2D 模型资源结果。
type ImportedPackage struct {
	Name          string
	AssetDir      string
	ModelPath     string
	ModelURL      string
	ThumbnailPath string
	ThumbnailURL  string
}

// ImportedBackground 描述后台上传后的舞台背景图片资源结果。
type ImportedBackground struct {
	FileName  string
	AssetPath string
	AssetURL  string
}

var assetDirCandidates = []string{
	"live2d-src",
	filepath.Join("..", "live2d-src"),
	filepath.Join("..", "..", "live2d-src"),
	filepath.Join("..", "..", "..", "live2d-src"),
	filepath.Join("..", "..", "..", "..", "live2d-src"),
}

var imageExtensions = []string{".png", ".jpg", ".jpeg", ".webp"}

// ResolveAssetsDir 返回当前进程可用的 Live2D 资源目录。
func ResolveAssetsDir() string {
	if configuredDir := strings.TrimSpace(os.Getenv(AssetsDirEnv)); configuredDir != "" {
		if isExistingDir(configuredDir) {
			return configuredDir
		}
		return ""
	}

	for _, candidate := range assetDirCandidates {
		if isExistingDir(candidate) {
			return candidate
		}
	}
	return ""
}

// EnsureAssetsDir 返回可写的 Live2D 资源目录；若目录不存在则自动创建。
func EnsureAssetsDir() (string, error) {
	if configuredDir := strings.TrimSpace(os.Getenv(AssetsDirEnv)); configuredDir != "" {
		if err := os.MkdirAll(configuredDir, 0o755); err != nil {
			return "", fmt.Errorf("create live2d assets dir: %w", err)
		}
		return configuredDir, nil
	}

	if resolved := ResolveAssetsDir(); resolved != "" {
		return resolved, nil
	}

	defaultDir := assetDirCandidates[0]
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		return "", fmt.Errorf("create live2d assets dir: %w", err)
	}
	return defaultDir, nil
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

// DiscoverLocalModels 扫描当前资源目录下的模型文件夹，并返回可直接访问的模型信息列表。
func DiscoverLocalModels() ([]ImportedPackage, error) {
	assetRoot := ResolveAssetsDir()
	if assetRoot == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(assetRoot)
	if err != nil {
		return nil, fmt.Errorf("read live2d assets dir: %w", err)
	}

	models := make([]ImportedPackage, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		assetDir := entry.Name()
		targetDir := filepath.Join(assetRoot, assetDir)
		modelPath, thumbnailPath, modelName, err := detectImportedAssets(targetDir, assetDir)
		if err != nil {
			continue
		}

		thumbnailURL := ""
		if strings.TrimSpace(thumbnailPath) != "" {
			thumbnailURL = AssetURL(thumbnailPath)
		}

		models = append(models, ImportedPackage{
			Name:          firstNonEmpty(strings.TrimSpace(modelName), assetDir),
			AssetDir:      filepath.ToSlash(assetDir),
			ModelPath:     modelPath,
			ModelURL:      AssetURL(modelPath),
			ThumbnailPath: thumbnailPath,
			ThumbnailURL:  thumbnailURL,
		})
	}

	slices.SortFunc(models, func(left ImportedPackage, right ImportedPackage) int {
		return comparePreferredPath(left.AssetDir, right.AssetDir)
	})
	return models, nil
}

// ManagedModelAssetDirFromURL 从模型 URL 中解析受管资源目录名，无法安全识别时返回空字符串。
func ManagedModelAssetDirFromURL(modelURL string) string {
	relativePath := managedAssetRelativePathFromURL(modelURL)
	if relativePath == "" {
		return ""
	}

	parts := strings.Split(relativePath, "/")
	if len(parts) == 0 {
		return ""
	}

	assetDir := strings.TrimSpace(parts[0])
	if assetDir == "" || strings.EqualFold(assetDir, "backgrounds") {
		return ""
	}
	return assetDir
}

// DeleteManagedModelAssetDir 删除指定受管模型资源目录，供后台确认删除模型时同步清理文件。
func DeleteManagedModelAssetDir(assetDir string) error {
	normalizedAssetDir := strings.TrimSpace(filepath.ToSlash(assetDir))
	if normalizedAssetDir == "" || strings.Contains(normalizedAssetDir, "/") || strings.EqualFold(normalizedAssetDir, "backgrounds") {
		return nil
	}

	assetsRoot := ResolveAssetsDir()
	if assetsRoot == "" {
		return nil
	}

	targetDir := filepath.Join(assetsRoot, filepath.FromSlash(normalizedAssetDir))
	if !isExistingDir(targetDir) {
		return nil
	}
	if !isSubPath(assetsRoot, targetDir) {
		return fmt.Errorf("invalid live2d asset dir: %s", assetDir)
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("remove live2d asset dir: %w", err)
	}
	return nil
}

// ImportBackgroundImage 导入后台上传的舞台背景图，并返回可直接访问的静态地址。
func ImportBackgroundImage(filename string, raw []byte) (*ImportedBackground, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty live2d background image")
	}

	assetsDir, err := EnsureAssetsDir()
	if err != nil {
		return nil, err
	}

	extension := strings.ToLower(strings.TrimSpace(filepath.Ext(filename)))
	if !isImageFile(extension) {
		return nil, fmt.Errorf("unsupported background image type")
	}

	backgroundsDir := filepath.Join(assetsDir, "backgrounds")
	if err := os.MkdirAll(backgroundsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create live2d backgrounds dir: %w", err)
	}

	baseName := sanitizeAssetDirName(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	if baseName == "" {
		baseName = "background"
	}

	targetFileName, err := allocateBackgroundFileName(backgroundsDir, baseName, extension)
	if err != nil {
		return nil, err
	}

	targetPath := filepath.Join(backgroundsDir, targetFileName)
	if err := os.WriteFile(targetPath, raw, 0o644); err != nil {
		return nil, fmt.Errorf("write live2d background image: %w", err)
	}

	assetPath := filepath.ToSlash(filepath.Join("backgrounds", targetFileName))
	return &ImportedBackground{
		FileName:  targetFileName,
		AssetPath: assetPath,
		AssetURL:  AssetURL(assetPath),
	}, nil
}

// ImportZip 导入后台上传的 Live2D ZIP 包，并返回自动识别的模型信息。
func ImportZip(filename string, raw []byte) (*ImportedPackage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty live2d package")
	}

	assetsDir, err := EnsureAssetsDir()
	if err != nil {
		return nil, err
	}

	packageName := detectPackageName(filename)
	targetDir, assetDir, err := prepareImportDir(assetsDir, packageName)
	if err != nil {
		return nil, err
	}

	if err := unzipPackage(raw, targetDir); err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, err
	}

	modelPath, thumbnailPath, modelName, err := detectImportedAssets(targetDir, assetDir)
	if err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, err
	}

	result := &ImportedPackage{
		Name:         firstNonEmpty(strings.TrimSpace(modelName), packageName, filepath.Base(assetDir)),
		AssetDir:     filepath.ToSlash(assetDir),
		ModelPath:    modelPath,
		ModelURL:     AssetURL(modelPath),
		ThumbnailURL: "",
	}
	if thumbnailPath != "" {
		result.ThumbnailPath = thumbnailPath
		result.ThumbnailURL = AssetURL(thumbnailPath)
	}

	return result, nil
}

// allocateBackgroundFileName 为舞台背景图分配一个不冲突的目标文件名。
func allocateBackgroundFileName(backgroundsDir string, baseName string, extension string) (string, error) {
	candidateName := baseName + extension
	if !isExistingPath(filepath.Join(backgroundsDir, candidateName)) {
		return candidateName, nil
	}

	for index := 2; index <= 9999; index++ {
		candidateName = fmt.Sprintf("%s-%d%s", baseName, index, extension)
		if isExistingPath(filepath.Join(backgroundsDir, candidateName)) {
			continue
		}
		return candidateName, nil
	}

	return "", fmt.Errorf("unable to allocate live2d background filename")
}

// detectPackageName 推导上传包的展示名称。
func detectPackageName(filename string) string {
	baseName := strings.TrimSpace(filepath.Base(filename))
	ext := filepath.Ext(baseName)
	name := strings.TrimSpace(strings.TrimSuffix(baseName, ext))
	return firstNonEmpty(name, "live2d-model")
}

// prepareImportDir 为新导入资源准备唯一的目标目录。
func prepareImportDir(assetsDir string, packageName string) (string, string, error) {
	dirName := sanitizeAssetDirName(packageName)
	if dirName == "" {
		dirName = fmt.Sprintf("live2d-%d", time.Now().Unix())
	}

	targetDir := filepath.Join(assetsDir, dirName)
	if !isExistingDir(targetDir) && !isExistingPath(targetDir) {
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return "", "", fmt.Errorf("create live2d import dir: %w", err)
		}
		return targetDir, dirName, nil
	}

	for index := 2; index <= 9999; index++ {
		candidateDir := fmt.Sprintf("%s-%d", dirName, index)
		targetDir = filepath.Join(assetsDir, candidateDir)
		if isExistingPath(targetDir) {
			continue
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return "", "", fmt.Errorf("create live2d import dir: %w", err)
		}
		return targetDir, candidateDir, nil
	}

	return "", "", fmt.Errorf("unable to allocate live2d import dir")
}

// unzipPackage 将 ZIP 包安全解压到目标目录。
func unzipPackage(raw []byte, targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fmt.Errorf("read live2d zip package: %w", err)
	}

	for _, file := range reader.File {
		relativePath, err := normalizeArchivePath(file.Name)
		if err != nil {
			return err
		}
		if relativePath == "" {
			continue
		}

		targetPath := filepath.Join(targetDir, filepath.FromSlash(relativePath))
		if !isSubPath(targetDir, targetPath) {
			return fmt.Errorf("invalid live2d zip entry: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create live2d dir: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create live2d parent dir: %w", err)
		}

		if err := writeArchiveFile(file, targetPath); err != nil {
			return err
		}
	}

	return nil
}

// normalizeArchivePath 清洗压缩包内路径，阻止目录穿越和非法绝对路径。
func normalizeArchivePath(path string) (string, error) {
	normalized := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "/")
	normalized = filepath.ToSlash(filepath.Clean(normalized))

	if normalized == "." || normalized == "" {
		return "", nil
	}
	if strings.HasPrefix(normalized, "../") || normalized == ".." {
		return "", fmt.Errorf("invalid live2d zip entry: %s", path)
	}

	return normalized, nil
}

// writeArchiveFile 将单个压缩包文件写入目标路径。
func writeArchiveFile(file *zip.File, targetPath string) error {
	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("open live2d zip entry: %w", err)
	}
	defer reader.Close()

	writer, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create live2d asset file: %w", err)
	}
	defer writer.Close()

	if _, err := io.Copy(writer, reader); err != nil {
		return fmt.Errorf("write live2d asset file: %w", err)
	}

	return nil
}

// detectImportedAssets 扫描导入目录并识别主模型文件和缩略图。
func detectImportedAssets(targetDir string, assetDir string) (string, string, string, error) {
	modelFiles := make([]string, 0)
	imageFiles := make([]string, 0)

	if err := filepath.WalkDir(targetDir, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		fileName := strings.ToLower(entry.Name())
		if strings.HasSuffix(fileName, ".model3.json") {
			modelFiles = append(modelFiles, currentPath)
		}
		if isImageFile(fileName) {
			imageFiles = append(imageFiles, currentPath)
		}
		return nil
	}); err != nil {
		return "", "", "", fmt.Errorf("scan live2d assets: %w", err)
	}

	if len(modelFiles) == 0 {
		return "", "", "", fmt.Errorf("live2d model3.json not found")
	}

	slices.SortFunc(modelFiles, comparePreferredPath)
	modelFile := modelFiles[0]
	thumbnailFile := detectThumbnailFile(modelFile, imageFiles)

	modelPath, err := toAssetRelativePath(targetDir, assetDir, modelFile)
	if err != nil {
		return "", "", "", err
	}

	thumbnailPath := ""
	if thumbnailFile != "" {
		thumbnailPath, err = toAssetRelativePath(targetDir, assetDir, thumbnailFile)
		if err != nil {
			return "", "", "", err
		}
	}

	modelName := strings.TrimSuffix(filepath.Base(modelFile), ".model3.json")
	return modelPath, thumbnailPath, modelName, nil
}

// detectThumbnailFile 根据主模型文件优先推导最合适的缩略图。
func detectThumbnailFile(modelFile string, imageFiles []string) string {
	if len(imageFiles) == 0 {
		return ""
	}

	modelBase := strings.TrimSuffix(filepath.Base(modelFile), ".model3.json")
	modelDir := filepath.Dir(modelFile)

	for _, extension := range imageExtensions {
		candidate := filepath.Join(modelDir, modelBase+extension)
		if containsPath(imageFiles, candidate) {
			return candidate
		}
	}

	for _, file := range imageFiles {
		lowerName := strings.ToLower(filepath.Base(file))
		if strings.Contains(lowerName, "thumb") || strings.Contains(lowerName, "cover") || strings.Contains(lowerName, "preview") || strings.Contains(lowerName, "poster") {
			return file
		}
	}

	slices.SortFunc(imageFiles, comparePreferredPath)
	return imageFiles[0]
}

// toAssetRelativePath 将目标文件转换为静态目录相对路径。
func toAssetRelativePath(targetDir string, assetDir string, targetFile string) (string, error) {
	relativePath, err := filepath.Rel(targetDir, targetFile)
	if err != nil {
		return "", fmt.Errorf("resolve live2d asset path: %w", err)
	}

	normalizedRelative := filepath.ToSlash(relativePath)
	normalizedRelative = strings.TrimPrefix(normalizedRelative, "./")
	if normalizedRelative == "." || normalizedRelative == "" {
		return "", fmt.Errorf("invalid live2d asset path")
	}
	return filepath.ToSlash(filepath.Join(assetDir, normalizedRelative)), nil
}

// managedAssetRelativePathFromURL 将 Live2D 静态 URL 转换为资源目录相对路径。
func managedAssetRelativePathFromURL(modelURL string) string {
	normalizedURL := strings.TrimSpace(filepath.ToSlash(modelURL))
	mountPrefix := MountPath + "/"
	if !strings.HasPrefix(normalizedURL, mountPrefix) {
		return ""
	}

	relativePath := strings.TrimPrefix(normalizedURL, mountPrefix)
	relativePath = strings.TrimSpace(filepath.ToSlash(relativePath))
	if relativePath == "" || strings.HasPrefix(relativePath, "../") || strings.Contains(relativePath, "//") {
		return ""
	}
	return relativePath
}

// comparePreferredPath 根据路径层级和字典序稳定排序资源。
func comparePreferredPath(left string, right string) int {
	leftValue := strings.TrimSpace(filepath.ToSlash(left))
	rightValue := strings.TrimSpace(filepath.ToSlash(right))
	if leftValue == rightValue {
		return 0
	}

	leftDepth := strings.Count(leftValue, "/")
	rightDepth := strings.Count(rightValue, "/")
	if leftDepth != rightDepth {
		if leftDepth < rightDepth {
			return -1
		}
		return 1
	}

	if strings.Compare(strings.ToLower(leftValue), strings.ToLower(rightValue)) < 0 {
		return -1
	}
	return 1
}

// sanitizeAssetDirName 将资源目录名归一为安全可读的 slug。
func sanitizeAssetDirName(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	builder := strings.Builder{}
	lastDash := false
	for _, char := range value {
		switch {
		case unicode.IsLetter(char), unicode.IsDigit(char):
			builder.WriteRune(unicode.ToLower(char))
			lastDash = false
		case char == '-', char == '_', unicode.IsSpace(char):
			if builder.Len() == 0 || lastDash {
				continue
			}
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

// firstNonEmpty 返回首个非空字符串。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// containsPath 判断路径列表中是否存在目标路径。
func containsPath(paths []string, target string) bool {
	normalizedTarget := filepath.Clean(target)
	for _, item := range paths {
		if filepath.Clean(item) == normalizedTarget {
			return true
		}
	}
	return false
}

// isImageFile 判断文件名或扩展名是否为支持的图片类型。
func isImageFile(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, extension := range imageExtensions {
		if normalized == extension || strings.HasSuffix(normalized, extension) {
			return true
		}
	}
	return false
}

// isExistingDir 判断目录是否存在。
func isExistingDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isExistingPath 判断路径是否已存在。
func isExistingPath(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isSubPath 判断 target 是否位于 base 目录内。
func isSubPath(base string, target string) bool {
	basePath, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	targetPath, err := filepath.Abs(target)
	if err != nil {
		return false
	}

	relativePath, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return false
	}
	normalizedRelative := filepath.ToSlash(relativePath)
	return normalizedRelative != ".." && !strings.HasPrefix(normalizedRelative, "../")
}
