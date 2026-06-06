package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// questionAnswerPair 表示一题及其对应的用户回答。
type questionAnswerPair struct {
	Index    int32
	Question *InterviewQuestion
	Answer   *InterviewMessage
}

// answerEvaluation 表示单题评估结果，用于汇总面试报告。
type answerEvaluation struct {
	Index       int32
	Question    *InterviewQuestion
	Score       float64
	IsCorrect   bool
	Feedback    string
	KeyPoints   []string
	Suggestions []string
}

// BuildQuestionAnswerPairs 按题号提取题目与用户回答配对。
func BuildQuestionAnswerPairs(messages []*InterviewMessage) []questionAnswerPair {
	questions := make(map[int32]*InterviewQuestion)
	answers := make(map[int32]*InterviewMessage)
	indexes := make(map[int32]struct{})
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		indexes[msg.QuestionIndex] = struct{}{}
		switch msg.Role {
		case "assistant":
			if _, exists := questions[msg.QuestionIndex]; !exists {
				questions[msg.QuestionIndex] = DecodeQuestionContent(msg.Content)
			}
		case "user":
			if _, exists := answers[msg.QuestionIndex]; !exists && strings.TrimSpace(msg.Content) != "" {
				answers[msg.QuestionIndex] = msg
			}
		}
	}

	ordered := make([]int, 0, len(indexes))
	for index := range indexes {
		ordered = append(ordered, int(index))
	}
	sort.Ints(ordered)

	pairs := make([]questionAnswerPair, 0, len(ordered))
	for _, index := range ordered {
		pairs = append(pairs, questionAnswerPair{
			Index:    int32(index),
			Question: questions[int32(index)],
			Answer:   answers[int32(index)],
		})
	}
	return pairs
}

// BuildTopic 从题目中提取报告聚合时使用的主题标签。
func BuildTopic(question *InterviewQuestion) string {
	if question == nil {
		return "综合能力"
	}
	if topic := strings.TrimSpace(question.Topic); topic != "" {
		return topic
	}
	if question.Type == "coding" {
		return "编程实现"
	}
	return "综合能力"
}

// EstimateAnswerScore 在 AI 评估不可用时，给文本回答生成保底分数。
func EstimateAnswerScore(answer string) float64 {
	text := strings.TrimSpace(answer)
	switch {
	case text == "":
		return 0
	case len([]rune(text)) < 20:
		return 45
	case len([]rune(text)) < 80:
		return 62
	case len([]rune(text)) < 200:
		return 75
	default:
		return 84
	}
}

// EvaluateAnswer 使用 AI 或本地规则评估一题文本回答。
func (uc *InterviewUseCase) EvaluateAnswer(ctx context.Context, interview *Interview, pair questionAnswerPair) answerEvaluation {
	evaluation := answerEvaluation{
		Index:    pair.Index,
		Question: pair.Question,
	}
	if pair.Answer == nil {
		evaluation.Feedback = "该题缺少有效回答。"
		return evaluation
	}
	if uc.ai != nil && pair.Question != nil {
		resp, err := uc.ai.QuizAnalyzer(ctx, &QuizAnalyzerRequest{
			Question:   pair.Question.Question,
			Answer:     pair.Answer.Content,
			Topic:      BuildTopic(pair.Question),
			Difficulty: interview.Difficulty,
		})
		if err == nil && resp != nil {
			evaluation.Score = resp.Score
			evaluation.IsCorrect = resp.IsCorrect
			evaluation.Feedback = strings.TrimSpace(resp.Feedback)
			evaluation.KeyPoints = append([]string(nil), resp.KeyPoints...)
			if suggestion := strings.TrimSpace(resp.Suggestions); suggestion != "" {
				evaluation.Suggestions = []string{suggestion}
			}
			return evaluation
		}
	}

	evaluation.Score = EstimateAnswerScore(pair.Answer.Content)
	evaluation.IsCorrect = evaluation.Score >= 60
	evaluation.Feedback = fmt.Sprintf("基于回答完整度的兜底评估，当前得分约 %.0f 分。", evaluation.Score)
	if evaluation.IsCorrect {
		evaluation.KeyPoints = []string{BuildTopic(pair.Question)}
	} else {
		evaluation.Suggestions = []string{"补充结构化表达，并结合更具体的技术细节回答问题。"}
	}
	return evaluation
}

// BuildCodingDiagnostics 为编程题尝试生成结构化诊断。
func (uc *InterviewUseCase) BuildCodingDiagnostics(ctx context.Context, interview *Interview, pairs []questionAnswerPair, attempts []*CodingAttempt) []*CodingDiagnosisBiz {
	pairMap := make(map[int32]questionAnswerPair, len(pairs))
	for _, pair := range pairs {
		pairMap[pair.Index] = pair
	}

	diagnostics := make([]*CodingDiagnosisBiz, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt == nil {
			continue
		}
		pair := pairMap[attempt.QuestionIndex]
		question := pair.Question
		score := attempt.AIScore
		feedback := strings.TrimSpace(attempt.AIFeedback)
		suggestions := []string{}
		if score <= 0 && uc.ai != nil {
			resp, err := uc.ai.QuizAnalyzer(ctx, &QuizAnalyzerRequest{
				Question:   NormalizeQuestionText(question),
				Answer:     attempt.Code,
				Topic:      BuildTopic(question),
				Difficulty: interview.Difficulty,
			})
			if err == nil && resp != nil {
				score = resp.Score
				feedback = strings.TrimSpace(resp.Feedback)
				attempt.AIScore = resp.Score
				attempt.AIFeedback = feedback
				if suggestion := strings.TrimSpace(resp.Suggestions); suggestion != "" {
					suggestions = append(suggestions, suggestion)
				}
				_ = uc.repo.UpdateCodingAttempt(ctx, attempt)
			}
		}
		if score <= 0 {
			if attempt.Passed {
				score = 78
			} else {
				score = 52
			}
		}
		if feedback == "" {
			if attempt.Passed {
				feedback = "代码可以通过当前执行校验，建议继续优化可读性和边界处理。"
			} else if strings.TrimSpace(attempt.ErrorMsg) != "" {
				feedback = "代码执行未通过，需优先修复编译或运行错误。"
			} else {
				feedback = "代码实现尚未完全通过校验，建议补充更多自测。"
			}
		}
		if len(suggestions) == 0 {
			if attempt.Passed {
				suggestions = []string{"补充极端输入和复杂度分析，进一步提升稳定性。"}
			} else {
				suggestions = []string{"优先根据报错信息修复主流程，再补充边界测试。"}
			}
		}

		topic := BuildTopic(question)
		mistakeTags := []string{}
		strengthTags := []string{}
		if score < 60 {
			mistakeTags = append(mistakeTags, topic)
		}
		if score >= 75 {
			strengthTags = append(strengthTags, topic)
		}

		diagnostics = append(diagnostics, &CodingDiagnosisBiz{
			QuestionIndex:   attempt.QuestionIndex,
			Language:        firstNonEmptyString(attempt.Language, NormalizeQuestionLanguage(question)),
			Topic:           topic,
			Score:           score,
			MistakeTags:     mistakeTags,
			StrengthTags:    strengthTags,
			EvidenceSummary: feedback,
			Suggestions:     suggestions,
		})
	}
	return diagnostics
}

// NormalizeQuestionText 返回题目的纯文本内容。
func NormalizeQuestionText(question *InterviewQuestion) string {
	if question == nil {
		return ""
	}
	return strings.TrimSpace(question.Question)
}

// NormalizeQuestionLanguage 返回题目的语言标签。
func NormalizeQuestionLanguage(question *InterviewQuestion) string {
	if question == nil {
		return ""
	}
	return strings.TrimSpace(question.Language)
}

// firstNonEmptyString 返回第一个非空字符串。
func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// appendUniqueStrings 将非空字符串追加到切片中并去重。
func appendUniqueStrings(target []string, values ...string) []string {
	seen := make(map[string]struct{}, len(target))
	for _, value := range target {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			seen[trimmed] = struct{}{}
		}
	}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		target = append(target, trimmed)
		seen[trimmed] = struct{}{}
	}
	return target
}

// decodeCodingDiagnostics 解析报告中的编程诊断 JSON 字段。
func decodeCodingDiagnostics(raw string) []*CodingDiagnosisBiz {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var diagnostics []*CodingDiagnosisBiz
	if err := json.Unmarshal([]byte(raw), &diagnostics); err != nil {
		return nil
	}
	return diagnostics
}

// uniqueNonEmptyStrings 对字符串切片去重并过滤空值。
func uniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
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
	return result
}

// finalizeDimensionScores 将主题累计分转换为平均分结果。
func finalizeDimensionScores(scoreSum map[string]float64, scoreCount map[string]int) map[string]float64 {
	result := make(map[string]float64, len(scoreSum))
	for topic, total := range scoreSum {
		if scoreCount[topic] <= 0 {
			continue
		}
		result[topic] = total / float64(scoreCount[topic])
	}
	return result
}

// buildReportSummary 生成面试报告摘要，概括总体分数与主要优劣势。
func buildReportSummary(overallScore float64, strengths, weaknesses []string, codingDiagnostics []*CodingDiagnosisBiz) string {
	strengthText := "暂无明显优势"
	if len(strengths) > 0 {
		strengthText = strings.Join(strengths, "、")
	}
	weaknessText := "暂无明显短板"
	if len(weaknesses) > 0 {
		weaknessText = strings.Join(weaknesses, "、")
	}
	codingText := ""
	if len(codingDiagnostics) > 0 {
		codingText = fmt.Sprintf("编程题覆盖 %d 道。", len(codingDiagnostics))
	}
	return fmt.Sprintf("本次面试综合得分 %.1f 分。优势集中在：%s。待加强项包括：%s。%s", overallScore, strengthText, weaknessText, codingText)
}
