package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	kratoserr "github.com/go-kratos/kratos/v2/errors"
	"gorm.io/gorm"

	"makejob/pkg/live2dassets"
)

const localLive2DModelKeyPrefix = "local:"

// CurrentLive2DModel 描述前台当前场景可直接消费的 Live2D 模型信息。
type CurrentLive2DModel struct {
	Name         string
	Scene        string
	IndustryCode string
	Path         string
	ModelURL     string
	ThumbnailURL string
	Config       map[string]interface{}
	Source       string
}

// SelectableLive2DModel 描述前台可用于切换的 Live2D 模型条目。
type SelectableLive2DModel struct {
	Key           string
	Name          string
	Scene         string
	ModelURL      string
	ThumbnailURL  string
	ConfigJSON    string
	Source        string
	MatchType     string
	IsGeneric     bool
	IsRecommended bool
	Motions       []*Live2DMotionInfo
}

// Live2DMotionInfo 描述前台模型可用的动作条目。
type Live2DMotionInfo struct {
	Key   string
	Group string
	File  string
	Label string
}

// ImportLive2DPackageResult 描述导入 Live2D 模型包后的自动识别结果。
type ImportLive2DPackageResult struct {
	Name         string
	AssetDir     string
	ModelURL     string
	ThumbnailURL string
	ModelID      uint64
	Created      bool
	IsActive     bool
}

// ImportLive2DBackgroundResult 描述导入舞台背景图后的静态资源结果。
type ImportLive2DBackgroundResult struct {
	FileName string
	AssetURL string
}

// localLive2DModel 描述从本地资源目录发现的一条可用模型。
type localLive2DModel struct {
	Key          string
	Name         string
	Scene        string
	AssetDir     string
	ModelURL     string
	ThumbnailURL string
}

type live2DManifestModelPayload struct {
	FileReferences struct {
		Motions map[string][]struct {
			File string `json:"File"`
		} `json:"Motions"`
	} `json:"FileReferences"`
}

// ListManagedLive2DModels 返回后台管理页所需的模型列表，并同步本地发现结果。
func (uc *AdminUseCase) ListManagedLive2DModels(ctx context.Context) ([]*Live2DModelRecord, error) {
	if err := uc.syncDiscoveredLive2DModels(ctx); err != nil {
		return nil, err
	}
	return uc.repo.ListLive2DModels(ctx)
}

// DeleteManagedLive2DModel 删除模型记录，并在无复用时清理对应受管资源目录。
func (uc *AdminUseCase) DeleteManagedLive2DModel(ctx context.Context, id uint64) error {
	models, err := uc.repo.ListLive2DModels(ctx)
	if err != nil {
		return err
	}

	var targetModel *Live2DModelRecord
	for _, item := range models {
		if item != nil && item.ID == id {
			targetModel = item
			break
		}
	}
	if targetModel == nil {
		return kratoserr.NotFound("LIVE2D_MODEL_NOT_FOUND", "live2d model not found")
	}

	if err := uc.repo.DeleteLive2DModel(ctx, id); err != nil {
		return err
	}
	return uc.deleteManagedLive2DAssetsIfUnused(models, targetModel)
}

// ListSelectableLive2DModels 返回当前场景下可供前台切换的 Live2D 模型列表。
func (uc *AdminUseCase) ListSelectableLive2DModels(ctx context.Context, scene string, industryCode string) ([]*SelectableLive2DModel, error) {
	normalizedScene, err := normalizeLive2DScene(scene)
	if err != nil {
		return nil, err
	}

	requestIndustryID, _, err := uc.findIndustryID(ctx, strings.TrimSpace(industryCode))
	if err != nil {
		return nil, err
	}

	models, err := uc.ListManagedLive2DModels(ctx)
	if err != nil {
		return nil, err
	}

	recommended := selectActiveLive2DModel(models, normalizedScene, requestIndustryID)
	items := buildSelectableDatabaseLive2DModels(models, normalizedScene, requestIndustryID, recommended)
	if len(items) > 0 {
		return items, nil
	}

	return buildSelectableLocalLive2DModels(normalizedScene)
}

// GetCurrentLive2DModel 返回当前场景最合适的 Live2D 模型信息。
func (uc *AdminUseCase) GetCurrentLive2DModel(ctx context.Context, scene string, industryCode string) (*CurrentLive2DModel, error) {
	normalizedScene, err := normalizeLive2DScene(scene)
	if err != nil {
		return nil, err
	}

	requestIndustryID, resolvedIndustryCode, err := uc.findIndustryID(ctx, strings.TrimSpace(industryCode))
	if err != nil {
		return nil, err
	}

	models, err := uc.ListManagedLive2DModels(ctx)
	if err != nil {
		return nil, err
	}
	if matched := selectActiveLive2DModel(models, normalizedScene, requestIndustryID); matched != nil {
		return buildDatabaseLive2DResponse(matched, normalizedScene, resolvedIndustryCode)
	}

	localModels, err := listLocalLive2DModels(normalizedScene)
	if err != nil {
		return nil, err
	}
	if len(localModels) > 0 {
		return buildLocalLive2DResponse(localModels[0], normalizedScene), nil
	}
	return nil, kratoserr.NotFound("LIVE2D_MODEL_NOT_FOUND", "live2d model not found")
}

// ImportLive2DPackage 导入 Live2D ZIP 包，并自动补录为待确认模型记录。
func (uc *AdminUseCase) ImportLive2DPackage(ctx context.Context, filename string, content []byte) (*ImportLive2DPackageResult, error) {
	importedPackage, err := live2dassets.ImportZip(filename, content)
	if err != nil {
		return nil, kratoserr.BadRequest("LIVE2D_IMPORT_FAILED", "导入 Live2D 模型包失败: "+err.Error())
	}

	importedModel, created, err := uc.ensureImportedLive2DModel(ctx, importedPackage)
	if err != nil {
		return nil, err
	}

	return &ImportLive2DPackageResult{
		Name:         importedPackage.Name,
		AssetDir:     importedPackage.AssetDir,
		ModelURL:     importedPackage.ModelURL,
		ThumbnailURL: importedPackage.ThumbnailURL,
		ModelID:      importedModel.ID,
		Created:      created,
		IsActive:     importedModel.IsActive,
	}, nil
}

// ImportLive2DBackground 导入舞台背景图，并返回可直接回填的静态地址。
func (uc *AdminUseCase) ImportLive2DBackground(_ context.Context, filename string, content []byte) (*ImportLive2DBackgroundResult, error) {
	importedBackground, err := live2dassets.ImportBackgroundImage(filename, content)
	if err != nil {
		return nil, kratoserr.BadRequest("LIVE2D_BACKGROUND_IMPORT_FAILED", "导入 Live2D 背景图失败: "+err.Error())
	}

	return &ImportLive2DBackgroundResult{
		FileName: importedBackground.FileName,
		AssetURL: importedBackground.AssetURL,
	}, nil
}

// syncDiscoveredLive2DModels 将本地资源目录中未入库的模型补登记到后台管理列表。
func (uc *AdminUseCase) syncDiscoveredLive2DModels(ctx context.Context) error {
	discoveredModels, err := live2dassets.DiscoverLocalModels()
	if err != nil {
		return kratoserr.InternalServer("LIVE2D_DISCOVER_FAILED", "扫描本地 Live2D 资源失败: "+err.Error())
	}

	existingModels, err := uc.repo.ListLive2DModels(ctx)
	if err != nil {
		return err
	}

	existingByModelURL := make(map[string]struct{}, len(existingModels))
	for _, existingModel := range existingModels {
		modelURL := normalizeLive2DAssetURL(existingModel.ModelURL)
		if modelURL == "" {
			continue
		}
		existingByModelURL[modelURL] = struct{}{}
	}

	for _, discoveredModel := range discoveredModels {
		modelURL := normalizeLive2DAssetURL(discoveredModel.ModelURL)
		if modelURL == "" {
			continue
		}
		if _, exists := existingByModelURL[modelURL]; exists {
			continue
		}

		newModel := buildPendingImportedLive2DModel(&discoveredModel)
		if err := uc.repo.CreateLive2DModel(ctx, newModel); err != nil {
			return err
		}
		existingByModelURL[modelURL] = struct{}{}
	}
	return nil
}

// ensureImportedLive2DModel 确保导入出来的模型资源已同步存在于后台模型表中。
func (uc *AdminUseCase) ensureImportedLive2DModel(ctx context.Context, importedPackage *live2dassets.ImportedPackage) (*Live2DModelRecord, bool, error) {
	if importedPackage == nil {
		return nil, false, kratoserr.BadRequest("LIVE2D_IMPORT_INVALID", "imported live2d package is required")
	}

	existingModels, err := uc.repo.ListLive2DModels(ctx)
	if err != nil {
		return nil, false, err
	}

	targetModelURL := normalizeLive2DAssetURL(importedPackage.ModelURL)
	for _, existingModel := range existingModels {
		if normalizeLive2DAssetURL(existingModel.ModelURL) == targetModelURL {
			return existingModel, false, nil
		}
	}

	newModel := buildPendingImportedLive2DModel(importedPackage)
	if err := uc.repo.CreateLive2DModel(ctx, newModel); err != nil {
		return nil, false, err
	}
	return newModel, true, nil
}

// deleteManagedLive2DAssetsIfUnused 在没有其他记录复用同一资源目录时删除对应本地目录。
func (uc *AdminUseCase) deleteManagedLive2DAssetsIfUnused(allModels []*Live2DModelRecord, targetModel *Live2DModelRecord) error {
	if targetModel == nil {
		return nil
	}

	managedAssetDir := live2dassets.ManagedModelAssetDirFromURL(targetModel.ModelURL)
	if managedAssetDir == "" {
		return nil
	}

	for _, currentModel := range allModels {
		if currentModel == nil || currentModel.ID == targetModel.ID {
			continue
		}
		if normalizeLive2DAssetURL(currentModel.ModelURL) == normalizeLive2DAssetURL(targetModel.ModelURL) {
			return nil
		}
		if live2dassets.ManagedModelAssetDirFromURL(currentModel.ModelURL) == managedAssetDir {
			return nil
		}
	}

	if err := live2dassets.DeleteManagedModelAssetDir(managedAssetDir); err != nil {
		return kratoserr.InternalServer("LIVE2D_ASSET_DELETE_FAILED", "删除 Live2D 模型资源失败: "+err.Error())
	}
	return nil
}

// buildPendingImportedLive2DModel 基于自动识别结果创建一条待后台确认的模型记录。
func buildPendingImportedLive2DModel(importedPackage *live2dassets.ImportedPackage) *Live2DModelRecord {
	return &Live2DModelRecord{
		Name:         strings.TrimSpace(importedPackage.Name),
		Scene:        "companion",
		ModelURL:     strings.TrimSpace(importedPackage.ModelURL),
		ThumbnailURL: strings.TrimSpace(importedPackage.ThumbnailURL),
		ConfigJSON:   "",
		IsActive:     false,
	}
}

// normalizeLive2DScene 规范并校验前台传入的 Live2D 场景参数。
func normalizeLive2DScene(scene string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(scene))
	switch normalized {
	case "interview", "companion":
		return normalized, nil
	default:
		return "", kratoserr.BadRequest("INVALID_LIVE2D_SCENE", "invalid live2d scene")
	}
}

// findIndustryID 解析行业编码，并返回数据库中的行业 ID 和规范化编码。
func (uc *AdminUseCase) findIndustryID(ctx context.Context, industryCode string) (uint64, string, error) {
	if industryCode == "" {
		return 0, "", nil
	}

	industry, err := uc.repo.GetIndustryByCode(ctx, industryCode)
	if err != nil {
		if err == gorm.ErrRecordNotFound || strings.Contains(strings.ToLower(err.Error()), "record not found") {
			return 0, "", nil
		}
		return 0, "", err
	}
	if industry == nil {
		return 0, "", nil
	}
	return industry.ID, strings.TrimSpace(industry.Code), nil
}

// selectActiveLive2DModel 按场景和行业优先级挑选当前应命中的激活模型。
func selectActiveLive2DModel(models []*Live2DModelRecord, scene string, industryID uint64) *Live2DModelRecord {
	var genericMatch *Live2DModelRecord

	for _, item := range models {
		if item == nil || !item.IsActive || item.Scene != scene {
			continue
		}
		if industryID > 0 && item.IndustryID == industryID {
			return item
		}
		if item.IndustryID == 0 && genericMatch == nil {
			genericMatch = item
		}
	}
	return genericMatch
}

// buildDatabaseLive2DResponse 组装数据库命中的当前模型响应。
func buildDatabaseLive2DResponse(model *Live2DModelRecord, scene string, industryCode string) (*CurrentLive2DModel, error) {
	config, err := parseLive2DConfig(model.ConfigJSON, scene)
	if err != nil {
		return nil, err
	}
	if model.TTSConfigID > 0 {
		config["tts_config_id"] = model.TTSConfigID
	}
	if model.IndustryID == 0 {
		industryCode = ""
	}

	return &CurrentLive2DModel{
		Name:         strings.TrimSpace(model.Name),
		Scene:        scene,
		IndustryCode: industryCode,
		Path:         strings.TrimSpace(model.ModelURL),
		ModelURL:     strings.TrimSpace(model.ModelURL),
		ThumbnailURL: strings.TrimSpace(model.ThumbnailURL),
		Config:       config,
		Source:       "database",
	}, nil
}

// buildSelectableDatabaseLive2DModels 组装数据库中的可切换模型列表，并按推荐优先级排序。
func buildSelectableDatabaseLive2DModels(models []*Live2DModelRecord, scene string, industryID uint64, recommended *Live2DModelRecord) []*SelectableLive2DModel {
	industryMatches := make([]*SelectableLive2DModel, 0)
	genericMatches := make([]*SelectableLive2DModel, 0)
	otherMatches := make([]*SelectableLive2DModel, 0)

	for _, item := range models {
		if item == nil || !item.IsActive || item.Scene != scene {
			continue
		}

		matchType := "other"
		switch {
		case industryID > 0 && item.IndustryID == industryID:
			matchType = "industry"
		case item.IndustryID == 0:
			matchType = "generic"
		}

		response := &SelectableLive2DModel{
			Key:           buildDatabaseLive2DModelKey(item.ID),
			Name:          strings.TrimSpace(item.Name),
			Scene:         scene,
			ModelURL:      strings.TrimSpace(item.ModelURL),
			ThumbnailURL:  strings.TrimSpace(item.ThumbnailURL),
			ConfigJSON:    strings.TrimSpace(item.ConfigJSON),
			Source:        "database",
			MatchType:     matchType,
			IsGeneric:     item.IndustryID == 0,
			IsRecommended: recommended != nil && item.ID == recommended.ID,
			Motions:       resolveSelectableLive2DModelMotions(item.ModelURL),
		}

		switch matchType {
		case "industry":
			industryMatches = append(industryMatches, response)
		case "generic":
			genericMatches = append(genericMatches, response)
		default:
			otherMatches = append(otherMatches, response)
		}
	}

	items := make([]*SelectableLive2DModel, 0, len(industryMatches)+len(genericMatches)+len(otherMatches))
	items = append(items, industryMatches...)
	items = append(items, genericMatches...)
	items = append(items, otherMatches...)
	return items
}

// buildLocalLive2DResponse 组装本地兜底命中的当前模型响应。
func buildLocalLive2DResponse(item localLive2DModel, scene string) *CurrentLive2DModel {
	return &CurrentLive2DModel{
		Name:         strings.TrimSpace(item.Name),
		Scene:        scene,
		IndustryCode: "",
		Path:         strings.TrimSpace(item.ModelURL),
		ModelURL:     strings.TrimSpace(item.ModelURL),
		ThumbnailURL: strings.TrimSpace(item.ThumbnailURL),
		Config:       defaultLive2DConfig(scene),
		Source:       "local",
	}
}

// buildSelectableLocalLive2DModels 组装当前场景可切换的本地兜底模型列表。
func buildSelectableLocalLive2DModels(scene string) ([]*SelectableLive2DModel, error) {
	localModels, err := listLocalLive2DModels(scene)
	if err != nil {
		return nil, err
	}

	items := make([]*SelectableLive2DModel, 0, len(localModels))
	for index, item := range localModels {
		items = append(items, &SelectableLive2DModel{
			Key:           item.Key,
			Name:          strings.TrimSpace(item.Name),
			Scene:         scene,
			ModelURL:      strings.TrimSpace(item.ModelURL),
			ThumbnailURL:  strings.TrimSpace(item.ThumbnailURL),
			Source:        "local",
			MatchType:     "generic",
			IsGeneric:     true,
			IsRecommended: index == 0,
			Motions:       resolveSelectableLive2DModelMotions(item.ModelURL),
		})
	}
	return items, nil
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

// buildLocalLive2DModelKey 为本地兜底模型生成稳定的前台选择键。
func buildLocalLive2DModelKey(assetDir string) string {
	return localLive2DModelKeyPrefix + strings.TrimSpace(assetDir)
}

// buildDatabaseLive2DModelKey 为数据库模型生成稳定的前台选择键。
func buildDatabaseLive2DModelKey(id uint64) string {
	return "db:" + strconv.FormatUint(id, 10)
}

// parseLive2DConfig 解析模型配置，并补齐场景级默认值。
func parseLive2DConfig(rawConfig string, scene string) (map[string]interface{}, error) {
	baseConfig := defaultLive2DConfig(scene)
	if strings.TrimSpace(rawConfig) == "" {
		return baseConfig, nil
	}

	var customConfig map[string]interface{}
	if err := json.Unmarshal([]byte(rawConfig), &customConfig); err != nil {
		return nil, kratoserr.BadRequest("INVALID_LIVE2D_CONFIG", "invalid live2d config_json")
	}

	for key, value := range customConfig {
		baseConfig[key] = value
	}
	return baseConfig, nil
}

// defaultLive2DConfig 返回场景级默认渲染配置。
func defaultLive2DConfig(scene string) map[string]interface{} {
	switch scene {
	case "interview":
		return map[string]interface{}{
			"scale":       0.34,
			"offset_x":    0.0,
			"offset_y":    0.02,
			"idle_motion": "interview_idle",
			"tap_motion":  "greeting",
			"background":  "transparent",
		}
	default:
		return map[string]interface{}{
			"scale":       0.4,
			"offset_x":    0.0,
			"offset_y":    0.08,
			"idle_motion": "companion_idle",
			"tap_motion":  "wave",
			"background":  "transparent",
		}
	}
}

// resolveSelectableLive2DModelMotions 解析当前模型的可用动作清单，供前台切换列表直接消费。
func resolveSelectableLive2DModelMotions(modelURL string) []*Live2DMotionInfo {
	motions, err := buildLive2DMotionsFromModelURL(modelURL)
	if err != nil {
		return nil
	}
	return motions
}

// buildLive2DMotionsFromModelURL 从模型 URL 解析前台所需的动作清单。
func buildLive2DMotionsFromModelURL(modelURL string) ([]*Live2DMotionInfo, error) {
	modelFilePath, err := resolveManagedModelFilePath(modelURL)
	if err != nil {
		return nil, err
	}

	modelPayload, err := readJSONFile[live2DManifestModelPayload](modelFilePath)
	if err != nil {
		return nil, fmt.Errorf("read live2d model manifest failed: %w", err)
	}

	items := make([]*Live2DMotionInfo, 0)
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
			if item == nil || item.Key == "" {
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
		if item == nil || item.Key == "" {
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

// resolveManagedModelFilePath 将受管静态 URL 映射到本地模型清单文件绝对路径。
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

// discoverManifestMotionsFromDirectory 在模型同目录内回退发现 motion3 文件。
func discoverManifestMotionsFromDirectory(modelFilePath string) ([]*Live2DMotionInfo, error) {
	entries, err := os.ReadDir(filepath.Dir(modelFilePath))
	if err != nil {
		return nil, err
	}

	items := make([]*Live2DMotionInfo, 0)
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

// buildManifestMotionItem 规整单条动作定义，供前端模型切换列表直接复用。
func buildManifestMotionItem(modelFilePath string, group string, motionFile string, index int) *Live2DMotionInfo {
	fileName := strings.TrimSpace(motionFile)
	if fileName == "" {
		return nil
	}

	baseName := strings.TrimSuffix(filepath.Base(fileName), ".motion3.json")
	keySource := baseName
	if group != "" && group != "auto" {
		keySource = group + "_" + baseName
	}

	return &Live2DMotionInfo{
		Key:   normalizeManifestMotionKey(keySource, index),
		Group: group,
		File:  resolveManifestMotionURL(modelFilePath, fileName),
		Label: formatManifestMotionLabel(baseName),
	}
}

// resolveManifestMotionURL 将动作相对路径转换为前端可直接访问的受管 URL。
func resolveManifestMotionURL(modelFilePath string, motionFile string) string {
	assetsRoot := live2dassets.ResolveAssetsDir()
	modelURL := live2dassets.MountPath + "/" + filepath.ToSlash(strings.TrimPrefix(modelFilePath, assetsRoot+string(filepath.Separator)))
	modelDirURL := filepath.ToSlash(filepath.Dir(modelURL))
	normalizedFile := strings.TrimPrefix(filepath.ToSlash(motionFile), "./")
	return modelDirURL + "/" + normalizedFile
}

// normalizeManifestMotionGroup 规整动作分组名，便于前后端统一匹配。
func normalizeManifestMotionGroup(raw string) string {
	return normalizeManifestMotionToken(raw)
}

// normalizeManifestMotionKey 规整动作键，必要时追加索引避免空键。
func normalizeManifestMotionKey(raw string, index int) string {
	normalized := normalizeManifestMotionToken(raw)
	if normalized != "" {
		return normalized
	}
	return fmt.Sprintf("motion_%d", index)
}

// normalizeManifestMotionToken 将动作名或分组名转换成稳定的小写 token。
func normalizeManifestMotionToken(raw string) string {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, ".", "_")
	normalized = strings.Trim(normalized, "_")
	return normalized
}

// formatManifestMotionLabel 将动作名整理成更适合展示的标签。
func formatManifestMotionLabel(raw string) string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "Motion"
	}
	return cleaned
}

// readJSONFile 读取并反序列化指定 JSON 文件。
func readJSONFile[T any](path string) (T, error) {
	var value T
	payload, err := os.ReadFile(path)
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(payload, &value); err != nil {
		return value, err
	}
	return value, nil
}

// normalizeLive2DAssetURL 统一清洗资源地址，避免因空格或斜杠差异导致重复落库。
func normalizeLive2DAssetURL(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
}
