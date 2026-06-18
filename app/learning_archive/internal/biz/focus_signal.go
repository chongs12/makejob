package biz

import (
	"fmt"
	"sort"
	"strings"
)

const defaultFocusSignalLimit = 3

// TrainingFocusSignal 表示可被成长页、推荐接口和学习计划共同复用的训练重点信号。
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
	CollectionHint            string   `json:"collection_hint,omitempty"`
}

// focusSignalSeed 表示构造训练重点信号时使用的中间聚合结果。
type focusSignalSeed struct {
	Tag            string
	Count          int
	TopicCode      string
	Suggestions    []string
	SourceRef      string
	DominantPhase  string
	ArchiveCount   int
	InterviewCount int
}

// BuildTrainingFocusSignals 将学习档案条目聚合为一组统一的训练重点信号。
// 对齐单体 buildTrainingFocusSignals 的完整逻辑。
func BuildTrainingFocusSignals(entries []*ArchiveEntry, limit int) []TrainingFocusSignal {
	if limit <= 0 {
		limit = defaultFocusSignalLimit
	}

	seeds := collectFocusSignalSeeds(entries)
	items := make([]TrainingFocusSignal, 0, len(seeds))
	for _, seed := range seeds {
		items = append(items, hydrateFocusSignal(seed))
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
	return normalizeFocusSignals(items)
}

// collectFocusSignalSeeds 从档案条目中按错因标签聚合统计。
func collectFocusSignalSeeds(entries []*ArchiveEntry) []focusSignalSeed {
	type seedAccum struct {
		tag            string
		count          int
		topicCode      string
		suggestions    []string
		sourceRef      string
		dominantPhase  string
		archiveCount   int
		interviewCount int
	}

	buckets := make(map[string]*seedAccum)
	for _, entry := range entries {
		tags := entry.MistakeTags
		suggestions := entry.Suggestions
		phase := resolveEntryLearningPhase(entry)
		isInterview := isInterviewSource(entry.SourceType)

		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			topicCode := ResolveMistakeTopicCodeByTag(tag)
			key := buildSignalKey(tag, topicCode)

			acc, exists := buckets[key]
			if !exists {
				acc = &seedAccum{
					tag:       tag,
					topicCode: topicCode,
				}
				buckets[key] = acc
			}
			acc.count++
			if isInterview {
				acc.interviewCount++
			} else {
				acc.archiveCount++
			}
			if acc.sourceRef == "" {
				acc.sourceRef = strings.TrimSpace(entry.SourceRef)
			}
			if acc.dominantPhase == "" && phase != "" {
				acc.dominantPhase = phase
			}
			acc.suggestions = appendUniqueStrings(acc.suggestions, suggestions...)
		}
	}

	result := make([]focusSignalSeed, 0, len(buckets))
	for _, acc := range buckets {
		result = append(result, focusSignalSeed{
			Tag:            acc.tag,
			Count:          acc.count,
			TopicCode:      acc.topicCode,
			Suggestions:    acc.suggestions,
			SourceRef:      acc.sourceRef,
			DominantPhase:  acc.dominantPhase,
			ArchiveCount:   acc.archiveCount,
			InterviewCount: acc.interviewCount,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Count == result[j].Count {
			return result[i].Tag < result[j].Tag
		}
		return result[i].Count > result[j].Count
	})
	return result
}

// hydrateFocusSignal 补齐信号专题信息、推荐动作和可解释说明。
func hydrateFocusSignal(seed focusSignalSeed) TrainingFocusSignal {
	signal := TrainingFocusSignal{
		Tag:                       seed.Tag,
		TopicCode:                 seed.TopicCode,
		OccurrenceCount:           seed.Count,
		ArchiveOccurrenceCount:    seed.ArchiveCount,
		InterviewOccurrenceCount:  seed.InterviewCount,
		DominantArchivePhase:      normalizePhase(seed.DominantPhase),
		RecommendedActions:        seed.Suggestions,
		SourceRef:                 seed.SourceRef,
	}

	// 先按 code 查找，找不到则按 tag 回退
	if topic, ok := ResolveMistakeTopicByCode(seed.TopicCode); ok {
		signal.TopicCode = topic.Code
		signal.TopicTitle = topic.Title
		signal.TopicProblemPattern = topic.ProblemPattern
		signal.RelatedQuestionSets = topic.RelatedQuestionSets
		signal.RecommendedActions = appendUniqueStrings(topic.RecommendedActions, signal.RecommendedActions...)
	} else if topic, ok := ResolveMistakeTopicByTag(seed.Tag); ok {
		signal.TopicCode = topic.Code
		signal.TopicTitle = topic.Title
		signal.TopicProblemPattern = topic.ProblemPattern
		signal.RelatedQuestionSets = topic.RelatedQuestionSets
		signal.RecommendedActions = appendUniqueStrings(topic.RecommendedActions, signal.RecommendedActions...)
	}

	if len(signal.RelatedQuestionSets) > 0 {
		signal.PrimaryQuestionSet = signal.RelatedQuestionSets[0]
	}
	signal.CollectionHint = signal.PrimaryQuestionSet
	signal.RecommendedActions = trimSuggestions(signal.RecommendedActions, 2)

	signal.DominantArchivePhaseLabel = buildPhaseLabel(signal.DominantArchivePhase)
	signal.Source, signal.SourceLabel = buildSignalSource(signal)
	signal.Reason = buildSignalReason(signal)
	return signal
}

// normalizeFocusSignals 统一清理训练重点信号中的字符串字段。
func normalizeFocusSignals(items []TrainingFocusSignal) []TrainingFocusSignal {
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
		item.CollectionHint = strings.TrimSpace(item.CollectionHint)
		item.DominantArchivePhase = normalizePhase(item.DominantArchivePhase)
		item.DominantArchivePhaseLabel = strings.TrimSpace(item.DominantArchivePhaseLabel)
		item.Source = strings.TrimSpace(item.Source)
		item.SourceLabel = strings.TrimSpace(item.SourceLabel)
		item.Reason = strings.TrimSpace(item.Reason)
		item.SourceRef = strings.TrimSpace(item.SourceRef)
		item.RecommendedActions = trimSuggestions(item.RecommendedActions, 2)
		result = append(result, item)
	}
	return result
}

// buildSignalKey 为训练重点信号生成稳定聚合键。
func buildSignalKey(tag, topicCode string) string {
	if tc := strings.TrimSpace(topicCode); tc != "" {
		return "topic:" + strings.ToLower(tc)
	}
	return "tag:" + strings.ToLower(strings.TrimSpace(tag))
}

// resolveEntryLearningPhase 为学习档案条目提取最贴近真实训练语义的阶段值。
func resolveEntryLearningPhase(entry *ArchiveEntry) string {
	for _, value := range []string{entry.TaskPhase, entry.PlanPhase, entry.EntryPhase} {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return normalizePhase(trimmed)
		}
	}
	switch strings.TrimSpace(entry.SourceType) {
	case "interview_coding", "interview_weak", "interview_strength":
		return "mock"
	case "practice_question":
		return "drill"
	default:
		return ""
	}
}

// isInterviewSource 判断来源类型是否来自面试。
func isInterviewSource(sourceType string) bool {
	switch strings.TrimSpace(sourceType) {
	case "interview_coding", "interview_weak", "interview_strength":
		return true
	default:
		return false
	}
}

// normalizePhase 将阶段值标准化为小写枚举。
func normalizePhase(phase string) string {
	return strings.ToLower(strings.TrimSpace(phase))
}

// buildPhaseLabel 将归档阶段枚举转换为中文阶段名称。
func buildPhaseLabel(phase string) string {
	switch normalizePhase(phase) {
	case "drill":
		return "专项突破阶段"
	case "review":
		return "复盘纠偏阶段"
	case "mock":
		return "模拟验证阶段"
	case "foundation":
		return "打基础阶段"
	default:
		return ""
	}
}

// buildSignalSource 生成训练重点信号的来源标识与展示标签。
func buildSignalSource(signal TrainingFocusSignal) (string, string) {
	switch {
	case signal.ArchiveOccurrenceCount > 0 && signal.InterviewOccurrenceCount > 0:
		return "mixed", "练习 + 面试"
	case signal.InterviewOccurrenceCount > 0:
		return "interview_report", "面试报告"
	default:
		return "learning_archive", "练习归档"
	}
}

// buildSignalReason 为统一信号生成可直接展示的解释文案。
func buildSignalReason(signal TrainingFocusSignal) string {
	switch signal.Source {
	case "mixed":
		if signal.DominantArchivePhaseLabel != "" {
			return fmt.Sprintf("最近练习里\"%s\"累计出现 %d 次，主要集中在%s，且最近 %d 场面试也反复暴露这个问题，适合优先补强。", signal.Tag, signal.ArchiveOccurrenceCount, signal.DominantArchivePhaseLabel, signal.InterviewOccurrenceCount)
		}
		return fmt.Sprintf("最近练习里\"%s\"累计出现 %d 次，且最近 %d 场面试也反复暴露这个问题，适合优先补强。", signal.Tag, signal.ArchiveOccurrenceCount, signal.InterviewOccurrenceCount)
	case "interview_report":
		return fmt.Sprintf("最近 %d 场已完成面试都指向\"%s\"，建议围绕这个薄弱点做一轮专项训练。", signal.InterviewOccurrenceCount, signal.Tag)
	default:
		if signal.DominantArchivePhaseLabel != "" {
			return fmt.Sprintf("最近练习归档里\"%s\"累计出现 %d 次，主要集中在%s，说明这个问题还在持续影响你的输出。", signal.Tag, signal.ArchiveOccurrenceCount, signal.DominantArchivePhaseLabel)
		}
		return fmt.Sprintf("最近练习归档里\"%s\"累计出现 %d 次，说明这个问题还在持续影响你的输出。", signal.Tag, signal.ArchiveOccurrenceCount)
	}
}

// trimSuggestions 将建议列表截断到指定上限。
func trimSuggestions(suggestions []string, limit int) []string {
	if limit <= 0 || len(suggestions) <= limit {
		return suggestions
	}
	return suggestions[:limit]
}

// appendUniqueStrings 追加去重字符串切片。
func appendUniqueStrings(existing []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(existing))
	for _, s := range existing {
		seen[strings.TrimSpace(s)] = struct{}{}
	}
	for _, s := range additions {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		existing = append(existing, s)
	}
	return existing
}
