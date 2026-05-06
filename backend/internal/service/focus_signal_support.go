package service

import (
	"fmt"
	"sort"
	"strings"

	"makejob-backend/internal/model"
)

const defaultTrainingFocusSignalLimit = 3

// trainingFocusSignal 表示可被成长页、推荐接口和学习计划共同复用的训练重点信号。
type trainingFocusSignal struct {
	Tag                      string   `json:"tag"`
	TopicCode                string   `json:"topic_code,omitempty"`
	TopicTitle               string   `json:"topic_title,omitempty"`
	TopicProblemPattern      string   `json:"topic_problem_pattern,omitempty"`
	RelatedQuestionSets      []string `json:"related_question_sets,omitempty"`
	RecommendedActions       []string `json:"recommended_actions,omitempty"`
	PrimaryQuestionSet       string   `json:"primary_question_set,omitempty"`
	ArchiveOccurrenceCount   int      `json:"archive_occurrence_count"`
	InterviewOccurrenceCount int      `json:"interview_occurrence_count"`
	OccurrenceCount          int      `json:"occurrence_count"`
	Source                   string   `json:"source"`
	SourceLabel              string   `json:"source_label"`
	Reason                   string   `json:"reason"`
	SourceRef                string   `json:"source_ref,omitempty"`
	CollectionHint           string   `json:"collection_hint,omitempty"`
}

// trainingFocusSignalSeed 表示构造训练重点信号时使用的中间聚合结果。
type trainingFocusSignalSeed struct {
	Tag         string
	Count       int
	TopicCode   string
	Suggestions []string
	SourceRef   string
}

// buildTrainingFocusSignals 将学习档案与面试报告聚合为一组统一的训练重点信号。
func buildTrainingFocusSignals(entries []model.LearningArchiveEntry, interviews []model.MockInterview, limit int) []trainingFocusSignal {
	if limit <= 0 {
		limit = defaultTrainingFocusSignalLimit
	}

	archiveSeeds := collectArchiveFocusSignalSeeds(entries)
	interviewSeeds := collectInterviewFocusSignalSeeds(interviews)
	merged := make(map[string]*trainingFocusSignal, len(archiveSeeds)+len(interviewSeeds))

	for _, seed := range archiveSeeds {
		key := buildTrainingFocusSignalKey(seed.Tag, seed.TopicCode)
		signal := ensureTrainingFocusSignal(merged, key, seed.Tag, seed.TopicCode)
		signal.ArchiveOccurrenceCount += seed.Count
		signal.OccurrenceCount += seed.Count
		signal.SourceRef = pickTrainingFocusSignalSourceRef(signal.SourceRef, seed.SourceRef)
		signal.RecommendedActions = appendUniqueStrings(signal.RecommendedActions, seed.Suggestions...)
	}

	for _, seed := range interviewSeeds {
		key := buildTrainingFocusSignalKey(seed.Tag, seed.TopicCode)
		signal := ensureTrainingFocusSignal(merged, key, seed.Tag, seed.TopicCode)
		signal.InterviewOccurrenceCount += seed.Count
		signal.OccurrenceCount += seed.Count
		signal.SourceRef = pickTrainingFocusSignalSourceRef(signal.SourceRef, seed.SourceRef)
		signal.RecommendedActions = appendUniqueStrings(signal.RecommendedActions, seed.Suggestions...)
	}

	items := make([]trainingFocusSignal, 0, len(merged))
	for _, signal := range merged {
		items = append(items, hydrateTrainingFocusSignal(*signal))
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].OccurrenceCount == items[j].OccurrenceCount {
			if items[i].InterviewOccurrenceCount == items[j].InterviewOccurrenceCount {
				if items[i].ArchiveOccurrenceCount == items[j].ArchiveOccurrenceCount {
					return items[i].Tag < items[j].Tag
				}
				return items[i].ArchiveOccurrenceCount > items[j].ArchiveOccurrenceCount
			}
			return items[i].InterviewOccurrenceCount > items[j].InterviewOccurrenceCount
		}
		return items[i].OccurrenceCount > items[j].OccurrenceCount
	})

	if len(items) > limit {
		items = items[:limit]
	}
	return normalizeTrainingFocusSignals(items)
}

// collectArchiveFocusSignalSeeds 从学习档案中提取错因统计与建议聚合结果。
func collectArchiveFocusSignalSeeds(entries []model.LearningArchiveEntry) []trainingFocusSignalSeed {
	buckets := make(map[string]*trainingFocusSignalSeed, len(entries))
	for _, entry := range entries {
		tags := decodeWeeklyFocusTextList(entry.MistakeTagsJSON)
		suggestions := decodeWeeklyFocusTextList(entry.SuggestionsJSON)
		for _, tag := range tags {
			key := buildTrainingFocusSignalKey(tag, resolveMistakeTopicCodeByTag(tag))
			seed, exists := buckets[key]
			if !exists {
				seed = &trainingFocusSignalSeed{
					Tag:       strings.TrimSpace(tag),
					TopicCode: resolveMistakeTopicCodeByTag(tag),
				}
				buckets[key] = seed
			}
			seed.Count++
			if seed.SourceRef == "" {
				seed.SourceRef = strings.TrimSpace(entry.SourceRef)
			}
			seed.Suggestions = appendUniqueStrings(seed.Suggestions, suggestions...)
		}
	}
	return sortTrainingFocusSignalSeeds(buckets)
}

// collectInterviewFocusSignalSeeds 从面试报告中提取薄弱项统计与建议聚合结果。
func collectInterviewFocusSignalSeeds(interviews []model.MockInterview) []trainingFocusSignalSeed {
	buckets := make(map[string]*trainingFocusSignalSeed, len(interviews))
	for _, interview := range interviews {
		if interview.Status != model.InterviewStatusCompleted {
			continue
		}

		report, err := parseStoredInterviewReport(interview.ReportJSON)
		if err != nil || report == nil {
			continue
		}

		weaknesses := sanitizeWeeklyFocusTextList(report.Weaknesses)
		suggestions := sanitizeWeeklyFocusTextList(report.Suggestions)
		sourceRef := fmt.Sprintf("interview:%d", interview.ID)
		for _, weakness := range weaknesses {
			topicCode := resolveMistakeTopicCodeByTag(weakness)
			key := buildTrainingFocusSignalKey(weakness, topicCode)
			seed, exists := buckets[key]
			if !exists {
				seed = &trainingFocusSignalSeed{
					Tag:       strings.TrimSpace(weakness),
					TopicCode: topicCode,
				}
				buckets[key] = seed
			}
			seed.Count++
			if seed.SourceRef == "" {
				seed.SourceRef = sourceRef
			}
			seed.Suggestions = appendUniqueStrings(seed.Suggestions, suggestions...)
		}
	}
	return sortTrainingFocusSignalSeeds(buckets)
}

// sortTrainingFocusSignalSeeds 将聚合后的种子结果稳定排序，便于后续截断和复用。
func sortTrainingFocusSignalSeeds(buckets map[string]*trainingFocusSignalSeed) []trainingFocusSignalSeed {
	items := make([]trainingFocusSignalSeed, 0, len(buckets))
	for _, seed := range buckets {
		items = append(items, trainingFocusSignalSeed{
			Tag:         strings.TrimSpace(seed.Tag),
			Count:       seed.Count,
			TopicCode:   strings.TrimSpace(seed.TopicCode),
			Suggestions: sanitizeWeeklyFocusTextList(seed.Suggestions),
			SourceRef:   strings.TrimSpace(seed.SourceRef),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Tag < items[j].Tag
		}
		return items[i].Count > items[j].Count
	})
	return items
}

// ensureTrainingFocusSignal 获取指定键对应的信号对象，不存在时会创建空壳。
func ensureTrainingFocusSignal(
	merged map[string]*trainingFocusSignal,
	key string,
	tag string,
	topicCode string,
) *trainingFocusSignal {
	if signal, exists := merged[key]; exists {
		if signal.Tag == "" {
			signal.Tag = strings.TrimSpace(tag)
		}
		if signal.TopicCode == "" {
			signal.TopicCode = strings.TrimSpace(topicCode)
		}
		return signal
	}

	signal := &trainingFocusSignal{
		Tag:       strings.TrimSpace(tag),
		TopicCode: strings.TrimSpace(topicCode),
	}
	merged[key] = signal
	return signal
}

// hydrateTrainingFocusSignal 补齐信号专题信息、推荐动作和可解释说明。
func hydrateTrainingFocusSignal(signal trainingFocusSignal) trainingFocusSignal {
	if topic, ok := resolveTrainingFocusSignalTopic(signal); ok {
		signal.TopicCode = topic.Code
		signal.TopicTitle = topic.Title
		signal.TopicProblemPattern = topic.ProblemPattern
		signal.RelatedQuestionSets = sanitizeWeeklyFocusTextList(topic.RelatedQuestionSets)
		signal.RecommendedActions = appendUniqueStrings(topic.RecommendedActions, signal.RecommendedActions...)
	}

	signal.RecommendedActions = trimWeeklyFocusSuggestions(signal.RecommendedActions)
	if len(signal.RelatedQuestionSets) > 0 {
		signal.PrimaryQuestionSet = signal.RelatedQuestionSets[0]
	}
	signal.CollectionHint = signal.PrimaryQuestionSet
	signal.Source, signal.SourceLabel = buildTrainingFocusSignalSource(signal)
	signal.Reason = buildTrainingFocusSignalReason(signal)
	return signal
}

// normalizeTrainingFocusSignals 统一清理训练重点信号中的字符串和切片字段。
func normalizeTrainingFocusSignals(items []trainingFocusSignal) []trainingFocusSignal {
	result := make([]trainingFocusSignal, 0, len(items))
	for _, item := range items {
		item.Tag = strings.TrimSpace(item.Tag)
		if item.Tag == "" {
			continue
		}
		item.TopicCode = strings.TrimSpace(item.TopicCode)
		item.TopicTitle = strings.TrimSpace(item.TopicTitle)
		item.TopicProblemPattern = strings.TrimSpace(item.TopicProblemPattern)
		item.PrimaryQuestionSet = strings.TrimSpace(item.PrimaryQuestionSet)
		item.Source = strings.TrimSpace(item.Source)
		item.SourceLabel = strings.TrimSpace(item.SourceLabel)
		item.Reason = strings.TrimSpace(item.Reason)
		item.SourceRef = strings.TrimSpace(item.SourceRef)
		item.CollectionHint = strings.TrimSpace(item.CollectionHint)
		item.RelatedQuestionSets = sanitizeWeeklyFocusTextList(item.RelatedQuestionSets)
		item.RecommendedActions = trimWeeklyFocusSuggestions(item.RecommendedActions)
		result = append(result, item)
	}
	return result
}

// buildTrainingFocusSignalKey 为训练重点信号生成稳定聚合键。
func buildTrainingFocusSignalKey(tag string, topicCode string) string {
	if trimmedTopicCode := strings.TrimSpace(topicCode); trimmedTopicCode != "" {
		return "topic:" + strings.ToLower(trimmedTopicCode)
	}
	return "tag:" + normalizeWeeklyFocusKey(tag)
}

// pickTrainingFocusSignalSourceRef 优先保留已有来源引用，否则回填新的来源引用。
func pickTrainingFocusSignalSourceRef(current string, candidate string) string {
	current = strings.TrimSpace(current)
	if current != "" {
		return current
	}
	return strings.TrimSpace(candidate)
}

// resolveTrainingFocusSignalTopic 根据专题编码或错因标签查找最贴近的专题卡片。
func resolveTrainingFocusSignalTopic(signal trainingFocusSignal) (*MistakeTopicCard, bool) {
	if topic, ok := resolveMistakeTopicByCode(signal.TopicCode); ok {
		return topic, true
	}
	return resolveMistakeTopicByTag(signal.Tag)
}

// buildTrainingFocusSignalSource 生成训练重点信号的来源标识与展示标签。
func buildTrainingFocusSignalSource(signal trainingFocusSignal) (string, string) {
	switch {
	case signal.ArchiveOccurrenceCount > 0 && signal.InterviewOccurrenceCount > 0:
		return "mixed", "练习 + 面试"
	case signal.InterviewOccurrenceCount > 0:
		return "interview_report", "面试报告"
	default:
		return "learning_archive", "练习归档"
	}
}

// buildTrainingFocusSignalReason 为统一信号生成可直接展示的解释文案。
func buildTrainingFocusSignalReason(signal trainingFocusSignal) string {
	switch signal.Source {
	case "mixed":
		return fmt.Sprintf("最近练习里“%s”累计出现 %d 次，且最近 %d 场面试也反复暴露这个问题，适合本周优先补强。", signal.Tag, signal.ArchiveOccurrenceCount, signal.InterviewOccurrenceCount)
	case "interview_report":
		return fmt.Sprintf("最近 %d 场已完成面试都指向“%s”，建议围绕这个薄弱点做一轮专项训练。", signal.InterviewOccurrenceCount, signal.Tag)
	default:
		return fmt.Sprintf("最近练习归档里“%s”累计出现 %d 次，说明这个问题还在持续影响你的输出。", signal.Tag, signal.ArchiveOccurrenceCount)
	}
}

// matchTrainingFocusSignal 从任务标题和描述中匹配最贴近的训练重点信号。
func matchTrainingFocusSignal(task model.LearningTask, signals []trainingFocusSignal) *trainingFocusSignal {
	searchText := strings.ToLower(strings.Join([]string{
		strings.TrimSpace(task.Title),
		strings.TrimSpace(task.Description),
	}, "\n"))
	if strings.TrimSpace(searchText) == "" {
		return nil
	}

	for _, signal := range signals {
		for _, term := range buildTrainingFocusSignalTerms(signal) {
			normalizedTerm := strings.ToLower(strings.TrimSpace(term))
			if normalizedTerm == "" {
				continue
			}
			if strings.Contains(searchText, normalizedTerm) {
				copy := signal
				return &copy
			}
		}
	}
	return nil
}

// buildTrainingFocusSignalTerms 构造单个训练重点信号用于文本匹配的一组核心关键词。
func buildTrainingFocusSignalTerms(signal trainingFocusSignal) []string {
	terms := []string{
		signal.Tag,
		signal.TopicTitle,
	}
	if topic, ok := resolveTrainingFocusSignalTopic(signal); ok {
		terms = append(terms, topic.Tag, topic.Title)
	}
	return sanitizeWeeklyFocusTextList(terms)
}
