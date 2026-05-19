package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/common"
	"makejob-backend/internal/live2dassets"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

var live2DDirectiveTimeout = 1200 * time.Millisecond

// Live2DDirectiveService 定义模型解析与结构化指令生成能力。
type Live2DDirectiveService interface {
	ResolveActiveManifest(ctx context.Context, scene string, modelKey string) (*ai.Live2DManifest, error)
	GenerateDirective(ctx context.Context, req ai.Live2DDirectiveContext) (*ai.Live2DDirective, error)
}

// live2dDirectiveService 实现模型校验、manifest 解析缓存与指令生成。
type live2dDirectiveService struct {
	live2DRepo repository.Live2DModelRepository
	director   ai.Live2DDirectiveGenerator
	cacheMu    sync.RWMutex
	cache      map[string]ai.Live2DManifest
}

// live2DManifestModelPayload 描述 model3.json 中与表达式发现相关的最小字段。
type live2DManifestModelPayload struct {
	FileReferences struct {
		DisplayInfo string `json:"DisplayInfo"`
		Expressions []struct {
			Name string `json:"Name"`
			File string `json:"File"`
		} `json:"Expressions"`
		Motions map[string][]struct {
			File string `json:"File"`
		} `json:"Motions"`
	} `json:"FileReferences"`
}

// live2DManifestDisplayInfoPayload 描述 cdi3.json 中的参数名称信息。
type live2DManifestDisplayInfoPayload struct {
	Parameters []struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
	} `json:"Parameters"`
}

// live2DManifestVtubePayload 描述 vtube.json 中的参数范围信息。
type live2DManifestVtubePayload struct {
	ParameterSettings []struct {
		Name             string  `json:"Name"`
		OutputLive2D     string  `json:"OutputLive2D"`
		OutputRangeLower float64 `json:"OutputRangeLower"`
		OutputRangeUpper float64 `json:"OutputRangeUpper"`
	} `json:"ParameterSettings"`
}

// NewLive2DDirectiveService 创建 Live2D 指令服务。
func NewLive2DDirectiveService(
	live2DRepo repository.Live2DModelRepository,
	director ai.Live2DDirectiveGenerator,
) Live2DDirectiveService {
	return &live2dDirectiveService{
		live2DRepo: live2DRepo,
		director:   director,
		cache:      make(map[string]ai.Live2DManifest),
	}
}

// ResolveActiveManifest 按模型键解析并返回当前可用模型的控制清单。
func (s *live2dDirectiveService) ResolveActiveManifest(ctx context.Context, scene string, modelKey string) (*ai.Live2DManifest, error) {
	scene, err := normalizeLive2DScene(scene)
	if err != nil {
		return nil, err
	}
	modelEntity, err := s.resolveActiveModel(ctx, scene, modelKey)
	if err != nil {
		return nil, err
	}
	if modelEntity == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "live2d model not found")
	}

	cacheKey := buildLive2DManifestCacheKey(modelEntity)
	s.cacheMu.RLock()
	if cached, ok := s.cache[cacheKey]; ok {
		s.cacheMu.RUnlock()
		copyValue := cached
		return &copyValue, nil
	}
	s.cacheMu.RUnlock()

	manifest, err := buildLive2DManifestFromModel(*modelEntity)
	if err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[cacheKey] = *manifest
	s.cacheMu.Unlock()
	return manifest, nil
}

// GenerateDirective 直接委托底层 Director 生成结构化指令。
func (s *live2dDirectiveService) GenerateDirective(ctx context.Context, req ai.Live2DDirectiveContext) (*ai.Live2DDirective, error) {
	if s == nil || s.director == nil {
		return nil, nil
	}
	return s.director.GenerateDirective(ctx, req)
}

// resolveActiveModel 校验模型键并确保只允许已启用的后台模型参与指令生成。
func (s *live2dDirectiveService) resolveActiveModel(ctx context.Context, scene string, modelKey string) (*model.Live2DModel, error) {
	if s.live2DRepo == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "live2d model repository unavailable")
	}

	modelID, err := parseDatabaseLive2DModelKey(modelKey)
	if err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "invalid live2d model key")
	}

	item, err := s.live2DRepo.GetByID(ctx, modelID)
	if err != nil {
		return nil, err
	}
	if item == nil || !item.IsActive || strings.TrimSpace(item.Scene) != scene {
		return nil, common.NewBusinessError(common.CodeNotFound, "live2d model not found")
	}
	return item, nil
}

// buildLive2DManifestCacheKey 生成当前模型 manifest 的缓存键。
func buildLive2DManifestCacheKey(item *model.Live2DModel) string {
	if item == nil {
		return ""
	}
	return fmt.Sprintf("%d:%s:%s", item.ID, strings.TrimSpace(item.ModelURL), strings.TrimSpace(item.UpdatedAt.String()))
}

// parseDatabaseLive2DModelKey 解析数据库模型键。
func parseDatabaseLive2DModelKey(modelKey string) (uint, error) {
	normalized := strings.TrimSpace(modelKey)
	if !strings.HasPrefix(normalized, "db:") {
		return 0, fmt.Errorf("unsupported live2d model key")
	}
	rawID := strings.TrimSpace(strings.TrimPrefix(normalized, "db:"))
	parsed, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("invalid live2d model id")
	}
	return uint(parsed), nil
}

// buildLive2DManifestFromModel 从数据库模型和本地资源文件生成可控白名单。
func buildLive2DManifestFromModel(item model.Live2DModel) (*ai.Live2DManifest, error) {
	modelFilePath, err := resolveManagedModelFilePath(item.ModelURL)
	if err != nil {
		return nil, err
	}

	modelPayload, err := readJSONFile[live2DManifestModelPayload](modelFilePath)
	if err != nil {
		return nil, fmt.Errorf("read live2d model manifest failed: %w", err)
	}

	expressions := buildManifestExpressions(modelFilePath, modelPayload)
	parameters, err := buildManifestParameters(modelFilePath, modelPayload)
	if err != nil {
		return nil, err
	}
	motions, err := buildManifestMotions(modelFilePath, modelPayload)
	if err != nil {
		return nil, err
	}

	return &ai.Live2DManifest{
		ModelKey:    buildDatabaseLive2DModelKey(item.ID),
		ModelName:   strings.TrimSpace(item.Name),
		Scene:       strings.TrimSpace(item.Scene),
		ModelURL:    strings.TrimSpace(item.ModelURL),
		Expressions: expressions,
		Parameters:  parameters,
		Motions:     motions,
	}, nil
}

// resolveManagedModelFilePath 把受管静态 URL 映射到本地模型文件绝对路径。
func resolveManagedModelFilePath(modelURL string) (string, error) {
	relativePath := strings.TrimSpace(strings.TrimPrefix(modelURL, live2dassets.MountPath+"/"))
	if relativePath == "" {
		return "", fmt.Errorf("invalid managed live2d model url")
	}
	assetsRoot := live2dassets.ResolveAssetsDir()
	if strings.TrimSpace(assetsRoot) == "" {
		return "", fmt.Errorf("live2d assets dir not found")
	}

	targetPath := filepath.Join(assetsRoot, filepath.FromSlash(relativePath))
	if !strings.HasSuffix(strings.ToLower(targetPath), ".model3.json") {
		return "", fmt.Errorf("live2d model manifest file is missing")
	}
	if _, err := os.Stat(targetPath); err != nil {
		return "", fmt.Errorf("live2d model manifest file not found")
	}
	return targetPath, nil
}

// buildManifestExpressions 从 model3.json 提取表达式白名单。
func buildManifestExpressions(modelFilePath string, modelPayload live2DManifestModelPayload) []ai.Live2DManifestExpression {
	items := make([]ai.Live2DManifestExpression, 0, len(modelPayload.FileReferences.Expressions)+1)
	items = append(items, ai.Live2DManifestExpression{
		Key:   "neutral",
		Label: "Neutral",
	})

	seen := map[string]struct{}{
		"neutral": {},
	}
	for _, item := range modelPayload.FileReferences.Expressions {
		rawName := strings.TrimSpace(item.Name)
		if rawName == "" {
			rawName = strings.TrimSuffix(filepath.Base(strings.TrimSpace(item.File)), ".exp3.json")
		}
		key := normalizeManifestExpressionID(rawName)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, ai.Live2DManifestExpression{
			Key:   key,
			Label: formatManifestExpressionLabel(rawName),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Key == "neutral" {
			return true
		}
		if items[j].Key == "neutral" {
			return false
		}
		return strings.Compare(items[i].Label, items[j].Label) < 0
	})
	return items
}

// buildManifestParameters 合并 cdi3 和 vtube 信息，生成参数白名单。
func buildManifestParameters(modelFilePath string, modelPayload live2DManifestModelPayload) ([]ai.Live2DManifestParameter, error) {
	displayInfoPath := strings.TrimSpace(modelPayload.FileReferences.DisplayInfo)
	if displayInfoPath == "" {
		displayInfoPath = strings.TrimSuffix(modelFilePath, ".model3.json") + ".cdi3.json"
	} else {
		displayInfoPath = filepath.Join(filepath.Dir(modelFilePath), filepath.FromSlash(displayInfoPath))
	}
	vtubePath := strings.TrimSuffix(modelFilePath, ".model3.json") + ".vtube.json"

	displayInfo := live2DManifestDisplayInfoPayload{}
	if fileExists(displayInfoPath) {
		payload, err := readJSONFile[live2DManifestDisplayInfoPayload](displayInfoPath)
		if err != nil {
			return nil, fmt.Errorf("read live2d display info failed: %w", err)
		}
		displayInfo = payload
	}

	vtubeInfo := live2DManifestVtubePayload{}
	if fileExists(vtubePath) {
		payload, err := readJSONFile[live2DManifestVtubePayload](vtubePath)
		if err != nil {
			return nil, fmt.Errorf("read live2d vtube info failed: %w", err)
		}
		vtubeInfo = payload
	}

	type parameterMeta struct {
		ID    string
		Label string
		Min   float64
		Max   float64
	}
	items := make(map[string]parameterMeta)
	for _, item := range displayInfo.Parameters {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		items[id] = parameterMeta{
			ID:    id,
			Label: strings.TrimSpace(item.Name),
			Min:   fallbackManifestParameterRange(id).Min,
			Max:   fallbackManifestParameterRange(id).Max,
		}
	}

	for _, item := range vtubeInfo.ParameterSettings {
		id := strings.TrimSpace(item.OutputLive2D)
		if id == "" {
			continue
		}
		meta := items[id]
		meta.ID = id
		if strings.TrimSpace(meta.Label) == "" {
			meta.Label = strings.TrimSpace(item.Name)
		}
		meta.Min = minFloat(item.OutputRangeLower, item.OutputRangeUpper)
		meta.Max = maxFloat(item.OutputRangeLower, item.OutputRangeUpper)
		if meta.Min == 0 && meta.Max == 0 {
			fallback := fallbackManifestParameterRange(id)
			meta.Min = fallback.Min
			meta.Max = fallback.Max
		}
		items[id] = meta
	}

	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]ai.Live2DManifestParameter, 0, len(keys))
	for _, key := range keys {
		item := items[key]
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		result = append(result, ai.Live2DManifestParameter{
			ID:    item.ID,
			Label: strings.TrimSpace(item.Label),
			Min:   item.Min,
			Max:   item.Max,
		})
	}
	return result, nil
}

// buildManifestMotions 优先从 model3.json 的动作分组中解析动作，缺失时回退扫描目录中的 motion3 文件。
func buildManifestMotions(modelFilePath string, modelPayload live2DManifestModelPayload) ([]ai.Live2DManifestMotion, error) {
	items := make([]ai.Live2DManifestMotion, 0)
	seen := make(map[string]struct{})

	groupNames := make([]string, 0, len(modelPayload.FileReferences.Motions))
	for group := range modelPayload.FileReferences.Motions {
		groupNames = append(groupNames, group)
	}
	sort.Strings(groupNames)

	for _, groupName := range groupNames {
		definitions := modelPayload.FileReferences.Motions[groupName]
		for index, definition := range definitions {
			fileName := strings.TrimSpace(definition.File)
			if fileName == "" {
				continue
			}
			item := buildManifestMotionItem(modelFilePath, normalizeManifestMotionGroup(groupName), fileName, index)
			if item.Key == "" {
				continue
			}
			if _, exists := seen[item.Key]; exists {
				continue
			}
			seen[item.Key] = struct{}{}
			items = append(items, item)
		}
	}

	if len(items) > 0 {
		return items, nil
	}

	discovered, err := discoverManifestMotionsFromDirectory(modelFilePath)
	if err != nil {
		return nil, err
	}
	for _, item := range discovered {
		if item.Key == "" {
			continue
		}
		if _, exists := seen[item.Key]; exists {
			continue
		}
		seen[item.Key] = struct{}{}
		items = append(items, item)
	}
	return items, nil
}

// discoverManifestMotionsFromDirectory 在 model 同目录内回退发现 motion3 文件，兼容未在 model3.json 声明动作的资源包。
func discoverManifestMotionsFromDirectory(modelFilePath string) ([]ai.Live2DManifestMotion, error) {
	entries, err := os.ReadDir(filepath.Dir(modelFilePath))
	if err != nil {
		return nil, err
	}

	items := make([]ai.Live2DManifestMotion, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if !strings.HasSuffix(strings.ToLower(name), ".motion3.json") {
			continue
		}
		items = append(items, buildManifestMotionItem(modelFilePath, "auto", name, len(items)))
	}

	sort.SliceStable(items, func(i, j int) bool {
		return strings.Compare(items[i].Label, items[j].Label) < 0
	})
	return items, nil
}

// buildManifestMotionItem 把动作文件定义规整成后续指令与前端运行时都可复用的稳定 manifest 条目。
func buildManifestMotionItem(modelFilePath string, group string, motionFile string, index int) ai.Live2DManifestMotion {
	fileName := strings.TrimSpace(motionFile)
	if fileName == "" {
		return ai.Live2DManifestMotion{}
	}

	baseName := strings.TrimSuffix(filepath.Base(fileName), ".motion3.json")
	keySource := baseName
	if group != "" && group != "auto" {
		keySource = group + "_" + baseName
	}

	return ai.Live2DManifestMotion{
		Key:   normalizeManifestMotionKey(keySource, index),
		Group: group,
		File:  resolveManifestMotionURL(modelFilePath, fileName),
		Label: formatManifestMotionLabel(baseName),
	}
}

// resolveManifestMotionURL 把动作相对路径转换为前端可直接访问的受管 URL。
func resolveManifestMotionURL(modelFilePath string, motionFile string) string {
	modelURL := live2dassets.MountPath + "/" + filepath.ToSlash(strings.TrimPrefix(modelFilePath, live2dassets.ResolveAssetsDir()+string(filepath.Separator)))
	modelDirURL := filepath.ToSlash(filepath.Dir(modelURL))
	normalizedFile := strings.TrimPrefix(filepath.ToSlash(motionFile), "./")
	return modelDirURL + "/" + normalizedFile
}

// normalizeManifestMotionGroup 规整动作分组名，便于前后端统一匹配。
func normalizeManifestMotionGroup(raw string) string {
	return normalizeManifestMotionToken(raw)
}

// normalizeManifestMotionKey 规整动作键，必要时附加索引避免空键。
func normalizeManifestMotionKey(raw string, index int) string {
	normalized := normalizeManifestMotionToken(raw)
	if normalized != "" {
		return normalized
	}
	return fmt.Sprintf("motion_%d", index)
}

// normalizeManifestMotionToken 把动作名或分组名转换成稳定的小写 token。
func normalizeManifestMotionToken(raw string) string {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, ".", "_")
	normalized = strings.Trim(normalized, "_")
	return normalized
}

// formatManifestMotionLabel 把动作名整理成更适合提示词与面板展示的标签。
func formatManifestMotionLabel(raw string) string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "Motion"
	}
	return cleaned
}

// fallbackManifestParameterRange 为常见 Live2D 参数提供兜底范围。
func fallbackManifestParameterRange(parameterID string) ai.Live2DManifestParameter {
	switch strings.TrimSpace(parameterID) {
	case "ParamAngleX", "ParamAngleY", "ParamAngleZ":
		return ai.Live2DManifestParameter{Min: -30, Max: 30}
	case "ParamBodyAngleX", "ParamBodyAngleY", "ParamBodyAngleZ":
		return ai.Live2DManifestParameter{Min: -15, Max: 15}
	case "ParamEyeBallX", "ParamEyeBallY", "ParamBrowLY", "ParamBrowRY", "ParamBrowLForm", "ParamBrowRForm":
		return ai.Live2DManifestParameter{Min: -1, Max: 1}
	case "ParamEyeLOpen", "ParamEyeROpen", "ParamMouthOpenY", "ParamCheek":
		return ai.Live2DManifestParameter{Min: 0, Max: 1}
	case "ParamMouthForm":
		return ai.Live2DManifestParameter{Min: -1, Max: 1}
	default:
		return ai.Live2DManifestParameter{Min: -1, Max: 1}
	}
}

// normalizeManifestExpressionID 将表达式名称整理成稳定 ID。
func normalizeManifestExpressionID(raw string) string {
	normalized := strings.TrimSpace(strings.TrimSuffix(raw, ".exp3.json"))
	normalized = strings.ToLower(normalized)
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.Trim(normalized, "_")
	return normalized
}

// formatManifestExpressionLabel 将表达式名格式化为更可读的标签。
func formatManifestExpressionLabel(raw string) string {
	normalized := strings.TrimSpace(strings.TrimSuffix(raw, ".exp3.json"))
	if normalized == "" {
		return "Expression"
	}
	return normalized
}

// readJSONFile 按 UTF-8 读取并解析 JSON 文件。
func readJSONFile[T any](path string) (T, error) {
	var payload T
	raw, err := os.ReadFile(path)
	if err != nil {
		return payload, err
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

// fileExists 判断文件是否存在。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// minFloat 返回较小值。
func minFloat(left float64, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

// maxFloat 返回较大值。
func maxFloat(left float64, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
