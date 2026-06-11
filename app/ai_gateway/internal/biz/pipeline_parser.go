package biz

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ==================== 解析核心函数 ====================

// decodeQuestionPipelineSingleCardResponse 解析逐张直生产出的单卡输出，返回命中的候选 JSON 片段供调试复盘。
func decodeQuestionPipelineSingleCardResponse(raw string) (*QuestionCandidate, string, error) {
	candidates := buildQuestionPipelineSingleCardJSONCandidates(raw)
	var lastErr error
	for _, candidate := range candidates {
		value, err := decodeQuestionPipelineJSONValue(candidate)
		if err != nil {
			lastErr = err
			continue
		}
		card, ok := normalizeQuestionPipelineSingleCardValue(value)
		if !ok {
			lastErr = fmt.Errorf("single card not found")
			continue
		}
		return card, candidate, nil
	}

	cards := parseQuestionPipelineCardsText(sanitizeQuestionPipelineModelOutput(raw))
	if card, ok := pickQuestionPipelineMostCompleteCard(cards); ok {
		return card, sanitizeQuestionPipelineModelOutput(raw), nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("json payload not found")
	}
	return nil, "", lastErr
}

// buildQuestionPipelineSingleCardJSONCandidates 构造单卡解析优先级更高的候选片段。
func buildQuestionPipelineSingleCardJSONCandidates(raw string) []string {
	trimmed := sanitizeQuestionPipelineModelOutput(raw)
	if trimmed == "" {
		return nil
	}

	candidates := make([]string, 0, 16)
	appendExpanded := func(candidate string) {
		for _, expanded := range expandQuestionPipelineJSONCandidate(candidate) {
			if !containsQuestionPipelineString(candidates, expanded) {
				candidates = append(candidates, expanded)
			}
		}
	}

	for _, marker := range []string{"```json", "```JSON", "```", "{\"cards\"", "{\"questions\"", "{\"title\"", "\"cards\":", "\"title\":"} {
		appendExpanded(extractQuestionPipelineTailCandidateAfterMarker(trimmed, marker))
	}
	for _, candidate := range buildQuestionPipelineJSONCandidates(trimmed) {
		if !containsQuestionPipelineString(candidates, candidate) {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

// extractQuestionPipelineTailCandidateAfterMarker 从末尾标记之后截取候选片段。
func extractQuestionPipelineTailCandidateAfterMarker(raw string, marker string) string {
	raw = strings.TrimSpace(raw)
	marker = strings.TrimSpace(marker)
	if raw == "" || marker == "" {
		return ""
	}

	loweredRaw := strings.ToLower(raw)
	loweredMarker := strings.ToLower(marker)
	index := strings.LastIndex(loweredRaw, loweredMarker)
	if index < 0 {
		return ""
	}

	start := index
	if strings.HasPrefix(loweredMarker, "```") {
		start += len(marker)
		if lineEnd := strings.Index(raw[start:], "\n"); lineEnd >= 0 {
			start += lineEnd + 1
		}
	}
	if start < 0 || start >= len(raw) {
		return ""
	}
	return strings.TrimSpace(raw[start:])
}

// normalizeQuestionPipelineSingleCardValue 将任意 JSON 值归一化为单张题卡。
func normalizeQuestionPipelineSingleCardValue(value any) (*QuestionCandidate, bool) {
	if item, ok := value.(map[string]any); ok {
		if card, ok := normalizeQuestionPipelineModelCard(item); ok {
			return card, true
		}
	}
	return pickQuestionPipelineMostCompleteCard(normalizeQuestionPipelineModelCards(value))
}

// pickQuestionPipelineMostCompleteCard 从候选题卡里挑选字段最完整的一张。
func pickQuestionPipelineMostCompleteCard(cards []*QuestionCandidate) (*QuestionCandidate, bool) {
	if len(cards) == 0 {
		return nil, false
	}

	best := cards[0]
	bestScore := scoreQuestionPipelineCard(best)
	for _, card := range cards[1:] {
		score := scoreQuestionPipelineCard(card)
		if score > bestScore {
			best = card
			bestScore = score
		}
	}
	return best, true
}

// scoreQuestionPipelineCard 评估题卡完整度。
func scoreQuestionPipelineCard(card *QuestionCandidate) int {
	if card == nil {
		return 0
	}
	score := 0
	if card.Title != "" {
		score += 2
	}
	if card.Content != "" {
		score += 2
	}
	if card.Answer != "" {
		score += 2
	}
	if card.Explanation != "" {
		score += 1
	}
	if card.Solution != "" {
		score += 1
	}
	if card.Type != "" {
		score += 1
	}
	if card.Difficulty != "" {
		score += 1
	}
	if len(card.Tags) > 0 {
		score += 1
	}
	return score
}

// normalizeQuestionPipelineModelCards 将任意 JSON 值归一化为题卡数组。
func normalizeQuestionPipelineModelCards(value any) []*QuestionCandidate {
	items := extractQuestionPipelineItemList(
		value,
		"cards", "questions", "items", "list", "results", "data", "result",
		"题卡", "题目", "候选题卡", "候选题目",
	)
	if len(items) == 0 {
		return nil
	}

	cards := make([]*QuestionCandidate, 0, len(items))
	for _, item := range items {
		normalized, ok := normalizeQuestionPipelineModelCard(item)
		if !ok {
			continue
		}
		cards = append(cards, normalized)
	}

	return cards
}

// normalizeQuestionPipelineModelCard 归一化单张题卡，兼容多种字段别名。
func normalizeQuestionPipelineModelCard(value any) (*QuestionCandidate, bool) {
	item, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}

	title := firstNonEmptyString(
		readQuestionPipelineString(item, "title"),
		readQuestionPipelineString(item, "name"),
		readQuestionPipelineString(item, "topic"),
		readQuestionPipelineString(item, "question_title"),
		readQuestionPipelineString(item, "标题"),
		readQuestionPipelineString(item, "题目标题"),
		readQuestionPipelineString(item, "考点"),
	)
	content := firstNonEmptyString(
		readQuestionPipelineString(item, "content"),
		readQuestionPipelineString(item, "question"),
		readQuestionPipelineString(item, "body"),
		readQuestionPipelineString(item, "prompt"),
		readQuestionPipelineString(item, "description"),
		readQuestionPipelineString(item, "题目"),
		readQuestionPipelineString(item, "题干"),
		readQuestionPipelineString(item, "问题"),
	)
	answer := firstNonEmptyString(
		readQuestionPipelineString(item, "answer"),
		readQuestionPipelineString(item, "reference_answer"),
		readQuestionPipelineString(item, "standard_answer"),
		readQuestionPipelineString(item, "sample_answer"),
		readQuestionPipelineString(item, "expected_answer"),
		readQuestionPipelineString(item, "analysis"),
		readQuestionPipelineString(item, "答案"),
		readQuestionPipelineString(item, "参考答案"),
		readQuestionPipelineString(item, "标准答案"),
		readQuestionPipelineString(item, "解析"),
	)
	solution := firstNonEmptyString(
		readQuestionPipelineString(item, "solution"),
		readQuestionPipelineString(item, "structured_solution"),
		readQuestionPipelineString(item, "solution_text"),
		readQuestionPipelineString(item, "code_solution"),
		readQuestionPipelineString(item, "solution_analysis"),
		readQuestionPipelineString(item, "code_analysis"),
		readQuestionPipelineString(item, "思路解析"),
		readQuestionPipelineString(item, "代码思路解析"),
		readQuestionPipelineString(item, "题解"),
	)
	if title == "" {
		title = summarizeQuestionPipelineTitle(content)
	}
	if content == "" {
		content = title
	}
	if title == "" || content == "" || answer == "" {
		return nil, false
	}

	judgeConfig := readQuestionPipelineObjectValue(item, "judge_config", "judgeConfig", "judge-config", "judgeconfig", "判题配置", "测评配置", "测试配置")

	tags := readQuestionPipelineStringSlice(item, "tags", "keywords", "points", "标签", "关键词")

	return &QuestionCandidate{
		Title:       title,
		Content:     content,
		Type:        normalizeQuestionPipelineType(readQuestionPipelineString(item, "type", "question_type", "kind", "题型", "类型")),
		Difficulty:  normalizeQuestionPipelineDifficulty(readQuestionPipelineString(item, "difficulty", "level", "难度")),
		Category:    readQuestionPipelineString(item, "category", "classification", "domain", "类别", "分类"),
		Answer:      answer,
		Solution:    solution,
		Explanation: firstNonEmptyString(readQuestionPipelineString(item, "explanation", "rationale", "reasoning", "解释", "说明")),
		Tags:        tags,
		JudgeConfig: judgeConfig,
	}, true
}

// extractQuestionPipelineItemList 递归提取列表字段。
func extractQuestionPipelineItemList(value any, keys ...string) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case map[string]any:
		for _, key := range keys {
			if child, ok := typed[key]; ok {
				if list := extractQuestionPipelineItemList(child, keys...); len(list) > 0 {
					return list
				}
			}
		}
		if looksLikeQuestionPipelineItem(typed) {
			return []any{typed}
		}
		for _, child := range typed {
			if list := extractQuestionPipelineItemList(child, keys...); len(list) > 0 {
				return list
			}
		}
	case string:
		decoded, ok := decodeQuestionPipelineEmbeddedValue(typed)
		if ok {
			return extractQuestionPipelineItemList(decoded, keys...)
		}
	}

	return nil
}

// looksLikeQuestionPipelineItem 判断对象是否像题卡。
func looksLikeQuestionPipelineItem(item map[string]any) bool {
	_, hasTitle := item["title"]
	_, hasContent := item["content"]
	_, hasAnswer := item["answer"]
	_, hasQuestion := item["question"]
	return (hasTitle || hasQuestion) && (hasContent || hasAnswer)
}

// decodeQuestionPipelineEmbeddedValue 尝试解析字符串化的 JSON。
func decodeQuestionPipelineEmbeddedValue(raw string) (any, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || (raw[0] != '{' && raw[0] != '[') {
		return nil, false
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, false
	}
	return value, true
}

// ==================== 清理函数 ====================

// sanitizeQuestionPipelineModelOutput 清理模型输出。
func sanitizeQuestionPipelineModelOutput(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// 移除 BOM
	raw = strings.TrimPrefix(raw, "")

	// 移除推理块
	raw = stripQuestionPipelineReasoningBlocks(raw)

	// 移除代码块
	raw = stripQuestionPipelineCodeFence(raw)

	if raw == "" {
		return ""
	}

	// 移除首行的 "json" 标记
	lines := strings.Split(raw, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(strings.ToLower(lines[0]))
		if firstLine == "json" || firstLine == "json输出" || firstLine == "outputjson" || firstLine == "resultjson" {
			return strings.TrimSpace(strings.Join(lines[1:], "\n"))
		}
	}

	return strings.TrimSpace(raw)
}

// stripQuestionPipelineReasoningBlocks 清理推理块。
func stripQuestionPipelineReasoningBlocks(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	type blockMarker struct {
		start string
		end   string
	}

	blocks := []blockMarker{
		{start: "<think>", end: "</think>"},
		{start: "<reasoning>", end: "</reasoning>"},
	}

	lowered := strings.ToLower(raw)
	for _, block := range blocks {
		for {
			start := strings.Index(lowered, block.start)
			if start < 0 {
				break
			}
			end := strings.Index(lowered[start+len(block.start):], block.end)
			if end < 0 {
				raw = strings.TrimSpace(raw[:start])
				lowered = strings.ToLower(raw)
				break
			}

			realEnd := start + len(block.start) + end + len(block.end)
			raw = raw[:start] + raw[realEnd:]
			lowered = strings.ToLower(raw)
		}
	}

	raw = strings.ReplaceAll(raw, "<think>", "")
	raw = strings.ReplaceAll(raw, "</think>", "")
	raw = strings.ReplaceAll(raw, "<reasoning>", "")
	raw = strings.ReplaceAll(raw, "</reasoning>", "")
	return strings.TrimSpace(raw)
}

// stripQuestionPipelineCodeFence 去除代码块包裹。
func stripQuestionPipelineCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "```") {
		if lineEnd := strings.Index(trimmed, "\n"); lineEnd >= 0 {
			trimmed = trimmed[lineEnd+1:]
		} else {
			trimmed = strings.TrimPrefix(trimmed, "```")
		}
	}
	trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
	return strings.TrimSpace(trimmed)
}

// ==================== JSON 提取函数 ====================

// decodeQuestionPipelineJSONValue 解析 JSON 值。
func decodeQuestionPipelineJSONValue(raw string) (any, error) {
	candidates := buildQuestionPipelineJSONCandidates(raw)
	for _, candidate := range candidates {
		var payload any
		if err := json.Unmarshal([]byte(candidate), &payload); err == nil {
			return payload, nil
		}
	}

	return nil, fmt.Errorf("json payload not found")
}

// buildQuestionPipelineJSONCandidates 构造候选 JSON 片段。
func buildQuestionPipelineJSONCandidates(raw string) []string {
	trimmed := sanitizeQuestionPipelineModelOutput(raw)
	candidates := make([]string, 0, 8)
	for _, candidate := range []string{
		trimmed,
		extractQuestionPipelineJSONObject(trimmed),
		extractQuestionPipelineJSONArray(trimmed),
	} {
		for _, expanded := range expandQuestionPipelineJSONCandidate(candidate) {
			if !containsQuestionPipelineString(candidates, expanded) {
				candidates = append(candidates, expanded)
			}
		}
	}

	for _, candidate := range extractQuestionPipelineBalancedJSONSegments(trimmed) {
		for _, expanded := range expandQuestionPipelineJSONCandidate(candidate) {
			if !containsQuestionPipelineString(candidates, expanded) {
				candidates = append(candidates, expanded)
			}
		}
	}

	return candidates
}

// expandQuestionPipelineJSONCandidate 展开候选 JSON 片段。
func expandQuestionPipelineJSONCandidate(candidate string) []string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return nil
	}

	values := []string{candidate}
	appendValue := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || containsQuestionPipelineString(values, value) {
			return
		}
		values = append(values, value)
	}

	for index := 0; index < len(values); index++ {
		current := values[index]
		if unquoted, err := strconv.Unquote(current); err == nil {
			appendValue(unquoted)
		}
		appendValue(normalizeQuestionPipelineJSONPunctuation(current))
		appendValue(normalizeQuestionPipelineSingleQuotedJSON(current))
		appendValue(stripQuestionPipelineJSONTrailingCommas(current))
	}

	for index := 0; index < len(values); index++ {
		current := values[index]
		appendValue(stripQuestionPipelineJSONTrailingCommas(normalizeQuestionPipelineJSONPunctuation(current)))
		appendValue(stripQuestionPipelineJSONTrailingCommas(normalizeQuestionPipelineSingleQuotedJSON(current)))
	}
	return values
}

// normalizeQuestionPipelineJSONPunctuation 统一标点符号。
func normalizeQuestionPipelineJSONPunctuation(raw string) string {
	replacer := strings.NewReplacer(
		"“", `"`, "”", `"`, "„", `"`, "‟", `"`,
		"‘", `'`, "’", `'`, "‚", `'`, "‛", `'`,
		"：", ":", "，", ",",
		"｛", "{", "｝", "}",
		"［", "[", "］", "]",
	)
	return strings.TrimSpace(replacer.Replace(raw))
}

// normalizeQuestionPipelineSingleQuotedJSON 将单引号 JSON 转为双引号。
func normalizeQuestionPipelineSingleQuotedJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.Contains(raw, "'") {
		return raw
	}

	var builder strings.Builder
	builder.Grow(len(raw))
	inDouble := false
	inSingle := false
	escaping := false
	changed := false

	for _, char := range raw {
		if escaping {
			if inSingle && char == '"' {
				builder.WriteString(`\"`)
			} else {
				builder.WriteRune(char)
			}
			escaping = false
			continue
		}

		if char == '\\' {
			builder.WriteRune(char)
			if inSingle || inDouble {
				escaping = true
			}
			continue
		}

		switch {
		case inSingle:
			if char == '\'' {
				builder.WriteRune('"')
				inSingle = false
				changed = true
				continue
			}
			if char == '"' {
				builder.WriteString(`\"`)
				changed = true
				continue
			}
			builder.WriteRune(char)
		case inDouble:
			builder.WriteRune(char)
			if char == '"' {
				inDouble = false
			}
		default:
			switch char {
			case '\'':
				builder.WriteRune('"')
				inSingle = true
				changed = true
			case '"':
				builder.WriteRune(char)
				inDouble = true
			default:
				builder.WriteRune(char)
			}
		}
	}

	if inSingle || !changed {
		return raw
	}
	return strings.TrimSpace(builder.String())
}

// stripQuestionPipelineJSONTrailingCommas 去掉尾部逗号。
func stripQuestionPipelineJSONTrailingCommas(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	trailingCommaPattern := regexp.MustCompile(`,\s*([}\]])`)
	return trailingCommaPattern.ReplaceAllString(raw, "$1")
}

// extractQuestionPipelineJSONObject 提取 JSON 对象。
func extractQuestionPipelineJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return ""
	}
	return strings.TrimSpace(raw[start : end+1])
}

// extractQuestionPipelineJSONArray 提取 JSON 数组。
func extractQuestionPipelineJSONArray(raw string) string {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end < start {
		return ""
	}
	return strings.TrimSpace(raw[start : end+1])
}

// extractQuestionPipelineBalancedJSONSegments 提取所有完整闭合的 JSON 片段。
func extractQuestionPipelineBalancedJSONSegments(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	segments := make([]string, 0)
	start := -1
	depth := 0
	inString := false
	escaping := false

	for index, char := range raw {
		if escaping {
			escaping = false
			continue
		}

		if char == '\\' && inString {
			escaping = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}

		switch char {
		case '{', '[':
			if depth == 0 {
				start = index
			}
			depth++
		case '}', ']':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				candidate := strings.TrimSpace(raw[start : index+1])
				if candidate != "" {
					segments = append(segments, candidate)
				}
				start = -1
			}
		}
	}

	// 反转，优先保留后出现的结果块
	reversed := make([]string, 0, len(segments))
	for index := len(segments) - 1; index >= 0; index-- {
		if !containsQuestionPipelineString(reversed, segments[index]) {
			reversed = append(reversed, segments[index])
		}
	}
	return reversed
}

// ==================== 文本解析回退 ====================

// parseQuestionPipelineCardsText 从纯文本中解析题卡。
func parseQuestionPipelineCardsText(raw string) []*QuestionCandidate {
	lines := splitQuestionPipelineLines(stripQuestionPipelineCodeFence(raw))
	if len(lines) == 0 {
		return nil
	}

	cards := make([]*QuestionCandidate, 0)
	current := &QuestionCandidate{}
	currentField := ""
	for _, line := range lines {
		normalizedLine := trimQuestionPipelineListMarker(line)
		if normalizedLine == "" || isQuestionPipelineCardNoiseLine(normalizedLine) {
			continue
		}

		key, value, ok := splitQuestionPipelineKeyValue(normalizedLine)
		if ok {
			field := normalizeQuestionPipelineFieldKey(key)
			if isQuestionPipelineCardBoundaryField(field) && hasQuestionPipelineCardContent(current) {
				cards = append(cards, current)
				current = &QuestionCandidate{}
			}
			applyQuestionPipelineCardField(current, field, value)
			currentField = field
			continue
		}

		if appendQuestionPipelineCardContinuation(current, currentField, normalizedLine) {
			continue
		}

		if strings.TrimSpace(current.Title) == "" {
			current.Title = normalizedLine
			if strings.TrimSpace(current.Content) == "" {
				current.Content = normalizedLine
			}
		}
	}

	if hasQuestionPipelineCardContent(current) {
		cards = append(cards, current)
	}
	return cards
}

// splitQuestionPipelineLines 分割行。
func splitQuestionPipelineLines(raw string) []string {
	parts := strings.Split(raw, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSpace(strings.TrimSuffix(part, "\r"))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// trimQuestionPipelineListMarker 去掉列表标记。
func trimQuestionPipelineListMarker(line string) string {
	line = strings.TrimSpace(line)
	for _, prefix := range []string{"- ", "* ", "• ", "· "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	// 去掉数字列表标记
	re := regexp.MustCompile(`^\d+[\.\)]\s*`)
	return re.ReplaceAllString(line, "")
}

// isQuestionPipelineCardNoiseLine 判断是否是噪音行。
func isQuestionPipelineCardNoiseLine(line string) bool {
	noisePatterns := []string{"---", "===", "***", "```"}
	for _, pattern := range noisePatterns {
		if strings.HasPrefix(line, pattern) {
			return true
		}
	}
	return false
}

// splitQuestionPipelineKeyValue 分割 key-value。
func splitQuestionPipelineKeyValue(line string) (string, string, bool) {
	for _, sep := range []string{":", "：", "="} {
		if index := strings.Index(line, sep); index > 0 {
			key := strings.TrimSpace(line[:index])
			value := strings.TrimSpace(line[index+len(sep):])
			if key != "" {
				return key, value, true
			}
		}
	}
	return "", "", false
}

// normalizeQuestionPipelineFieldKey 规范化字段名。
func normalizeQuestionPipelineFieldKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.ToLower(key)
	key = strings.TrimRight(key, ":：")
	return key
}

// isQuestionPipelineCardBoundaryField 判断是否是题卡边界字段。
func isQuestionPipelineCardBoundaryField(field string) bool {
	boundaryFields := []string{"title", "标题", "题目", "question", "topic"}
	for _, f := range boundaryFields {
		if field == f {
			return true
		}
	}
	return false
}

// hasQuestionPipelineCardContent 判断题卡是否有内容。
func hasQuestionPipelineCardContent(card *QuestionCandidate) bool {
	return card != nil && (card.Title != "" || card.Content != "" || card.Answer != "")
}

// applyQuestionPipelineCardField 应用字段值。
func applyQuestionPipelineCardField(card *QuestionCandidate, field, value string) {
	switch field {
	case "title", "标题", "题目", "name", "topic":
		card.Title = value
	case "content", "题干", "问题", "question", "body", "prompt", "description":
		card.Content = value
	case "answer", "答案", "参考答案", "标准答案", "reference_answer", "standard_answer":
		card.Answer = value
	case "explanation", "解析", "说明", "rationale", "reasoning":
		card.Explanation = value
	case "solution", "思路解析", "代码思路解析", "题解":
		card.Solution = value
	case "type", "题型", "类型", "question_type", "kind":
		card.Type = normalizeQuestionPipelineType(value)
	case "difficulty", "难度", "level":
		card.Difficulty = normalizeQuestionPipelineDifficulty(value)
	case "category", "分类", "类别", "classification", "domain":
		card.Category = value
	}
}

// appendQuestionPipelineCardContinuation 追加续行内容。
func appendQuestionPipelineCardContinuation(card *QuestionCandidate, field, line string) bool {
	if card == nil || field == "" || line == "" {
		return false
	}
	switch field {
	case "content", "题干", "问题", "question", "body", "prompt", "description":
		card.Content += "\n" + line
		return true
	case "answer", "答案", "参考答案", "标准答案", "reference_answer", "standard_answer":
		card.Answer += "\n" + line
		return true
	case "explanation", "解析", "说明", "rationale", "reasoning":
		card.Explanation += "\n" + line
		return true
	case "solution", "思路解析", "代码思路解析", "题解":
		card.Solution += "\n" + line
		return true
	}
	return false
}

// ==================== 辅助函数 ====================

// normalizeQuestionPipelineType 规范化题型。
func normalizeQuestionPipelineType(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	switch raw {
	case "code", "coding", "编程", "编程题", "代码", "代码题":
		return "code"
	case "subjective", "主观", "主观题", "问答", "问答题":
		return "subjective"
	case "choice", "单选", "单选题":
		return "choice"
	case "multi", "多选", "多选题":
		return "multi"
	default:
		return raw
	}
}

// normalizeQuestionPipelineDifficulty 规范化难度。
func normalizeQuestionPipelineDifficulty(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	switch raw {
	case "easy", "简单", "初级":
		return "easy"
	case "medium", "中等", "中级":
		return "medium"
	case "hard", "困难", "高级", "难":
		return "hard"
	default:
		return raw
	}
}

// readQuestionPipelineString 从 map 中读取字符串值。
func readQuestionPipelineString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			if str, ok := value.(string); ok {
				return strings.TrimSpace(str)
			}
		}
	}
	return ""
}

// readQuestionPipelineStringSlice 从 map 中读取字符串数组。
func readQuestionPipelineStringSlice(item map[string]any, keys ...string) []string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch typed := value.(type) {
			case []string:
				return typed
			case []any:
				result := make([]string, 0, len(typed))
				for _, v := range typed {
					if str, ok := v.(string); ok {
						result = append(result, str)
					}
				}
				return result
			}
		}
	}
	return nil
}

// readQuestionPipelineObjectValue 从 map 中读取对象值。
func readQuestionPipelineObjectValue(item map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := item[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

// firstNonEmptyString 返回第一个非空字符串。
func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// containsQuestionPipelineString 检查字符串切片是否包含指定字符串。
func containsQuestionPipelineString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

// summarizeQuestionPipelineTitle 从内容中提取标题摘要。
func summarizeQuestionPipelineTitle(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	// 取前 50 个字符作为标题
	runes := []rune(content)
	if len(runes) > 50 {
		return string(runes[:50]) + "..."
	}
	return content
}
