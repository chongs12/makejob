package biz

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const defaultTrainingFocusSignalLimit = 3

// TrainingFocusSignal 表示可被成长页、推荐接口和学习计划共同复用的训练重点信号
type TrainingFocusSignal struct {
	Tag                       string   `json:"tag"`
	TopicCode                 string   `json:"topic_code,omitempty"`
	TopicTitle                string   `json:"topic_title,omitempty"`
	TopicProblemPattern       string   `json:"topic_problem_pattern,omitempty"`
	RelatedQuestionSets       []string `json:"related_question_sets,omitempty"`
	RecommendedActions        []string `json:"recommended_actions,omitempty"`
	PrimaryQuestionSet        string   `json:"primary_question_set,omitempty"`
	ArchiveOccurrenceCount    int      `json:"archive_occurrence_count"`
	InterviewOccurrenceCount  int      `json:"interview_occurrence_count"`
	OccurrenceCount           int      `json:"occurrence_count"`
	DominantArchivePhase      string   `json:"dominant_archive_phase,omitempty"`
	DominantArchivePhaseLabel string   `json:"dominant_archive_phase_label,omitempty"`
	Source                    string   `json:"source"`
	SourceLabel               string   `json:"source_label"`
	Reason                    string   `json:"reason"`
	SourceRef                 string   `json:"source_ref,omitempty"`
}

// trainingFocusSignalSeed 表示构造训练重点信号时使用的中间聚合结果
type trainingFocusSignalSeed struct {
	Tag           string
	Count         int
	TopicCode     string
	Suggestions   []string
	SourceRef     string
	DominantPhase string
}

// BuildTrainingFocusSignals 将学习档案与面试报告聚合为一组统一的训练重点信号
func BuildTrainingFocusSignals(entries []*LearningArchiveEntry, limit int) []TrainingFocusSignal {
	if limit <= 0 {
		limit = defaultTrainingFocusSignalLimit
	}

	archiveSeeds := collectArchiveFocusSignalSeeds(entries)
	merged := make(map[string]*TrainingFocusSignal, len(archiveSeeds))

	for _, seed := range archiveSeeds {
		key := buildTrainingFocusSignalKey(seed.Tag, seed.TopicCode)
		signal := ensureTrainingFocusSignal(merged, key, seed.Tag, seed.TopicCode)
		signal.ArchiveOccurrenceCount += seed.Count
		signal.OccurrenceCount += seed.Count
		signal.DominantArchivePhase = pickTrainingFocusSignalDominantPhase(signal.DominantArchivePhase, seed.DominantPhase)
		signal.SourceRef = pickTrainingFocusSignalSourceRef(signal.SourceRef, seed.SourceRef)
		signal.RecommendedActions = appendUniqueStrings(signal.RecommendedActions, seed.Suggestions...)
	}

	items := make([]TrainingFocusSignal, 0, len(merged))
	for _, signal := range merged {
		items = append(items, hydrateTrainingFocusSignal(*signal))
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].OccurrenceCount == items[j].OccurrenceCount {
			if items[i].ArchiveOccurrenceCount == items[j].ArchiveOccurrenceCount {
				return items[i].Tag < items[j].Tag
			}
			return items[i].ArchiveOccurrenceCount > items[j].ArchiveOccurrenceCount
		}
		return items[i].OccurrenceCount > items[j].OccurrenceCount
	})

	if len(items) > limit {
		items = items[:limit]
	}
	return normalizeTrainingFocusSignals(items)
}

// collectArchiveFocusSignalSeeds 从学习档案中提取错因统计与建议聚合结果
func collectArchiveFocusSignalSeeds(entries []*LearningArchiveEntry) []trainingFocusSignalSeed {
	buckets := make(map[string]*trainingFocusSignalSeed, len(entries))
	for _, entry := range entries {
		tags := decodeJSONStringList(entry.MistakeTagsJSON)
		suggestions := decodeJSONStringList(entry.SuggestionsJSON)
		for _, tag := range tags {
			topicCode := ResolveMistakeTopicCodeByTag(tag)
			key := buildTrainingFocusSignalKey(tag, topicCode)
			seed, exists := buckets[key]
			if !exists {
				seed = &trainingFocusSignalSeed{
					Tag:       strings.TrimSpace(tag),
					TopicCode: topicCode,
				}
				buckets[key] = seed
			}
			seed.Count++
			if seed.SourceRef == "" {
				seed.SourceRef = strings.TrimSpace(entry.SourceRef)
			}
			seed.DominantPhase = pickTrainingFocusSignalDominantPhase(seed.DominantPhase, resolveArchiveLearningPhase(entry))
			seed.Suggestions = appendUniqueStrings(seed.Suggestions, suggestions...)
		}
	}
	return sortTrainingFocusSignalSeeds(buckets)
}

// sortTrainingFocusSignalSeeds 将聚合后的种子结果稳定排序
func sortTrainingFocusSignalSeeds(buckets map[string]*trainingFocusSignalSeed) []trainingFocusSignalSeed {
	items := make([]trainingFocusSignalSeed, 0, len(buckets))
	for _, seed := range buckets {
		items = append(items, trainingFocusSignalSeed{
			Tag:           strings.TrimSpace(seed.Tag),
			Count:         seed.Count,
			TopicCode:     strings.TrimSpace(seed.TopicCode),
			Suggestions:   sanitizeTextList(seed.Suggestions),
			SourceRef:     strings.TrimSpace(seed.SourceRef),
			DominantPhase: NormalizeLearningPhase(seed.DominantPhase),
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

// ensureTrainingFocusSignal 获取指定键对应的信号对象，不存在时会创建空壳
func ensureTrainingFocusSignal(merged map[string]*TrainingFocusSignal, key, tag, topicCode string) *TrainingFocusSignal {
	if signal, exists := merged[key]; exists {
		if signal.Tag == "" {
			signal.Tag = strings.TrimSpace(tag)
		}
		if signal.TopicCode == "" {
			signal.TopicCode = strings.TrimSpace(topicCode)
		}
		return signal
	}
	signal := &TrainingFocusSignal{
		Tag:       strings.TrimSpace(tag),
		TopicCode: strings.TrimSpace(topicCode),
	}
	merged[key] = signal
	return signal
}

// hydrateTrainingFocusSignal 补齐信号专题信息、推荐动作和可解释说明
func hydrateTrainingFocusSignal(signal TrainingFocusSignal) TrainingFocusSignal {
	if topic, ok := ResolveMistakeTopicByCode(signal.TopicCode); ok {
		signal.TopicCode = topic.Code
		signal.TopicTitle = topic.Title
		signal.TopicProblemPattern = topic.ProblemPattern
		signal.RelatedQuestionSets = sanitizeTextList(topic.RelatedQuestionSets)
		signal.RecommendedActions = appendUniqueStrings(topic.RecommendedActions, signal.RecommendedActions...)
	} else if topic, ok := ResolveMistakeTopicByTag(signal.Tag); ok {
		signal.TopicCode = topic.Code
		signal.TopicTitle = topic.Title
		signal.TopicProblemPattern = topic.ProblemPattern
		signal.RelatedQuestionSets = sanitizeTextList(topic.RelatedQuestionSets)
		signal.RecommendedActions = appendUniqueStrings(topic.RecommendedActions, signal.RecommendedActions...)
	}

	signal.RecommendedActions = trimSuggestions(signal.RecommendedActions)
	if len(signal.RelatedQuestionSets) > 0 {
		signal.PrimaryQuestionSet = signal.RelatedQuestionSets[0]
	}
	signal.DominantArchivePhase = NormalizeLearningPhase(signal.DominantArchivePhase)
	signal.DominantArchivePhaseLabel = buildTrainingFocusSignalPhaseLabel(signal.DominantArchivePhase)
	signal.Source, signal.SourceLabel = buildTrainingFocusSignalSource(signal)
	signal.Reason = buildTrainingFocusSignalReason(signal)
	return signal
}

// normalizeTrainingFocusSignals 统一清理训练重点信号中的字符串和切片字段
func normalizeTrainingFocusSignals(items []TrainingFocusSignal) []TrainingFocusSignal {
	result := make([]TrainingFocusSignal, 0, len(items))
	for _, item := range items {
		item.Tag = strings.TrimSpace(item.Tag)
		if item.Tag == "" {
			continue
		}
		item.TopicCode = strings.TrimSpace(item.TopicCode)
		item.TopicTitle = strings.TrimSpace(item.TopicTitle)
		item.TopicProblemPattern = strings.TrimSpace(item.TopicProblemPattern)
		item.PrimaryQuestionSet = strings.TrimSpace(item.PrimaryQuestionSet)
		item.DominantArchivePhase = NormalizeLearningPhase(item.DominantArchivePhase)
		item.DominantArchivePhaseLabel = strings.TrimSpace(item.DominantArchivePhaseLabel)
		item.Source = strings.TrimSpace(item.Source)
		item.SourceLabel = strings.TrimSpace(item.SourceLabel)
		item.Reason = strings.TrimSpace(item.Reason)
		item.SourceRef = strings.TrimSpace(item.SourceRef)
		item.RelatedQuestionSets = sanitizeTextList(item.RelatedQuestionSets)
		item.RecommendedActions = trimSuggestions(item.RecommendedActions)
		result = append(result, item)
	}
	return result
}

// buildTrainingFocusSignalKey 为训练重点信号生成稳定聚合键
func buildTrainingFocusSignalKey(tag, topicCode string) string {
	if trimmedTopicCode := strings.TrimSpace(topicCode); trimmedTopicCode != "" {
		return "topic:" + strings.ToLower(trimmedTopicCode)
	}
	return "tag:" + strings.ToLower(strings.TrimSpace(tag))
}

// pickTrainingFocusSignalSourceRef 优先保留已有来源引用，否则回填新的来源引用
func pickTrainingFocusSignalSourceRef(current, candidate string) string {
	current = strings.TrimSpace(current)
	if current != "" {
		return current
	}
	return strings.TrimSpace(candidate)
}

// buildTrainingFocusSignalSource 生成训练重点信号的来源标识与展示标签
func buildTrainingFocusSignalSource(signal TrainingFocusSignal) (string, string) {
	switch {
	case signal.ArchiveOccurrenceCount > 0 && signal.InterviewOccurrenceCount > 0:
		return "mixed", "练习 + 面试"
	case signal.InterviewOccurrenceCount > 0:
		return "interview_report", "面试报告"
	default:
		return "learning_archive", "练习归档"
	}
}

// buildTrainingFocusSignalReason 为统一信号生成可直接展示的解释文案
func buildTrainingFocusSignalReason(signal TrainingFocusSignal) string {
	switch signal.Source {
	case "mixed":
		if signal.DominantArchivePhaseLabel != "" {
			return fmt.Sprintf("最近练习里\"%s\"累计出现 %d 次，主要集中在%s，且最近 %d 场面试也反复暴露这个问题，适合本周优先补强。", signal.Tag, signal.ArchiveOccurrenceCount, signal.DominantArchivePhaseLabel, signal.InterviewOccurrenceCount)
		}
		return fmt.Sprintf("最近练习里\"%s\"累计出现 %d 次，且最近 %d 场面试也反复暴露这个问题，适合本周优先补强。", signal.Tag, signal.ArchiveOccurrenceCount, signal.InterviewOccurrenceCount)
	case "interview_report":
		return fmt.Sprintf("最近 %d 场已完成面试都指向\"%s\"，建议围绕这个薄弱点做一轮专项训练。", signal.InterviewOccurrenceCount, signal.Tag)
	default:
		if signal.DominantArchivePhaseLabel != "" {
			return fmt.Sprintf("最近练习归档里\"%s\"累计出现 %d 次，主要集中在%s，说明这个问题还在持续影响你的输出。", signal.Tag, signal.ArchiveOccurrenceCount, signal.DominantArchivePhaseLabel)
		}
		return fmt.Sprintf("最近练习归档里\"%s\"累计出现 %d 次，说明这个问题还在持续影响你的输出。", signal.Tag, signal.ArchiveOccurrenceCount)
	}
}

// resolveArchiveLearningPhase 为学习档案条目提取最贴近真实训练语义的阶段值
func resolveArchiveLearningPhase(entry *LearningArchiveEntry) string {
	for _, value := range []string{entry.TaskPhase} {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		return NormalizeLearningPhase(trimmed)
	}

	switch strings.TrimSpace(entry.SourceType) {
	case LearningArchiveSourceInterviewCoding:
		return LearningPhaseMock
	case LearningArchiveSourcePracticeQuestion:
		return LearningPhaseDrill
	default:
		return ""
	}
}

// pickTrainingFocusSignalDominantPhase 在归档聚合时保留最先出现的有效阶段值
func pickTrainingFocusSignalDominantPhase(current, candidate string) string {
	current = strings.TrimSpace(current)
	candidate = strings.TrimSpace(candidate)
	if current != "" {
		return NormalizeLearningPhase(current)
	}
	if candidate == "" {
		return ""
	}
	return NormalizeLearningPhase(candidate)
}

// buildTrainingFocusSignalPhaseLabel 将归档阶段枚举转换为中文阶段名称
func buildTrainingFocusSignalPhaseLabel(phase string) string {
	switch NormalizeLearningPhase(phase) {
	case LearningPhaseDrill:
		return "专项突破阶段"
	case LearningPhaseReview:
		return "复盘纠偏阶段"
	case LearningPhaseMock:
		return "模拟验证阶段"
	case LearningPhaseFoundation:
		return "打基础阶段"
	default:
		return ""
	}
}

// decodeJSONStringList 解析 JSON 字符串数组，并统一清理空值与重复项
func decodeJSONStringList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []string{}
	}
	return sanitizeTextList(values)
}

// sanitizeTextList 清理文本列表中的空值和重复项
func sanitizeTextList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// trimSuggestions 控制补强建议数量，避免单张卡片信息过载
func trimSuggestions(values []string) []string {
	suggestions := sanitizeTextList(values)
	if len(suggestions) > 5 {
		return suggestions[:5]
	}
	return suggestions
}

// appendUniqueStrings 向字符串切片追加去重后的非空值
func appendUniqueStrings(values []string, next ...string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values)+len(next))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	for _, value := range next {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
