package runtime

import (
	"context"
	"strconv"
	"strings"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/ai/eino"
	"makejob-backend/internal/ai/mock"
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
			base:    mock.NewCompanionAgent(companionProvider),
			prompts: prompts,
		},
		QuizAnalyzer: newQuizAnalyzer(
			quizProvider,
			prompts,
			newAICallLogRecorder(b.aiCallLogRepo, model.AICallSourceQuizRuntime, model.PromptSceneQuiz, runtimeConfig, quizSceneConfig),
		),
	}
}

func buildProvider(ctx context.Context, sceneConfig map[string]string) ai.AIProvider {
	primary, err := newProvider(ctx, sceneConfig[ai.ConfigKeyProvider], sceneConfig)
	if err != nil {
		fallback, fallbackErr := newProvider(ctx, sceneConfig[ai.ConfigKeyFallbackProvider], sceneConfig)
		if fallbackErr == nil {
			return fallback
		}

		return &unavailableProvider{
			modelName: sceneConfig[ai.ConfigKeyModel],
			err:       err,
		}
	}

	fallbackType := strings.TrimSpace(sceneConfig[ai.ConfigKeyFallbackProvider])
	if fallbackType == "" || fallbackType == sceneConfig[ai.ConfigKeyProvider] {
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

func newProvider(ctx context.Context, providerType string, config map[string]string) (ai.AIProvider, error) {
	switch ai.NormalizeProviderType(providerType) {
	case string(ai.ProviderTypeEino):
		return eino.NewProvider(ctx, config)
	case string(ai.ProviderTypeMock), "":
		return mock.NewAIProvider(string(ai.ProviderTypeMock), config), nil
	default:
		return mock.NewAIProvider(string(ai.ProviderTypeMock), config), nil
	}
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
	base    ai.CompanionAgent
	prompts *promptResolver
}

func (a *companionAgent) Chat(ctx context.Context, messages []ai.Message, userEmotion string) (ai.CompanionResponse, error) {
	if a.prompts != nil {
		prompt := a.prompts.ResolveByIndustryID(ctx, model.PromptSceneCompanion, nil, map[string]string{
			"user_emotion":        userEmotion,
			"latest_user_message": latestUserMessage(messages),
		})
		messages = prependSystemPrompt(messages, prompt)
	}

	return a.base.Chat(ctx, messages, userEmotion)
}

func (a *companionAgent) GetGreeting(ctx context.Context, profile ai.UserProfile, timeOfDay string) (ai.CompanionResponse, error) {
	return a.base.GetGreeting(ctx, profile, timeOfDay)
}

func (a *companionAgent) GetEncouragement(ctx context.Context, achievement string) (ai.CompanionResponse, error) {
	return a.base.GetEncouragement(ctx, achievement)
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

type providerWithFallback struct {
	primary  ai.AIProvider
	fallback ai.AIProvider
}

func (p *providerWithFallback) Chat(ctx context.Context, messages []ai.Message) (string, error) {
	content, err := p.primary.Chat(ctx, messages)
	if err == nil {
		return content, nil
	}
	return p.fallback.Chat(ctx, messages)
}

func (p *providerWithFallback) StreamChat(ctx context.Context, messages []ai.Message) (<-chan string, error) {
	stream, err := p.primary.StreamChat(ctx, messages)
	if err == nil {
		return stream, nil
	}
	return p.fallback.StreamChat(ctx, messages)
}

func (p *providerWithFallback) GetModelName() string {
	return p.primary.GetModelName()
}

type unavailableProvider struct {
	modelName string
	err       error
}

func (p *unavailableProvider) Chat(context.Context, []ai.Message) (string, error) {
	return "", p.err
}

func (p *unavailableProvider) StreamChat(context.Context, []ai.Message) (<-chan string, error) {
	return nil, p.err
}

func (p *unavailableProvider) GetModelName() string {
	if strings.TrimSpace(p.modelName) != "" {
		return p.modelName
	}
	return "unavailable"
}
