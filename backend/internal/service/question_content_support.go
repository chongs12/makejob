package service

import (
	"encoding/json"
	"strings"

	"makejob-backend/internal/model"
)

// QuestionStructuredSolution 描述编程题或客观题可复用的结构化解析内容。
type QuestionStructuredSolution struct {
	Summary         string   `json:"summary"`
	Approach        string   `json:"approach"`
	KeySteps        []string `json:"key_steps"`
	EdgeCases       []string `json:"edge_cases"`
	Complexity      string   `json:"complexity"`
	CommonMistakes  []string `json:"common_mistakes"`
	RecommendedTags []string `json:"recommended_tags"`
}

// QuestionAnswerTemplate 描述主观题的参考作答模板。
type QuestionAnswerTemplate struct {
	CoreConclusion string   `json:"core_conclusion"`
	KeyPoints      []string `json:"key_points"`
	SampleAnswer   string   `json:"sample_answer"`
	FollowUps      []string `json:"follow_ups"`
	Pitfalls       []string `json:"pitfalls"`
}

// QuestionTagTaxonomyGroup 描述一组标准题目标签及其用途说明。
type QuestionTagTaxonomyGroup struct {
	Group       string   `json:"group"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// QuestionSetPreview 描述题单中的简要题目预览信息。
type QuestionSetPreview struct {
	ID         uint   `json:"id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Difficulty string `json:"difficulty"`
}

// QuestionSetSummary 描述前台可展示的核心题单摘要。
type QuestionSetSummary struct {
	Slug          string               `json:"slug"`
	Title         string               `json:"title"`
	Description   string               `json:"description"`
	FocusTags     []string             `json:"focus_tags"`
	QuestionCount int                  `json:"question_count"`
	Questions     []QuestionSetPreview `json:"questions"`
}

type questionSetDefinition struct {
	Slug          string
	Title         string
	Description   string
	FocusTags     []string
	Keywords      []string
	PreferredTags []string
}

var standardQuestionTagAlias = map[string]string{
	"go":         "Go基础",
	"golang":     "Go基础",
	"基础概念":       "Go基础",
	"grpc":       "gRPC",
	"rest":       "REST",
	"jwt":        "JWT",
	"gorm":       "GORM",
	"gin":        "Gin",
	"preload":    "Preload",
	"joins":      "Joins",
	"context":    "context",
	"channel":    "channel",
	"goroutine":  "goroutine",
	"mutex":      "Mutex",
	"rwmutex":    "RWMutex",
	"atomic":     "atomic",
	"skiplist":   "跳表",
	"workerpool": "WorkerPool",
	"ioc":        "IoC",
	"di":         "DI",
	"http/2":     "HTTP/2",
	"http/3":     "HTTP/3",
	"quic":       "QUIC",
	"sql注入":      "SQL注入",
}

// standardQuestionTagTaxonomy 返回题库管理与推荐共用的标准标签分组。
func standardQuestionTagTaxonomy() []QuestionTagTaxonomyGroup {
	return []QuestionTagTaxonomyGroup{
		{
			Group:       "语言基础",
			Description: "用于 Go 语言语法、内存模型和常见基础问法。",
			Tags:        []string{"Go基础", "slice", "map", "defer", "interface", "make", "new", "context"},
		},
		{
			Group:       "并发控制",
			Description: "用于协程、同步原语、并发安全和调试问题。",
			Tags:        []string{"goroutine", "channel", "select", "Mutex", "RWMutex", "atomic", "并发安全"},
		},
		{
			Group:       "Web 与工程",
			Description: "用于 Web 框架、中间件、认证、项目结构与测试。",
			Tags:        []string{"Gin", "JWT", "中间件", "路由", "参数绑定", "测试", "项目结构", "最佳实践"},
		},
		{
			Group:       "数据库与存储",
			Description: "用于 ORM、事务、SQL 安全、索引和缓存。",
			Tags:        []string{"GORM", "Preload", "事务", "连接池", "索引", "缓存", "SQL注入"},
		},
		{
			Group:       "微服务与网络",
			Description: "用于 RPC、限流、链路追踪和网络协议。",
			Tags:        []string{"gRPC", "REST", "微服务", "服务发现", "熔断器", "链路追踪", "TCP", "HTTP/2", "WebSocket"},
		},
		{
			Group:       "算法与数据结构",
			Description: "用于算法题和数据结构题的知识点归类。",
			Tags:        []string{"数组", "字符串", "链表", "哈希", "双指针", "二叉树", "动态规划", "LRU", "跳表"},
		},
		{
			Group:       "错因标签",
			Description: "用于学习档案、报告诊断和对症推荐。",
			Tags:        []string{"状态定义不清", "边界条件生疏", "循环/索引控制不稳", "数据结构选择不当", "复杂度意识薄弱", "调试路径混乱", "代码实现不完整"},
		},
	}
}

// curatedQuestionSetDefinitions 返回当前前台可展示的核心题单定义。
func curatedQuestionSetDefinitions() []questionSetDefinition {
	return []questionSetDefinition{
		{
			Slug:          "go-runtime-core",
			Title:         "Go 运行时基础题单",
			Description:   "围绕 slice、map、defer、interface、GMP 调度等高频基础问法建立第一层稳定认知。",
			FocusTags:     []string{"Go基础", "slice", "map", "interface", "goroutine"},
			Keywords:      []string{"slice", "map", "interface", "GMP", "goroutine", "defer"},
			PreferredTags: []string{"Go基础", "slice", "map", "interface", "defer", "goroutine"},
		},
		{
			Slug:          "go-concurrency-debug",
			Title:         "并发控制与排错题单",
			Description:   "集中训练 channel、select、锁、goroutine 泄漏与并发安全这类高频实战问题。",
			FocusTags:     []string{"goroutine", "channel", "Mutex", "并发安全", "调试路径混乱"},
			Keywords:      []string{"channel", "select", "goroutine", "Mutex", "RWMutex", "atomic", "泄漏"},
			PreferredTags: []string{"goroutine", "channel", "select", "Mutex", "RWMutex", "atomic", "并发安全"},
		},
		{
			Slug:          "gin-backend-flow",
			Title:         "Gin 后端实战题单",
			Description:   "聚焦中间件、认证、参数绑定、错误处理和 Context 使用边界。",
			FocusTags:     []string{"Gin", "中间件", "JWT", "Context"},
			Keywords:      []string{"Gin", "中间件", "JWT", "参数绑定", "Context"},
			PreferredTags: []string{"Gin", "中间件", "JWT", "参数绑定", "Context"},
		},
		{
			Slug:          "gorm-database-core",
			Title:         "GORM 与数据库题单",
			Description:   "围绕事务、预加载、关联、索引和连接池，补齐服务端面试的数据库主线。",
			FocusTags:     []string{"GORM", "事务", "Preload", "索引", "连接池"},
			Keywords:      []string{"GORM", "事务", "Preload", "Joins", "索引", "连接池", "SQL"},
			PreferredTags: []string{"GORM", "事务", "Preload", "Joins", "索引", "连接池", "SQL注入"},
		},
		{
			Slug:          "microservice-network",
			Title:         "微服务与网络基础题单",
			Description:   "覆盖 gRPC、REST、服务发现、限流、链路追踪和 TCP/HTTP 高频网络题。",
			FocusTags:     []string{"微服务", "gRPC", "REST", "TCP", "HTTP/2"},
			Keywords:      []string{"gRPC", "REST", "微服务", "Consul", "etcd", "TCP", "HTTP"},
			PreferredTags: []string{"微服务", "gRPC", "REST", "服务发现", "限流", "链路追踪", "TCP", "HTTP/2"},
		},
		{
			Slug:          "algorithm-structure",
			Title:         "算法与数据结构补强题单",
			Description:   "优先补数组、哈希、LRU、跳表等容易在面试中穿插提问的算法基础。",
			FocusTags:     []string{"数组", "哈希", "数据结构", "LRU", "跳表"},
			Keywords:      []string{"LRU", "跳表", "哈希", "数据结构", "算法"},
			PreferredTags: []string{"数组", "字符串", "哈希", "数据结构", "LRU", "跳表"},
		},
	}
}

// normalizeQuestionTagStrings 将原始标签数组收敛为标准标签集合。
func normalizeQuestionTagStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		tag := strings.TrimSpace(value)
		if tag == "" {
			continue
		}
		lowered := strings.ToLower(tag)
		if mapped, ok := standardQuestionTagAlias[lowered]; ok {
			tag = mapped
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}

// normalizeQuestionTagsForStorage 将后台输入的标签字符串规范化后回写到存储层。
func normalizeQuestionTagsForStorage(raw string) string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || r == '\n' || r == '\r'
	})
	return strings.Join(normalizeQuestionTagStrings(parts), ",")
}

// parseQuestionTagsFromStorage 解析并标准化数据库中的题目标签。
func parseQuestionTagsFromStorage(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，'
	})
	return normalizeQuestionTagStrings(parts)
}

// buildFallbackQuestionSolution 基于旧字段为题目生成一份最低可用的结构化解析。
func buildFallbackQuestionSolution(question *model.Question) *QuestionStructuredSolution {
	if question == nil {
		return nil
	}

	summary := strings.TrimSpace(question.Explanation)
	if summary == "" {
		summary = "当前题目尚未补齐结构化解析，建议先根据标准答案理解核心思路。"
	}

	approach := "先明确题目要验证的核心能力，再从输入输出、关键数据结构和边界情况拆解解法。"
	if question.IsCode() {
		approach = "先确定数据结构和状态设计，再补齐主流程、边界条件和复杂度分析。"
	}

	return &QuestionStructuredSolution{
		Summary:         summary,
		Approach:        approach,
		KeySteps:        []string{"提炼题目输入输出与约束。", "选择合适的数据结构或核心策略。", "按主流程实现并校验关键分支。"},
		EdgeCases:       []string{"空输入或零值场景。", "边界索引、长度为 1、重复值等极端情况。"},
		Complexity:      "建议补充时间复杂度和空间复杂度分析。",
		CommonMistakes:  []string{"只写主流程，遗漏边界条件。", "没有说明数据结构选择原因。", "复杂度分析过于笼统。"},
		RecommendedTags: parseQuestionTagsFromStorage(question.Tags),
	}
}

// parseQuestionStructuredSolution 解析题目的结构化解析 JSON，并在缺失时自动兜底。
func parseQuestionStructuredSolution(raw string, question *model.Question) *QuestionStructuredSolution {
	var solution QuestionStructuredSolution
	if strings.TrimSpace(raw) != "" && json.Unmarshal([]byte(raw), &solution) == nil {
		return normalizeQuestionStructuredSolution(&solution, question)
	}
	return buildFallbackQuestionSolution(question)
}

// normalizeQuestionStructuredSolution 清理结构化解析中的空值并补齐标签。
func normalizeQuestionStructuredSolution(value *QuestionStructuredSolution, question *model.Question) *QuestionStructuredSolution {
	if value == nil {
		return buildFallbackQuestionSolution(question)
	}

	value.Summary = strings.TrimSpace(value.Summary)
	value.Approach = strings.TrimSpace(value.Approach)
	value.Complexity = strings.TrimSpace(value.Complexity)
	value.KeySteps = normalizeQuestionTagStrings(value.KeySteps)
	value.EdgeCases = normalizeQuestionTagStrings(value.EdgeCases)
	value.CommonMistakes = normalizeQuestionTagStrings(value.CommonMistakes)
	value.RecommendedTags = normalizeQuestionTagStrings(value.RecommendedTags)
	if len(value.RecommendedTags) == 0 && question != nil {
		value.RecommendedTags = parseQuestionTagsFromStorage(question.Tags)
	}
	if value.Summary == "" || value.Approach == "" {
		return buildFallbackQuestionSolution(question)
	}
	return value
}

// marshalQuestionStructuredSolution 将结构化解析对象写回数据库字段。
func marshalQuestionStructuredSolution(value *QuestionStructuredSolution, question *model.Question) (string, error) {
	normalized := normalizeQuestionStructuredSolution(value, question)
	if normalized == nil {
		return "", nil
	}
	content, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// buildFallbackQuestionAnswerTemplate 基于旧字段为主观题生成一份最低可用的作答模板。
func buildFallbackQuestionAnswerTemplate(question *model.Question) *QuestionAnswerTemplate {
	if question == nil || question.Type != model.QuestionTypeSubjective {
		return nil
	}

	coreConclusion := strings.TrimSpace(question.Answer)
	if coreConclusion == "" {
		coreConclusion = "先给出结论，再补充原因、适用场景和常见权衡。"
	}

	return &QuestionAnswerTemplate{
		CoreConclusion: coreConclusion,
		KeyPoints:      []string{"先给核心结论。", "再解释原理或实现机制。", "补充适用场景、优缺点和边界。"},
		SampleAnswer:   strings.TrimSpace(question.Explanation),
		FollowUps:      []string{"为什么这样设计？", "适用边界是什么？", "如果场景变化，你会怎么调整？"},
		Pitfalls:       []string{"只背概念，不结合场景。", "没有说明优缺点和权衡。", "回答过长但没有结构。"},
	}
}

// parseQuestionAnswerTemplate 解析主观题参考回答模板，并在缺失时自动兜底。
func parseQuestionAnswerTemplate(raw string, question *model.Question) *QuestionAnswerTemplate {
	if question != nil && question.Type != model.QuestionTypeSubjective {
		return nil
	}
	var template QuestionAnswerTemplate
	if strings.TrimSpace(raw) != "" && json.Unmarshal([]byte(raw), &template) == nil {
		return normalizeQuestionAnswerTemplate(&template, question)
	}
	return buildFallbackQuestionAnswerTemplate(question)
}

// normalizeQuestionAnswerTemplate 清理主观题参考模板中的空值。
func normalizeQuestionAnswerTemplate(value *QuestionAnswerTemplate, question *model.Question) *QuestionAnswerTemplate {
	if value == nil {
		return buildFallbackQuestionAnswerTemplate(question)
	}

	value.CoreConclusion = strings.TrimSpace(value.CoreConclusion)
	value.SampleAnswer = strings.TrimSpace(value.SampleAnswer)
	value.KeyPoints = normalizeQuestionTagStrings(value.KeyPoints)
	value.FollowUps = normalizeQuestionTagStrings(value.FollowUps)
	value.Pitfalls = normalizeQuestionTagStrings(value.Pitfalls)
	if value.CoreConclusion == "" {
		return buildFallbackQuestionAnswerTemplate(question)
	}
	return value
}

// marshalQuestionAnswerTemplate 将主观题参考模板写回数据库字段。
func marshalQuestionAnswerTemplate(value *QuestionAnswerTemplate, question *model.Question) (string, error) {
	if question != nil && question.Type != model.QuestionTypeSubjective && value == nil {
		return "", nil
	}
	normalized := normalizeQuestionAnswerTemplate(value, question)
	if normalized == nil {
		return "", nil
	}
	content, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// buildQuestionSetSummaries 根据当前题库内容构建核心题单摘要。
func buildQuestionSetSummaries(questions []model.Question) []QuestionSetSummary {
	definitions := curatedQuestionSetDefinitions()
	summaries := make([]QuestionSetSummary, 0, len(definitions))
	for _, definition := range definitions {
		matched := make([]QuestionSetPreview, 0, 4)
		for _, question := range questions {
			if !matchesQuestionSetDefinition(question, definition) {
				continue
			}
			matched = append(matched, QuestionSetPreview{
				ID:         question.ID,
				Title:      question.Title,
				Type:       question.Type,
				Difficulty: question.Difficulty,
			})
			if len(matched) >= 4 {
				break
			}
		}
		if len(matched) == 0 {
			continue
		}
		summaries = append(summaries, QuestionSetSummary{
			Slug:          definition.Slug,
			Title:         definition.Title,
			Description:   definition.Description,
			FocusTags:     definition.FocusTags,
			QuestionCount: len(matched),
			Questions:     matched,
		})
	}
	return summaries
}

// matchesQuestionSetDefinition 判断题目是否命中当前题单定义。
func matchesQuestionSetDefinition(question model.Question, definition questionSetDefinition) bool {
	content := strings.ToLower(strings.Join([]string{
		question.Title,
		question.Content,
		question.Tags,
		question.Answer,
	}, " "))

	for _, keyword := range definition.Keywords {
		if strings.Contains(content, strings.ToLower(keyword)) {
			return true
		}
	}

	for _, tag := range parseQuestionTagsFromStorage(question.Tags) {
		for _, preferred := range definition.PreferredTags {
			if strings.EqualFold(strings.TrimSpace(tag), strings.TrimSpace(preferred)) {
				return true
			}
		}
	}

	return false
}
