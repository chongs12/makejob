package biz

import (
	"context"
	"encoding/json"
	"fmt"
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

// PromptTemplate Prompt 模板实体
type PromptTemplate struct {
	model.BaseModel
	Scene         string `gorm:"size:50;not null;index"`
	Version       int    `gorm:"not null;default:1"`
	TemplateText  string `gorm:"type:text;not null"`
	VariablesJSON string `gorm:"type:text"`
	IsActive      bool   `gorm:"not null;default:true;index"`
}

func (PromptTemplate) TableName() string { return "prompt_templates" }

// AICallLog AI 调用日志实体
// FIX: 嵌入model.BaseModel，移除手写的ID和CreatedAt字段
type AICallLog struct {
	model.BaseModel
	Scene        string `gorm:"size:50;not null;index"`
	Model        string `gorm:"size:100"`
	InputTokens  int    `gorm:"not null;default:0"`
	OutputTokens int    `gorm:"not null;default:0"`
	LatencyMs    int64  `gorm:"not null;default:0"`
	Status       string `gorm:"size:20"`
	ErrorMsg     string `gorm:"type:text"`
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
	Question      string
	Topic         string
	Difficulty    string
	Type          string
	Hints         string
	Feedback      string
	Score         float64
	ShouldEnd     bool
	Live2DEmotion string
	Live2DAction  string
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

	promptText := RenderPrompt(tpl.TemplateText, map[string]string{
		"industry_code":   industryCode,
		"difficulty":      difficulty,
		"user_answer":     userAnswer,
		"resume_text":     resumeText,
		"job_description": jobDescription,
		"question_index":  fmt.Sprintf("%d", questionIndex),
	})

	messages := []Message{{Role: "system", Content: promptText}}
	messages = append(messages, history...)

	resp, err := uc.llm.Chat(ctx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, ErrLLMCallFailed
	}

	var result InterviewResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return nil, ErrParseFailed
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

	promptText := RenderPrompt(tpl.TemplateText, map[string]string{
		"industry_code":      industryCode,
		"goal":               goal,
		"daily_hours":        fmt.Sprintf("%d", dailyHours),
		"weak_topics":        joinStrings(weakTopics),
		"recent_activities":  joinStrings(recentActivities),
	})

	messages := []Message{{Role: "user", Content: promptText}}

	resp, err := uc.llm.Chat(ctx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, ErrLLMCallFailed
	}

	var result PlanResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return nil, ErrParseFailed
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
	Reply       string   `json:"reply"`
	Emotion     string   `json:"emotion"`
	Suggestions []string `json:"suggestions"`
}

// Chat 生成陪伴聊天回复
func (uc *CompanionAgentUseCase) Chat(ctx context.Context, userMessage, contextType string, recentTopics []string) (*CompanionResult, error) {
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

	promptText := RenderPrompt(tpl.TemplateText, map[string]string{
		"user_message":   userMessage,
		"context_type":   contextType,
		"recent_topics":  joinStrings(recentTopics),
	})

	messages := []Message{{Role: "user", Content: promptText}}

	resp, err := uc.llm.Chat(ctx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, ErrLLMCallFailed
	}

	var result CompanionResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return nil, ErrParseFailed
	}
	return &result, nil
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

	promptText := RenderPrompt(tpl.TemplateText, map[string]string{
		"question":      question,
		"answer":        answer,
		"topic":         topic,
		"difficulty":    difficulty,
		"question_type": questionType,
	})

	messages := []Message{{Role: "user", Content: promptText}}

	resp, err := uc.llm.Chat(ctx, messages, cfg)
	uc.saveLog(ctx, scene, cfg.Model, resp, err, time.Since(start).Milliseconds())
	if err != nil {
		return nil, ErrLLMCallFailed
	}

	var result QuizResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return nil, ErrParseFailed
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
	Skills     []string `json:"skills"`
	Experience []string `json:"experience"`
	Education  []string `json:"education"`
	Projects   []string `json:"projects"`
	Summary    string   `json:"summary"`
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

	promptText := RenderPrompt(tpl.TemplateText, map[string]string{
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
	Emotion     string `json:"emotion"`
	Action      string `json:"action"`
	Reply       string `json:"reply"`
	MotionKey   string `json:"motion_key"`
	MotionGroup string `json:"motion_group"`
	DurationMs  int32  `json:"duration_ms"`
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

	promptText := RenderPrompt(tpl.TemplateText, map[string]string{
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
