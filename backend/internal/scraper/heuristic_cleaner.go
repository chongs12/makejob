package scraper

import (
	"context"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// HeuristicCleaner 使用规则与文本启发式从原始面经中抽取题目。
type HeuristicCleaner struct{}

// NewHeuristicCleaner 创建默认启发式清洗器。
func NewHeuristicCleaner() QuestionCleaner {
	return &HeuristicCleaner{}
}

// Clean 使用启发式规则清洗原始面经文本。
func (c *HeuristicCleaner) Clean(ctx context.Context, req CleanRequest) (*CleanResult, error) {
	_ = ctx
	questions := c.extractQuestions(req.Content)

	return &CleanResult{
		Questions:   questions,
		TotalFound:  len(questions),
		SourceTitle: "面经来源",
		SourceURL:   req.SourceURL,
	}, nil
}

// extractQuestions 从原始面经文本中提取候选题目。
func (c *HeuristicCleaner) extractQuestions(content string) []CleanedQuestion {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	lines := strings.Split(content, "\n")
	questionPattern := regexp.MustCompile(`^(\d+)[.、．]\s*(.+)$`)

	questions := make([]CleanedQuestion, 0)
	var currentQuestion *CleanedQuestion
	answerLines := make([]string, 0)

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
				questions = append(questions, *currentQuestion)
			}

			currentQuestion = &CleanedQuestion{
				Title:       matches[2],
				Tags:        c.inferTags(matches[2]),
				Confidence:  0.72 + r.Float64()*0.2,
				Difficulty:  c.inferDifficulty(matches[2]),
				Category:    c.inferCategory(matches[2]),
				Type:        c.inferType(matches[2]),
				Explanation: c.generateExplanation(matches[2]),
			}
			answerLines = answerLines[:0]
			continue
		}

		if currentQuestion != nil && !isHeuristicSectionHeader(line) {
			answerLines = append(answerLines, line)
		}
	}

	if currentQuestion != nil {
		currentQuestion.Answer = strings.Join(answerLines, "\n")
		currentQuestion.Content = currentQuestion.Title
		questions = append(questions, *currentQuestion)
	}

	return questions
}

// inferTags 根据题干关键词推断标签集合。
func (c *HeuristicCleaner) inferTags(title string) []string {
	tags := make([]string, 0)
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
	return uniqueHeuristicStrings(tags)
}

// inferDifficulty 根据题目文本估算难度。
func (c *HeuristicCleaner) inferDifficulty(title string) string {
	titleLower := strings.ToLower(title)
	hardKeywords := []string{"gc", "架构", "分布式", "设计", "微服务", "一致性"}
	for _, keyword := range hardKeywords {
		if strings.Contains(titleLower, keyword) {
			return "hard"
		}
	}

	mediumKeywords := []string{"goroutine", "channel", "redis", "mysql", "索引", "锁", "sync", "并发", "算法", "链表"}
	for _, keyword := range mediumKeywords {
		if strings.Contains(titleLower, keyword) {
			return "medium"
		}
	}
	return "easy"
}

// inferCategory 根据题目内容推断后台题库分类。
func (c *HeuristicCleaner) inferCategory(title string) string {
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

// inferType 根据题目文本推断题目类型。
func (c *HeuristicCleaner) inferType(title string) string {
	titleLower := strings.ToLower(title)
	codeKeywords := []string{"手撕", "算法", "实现", "编写", "代码", "反转", "排序", "链表"}
	for _, keyword := range codeKeywords {
		if strings.Contains(titleLower, keyword) {
			return "code"
		}
	}
	return "subjective"
}

// generateExplanation 生成一段可直接回填到题库的解释文本。
func (c *HeuristicCleaner) generateExplanation(title string) string {
	explanations := map[string]string{
		"slice":     "Slice 是 Go 中常用的动态数组类型，底层包含指针、长度和容量。扩容时容量较小时通常翻倍，容量较大后按比例增长。",
		"goroutine": "Goroutine 是 Go 运行时调度的轻量级执行单元，典型考点是 GMP 调度模型、栈扩缩容和抢占式调度。",
		"channel":   "Channel 是 Go 的通信原语，常考无缓冲与有缓冲 Channel 差异、关闭语义以及 select 调度行为。",
		"gc":        "Go 的 GC 采用并发标记清除模型，核心考点通常包括三色标记、写屏障和 STW 时机。",
		"redis":     "Redis 常见考点包括数据结构选型、缓存一致性、分布式锁和高可用部署。",
		"mysql":     "MySQL 常见考点包括 B+Tree 索引、事务隔离级别、锁机制和 SQL 优化。",
	}
	for keyword, explanation := range explanations {
		if strings.Contains(strings.ToLower(title), keyword) {
			return explanation
		}
	}
	return "此题主要考察候选人对相关知识点的理解深度，以及在真实工程场景中的应用能力。"
}

// isHeuristicSectionHeader 过滤面经中的章节标题行，避免误并入答案。
func isHeuristicSectionHeader(line string) bool {
	sectionKeywords := []string{"一面", "二面", "三面", "HR面", "技术面", "项目面"}
	for _, keyword := range sectionKeywords {
		if strings.Contains(line, keyword) {
			return true
		}
	}
	return false
}

// uniqueHeuristicStrings 对启发式标签做稳定去重。
func uniqueHeuristicStrings(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}
