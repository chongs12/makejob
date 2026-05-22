package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
	"makejob-backend/internal/tts"
	ttsfactory "makejob-backend/internal/tts/factory"
	ttsmimo "makejob-backend/internal/tts/xiaomi_mimo"
	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
)

const (
	// TTSDefaultConfigKeyInterview 表示面试场景默认 TTS 绑定配置键。
	TTSDefaultConfigKeyInterview = "tts_default_interview_config_id"
	// TTSDefaultConfigKeyCompanion 表示陪伴场景默认 TTS 绑定配置键。
	TTSDefaultConfigKeyCompanion = "tts_default_companion_config_id"
)

// SceneTTSRequest 描述按场景和 Live2D 模型选择 TTS 的合成请求。
type SceneTTSRequest struct {
	Scene          string
	Live2DModelKey string
	Text           string
	Format         string
}

// SceneTTSService 定义场景级 TTS 选择与合成能力。
type SceneTTSService interface {
	SynthesizeForScene(ctx context.Context, req SceneTTSRequest) (tts.SynthesizeResult, error)
}

// TTSProviderFieldDefinition 描述某个供应商配置项的前端展示元数据。
type TTSProviderFieldDefinition struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret,omitempty"`
}

// TTSProviderDescriptor 描述后台当前支持的 TTS 供应商目录项。
type TTSProviderDescriptor struct {
	Key            string                       `json:"key"`
	Label          string                       `json:"label"`
	Description    string                       `json:"description"`
	SupportStatus  string                       `json:"support_status"`
	SupportMessage string                       `json:"support_message"`
	AuthTemplate   string                       `json:"auth_template"`
	ParamsTemplate string                       `json:"params_template"`
	AuthFields     []TTSProviderFieldDefinition `json:"auth_fields"`
	ParamFields    []TTSProviderFieldDefinition `json:"param_fields"`
}

// TTSConfigItemResponse 描述后台页单条 TTS 配置记录及其运行状态。
type TTSConfigItemResponse struct {
	model.TTSConfig
	SupportStatus  string `json:"support_status"`
	SupportMessage string `json:"support_message"`
}

// TTSConfigListResponse 描述后台页初始化所需的 TTS 配置、供应商目录与场景默认绑定。
type TTSConfigListResponse struct {
	Configs         []TTSConfigItemResponse `json:"configs"`
	Providers       []TTSProviderDescriptor `json:"providers"`
	DefaultBindings map[string]uint         `json:"default_bindings"`
}

// UpdateTTSSceneDefaultsRequest 描述后台更新场景默认 TTS 绑定的请求体。
type UpdateTTSSceneDefaultsRequest struct {
	DefaultBindings map[string]uint `json:"default_bindings"`
}

// sceneTTSService 实现场景级 TTS 配置解析、回退和实际合成。
type sceneTTSService struct {
	ttsRepo          repository.TTSConfigRepository
	adminConfigRepo  repository.AdminConfigRepository
	live2DRepo       repository.Live2DModelRepository
	fallbackProvider tts.TTSProvider
}

// NewSceneTTSService 创建可按 Live2D 绑定和场景默认回退的 TTS 服务。
func NewSceneTTSService(
	ttsRepo repository.TTSConfigRepository,
	adminConfigRepo repository.AdminConfigRepository,
	live2DRepo repository.Live2DModelRepository,
	fallbackProvider tts.TTSProvider,
) SceneTTSService {
	return &sceneTTSService{
		ttsRepo:          ttsRepo,
		adminConfigRepo:  adminConfigRepo,
		live2DRepo:       live2DRepo,
		fallbackProvider: fallbackProvider,
	}
}

// SynthesizeForScene 根据 Live2D 绑定、场景默认和全局兜底顺序执行文本转语音。
func (s *sceneTTSService) SynthesizeForScene(ctx context.Context, req SceneTTSRequest) (tts.SynthesizeResult, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return tts.SynthesizeResult{}, fmt.Errorf("tts text is empty")
	}

	provider, configRecord, err := s.resolveProvider(ctx, strings.TrimSpace(req.Scene), strings.TrimSpace(req.Live2DModelKey))
	if err != nil {
		return tts.SynthesizeResult{}, err
	}
	if provider == nil {
		return tts.SynthesizeResult{}, fmt.Errorf("tts provider is not configured")
	}

	synthesizeReq := tts.SynthesizeRequest{
		Text:   text,
		Engine: resolveSceneTTSEngine(provider),
		Format: strings.TrimSpace(req.Format),
	}
	if configRecord != nil {
		synthesizeReq.VoiceID = strings.TrimSpace(configRecord.VoiceID)
	}
	return provider.Synthesize(ctx, synthesizeReq)
}

// resolveProvider 解析本次场景应使用的 TTS Provider，并返回命中的后台配置记录。
func (s *sceneTTSService) resolveProvider(ctx context.Context, scene string, modelKey string) (tts.TTSProvider, *model.TTSConfig, error) {
	normalizedScene, err := normalizeLive2DScene(scene)
	if err != nil {
		if scene == "" {
			normalizedScene = model.Live2DSceneCompanion
		} else {
			return nil, nil, err
		}
	}

	if record, err := s.findTTSConfigFromLive2DModel(ctx, normalizedScene, modelKey); err != nil {
		return nil, nil, err
	} else if record != nil {
		provider, buildErr := ttsfactory.NewTTSProviderFromConfigRecord(record)
		if buildErr != nil {
			return nil, record, buildErr
		}
		applogger.Info("scene tts resolved from live2d binding",
			zap.String("scene", normalizedScene),
			zap.String("live2d_model_key", modelKey),
			zap.Uint("tts_config_id", record.ID),
			zap.String("engine", strings.TrimSpace(record.Engine)),
		)
		return provider, record, nil
	}

	if record, err := s.findDefaultTTSConfigByScene(ctx, normalizedScene); err != nil {
		return nil, nil, err
	} else if record != nil {
		provider, buildErr := ttsfactory.NewTTSProviderFromConfigRecord(record)
		if buildErr != nil {
			return nil, record, buildErr
		}
		applogger.Info("scene tts resolved from scene default binding",
			zap.String("scene", normalizedScene),
			zap.Uint("tts_config_id", record.ID),
			zap.String("engine", strings.TrimSpace(record.Engine)),
		)
		return provider, record, nil
	}

	applogger.Warn("scene tts resolved to fallback provider",
		zap.String("scene", normalizedScene),
		zap.String("live2d_model_key", modelKey),
	)
	return s.fallbackProvider, nil, nil
}

// findTTSConfigFromLive2DModel 根据当前 Live2D 模型绑定关系查找 TTS 配置。
func (s *sceneTTSService) findTTSConfigFromLive2DModel(ctx context.Context, scene string, modelKey string) (*model.TTSConfig, error) {
	if s.live2DRepo == nil || strings.TrimSpace(modelKey) == "" {
		return nil, nil
	}

	modelID, err := parseDatabaseLive2DModelKey(modelKey)
	if err != nil {
		return nil, nil
	}
	live2dModel, err := s.live2DRepo.GetByID(ctx, modelID)
	if err != nil {
		return nil, err
	}
	if live2dModel == nil || !live2dModel.IsActive || strings.TrimSpace(live2dModel.Scene) != scene || live2dModel.TTSConfigID == nil || *live2dModel.TTSConfigID == 0 {
		return nil, nil
	}
	return s.loadActiveTTSConfig(ctx, *live2dModel.TTSConfigID)
}

// findDefaultTTSConfigByScene 根据后台场景默认绑定查找 TTS 配置。
func (s *sceneTTSService) findDefaultTTSConfigByScene(ctx context.Context, scene string) (*model.TTSConfig, error) {
	if s.adminConfigRepo == nil {
		return nil, nil
	}

	configKey := resolveSceneDefaultTTSConfigKey(scene)
	if configKey == "" {
		return nil, nil
	}
	item, err := s.adminConfigRepo.GetByKey(ctx, configKey)
	if err != nil || item == nil {
		return nil, err
	}

	configID, parseErr := parseUintString(item.ConfigValue)
	if parseErr != nil || configID == 0 {
		return nil, nil
	}
	return s.loadActiveTTSConfig(ctx, configID)
}

// loadActiveTTSConfig 加载并校验一条仍处于启用态的 TTS 配置。
func (s *sceneTTSService) loadActiveTTSConfig(ctx context.Context, configID uint) (*model.TTSConfig, error) {
	if s.ttsRepo == nil || configID == 0 {
		return nil, nil
	}

	record, err := s.ttsRepo.GetByID(ctx, configID)
	if err != nil {
		return nil, err
	}
	if record == nil || !record.IsActive {
		return nil, nil
	}
	return record, nil
}

// ListTTSProviderCatalog 返回后台可展示的 TTS 供应商目录定义。
func ListTTSProviderCatalog() []TTSProviderDescriptor {
	return []TTSProviderDescriptor{
		{
			Key:            model.TTSEngineVolcengine,
			Label:          "豆包语音 / 火山语音",
			Description:    "支持当前项目已验证可用的豆包语音 V3 单向流式接口，并兼容旧版火山 TTS 配置。",
			SupportStatus:  "ready",
			SupportMessage: "当前仓库已内置运行时适配，可直接用于真实播报。",
			AuthTemplate:   "{\n  \"api_key\": \"\",\n  \"base_url\": \"https://openspeech.bytedance.com/api/v3/tts/unidirectional\"\n}",
			ParamsTemplate: "{\n  \"resource_id\": \"seed-tts-2.0\",\n  \"voice_type\": \"zh_female_vv_uranus_bigtts\",\n  \"encoding\": \"mp3\",\n  \"sample_rate\": 24000,\n  \"speed_ratio\": 100,\n  \"volume_ratio\": 100,\n  \"pitch_ratio\": 100,\n  \"cluster\": \"\"\n}",
			AuthFields: []TTSProviderFieldDefinition{
				{Key: "api_key", Label: "API Key", Description: "新版豆包语音单 Key 鉴权。", Required: false, Secret: true},
				{Key: "app_id", Label: "App ID", Description: "旧版或双 Key 鉴权时的应用 ID。", Required: false},
				{Key: "access_token", Label: "Access Token", Description: "旧版或双 Key 鉴权时的访问密钥。", Required: false, Secret: true},
				{Key: "base_url", Label: "Base URL", Description: "默认建议使用 V3 单向流式地址。", Required: false},
			},
			ParamFields: []TTSProviderFieldDefinition{
				{Key: "resource_id", Label: "Resource ID", Description: "豆包语音资源标识，V3 推荐必填。", Required: false},
				{Key: "voice_type", Label: "Voice Type", Description: "音色 / 说话人 ID。", Required: true},
				{Key: "encoding", Label: "Encoding", Description: "输出格式，例如 mp3。", Required: false},
				{Key: "sample_rate", Label: "Sample Rate", Description: "输出采样率。", Required: false},
				{Key: "speed_ratio", Label: "Speed Ratio", Description: "语速百分比，100 表示默认。", Required: false},
				{Key: "volume_ratio", Label: "Volume Ratio", Description: "音量百分比，100 表示默认。", Required: false},
				{Key: "pitch_ratio", Label: "Pitch Ratio", Description: "音高百分比，100 表示默认。", Required: false},
				{Key: "cluster", Label: "Cluster", Description: "旧版火山接口的集群参数。", Required: false},
			},
		},
		{
			Key:            model.TTSEngineMinimax,
			Label:          "MiniMax TTS",
			Description:    "支持 MiniMax 官方 HTTP TTS 接口，兼容旧版 GroupId 查询参数，但当前模板默认按官方最新 API Key 方式填写。",
			SupportStatus:  "ready",
			SupportMessage: "当前仓库已内置运行时适配，可直接用于真实播报。",
			AuthTemplate:   "{\n  \"api_key\": \"\",\n  \"base_url\": \"https://api.minimax.io/v1/t2a_v2\"\n}",
			ParamsTemplate: "{\n  \"model\": \"speech-2.8-turbo\",\n  \"voice_id\": \"male-qn-jingying\",\n  \"emotion\": \"neutral\",\n  \"format\": \"mp3\",\n  \"sample_rate\": 32000,\n  \"bitrate\": 128000,\n  \"channel\": 1,\n  \"speed\": 1,\n  \"volume\": 1,\n  \"pitch\": 0,\n  \"subtitle_enable\": false,\n  \"output_format\": \"hex\",\n  \"timeout_seconds\": 45\n}",
			AuthFields: []TTSProviderFieldDefinition{
				{Key: "api_key", Label: "API Key", Description: "MiniMax 访问密钥。", Required: true, Secret: true},
				{Key: "base_url", Label: "Base URL", Description: "默认可留空或使用 https://api.minimax.io/v1/t2a_v2。", Required: false},
				{Key: "group_id", Label: "Group ID", Description: "旧版兼容参数；如仍在使用旧链路可选填。", Required: false},
			},
			ParamFields: []TTSProviderFieldDefinition{
				{Key: "model", Label: "Model", Description: "TTS 模型名。", Required: false},
				{Key: "voice_id", Label: "Voice ID", Description: "MiniMax 音色 ID。", Required: true},
				{Key: "emotion", Label: "Emotion", Description: "情绪参数。", Required: false},
				{Key: "format", Label: "Format", Description: "输出格式。", Required: false},
				{Key: "sample_rate", Label: "Sample Rate", Description: "输出采样率。", Required: false},
				{Key: "bitrate", Label: "Bitrate", Description: "输出码率。", Required: false},
				{Key: "channel", Label: "Channel", Description: "声道数。", Required: false},
				{Key: "speed", Label: "Speed", Description: "语速倍率。", Required: false},
				{Key: "volume", Label: "Volume", Description: "音量倍率。", Required: false},
				{Key: "pitch", Label: "Pitch", Description: "音高偏移。", Required: false},
				{Key: "subtitle_enable", Label: "Subtitle Enable", Description: "是否返回字幕信息。", Required: false},
				{Key: "output_format", Label: "Output Format", Description: "建议使用 hex 或 url。", Required: false},
				{Key: "timeout_seconds", Label: "Timeout Seconds", Description: "请求超时时间。", Required: false},
			},
		},
		{
			Key:            model.TTSEngineXiaomiMIMO,
			Label:          "Xiaomi MiMo",
			Description:    "支持 Xiaomi MiMo 官方 OpenAI 风格语音合成接口，当前仅接入 mimo-v2-tts 与 mimo-v2.5-tts 两个模型。",
			SupportStatus:  "ready",
			SupportMessage: "当前仓库已内置运行时适配，可直接绑定到 Live2D 使用。上方 Voice ID 字段会映射到官方 audio.voice 参数。",
			AuthTemplate:   "{\n  \"api_key\": \"\",\n  \"base_url\": \"https://api.xiaomimimo.com/v1/chat/completions\"\n}",
			ParamsTemplate: "{\n  \"model\": \"mimo-v2.5-tts\",\n  \"format\": \"wav\",\n  \"temperature\": 0.6,\n  \"max_completion_tokens\": 2048,\n  \"timeout_seconds\": 45\n}",
			AuthFields: []TTSProviderFieldDefinition{
				{Key: "api_key", Label: "API Key", Description: "MiMo 平台控制台签发的 API Key，请放在请求头 api-key 中使用。", Required: true, Secret: true},
				{Key: "base_url", Label: "Base URL", Description: "默认官方地址为 https://api.xiaomimimo.com/v1/chat/completions。", Required: false},
			},
			ParamFields: []TTSProviderFieldDefinition{
				{Key: "model", Label: "Model", Description: "当前仅支持 mimo-v2-tts 或 mimo-v2.5-tts。", Required: true},
				{Key: "format", Label: "Format", Description: "官方输出格式，当前实现支持 wav / mp3 / pcm16。", Required: false},
				{Key: "temperature", Label: "Temperature", Description: "OpenAI 风格采样温度，留空时回退到默认值。", Required: false},
				{Key: "max_completion_tokens", Label: "Max Completion Tokens", Description: "最大输出 token 数，留空时回退到实现默认值。", Required: false},
				{Key: "timeout_seconds", Label: "Timeout Seconds", Description: "HTTP 请求超时秒数。", Required: false},
			},
		},
	}
}

// BuildTTSConfigListResponse 组装后台页需要的 TTS 列表响应。
func BuildTTSConfigListResponse(ctx context.Context, configs []model.TTSConfig, adminConfigRepo repository.AdminConfigRepository) (*TTSConfigListResponse, error) {
	items := make([]TTSConfigItemResponse, 0, len(configs))
	for _, item := range configs {
		status, message := EvaluateTTSConfigSupport(item)
		items = append(items, TTSConfigItemResponse{
			TTSConfig:      item,
			SupportStatus:  status,
			SupportMessage: message,
		})
	}

	defaultBindings := map[string]uint{}
	for _, scene := range []string{model.Live2DSceneInterview, model.Live2DSceneCompanion} {
		configKey := resolveSceneDefaultTTSConfigKey(scene)
		if configKey == "" || adminConfigRepo == nil {
			continue
		}
		item, err := adminConfigRepo.GetByKey(ctx, configKey)
		if err != nil {
			return nil, err
		}
		if item == nil {
			continue
		}
		configID, parseErr := parseUintString(item.ConfigValue)
		if parseErr == nil && configID > 0 {
			defaultBindings[scene] = configID
		}
	}

	return &TTSConfigListResponse{
		Configs:         items,
		Providers:       ListTTSProviderCatalog(),
		DefaultBindings: defaultBindings,
	}, nil
}

// EvaluateTTSConfigSupport 判断某条 TTS 配置当前是否可运行，并返回后台展示说明。
func EvaluateTTSConfigSupport(item model.TTSConfig) (string, string) {
	engine := strings.TrimSpace(item.Engine)
	switch engine {
	case model.TTSEngineVolcengine, model.TTSEngineMinimax, model.TTSEngineXiaomiMIMO:
		if err := ValidateTTSConfigRecord(item); err != nil {
			return "invalid", err.Error()
		}
		return "ready", "参数校验通过，可直接绑定到 Live2D 使用。"
	default:
		return "legacy_unsupported", "这是旧版引擎记录；当前仓库没有对应运行时适配，不能作为新的可用配置。"
	}
}

// ValidateTTSConfigInput 校验后台提交的 TTS 配置请求，并统一标准化 JSON 文本。
func ValidateTTSConfigInput(engine string, voiceID string, authConfigJSON string, paramsJSON string) (string, string, error) {
	normalizedEngine := strings.TrimSpace(engine)
	if normalizedEngine == "" {
		return "", "", fmt.Errorf("tts engine is required")
	}

	switch normalizedEngine {
	case model.TTSEngineVolcengine, model.TTSEngineMinimax, model.TTSEngineXiaomiMIMO:
	default:
		return "", "", fmt.Errorf("unsupported tts engine: %s", normalizedEngine)
	}

	if strings.TrimSpace(voiceID) == "" {
		return "", "", fmt.Errorf("voice_id is required")
	}

	authMap, err := normalizeJSONObjectString(authConfigJSON)
	if err != nil {
		return "", "", fmt.Errorf("invalid auth_config_json: %w", err)
	}
	paramsMap, err := normalizeJSONObjectString(paramsJSON)
	if err != nil {
		return "", "", fmt.Errorf("invalid params_json: %w", err)
	}

	switch normalizedEngine {
	case model.TTSEngineVolcengine:
		if !hasNonEmptyValue(authMap, "api_key") && !(hasNonEmptyValue(authMap, "app_id") && hasNonEmptyValue(authMap, "access_token")) {
			return "", "", fmt.Errorf("volcengine requires api_key or app_id + access_token")
		}
	case model.TTSEngineMinimax:
		if !hasNonEmptyValue(authMap, "api_key") {
			return "", "", fmt.Errorf("minimax requires api_key")
		}
	case model.TTSEngineXiaomiMIMO:
		if !hasNonEmptyValue(authMap, "api_key") {
			return "", "", fmt.Errorf("xiaomi_mimo requires api_key")
		}
		modelName := ttsmimo.NormalizeModel(getStringValue(paramsMap, "model"))
		if !ttsmimo.IsSupportedModel(modelName) {
			return "", "", fmt.Errorf("xiaomi_mimo only supports models: %s", strings.Join(ttsmimo.SupportedModels(), ", "))
		}
		if !ttsmimo.IsSupportedVoice(modelName, strings.TrimSpace(voiceID)) {
			return "", "", fmt.Errorf("xiaomi_mimo model %s does not support voice_id %s", modelName, strings.TrimSpace(voiceID))
		}
		paramsMap["model"] = modelName
		paramsMap["format"] = ttsmimo.NormalizeFormat(getStringValue(paramsMap, "format"))
	}

	normalizedAuth, err := marshalNormalizedJSONMap(authMap)
	if err != nil {
		return "", "", err
	}
	normalizedParams, err := marshalNormalizedJSONMap(paramsMap)
	if err != nil {
		return "", "", err
	}
	return normalizedAuth, normalizedParams, nil
}

// ValidateTTSConfigRecord 校验单条已落库的 TTS 配置记录是否满足当前运行时要求。
func ValidateTTSConfigRecord(item model.TTSConfig) error {
	_, _, err := ValidateTTSConfigInput(item.Engine, item.VoiceID, item.AuthConfigJSON, item.ParamsJSON)
	return err
}

// BuildTTSDefaultSceneConfigs 生成后台场景默认绑定需要写入的配置记录。
func BuildTTSDefaultSceneConfigs(bindings map[string]uint) ([]model.AdminConfig, error) {
	items := make([]model.AdminConfig, 0, 2)
	for _, scene := range []string{model.Live2DSceneInterview, model.Live2DSceneCompanion} {
		configKey := resolveSceneDefaultTTSConfigKey(scene)
		if configKey == "" {
			continue
		}

		value := uint(0)
		if bindings != nil {
			value = bindings[scene]
		}
		items = append(items, model.AdminConfig{
			ConfigKey:   configKey,
			ConfigValue: strconv.FormatUint(uint64(value), 10),
			ConfigType:  model.ConfigTypeString,
			Description: fmt.Sprintf("%s 场景默认 TTS 配置 ID", scene),
		})
	}
	return items, nil
}

// resolveSceneDefaultTTSConfigKey 返回指定场景对应的默认 TTS 配置键名。
func resolveSceneDefaultTTSConfigKey(scene string) string {
	switch strings.TrimSpace(scene) {
	case model.Live2DSceneInterview:
		return TTSDefaultConfigKeyInterview
	case model.Live2DSceneCompanion:
		return TTSDefaultConfigKeyCompanion
	default:
		return ""
	}
}

// normalizeJSONObjectString 把原始 JSON 文本解析为对象，并兼容空串。
func normalizeJSONObjectString(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return map[string]any{}, nil
	}
	return payload, nil
}

// marshalNormalizedJSONMap 以稳定格式输出对象 JSON，便于后台再次回填编辑。
func marshalNormalizedJSONMap(payload map[string]any) (string, error) {
	if len(payload) == 0 {
		return "{}", nil
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// hasNonEmptyValue 判断对象中指定字段是否为非空字符串。
func hasNonEmptyValue(payload map[string]any, key string) bool {
	value, ok := payload[key]
	if !ok {
		return false
	}
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

// getStringValue 兼容读取对象中的字符串或可字符串化字段。
func getStringValue(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

// parseUintString 把字符串配置值解析为正整数 ID。
func parseUintString(raw string) (uint, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(parsed), nil
}

// resolveSceneTTSEngine 返回场景级 TTS Provider 的首选引擎标识。
func resolveSceneTTSEngine(provider tts.TTSProvider) string {
	if provider == nil {
		return ""
	}

	supportedEngines := provider.GetSupportedEngines()
	if len(supportedEngines) == 0 {
		return ""
	}
	return strings.TrimSpace(supportedEngines[0])
}

// firstNonEmpty 返回传入列表中的第一个非空字符串。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
