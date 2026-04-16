package mock

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"makejob-backend/internal/ai"

	"github.com/google/uuid"
)

// 预设的Go语言面试题库
var goInterviewQuestions = []ai.InterviewQuestion{
	{
		Question:   "请解释Go语言中的Goroutine是什么，以及它与线程的区别？",
		Topic:      "并发编程",
		Difficulty: "medium",
		Type:       "technical",
		Hints:      "可以从调度方式、内存占用、创建成本等角度思考",
	},
	{
		Question:   "Go语言的Channel有什么作用？有缓冲Channel和无缓冲Channel的区别是什么？",
		Topic:      "并发编程",
		Difficulty: "medium",
		Type:       "technical",
		Hints:      "考虑通信同步机制和阻塞行为",
	},
	{
		Question:   "请解释Go语言中的Interface是如何实现的？什么是鸭子类型？",
		Topic:      "语言特性",
		Difficulty: "medium",
		Type:       "technical",
		Hints:      "关注隐式实现和运行时类型检查",
	},
	{
		Question:   "Go语言的GMP模型是什么？请详细描述调度器的工作原理。",
		Topic:      "运行时机制",
		Difficulty: "hard",
		Type:       "technical",
		Hints:      "G=Goroutine, M=Machine, P=Processor",
	},
	{
		Question:   "Go语言的垃圾回收机制是怎样的？有哪些优化手段？",
		Topic:      "内存管理",
		Difficulty: "hard",
		Type:       "technical",
		Hints:      "三色标记法、写屏障、并发回收",
	},
	{
		Question:   "请解释Go语言中的Slice和Array的区别，以及Slice的底层实现。",
		Topic:      "数据结构",
		Difficulty: "easy",
		Type:       "technical",
		Hints:      "关注长度、容量、底层数组指针",
	},
	{
		Question:   "Go语言中的Map是如何实现的？扩容机制是什么？",
		Topic:      "数据结构",
		Difficulty: "hard",
		Type:       "technical",
		Hints:      "哈希表、桶、渐进式扩容",
	},
	{
		Question:   "请解释defer语句的执行顺序和使用场景。",
		Topic:      "语言特性",
		Difficulty: "easy",
		Type:       "technical",
		Hints:      "LIFO顺序、资源释放、 panic恢复",
	},
	{
		Question:   "Go语言中的Context包有什么作用？有哪些使用场景？",
		Topic:      "标准库",
		Difficulty: "medium",
		Type:       "technical",
		Hints:      "超时控制、取消信号、传递元数据",
	},
	{
		Question:   "请实现一个线程安全的单例模式（Singleton）。",
		Topic:      "设计模式",
		Difficulty: "medium",
		Type:       "coding",
		Hints:      "考虑sync.Once和原子操作",
	},
}

// MockInterviewAgent Mock面试Agent实现
type MockInterviewAgent struct {
	provider *MockProvider
	sessions *sync.Map // 存储面试会话状态
}

// NewMockInterviewAgent 创建Mock面试Agent
func NewMockInterviewAgent(provider *MockProvider) *MockInterviewAgent {
	return &MockInterviewAgent{
		provider: provider,
		sessions: &sync.Map{},
	}
}

// StartInterview 开始面试
func (a *MockInterviewAgent) StartInterview(ctx context.Context, config ai.InterviewConfig) (string, ai.InterviewQuestion, error) {
	select {
	case <-ctx.Done():
		return "", ai.InterviewQuestion{}, ctx.Err()
	default:
	}

	// 生成会话ID
	sessionID := uuid.New().String()

	// 根据配置选择题目
	questions := a.selectQuestions(config)
	if len(questions) == 0 {
		return "", ai.InterviewQuestion{}, fmt.Errorf("no questions available for the given config")
	}

	// 创建会话状态
	session := &ai.InterviewSession{
		SessionID:    sessionID,
		Config:       config,
		CurrentIndex: 0,
		Questions:    questions,
		Answers:      make([]string, len(questions)),
		Feedbacks:    make([]ai.AnswerFeedback, len(questions)),
		StartTime:    time.Now().Unix(),
		IsActive:     true,
	}

	a.sessions.Store(sessionID, session)

	return sessionID, questions[0], nil
}

// EvaluateAnswer 评估答案
func (a *MockInterviewAgent) EvaluateAnswer(ctx context.Context, sessionID string, questionIndex int, answer string) (ai.AnswerFeedback, error) {
	select {
	case <-ctx.Done():
		return ai.AnswerFeedback{}, ctx.Err()
	default:
	}

	sessionVal, ok := a.sessions.Load(sessionID)
	if !ok {
		return ai.AnswerFeedback{}, fmt.Errorf("session not found: %s", sessionID)
	}

	session := sessionVal.(*ai.InterviewSession)
	if !session.IsActive {
		return ai.AnswerFeedback{}, fmt.Errorf("session is not active")
	}

	if questionIndex < 0 || questionIndex >= len(session.Questions) {
		return ai.AnswerFeedback{}, fmt.Errorf("invalid question index: %d", questionIndex)
	}

	// 保存答案
	session.Answers[questionIndex] = answer

	// 生成反馈（基于答案长度和质量模拟评分）
	feedback := a.generateFeedback(session.Questions[questionIndex], answer)
	session.Feedbacks[questionIndex] = feedback

	return feedback, nil
}

// GetNextQuestion 获取下一题
func (a *MockInterviewAgent) GetNextQuestion(ctx context.Context, sessionID string) (ai.InterviewQuestion, bool, error) {
	select {
	case <-ctx.Done():
		return ai.InterviewQuestion{}, false, ctx.Err()
	default:
	}

	sessionVal, ok := a.sessions.Load(sessionID)
	if !ok {
		return ai.InterviewQuestion{}, false, fmt.Errorf("session not found: %s", sessionID)
	}

	session := sessionVal.(*ai.InterviewSession)
	if !session.IsActive {
		return ai.InterviewQuestion{}, false, fmt.Errorf("session is not active")
	}

	nextIndex := session.CurrentIndex + 1
	if nextIndex >= len(session.Questions) {
		return ai.InterviewQuestion{}, false, nil // 没有更多题目了
	}

	session.CurrentIndex = nextIndex
	return session.Questions[nextIndex], true, nil
}

// GenerateReport 生成面试报告
func (a *MockInterviewAgent) GenerateReport(ctx context.Context, sessionID string) (ai.InterviewReport, error) {
	select {
	case <-ctx.Done():
		return ai.InterviewReport{}, ctx.Err()
	default:
	}

	sessionVal, ok := a.sessions.Load(sessionID)
	if !ok {
		return ai.InterviewReport{}, fmt.Errorf("session not found: %s", sessionID)
	}

	session := sessionVal.(*ai.InterviewSession)

	// 计算各维度评分
	dimensionScores := make(map[string]float64)
	dimensionCounts := make(map[string]int)

	var totalScore float64
	var correctCount int

	for i, feedback := range session.Feedbacks {
		if i < len(session.Answers) && session.Answers[i] != "" {
			totalScore += feedback.Score
			if feedback.IsCorrect {
				correctCount++
			}

			topic := session.Questions[i].Topic
			dimensionScores[topic] += feedback.Score
			dimensionCounts[topic]++
		}
	}

	// 计算平均分
	answeredCount := 0
	for _, answer := range session.Answers {
		if answer != "" {
			answeredCount++
		}
	}

	var overallScore float64
	if answeredCount > 0 {
		overallScore = totalScore / float64(answeredCount)
	}

	for topic := range dimensionScores {
		if dimensionCounts[topic] > 0 {
			dimensionScores[topic] = dimensionScores[topic] / float64(dimensionCounts[topic])
		}
	}

	// 生成优劣势分析
	strengths, weaknesses := a.analyzeStrengthsWeaknesses(dimensionScores)

	return ai.InterviewReport{
		OverallScore:    overallScore,
		TotalQuestions:  len(session.Questions),
		CorrectCount:    correctCount,
		DimensionScores: dimensionScores,
		Strengths:       strengths,
		Weaknesses:      weaknesses,
		Suggestions:     a.generateSuggestions(weaknesses),
		Summary:         a.generateSummary(overallScore, answeredCount, len(session.Questions)),
	}, nil
}

// EndInterview 结束面试
func (a *MockInterviewAgent) EndInterview(ctx context.Context, sessionID string) error {
	sessionVal, ok := a.sessions.Load(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session := sessionVal.(*ai.InterviewSession)
	session.IsActive = false

	// 可以选择在这里删除会话，或者保留用于后续查询
	// a.sessions.Delete(sessionID)

	return nil
}

// selectQuestions 根据配置选择题目
func (a *MockInterviewAgent) selectQuestions(config ai.InterviewConfig) []ai.InterviewQuestion {
	var filtered []ai.InterviewQuestion

	// 根据难度和主题过滤
	for _, q := range goInterviewQuestions {
		if config.Difficulty != "mixed" && q.Difficulty != config.Difficulty {
			continue
		}
		if len(config.Topics) > 0 && !containsTopic(config.Topics, q.Topic) {
			continue
		}
		filtered = append(filtered, q)
	}

	// 如果没有匹配的题目，返回所有题目
	if len(filtered) == 0 {
		filtered = goInterviewQuestions
	}

	// 随机打乱顺序
	rand.Shuffle(len(filtered), func(i, j int) {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	})

	// 限制题目数量
	count := config.QuestionCount
	if count <= 0 || count > len(filtered) {
		count = len(filtered)
	}

	return filtered[:count]
}

// generateFeedback 生成答案反馈
func (a *MockInterviewAgent) generateFeedback(question ai.InterviewQuestion, answer string) ai.AnswerFeedback {
	// 基于答案长度和质量模拟评分
	score := 70.0 // 基础分

	// 根据答案长度调整分数
	answerLen := len(answer)
	if answerLen > 200 {
		score += 15
	} else if answerLen > 100 {
		score += 10
	} else if answerLen < 20 {
		score -= 20
	}

	// 添加随机波动
	score += float64(rand.Intn(11) - 5)

	// 限制分数范围
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	isCorrect := score >= 60

	// 根据题目生成关键知识点
	keyPoints := a.getKeyPointsForQuestion(question)

	// 根据分数生成反馈
	var feedback, suggestions string
	if score >= 85 {
		feedback = "回答非常出色！你展现了扎实的知识功底和清晰的表达能力。"
		suggestions = "继续保持，可以尝试深入了解更多底层原理。"
	} else if score >= 70 {
		feedback = "回答良好，涵盖了主要知识点，但还有一些可以补充的地方。"
		suggestions = "建议多关注细节，并尝试结合实际项目经验来回答问题。"
	} else if score >= 60 {
		feedback = "回答基本正确，但不够完整，缺少一些重要的知识点。"
		suggestions = "建议系统性地复习相关知识点，并多做练习巩固。"
	} else {
		feedback = "回答不够准确，可能存在概念混淆或理解偏差。"
		suggestions = "建议重新学习相关基础知识，可以参考官方文档和经典教材。"
	}

	return ai.AnswerFeedback{
		Score:       score,
		IsCorrect:   isCorrect,
		Feedback:    feedback,
		KeyPoints:   keyPoints,
		Suggestions: suggestions,
		FollowUp:    "",
	}
}

// getKeyPointsForQuestion 获取题目的关键知识点
func (a *MockInterviewAgent) getKeyPointsForQuestion(question ai.InterviewQuestion) []string {
	keyPointsMap := map[string][]string{
		"并发编程":  {"Goroutine轻量级线程", "Go运行时调度", "Channel通信机制", "同步原语"},
		"语言特性":  {"隐式接口实现", "鸭子类型", "类型断言", "反射机制"},
		"运行时机制": {"GMP调度模型", "工作窃取算法", "系统调用处理", "网络轮询器"},
		"内存管理":  {"三色标记法", "写屏障", "并发GC", "内存分配器"},
		"数据结构":  {"底层数组", "哈希表实现", "桶结构", "扩容机制"},
		"标准库":   {"超时控制", "取消信号", "值传递", "链式调用"},
		"设计模式":  {"单例模式", "懒加载", "线程安全", "sync.Once"},
	}

	if points, ok := keyPointsMap[question.Topic]; ok {
		return points
	}
	return []string{"核心概念", "实现原理", "应用场景", "注意事项"}
}

// analyzeStrengthsWeaknesses 分析优劣势
func (a *MockInterviewAgent) analyzeStrengthsWeaknesses(dimensionScores map[string]float64) ([]string, []string) {
	var strengths, weaknesses []string

	for topic, score := range dimensionScores {
		if score >= 80 {
			strengths = append(strengths, fmt.Sprintf("%s掌握扎实（%.0f分）", topic, score))
		} else if score < 60 {
			weaknesses = append(weaknesses, fmt.Sprintf("%s需要加强（%.0f分）", topic, score))
		}
	}

	if len(strengths) == 0 {
		strengths = append(strengths, "基础知识掌握尚可，继续努力")
	}
	if len(weaknesses) == 0 {
		weaknesses = append(weaknesses, "各方面表现均衡，建议挑战更高难度")
	}

	return strengths, weaknesses
}

// generateSuggestions 生成建议
func (a *MockInterviewAgent) generateSuggestions(weaknesses []string) []string {
	suggestions := []string{
		"建议制定系统的学习计划，针对性地提升薄弱环节",
		"多做模拟面试练习，提升临场表达能力",
		"关注实际项目经验，学会用具体案例支撑回答",
	}

	for _, w := range weaknesses {
		suggestions = append(suggestions, fmt.Sprintf("重点复习：%s", w))
	}

	return suggestions
}

// generateSummary 生成总结
func (a *MockInterviewAgent) generateSummary(overallScore float64, answered, total int) string {
	if overallScore >= 85 {
		return fmt.Sprintf("本次面试表现优秀！综合得分%.0f分，完成了%d/%d道题目。你展现了良好的技术功底和表达能力，建议继续保持并挑战更高难度。", overallScore, answered, total)
	} else if overallScore >= 70 {
		return fmt.Sprintf("本次面试表现良好。综合得分%.0f分，完成了%d/%d道题目。基础扎实，但在某些知识点上还有提升空间。", overallScore, answered, total)
	} else if overallScore >= 60 {
		return fmt.Sprintf("本次面试表现合格。综合得分%.0f分，完成了%d/%d道题目。建议针对薄弱环节进行针对性学习和练习。", overallScore, answered, total)
	}
	return fmt.Sprintf("本次面试需要加强。综合得分%.0f分，完成了%d/%d道题目。建议系统性地复习基础知识，多进行模拟练习。", overallScore, answered, total)
}

// containsTopic 检查主题列表是否包含指定主题
func containsTopic(topics []string, topic string) bool {
	for _, t := range topics {
		if t == topic {
			return true
		}
	}
	return false
}
