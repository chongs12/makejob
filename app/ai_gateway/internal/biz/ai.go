package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"makejob/pkg/model"
)

// ==================== 实体定义 ====================

// AIConfig AI 场景运行配置实体
type AIConfig struct {
	model.BaseModel
	Scene           string  `gorm:"size:50;not null;index"`
	Provider        string  `gorm:"size:50;not null"`
	Model           string  `gorm:"size:100;not null"`
	Temperature     float64 `gorm:"not null;default:0.7"`
	MaxTokens       int     `gorm:"not null;default:2048"`
	ExtraParamsJSON string  `gorm:"type:text"`
	IsActive        bool    `gorm:"not null;default:true;index"`
}

func (AIConfig) TableName() string { return "ai_configs" }

// PromptTemplate Prompt 模板实体（对齐单体后端 schema）
type PromptTemplate struct {
	model.BaseModel
	IndustryID      *uint  `json:"industry_id" gorm:"index;comment:所属行业ID，NULL表示通用"`
	Name            string `json:"name" gorm:"size:100;not null;comment:模板名称"`
	Scene           string `json:"scene" gorm:"size:20;not null;index;comment:使用场景"`
	TemplateContent string `json:"template_content" gorm:"type:text;not null;comment:模板内容"`
	Variables       string `json:"variables" gorm:"type:text;comment:模板变量说明JSON"`
	IsActive        bool   `json:"is_active" gorm:"not null;default:true;comment:是否启用"`
}

func (PromptTemplate) TableName() string { return "prompt_templates" }

// AICallLog AI 调用日志实体
// FIX: 嵌入model.BaseModel，移除手写的ID和CreatedAt字段
type AICallLog struct {
	model.BaseModel
	TraceID            string `gorm:"size:64;not null;index;comment:调用链路ID"`
	TaskID             *uint  `gorm:"index;comment:关联异步任务ID"`
	Source             string `gorm:"size:32;not null;index;comment:调用来源"`
	Scene              string `gorm:"size:50;not null;index;comment:场景"`
	IndustryID         *uint  `gorm:"index;comment:行业ID"`
	PromptSource       string `gorm:"size:64;comment:Prompt来源"`
	SelectedPromptID   *uint  `gorm:"index;comment:命中的Prompt模板ID"`
	SelectedPromptName string `gorm:"size:255;comment:命中的Prompt模板名称"`
	RenderedPrompt     string `gorm:"type:text;comment:渲染后的Prompt"`
	RequestMessages    string `gorm:"type:text;comment:请求消息JSON"`
	RuntimeConfig      string `gorm:"type:text;comment:运行时配置JSON"`
	SceneConfig        string `gorm:"type:text;comment:场景配置JSON"`
	Provider           string `gorm:"size:64;comment:Provider"`
	Model              string `gorm:"size:128;comment:模型"`
	UserInput          string `gorm:"type:text;comment:用户输入"`
	ModelOutput        string `gorm:"type:text;comment:模型输出"`
	ModelError         string `gorm:"type:text;comment:模型错误"`
	LatencyMs          int64  `gorm:"not null;default:0;comment:耗时毫秒"`
	IsSuccess          bool   `gorm:"not null;default:false;index;comment:是否成功"`
	InputTokens        int    `gorm:"not null;default:0;comment:输入token数"`
	OutputTokens       int    `gorm:"not null;default:0;comment:输出token数"`
	Status             string `gorm:"size:20;comment:状态"`
	ErrorMsg           string `gorm:"type:text;comment:错误信息"`
}

func (AICallLog) TableName() string { return "ai_call_logs" }

// ==================== 仓库接口 ====================

// AIConfigRepo AI 配置仓库接口
type AIConfigRepo interface {
	// GetActiveConfig 根据场景获取当前生效的 AI 配置
	GetActiveConfig(ctx context.Context, scene string) (*AIConfig, error)
}

// PromptRepo Prompt 模板仓库接口
type PromptRepo interface {
	// GetActiveTemplate 根据场景获取最新版本的生效模板
	GetActiveTemplate(ctx context.Context, scene string) (*PromptTemplate, error)
}

// CallLogRepo AI 调用日志仓库接口
type CallLogRepo interface {
	// Create 创建调用日志记录
	Create(ctx context.Context, log *AICallLog) error
}

// ==================== 各场景 UseCase ====================

// InterviewAgentUseCase 面试出题用例
type InterviewAgentUseCase struct {
	configRepo AIConfigRepo
	promptRepo PromptRepo
	callLogRepo CallLogRepo
	llm        LLMClient
	logger     log.Logger
}

// NewInterviewAgentUseCase 创建面试出题用例
func NewInterviewAgentUseCase(configRepo AIConfigRepo, promptRepo PromptRepo, callLogRepo CallLogRepo, llm LLMClient, logger log.Logger) *InterviewAgentUseCase {
	return &InterviewAgentUseCase{configRepo: configRepo, promptRepo: promptRepo, callLogRepo: callLogRepo, llm: llm, logger: logger}
}

// InterviewResult 面试出题返回结果
type InterviewResult struct {
	Question      string  `json:"question"`
	Topic         string  `json:"topic"`
	Difficulty    string  `json:"difficulty"`
	Type          string  `json:"type"`
	Hints         string  `json:"hints"`
	Feedback      string  `json:"feedback"`
	Score         float64 `json:"score"`
	ShouldEnd     bool    `json:"should_end"`
	Live2DEmotion string  `json:"live2d_emotion"`
	Live2DAction  string  `json:"live2d_action"`
}

// GenerateQuestion 生成面试题目或对用户回答进行反馈
func (uc *InterviewAgentUseCase) GenerateQuestion(ctx context.Context, industryCode, difficulty, userAnswer, resumeText, jobDescription string, history []Message, questionIndex int32) (*InterviewResult, error) {
	const scene = "interview_agent"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	tpl, err := uc.promptRepo.GetActiveTemplate(ctx, scene)
	if err != nil {
		return nil, ErrPromptRenderFailed
	}

	promptText := RenderPrompt(tpl.TemplateContent, map[string]string{
		"industry_code":   industryCode,
		"difficulty":      difficulty,
		"user_answer":     userAnswer,
		"resume_text":     resumeText,
		"job_description": jobDescription,
		"question_index":  fmt.Sprintf("%d", questionIndex),
	})

	schema := interviewResultSchema()
	messages := []Message{{Role: "system", Content: buildJSONContractPrompt(promptText, schema)}}
	messages = append(messages, history...)

	resp, err := uc.llm.Chat(ctx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, ErrLLMCallFailed
	}

	result, err := parseStructuredJSON[InterviewResult](ctx, uc.llm, cfg, resp.Content, schema)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// saveLog 记录 LLM 调用日志
func (uc *InterviewAgentUseCase) saveLog(ctx context.Context, scene, model string, resp *LLMResponse, callErr error, latencyMs int64) {
	logEntry := &AICallLog{Scene: scene, Model: model, LatencyMs: latencyMs}
	if resp != nil {
		logEntry.InputTokens = resp.InputTokens
		logEntry.OutputTokens = resp.OutputTokens
	}
	if callErr != nil {
		logEntry.Status = "error"
		logEntry.ErrorMsg = callErr.Error()
	} else {
		logEntry.Status = "success"
	}
	// FIX: 记录日志写入失败，但不影响主流程
	if err := uc.callLogRepo.Create(ctx, logEntry); err != nil {
		log.Warnf("写入AI调用日志失败: %v", err)
	}
}

// PlanAgentUseCase 学习计划生成用例
type PlanAgentUseCase struct {
	configRepo  AIConfigRepo
	promptRepo  PromptRepo
	callLogRepo CallLogRepo
	llm         LLMClient
	logger      log.Logger
}

// NewPlanAgentUseCase 创建学习计划生成用例
func NewPlanAgentUseCase(configRepo AIConfigRepo, promptRepo PromptRepo, callLogRepo CallLogRepo, llm LLMClient, logger log.Logger) *PlanAgentUseCase {
	return &PlanAgentUseCase{configRepo: configRepo, promptRepo: promptRepo, callLogRepo: callLogRepo, llm: llm, logger: logger}
}

// PlanResult 学习计划返回结果
type PlanResult struct {
	PlanTitle string     `json:"plan_title"`
	Tasks     []PlanTask `json:"tasks"`
	Summary   string     `json:"summary"`
}

// PlanTask 计划任务项
type PlanTask struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	Phase          string `json:"phase"`
	OrderIndex     int32  `json:"order_index"`
	EstimatedHours int32  `json:"estimated_hours"`
}

// GeneratePlan 生成个性化学习计划
func (uc *PlanAgentUseCase) GeneratePlan(ctx context.Context, industryCode, goal string, dailyHours int32, weakTopics, recentActivities []string) (*PlanResult, error) {
	const scene = "plan_agent"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	tpl, err := uc.promptRepo.GetActiveTemplate(ctx, scene)
	if err != nil {
		return nil, ErrPromptRenderFailed
	}

	promptText := RenderPrompt(tpl.TemplateContent, map[string]string{
		"industry_code":      industryCode,
		"goal":               goal,
		"daily_hours":        fmt.Sprintf("%d", dailyHours),
		"weak_topics":        joinStrings(weakTopics),
		"recent_activities":  joinStrings(recentActivities),
	})

	schema := planResultSchema()
	messages := []Message{{Role: "user", Content: buildJSONContractPrompt(promptText, schema)}}

	resp, err := uc.llm.Chat(ctx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, ErrLLMCallFailed
	}

	result, err := parseStructuredJSON[PlanResult](ctx, uc.llm, cfg, resp.Content, schema)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// saveLog 记录 LLM 调用日志
func (uc *PlanAgentUseCase) saveLog(ctx context.Context, scene, model string, resp *LLMResponse, callErr error, latencyMs int64) {
	logEntry := &AICallLog{Scene: scene, Model: model, LatencyMs: latencyMs}
	if resp != nil {
		logEntry.InputTokens = resp.InputTokens
		logEntry.OutputTokens = resp.OutputTokens
	}
	if callErr != nil {
		logEntry.Status = "error"
		logEntry.ErrorMsg = callErr.Error()
	} else {
		logEntry.Status = "success"
	}
	// FIX: 记录日志写入失败，但不影响主流程
	if err := uc.callLogRepo.Create(ctx, logEntry); err != nil {
		log.Warnf("写入AI调用日志失败: %v", err)
	}
}

// CompanionAgentUseCase AI 陪伴聊天用例
type CompanionAgentUseCase struct {
	configRepo  AIConfigRepo
	promptRepo  PromptRepo
	callLogRepo CallLogRepo
	llm         LLMClient
	logger      log.Logger
}

// NewCompanionAgentUseCase 创建 AI 陪伴聊天用例
func NewCompanionAgentUseCase(configRepo AIConfigRepo, promptRepo PromptRepo, callLogRepo CallLogRepo, llm LLMClient, logger log.Logger) *CompanionAgentUseCase {
	return &CompanionAgentUseCase{configRepo: configRepo, promptRepo: promptRepo, callLogRepo: callLogRepo, llm: llm, logger: logger}
}

// CompanionResult 陪伴聊天返回结果
type CompanionResult struct {
	Reply          string              `json:"reply"`
	Emotion        string              `json:"emotion"`
	Suggestions    []string            `json:"suggestions"`
	Action         string              `json:"action"`
	Live2DDirective *Live2DDirectiveResult `json:"live2d_directive,omitempty"`
}

// Chat 生成陪伴聊天回复
func (uc *CompanionAgentUseCase) Chat(ctx context.Context, userMessage, contextType, username string, recentTopics []string) (*CompanionResult, error) {
	const scene = "companion_agent"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	tpl, err := uc.promptRepo.GetActiveTemplate(ctx, scene)
	if err != nil {
		return nil, ErrPromptRenderFailed
	}

	// 对齐单体实现：Prompt 模板作为 system message，用户消息作为 user message
	promptText := renderCompanionPrompt(tpl.TemplateContent, userMessage, contextType, username, recentTopics)

	// 构建消息列表：system prompt + user message（对齐单体 prependSystemPrompt）
	messages := []Message{
		{Role: "system", Content: promptText},
		{Role: "user", Content: userMessage},
	}

	resp, err := uc.llm.Chat(ctx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, ErrLLMCallFailed
	}

	// 对齐单体实现：陪伴回复为纯文本，不解析 JSON；emotion 本地推导。
	reply := strings.TrimSpace(resp.Content)
	if reply == "" {
		return nil, ErrParseFailed
	}
	emotion := normalizeCompanionEmotion(contextType)
	return &CompanionResult{
		Reply:       reply,
		Emotion:     emotion,
		Suggestions: []string{},
		Action:      companionActionForEmotion(emotion),
	}, nil
}

// renderCompanionPrompt 渲染陪伴场景 Prompt 模板（对齐单体 renderPrompt）
func renderCompanionPrompt(template, userMessage, contextType, username string, recentTopics []string) string {
	rendered := strings.TrimSpace(template)
	if rendered == "" {
		return ""
	}

	// 对齐单体变量名
	vars := map[string]string{
		"user_emotion":        contextType,
		"latest_user_message": userMessage,
		"recent_topics":       joinStrings(recentTopics),
		"username":            username,
	}

	for key, value := range vars {
		placeholder := "{{" + key + "}}"
		rendered = strings.ReplaceAll(rendered, placeholder, strings.TrimSpace(value))
		rendered = strings.ReplaceAll(rendered, "{{ "+key+" }}", strings.TrimSpace(value))
	}

	return rendered
}

// GetGreeting 生成本地欢迎语（对齐单体 CompanionAgent.GetGreeting）
func (uc *CompanionAgentUseCase) GetGreeting(ctx context.Context, level, timeOfDay string) (*CompanionResult, error) {
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

	if strings.EqualFold(strings.TrimSpace(level), "beginner") {
		content += " 先稳住基础，不用追求一步到位。"
	}
	if strings.EqualFold(strings.TrimSpace(level), "advanced") {
		content += " 今天可以主动挑战一个更难的问题。"
	}

	return &CompanionResult{
		Reply:   content,
		Emotion: emotion,
		Action:  action,
	}, nil
}

// GetEncouragement 生成本地鼓励语（对齐单体 CompanionAgent.GetEncouragement）
func (uc *CompanionAgentUseCase) GetEncouragement(ctx context.Context, achievement string) (*CompanionResult, error) {
	achievement = strings.TrimSpace(achievement)
	if achievement == "" {
		achievement = "当前这一步"
	}

	return &CompanionResult{
		Reply:   achievement + " 做得不错，继续保持这个节奏，不要被短期波动打断。",
		Emotion: "encouraging",
		Action:  "nod",
	}, nil
}

// companionActionForEmotion 根据情绪选择默认动作（对齐单体 companionActionForEmotion）
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

// normalizeCompanionEmotion 规范化陪伴场景情绪值，对齐单体本地推导逻辑。
func normalizeCompanionEmotion(emotionHint string) string {
	switch strings.ToLower(strings.TrimSpace(emotionHint)) {
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

// saveLog 记录 LLM 调用日志
func (uc *CompanionAgentUseCase) saveLog(ctx context.Context, scene, model string, resp *LLMResponse, callErr error, latencyMs int64) {
	logEntry := &AICallLog{Scene: scene, Model: model, LatencyMs: latencyMs}
	if resp != nil {
		logEntry.InputTokens = resp.InputTokens
		logEntry.OutputTokens = resp.OutputTokens
	}
	if callErr != nil {
		logEntry.Status = "error"
		logEntry.ErrorMsg = callErr.Error()
	} else {
		logEntry.Status = "success"
	}
	// FIX: 记录日志写入失败，但不影响主流程
	if err := uc.callLogRepo.Create(ctx, logEntry); err != nil {
		log.Warnf("写入AI调用日志失败: %v", err)
	}
}

// QuizAnalyzerUseCase 答题分析评估用例
type QuizAnalyzerUseCase struct {
	configRepo  AIConfigRepo
	promptRepo  PromptRepo
	callLogRepo CallLogRepo
	llm         LLMClient
	logger      log.Logger
}

// NewQuizAnalyzerUseCase 创建答题分析评估用例
func NewQuizAnalyzerUseCase(configRepo AIConfigRepo, promptRepo PromptRepo, callLogRepo CallLogRepo, llm LLMClient, logger log.Logger) *QuizAnalyzerUseCase {
	return &QuizAnalyzerUseCase{configRepo: configRepo, promptRepo: promptRepo, callLogRepo: callLogRepo, llm: llm, logger: logger}
}

// QuizResult 答题分析返回结果
type QuizResult struct {
	Score         float64  `json:"score"`
	IsCorrect     bool     `json:"is_correct"`
	Feedback      string   `json:"feedback"`
	KeyPoints     []string `json:"key_points"`
	Suggestions   string   `json:"suggestions"`
	CorrectAnswer string   `json:"correct_answer"`
}

// Analyze 对用户答题进行分析评估
func (uc *QuizAnalyzerUseCase) Analyze(ctx context.Context, question, answer, topic, difficulty, questionType string) (*QuizResult, error) {
	const scene = "quiz_analyzer"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	tpl, err := uc.promptRepo.GetActiveTemplate(ctx, scene)
	if err != nil {
		return nil, ErrPromptRenderFailed
	}

	promptText := RenderPrompt(tpl.TemplateContent, map[string]string{
		"question":      question,
		"answer":        answer,
		"topic":         topic,
		"difficulty":    difficulty,
		"question_type": questionType,
	})

	schema := quizResultSchema()
	messages := []Message{{Role: "user", Content: buildJSONContractPrompt(promptText, schema)}}

	resp, err := uc.llm.Chat(ctx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, ErrLLMCallFailed
	}

	result, err := parseStructuredJSON[QuizResult](ctx, uc.llm, cfg, resp.Content, schema)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// saveLog 记录 LLM 调用日志
func (uc *QuizAnalyzerUseCase) saveLog(ctx context.Context, scene, model string, resp *LLMResponse, callErr error, latencyMs int64) {
	logEntry := &AICallLog{Scene: scene, Model: model, LatencyMs: latencyMs}
	if resp != nil {
		logEntry.InputTokens = resp.InputTokens
		logEntry.OutputTokens = resp.OutputTokens
	}
	if callErr != nil {
		logEntry.Status = "error"
		logEntry.ErrorMsg = callErr.Error()
	} else {
		logEntry.Status = "success"
	}
	// FIX: 记录日志写入失败，但不影响主流程
	if err := uc.callLogRepo.Create(ctx, logEntry); err != nil {
		log.Warnf("写入AI调用日志失败: %v", err)
	}
}

// ResumeParserUseCase 简历解析用例
type ResumeParserUseCase struct {
	configRepo  AIConfigRepo
	promptRepo  PromptRepo
	callLogRepo CallLogRepo
	llm         LLMClient
	logger      log.Logger
}

// NewResumeParserUseCase 创建简历解析用例
func NewResumeParserUseCase(configRepo AIConfigRepo, promptRepo PromptRepo, callLogRepo CallLogRepo, llm LLMClient, logger log.Logger) *ResumeParserUseCase {
	return &ResumeParserUseCase{configRepo: configRepo, promptRepo: promptRepo, callLogRepo: callLogRepo, llm: llm, logger: logger}
}

// ResumeResult 简历解析返回结果
type ResumeResult struct {
	Skills      []string `json:"skills"`
	Experience  []string `json:"experience"`
	Education   []string `json:"education"`
	Projects    []string `json:"projects"`
	Summary     string   `json:"summary"`
	WeakSignals []string `json:"weak_signals"`
}

// Parse 解析简历文本并提取结构化信息
func (uc *ResumeParserUseCase) Parse(ctx context.Context, resumeText string) (*ResumeResult, error) {
	const scene = "resume_parser"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	tpl, err := uc.promptRepo.GetActiveTemplate(ctx, scene)
	if err != nil {
		return nil, ErrPromptRenderFailed
	}

	promptText := RenderPrompt(tpl.TemplateContent, map[string]string{
		"resume_text": resumeText,
	})

	messages := []Message{{Role: "user", Content: promptText}}

	resp, err := uc.llm.Chat(ctx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, ErrLLMCallFailed
	}

	var result ResumeResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return nil, ErrParseFailed
	}
	return &result, nil
}

// saveLog 记录 LLM 调用日志
func (uc *ResumeParserUseCase) saveLog(ctx context.Context, scene, model string, resp *LLMResponse, callErr error, latencyMs int64) {
	logEntry := &AICallLog{Scene: scene, Model: model, LatencyMs: latencyMs}
	if resp != nil {
		logEntry.InputTokens = resp.InputTokens
		logEntry.OutputTokens = resp.OutputTokens
	}
	if callErr != nil {
		logEntry.Status = "error"
		logEntry.ErrorMsg = callErr.Error()
	} else {
		logEntry.Status = "success"
	}
	// FIX: 记录日志写入失败，但不影响主流程
	if err := uc.callLogRepo.Create(ctx, logEntry); err != nil {
		log.Warnf("写入AI调用日志失败: %v", err)
	}
}

// Live2DDirectorUseCase Live2D 角色控制指令生成用例
type Live2DDirectorUseCase struct {
	configRepo  AIConfigRepo
	promptRepo  PromptRepo
	callLogRepo CallLogRepo
	llm         LLMClient
	logger      log.Logger
}

// NewLive2DDirectorUseCase 创建 Live2D 指令生成用例
func NewLive2DDirectorUseCase(configRepo AIConfigRepo, promptRepo PromptRepo, callLogRepo CallLogRepo, llm LLMClient, logger log.Logger) *Live2DDirectorUseCase {
	return &Live2DDirectorUseCase{configRepo: configRepo, promptRepo: promptRepo, callLogRepo: callLogRepo, llm: llm, logger: logger}
}

// Live2DDirectiveResult Live2D 指令返回结果
type Live2DDirectiveResult struct {
	Emotion            string               `json:"emotion"`
	Action             string               `json:"action"`
	Reply              string               `json:"reply"`
	MotionKey          string               `json:"motion_key"`
	MotionGroup        string               `json:"motion_group"`
	MotionPriority     string               `json:"motion_priority"`
	MotionDurationMS   int                  `json:"motion_duration_ms"`
	Intensity          float64              `json:"intensity"`
	DurationMS         int                  `json:"duration_ms"`
	MouthOpen          *float64             `json:"mouth_open"`
	Source             string               `json:"source"`
	ExpressionMix      []ExpressionLayer    `json:"expression_mix"`
	ParameterOverrides []ParameterOverride  `json:"parameter_overrides"`
}

// ExpressionLayer Live2D 表情混合层
type ExpressionLayer struct {
	Key    string  `json:"key"`
	Weight float64 `json:"weight"`
}

// ParameterOverride Live2D 参数覆盖
type ParameterOverride struct {
	ID    string  `json:"id"`
	Value float64 `json:"value"`
}

// GenerateDirective 生成 Live2D 角色控制指令
func (uc *Live2DDirectorUseCase) GenerateDirective(ctx context.Context, contextText, emotionHint, replyText string) (*Live2DDirectiveResult, error) {
	const scene = "live2d_director"
	start := time.Now()

	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		return nil, ErrAIConfigNotFound
	}

	tpl, err := uc.promptRepo.GetActiveTemplate(ctx, scene)
	if err != nil {
		return nil, ErrPromptRenderFailed
	}

	promptText := RenderPrompt(tpl.TemplateContent, map[string]string{
		"context":      contextText,
		"emotion_hint": emotionHint,
		"reply_text":   replyText,
	})

	messages := []Message{{Role: "user", Content: promptText}}

	resp, err := uc.llm.Chat(ctx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, ErrLLMCallFailed
	}

	var result Live2DDirectiveResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return nil, ErrParseFailed
	}
	return &result, nil
}

// saveLog 记录 LLM 调用日志
func (uc *Live2DDirectorUseCase) saveLog(ctx context.Context, scene, model string, resp *LLMResponse, callErr error, latencyMs int64) {
	logEntry := &AICallLog{Scene: scene, Model: model, LatencyMs: latencyMs}
	if resp != nil {
		logEntry.InputTokens = resp.InputTokens
		logEntry.OutputTokens = resp.OutputTokens
	}
	if callErr != nil {
		logEntry.Status = "error"
		logEntry.ErrorMsg = callErr.Error()
	} else {
		logEntry.Status = "success"
	}
	// FIX: 记录日志写入失败，但不影响主流程
	if err := uc.callLogRepo.Create(ctx, logEntry); err != nil {
		log.Warnf("写入AI调用日志失败: %v", err)
	}
}

// joinStrings 将字符串切片用逗号连接
func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}
