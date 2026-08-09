package biz

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// AdminUseCase AI Gateway 管理调试用例
type AdminUseCase struct {
	configRepo  AIConfigRepo
	promptRepo  PromptRepo
	callLogRepo CallLogRepo
	llm         LLMClient
	logger      log.Logger
}

// NewAdminUseCase 创建管理调试用例
func NewAdminUseCase(configRepo AIConfigRepo, promptRepo PromptRepo, callLogRepo CallLogRepo, llm LLMClient, logger log.Logger) *AdminUseCase {
	return &AdminUseCase{
		configRepo:  configRepo,
		promptRepo:  promptRepo,
		callLogRepo: callLogRepo,
		llm:         llm,
		logger:      logger,
	}
}

// RenderPromptResult Prompt 渲染结果
type RenderPromptResult struct {
	RenderedPrompt    string
	ResolvedVariables map[string]string
	LLMResponse       string
	Model             string
	LatencyMs         int64
}

// RenderPrompt 渲染 Prompt 模板，可选调用 LLM
func (uc *AdminUseCase) RenderPrompt(ctx context.Context, scene, templateText string, variables map[string]string, runWithLLM bool) (*RenderPromptResult, error) {
	// 如果没有传入模板文本，从 DB 加载
	if strings.TrimSpace(templateText) == "" {
		if scene == "" {
			return nil, ErrPromptRenderFailed
		}
		tpl, err := uc.promptRepo.GetActiveTemplate(ctx, scene)
		if err != nil {
			return nil, ErrPromptRenderFailed.WithCause(err)
		}
		templateText = tpl.TemplateContent
	}

	// 渲染 prompt
	renderedPrompt := RenderPrompt(templateText, variables)

	result := &RenderPromptResult{
		RenderedPrompt:    renderedPrompt,
		ResolvedVariables: variables,
	}

	// 如果需要调用 LLM
	if runWithLLM {
		cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
		if err != nil {
			cfg = &AIConfig{Model: "default"}
		}

		messages := []Message{{Role: "user", Content: renderedPrompt}}
		start := time.Now()
		resp, err := uc.llm.Chat(ctx, messages, cfg)
		latencyMs := time.Since(start).Milliseconds()

		if err != nil {
			uc.saveLog(ctx, scene, cfg.Model, nil, err, latencyMs)
			return nil, ErrLLMCallFailed.WithCause(err)
		}

		result.LLMResponse = resp.Content
		result.Model = cfg.Model
		result.LatencyMs = latencyMs
	}

	return result, nil
}

// DebugAIResult AI 调试结果
type DebugAIResult struct {
	RenderedPrompt string
	Response       string
	Model          string
	InputTokens    int
	OutputTokens   int
	LatencyMs      int64
	Error          string
}

// DebugAI 执行 AI 调试调用
func (uc *AdminUseCase) DebugAI(ctx context.Context, scene, prompt string, params map[string]string, modelOverride string) (*DebugAIResult, error) {
	// 获取配置
	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		cfg = &AIConfig{Model: "default", Temperature: 0.7, MaxTokens: 2048}
	}

	// 模型覆盖
	if modelOverride != "" {
		cfg.Model = modelOverride
	}

	// 渲染 prompt
	renderedPrompt := prompt
	if len(params) > 0 {
		renderedPrompt = RenderPrompt(prompt, params)
	}

	// 调用 LLM
	messages := []Message{{Role: "user", Content: renderedPrompt}}
	start := time.Now()
	resp, callErr := uc.llm.Chat(ctx, messages, cfg)
	latencyMs := time.Since(start).Milliseconds()

	result := &DebugAIResult{
		RenderedPrompt: renderedPrompt,
		Model:          cfg.Model,
		LatencyMs:      latencyMs,
	}

	if callErr != nil {
		result.Error = callErr.Error()
		// 记录日志
		uc.saveLog(ctx, scene, cfg.Model, nil, callErr, latencyMs)
		return result, nil
	}

	result.Response = resp.Content
	result.InputTokens = resp.InputTokens
	result.OutputTokens = resp.OutputTokens

	// 记录日志
	uc.saveLog(ctx, scene, cfg.Model, resp, nil, latencyMs)

	return result, nil
}

// QuestionCandidate 题目候选（FIX H6: 补齐与 PipelineCard 对齐的字段）
type QuestionCandidate struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Type        string   `json:"type"`
	Difficulty  string   `json:"difficulty"`
	Category    string   `json:"category"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
	Tags        []string `json:"tags"`
	SourceType  string   `json:"source_type"`
	Confidence  float64  `json:"confidence"`
	Solution    string   `json:"solution,omitempty"`
	JudgeConfig any      `json:"judge_config,omitempty"` // 使用 any 类型，序列化时会作为对象
	SourceLabel string   `json:"source_label,omitempty"`
	SourceTitle string   `json:"source_title,omitempty"`
	SourceURL   string   `json:"source_url,omitempty"`
}

// GenerateQuestionCandidatesResult 同步候选题生成结果
type GenerateQuestionCandidatesResult struct {
	IndustryCode string
	Requirement  string
	Candidates   []*QuestionCandidate
	Warnings     []string
}

// GenerateQuestionCandidates 同步生成题目候选。
// 采用逐张生成模式（复刻单体架构），每张题卡独立调用 AI，带上下文避免重复。
func (uc *AdminUseCase) GenerateQuestionCandidates(ctx context.Context, industryCode, requirement, agentPrompt string, candidateCount int32, generationMode string, includeScraped, includeGenerated bool, sources []string, industryName string, categories []string) (*GenerateQuestionCandidatesResult, error) {
	// 使用逐张生成模式
	return uc.GenerateQuestionCandidatesDirect(ctx, industryCode, requirement, agentPrompt, candidateCount, industryName, categories)
}

// saveLog 记录 LLM 调用日志
func (uc *AdminUseCase) saveLog(ctx context.Context, scene, model string, resp *LLMResponse, callErr error, latencyMs int64) {
	logEntry := &AICallLog{
		TraceID:   fmt.Sprintf("%d", time.Now().UnixNano()),
		Source:    "ai_gateway",
		Scene:     scene,
		Model:     model,
		LatencyMs: latencyMs,
	}
	if resp != nil {
		logEntry.InputTokens = resp.InputTokens
		logEntry.OutputTokens = resp.OutputTokens
		logEntry.IsSuccess = true
		logEntry.Status = "success"
	}
	if callErr != nil {
		logEntry.IsSuccess = false
		logEntry.Status = "error"
		logEntry.ErrorMsg = callErr.Error()
	}
	recordAICallMetrics(scene, model, logEntry.Status, logEntry.InputTokens, logEntry.OutputTokens, latencyMs)
	logCtx, cancel := newCallLogContext(ctx)
	defer cancel()
	if err := uc.callLogRepo.Create(logCtx, logEntry); err != nil {
		log.Warnf("写入AI调用日志失败: %v", err)
	}
}

// sanitizeLLMOutput 清理 LLM 输出，移除代码块、推理块等
func sanitizeLLMOutput(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// 移除代码块标记
	if strings.HasPrefix(raw, "```") {
		if lineEnd := strings.Index(raw, "\n"); lineEnd >= 0 {
			raw = raw[lineEnd+1:]
		} else {
			raw = strings.TrimPrefix(raw, "```")
		}
	}
	raw = strings.TrimSuffix(strings.TrimSpace(raw), "```")

	return strings.TrimSpace(raw)
}

// extractJSON 从文本中提取 JSON 对象或数组
func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// 尝试提取 JSON 对象
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return strings.TrimSpace(raw[start : end+1])
	}

	// 尝试提取 JSON 数组
	start = strings.Index(raw, "[")
	end = strings.LastIndex(raw, "]")
	if start >= 0 && end > start {
		return strings.TrimSpace(raw[start : end+1])
	}

	return ""
}
