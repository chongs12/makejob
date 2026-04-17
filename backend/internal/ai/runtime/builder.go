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
	model.PromptScenePlan:      "You are a study planner for {{industry_code}} learners. Build a practical plan for level {{level}} with daily study time {{daily_study_time}} minutes and goal {{goal_description}}.",
	model.PromptSceneCompanion: "You are a supportive study companion. Reply briefly, stay empathetic to the user's emotion {{user_emotion}}, and give concrete encouragement.",
	model.PromptSceneQuiz:      "You are a code and answer reviewer. Judge correctness carefully, explain issues clearly, and provide actionable improvements.",
}

type Builder struct {
	configRepo   repository.AdminConfigRepository
	promptRepo   repository.PromptTemplateRepository
	industryRepo repository.IndustryRepository
}

func NewBuilder(
	configRepo repository.AdminConfigRepository,
	promptRepo repository.PromptTemplateRepository,
	industryRepo repository.IndustryRepository,
) *Builder {
	return &Builder{
		configRepo:   configRepo,
		promptRepo:   promptRepo,
		industryRepo: industryRepo,
	}
}

func (b *Builder) Build(ctx context.Context) *ai.AIClient {
	runtimeConfig := b.loadRuntimeConfig(ctx)
	prompts := &promptResolver{
		promptRepo:   b.promptRepo,
		industryRepo: b.industryRepo,
	}

	interviewProvider := buildProvider(ctx, runtimeConfig, model.PromptSceneInterview)
	planProvider := buildProvider(ctx, runtimeConfig, model.PromptScenePlan)
	companionProvider := buildProvider(ctx, runtimeConfig, model.PromptSceneCompanion)
	quizProvider := buildProvider(ctx, runtimeConfig, model.PromptSceneQuiz)

	return &ai.AIClient{
		Provider: buildProvider(ctx, runtimeConfig, ""),
		InterviewAgent: &interviewAgent{
			base:    mock.NewInterviewAgent(interviewProvider),
			prompts: prompts,
		},
		PlanAgent: &planAgent{
			base:    mock.NewPlanAgent(planProvider),
			prompts: prompts,
		},
		CompanionAgent: &companionAgent{
			base:    mock.NewCompanionAgent(companionProvider),
			prompts: prompts,
		},
		QuizAnalyzer: &quizAnalyzer{
			base:    mock.NewQuizAnalyzer(quizProvider),
			prompts: prompts,
		},
	}
}

func buildProvider(ctx context.Context, runtimeConfig map[string]string, scene string) ai.AIProvider {
	sceneConfig := buildSceneConfig(runtimeConfig, scene)

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

func (b *Builder) loadRuntimeConfig(ctx context.Context) map[string]string {
	if b.configRepo == nil {
		return ai.DefaultRuntimeConfig()
	}

	items, err := b.configRepo.List(ctx)
	if err != nil {
		return ai.DefaultRuntimeConfig()
	}

	raw := make(map[string]string, len(items))
	for _, item := range items {
		raw[item.ConfigKey] = item.ConfigValue
	}

	return ai.NormalizeRuntimeConfig(raw)
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

func (r *promptResolver) ResolveByIndustryCode(ctx context.Context, scene string, industryCode string, vars map[string]string) string {
	if industryID := r.lookupIndustryID(ctx, industryCode); industryID != nil {
		return r.ResolveByIndustryID(ctx, scene, industryID, vars)
	}
	return r.ResolveByIndustryID(ctx, scene, nil, vars)
}

func (r *promptResolver) ResolveByIndustryID(ctx context.Context, scene string, industryID *uint, vars map[string]string) string {
	if r.promptRepo != nil {
		if tpl := r.findActiveTemplate(ctx, scene, industryID, false); tpl != nil {
			return renderPrompt(tpl.TemplateContent, vars)
		}
		if tpl := r.findActiveTemplate(ctx, scene, nil, true); tpl != nil {
			return renderPrompt(tpl.TemplateContent, vars)
		}
	}

	return renderPrompt(builtInScenePrompts[scene], vars)
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

type interviewAgent struct {
	base    ai.InterviewAgent
	prompts *promptResolver
}

func (a *interviewAgent) StartInterview(ctx context.Context, config ai.InterviewConfig) (string, ai.InterviewQuestion, error) {
	if a.prompts != nil {
		_ = a.prompts.ResolveByIndustryCode(ctx, model.PromptSceneInterview, config.IndustryCode, map[string]string{
			"industry_code":  config.IndustryCode,
			"difficulty":     config.Difficulty,
			"topics":         strings.Join(config.Topics, ", "),
			"question_count": intToString(config.QuestionCount),
		})
	}

	return a.base.StartInterview(ctx, config)
}

func (a *interviewAgent) EvaluateAnswer(ctx context.Context, sessionID string, questionIndex int, answer string) (ai.AnswerFeedback, error) {
	return a.base.EvaluateAnswer(ctx, sessionID, questionIndex, answer)
}

func (a *interviewAgent) GetNextQuestion(ctx context.Context, sessionID string) (ai.InterviewQuestion, bool, error) {
	return a.base.GetNextQuestion(ctx, sessionID)
}

func (a *interviewAgent) GenerateReport(ctx context.Context, sessionID string) (ai.InterviewReport, error) {
	return a.base.GenerateReport(ctx, sessionID)
}

func (a *interviewAgent) EndInterview(ctx context.Context, sessionID string) error {
	return a.base.EndInterview(ctx, sessionID)
}

type planAgent struct {
	base    ai.PlanAgent
	prompts *promptResolver
}

func (a *planAgent) GeneratePlan(ctx context.Context, profile ai.UserProfile, industryCode string) (ai.LearningPlan, error) {
	if a.prompts != nil {
		prompt := a.prompts.ResolveByIndustryCode(ctx, model.PromptScenePlan, industryCode, map[string]string{
			"industry_code":    industryCode,
			"level":            profile.Level,
			"daily_study_time": intToString(profile.DailyStudyTime),
			"goal_description": profile.GoalDescription,
			"weak_topics":      strings.Join(profile.WeakTopics, ", "),
			"strong_topics":    strings.Join(profile.StrongTopics, ", "),
		})
		profile.GoalDescription = mergePrompt(prompt, profile.GoalDescription)
	}

	return a.base.GeneratePlan(ctx, profile, industryCode)
}

func (a *planAgent) AdjustPlan(ctx context.Context, planID string, completedTasks []string, performance map[string]float64) (ai.LearningPlan, error) {
	return a.base.AdjustPlan(ctx, planID, completedTasks, performance)
}

func (a *planAgent) GetStudySuggestion(ctx context.Context, profile ai.UserProfile) (string, error) {
	return a.base.GetStudySuggestion(ctx, profile)
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

type quizAnalyzer struct {
	base    ai.QuizAnalyzer
	prompts *promptResolver
}

func (a *quizAnalyzer) AnalyzeCode(ctx context.Context, code string, language string, question string) (ai.CodeAnalysis, error) {
	return a.base.AnalyzeCode(ctx, code, language, question)
}

func (a *quizAnalyzer) ExplainAnswer(ctx context.Context, questionTitle string, questionContent string, correctAnswer string) (string, error) {
	return a.base.ExplainAnswer(ctx, questionTitle, questionContent, correctAnswer)
}

func (a *quizAnalyzer) GenerateHint(ctx context.Context, questionTitle string, questionContent string) (string, error) {
	return a.base.GenerateHint(ctx, questionTitle, questionContent)
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
