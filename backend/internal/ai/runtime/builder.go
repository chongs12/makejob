package runtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/ai/eino"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

var builtInScenePrompts = map[string]string{
	model.PromptSceneInterview: "You are a strict but constructive technical interviewer for {{industry_code}} roles. Keep questions progressive, clear, and grounded in the user's difficulty {{difficulty}}.",
	model.PromptScenePlan:      "You are a study planner for {{industry_code}} learners. Build a practical plan for level {{level}} with daily study time {{daily_study_time}} minutes, duration {{duration_days}} days, and goal {{goal_description}}.",
	model.PromptSceneCompanion: "You are a supportive study companion. Reply briefly, stay empathetic to the user's emotion {{user_emotion}}, and give concrete encouragement.",
	model.PromptSceneQuiz:      "You are a code and answer reviewer. Judge correctness carefully, explain issues clearly, and provide actionable improvements.",
}

type Builder struct {
	configRepo    repository.AdminConfigRepository
	promptRepo    repository.PromptTemplateRepository
	industryRepo  repository.IndustryRepository
	aiCallLogRepo repository.AICallLogRepository
	baseConfig    map[string]string
}

// NewBuilder 创建 AI runtime 构建器。
func NewBuilder(
	configRepo repository.AdminConfigRepository,
	promptRepo repository.PromptTemplateRepository,
	industryRepo repository.IndustryRepository,
	aiCallLogRepo repository.AICallLogRepository,
	baseConfig map[string]string,
) *Builder {
	return &Builder{
		configRepo:    configRepo,
		promptRepo:    promptRepo,
		industryRepo:  industryRepo,
		aiCallLogRepo: aiCallLogRepo,
		baseConfig:    ai.NormalizeRuntimeConfig(baseConfig),
	}
}

// Build 根据默认配置和后台配置组装 AI 客户端。
func (b *Builder) Build(ctx context.Context) *ai.AIClient {
	runtimeConfig := b.loadRuntimeConfig(ctx)
	return b.buildClient(ctx, runtimeConfig)
}

// buildClient 基于已经归一化的 runtime 配置构建一套 AI 客户端。
func (b *Builder) buildClient(ctx context.Context, runtimeConfig map[string]string) *ai.AIClient {
	prompts := &promptResolver{
		promptRepo:   b.promptRepo,
		industryRepo: b.industryRepo,
	}

	interviewSceneConfig := buildSceneConfig(runtimeConfig, model.PromptSceneInterview)
	planSceneConfig := buildSceneConfig(runtimeConfig, model.PromptScenePlan)
	companionSceneConfig := buildSceneConfig(runtimeConfig, model.PromptSceneCompanion)
	quizSceneConfig := buildSceneConfig(runtimeConfig, model.PromptSceneQuiz)

	interviewProvider := buildProvider(ctx, interviewSceneConfig)
	planProvider := buildProvider(ctx, planSceneConfig)
	companionProvider := buildProvider(ctx, companionSceneConfig)
	quizProvider := buildProvider(ctx, quizSceneConfig)

	return &ai.AIClient{
		Provider: buildProvider(ctx, buildSceneConfig(runtimeConfig, "")),
		InterviewAgent: newInterviewAgent(
			interviewProvider,
			prompts,
			newAICallLogRecorder(b.aiCallLogRepo, model.AICallSourceInterviewRuntime, model.PromptSceneInterview, runtimeConfig, interviewSceneConfig),
		),
		PlanAgent: newPlanAgent(
			planProvider,
			prompts,
			newAICallLogRecorder(b.aiCallLogRepo, model.AICallSourcePlanRuntime, model.PromptScenePlan, runtimeConfig, planSceneConfig),
		),
		CompanionAgent: newCompanionAgent(
			companionProvider,
			prompts,
			newAICallLogRecorder(b.aiCallLogRepo, model.AICallSourceCompanionRuntime, model.PromptSceneCompanion, runtimeConfig, companionSceneConfig),
		),
		QuizAnalyzer: newQuizAnalyzer(
			quizProvider,
			prompts,
			newAICallLogRecorder(b.aiCallLogRepo, model.AICallSourceQuizRuntime, model.PromptSceneQuiz, runtimeConfig, quizSceneConfig),
		),
		Live2DDirector: newLive2DDirector(
			companionProvider,
			prompts,
			newAICallLogRecorder(b.aiCallLogRepo, model.AICallSourceCompanionRuntime, model.PromptSceneCompanion, runtimeConfig, companionSceneConfig),
		),
		ResumeParser: newResumeParser(
			interviewProvider,
			prompts,
			newAICallLogRecorder(b.aiCallLogRepo, model.AICallSourceInterviewRuntime, model.PromptSceneInterview, runtimeConfig, interviewSceneConfig),
		),
	}
}

// buildProvider 根据场景配置构造可观测的 Provider，并在需要时挂上显式配置的 fallback 链路。
func buildProvider(ctx context.Context, sceneConfig map[string]string) ai.AIProvider {
	primaryType := strings.TrimSpace(sceneConfig[ai.ConfigKeyProvider])
	primary, err := newProvider(ctx, primaryType, sceneConfig)
	if err != nil {
		fallbackType := strings.TrimSpace(sceneConfig[ai.ConfigKeyFallbackProvider])
		fallback, fallbackErr := newProvider(ctx, fallbackType, sceneConfig)
		if fallbackErr == nil {
			return fallback
		}

		return &unavailableProvider{
			modelName: sceneConfig[ai.ConfigKeyModel],
			err:       err,
		}
	}

	fallbackType := strings.TrimSpace(sceneConfig[ai.ConfigKeyFallbackProvider])
	if fallbackType == "" || fallbackType == primaryType {
		return primary
	}

	fallback, err := newProvider(ctx, fallbackType, sceneConfig)
	if err != nil {
		return primary
	}

	return &providerWithFallback{
		primary:  primary,
		fallback: fallback,
	}
}

// newProvider 根据 provider 类型创建具体实现，并统一包上一层可追踪执行元信息的包装器。
func newProvider(ctx context.Context, providerType string, config map[string]string) (executionTraceProvider, error) {
	var (
		provider ai.AIProvider
		err      error
	)
	switch ai.NormalizeProviderType(providerType) {
	case string(ai.ProviderTypeEino):
		provider, err = eino.NewProvider(ctx, config)
	case string(ai.ProviderTypeMock):
		return nil, fmt.Errorf("mock ai provider has been removed, please configure a real provider")
	case "":
		return nil, fmt.Errorf("ai provider is not configured")
	default:
		return nil, fmt.Errorf("unsupported ai provider: %s", strings.TrimSpace(providerType))
	}

	if err != nil {
		return nil, err
	}
	return wrapNamedProvider(provider, providerType), nil
}

// loadRuntimeConfig 加载 AI runtime 配置，以 config.yaml 为默认值，并让后台配置优先生效。
func (b *Builder) loadRuntimeConfig(ctx context.Context) map[string]string {
	merged := ai.DefaultRuntimeConfig()

	for key, value := range ai.NormalizeRuntimeConfig(b.baseConfig) {
		merged[key] = value
	}

	if b.configRepo != nil {
		items, err := b.configRepo.List(ctx)
		if err == nil {
			for _, item := range items {
				merged[item.ConfigKey] = item.ConfigValue
			}
		}
	}

	return ai.NormalizeRuntimeConfig(merged)
}

func buildSceneConfig(runtimeConfig map[string]string, scene string) map[string]string {
	sceneConfig := ai.NormalizeRuntimeConfig(runtimeConfig)

	overrideKey := ""
	switch scene {
	case model.PromptSceneInterview:
		overrideKey = ai.ConfigKeyInterviewModel
	case model.PromptScenePlan:
		overrideKey = ai.ConfigKeyPlanModel
	case model.PromptSceneCompanion:
		overrideKey = ai.ConfigKeyCompanionModel
	case model.PromptSceneQuiz:
		overrideKey = ai.ConfigKeyQuizModel
	}

	if overrideKey != "" {
		if modelName := strings.TrimSpace(sceneConfig[overrideKey]); modelName != "" {
			sceneConfig[ai.ConfigKeyModel] = modelName
		}
	}

	return sceneConfig
}

type promptResolver struct {
	promptRepo   repository.PromptTemplateRepository
	industryRepo repository.IndustryRepository
}

// resolvedPromptDetails 描述 prompt 解析后的来源与内容。
type resolvedPromptDetails struct {
	Prompt       string
	Source       string
	TemplateID   *uint
	TemplateName string
}

// ResolveByIndustryCode 按行业编码解析 prompt 文本。
func (r *promptResolver) ResolveByIndustryCode(ctx context.Context, scene string, industryCode string, vars map[string]string) string {
	return r.ResolveDetailsByIndustryCode(ctx, scene, industryCode, vars).Prompt
}

// ResolveByIndustryID 按行业 ID 解析 prompt 文本。
func (r *promptResolver) ResolveByIndustryID(ctx context.Context, scene string, industryID *uint, vars map[string]string) string {
	return r.ResolveDetailsByIndustryID(ctx, scene, industryID, vars).Prompt
}

// ResolveDetailsByIndustryCode 按行业编码解析 prompt 详情。
func (r *promptResolver) ResolveDetailsByIndustryCode(ctx context.Context, scene string, industryCode string, vars map[string]string) resolvedPromptDetails {
	if industryID := r.lookupIndustryID(ctx, industryCode); industryID != nil {
		return r.ResolveDetailsByIndustryID(ctx, scene, industryID, vars)
	}
	return r.ResolveDetailsByIndustryID(ctx, scene, nil, vars)
}

// ResolveDetailsByIndustryID 按行业 ID 解析 prompt 详情。
func (r *promptResolver) ResolveDetailsByIndustryID(ctx context.Context, scene string, industryID *uint, vars map[string]string) resolvedPromptDetails {
	if r.promptRepo != nil {
		if tpl := r.findActiveTemplate(ctx, scene, industryID, false); tpl != nil {
			return buildResolvedPromptDetails(tpl, "template_industry", vars)
		}
		if tpl := r.findActiveTemplate(ctx, scene, nil, true); tpl != nil {
			return buildResolvedPromptDetails(tpl, "template_generic", vars)
		}
	}

	return resolvedPromptDetails{
		Prompt: renderPrompt(builtInScenePrompts[scene], vars),
		Source: "built_in",
	}
}

func (r *promptResolver) lookupIndustryID(ctx context.Context, industryCode string) *uint {
	if r.industryRepo == nil {
		return nil
	}

	industryCode = strings.TrimSpace(industryCode)
	if industryCode == "" {
		return nil
	}

	industry, err := r.industryRepo.GetByCode(ctx, industryCode)
	if err != nil || industry == nil || industry.ID == 0 {
		return nil
	}

	industryID := industry.ID
	return &industryID
}

func (r *promptResolver) findActiveTemplate(ctx context.Context, scene string, industryID *uint, genericOnly bool) *model.PromptTemplate {
	templates, err := r.promptRepo.List(ctx, industryID, scene)
	if err != nil {
		return nil
	}

	for i := range templates {
		tpl := &templates[i]
		if !tpl.IsActive {
			continue
		}
		if genericOnly && !tpl.IsGeneric() {
			continue
		}
		if !genericOnly && industryID != nil {
			if tpl.IndustryID == nil || *tpl.IndustryID != *industryID {
				continue
			}
		}
		return tpl
	}

	return nil
}

// buildResolvedPromptDetails 构造带来源信息的 prompt 解析结果。
func buildResolvedPromptDetails(tpl *model.PromptTemplate, source string, vars map[string]string) resolvedPromptDetails {
	if tpl == nil {
		return resolvedPromptDetails{Source: source}
	}

	var templateID *uint
	if tpl.ID != 0 {
		templateIDValue := tpl.ID
		templateID = &templateIDValue
	}

	return resolvedPromptDetails{
		Prompt:       renderPrompt(tpl.TemplateContent, vars),
		Source:       source,
		TemplateID:   templateID,
		TemplateName: strings.TrimSpace(tpl.Name),
	}
}

func renderPrompt(template string, vars map[string]string) string {
	rendered := strings.TrimSpace(template)
	if rendered == "" {
		return ""
	}

	for key, value := range vars {
		placeholder := "{{" + key + "}}"
		rendered = strings.ReplaceAll(rendered, placeholder, strings.TrimSpace(value))
		rendered = strings.ReplaceAll(rendered, "{{ "+key+" }}", strings.TrimSpace(value))
	}

	return rendered
}

func mergePrompt(prompt string, content string) string {
	prompt = strings.TrimSpace(prompt)
	content = strings.TrimSpace(content)

	switch {
	case prompt == "":
		return content
	case content == "":
		return prompt
	default:
		return prompt + "\n\n" + content
	}
}

func intToString(value int) string {
	return strconv.Itoa(value)
}

// providerExecutionMeta 描述最近一次调用实际命中的 provider、模型与 fallback 状态。
type providerExecutionMeta struct {
	Provider     string
	Model        string
	UsedFallback bool
	PrimaryError string
}

// executionTraceProvider 描述支持暴露最近一次执行元信息的 Provider。
type executionTraceProvider interface {
	ai.AIProvider
	LastExecutionMeta() providerExecutionMeta
}

// namedProvider 为基础 Provider 补充可观测的真实 provider 名称与最近一次执行快照。
type namedProvider struct {
	name string
	base ai.AIProvider
	mu   sync.Mutex
	last providerExecutionMeta
}

// wrapNamedProvider 为 Provider 加上一层执行元信息包装，避免日志只能看到配置值看不到真实命中结果。
func wrapNamedProvider(provider ai.AIProvider, providerType string) executionTraceProvider {
	if traced, ok := provider.(executionTraceProvider); ok {
		return traced
	}
	return &namedProvider{
		name: strings.TrimSpace(ai.NormalizeProviderType(providerType)),
		base: provider,
	}
}

// Chat 调用底层 Provider，并记录最近一次实际执行的 provider 与模型。
func (p *namedProvider) Chat(ctx context.Context, messages []ai.Message) (*ai.ChatResponse, error) {
	resp, err := p.base.Chat(ctx, messages)
	p.setLast(providerExecutionMeta{
		Provider: strings.TrimSpace(p.name),
		Model:    strings.TrimSpace(p.base.GetModelName()),
	})
	return resp, err
}

// StreamChat 调用底层流式 Provider，并记录最近一次实际执行的 provider 与模型。
func (p *namedProvider) StreamChat(ctx context.Context, messages []ai.Message) (<-chan string, error) {
	stream, err := p.base.StreamChat(ctx, messages)
	p.setLast(providerExecutionMeta{
		Provider: strings.TrimSpace(p.name),
		Model:    strings.TrimSpace(p.base.GetModelName()),
	})
	return stream, err
}

// GetModelName 返回底层 Provider 的模型名称。
func (p *namedProvider) GetModelName() string {
	return p.base.GetModelName()
}

// LastExecutionMeta 返回最近一次执行快照，用于调试日志识别真实命中的 Provider。
func (p *namedProvider) LastExecutionMeta() providerExecutionMeta {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.last
}

// setLast 更新最近一次执行快照。
func (p *namedProvider) setLast(meta providerExecutionMeta) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.last = meta
}

// providerWithFallback 在主 Provider 失败时自动切换到 fallback，并记录实际命中的执行元信息。
type providerWithFallback struct {
	primary  executionTraceProvider
	fallback executionTraceProvider
	mu       sync.Mutex
	last     providerExecutionMeta
}

// Chat 优先调用主 Provider，失败后自动切换到 fallback。
func (p *providerWithFallback) Chat(ctx context.Context, messages []ai.Message) (*ai.ChatResponse, error) {
	resp, err := p.primary.Chat(ctx, messages)
	if err == nil {
		p.setLast(p.primary.LastExecutionMeta())
		return resp, nil
	}

	resp, fallbackErr := p.fallback.Chat(ctx, messages)
	meta := p.fallback.LastExecutionMeta()
	meta.UsedFallback = true
	meta.PrimaryError = err.Error()
	p.setLast(meta)
	return resp, fallbackErr
}

// StreamChat 优先调用主 Provider 的流式输出，失败后自动切换到 fallback。
func (p *providerWithFallback) StreamChat(ctx context.Context, messages []ai.Message) (<-chan string, error) {
	stream, err := p.primary.StreamChat(ctx, messages)
	if err == nil {
		p.setLast(p.primary.LastExecutionMeta())
		return stream, nil
	}

	stream, fallbackErr := p.fallback.StreamChat(ctx, messages)
	meta := p.fallback.LastExecutionMeta()
	meta.UsedFallback = true
	meta.PrimaryError = err.Error()
	p.setLast(meta)
	return stream, fallbackErr
}

// GetModelName 返回主 Provider 的模型名，保持兼容既有调用方。
func (p *providerWithFallback) GetModelName() string {
	return p.primary.GetModelName()
}

// LastExecutionMeta 返回最近一次调用实际命中的 Provider 元信息。
func (p *providerWithFallback) LastExecutionMeta() providerExecutionMeta {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.last
}

// setLast 更新最近一次调用实际命中的 Provider 元信息。
func (p *providerWithFallback) setLast(meta providerExecutionMeta) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.last = meta
}

type unavailableProvider struct {
	modelName string
	err       error
}

// Chat 返回不可用 Provider 的固定错误，避免运行时静默落入其他实现。
func (p *unavailableProvider) Chat(context.Context, []ai.Message) (*ai.ChatResponse, error) {
	return nil, p.err
}

// StreamChat 返回不可用 Provider 的固定错误，避免流式链路静默降级。
func (p *unavailableProvider) StreamChat(context.Context, []ai.Message) (<-chan string, error) {
	return nil, p.err
}

// GetModelName 返回当前不可用 Provider 对应的模型名，用于日志排查。
func (p *unavailableProvider) GetModelName() string {
	if strings.TrimSpace(p.modelName) != "" {
		return p.modelName
	}
	return "unavailable"
}

// LastExecutionMeta 返回不可用 Provider 的执行元信息，便于调试页显示真实命中结果。
func (p *unavailableProvider) LastExecutionMeta() providerExecutionMeta {
	return providerExecutionMeta{
		Provider: "unavailable",
		Model:    p.GetModelName(),
	}
}
