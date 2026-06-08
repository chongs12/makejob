package data

import (
	"regexp"
	"strings"

	"makejob/app/admin/internal/biz"
)

// ScraperCleaner 面经内容清洗器，从原始文本中提取结构化题目。
type ScraperCleaner struct{}

// NewScraperCleaner 创建清洗器实例。
func NewScraperCleaner() *ScraperCleaner {
	return &ScraperCleaner{}
}

// Clean 清洗面经内容，提取结构化题目。
func (c *ScraperCleaner) Clean(content, industryCode, source, sourceURL string) ([]*biz.ScraperCleanedQuestionRecord, int) {
	questions := c.extractQuestions(content)
	return questions, len(questions)
}

func (c *ScraperCleaner) extractQuestions(content string) []*biz.ScraperCleanedQuestionRecord {
	var questions []*biz.ScraperCleanedQuestionRecord

	lines := strings.Split(content, "\n")
	questionPattern := regexp.MustCompile(`^(\d+)[.、．]\s*(.+)$`)

	var currentQuestion *biz.ScraperCleanedQuestionRecord
	var answerLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := questionPattern.FindStringSubmatch(line)
		if len(matches) >= 3 {
			if currentQuestion != nil {
				currentQuestion.Answer = strings.Join(answerLines, "\n")
				currentQuestion.Content = currentQuestion.Title
				questions = append(questions, currentQuestion)
			}

			currentQuestion = &biz.ScraperCleanedQuestionRecord{
				Title:       matches[2],
				Tags:        strings.Join(c.inferTags(matches[2]), ","),
				Difficulty:  c.inferDifficulty(matches[2]),
				CategoryName: c.inferCategory(matches[2]),
				Type:        c.inferType(matches[2]),
				Explanation: c.generateExplanation(matches[2]),
			}
			answerLines = nil
		} else if currentQuestion != nil {
			if !isSectionHeader(line) {
				answerLines = append(answerLines, line)
			}
		}
	}

	if currentQuestion != nil {
		currentQuestion.Answer = strings.Join(answerLines, "\n")
		currentQuestion.Content = currentQuestion.Title
		questions = append(questions, currentQuestion)
	}

	return questions
}

func (c *ScraperCleaner) inferTags(title string) []string {
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
	return uniqueStrings(tags)
}

func (c *ScraperCleaner) inferDifficulty(title string) string {
	titleLower := strings.ToLower(title)
	hardKeywords := []string{"gc", "架构", "分布式", "设计", "微服务", "一致性"}
	for _, kw := range hardKeywords {
		if strings.Contains(titleLower, kw) {
			return "hard"
		}
	}
	mediumKeywords := []string{"goroutine", "channel", "redis", "mysql", "索引", "锁", "sync", "并发", "算法", "链表"}
	for _, kw := range mediumKeywords {
		if strings.Contains(titleLower, kw) {
			return "medium"
		}
	}
	return "easy"
}

func (c *ScraperCleaner) inferCategory(title string) string {
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

func (c *ScraperCleaner) inferType(title string) string {
	titleLower := strings.ToLower(title)
	codeKeywords := []string{"手撕", "算法", "实现", "编写", "代码", "反转", "排序", "链表"}
	for _, kw := range codeKeywords {
		if strings.Contains(titleLower, kw) {
			return "code"
		}
	}
	return "subjective"
}

func (c *ScraperCleaner) generateExplanation(title string) string {
	explanations := map[string]string{
		"slice":     "Slice是Go中常用的动态数组类型，底层指向一个数组，包含指针、长度和容量三个字段。",
		"goroutine": "Goroutine是Go的轻量级线程，由Go运行时调度。GMP模型中，G代表goroutine，M代表机器线程，P代表处理器。",
		"channel":   "Channel是goroutine之间的通信机制。无缓冲channel同步发送接收，有缓冲channel可以异步。",
		"gc":        "Go使用并发标记清除垃圾回收器，采用三色标记法。",
		"redis":     "Redis是高性能的key-value内存数据库，支持多种数据结构。",
		"mysql":     "MySQL索引使用B+树结构，聚簇索引叶子节点存储完整数据行。",
	}
	for keyword, explanation := range explanations {
		if strings.Contains(strings.ToLower(title), keyword) {
			return explanation
		}
	}
	return "此题考察对相关知识点的理解和实际应用能力。"
}

func isSectionHeader(line string) bool {
	sectionKeywords := []string{"一面", "二面", "三面", "HR面", "技术面", "项目面"}
	for _, kw := range sectionKeywords {
		if strings.Contains(line, kw) {
			return true
		}
	}
	return false
}

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
