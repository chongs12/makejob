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
		CompanionAgent: &companionAgent{
			provider: companionProvider,
			prompts:  prompts,
		},
		QuizAnalyzer: newQuizAnalyzer(
			quizProvider,
			prompts,
			newAICallLogRecorder(b.aiCallLogRepo, model.AICallSourceQuizRuntime, model.PromptSceneQuiz, runtimeConfig, quizSceneConfig),
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

// loadRuntimeConfig 加载 AI runtime 配置，并让 config.yaml 中的显式配置优先生效。
func (b *Builder) loadRuntimeConfig(ctx context.Context) map[string]string {
	merged := ai.DefaultRuntimeConfig()

	if b.configRepo != nil {
		items, err := b.configRepo.List(ctx)
		if err == nil {
			for _, item := range items {
				merged[item.ConfigKey] = item.ConfigValue
			}
		}
	}

	for key, value := range ai.NormalizeRuntimeConfig(b.baseConfig) {
		merged[key] = value
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

type companionAgent struct {
	provider ai.AIProvider
	prompts  *promptResolver
}

// Chat 调用真实 Provider 生成陪伴回复，并补齐前端需要的情绪与动作字段。
func (a *companionAgent) Chat(ctx context.Context, messages []ai.Message, userEmotion string) (ai.CompanionResponse, error) {
	if a.prompts != nil {
		prompt := a.prompts.ResolveByIndustryID(ctx, model.PromptSceneCompanion, nil, map[string]string{
			"user_emotion":        userEmotion,
			"latest_user_message": latestUserMessage(messages),
		})
		messages = prependSystemPrompt(messages, prompt)
	}

	if a.provider == nil {
		return ai.CompanionResponse{}, fmt.Errorf("ai provider is unavailable")
	}

	content, err := a.provider.Chat(ctx, messages)
	if err != nil {
		return ai.CompanionResponse{}, err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ai.CompanionResponse{}, fmt.Errorf("empty companion response")
	}

	emotion := normalizeCompanionEmotion(userEmotion)
	return ai.CompanionResponse{
		Content: content,
		Emotion: emotion,
		Action:  companionActionForEmotion(emotion),
	}, nil
}

// GetGreeting 生成无需调用模型的本地欢迎语，避免陪伴首页因 Provider 异常直接报错。
func (a *companionAgent) GetGreeting(ctx context.Context, profile ai.UserProfile, timeOfDay string) (ai.CompanionResponse, error) {
	select {
	case <-ctx.Done():
		return ai.CompanionResponse{}, ctx.Err()
	default:
	}

	content := "你好，今天继续推进你的学习计划。"
	emotion := "happy"
	action := "wave"
	switch strings.ToLower(strings.TrimSpace(timeOfDay)) {
	case "morning":
		content = "早上好，先用一个清晰的小目标打开今天的学习节奏。"
	case "afternoon":
		content = "下午好，保持专注，把今天最重要的一件学习任务收掉。"
		emotion = "encouraging"
		action = "nod"
	case "evening":
		content = "晚上好，适合做复盘和查漏补缺，把今天的收获沉淀下来。"
		emotion = "neutral"
		action = "idle"
	case "night":
		content = "夜深了，注意节奏，优先做轻量复盘，不要透支状态。"
		emotion = "encouraging"
		action = "nod"
	}
	if strings.EqualFold(strings.TrimSpace(profile.Level), "beginner") {
		content += " 先稳住基础，不用追求一步到位。"
	}
	if strings.EqualFold(strings.TrimSpace(profile.Level), "advanced") {
		content += " 今天可以主动挑战一个更难的问题。"
	}
	return ai.CompanionResponse{Content: content, Emotion: emotion, Action: action}, nil
}

// GetEncouragement 生成无需调用模型的本地鼓励语，保证基础交互始终可用。
func (a *companionAgent) GetEncouragement(ctx context.Context, achievement string) (ai.CompanionResponse, error) {
	select {
	case <-ctx.Done():
		return ai.CompanionResponse{}, ctx.Err()
	default:
	}

	achievement = strings.TrimSpace(achievement)
	if achievement == "" {
		achievement = "当前这一步"
	}
	return ai.CompanionResponse{
		Content: achievement + " 做得不错，继续保持这个节奏，不要被短期波动打断。",
		Emotion: "encouraging",
		Action:  "nod",
	}, nil
}

// normalizeCompanionEmotion 规范化陪伴场景情绪值，便于前端动作与表情映射保持稳定。
func normalizeCompanionEmotion(userEmotion string) string {
	switch strings.ToLower(strings.TrimSpace(userEmotion)) {
	case "happy", "excited":
		return "happy"
	case "sad", "tired":
		return "encouraging"
	case "frustrated", "confused":
		return "thinking"
	default:
		return "neutral"
	}
}

// companionActionForEmotion 根据情绪选择默认动作，避免陪伴场景出现空动作。
func companionActionForEmotion(emotion string) string {
	switch emotion {
	case "happy":
		return "wave"
	case "encouraging":
		return "nod"
	case "thinking":
		return "thinking"
	default:
		return "idle"
	}
}

func prependSystemPrompt(messages []ai.Message, prompt string) []ai.Message {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return messages
	}

	result := make([]ai.Message, 0, len(messages)+1)
	result = append(result, ai.Message{
		Role:    "system",
		Content: prompt,
	})
	result = append(result, messages...)
	return result
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

func latestUserMessage(messages []ai.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(messages[i].Role), "user") {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
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
func (p *namedProvider) Chat(ctx context.Context, messages []ai.Message) (string, error) {
	content, err := p.base.Chat(ctx, messages)
	p.setLast(providerExecutionMeta{
		Provider: strings.TrimSpace(p.name),
		Model:    strings.TrimSpace(p.base.GetModelName()),
	})
	return content, err
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
func (p *providerWithFallback) Chat(ctx context.Context, messages []ai.Message) (string, error) {
	content, err := p.primary.Chat(ctx, messages)
	if err == nil {
		p.setLast(p.primary.LastExecutionMeta())
		return content, nil
	}

	content, fallbackErr := p.fallback.Chat(ctx, messages)
	meta := p.fallback.LastExecutionMeta()
	meta.UsedFallback = true
	meta.PrimaryError = err.Error()
	p.setLast(meta)
	return content, fallbackErr
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
func (p *unavailableProvider) Chat(context.Context, []ai.Message) (string, error) {
	return "", p.err
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
