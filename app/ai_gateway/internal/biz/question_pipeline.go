package biz

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// ==================== 类型定义 ====================

const (
	defaultQuestionPipelineCount = 8
	maxQuestionPipelineCount     = 20
)

// questionPipelineConstraintProfile 描述从岗位要求与智能体指令中提炼出的硬约束。
type questionPipelineConstraintProfile struct {
	CandidateCount      int
	RequireSubjective   bool
	RequireCode         bool
	PreferDistinctTopic bool
	ExcludeProjectCards bool
	GoFeatureOnly       bool
	ExactLanguageCounts map[string]int
	ExactLanguageOrder  []string
	RemainingLanguage   string
}

// questionPipelineSingleCardAttempt 描述单张题卡生成尝试的结果。
type questionPipelineSingleCardAttempt struct {
	TraceOutput        string
	TraceID            string
	FailureStage       string
	CandidateExcerpt   string
	RepairAttempted    bool
	SupplementAttempted bool
	Cards              []*QuestionCandidate
}

// ==================== 约束构建 ====================

// buildConstraintProfile 从需求与智能体指令中提炼题卡硬约束。
func buildConstraintProfile(requirement string, agentPrompt string, candidateLimit int) questionPipelineConstraintProfile {
	combined := strings.ToLower(requirement + "\n" + agentPrompt)
	requireCode := shouldRequireCode(requirement, agentPrompt)
	profile := questionPipelineConstraintProfile{
		CandidateCount:      candidateLimit,
		RequireSubjective:   !requireCode && (strings.Contains(combined, "问答题") || strings.Contains(combined, "主观题")),
		RequireCode:         requireCode,
		PreferDistinctTopic: strings.Contains(combined, "不同考点") || strings.Contains(combined, "互不重复"),
		ExcludeProjectCards: shouldFilterProjectCards(requirement, agentPrompt),
		GoFeatureOnly:       shouldFilterGoFeatureOnly(requirement, agentPrompt),
		ExactLanguageCounts: make(map[string]int),
		ExactLanguageOrder:  make([]string, 0),
	}

	langCounts, langOrder := extractLanguageCounts(agentPrompt)
	for _, language := range langOrder {
		profile.ExactLanguageCounts[language] = langCounts[language]
		profile.ExactLanguageOrder = append(profile.ExactLanguageOrder, language)
	}
	profile.RemainingLanguage = extractRemainingLanguage(agentPrompt)

	return profile
}

// shouldRequireCode 判断是否要求编程题。
func shouldRequireCode(requirement, agentPrompt string) bool {
	combined := strings.ToLower(requirement + "\n" + agentPrompt)
	codeKeywords := []string{"编程题", "代码题", "coding", "code", "算法题", "手撕", "实现"}
	for _, keyword := range codeKeywords {
		if strings.Contains(combined, keyword) {
			return true
		}
	}
	return false
}

// shouldFilterProjectCards 判断是否过滤项目类题目。
func shouldFilterProjectCards(requirement, agentPrompt string) bool {
	combined := strings.ToLower(requirement + "\n" + agentPrompt)
	return strings.Contains(combined, "语言特性") ||
		strings.Contains(combined, "语言机制") ||
		strings.Contains(combined, "底层原理") ||
		strings.Contains(combined, "标准库")
}

// shouldFilterGoFeatureOnly 判断是否只保留 Go 语言特性题目。
func shouldFilterGoFeatureOnly(requirement, agentPrompt string) bool {
	combined := strings.ToLower(requirement + "\n" + agentPrompt)
	return strings.Contains(combined, "go语言") ||
		strings.Contains(combined, "golang") ||
		strings.Contains(combined, "go特性")
}

// extractLanguageCounts 从智能体指令中提取语言配额。
func extractLanguageCounts(agentPrompt string) (map[string]int, []string) {
	counts := make(map[string]int)
	order := make([]string, 0)

	re := regexp.MustCompile(`(\d+)\s*(?:道|题|个)?\s*(go|golang|java|python|c\+\+)`)
	matches := re.FindAllStringSubmatch(strings.ToLower(agentPrompt), -1)
	for _, match := range matches {
		count := 0
		fmt.Sscanf(match[1], "%d", &count)
		language := normalizeLanguage(match[2])
		if count > 0 && language != "" {
			if _, exists := counts[language]; !exists {
				order = append(order, language)
			}
			counts[language] += count
		}
	}

	return counts, order
}

// extractRemainingLanguage 解析"其他是 Go"之类的剩余题卡语言约束。
func extractRemainingLanguage(agentPrompt string) string {
	re := regexp.MustCompile(`(?i)(?:其他|其余|剩下|剩余)[^。\n]{0,10}?(?:是|为|都用|都为|都必须是)?\s*(java|go|golang)`)
	match := re.FindStringSubmatch(strings.ToLower(agentPrompt))
	if len(match) < 2 {
		return ""
	}
	return normalizeLanguage(match[1])
}

// normalizeLanguage 规范化语言标识。
func normalizeLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "go", "golang":
		return "go"
	case "java":
		return "java"
	case "python":
		return "python"
	default:
		return ""
	}
}

// ==================== 约束摘要 ====================

// buildConstraintSummary 构建约束摘要文本。
func buildConstraintSummary(profile questionPipelineConstraintProfile) string {
	parts := []string{
		fmt.Sprintf("必须返回 %d 张候选题卡。", profile.CandidateCount),
	}
	if profile.RequireCode {
		parts = append(parts, "本轮明确要求编程题，type 必须使用 code，禁止回退成 subjective。")
		parts = append(parts, "编程题必须同时提供 4 个核心部分：answer（代码参考答案）、solution（代码思路解析）、judge_config.public_test_cases（恰好 3 条公开运行用例）、judge_config.hidden_test_cases（提交判题隐藏用例集）。")
		parts = append(parts, "编程题 judge_config 必须使用 testcase 判题模式，并包含 default_language、allowed_languages、starter_code、reference_solutions、time_limit_ms、memory_limit_mb。")
	} else if profile.RequireSubjective {
		parts = append(parts, "题型优先使用问答题（subjective），不要退化成选择题。")
	}
	if profile.PreferDistinctTopic {
		parts = append(parts, "每张题卡必须覆盖不同考点，禁止同义改写凑数。")
	}
	if profile.ExcludeProjectCards {
		parts = append(parts, "禁止输出项目经历、职业规划、微服务治理、行为面试等泛项目题。")
	}
	if profile.GoFeatureOnly {
		parts = append(parts, "题目主体必须聚焦语言特性、语言机制、底层原理或标准库核心语义。")
	}
	for _, language := range profile.ExactLanguageOrder {
		count := profile.ExactLanguageCounts[language]
		parts = append(parts, fmt.Sprintf("必须恰好有 %d 题使用 %s 语言。", count, strings.ToUpper(language)))
	}
	if profile.RemainingLanguage != "" {
		parts = append(parts, fmt.Sprintf("其他题卡使用 %s 语言。", strings.ToUpper(profile.RemainingLanguage)))
	}
	return strings.Join(parts, "\n")
}

// buildTypeRequirements 构建类型要求文本。
func buildTypeRequirements(profile questionPipelineConstraintProfile) string {
	if !profile.RequireCode {
		return ""
	}
	return strings.TrimSpace(`
6. 本轮需求明确要求编程题，type 必须固定为 code，禁止输出 subjective。
7. 编程题必须同时输出 4 个核心部分：answer（代码参考答案）、solution（代码思路解析）、judge_config.public_test_cases（恰好 3 条公开样例）、judge_config.hidden_test_cases（提交判题使用的隐藏用例集）。
8. 当 type=code 时，judge_config 必须输出为对象，不能缺省也不能写成字符串说明；evaluation_mode 必须固定为 testcase。
9. judge_config 至少包含 evaluation_mode、default_language、allowed_languages、starter_code、public_test_cases、hidden_test_cases、reference_solutions、time_limit_ms、memory_limit_mb；reference_solutions 至少 1 条。
10. 测试用例字段固定为 input、expected_output、description，按标准输入/标准输出模式编写；public_test_cases 必须正好 3 条。`)
}

// buildAgentPrompt 构建智能体指令。
func buildAgentPrompt(agentPrompt string) string {
	baseRules := "必须严格执行用户在智能体指令中的显式要求。凡是\"必须包含\"\"必须排除\"\"至少/恰好几题\"\"指定语言或主题\"的约束，都视为硬约束，不得忽略。若用户要求聚焦语言特性、语言机制或底层原理，默认禁止输出项目经历、职业规划、微服务治理、行为面试等泛项目题，除非智能体指令明确要求保留。"
	if strings.TrimSpace(agentPrompt) == "" {
		return "优先保证题卡之间互不重复，覆盖不同考点，避免模板化表述。\n" + baseRules
	}
	return strings.TrimSpace(agentPrompt) + "\n" + baseRules
}

// buildTargetLanguageLabel 构建目标语言标签。
func buildTargetLanguageLabel(language string) string {
	if language == "" {
		return "不限"
	}
	return strings.ToUpper(language)
}

// buildExistingCardsExcerpt 构建已生成题卡摘要。
func buildExistingCardsExcerpt(cards []*QuestionCandidate) string {
	if len(cards) == 0 {
		return "（暂无）"
	}

	var parts []string
	for i, card := range cards {
		title := strings.TrimSpace(card.Title)
		if title == "" {
			title = "未命名"
		}
		parts = append(parts, fmt.Sprintf("%d. %s", i+1, title))
	}
	return strings.Join(parts, "\n")
}

// buildCategoryHints 构建分类提示。
func buildCategoryHints(categories []string) string {
	if len(categories) == 0 {
		return "（暂无分类信息）"
	}
	return strings.Join(categories, "、")
}

// ==================== Prompt 构建 ====================

// buildSingleCardPrompt 构建单张题卡生成的 Prompt。
func buildSingleCardPrompt(
	industryName string,
	requirement string,
	agentPrompt string,
	constraints questionPipelineConstraintProfile,
	existingCards []*QuestionCandidate,
	categories []string,
	targetLanguage string,
	slotIndex int,
) string {
	prompt := fmt.Sprintf(`你是 MakeJob 的逐张题卡生成智能体。当前只允许生成 1 张中文面试题卡，请不要一次输出多张。

严格要求：
1. 只返回一个题卡 JSON 对象，首字符必须是 {，末字符必须是 }，不要包裹 cards 数组。
2. 不允许额外解释、Markdown、代码块或前后缀文本；尤其不要在 answer、solution 或 reference_solutions.code 内再嵌三反引号 json 代码块。
3. 本轮题卡必须与已生成题卡考点明显不同，禁止同义改写。
4. 如果指定了目标语言，必须严格按目标语言出题；不要输出项目经历、职业规划、微服务治理等凑数题。
5. title、content、answer、explanation 都必须填写完整；当 type=code 时，还必须额外输出 solution 字段。
%s

行业：
%s

用户目标：
%s

智能体指令：
%s

硬约束摘要：
%s

本轮目标语言：
%s

当前是第 %d / %d 张题卡。

已生成题卡摘要：
%s

现有分类：
%s`,
		buildTypeRequirements(constraints),
		industryName,
		requirement,
		buildAgentPrompt(agentPrompt),
		buildConstraintSummary(constraints),
		buildTargetLanguageLabel(targetLanguage),
		slotIndex+1,
		constraints.CandidateCount,
		buildExistingCardsExcerpt(existingCards),
		buildCategoryHints(categories),
	)

	return prompt
}

// buildTargetLanguages 按语言配额构造逐张直生模式的目标语言序列。
func buildTargetLanguages(profile questionPipelineConstraintProfile, candidateLimit int) []string {
	targets := make([]string, 0, candidateLimit)
	for _, language := range profile.ExactLanguageOrder {
		for count := 0; count < profile.ExactLanguageCounts[language] && len(targets) < candidateLimit; count++ {
			targets = append(targets, language)
		}
	}
	for profile.RemainingLanguage != "" && len(targets) < candidateLimit {
		targets = append(targets, profile.RemainingLanguage)
	}
	for len(targets) < candidateLimit {
		targets = append(targets, "")
	}
	return targets
}

// ==================== 结果解析 ====================
// 使用 pipeline_parser.go 中的完整解析逻辑（复刻单体架构）

// ==================== 生成逻辑 ====================

// generateSingleCard 生成单张题卡（复刻单体架构完整流程，含容错机制）。
func (uc *AdminUseCase) generateSingleCard(
	ctx context.Context,
	industryName string,
	requirement string,
	agentPrompt string,
	constraints questionPipelineConstraintProfile,
	existingCards []*QuestionCandidate,
	categories []string,
	targetLanguage string,
	slotIndex int,
	cfg *AIConfig,
) (*questionPipelineSingleCardAttempt, error) {
	prompt := buildSingleCardPrompt(
		industryName,
		requirement,
		agentPrompt,
		constraints,
		existingCards,
		categories,
		targetLanguage,
		slotIndex,
	)

	attempt := &questionPipelineSingleCardAttempt{}

	// 创建独立的 context，避免上游超时影响
	llmCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	messages := []Message{{Role: "user", Content: prompt}}
	resp, err := uc.llm.Chat(llmCtx, messages, cfg)
	if err != nil {
		attempt.FailureStage = "model_call"
		return attempt, fmt.Errorf("模型调用失败: %w", err)
	}

	attempt.TraceOutput = resp.Content

	// 第一步：解析结果（使用复刻单体架构的完整解析逻辑）
	modelCard, rawCandidate, decodeErr := decodeQuestionPipelineSingleCardResponse(resp.Content)
	attempt.CandidateExcerpt = rawCandidate
	if decodeErr != nil {
		// 第二步：解析失败，尝试 AI 修复
		attempt.FailureStage = "parse"
		attempt.RepairAttempted = true

		repaired, repairErr := uc.repairQuestionPipelineCardResponse(
			ctx,
			firstNonEmptyString(strings.TrimSpace(rawCandidate), strings.TrimSpace(resp.Content)),
			constraints.RequireCode,
		)
		if repairErr != nil {
			return attempt, fmt.Errorf("解析失败且修复失败: %w", decodeErr)
		}
		attempt.TraceOutput = firstNonEmptyString(strings.TrimSpace(repaired.TraceOutput), attempt.TraceOutput)
		modelCard = repaired.Card
	}

	// 第三步：清理题卡字段
	modelCard = sanitizeQuestionCandidate(modelCard)

	// 第四步：补齐编程题字段（finalizeQuestionPipelineModelCard）
	finalizedCard, finalizeWarning, supplementAttempted := uc.finalizeQuestionPipelineModelCard(ctx, modelCard, constraints)
	attempt.SupplementAttempted = supplementAttempted
	if strings.TrimSpace(finalizeWarning) != "" {
		attempt.FailureStage = "supplement"
		attempt.TraceOutput = strings.TrimSpace(attempt.TraceOutput + "\n\n[finalize_warning]\n" + finalizeWarning)
		return attempt, fmt.Errorf("题卡结构补齐失败: %s", finalizeWarning)
	}

	// 第五步：校验基本字段
	if finalizedCard.Title == "" || finalizedCard.Content == "" {
		attempt.FailureStage = "validation"
		return attempt, fmt.Errorf("题卡缺少必要字段")
	}

	// 第六步：编程题完整性校验
	if constraints.RequireCode || normalizeQuestionPipelineType(finalizedCard.Type) == "code" {
		checkedCard, reason, valid := normalizeCodeCardWithReason(finalizedCard)
		if !valid {
			attempt.FailureStage = "constraint"
			return attempt, fmt.Errorf("编程题完整性校验失败: %s", reason)
		}
		finalizedCard = checkedCard
	}

	attempt.Cards = []*QuestionCandidate{finalizedCard}
	attempt.FailureStage = ""
	return attempt, nil
}

// QuestionPipelineStreamEvent 描述题目流水线流式事件。
type QuestionPipelineStreamEvent struct {
	Event            string                       `json:"event"`
	Message          string                       `json:"message,omitempty"`
	TraceID          string                       `json:"trace_id,omitempty"`
	RawOutput        string                       `json:"raw_output,omitempty"`
	FailureStage     string                       `json:"failure_stage,omitempty"`
	CandidateExcerpt string                       `json:"candidate_excerpt,omitempty"`
	RepairAttempted  bool                         `json:"repair_attempted,omitempty"`
	SupplementAttempted bool                      `json:"supplement_attempted,omitempty"`
	SlotIndex        int32                        `json:"slot_index,omitempty"`
	RetryIndex       int32                        `json:"retry_index,omitempty"`
	Card             *QuestionCandidate           `json:"card,omitempty"`
	Response         *GenerateQuestionCandidatesResult `json:"response,omitempty"`
}

// QuestionPipelineStreamEmitter 描述题目流水线流式推送回调。
type QuestionPipelineStreamEmitter func(event *QuestionPipelineStreamEvent) error

// GenerateQuestionCandidatesDirect 逐张生成题卡（复刻单体架构核心逻辑）。
func (uc *AdminUseCase) GenerateQuestionCandidatesDirect(
	ctx context.Context,
	industryCode string,
	requirement string,
	agentPrompt string,
	candidateCount int32,
) (*GenerateQuestionCandidatesResult, error) {
	return uc.GenerateQuestionCandidatesStream(ctx, industryCode, requirement, agentPrompt, candidateCount, nil)
}

// GenerateQuestionCandidatesStream 逐张生成题卡，支持流式事件推送。
func (uc *AdminUseCase) GenerateQuestionCandidatesStream(
	ctx context.Context,
	industryCode string,
	requirement string,
	agentPrompt string,
	candidateCount int32,
	emit QuestionPipelineStreamEmitter,
) (*GenerateQuestionCandidatesResult, error) {
	const scene = "question_generator"
	start := time.Now()

	// 推送初始化状态
	if emit != nil {
		if err := emit(&QuestionPipelineStreamEvent{
			Event:   "status",
			Message: "已建立流式生成连接，准备加载行业、分类与约束。",
		}); err != nil {
			return nil, err
		}
	}

	// 加载配置
	cfg, err := uc.configRepo.GetActiveConfig(ctx, scene)
	if err != nil {
		if emit != nil {
			_ = emit(&QuestionPipelineStreamEvent{
				Event:   "error",
				Message: "AI 配置加载失败",
			})
		}
		return nil, ErrAIConfigNotFound
	}

	// 标准化候选数量
	if candidateCount <= 0 {
		candidateCount = int32(defaultQuestionPipelineCount)
	}
	if candidateCount > int32(maxQuestionPipelineCount) {
		candidateCount = int32(maxQuestionPipelineCount)
	}

	// 构建约束
	constraints := buildConstraintProfile(requirement, agentPrompt, int(candidateCount))
	targetLanguages := buildTargetLanguages(constraints, int(candidateCount))

	// 获取行业信息（简化：使用 industryCode 作为名称）
	industryName := industryCode

	// 获取分类信息（简化）
	categories := make([]string, 0)

	// 推送开始生成状态
	if emit != nil {
		if err := emit(&QuestionPipelineStreamEvent{
			Event:   "status",
			Message: "正在逐张生成候选题卡，生成结果会实时显示。",
		}); err != nil {
			return nil, err
		}
	}

	// 逐张生成题卡
	cards := make([]*QuestionCandidate, 0, candidateCount)
	warnings := make([]string, 0)

	for slot := 0; slot < int(candidateCount); slot++ {
		targetLanguage := ""
		if slot < len(targetLanguages) {
			targetLanguage = targetLanguages[slot]
		}

		// 推送当前题卡生成状态
		if emit != nil {
			message := fmt.Sprintf("正在生成第 %d / %d 张题卡。", slot+1, candidateCount)
			if strings.TrimSpace(targetLanguage) != "" {
				message = fmt.Sprintf("正在生成第 %d / %d 张题卡，目标语言：%s。", slot+1, candidateCount, strings.ToUpper(targetLanguage))
			}
			if err := emit(&QuestionPipelineStreamEvent{
				Event:   "status",
				Message: message,
			}); err != nil {
				return nil, err
			}
		}

		slotSucceeded := false
		for retry := 0; retry < 3; retry++ {
			// 推送重试状态
			if emit != nil && retry > 0 {
				if err := emit(&QuestionPipelineStreamEvent{
					Event:     "status",
					Message:   fmt.Sprintf("第 %d 张题卡进入第 %d 次重试。", slot+1, retry+1),
					SlotIndex: int32(slot),
					RetryIndex: int32(retry),
				}); err != nil {
					return nil, err
				}
			}

			log.Infof("生成第 %d/%d 张题卡，重试 %d 次", slot+1, candidateCount, retry)

			attempt, err := uc.generateSingleCard(
				ctx,
				industryName,
				requirement,
				agentPrompt,
				constraints,
				cards,
				categories,
				targetLanguage,
				slot,
				cfg,
			)

			if err != nil {
				warning := fmt.Sprintf("第 %d 张题卡生成失败: %v", slot+1, err)
				warnings = append(warnings, warning)
				log.Warnf(warning)

				// 推送警告事件
				if emit != nil {
					if err := emit(&QuestionPipelineStreamEvent{
						Event:            "warning",
						Message:          warning,
						SlotIndex:        int32(slot),
						RetryIndex:       int32(retry),
						FailureStage:     attempt.FailureStage,
						RawOutput:        attempt.TraceOutput,
						CandidateExcerpt: attempt.CandidateExcerpt,
					}); err != nil {
						return nil, err
					}
				}
				continue
			}

			// 检查是否重复
			isDuplicate := false
			for _, existing := range cards {
				if existing.Title == attempt.Cards[0].Title {
					isDuplicate = true
					break
				}
			}
			if isDuplicate {
				warning := fmt.Sprintf("第 %d 张题卡与已生成题卡重复，重试", slot+1)
				warnings = append(warnings, warning)

				// 推送重复警告
				if emit != nil {
					if err := emit(&QuestionPipelineStreamEvent{
						Event:     "warning",
						Message:   warning,
						SlotIndex: int32(slot),
						RetryIndex: int32(retry),
					}); err != nil {
						return nil, err
					}
				}
				continue
			}

			// 添加题卡并设置 ID
			for _, card := range attempt.Cards {
				card.ID = fmt.Sprintf("pipeline-card-%d", len(cards)+1)
				cards = append(cards, card)
			}

			// 推送题卡事件
			if emit != nil && len(cards) > 0 {
				// 推送刚添加的题卡（带 ID）
				lastCard := cards[len(cards)-1]
				if err := emit(&QuestionPipelineStreamEvent{
					Event:   "card",
					Message: fmt.Sprintf("第 %d 张题卡已生成。", len(cards)),
					Card:    lastCard,
				}); err != nil {
					return nil, err
				}

				// 推送进度状态
				if err := emit(&QuestionPipelineStreamEvent{
					Event:   "status",
					Message: fmt.Sprintf("已生成 %d / %d 张候选题卡。", len(cards), candidateCount),
				}); err != nil {
					return nil, err
				}
			}

			slotSucceeded = true
			break
		}

		if !slotSucceeded {
			warning := fmt.Sprintf("第 %d 张题卡在逐张直生模式下未能生成满足要求的结果。", slot+1)
			warnings = append(warnings, warning)

			// 推送失败警告
			if emit != nil {
				if err := emit(&QuestionPipelineStreamEvent{
					Event:     "warning",
					Message:   warning,
					SlotIndex: int32(slot),
				}); err != nil {
					return nil, err
				}
			}
		}
	}

	// 记录日志
	latencyMs := time.Since(start).Milliseconds()
	uc.saveLog(ctx, scene, cfg.Model, nil, nil, latencyMs)

	if len(cards) == 0 {
		if emit != nil {
			_ = emit(&QuestionPipelineStreamEvent{
				Event:   "warning",
				Message: "逐张直生模式未返回可用题卡，请检查 AI 配置或调整提示词。",
			})
		}
		return nil, ErrParseFailed
	}

	result := &GenerateQuestionCandidatesResult{
		IndustryCode: industryCode,
		Requirement:  requirement,
		Candidates:   cards,
		Warnings:     warnings,
	}

	// 推送完成事件
	if emit != nil {
		if err := emit(&QuestionPipelineStreamEvent{
			Event:    "complete",
			Response: result,
		}); err != nil {
			return nil, err
		}
	}

	return result, nil
}
