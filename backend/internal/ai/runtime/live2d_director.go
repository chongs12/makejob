package runtime

import (
	"context"
	"fmt"
	"math"
	"strings"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
)

// live2dDirectivePayload 定义大模型返回的 Live2D 结构化控制输出。
type live2dDirectivePayload struct {
	Reply              string                         `json:"reply"`
	Emotion            string                         `json:"emotion"`
	Action             string                         `json:"action"`
	ExpressionMix      []ai.Live2DExpressionLayer    `json:"expression_mix"`
	ParameterOverrides []ai.Live2DParameterOverride  `json:"parameter_overrides"`
	MotionKey          string                         `json:"motion_key"`
	MotionGroup        string                         `json:"motion_group"`
	MotionPriority     string                         `json:"motion_priority"`
	MotionDurationMS   int                            `json:"motion_duration_ms"`
	Intensity          float64                        `json:"intensity"`
	DurationMS         int                            `json:"duration_ms"`
	MouthOpen          *float64                       `json:"mouth_open"`
}

// providerLive2DDirector 基于场景 Provider 生成结构化 Live2D 控制指令。
type providerLive2DDirector struct {
	provider ai.AIProvider
	prompts  *promptResolver
	logger   *aiCallLogRecorder
}

// newLive2DDirector 创建 Live2D 指令生成器。
func newLive2DDirector(provider ai.AIProvider, prompts *promptResolver, logger *aiCallLogRecorder) ai.Live2DDirectiveGenerator {
	return &providerLive2DDirector{
		provider: provider,
		prompts:  prompts,
		logger:   logger,
	}
}

// GenerateDirective 根据回复语义和模型清单生成结构化 Live2D 指令。
func (d *providerLive2DDirector) GenerateDirective(ctx context.Context, req ai.Live2DDirectiveContext) (*ai.Live2DDirective, error) {
	if d == nil || d.provider == nil {
		return nil, fmt.Errorf("live2d director provider is unavailable")
	}
	if strings.TrimSpace(req.AssistantReply) == "" {
		return nil, nil
	}
	if strings.TrimSpace(req.Model.ModelKey) == "" {
		return nil, fmt.Errorf("live2d manifest model key is required")
	}

	systemPrompt := d.buildSystemPrompt(ctx, req)
	userPrompt := buildLive2DDirectiveUserPrompt(req)
	messages := []ai.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	payload, _, err := callStructuredJSON[live2dDirectivePayload](ctx, d.provider, messages, live2dDirectivePayloadSchema())
	if err != nil {
		return nil, err
	}

	directive := normalizeLive2DDirectivePayload(payload, req)
	return directive, nil
}

// buildSystemPrompt 组合场景提示词和 Live2D 模型白名单提示。
func (d *providerLive2DDirector) buildSystemPrompt(ctx context.Context, req ai.Live2DDirectiveContext) string {
	scenePrompt := builtInScenePrompts[model.PromptSceneCompanion]
	switch strings.TrimSpace(req.Scene) {
	case model.Live2DSceneInterview:
		scenePrompt = builtInScenePrompts[model.PromptSceneInterview]
	}

	if d.prompts != nil {
		sceneKey := model.PromptSceneCompanion
		if strings.TrimSpace(req.Scene) == model.Live2DSceneInterview {
			sceneKey = model.PromptSceneInterview
		}
		scenePrompt = d.prompts.ResolveByIndustryID(ctx, sceneKey, nil, req.AdditionalContext)
	}

	return strings.TrimSpace(scenePrompt) + `

你正在执行 Live2D 视觉控制任务。你必须严格输出一个 JSON 对象，不得输出解释、Markdown 或代码块。
只允许使用系统提供的 expression_mix 和 parameter_overrides 白名单。
expression_mix 最多 3 层，weight 范围 0 到 1。
parameter_overrides 只能使用当前模型 manifest 中列出的参数，value 必须位于对应范围内。
若无需特殊表情，可返回空数组。
嘴型 mouth_open 仅用于给前端提供当前回复推荐张口幅度，范围 0 到 1。`
}

// live2dDirectivePayloadSchema 返回结构化 Live2D 指令的 JSON 合同。
func live2dDirectivePayloadSchema() string {
	return `{
  "reply": "回复文本，可与输入 assistant reply 相同",
  "emotion": "happy|neutral|encouraging|thinking|serious|warning|praise",
  "action": "idle|wave|nod|celebrate|thinking|ask",
  "expression_mix": [
    { "key": "neutral", "weight": 1.0 }
  ],
  "parameter_overrides": [
    { "id": "ParamMouthForm", "value": 0.25 }
  ],
  "motion_key": "wave",
  "motion_group": "tapbody",
  "motion_priority": "normal|force",
  "motion_duration_ms": 1800,
  "intensity": 0.7,
  "duration_ms": 2400,
  "mouth_open": 0.22
}`
}

// buildLive2DDirectiveUserPrompt 构造一次指令生成所需的完整用户提示。
func buildLive2DDirectiveUserPrompt(req ai.Live2DDirectiveContext) string {
	expressionLines := make([]string, 0, len(req.Model.Expressions))
	for _, item := range req.Model.Expressions {
		expressionLines = append(expressionLines, fmt.Sprintf("- key=%s label=%s", strings.TrimSpace(item.Key), defaultLive2DLabel(item.Label, item.Key)))
	}

	parameterLines := make([]string, 0, len(req.Model.Parameters))
	for _, item := range req.Model.Parameters {
		parameterLines = append(parameterLines, fmt.Sprintf("- id=%s range=[%.4f, %.4f] label=%s", strings.TrimSpace(item.ID), item.Min, item.Max, defaultLive2DLabel(item.Label, item.ID)))
	}

	motionLines := make([]string, 0, len(req.Model.Motions))
	for _, item := range req.Model.Motions {
		motionLines = append(motionLines, fmt.Sprintf("- key=%s group=%s label=%s", strings.TrimSpace(item.Key), defaultLive2DValue(strings.TrimSpace(item.Group), "auto"), defaultLive2DLabel(item.Label, item.Key)))
	}

	historyLines := make([]string, 0, len(req.RecentMessages))
	for _, item := range req.RecentMessages {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		historyLines = append(historyLines, fmt.Sprintf("[%s] %s", strings.TrimSpace(item.Role), content))
	}

	currentDirective := "无"
	if req.CurrentDirective != nil {
		currentDirective = summarizeCurrentDirective(*req.CurrentDirective)
	}

	questionSummary := ""
	if req.Question != nil {
		questionSummary = fmt.Sprintf("\n当前题目：%s\n题型：%s\n题目主题：%s", strings.TrimSpace(req.Question.Question), strings.TrimSpace(req.Question.Type), strings.TrimSpace(req.Question.Topic))
	}

	return fmt.Sprintf(
		"场景：%s\n模型：%s (%s)\n候选表达式：\n%s\n候选参数：\n%s\n候选动作：\n%s\n用户最近消息：%s\n待配合的回复文本：%s\n用户情绪：%s\n上一轮角色状态：%s%s\n对话历史：\n%s\n\n请输出一条最贴近这段回复语义的 Live2D 控制指令。",
		strings.TrimSpace(req.Scene),
		strings.TrimSpace(req.Model.ModelName),
		strings.TrimSpace(req.Model.ModelKey),
		strings.Join(expressionLines, "\n"),
		strings.Join(parameterLines, "\n"),
		strings.Join(motionLines, "\n"),
		defaultLive2DValue(strings.TrimSpace(req.UserMessage), "无"),
		strings.TrimSpace(req.AssistantReply),
		defaultLive2DValue(strings.TrimSpace(req.UserEmotion), "neutral"),
		currentDirective,
		questionSummary,
		strings.Join(historyLines, "\n"),
	)
}

// normalizeLive2DDirectivePayload 过滤模型输出，只保留 manifest 白名单内的结构。
func normalizeLive2DDirectivePayload(payload live2dDirectivePayload, req ai.Live2DDirectiveContext) *ai.Live2DDirective {
	expressionAllowed := make(map[string]struct{}, len(req.Model.Expressions))
	for _, item := range req.Model.Expressions {
		key := strings.TrimSpace(item.Key)
		if key != "" {
			expressionAllowed[key] = struct{}{}
		}
	}

	parameterAllowed := make(map[string]ai.Live2DManifestParameter, len(req.Model.Parameters))
	for _, item := range req.Model.Parameters {
		id := strings.TrimSpace(item.ID)
		if id != "" {
			parameterAllowed[id] = item
		}
	}

	motionAllowed := make(map[string]ai.Live2DManifestMotion, len(req.Model.Motions))
	for _, item := range req.Model.Motions {
		key := strings.TrimSpace(item.Key)
		if key != "" {
			motionAllowed[key] = item
		}
	}

	expressionMix := make([]ai.Live2DExpressionLayer, 0, len(payload.ExpressionMix))
	expressionSeen := make(map[string]struct{}, len(payload.ExpressionMix))
	for _, item := range payload.ExpressionMix {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			continue
		}
		if _, ok := expressionAllowed[key]; !ok {
			continue
		}
		if _, exists := expressionSeen[key]; exists {
			continue
		}
		weight := clampFloat(item.Weight, 0, 1)
		if weight < 0.02 {
			continue
		}
		expressionSeen[key] = struct{}{}
		expressionMix = append(expressionMix, ai.Live2DExpressionLayer{
			Key:    key,
			Weight: roundFloat(weight, 4),
		})
		if len(expressionMix) >= 3 {
			break
		}
	}

	parameterOverrides := make([]ai.Live2DParameterOverride, 0, len(payload.ParameterOverrides))
	parameterSeen := make(map[string]struct{}, len(payload.ParameterOverrides))
	for _, item := range payload.ParameterOverrides {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		manifest, ok := parameterAllowed[id]
		if !ok {
			continue
		}
		if _, exists := parameterSeen[id]; exists {
			continue
		}
		parameterSeen[id] = struct{}{}
		parameterOverrides = append(parameterOverrides, ai.Live2DParameterOverride{
			ID:    id,
			Value: roundFloat(clampFloat(item.Value, manifest.Min, manifest.Max), 4),
		})
	}

	var mouthOpen *float64
	if payload.MouthOpen != nil {
		value := roundFloat(clampFloat(*payload.MouthOpen, 0, 1), 4)
		mouthOpen = &value
	}

	motionKey := ""
	motionGroup := ""
	if motion, ok := motionAllowed[strings.TrimSpace(payload.MotionKey)]; ok {
		motionKey = strings.TrimSpace(motion.Key)
		motionGroup = strings.TrimSpace(motion.Group)
	}

	return &ai.Live2DDirective{
		Reply:              defaultLive2DValue(strings.TrimSpace(payload.Reply), strings.TrimSpace(req.AssistantReply)),
		Emotion:            strings.TrimSpace(payload.Emotion),
		Action:             strings.TrimSpace(payload.Action),
		ExpressionMix:      expressionMix,
		ParameterOverrides: parameterOverrides,
		MotionKey:          motionKey,
		MotionGroup:        motionGroup,
		MotionPriority:     normalizeLive2DMotionPriority(payload.MotionPriority),
		MotionDurationMS:   normalizeLive2DMotionDuration(payload.MotionDurationMS),
		Intensity:          roundFloat(clampFloat(payload.Intensity, 0, 1), 4),
		DurationMS:         clampInt(payload.DurationMS, 300, 8000),
		MouthOpen:          mouthOpen,
		Source:             "llm_structured",
	}
}

// summarizeCurrentDirective 生成上一轮角色状态摘要，帮助模型维持状态连续性。
func summarizeCurrentDirective(directive ai.Live2DDirective) string {
	expressionKeys := make([]string, 0, len(directive.ExpressionMix))
	for _, item := range directive.ExpressionMix {
		if strings.TrimSpace(item.Key) == "" {
			continue
		}
		expressionKeys = append(expressionKeys, fmt.Sprintf("%s:%.2f", strings.TrimSpace(item.Key), item.Weight))
	}
	parameterKeys := make([]string, 0, len(directive.ParameterOverrides))
	for _, item := range directive.ParameterOverrides {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		parameterKeys = append(parameterKeys, fmt.Sprintf("%s=%.2f", strings.TrimSpace(item.ID), item.Value))
	}
	return fmt.Sprintf(
		"emotion=%s action=%s motion=%s/%s expressions=[%s] parameters=[%s]",
		defaultLive2DValue(strings.TrimSpace(directive.Emotion), "neutral"),
		defaultLive2DValue(strings.TrimSpace(directive.Action), "idle"),
		defaultLive2DValue(strings.TrimSpace(directive.MotionGroup), "none"),
		defaultLive2DValue(strings.TrimSpace(directive.MotionKey), "none"),
		strings.Join(expressionKeys, ", "),
		strings.Join(parameterKeys, ", "),
	)
}

// normalizeLive2DMotionPriority 规整动作优先级，避免模型输出无效值。
func normalizeLive2DMotionPriority(priority string) string {
	switch strings.TrimSpace(strings.ToLower(priority)) {
	case "force":
		return "force"
	default:
		return "normal"
	}
}

// normalizeLive2DMotionDuration 规整动作时长，便于前端做统一节流和展示。
func normalizeLive2DMotionDuration(durationMS int) int {
	if durationMS <= 0 {
		return 0
	}
	return clampInt(durationMS, 300, 12000)
}

// defaultLive2DLabel 返回展示时可读的 label。
func defaultLive2DLabel(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return strings.TrimSpace(fallback)
	}
	return strings.TrimSpace(value)
}

// defaultLive2DValue 返回非空值，否则回退到默认值。
func defaultLive2DValue(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return strings.TrimSpace(fallback)
	}
	return strings.TrimSpace(value)
}

// clampFloat 约束浮点值范围。
func clampFloat(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// roundFloat 对浮点值做指定位数规整。
func roundFloat(value float64, precision int) float64 {
	if precision <= 0 {
		return math.Round(value)
	}
	factor := math.Pow10(precision)
	return math.Round(value*factor) / factor
}

// clampInt 约束整数范围。
func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
