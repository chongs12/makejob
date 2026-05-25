package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"makejob-backend/internal/ai"
)

type resumeParser struct {
	provider ai.AIProvider
	prompts  *promptResolver
	logger   *aiCallLogRecorder
}

func newResumeParser(provider ai.AIProvider, prompts *promptResolver, logger *aiCallLogRecorder) ai.ResumeParser {
	return &resumeParser{
		provider: provider,
		prompts:  prompts,
		logger:   logger,
	}
}

type resumeProfilePayload struct {
	Summary     string   `json:"summary"`
	Skills      []string `json:"skills"`
	Projects    []string `json:"projects"`
	Strengths   []string `json:"strengths"`
	WeakSignals []string `json:"weak_signals"`
}

func resumeProfilePayloadSchema() string {
	return `{
  "type": "object",
  "properties": {
    "summary": {"type": "string", "description": "一段话概括候选人背景"},
    "skills": {"type": "array", "items": {"type": "string"}, "description": "核心技术栈"},
    "projects": {"type": "array", "items": {"type": "string"}, "description": "重点项目经历简述"},
    "strengths": {"type": "array", "items": {"type": "string"}, "description": "简历中体现的优势"},
    "weak_signals": {"type": "array", "items": {"type": "string"}, "description": "简历中可能的薄弱信号"}
  },
  "required": ["summary", "skills", "projects", "strengths", "weak_signals"]
}`
}

func (p *resumeParser) Parse(ctx context.Context, resumeText string, jobDescription string) (*ai.ResumeProfile, error) {
	if p.provider == nil {
		return nil, fmt.Errorf("ai provider is unavailable for resume parsing")
	}
	resumeText = strings.TrimSpace(resumeText)
	if resumeText == "" {
		return nil, fmt.Errorf("resume text is empty")
	}

	systemPrompt := "你是一位资深的技术招聘专家。请从候选人简历中提取结构化画像。输出严格 JSON。"
	userPrompt := fmt.Sprintf("以下是候选人简历原文：\n%s", resumeText)
	if jd := strings.TrimSpace(jobDescription); jd != "" {
		userPrompt += fmt.Sprintf("\n\n目标岗位描述：\n%s", jd)
		userPrompt += "\n\n请结合岗位要求，重点分析候选人与该岗位的匹配度和潜在薄弱点。"
	}

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	startedAt := time.Now()
	payload, _, usage, err := callStructuredJSON[resumeProfilePayload](ctx, p.provider, messages, resumeProfilePayloadSchema())
	if err != nil {
		p.recordCall(ctx, userPrompt, messages, "", err, startedAt, usage.InputTokens, usage.OutputTokens)
		return nil, fmt.Errorf("parse resume failed: %w", err)
	}

	output, _ := json.Marshal(payload)
	p.recordCall(ctx, userPrompt, messages, string(output), nil, startedAt, usage.InputTokens, usage.OutputTokens)

	return &ai.ResumeProfile{
		Summary:     strings.TrimSpace(payload.Summary),
		Skills:      normalizeStringSlice(payload.Skills),
		Projects:    normalizeStringSlice(payload.Projects),
		Strengths:   normalizeStringSlice(payload.Strengths),
		WeakSignals: normalizeStringSlice(payload.WeakSignals),
	}, nil
}

// recordCall 记录一次简历解析的运行时模型调用。
func (p *resumeParser) recordCall(ctx context.Context, userInput string, messages []ai.Message, response string, err error, startedAt time.Time, inputTokens int, outputTokens int) {
	if p.logger == nil {
		return
	}

	p.logger.Record(ctx, runtimeCallLogEntry{
		Request:      messages,
		UserInput:    userInput,
		Model:        p.provider.GetModelName(),
		Output:       response,
		Err:          err,
		StartedAt:    startedAt,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	})
}
