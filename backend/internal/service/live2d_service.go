package service

import (
	"context"
	"encoding/json"
	"strings"

	"makejob-backend/internal/common"
	"makejob-backend/internal/live2dassets"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

const (
	bundledLive2DName          = "Ariu"
	bundledLive2DRelativeModel = "ariu/ariu.model3.json"
	bundledLive2DThumbnail     = "ariu/ariu.png"
)

// CurrentLive2DModelRequest 描述前台查询当前场景模型的请求。
type CurrentLive2DModelRequest struct {
	Scene        string
	IndustryCode string
}

// CurrentLive2DModelResponse 描述前台可直接消费的 Live2D 模型信息。
type CurrentLive2DModelResponse struct {
	Name         string                 `json:"name"`
	Scene        string                 `json:"scene"`
	IndustryCode string                 `json:"industry_code"`
	Path         string                 `json:"path"`
	ModelURL     string                 `json:"model_url"`
	ThumbnailURL string                 `json:"thumbnail_url"`
	Config       map[string]interface{} `json:"config"`
	Source       string                 `json:"source"`
}

// Live2DService 定义前台 Live2D 模型查询能力。
type Live2DService interface {
	GetCurrentModel(ctx context.Context, req *CurrentLive2DModelRequest) (*CurrentLive2DModelResponse, error)
}

// live2dService 实现前台 Live2D 模型选择逻辑。
type live2dService struct {
	live2DRepo   repository.Live2DModelRepository
	industryRepo repository.IndustryRepository
}

// NewLive2DService 创建前台 Live2D 服务。
func NewLive2DService(live2DRepo repository.Live2DModelRepository, industryRepo repository.IndustryRepository) Live2DService {
	return &live2dService{
		live2DRepo:   live2DRepo,
		industryRepo: industryRepo,
	}
}

// GetCurrentModel 返回当前场景最合适的 Live2D 模型。
func (s *live2dService) GetCurrentModel(ctx context.Context, req *CurrentLive2DModelRequest) (*CurrentLive2DModelResponse, error) {
	if req == nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "live2d request is required")
	}

	scene, err := normalizeLive2DScene(req.Scene)
	if err != nil {
		return nil, err
	}

	requestIndustryCode := strings.TrimSpace(req.IndustryCode)
	requestIndustryID, resolvedIndustryCode, err := s.findIndustryID(ctx, requestIndustryCode)
	if err != nil {
		return nil, err
	}

	if s.live2DRepo != nil {
		models, err := s.live2DRepo.List(ctx)
		if err != nil {
			return nil, err
		}

		if matched := selectActiveLive2DModel(models, scene, requestIndustryID); matched != nil {
			return buildDatabaseLive2DResponse(matched, scene, resolvedIndustryCode)
		}
	}

	return buildBundledLive2DResponse(scene)
}

// normalizeLive2DScene 规范并校验场景参数。
func normalizeLive2DScene(scene string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(scene))
	switch normalized {
	case model.Live2DSceneInterview, model.Live2DSceneCompanion:
		return normalized, nil
	default:
		return "", common.NewBusinessError(common.CodeBadRequest, "invalid live2d scene")
	}
}

// findIndustryID 解析行业编码并返回数据库中的行业 ID。
func (s *live2dService) findIndustryID(ctx context.Context, industryCode string) (uint, string, error) {
	if industryCode == "" || s.industryRepo == nil {
		return 0, "", nil
	}

	industry, err := s.industryRepo.GetByCode(ctx, industryCode)
	if err != nil {
		return 0, "", err
	}
	if industry == nil {
		return 0, "", nil
	}

	return industry.ID, industry.Code, nil
}

// selectActiveLive2DModel 按场景和行业优先级挑选激活模型。
func selectActiveLive2DModel(models []model.Live2DModel, scene string, industryID uint) *model.Live2DModel {
	var genericMatch *model.Live2DModel

	for i := range models {
		item := &models[i]
		if !item.IsActive || item.Scene != scene {
			continue
		}
		if industryID > 0 && item.IndustryID == industryID {
			return item
		}
		if item.IsGeneric() && genericMatch == nil {
			genericMatch = item
		}
	}

	return genericMatch
}

// buildDatabaseLive2DResponse 组装数据库命中的模型响应。
func buildDatabaseLive2DResponse(m *model.Live2DModel, scene, industryCode string) (*CurrentLive2DModelResponse, error) {
	config, err := parseLive2DConfig(m.ConfigJSON, scene)
	if err != nil {
		return nil, err
	}
	if m.IsGeneric() {
		industryCode = ""
	}

	return &CurrentLive2DModelResponse{
		Name:         m.Name,
		Scene:        scene,
		IndustryCode: industryCode,
		Path:         strings.TrimSpace(m.ModelURL),
		ModelURL:     strings.TrimSpace(m.ModelURL),
		ThumbnailURL: strings.TrimSpace(m.ThumbnailURL),
		Config:       config,
		Source:       "database",
	}, nil
}

// buildBundledLive2DResponse 组装内置模型回退响应。
func buildBundledLive2DResponse(scene string) (*CurrentLive2DModelResponse, error) {
	if !live2dassets.HasAsset(bundledLive2DRelativeModel) {
		return nil, common.NewBusinessError(common.CodeNotFound, "live2d model not found")
	}

	thumbnailURL := ""
	if live2dassets.HasAsset(bundledLive2DThumbnail) {
		thumbnailURL = live2dassets.AssetURL(bundledLive2DThumbnail)
	}

	return &CurrentLive2DModelResponse{
		Name:         bundledLive2DName,
		Scene:        scene,
		IndustryCode: "",
		Path:         live2dassets.AssetURL(bundledLive2DRelativeModel),
		ModelURL:     live2dassets.AssetURL(bundledLive2DRelativeModel),
		ThumbnailURL: thumbnailURL,
		Config:       defaultLive2DConfig(scene),
		Source:       "bundled",
	}, nil
}

// parseLive2DConfig 解析模型配置并补齐默认值。
func parseLive2DConfig(rawConfig string, scene string) (map[string]interface{}, error) {
	baseConfig := defaultLive2DConfig(scene)
	if strings.TrimSpace(rawConfig) == "" {
		return baseConfig, nil
	}

	var customConfig map[string]interface{}
	if err := json.Unmarshal([]byte(rawConfig), &customConfig); err != nil {
		return nil, common.NewBusinessError(common.CodeBadRequest, "invalid live2d config_json")
	}

	for key, value := range customConfig {
		baseConfig[key] = value
	}
	return baseConfig, nil
}

// defaultLive2DConfig 返回场景级默认渲染配置。
func defaultLive2DConfig(scene string) map[string]interface{} {
	switch scene {
	case model.Live2DSceneInterview:
		return map[string]interface{}{
			"scale":        0.34,
			"offset_x":     0.0,
			"offset_y":     0.02,
			"idle_motion":  "interview_idle",
			"tap_motion":   "greeting",
			"background":   "transparent",
			"voice_source": "volcengine",
		}
	default:
		return map[string]interface{}{
			"scale":        0.4,
			"offset_x":     0.0,
			"offset_y":     0.08,
			"idle_motion":  "companion_idle",
			"tap_motion":   "wave",
			"background":   "transparent",
			"voice_source": "volcengine",
		}
	}
}
