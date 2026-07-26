package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// buildJSONContractPrompt 在基础 Prompt 末尾追加结构化输出约束，要求模型仅返回 JSON。
// 对齐单体 runtime 的 buildXxxSystemPrompt 做法：种子 Prompt 本身是对话式的，
// 需显式追加 JSON 合同，模型才会返回可解析的结构化结果。
func buildJSONContractPrompt(basePrompt, schema string) string {
	return strings.TrimSpace(basePrompt) + "\n\n" +
		"你正在执行结构化输出任务。你必须只返回一个 JSON 对象，不要返回 Markdown、解释、代码块或额外文字。\n" +
		"JSON 结构如下：\n" + strings.TrimSpace(schema)
}

// extractJSONObject 从模型输出中提取 JSON 对象主体，兼容 ```json 围栏与前后说明文字。
// 直接移植单体 runtime.extractJSONObject。
func extractJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return ""
	}

	return strings.TrimSpace(trimmed[start : end+1])
}

// decodeJSONPayload 提取并解析模型输出为目标结构。
func decodeJSONPayload[T any](raw string) (T, error) {
	var payload T
	cleaned := extractJSONObject(raw)
	if cleaned == "" {
		return payload, fmt.Errorf("json payload not found")
	}
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

// buildJSONRepairMessages 构造一次性 JSON 修复请求，把散文输出重写为目标结构。
// 移植单体 runtime.buildJSONRepairMessages。
func buildJSONRepairMessages(schema, raw string) []Message {
	return []Message{
		{
			Role: "system",
			Content: strings.TrimSpace(`你是一个 JSON 修复助手。
你的任务不是回答原问题，而是把给定内容重写成一个合法 JSON 对象。
你必须严格遵守以下规则：
1. 只能输出一个 JSON 对象。
2. 首字符必须是 {，末字符必须是 }。
3. 不要输出 Markdown、解释、前后缀、注释或代码块。
4. 不要凭空扩展字段，只保留能从原内容中稳定提取的信息。`),
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("目标 JSON 结构：\n%s\n\n待修复内容：\n%s", strings.TrimSpace(schema), strings.TrimSpace(raw)),
		},
	}
}

// parseStructuredJSON 解析模型结构化输出，首解析失败时发起一次修复重试，全部失败返回 ErrParseFailed。
func parseStructuredJSON[T any](ctx context.Context, llm LLMClient, cfg *AIConfig, raw, schema string) (T, error) {
	if payload, err := decodeJSONPayload[T](raw); err == nil {
		return payload, nil
	}

	repaired, repairErr := llm.Chat(ctx, buildJSONRepairMessages(schema, raw), cfg)
	if repairErr != nil || repaired == nil {
		var zero T
		return zero, ErrParseFailed
	}
	repairedPayload, repairedErr := decodeJSONPayload[T](repaired.Content)
	if repairedErr != nil {
		var zero T
		return zero, ErrParseFailed
	}
	return repairedPayload, nil
}

// ---------- 各场景 JSON 合同 ----------

// interviewResultSchema 面试出题/评分结构化输出合同。
func interviewResultSchema() string {
	return `{
  "question": "题目正文，生成新题时必填",
  "topic": "知识点主题",
  "difficulty": "easy|medium|hard",
  "type": "technical|behavioral|coding",
  "hints": "简短提示，可为空字符串",
  "feedback": "对上一题回答的评价，无则空字符串",
  "score": 0,
  "should_end": false,
  "live2d_emotion": "情绪标签，可为空字符串",
  "live2d_action": "动作标签，可为空字符串"
}`
}

// interviewEvaluateSchema 答案评估结构化输出合同，明确要求 0-100 打分 + 关键点 + 建议，
// 供 EvaluateAnswer 使用，避免与出题合同混用导致 LLM 漏返回评分。
func interviewEvaluateSchema() string {
	return `{
  "score": 75,
  "is_correct": true,
  "feedback": "对回答的总体评价，先肯定亮点再指出不足",
  "key_points": ["关键知识点1", "关键知识点2"],
  "suggestions": "针对性的改进建议"
}`
}

// planResultSchema 学习计划结构化输出合同。
func planResultSchema() string {
	return `{
  "plan_title": "计划标题",
  "tasks": [
    {
      "title": "任务标题",
      "description": "任务说明",
      "phase": "阶段名",
      "order_index": 1,
      "estimated_hours": 2,
      "task_type": "study|practice|interview|review",
      "phase_goal": "阶段目标",
      "day_number": 1,
      "duration_minutes": 60,
      "priority": "high|medium|low"
    }
  ],
  "summary": "计划总结",
  "phase": "foundation|drill|review|mock",
  "phase_goal": "阶段目标",
  "duration_days": 30
}`
}

// quizResultSchema 答题分析结构化输出合同。
func quizResultSchema() string {
	return `{
  "score": 0,
  "is_correct": true,
  "feedback": "总体评价",
  "key_points": ["关键点1", "关键点2"],
  "suggestions": "改进建议",
  "correct_answer": "参考答案"
}`
}

// codeAnalysisSchema 代码分析结构化输出合同。
func codeAnalysisSchema() string {
	return `{
  "is_correct": true,
  "score": 80,
  "feedback": "总体反馈",
  "issues": ["问题1", "问题2"],
  "improvements": ["改进1", "改进2"],
  "mistake_tags": ["错因标签1"],
  "strength_tags": ["优势标签1"],
  "time_complexity": "O(n)",
  "space_complexity": "O(1)"
}`
}

// codingDiagnosisSchema 编程面试诊断结构化输出合同。
func codingDiagnosisSchema() string {
	return `{
  "score": 78,
  "mistake_tags": ["状态定义不清"],
  "strength_tags": ["愿意主动验证思路"],
  "evidence": ["多次运行后仍集中修改同一逻辑分支"],
  "suggestions": ["补练边界条件和状态设计"],
  "process_summary": "能够持续迭代，但调试路径还不够稳定。"
}`
}

// knowledgeReportSchema 知识点专项面试报告结构化输出合同。
// 仅围绕用户自定义知识点考核，输出 8 大板块的完整结构化报告。
func knowledgeReportSchema() string {
	return `{
  "overall_score": 78,
  "rating": "良好",
  "conclusion": "一句话整体考核结论",
  "basic_info": {
    "knowledge_topics": ["Java集合"],
    "question_type": "问答",
    "duration_seconds": 600,
    "total_questions": 5,
    "correct_count": 3,
    "accuracy": 0.6
  },
  "question_reviews": [
    {
      "question_index": 0,
      "question": "题目正文",
      "user_answer": "用户作答原文",
      "score": 80,
      "max_score": 100,
      "errors": ["错误点"],
      "omissions": ["遗漏点"],
      "highlights": ["亮点"],
      "standard_answer": "标准答案",
      "key_points": ["核心得分点1"]
    }
  ],
  "dimension_scores": [
    {"dimension": "知识点基础掌握度", "score": 80, "comment": "评语"},
    {"dimension": "知识点应用落地能力", "score": 70, "comment": "评语"},
    {"dimension": "知识延伸与深度", "score": 60, "comment": "评语"},
    {"dimension": "答题精准度与严谨度", "score": 75, "comment": "评语"}
  ],
  "mastered_points": ["已掌握知识点1"],
  "blind_spots": [
    {"topic": "知识点", "level": "完全不会", "detail": "说明"}
  ],
  "study_suggestions": [
    {"focus": "重点背诵", "detail": "具体内容"}
  ],
  "next_quiz_topics": [
    {"topic": "知识点", "reason": "针对短板"}
  ]
}`
}

// jobReportSchema 岗位求职面试报告结构化输出合同。
// 围绕简历+岗位+面试表现，输出 6 维加权评分 + 9 板块的求职型报告。
func jobReportSchema() string {
	return `{
  "overall_score": 78,
  "rating": "良好",
  "hire_recommendation": "建议复试考察",
  "basic_info": {
    "candidate_name": "候选人姓名",
    "target_position": "应聘岗位",
    "interview_type": "技术面",
    "duration_seconds": 600,
    "total_questions": 8,
    "overall_score": 78,
    "rating": "良好"
  },
  "jd_match_overview": {
    "matched_items": ["匹配项"],
    "missing_items": ["缺失项"],
    "hard_requirements_met": true,
    "resume_highlights": ["简历优势"],
    "resume_hard_wounds": ["简历硬伤"]
  },
  "question_reviews": [
    {
      "question_index": 0,
      "question": "面试问题",
      "user_answer": "用户回答原文",
      "score": 80,
      "max_score": 100,
      "highlights": ["面试亮点"],
      "loopholes": ["回答漏洞"],
      "pitfalls": ["踩坑点"],
      "taboos": ["职场禁忌点"]
    }
  ],
  "dimension_scores": [
    {"dimension": "岗位硬技能匹配度", "score": 80, "weight": 0.35, "comment": "优缺点解读"},
    {"dimension": "简历项目真实性&含金量", "score": 75, "weight": 0.25, "comment": "优缺点解读"},
    {"dimension": "逻辑思维与表达能力", "score": 70, "weight": 0.15, "comment": "优缺点解读"},
    {"dimension": "求职动机与岗位认知", "score": 65, "weight": 0.10, "comment": "优缺点解读"},
    {"dimension": "职业素养与稳定性", "score": 72, "weight": 0.10, "comment": "优缺点解读"},
    {"dimension": "综合面试印象", "score": 78, "weight": 0.05, "comment": "优缺点解读"}
  ],
  "core_advantages": ["核心求职优势"],
  "weaknesses_risks": [
    {"item": "短板", "level": "致命", "impact": "对录用的影响"}
  ],
  "hire_decision": {
    "decision": "建议复试考察",
    "rationale": "核心依据"
  },
  "optimization_plan": [
    {"aspect": "话术优化", "detail": "具体方案"}
  ],
  "next_round_questions": [
    {"question": "预测题", "focus": "考点", "difficulty": "medium"}
  ]
}`
}

// SuggestedActionItem 结构化引导动作 biz 结构，对齐 ai.proto SuggestedAction。
type SuggestedActionItem struct {
	Type   string `json:"type"`
	Target string `json:"target"`
	Params string `json:"params"`
}

// InlineTriggerItem 表示回复文本中的可点击关键词及其关联动作（字幕行内关键词）。
type InlineTriggerItem struct {
	Keyword      string `json:"keyword"`       // 在 reply 中出现的可点击关键词
	ActionType   string `json:"action_type"`   // practice | interview | adjust_plan
	Target       string `json:"target"`        // 导航目标标识（题集编码等）
	PositionHint string `json:"position_hint"` // 关键词在 reply 中的位置提示（head|middle|tail），供前端定位
}

// IntentInfo LLM 意图识别结果，驱动多轮对话状态。
type IntentInfo struct {
	Type       string  `json:"type"`       // practice | adjust_plan | interview | chat
	Confidence float64 `json:"confidence"` // 0-1
	Stage      string  `json:"stage"`      // collecting_info | ready_to_execute | none
}

// PendingAction 当 LLM 判定信息收集完毕时可自动触发的待执行动作。
type PendingAction struct {
	Type        string            `json:"type"`         // adjust_plan | practice 等
	Ready       bool              `json:"ready"`        // 是否已收集完信息，前端可自动触发
	Params      map[string]string `json:"params"`       // 已收集的参数
	MissingInfo []string          `json:"missing_info"` // 还缺哪些信息（ready=false 时）
}

// ConversationState 多轮对话状态跟踪，跨轮次传入模板保持上下文。
type ConversationState struct {
	Phase           string            `json:"phase"`            // greeting | collecting | ready | executing
	CollectedParams map[string]string `json:"collected_params"` // 已收集的参数（如 goal, time, direction）
}

// CompanionPayload 陪伴聊天结构化输出合同解析结果。
type CompanionPayload struct {
	Reply             string                `json:"reply"`
	SuggestedActions  []SuggestedActionItem `json:"suggested_actions"`
	InlineTriggers    []InlineTriggerItem   `json:"inline_triggers,omitempty"`
	Intent            *IntentInfo           `json:"intent,omitempty"`
	PendingAction     *PendingAction        `json:"pending_action,omitempty"`
	ConversationState *ConversationState    `json:"conversation_state,omitempty"`
}

// companionResultSchema 陪伴聊天结构化输出合同。
// reply 保持陪伴口吻；suggested_actions 由 LLM 基于上下文里的题集/弱项产出引导动作。
// inline_triggers / intent / pending_action / conversation_state 为选填字段，
// LLM 根据对话阶段按需产出。
func companionResultSchema() string {
	return `{
  "reply": "给用户的自然语言回复，保持陪伴口吻",
  "suggested_actions": [
    {"type": "practice", "target": "题集编码", "params": ""},
    {"type": "interview", "target": "", "params": ""},
    {"type": "adjust_plan", "target": "", "params": ""},
    {"type": "chat", "target": "", "params": "可让用户直接发送的追问"}
  ],
  "inline_triggers": [
    {"keyword": "回复文本中出现的关键词", "action_type": "practice", "target": "题集编码", "position_hint": "head|middle|tail"}
  ],
  "intent": {"type": "practice|adjust_plan|interview|chat", "confidence": 0.9, "stage": "collecting_info|ready_to_execute|none"},
  "pending_action": {"type": "adjust_plan|practice", "ready": false, "params": {}, "missing_info": ["还缺的信息"]},
  "conversation_state": {"phase": "greeting|collecting|ready|executing", "collected_params": {"goal": "目标值"}}
}`
}

// buildCodeAnalysisSystemPrompt 构造代码分析系统提示词。
func buildCodeAnalysisSystemPrompt() string {
	return `请分析这道题的答案，并严格返回 JSON，不要输出 Markdown 或额外解释。JSON 结构如下：
{
  "is_correct": true,
  "score": 0,
  "feedback": "整体评价",
  "issues": ["问题1", "问题2"],
  "improvements": ["改进建议1", "改进建议2"],
  "mistake_tags": ["错因标签1"],
  "strength_tags": ["优势标签1"],
  "time_complexity": "O(n)",
  "space_complexity": "O(1)"
}
要求：
1. score 必须在 0 到 100 之间。
2. 只评价答案本身是否正确、完整、高质量。不要评价题目描述中与答案无关的内容。
3. is_correct 应基于答案是否正确回应了题目的核心要求来判定。
4. issues 和 improvements 至少各返回 1 条。
5. mistake_tags 要尽量具体，不要只写"基础不好"这类空泛表述。`
}

// buildCodingDiagnosisSystemPrompt 构造编程面试诊断系统提示词。
func buildCodingDiagnosisSystemPrompt() string {
	return `请根据题目、最终代码和过程事件分析候选人的编程面试表现，并严格返回 JSON，不要输出 Markdown 或额外解释。JSON 结构如下：
{
  "score": 0,
  "mistake_tags": ["错因标签1"],
  "strength_tags": ["优势标签1"],
  "evidence": ["证据1", "证据2"],
  "suggestions": ["建议1", "建议2"],
  "process_summary": "过程总结"
}
要求：
1. score 必须在 0 到 100 之间。
2. 错因标签要尽量具体，优先围绕状态定义、边界条件、索引控制、数据结构选择、复杂度意识和调试路径。
3. evidence 必须引用过程中的现象，不要只写空泛判断。
4. suggestions 给出可执行的补强动作。`
}
