package biz

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	kratosErr "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// SceneTTSRequest 场景 TTS 合成请求
type SceneTTSRequest struct {
	Scene          string // "interview" 或 "companion"
	Live2DModelKey string // "db:123" 格式的模型标识
	Text           string // 待合成文本
}

// SceneTTSService 场景级 TTS 服务接口（对齐单体 service.SceneTTSService）
type SceneTTSService interface {
	// SynthesizeForScene 根据场景和 Live2D 模型选择 TTS 配置并合成语音
	SynthesizeForScene(ctx context.Context, req SceneTTSRequest) (*TTSResult, error)
}

// sceneTTSService 实现场景级 TTS 选择与合成
type sceneTTSService struct {
	ttsConfigRepo    TTSConfigRepo
	live2DModelRepo  Live2DModelRepo
	adminConfigRepo  AdminConfigRepo
	fallbackProvider TTSProvider
	providerFactory  func(*TTSConfig) (TTSProvider, error)
}

// NewSceneTTSService 创建场景级 TTS 服务
func NewSceneTTSService(
	ttsConfigRepo TTSConfigRepo,
	live2DModelRepo Live2DModelRepo,
	adminConfigRepo AdminConfigRepo,
	fallbackProvider TTSProvider,
	providerFactory func(*TTSConfig) (TTSProvider, error),
) SceneTTSService {
	return &sceneTTSService{
		ttsConfigRepo:    ttsConfigRepo,
		live2DModelRepo:  live2DModelRepo,
		adminConfigRepo:  adminConfigRepo,
		fallbackProvider: fallbackProvider,
		providerFactory:  providerFactory,
	}
}

// SynthesizeForScene 根据场景和 Live2D 模型选择 TTS 配置并合成语音
func (s *sceneTTSService) SynthesizeForScene(ctx context.Context, req SceneTTSRequest) (*TTSResult, error) {
	if strings.TrimSpace(req.Text) == "" {
		return &TTSResult{}, nil
	}

	provider, configRecord, err := s.resolveProvider(ctx, req.Scene, req.Live2DModelKey)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, kratosErr.InternalServer("TTS_PROVIDER_NOT_FOUND", "未找到可用的 TTS 供应商")
	}

	result, err := provider.Synthesize(ctx, TTSRequest{
		Text:    req.Text,
		VoiceID: getVoiceIDFromConfig(configRecord),
	})
	if err != nil {
		// 如果场景级 Provider 失败，尝试 fallback
		if s.fallbackProvider != nil && configRecord != nil {
			log.Context(ctx).Warnf("scene tts failed, fallback to global provider: %v", err)
			return s.fallbackProvider.Synthesize(ctx, TTSRequest{Text: req.Text})
		}
		return nil, err
	}

	return result, nil
}

// resolveProvider 三级回退解析 TTS Provider
func (s *sceneTTSService) resolveProvider(ctx context.Context, scene, modelKey string) (TTSProvider, *TTSConfig, error) {
	normalizedScene := normalizeScene(scene)

	// 第一级：Live2D 模型绑定
	config, err := s.findTTSConfigFromLive2DModel(ctx, normalizedScene, modelKey)
	if err != nil {
		log.Context(ctx).Warnf("find tts config from live2d model failed: %v", err)
	}
	if config != nil {
		provider, err := s.buildProvider(config)
		if err == nil && provider != nil {
			return provider, config, nil
		}
	}

	// 第二级：场景默认绑定
	config, err = s.findDefaultTTSConfigByScene(ctx, normalizedScene)
	if err != nil {
		log.Context(ctx).Warnf("find default tts config by scene failed: %v", err)
	}
	if config != nil {
		provider, err := s.buildProvider(config)
		if err == nil && provider != nil {
			return provider, config, nil
		}
	}

	// 第三级：全局 fallback
	log.Context(ctx).Warnf("scene tts resolved to fallback provider, scene=%s modelKey=%s", scene, modelKey)
	return s.fallbackProvider, nil, nil
}

// findTTSConfigFromLive2DModel 第一级：从 Live2D 模型绑定获取 TTS 配置
func (s *sceneTTSService) findTTSConfigFromLive2DModel(ctx context.Context, scene, modelKey string) (*TTSConfig, error) {
	if s.live2DModelRepo == nil || strings.TrimSpace(modelKey) == "" {
		return nil, nil
	}

	model, err := s.live2DModelRepo.GetByKey(ctx, modelKey)
	if err != nil || model == nil {
		return nil, nil
	}

	// 校验：模型必须启用、场景匹配、且绑定了 TTS 配置
	if !model.IsActive || model.Scene != scene || model.TTSConfigID == nil || *model.TTSConfigID == 0 {
		return nil, nil
	}

	return s.loadActiveTTSConfig(ctx, *model.TTSConfigID)
}

// findDefaultTTSConfigByScene 第二级：从场景默认配置获取 TTS 配置
func (s *sceneTTSService) findDefaultTTSConfigByScene(ctx context.Context, scene string) (*TTSConfig, error) {
	if s.adminConfigRepo == nil {
		return nil, nil
	}

	configKey := resolveSceneDefaultTTSConfigKey(scene)
	if configKey == "" {
		return nil, nil
	}

	item, err := s.adminConfigRepo.GetByKey(ctx, configKey)
	if err != nil || item == nil || strings.TrimSpace(item.ConfigValue) == "" {
		return nil, nil
	}

	configID, err := parseUintString(item.ConfigValue)
	if err != nil || configID == 0 {
		return nil, nil
	}

	return s.loadActiveTTSConfig(ctx, configID)
}

// loadActiveTTSConfig 加载并校验 TTS 配置
func (s *sceneTTSService) loadActiveTTSConfig(ctx context.Context, configID uint) (*TTSConfig, error) {
	if s.ttsConfigRepo == nil || configID == 0 {
		return nil, nil
	}

	record, err := s.ttsConfigRepo.GetByID(ctx, configID)
	if err != nil || record == nil || !record.IsActive {
		return nil, nil
	}

	return record, nil
}

// buildProvider 从数据库记录构建 TTS Provider
func (s *sceneTTSService) buildProvider(config *TTSConfig) (TTSProvider, error) {
	if s.providerFactory == nil {
		return nil, fmt.Errorf("provider factory not configured")
	}
	return s.providerFactory(config)
}

// normalizeScene 标准化场景名
func normalizeScene(scene string) string {
	switch strings.ToLower(strings.TrimSpace(scene)) {
	case Live2DSceneInterview:
		return Live2DSceneInterview
	case Live2DSceneCompanion:
		return Live2DSceneCompanion
	default:
		return Live2DSceneCompanion
	}
}

// resolveSceneDefaultTTSConfigKey 根据场景返回配置键名
func resolveSceneDefaultTTSConfigKey(scene string) string {
	switch scene {
	case Live2DSceneInterview:
		return TTSDefaultConfigKeyInterview
	case Live2DSceneCompanion:
		return TTSDefaultConfigKeyCompanion
	default:
		return ""
	}
}

// parseUintString 解析 uint 字符串
func parseUintString(s string) (uint, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}

// getVoiceIDFromConfig 从配置记录获取音色 ID
func getVoiceIDFromConfig(config *TTSConfig) string {
	if config == nil {
		return ""
	}
	return config.VoiceID
}
