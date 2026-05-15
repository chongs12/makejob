package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
	applogger "makejob-backend/pkg/logger"
)

// DebugRequest 定义管理端 AI 调试请求。
type DebugRequest struct {
	Scene            string            `json:"scene"`
	TaskID           *uint             `json:"task_id,omitempty"`
	IndustryID       *uint             `json:"industry_id,omitempty"`
	TemplateID       *uint             `json:"template_id,omitempty"`
	TemplateContent  string            `json:"template_content,omitempty"`
	Variables        map[string]string `json:"variables,omitempty"`
	RuntimeOverrides map[string]string `json:"runtime_overrides,omitempty"`
	RunModel         bool              `json:"run_model"`
	UserInput        string            `json:"user_input,omitempty"`
}

// DebugResponse 定义管理端 AI 调试响应。
type DebugResponse struct {
	TraceID          string            `json:"trace_id"`
	Scene            string            `json:"scene"`
	PromptSource     string            `json:"prompt_source"`
	SelectedPromptID *uint             `json:"selected_prompt_id,omitempty"`
	SelectedPrompt   string            `json:"selected_prompt_name,omitempty"`
	RenderedPrompt   string            `json:"rendered_prompt"`
	RuntimeConfig    map[string]string `json:"runtime_config"`
	SceneConfig      map[string]string `json:"scene_config"`
	RequestMessages  []ai.Message      `json:"request_messages,omitempty"`
	Provider         string            `json:"provider"`
	Model            string            `json:"model"`
	LatencyMS        int64             `json:"latency_ms"`
	ModelOutput      string            `json:"model_output,omitempty"`
	ModelError       string            `json:"model_error,omitempty"`
}

// Debugger 提供管理端运行时调试能力。
type Debugger struct {
	configRepo   repository.AdminConfigRepository
	promptRepo   repository.PromptTemplateRepository
	industryRepo repository.IndustryRepository
	baseConfig   map[string]string
}

// NewDebugger 创建运行时调试器。
func NewDebugger(
	configRepo repository.AdminConfigRepository,
	promptRepo repository.PromptTemplateRepository,
	industryRepo repository.IndustryRepository,
	baseConfig map[string]string,
) *Debugger {
	return &Debugger{
		configRepo:   configRepo,
		promptRepo:   promptRepo,
		industryRepo: industryRepo,
		baseConfig:   ai.NormalizeRuntimeConfig(baseConfig),
	}
}

// Run 执行一次管理端 AI 调试请求。
func (d *Debugger) Run(ctx context.Context, req DebugRequest) (*DebugResponse, error) {
	scene := normalizeDebugScene(req.Scene)
	if scene == "" {
		return nil, fmt.Errorf("invalid scene: %s", req.Scene)
	}

	traceID := uuid.NewString()
	builder := &Builder{
		configRepo:   d.configRepo,
		promptRepo:   d.promptRepo,
		industryRepo: d.industryRepo,
		baseConfig:   d.baseConfig,
	}
	runtimeConfig := builder.loadRuntimeConfig(ctx)
	runtimeConfig = applyDebugRuntimeOverrides(runtimeConfig, req.RuntimeOverrides)
	sceneConfig := buildSceneConfig(runtimeConfig, scene)

	prompts := &promptResolver{
		promptRepo:   d.promptRepo,
		industryRepo: d.industryRepo,
	}

	resolvedPrompt, err := d.resolvePromptDetails(ctx, prompts, scene, req)
	if err != nil {
		return nil, err
	}

	response := &DebugResponse{
		TraceID:          traceID,
		Scene:            scene,
		PromptSource:     resolvedPrompt.Source,
		SelectedPromptID: resolvedPrompt.TemplateID,
		SelectedPrompt:   resolvedPrompt.TemplateName,
		RenderedPrompt:   resolvedPrompt.Prompt,
		RuntimeConfig:    sanitizeDebugConfig(runtimeConfig),
		SceneConfig:      sanitizeDebugConfig(sceneConfig),
		Provider:         strings.TrimSpace(sceneConfig[ai.ConfigKeyProvider]),
		Model:            strings.TrimSpace(sceneConfig[ai.ConfigKeyModel]),
	}

	logFields := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("scene", scene),
		zap.String("prompt_source", response.PromptSource),
		zap.String("provider", response.Provider),
		zap.String("model", response.Model),
	}
	if req.TaskID != nil && *req.TaskID > 0 {
		logFields = append(logFields, zap.Uint("task_id", *req.TaskID))
	}

	if !req.RunModel {
		applogger.Info("admin ai debug completed without model run", logFields...)
		return response, nil
	}

	response.RequestMessages = buildDebugMessages(scene, resolvedPrompt.Prompt, req.UserInput)

	startedAt := time.Now()
	provider := buildProvider(ctx, sceneConfig)
	content, runErr := provider.Chat(ctx, response.RequestMessages)
	response.LatencyMS = time.Since(startedAt).Milliseconds()
	response.ModelOutput = strings.TrimSpace(content)
	if runErr != nil {
		response.ModelError = runErr.Error()
	}
	if traced, ok := provider.(interface{ LastExecutionMeta() providerExecutionMeta }); ok {
		meta := traced.LastExecutionMeta()
		if strings.TrimSpace(meta.Provider) != "" {
			response.Provider = strings.TrimSpace(meta.Provider)
		}
		if strings.TrimSpace(meta.Model) != "" {
			response.Model = strings.TrimSpace(meta.Model)
		}
	}

	logFields = append(logFields, zap.Int64("latency_ms", response.LatencyMS))
	if response.ModelError != "" {
		applogger.Warn("admin ai debug model run failed", append(logFields, zap.String("error", response.ModelError))...)
	} else {
		applogger.Info("admin ai debug model run completed", logFields...)
	}

	return response, nil
}

// resolvePromptDetails 解析调试请求对应的 prompt 内容与来源。
func (d *Debugger) resolvePromptDetails(ctx context.Context, prompts *promptResolver, scene string, req DebugRequest) (resolvedPromptDetails, error) {
	if strings.TrimSpace(req.TemplateContent) != "" {
		return resolvedPromptDetails{
			Prompt: renderPrompt(req.TemplateContent, req.Variables),
			Source: "template_custom",
		}, nil
	}

	if req.TemplateID != nil {
		if d.promptRepo == nil {
			return resolvedPromptDetails{}, fmt.Errorf("prompt repository is unavailable")
		}

		tpl, err := d.promptRepo.GetByID(ctx, *req.TemplateID)
		if err != nil {
			return resolvedPromptDetails{}, err
		}
		if tpl == nil {
			return resolvedPromptDetails{}, fmt.Errorf("prompt template not found")
		}

		if strings.TrimSpace(tpl.Scene) != "" && !strings.EqualFold(strings.TrimSpace(tpl.Scene), scene) {
			return resolvedPromptDetails{}, fmt.Errorf("prompt template scene mismatch")
		}

		return buildResolvedPromptDetails(tpl, "template_id", req.Variables), nil
	}

	return prompts.ResolveDetailsByIndustryID(ctx, scene, req.IndustryID, req.Variables), nil
}

// normalizeDebugScene 规范化调试场景名称。
func normalizeDebugScene(scene string) string {
	switch strings.ToLower(strings.TrimSpace(scene)) {
	case model.PromptSceneInterview:
		return model.PromptSceneInterview
	case model.PromptScenePlan:
		return model.PromptScenePlan
	case model.PromptSceneCompanion:
		return model.PromptSceneCompanion
	case model.PromptSceneQuiz:
		return model.PromptSceneQuiz
	default:
		return ""
	}
}

// sanitizeDebugConfig 对调试返回中的敏感配置进行脱敏。
func sanitizeDebugConfig(config map[string]string) map[string]string {
	result := make(map[string]string, len(config))
	for key, value := range config {
		if key == ai.ConfigKeyAPIKey && strings.TrimSpace(value) != "" {
			result[key] = maskSecret(value)
			continue
		}
		result[key] = value
	}
	return result
}

// buildDebugMessages 构造调试试跑时的消息列表。
func buildDebugMessages(scene string, prompt string, userInput string) []ai.Message {
	messages := make([]ai.Message, 0, 2)
	if strings.TrimSpace(prompt) != "" {
		messages = append(messages, ai.Message{
			Role:    "system",
			Content: strings.TrimSpace(prompt),
		})
	}

	messages = append(messages, ai.Message{
		Role:    "user",
		Content: defaultDebugUserInput(scene, userInput),
	})
	return messages
}

// defaultDebugUserInput 为不同场景补充默认试跑输入。
func defaultDebugUserInput(scene string, userInput string) string {
	if strings.TrimSpace(userInput) != "" {
		return strings.TrimSpace(userInput)
	}

	switch scene {
	case model.PromptSceneInterview:
		return "请生成一个用于调试的后端面试问题。"
	case model.PromptScenePlan:
		return "请生成一个简短的 Go 后端 7 天学习计划示例。"
	case model.PromptSceneCompanion:
		return "我今天学习有点累，帮我鼓励一下。"
	case model.PromptSceneQuiz:
		return "请示例性评估一个解题答案，并给出简短建议。"
	default:
		return "请返回一个调试用示例输出。"
	}
}

// applyDebugRuntimeOverrides 将调试请求中显式传入的 runtime 覆盖项合并进当前配置。
func applyDebugRuntimeOverrides(runtimeConfig map[string]string, overrides map[string]string) map[string]string {
	if len(overrides) == 0 {
		return runtimeConfig
	}

	merged := ai.NormalizeRuntimeConfig(runtimeConfig)
	for key, value := range normalizeExplicitDebugRuntimeOverrides(overrides) {
		merged[key] = value
	}
	return ai.NormalizeRuntimeConfig(merged)
}

// normalizeExplicitDebugRuntimeOverrides 仅规范化调试请求显式传入的覆盖键，避免默认值覆盖现有运行时配置。
func normalizeExplicitDebugRuntimeOverrides(overrides map[string]string) map[string]string {
	normalized := make(map[string]string, len(overrides))
	for rawKey, rawValue := range overrides {
		key := normalizeExplicitDebugRuntimeOverrideKey(rawKey)
		value := strings.TrimSpace(rawValue)
		if key == "" || value == "" {
			continue
		}
		normalized[key] = value
	}
	return normalized
}

// normalizeExplicitDebugRuntimeOverrideKey 将调试覆盖项键名收敛为 runtime 使用的标准配置键。
func normalizeExplicitDebugRuntimeOverrideKey(key string) string {
	trimmed := strings.TrimSpace(key)
	switch trimmed {
	case "provider":
		return ai.ConfigKeyProvider
	case "model_name":
		return ai.ConfigKeyModel
	case "api_key":
		return ai.ConfigKeyAPIKey
	case "base_url":
		return ai.ConfigKeyBaseURL
	case "temperature":
		return ai.ConfigKeyTemperature
	case "top_p":
		return ai.ConfigKeyTopP
	case "max_tokens":
		return ai.ConfigKeyMaxTokens
	case "interview_model":
		return ai.ConfigKeyInterviewModel
	case "plan_model":
		return ai.ConfigKeyPlanModel
	case "companion_model":
		return ai.ConfigKeyCompanionModel
	case "quiz_model":
		return ai.ConfigKeyQuizModel
	default:
		if ai.IsRuntimeConfigKey(trimmed) {
			return trimmed
		}
		return ""
	}
}

// maskSecret 对密钥类字段进行简单脱敏。
func maskSecret(secret string) string {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return "********"
	}
	return trimmed[:4] + "****" + trimmed[len(trimmed)-4:]
}
