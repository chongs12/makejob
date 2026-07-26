package biz

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// localInterviewQuestionTemplates 模型不可用时的兜底题目模板。
var localInterviewQuestionTemplates = []string{
	"请解释 %s 的核心概念、常见使用场景以及容易踩坑的地方。",
	"如果你要向初级同学讲清楚 %s，你会怎样从原理、实现和实践三个层面展开？",
	"请结合一个真实项目场景，说明 %s 为什么重要，以及你会如何落地。",
	"针对 %s，请先给出结论，再补充边界情况、性能影响和调试思路。",
}

// buildLocalQuestionResult 构造本地兜底题目（LLM 不可用或返回空题目时使用）。
// 优先用用户自定义知识点，否则按行业取默认主题；按题号轮换模板，避免连续多题重复。
func buildLocalQuestionResult(topics []string, industryCode string, questionIndex int32) *InterviewResult {
	topic := ""
	if len(topics) > 0 {
		topic = strings.TrimSpace(topics[0])
	}
	if topic == "" {
		topic = defaultTopicsByIndustry(industryCode)[0]
	}
	tpl := localInterviewQuestionTemplates[int(questionIndex)%len(localInterviewQuestionTemplates)]
	return &InterviewResult{
		Question:   fmt.Sprintf(tpl, topic),
		Topic:      topic,
		Difficulty: "medium",
		Type:       "technical",
		Hints:      "请从定义、原理、使用场景和常见问题四个角度组织回答。",
	}
}

// buildLocalStartResponse 在 LLM 出题失败时返回模板题目。
func buildLocalStartResponse(req *StartInterviewRequest) *StartInterviewResponse {
	r := buildLocalQuestionResult(req.Topics, req.IndustryCode, 0)
	return &StartInterviewResponse{
		SessionID:  uuid.NewString(),
		Question:   r.Question,
		Topic:      r.Topic,
		Difficulty: r.Difficulty,
		Type:       r.Type,
		Hints:      r.Hints,
	}
}

// buildLocalEvaluateResponse 在 LLM 评分失败时基于回答长度和关键词评分。
func buildLocalEvaluateResponse(answer string) *EvaluateAnswerResponse {
	answer = strings.TrimSpace(answer)
	score := 55.0

	switch {
	case len([]rune(answer)) >= 180:
		score = 88
	case len([]rune(answer)) >= 100:
		score = 78
	case len([]rune(answer)) >= 40:
		score = 68
	}

	for _, keyword := range []string{"例如", "example", "原理", "why"} {
		if strings.Contains(strings.ToLower(answer), strings.ToLower(keyword)) {
			score += 4
		}
	}
	if score > 100 {
		score = 100
	}

	return &EvaluateAnswerResponse{
		Score:      score,
		IsCorrect:  score >= 60,
		Feedback:   "基于回答完整度的兜底评估。回答覆盖了相关内容，但还需要进一步突出核心原理和工程场景。",
		KeyPoints:  []string{"核心概念", "原理说明", "使用场景", "边界情况"},
		Suggestions: "建议按「概念定义 → 实现原理 → 使用场景 → 常见坑点」的顺序组织答案。",
	}
}

// buildLocalKnowledgeReport 在 LLM 报告生成全部失败时，基于对话历史生成一份基础结构化报告，
// 保证不落 report_failed。报告内容偏模板化，仅作兜底。
func buildLocalKnowledgeReport(history []Message, topics []string, totalQuestions int32) KnowledgeReportResult {
	type qaPair struct{ question, answer string }
	var pairs []qaPair
	var currentQuestion string
	for _, msg := range history {
		if msg.Role == "assistant" {
			currentQuestion = msg.Content
		} else if msg.Role == "user" && currentQuestion != "" {
			pairs = append(pairs, qaPair{currentQuestion, msg.Content})
			currentQuestion = ""
		}
	}

	answered := int32(len(pairs))
	var reviews []KnowledgeQuestionReview
	for i, pair := range pairs {
		reviews = append(reviews, KnowledgeQuestionReview{
			QuestionIndex:  int32(i),
			Question:       pair.question,
			UserAnswer:     pair.answer,
			Score:          70,
			MaxScore:       100,
			Errors:         []string{},
			Omissions:      []string{},
			Highlights:     []string{},
			StandardAnswer: "",
			KeyPoints:      []string{},
		})
	}

	return KnowledgeReportResult{
		OverallScore: 70,
		Rating:       "合格",
		Conclusion:   fmt.Sprintf("本场面试共回答 %d 题（LLM 报告生成不可用，此为本地兜底报告，建议稍后重试生成）。", answered),
		BasicInfo: KnowledgeReportBasicInfo{
			KnowledgeTopics: topics,
			QuestionType:    "问答",
			DurationSeconds: 0,
			TotalQuestions:  totalQuestions,
			CorrectCount:    0,
			Accuracy:        0,
		},
		QuestionReviews: reviews,
		DimensionScores: []KnowledgeDimensionScore{
			{Dimension: "知识点基础掌握度", Score: 70, Comment: "本地兜底评估，未深入分析。"},
			{Dimension: "知识点应用落地能力", Score: 70, Comment: "本地兜底评估，未深入分析。"},
			{Dimension: "知识延伸与深度", Score: 70, Comment: "本地兜底评估，未深入分析。"},
			{Dimension: "答题精准度与严谨度", Score: 70, Comment: "本地兜底评估，未深入分析。"},
		},
		MasteredPoints:   []string{},
		BlindSpots:       []KnowledgeBlindSpot{},
		StudySuggestions: []KnowledgeStudySuggestion{{Focus: "巩固基础", Detail: "建议针对本场涉及的知识点重新梳理。"}},
		NextQuizTopics:   []KnowledgeNextQuizTopic{},
	}
}

// buildLocalReportResponse 在 LLM 报告生成失败时基于已有的 Feedbacks 聚合报告。
func buildLocalReportResponse(session *interviewSessionState) *GenerateInterviewReportResponse {
	dimensionScores := make(map[string]float64)
	dimensionCounts := make(map[string]int)
	var totalScore float64
	var answered int

	for _, fb := range session.Feedbacks {
		totalScore += fb.Score
		answered++
		topic := "综合能力"
		if int(fb.Index) < len(session.Questions) {
			if t := strings.TrimSpace(session.Questions[fb.Index].Topic); t != "" {
				topic = t
			}
		}
		dimensionScores[topic] += fb.Score
		dimensionCounts[topic]++
	}

	for topic, sum := range dimensionScores {
		if dimensionCounts[topic] > 0 {
			dimensionScores[topic] = sum / float64(dimensionCounts[topic])
		}
	}

	overallScore := 0.0
	if answered > 0 {
		overallScore = totalScore / float64(answered)
	}

	strengths, weaknesses := summarizeByScore(dimensionScores)
	return &GenerateInterviewReportResponse{
		OverallScore:    overallScore,
		Summary:         fmt.Sprintf("本次面试共回答 %d 题，综合得分 %.0f 分。", answered, overallScore),
		DimensionScores: dimensionScores,
		Strengths:       strengths,
		Weaknesses:      weaknesses,
		Suggestions:     []string{"优先补强低分主题", "每道题回答时加入真实项目例子", "复盘回答结构"},
	}
}

// defaultTopicsByIndustry 根据行业返回默认主题。
func defaultTopicsByIndustry(industryCode string) []string {
	switch strings.ToLower(strings.TrimSpace(industryCode)) {
	case "frontend":
		return []string{"JavaScript", "浏览器原理", "性能优化", "工程化"}
	case "java":
		return []string{"JVM", "并发编程", "集合框架", "Spring"}
	case "python":
		return []string{"语言特性", "并发模型", "Web 开发", "性能优化"}
	default:
		return []string{"基础语法", "并发编程", "工程实践", "性能优化"}
	}
}

// summarizeByScore 根据维度分数总结优劣势。
func summarizeByScore(dimensionScores map[string]float64) ([]string, []string) {
	var strengths []string
	var weaknesses []string
	for topic, score := range dimensionScores {
		switch {
		case score >= 80:
			strengths = append(strengths, fmt.Sprintf("%s 表现较强（%.0f 分）", topic, score))
		case score < 65:
			weaknesses = append(weaknesses, fmt.Sprintf("%s 仍需加强（%.0f 分）", topic, score))
		}
	}
	if len(strengths) == 0 {
		strengths = append(strengths, "回答态度稳定，具备继续提升的基础。")
	}
	if len(weaknesses) == 0 {
		weaknesses = append(weaknesses, "整体表现较均衡，建议继续挑战更深层问题。")
	}
	return strengths, weaknesses
}
