package biz

import (
	"context"
	"fmt"
	"regexp"
	"sort"
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

	re := regexp.MustCompile(`(?i)(?:必须|至少|恰好|其中必须有|其中有|至少有|确保有)?\s*([0-9一二两三四五六七八九十]+)\s*(?:个|道|张)?(?:题|题卡)[^。\n]{0,12}?(?:是|为)?\s*(java|go|golang)`)
	matches := re.FindAllStringSubmatch(agentPrompt, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		count := parseQuestionPipelineCountToken(match[1])
		language := normalizeLanguage(match[2])
		if count <= 0 || language == "" {
			continue
		}
		if _, exists := counts[language]; !exists {
			order = append(order, language)
		}
		counts[language] += count
	}

	return counts, order
}

// parseQuestionPipelineCountToken 将阿拉伯数字或常见中文数字转换为数量。
func parseQuestionPipelineCountToken(token string) int {
	switch strings.TrimSpace(strings.ToLower(token)) {
	case "1", "一", "一个":
		return 1
	case "2", "二", "两", "两个":
		return 2
	case "3", "三", "三个":
		return 3
	case "4", "四", "四个":
		return 4
	case "5", "五", "五个":
		return 5
	case "6", "六", "六个":
		return 6
	case "7", "七", "七个":
		return 7
	case "8", "八", "八个":
		return 8
	case "9", "九", "九个":
		return 9
	case "10", "十", "十个":
		return 10
	default:
		return 0
	}
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
		parts = append(parts, "编程题必须同时提供 4 个核心部分：answer（代码参考答案）、solution（代码思路解析，必须按标题式小节输出，含题意总结/解题思路/关键步骤/边界条件/复杂度分析/常见错法六节）、judge_config.public_test_cases（恰好 3 条公开运行用例）、judge_config.hidden_test_cases（提交判题隐藏用例集）。")
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
	parts = append(parts, "如果无法满足约束，宁可少输出，也不要补无关题凑数。")
	return strings.Join(parts, "\n")
}

// buildTypeRequirements 构建类型要求文本。
func buildTypeRequirements(profile questionPipelineConstraintProfile) string {
	if !profile.RequireCode {
		return ""
	}
	return strings.TrimSpace(`
6. 本轮需求明确要求编程题，type 必须固定为 code，禁止输出 subjective。
7. 编程题必须同时输出 4 个核心部分：answer（代码参考答案）、solution（代码思路解析，必须按标题式小节输出，含题意总结/解题思路/关键步骤/边界条件/复杂度分析/常见错法六节）、judge_config.public_test_cases（恰好 3 条公开样例）、judge_config.hidden_test_cases（提交判题使用的隐藏用例集）。
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
	switch strings.TrimSpace(language) {
	case "go":
		return "本轮必须生成 Go 语言特性或 Go 核心机制相关题卡。"
	case "java":
		return "本轮必须生成 Java 相关题卡，用于满足显式语言配额。"
	default:
		return "未指定单独语言配额，本轮遵循整体需求与智能体指令。"
	}
}

// isPipelineMockOutput 判断当前模型输出是否来自仓库内置的 Mock Provider。
func isPipelineMockOutput(raw string) bool {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return false
	}
	for _, marker := range []string{
		"作为一个Mock AI",
		"这是一个Mock流式响应",
		"实际集成后将连接真实的AI模型",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

// applyPipelineCardDefaults 为题卡填充默认的来源和置信度字段。
func applyPipelineCardDefaults(card *QuestionCandidate) {
	if card.Confidence == 0 {
		card.Confidence = 0.94
	}
	if card.SourceType == "" {
		card.SourceType = "generated"
	}
	if card.SourceLabel == "" {
		card.SourceLabel = "AI 智能体生成"
	}
	if card.SourceTitle == "" {
		card.SourceTitle = "智能体候选题卡"
	}
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
5. title、content、answer、explanation 都必须填写完整；当 type=code 时，solution 必须按下面固定小节格式输出（用纯文本小节标题，不要用 JSON 对象，不要用 Markdown 的 # 符号）：
题意总结：一句话概括题目要做什么
解题思路：详细说明核心算法思路
关键步骤：1. 步骤一 2. 步骤二 3. 步骤三
边界条件：空输入、单个元素、超大输入等特殊情况如何处理
复杂度分析：时间复杂度 O(...)，空间复杂度 O(...)
常见错法：容易犯的错误点
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

	// 应用运行时覆盖参数（对齐单体 compact 模式）
	effectiveCfg := *cfg
	if constraints.RequireCode {
		effectiveCfg.MaxTokens = 2600
		effectiveCfg.Temperature = 0.2
	} else {
		effectiveCfg.MaxTokens = 1400
		effectiveCfg.Temperature = 0.4
	}

	// 创建独立的 context，避免上游超时影响
	llmCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	messages := []Message{{Role: "user", Content: prompt}}
	resp, err := uc.llm.Chat(llmCtx, messages, &effectiveCfg)
	if err != nil {
		attempt.FailureStage = "model_call"
		return attempt, fmt.Errorf("模型调用失败: %w", err)
	}

	attempt.TraceOutput = resp.Content

	// 检测 Mock Provider 输出
	if isPipelineMockOutput(resp.Content) {
		attempt.FailureStage = "provider"
		return attempt, fmt.Errorf("当前调用实际返回了 Mock Provider 输出，请检查 AI 配置中的主 provider、fallback provider 与 API Key")
	}

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

	// 第七步：应用卡片默认值（对齐单体）
	applyPipelineCardDefaults(finalizedCard)

	// 第八步：补充标签（对齐单体 buildQuestionPipelineTags）
	finalizedCard.Tags = buildPipelineTags(finalizedCard.Tags, requirement)

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
	industryName string,
	categories []string,
) (*GenerateQuestionCandidatesResult, error) {
	return uc.GenerateQuestionCandidatesStream(ctx, industryCode, requirement, agentPrompt, candidateCount, industryName, categories, nil)
}

// GenerateQuestionCandidatesStream 逐张生成题卡，支持流式事件推送。
func (uc *AdminUseCase) GenerateQuestionCandidatesStream(
	ctx context.Context,
	industryCode string,
	requirement string,
	agentPrompt string,
	candidateCount int32,
	industryName string,
	categories []string,
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

	// 使用传入的行业名称和分类列表
	if industryName == "" {
		industryName = industryCode
	}
	if categories == nil {
		categories = make([]string, 0)
	}

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
				log.Warnf("%s", warning)

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

			// 检查是否重复（对齐单体：title+answer 复合 key）
			newCard := attempt.Cards[0]
			isDuplicate := false
			newKey := strings.ToLower(strings.TrimSpace(newCard.Title)) + "||" + strings.ToLower(strings.TrimSpace(newCard.Answer))
			if newKey != "||" {
				for _, existing := range cards {
					existKey := strings.ToLower(strings.TrimSpace(existing.Title)) + "||" + strings.ToLower(strings.TrimSpace(existing.Answer))
					if existKey == newKey {
						isDuplicate = true
						break
					}
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

	// 后处理：去重、过滤、约束强制执行（对齐单体）
	cards = dedupeQuestionPipelineCards(cards)
	cards = filterQuestionPipelineCardsByIntent(cards, constraints)
	cards = enforceQuestionPipelineCardConstraints(cards, constraints)

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

// ==================== 后处理：去重、过滤、约束 ====================

// dedupeQuestionPipelineCards 按 title+answer 复合 key 去重，按 confidence 降序排列。
func dedupeQuestionPipelineCards(cards []*QuestionCandidate) []*QuestionCandidate {
	seen := make(map[string]bool, len(cards))
	filtered := make([]*QuestionCandidate, 0, len(cards))
	for _, card := range cards {
		key := strings.ToLower(strings.TrimSpace(card.Title)) + "||" + strings.ToLower(strings.TrimSpace(card.Answer))
		if key == "||" || seen[key] {
			continue
		}
		seen[key] = true
		filtered = append(filtered, card)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Confidence > filtered[j].Confidence
	})
	return filtered
}

// filterQuestionPipelineCardsByIntent 当需求聚焦语言特性时，过滤掉泛项目类题卡。
func filterQuestionPipelineCardsByIntent(cards []*QuestionCandidate, constraints questionPipelineConstraintProfile) []*QuestionCandidate {
	if !constraints.ExcludeProjectCards && !constraints.GoFeatureOnly {
		return cards
	}
	filtered := make([]*QuestionCandidate, 0, len(cards))
	for _, card := range cards {
		if shouldDropQuestionPipelineProjectCard(card, constraints) {
			continue
		}
		filtered = append(filtered, card)
	}
	// 如果过滤后为空且没有硬约束，返回原始卡片
	if len(filtered) == 0 && !constraints.RequireCode && !constraints.RequireSubjective {
		return cards
	}
	if len(filtered) == 0 {
		return cards
	}
	return filtered
}

// shouldDropQuestionPipelineProjectCard 判断是否应丢弃泛项目类题卡。
func shouldDropQuestionPipelineProjectCard(card *QuestionCandidate, constraints questionPipelineConstraintProfile) bool {
	if constraints.ExcludeProjectCards && isQuestionPipelineGenericProjectCard(card) {
		return true
	}
	if constraints.GoFeatureOnly && !matchesQuestionPipelineLanguageFocus(card, "go") {
		return true
	}
	return false
}

// isQuestionPipelineGenericProjectCard 判断是否为泛项目类题卡。
func isQuestionPipelineGenericProjectCard(card *QuestionCandidate) bool {
	combined := strings.ToLower(card.Title + " " + card.Content + " " + card.Category)
	projectKeywords := []string{"项目经历", "职业规划", "微服务治理", "行为面试", "项目管理", "团队协作"}
	for _, kw := range projectKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

// matchesQuestionPipelineLanguageFocus 判断题卡是否匹配目标语言焦点。
func matchesQuestionPipelineLanguageFocus(card *QuestionCandidate, language string) bool {
	if language == "" {
		return true
	}
	combined := strings.ToLower(card.Title + " " + card.Content + " " + card.Answer + " " + strings.Join(card.Tags, " "))
	switch language {
	case "go":
		return strings.Contains(combined, "go") || strings.Contains(combined, "golang") || strings.Contains(combined, "goroutine") || strings.Contains(combined, "channel")
	case "java":
		return strings.Contains(combined, "java") || strings.Contains(combined, "jvm") || strings.Contains(combined, "spring")
	case "python":
		return strings.Contains(combined, "python") || strings.Contains(combined, "py")
	}
	return false
}

// enforceQuestionPipelineCardConstraints 对题卡执行硬约束强制检查。
func enforceQuestionPipelineCardConstraints(cards []*QuestionCandidate, constraints questionPipelineConstraintProfile) []*QuestionCandidate {
	if !constraints.RequireCode && !constraints.RequireSubjective && !constraints.GoFeatureOnly && !constraints.ExcludeProjectCards {
		return cards
	}
	filtered := make([]*QuestionCandidate, 0, len(cards))
	for _, card := range cards {
		if constraints.RequireCode && normalizeQuestionPipelineType(card.Type) != "code" {
			card.Type = "code"
		}
		if constraints.RequireSubjective && normalizeQuestionPipelineType(card.Type) != "subjective" {
			card.Type = "subjective"
		}
		if constraints.ExcludeProjectCards && isQuestionPipelineGenericProjectCard(card) {
			continue
		}
		if constraints.GoFeatureOnly && !matchesQuestionPipelineLanguageFocus(card, "go") {
			continue
		}
		filtered = append(filtered, card)
	}
	if len(filtered) == 0 {
		return cards
	}
	return filtered
}

// pipelineCommandPrefixes 岗位要求片段里常见的命令引导词，提取主题词时需剔除。
// 注意：更长的前缀需放在较短前缀之前（如"但保证"先于"保证"），保证一次剥离到主题词。
var pipelineCommandPrefixes = []string{
	"请生成", "生成", "聚焦于", "其中必须包括", "其中必须", "必须包括", "必须",
	"务必", "确保", "但保证", "保证", "其余题目", "其余", "并且", "同时", "要求",
	"需要", "结合", "输出", "请", "但", "例如",
}

// pipelineNoiseWords 剥离引导词后剩余的无信息量词，不作为主题词。
var pipelineNoiseWords = map[string]bool{
	"随意": true, "任意": true, "即可": true, "就行": true, "就好": true,
	"都可以": true, "都行": true, "等等": true, "等": true, "其它": true, "其他": true,
}

// buildPipelineTags 合并标签并从需求中提取关键词补充（对齐单体 buildQuestionPipelineTags）。
func buildPipelineTags(tags []string, requirement string) []string {
	merged := make([]string, 0, len(tags)+2)
	merged = append(merged, tags...)

	for _, topic := range extractTopicsFromRequirement(requirement) {
		if len([]rune(topic)) > 18 {
			continue
		}
		merged = append(merged, topic)
		if len(merged) >= 6 {
			break
		}
	}

	return deduplicateStrings(merged)
}

// extractTopicsFromRequirement 从岗位要求中提炼主题词：剔除命令式引导语，只保留有信息量的短词，
// 并额外提取"XX和YY问题"句式里的明确知识点（如"岛屿数量和无重复最长子串两个问题"）。
func extractTopicsFromRequirement(requirement string) []string {
	parts := strings.FieldsFunc(requirement, func(r rune) bool {
		return r == '，' || r == ',' || r == '；' || r == ';' || r == '、' || r == '\n' || r == '\r'
	})

	topics := make([]string, 0, len(parts)+2)
	seen := make(map[string]bool, len(parts)+2)
	appendTopic := func(raw string) {
		cleaned := cleanPipelineTopic(raw)
		if cleaned == "" || seen[cleaned] {
			return
		}
		// 剔除无信息量的虚词
		if pipelineNoiseWords[cleaned] {
			return
		}
		// 只保留 2~14 个字符的主题词：过短无信息量，过长说明不是提炼结果
		runeLen := len([]rune(cleaned))
		if runeLen < 2 || runeLen > 14 {
			return
		}
		seen[cleaned] = true
		topics = append(topics, cleaned)
	}

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		// 去掉开头的命令引导词与句尾残留标点
		stripped := trimPipelineCommandPrefix(trimmed)
		stripped = strings.TrimRight(stripped, "。.!！?？;；,，")
		appendTopic(stripped)
	}

	// 提取"XX和YY（两个）问题"句式里的知识点
	for _, knowledge := range extractPipelineKnowledgePoints(requirement) {
		appendTopic(knowledge)
	}

	if len(topics) == 0 {
		appendTopic(requirement)
	}
	return topics
}

// trimPipelineCommandPrefix 去掉片段开头出现的命令引导词。
func trimPipelineCommandPrefix(raw string) string {
	text := strings.TrimSpace(raw)
	for _, prefix := range pipelineCommandPrefixes {
		if strings.HasPrefix(text, prefix) {
			text = strings.TrimSpace(strings.TrimPrefix(text, prefix))
			break
		}
	}
	return text
}

// cleanPipelineTopic 清理主题词里的平台修饰词，保留知识点本体。
func cleanPipelineTopic(raw string) string {
	text := strings.TrimSpace(raw)
	for _, repl := range []string{"力扣同款的", "力扣同款", "力扣", "LeetCode", "leetcode"} {
		text = strings.ReplaceAll(text, repl, "")
	}
	// 去掉可能残留的定语"的"字开头
	text = strings.TrimPrefix(strings.TrimSpace(text), "的")
	return strings.TrimSpace(text)
}

// extractPipelineKnowledgePoints 提取"XX和YY问题"句式里的知识点名称。
// 先移除命令引导词与平台修饰词，避免被卷进知识点（如"其中必须包括力扣同款的岛屿数量"）。
func extractPipelineKnowledgePoints(requirement string) []string {
	cleaned := requirement
	for _, token := range []string{
		"请生成", "力扣同款的", "其中必须包括", "其中必须", "必须包括", "聚焦于",
		"力扣同款", "必须", "力扣", "LeetCode", "leetcode", "生成",
	} {
		cleaned = strings.ReplaceAll(cleaned, token, "")
	}
	// 组 1/组 2 用非贪婪匹配，避免把"两个"等量词卷进知识点
	re := regexp.MustCompile(`([\p{Han}A-Za-z0-9]{2,12}?)[和、与及]([\p{Han}A-Za-z0-9]{2,12}?)\s*(?:两个|两|等|等等)?\s*问题`)
	matches := re.FindAllStringSubmatch(cleaned, -1)
	result := make([]string, 0, len(matches)*2)
	for _, m := range matches {
		if len(m) >= 3 {
			result = append(result, m[1], m[2])
		}
	}
	return result
}

// deduplicateStrings 字符串切片去重。
func deduplicateStrings(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	return result
}
