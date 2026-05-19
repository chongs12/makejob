package runtime

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"makejob-backend/internal/ai"
)

// RuntimeManager 负责按当前后台配置复用或重建可热切换的 AI runtime。
type RuntimeManager struct {
	builder     *Builder
	mu          sync.RWMutex
	fingerprint string
	client      *ai.AIClient
}

// NewRuntimeManager 创建支持配置热切换的 AI runtime 管理器。
func NewRuntimeManager(builder *Builder) *RuntimeManager {
	return &RuntimeManager{builder: builder}
}

// BuildDynamicClient 返回一组会在调用时读取当前 runtime 的动态 Agent。
func (m *RuntimeManager) BuildDynamicClient() *ai.AIClient {
	return &ai.AIClient{
		Provider:       &runtimeProvider{manager: m},
		InterviewAgent: newRuntimeInterviewAgent(m),
		PlanAgent:      newRuntimePlanAgent(m),
		CompanionAgent: newRuntimeCompanionAgent(m),
		QuizAnalyzer:   newRuntimeQuizAnalyzer(m),
		Live2DDirector: newRuntimeLive2DDirector(m),
	}
}

// CurrentClient 返回与当前后台配置匹配的 AI 客户端，配置变化时会自动重建。
func (m *RuntimeManager) CurrentClient(ctx context.Context) *ai.AIClient {
	if m == nil || m.builder == nil {
		return &ai.AIClient{}
	}

	runtimeConfig := m.builder.loadRuntimeConfig(ctx)
	fingerprint := runtimeConfigFingerprint(runtimeConfig)

	m.mu.RLock()
	if m.client != nil && m.fingerprint == fingerprint {
		client := m.client
		m.mu.RUnlock()
		return client
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client != nil && m.fingerprint == fingerprint {
		return m.client
	}

	m.client = m.builder.buildClient(ctx, runtimeConfig)
	m.fingerprint = fingerprint
	return m.client
}

// runtimeConfigFingerprint 生成一份稳定的 runtime 配置指纹，供热更新判断复用条件。
func runtimeConfigFingerprint(config map[string]string) string {
	if len(config) == 0 {
		return ""
	}

	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(strings.TrimSpace(config[key]))
		builder.WriteString("\n")
	}
	return builder.String()
}

// runtimeProvider 把动态 runtime 暴露成统一 Provider，便于保留 AIClient 既有结构。
type runtimeProvider struct {
	manager *RuntimeManager
}

// Chat 委托当前生效 Provider 执行同步对话。
func (p *runtimeProvider) Chat(ctx context.Context, messages []ai.Message) (string, error) {
	provider, err := p.currentProvider(ctx)
	if err != nil {
		return "", err
	}
	return provider.Chat(ctx, messages)
}

// StreamChat 委托当前生效 Provider 执行流式对话。
func (p *runtimeProvider) StreamChat(ctx context.Context, messages []ai.Message) (<-chan string, error) {
	provider, err := p.currentProvider(ctx)
	if err != nil {
		return nil, err
	}
	return provider.StreamChat(ctx, messages)
}

// GetModelName 返回当前生效 Provider 的模型名，便于调试运行时切换结果。
func (p *runtimeProvider) GetModelName() string {
	if p == nil || p.manager == nil {
		return "unknown"
	}
	return p.manager.CurrentClient(context.Background()).GetModelName()
}

// currentProvider 获取当前生效的底层 Provider。
func (p *runtimeProvider) currentProvider(ctx context.Context) (ai.AIProvider, error) {
	if p == nil || p.manager == nil {
		return nil, fmt.Errorf("ai provider is unavailable")
	}
	provider := p.manager.CurrentClient(ctx).Provider
	if provider == nil {
		return nil, fmt.Errorf("ai provider is unavailable")
	}
	return provider, nil
}

// runtimeCompanionAgent 为陪伴场景提供可热切换的动态代理。
type runtimeCompanionAgent struct {
	manager *RuntimeManager
}

// newRuntimeCompanionAgent 创建陪伴场景的动态 Agent。
func newRuntimeCompanionAgent(manager *RuntimeManager) ai.CompanionAgent {
	return &runtimeCompanionAgent{manager: manager}
}

// Chat 委托当前生效的陪伴 Agent 生成回复。
func (a *runtimeCompanionAgent) Chat(ctx context.Context, messages []ai.Message, userEmotion string) (ai.CompanionResponse, error) {
	agent, err := a.currentAgent(ctx)
	if err != nil {
		return ai.CompanionResponse{}, err
	}
	return agent.Chat(ctx, messages, userEmotion)
}

// GetGreeting 委托当前生效的陪伴 Agent 生成问候语。
func (a *runtimeCompanionAgent) GetGreeting(ctx context.Context, profile ai.UserProfile, timeOfDay string) (ai.CompanionResponse, error) {
	agent, err := a.currentAgent(ctx)
	if err != nil {
		return ai.CompanionResponse{}, err
	}
	return agent.GetGreeting(ctx, profile, timeOfDay)
}

// GetEncouragement 委托当前生效的陪伴 Agent 生成鼓励语。
func (a *runtimeCompanionAgent) GetEncouragement(ctx context.Context, achievement string) (ai.CompanionResponse, error) {
	agent, err := a.currentAgent(ctx)
	if err != nil {
		return ai.CompanionResponse{}, err
	}
	return agent.GetEncouragement(ctx, achievement)
}

// currentAgent 获取当前生效的陪伴 Agent。
func (a *runtimeCompanionAgent) currentAgent(ctx context.Context) (ai.CompanionAgent, error) {
	if a == nil || a.manager == nil {
		return nil, fmt.Errorf("companion agent is unavailable")
	}
	agent := a.manager.CurrentClient(ctx).CompanionAgent
	if agent == nil {
		return nil, fmt.Errorf("companion agent is unavailable")
	}
	return agent, nil
}

// runtimePlanAgent 为学习计划场景提供可热切换的动态代理。
type runtimePlanAgent struct {
	manager *RuntimeManager
}

// newRuntimePlanAgent 创建学习计划场景的动态 Agent。
func newRuntimePlanAgent(manager *RuntimeManager) ai.PlanAgent {
	return &runtimePlanAgent{manager: manager}
}

// GeneratePlan 委托当前生效的计划 Agent 生成计划。
func (a *runtimePlanAgent) GeneratePlan(ctx context.Context, profile ai.UserProfile, industryCode string) (ai.LearningPlan, error) {
	agent, err := a.currentAgent(ctx)
	if err != nil {
		return ai.LearningPlan{}, err
	}
	return agent.GeneratePlan(ctx, profile, industryCode)
}

// AdjustPlan 委托当前生效的计划 Agent 调整计划。
func (a *runtimePlanAgent) AdjustPlan(ctx context.Context, input ai.PlanAdjustmentInput) (ai.LearningPlan, error) {
	agent, err := a.currentAgent(ctx)
	if err != nil {
		return ai.LearningPlan{}, err
	}
	return agent.AdjustPlan(ctx, input)
}

// GetStudySuggestion 委托当前生效的计划 Agent 生成学习建议。
func (a *runtimePlanAgent) GetStudySuggestion(ctx context.Context, profile ai.UserProfile) (string, error) {
	agent, err := a.currentAgent(ctx)
	if err != nil {
		return "", err
	}
	return agent.GetStudySuggestion(ctx, profile)
}

// currentAgent 获取当前生效的计划 Agent。
func (a *runtimePlanAgent) currentAgent(ctx context.Context) (ai.PlanAgent, error) {
	if a == nil || a.manager == nil {
		return nil, fmt.Errorf("plan agent is unavailable")
	}
	agent := a.manager.CurrentClient(ctx).PlanAgent
	if agent == nil {
		return nil, fmt.Errorf("plan agent is unavailable")
	}
	return agent, nil
}

// runtimeQuizAnalyzer 为题目分析场景提供可热切换的动态代理。
type runtimeQuizAnalyzer struct {
	manager *RuntimeManager
}

// newRuntimeQuizAnalyzer 创建题目分析场景的动态 Agent。
func newRuntimeQuizAnalyzer(manager *RuntimeManager) ai.QuizAnalyzer {
	return &runtimeQuizAnalyzer{manager: manager}
}

// AnalyzeCode 委托当前生效的分析 Agent 分析题目答案。
func (a *runtimeQuizAnalyzer) AnalyzeCode(ctx context.Context, code string, language string, question string) (ai.CodeAnalysis, error) {
	agent, err := a.currentAgent(ctx)
	if err != nil {
		return ai.CodeAnalysis{}, err
	}
	return agent.AnalyzeCode(ctx, code, language, question)
}

// DiagnoseInterviewCoding 委托当前生效的分析 Agent 诊断编程面试过程。
func (a *runtimeQuizAnalyzer) DiagnoseInterviewCoding(ctx context.Context, input ai.InterviewCodingDiagnosisInput) (ai.CodingQuestionDiagnosis, error) {
	agent, err := a.currentAgent(ctx)
	if err != nil {
		return ai.CodingQuestionDiagnosis{}, err
	}
	return agent.DiagnoseInterviewCoding(ctx, input)
}

// ExplainAnswer 委托当前生效的分析 Agent 解释参考答案。
func (a *runtimeQuizAnalyzer) ExplainAnswer(ctx context.Context, questionTitle string, questionContent string, correctAnswer string) (string, error) {
	agent, err := a.currentAgent(ctx)
	if err != nil {
		return "", err
	}
	return agent.ExplainAnswer(ctx, questionTitle, questionContent, correctAnswer)
}

// GenerateHint 委托当前生效的分析 Agent 生成提示。
func (a *runtimeQuizAnalyzer) GenerateHint(ctx context.Context, questionTitle string, questionContent string) (string, error) {
	agent, err := a.currentAgent(ctx)
	if err != nil {
		return "", err
	}
	return agent.GenerateHint(ctx, questionTitle, questionContent)
}

// currentAgent 获取当前生效的题目分析 Agent。
func (a *runtimeQuizAnalyzer) currentAgent(ctx context.Context) (ai.QuizAnalyzer, error) {
	if a == nil || a.manager == nil {
		return nil, fmt.Errorf("quiz analyzer is unavailable")
	}
	agent := a.manager.CurrentClient(ctx).QuizAnalyzer
	if agent == nil {
		return nil, fmt.Errorf("quiz analyzer is unavailable")
	}
	return agent, nil
}

// runtimeLive2DDirector 为 Live2D 指令场景提供可热切换的动态代理。
type runtimeLive2DDirector struct {
	manager *RuntimeManager
}

// newRuntimeLive2DDirector 创建 Live2D 指令场景的动态 Generator。
func newRuntimeLive2DDirector(manager *RuntimeManager) ai.Live2DDirectiveGenerator {
	return &runtimeLive2DDirector{manager: manager}
}

// GenerateDirective 委托当前生效的 Live2D 指令生成器输出结构化指令。
func (a *runtimeLive2DDirector) GenerateDirective(ctx context.Context, req ai.Live2DDirectiveContext) (*ai.Live2DDirective, error) {
	agent, err := a.currentAgent(ctx)
	if err != nil {
		return nil, err
	}
	return agent.GenerateDirective(ctx, req)
}

// currentAgent 获取当前生效的 Live2D 指令生成器。
func (a *runtimeLive2DDirector) currentAgent(ctx context.Context) (ai.Live2DDirectiveGenerator, error) {
	if a == nil || a.manager == nil {
		return nil, fmt.Errorf("live2d directive generator is unavailable")
	}
	agent := a.manager.CurrentClient(ctx).Live2DDirector
	if agent == nil {
		return nil, fmt.Errorf("live2d directive generator is unavailable")
	}
	return agent, nil
}

// runtimeInterviewAgent 为面试场景提供可热切换且保留会话绑定的动态代理。
type runtimeInterviewAgent struct {
	manager  *RuntimeManager
	sessions sync.Map
}

// newRuntimeInterviewAgent 创建面试场景的动态 Agent。
func newRuntimeInterviewAgent(manager *RuntimeManager) ai.InterviewAgent {
	return &runtimeInterviewAgent{manager: manager}
}

// StartInterview 使用当前生效的面试 Agent 创建新会话并绑定后续调用。
func (a *runtimeInterviewAgent) StartInterview(ctx context.Context, config ai.InterviewConfig) (string, ai.InterviewQuestion, error) {
	agent, err := a.currentAgent(ctx)
	if err != nil {
		return "", ai.InterviewQuestion{}, err
	}

	sessionID, firstQuestion, err := agent.StartInterview(ctx, config)
	if err == nil && strings.TrimSpace(sessionID) != "" {
		a.sessions.Store(strings.TrimSpace(sessionID), agent)
	}
	return sessionID, firstQuestion, err
}

// EvaluateAnswer 优先使用会话绑定的面试 Agent 继续当前面试流程。
func (a *runtimeInterviewAgent) EvaluateAnswer(ctx context.Context, sessionID string, questionIndex int, answer string) (ai.AnswerFeedback, error) {
	agent, err := a.sessionAgent(ctx, sessionID)
	if err != nil {
		return ai.AnswerFeedback{}, err
	}
	return agent.EvaluateAnswer(ctx, sessionID, questionIndex, answer)
}

// GetNextQuestion 优先使用会话绑定的面试 Agent 继续当前面试流程。
func (a *runtimeInterviewAgent) GetNextQuestion(ctx context.Context, sessionID string) (ai.InterviewQuestion, bool, error) {
	agent, err := a.sessionAgent(ctx, sessionID)
	if err != nil {
		return ai.InterviewQuestion{}, false, err
	}
	return agent.GetNextQuestion(ctx, sessionID)
}

// GenerateReport 优先使用会话绑定的面试 Agent 生成报告，避免切配置后丢失上下文。
func (a *runtimeInterviewAgent) GenerateReport(ctx context.Context, sessionID string) (ai.InterviewReport, error) {
	agent, err := a.sessionAgent(ctx, sessionID)
	if err != nil {
		return ai.InterviewReport{}, err
	}
	return agent.GenerateReport(ctx, sessionID)
}

// EndInterview 优先结束会话绑定的面试 Agent，并在成功后清理绑定关系。
func (a *runtimeInterviewAgent) EndInterview(ctx context.Context, sessionID string) error {
	agent, err := a.sessionAgent(ctx, sessionID)
	if err != nil {
		return err
	}
	if err := agent.EndInterview(ctx, sessionID); err != nil {
		return err
	}
	a.sessions.Delete(strings.TrimSpace(sessionID))
	return nil
}

// currentAgent 获取当前生效的面试 Agent，用于新会话创建。
func (a *runtimeInterviewAgent) currentAgent(ctx context.Context) (ai.InterviewAgent, error) {
	if a == nil || a.manager == nil {
		return nil, fmt.Errorf("interview agent is unavailable")
	}
	agent := a.manager.CurrentClient(ctx).InterviewAgent
	if agent == nil {
		return nil, fmt.Errorf("interview agent is unavailable")
	}
	return agent, nil
}

// sessionAgent 优先返回已绑定会话的面试 Agent，缺失时回退到当前生效 Agent。
func (a *runtimeInterviewAgent) sessionAgent(ctx context.Context, sessionID string) (ai.InterviewAgent, error) {
	if agent, ok := a.loadSessionAgent(sessionID); ok {
		return agent, nil
	}
	return a.currentAgent(ctx)
}

// loadSessionAgent 读取会话绑定的面试 Agent，确保切换配置后旧会话仍能继续。
func (a *runtimeInterviewAgent) loadSessionAgent(sessionID string) (ai.InterviewAgent, bool) {
	if a == nil {
		return nil, false
	}
	value, ok := a.sessions.Load(strings.TrimSpace(sessionID))
	if !ok {
		return nil, false
	}
	agent, ok := value.(ai.InterviewAgent)
	return agent, ok && agent != nil
}
