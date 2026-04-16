// Package scraper 提供面经爬取与清洗导入功能
package scraper

import (
	"context"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// MockCleaner Mock题目清洗器实现
type MockCleaner struct{}

// NewMockCleaner 创建Mock清洗器实例
func NewMockCleaner() QuestionCleaner {
	return &MockCleaner{}
}

// Clean 清洗面经内容，提取结构化题目
func (c *MockCleaner) Clean(ctx context.Context, req CleanRequest) (*CleanResult, error) {
	questions := c.extractQuestions(req.Content)

	return &CleanResult{
		Questions:   questions,
		TotalFound:  len(questions),
		SourceTitle: "面经来源",
		SourceURL:   req.SourceURL,
	}, nil
}

// extractQuestions 从面经内容中提取题目
func (c *MockCleaner) extractQuestions(content string) []CleanedQuestion {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	var questions []CleanedQuestion

	// 按行分割内容
	lines := strings.Split(content, "\n")

	// 匹配题目的正则: 数字+点或顿号开头
	questionPattern := regexp.MustCompile(`^(\d+)[.、．]\s*(.+)$`)

	var currentQuestion *CleanedQuestion
	var answerLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 检查是否是题目开头
		matches := questionPattern.FindStringSubmatch(line)
		if len(matches) >= 3 {
			// 保存上一个题目
			if currentQuestion != nil {
				currentQuestion.Answer = strings.Join(answerLines, "\n")
				currentQuestion.Content = currentQuestion.Title
				questions = append(questions, *currentQuestion)
			}

			// 开始新题目
			currentQuestion = &CleanedQuestion{
				Title:       matches[2],
				Tags:        c.inferTags(matches[2]),
				Confidence:  0.7 + r.Float64()*0.25, // 0.7-0.95
				Difficulty:  c.inferDifficulty(matches[2]),
				Category:    c.inferCategory(matches[2]),
				Type:        c.inferType(matches[2]),
				Explanation: c.generateExplanation(matches[2]),
			}
			answerLines = nil
		} else if currentQuestion != nil {
			// 收集答案内容
			if !isSectionHeader(line) {
				answerLines = append(answerLines, line)
			}
		}
	}

	// 保存最后一个题目
	if currentQuestion != nil {
		currentQuestion.Answer = strings.Join(answerLines, "\n")
		currentQuestion.Content = currentQuestion.Title
		questions = append(questions, *currentQuestion)
	}

	return questions
}

// inferTags 根据题目内容推断标签
func (c *MockCleaner) inferTags(title string) []string {
	tags := []string{}

	keywords := map[string]string{
		"goroutine": "并发,GMP",
		"channel":   "并发,Channel",
		"slice":     "数据结构,Slice",
		"map":       "数据结构,Map",
		"gc":        "GC,内存管理",
		"redis":     "Redis,缓存",
		"mysql":     "MySQL,数据库",
		"索引":        "MySQL,数据库优化",
		"微服务":       "架构,微服务",
		"分布式":       "架构,分布式",
		"算法":        "算法,数据结构",
		"链表":        "算法,数据结构",
		"项目":        "项目经验",
		"幂等":        "架构,设计模式",
		"锁":         "并发,同步",
		"sync":      "并发,同步",
	}

	for keyword, tagStr := range keywords {
		if strings.Contains(strings.ToLower(title), keyword) {
			tags = append(tags, strings.Split(tagStr, ",")...)
		}
	}

	if len(tags) == 0 {
		tags = []string{"Go基础"}
	}

	// 去重
	return uniqueStrings(tags)
}

// inferDifficulty 推断难度
func (c *MockCleaner) inferDifficulty(title string) string {
	titleLower := strings.ToLower(title)

	// 困难关键词
	hardKeywords := []string{"gc", "架构", "分布式", "设计", "微服务", "一致性"}
	for _, kw := range hardKeywords {
		if strings.Contains(titleLower, kw) {
			return "hard"
		}
	}

	// 中等关键词
	mediumKeywords := []string{"goroutine", "channel", "redis", "mysql", "索引", "锁", "sync", "并发", "算法", "链表"}
	for _, kw := range mediumKeywords {
		if strings.Contains(titleLower, kw) {
			return "medium"
		}
	}

	// 默认简单
	return "easy"
}

// inferCategory 推断分类
func (c *MockCleaner) inferCategory(title string) string {
	titleLower := strings.ToLower(title)

	switch {
	case strings.Contains(titleLower, "goroutine") || strings.Contains(titleLower, "channel") || strings.Contains(titleLower, "并发") || strings.Contains(titleLower, "sync"):
		return "Go并发编程"
	case strings.Contains(titleLower, "slice") || strings.Contains(titleLower, "map") || strings.Contains(titleLower, "基础"):
		return "Go语言基础"
	case strings.Contains(titleLower, "redis") || strings.Contains(titleLower, "缓存"):
		return "Redis"
	case strings.Contains(titleLower, "mysql") || strings.Contains(titleLower, "数据库") || strings.Contains(titleLower, "索引"):
		return "MySQL数据库"
	case strings.Contains(titleLower, "微服务") || strings.Contains(titleLower, "架构") || strings.Contains(titleLower, "分布式"):
		return "系统架构"
	case strings.Contains(titleLower, "算法") || strings.Contains(titleLower, "链表") || strings.Contains(titleLower, "树") || strings.Contains(titleLower, "排序"):
		return "算法与数据结构"
	case strings.Contains(titleLower, "项目"):
		return "项目经验"
	default:
		return "Go语言基础"
	}
}

// inferType 推断题目类型
func (c *MockCleaner) inferType(title string) string {
	titleLower := strings.ToLower(title)

	// 编程题关键词
	codeKeywords := []string{"手撕", "算法", "实现", "编写", "代码", "反转", "排序", "链表"}
	for _, kw := range codeKeywords {
		if strings.Contains(titleLower, kw) {
			return "code"
		}
	}

	// 默认主观题（面试问答）
	return "subjective"
}

// generateExplanation 生成参考解析
func (c *MockCleaner) generateExplanation(title string) string {
	// 根据关键词生成简单的参考解析
	explanations := map[string]string{
		"slice":     "Slice是Go中常用的动态数组类型，底层指向一个数组，包含指针、长度和容量三个字段。扩容时，容量小于1024直接翻倍，大于1024增长25%。",
		"goroutine": "Goroutine是Go的轻量级线程，由Go运行时调度。GMP模型中，G代表goroutine，M代表机器线程，P代表处理器。P的数量默认等于GOMAXPROCS。",
		"channel":   "Channel是goroutine之间的通信机制。无缓冲channel同步发送接收，有缓冲channel可以异步。底层使用互斥锁和条件变量实现。",
		"gc":        "Go使用并发标记清除垃圾回收器，采用三色标记法。白色表示未访问对象，灰色表示已访问但引用未处理完，黑色表示已完全处理。",
		"redis":     "Redis是高性能的key-value内存数据库，支持多种数据结构。常用于缓存、分布式锁、消息队列等场景。",
		"mysql":     "MySQL索引使用B+树结构，聚簇索引叶子节点存储完整数据行，非聚簇索引存储主键值。索引失效场景包括：函数操作、类型转换、LIKE以%开头等。",
	}

	for keyword, explanation := range explanations {
		if strings.Contains(strings.ToLower(title), keyword) {
			return explanation
		}
	}

	return "此题考察对相关知识点的理解和实际应用能力。"
}

// isSectionHeader 判断是否是章节标题
func isSectionHeader(line string) bool {
	sectionKeywords := []string{"一面", "二面", "三面", "HR面", "技术面", "项目面"}
	for _, kw := range sectionKeywords {
		if strings.Contains(line, kw) {
			return true
		}
	}
	return false
}

// uniqueStrings 字符串数组去重
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
