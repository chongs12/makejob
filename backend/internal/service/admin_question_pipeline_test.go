package service

import (
	"strings"
	"testing"

	"makejob-backend/internal/model"
)

// TestDecodeQuestionPipelinePlanResponseSupportsArray 验证考点计划解析兼容数组根节点与别名字段。
func TestDecodeQuestionPipelinePlanResponseSupportsArray(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelinePlanResponse("```json\n[\n  {\n    \"title\": \"goroutine 调度\",\n    \"point\": \"考察 GMP 调度模型与抢占式调度\",\n    \"level\": \"hard\",\n    \"classification\": \"Go 并发\",\n    \"keywords\": [\"goroutine\", \"scheduler\"]\n  }\n]\n```")
	if err != nil {
		t.Fatalf("decodeQuestionPipelinePlanResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Topics) != 1 {
		t.Fatalf("expected 1 topic, got %#v", payload)
	}
	if payload.Topics[0].Topic != "goroutine 调度" {
		t.Fatalf("unexpected topic: %#v", payload.Topics[0])
	}
	if payload.Topics[0].Focus != "考察 GMP 调度模型与抢占式调度" {
		t.Fatalf("unexpected focus: %#v", payload.Topics[0])
	}
	if payload.Topics[0].Difficulty != "hard" {
		t.Fatalf("unexpected difficulty: %#v", payload.Topics[0])
	}
}

// TestDecodeQuestionPipelinePlanResponseSupportsNestedChineseKeys 验证考点计划解析兼容嵌套 result/data 与中文字段名。
func TestDecodeQuestionPipelinePlanResponseSupportsNestedChineseKeys(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelinePlanResponse(`{
  "result": {
    "计划": [
      {
        "考点": "channel 同步语义",
        "考察重点": "区分无缓冲与有缓冲 channel 的阻塞行为",
        "难度": "medium",
        "分类": "Go 并发",
        "标签": ["channel", "并发"]
      }
    ]
  }
}`)
	if err != nil {
		t.Fatalf("decodeQuestionPipelinePlanResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Topics) != 1 {
		t.Fatalf("expected 1 topic, got %#v", payload)
	}
	if payload.Topics[0].Topic != "channel 同步语义" {
		t.Fatalf("unexpected topic: %#v", payload.Topics[0])
	}
	if payload.Topics[0].Focus != "区分无缓冲与有缓冲 channel 的阻塞行为" {
		t.Fatalf("unexpected focus: %#v", payload.Topics[0])
	}
}

// TestDecodeQuestionPipelinePlanResponseSupportsPlainTextList 验证考点计划解析兼容非 JSON 的纯文本列表。
func TestDecodeQuestionPipelinePlanResponseSupportsPlainTextList(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelinePlanResponse(`
以下是规划结果：
1. 考点：goroutine 调度模型
   考察重点：GMP 调度、抢占式调度与线程复用
   难度：hard
   分类：Go 并发

2. 考点：channel 阻塞语义
   考察重点：无缓冲与有缓冲 channel 的同步差异
   难度：medium
   分类：Go 并发
`)
	if err != nil {
		t.Fatalf("decodeQuestionPipelinePlanResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Topics) != 2 {
		t.Fatalf("expected 2 topics, got %#v", payload)
	}
	if payload.Topics[0].Topic != "goroutine 调度模型" {
		t.Fatalf("unexpected first topic: %#v", payload.Topics[0])
	}
	if payload.Topics[1].Focus != "无缓冲与有缓冲 channel 的同步差异" {
		t.Fatalf("unexpected second focus: %#v", payload.Topics[1])
	}
}

// TestDecodeQuestionPipelinePlanResponseSupportsYamlLikeText 验证考点计划解析兼容 YAML 风格文本。
func TestDecodeQuestionPipelinePlanResponseSupportsYamlLikeText(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelinePlanResponse("```yaml\n- topic: map 扩容机制\n  focus: 渐进迁移、oldbuckets 与扩容触发条件\n  difficulty: hard\n  category: Go 基础\n```")
	if err != nil {
		t.Fatalf("decodeQuestionPipelinePlanResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Topics) != 1 {
		t.Fatalf("expected 1 topic, got %#v", payload)
	}
	if payload.Topics[0].Topic != "map 扩容机制" {
		t.Fatalf("unexpected topic: %#v", payload.Topics[0])
	}
}

// TestDecodeQuestionPipelineCardsResponseSupportsAliases 验证题卡解析兼容 questions 根字段和 answer 别名。
func TestDecodeQuestionPipelineCardsResponseSupportsAliases(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelineCardsResponse(`{
  "questions": [
    {
      "topic": "channel 底层机制",
      "question": "请解释无缓冲 channel 与有缓冲 channel 的阻塞语义差异。",
      "kind": "subjective",
      "level": "medium",
      "classification": "Go 并发",
      "reference_answer": "无缓冲 channel 要求收发双方同步握手；有缓冲 channel 在缓冲未满时发送方可先继续执行。",
      "reasoning": "用于区分候选人是否真正理解 channel 同步语义。",
      "keywords": "channel,并发"
    }
  ]
}`)
	if err != nil {
		t.Fatalf("decodeQuestionPipelineCardsResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Cards) != 1 {
		t.Fatalf("expected 1 card, got %#v", payload)
	}
	card := payload.Cards[0]
	if card.Title != "channel 底层机制" {
		t.Fatalf("unexpected title: %#v", card)
	}
	if card.Answer == "" {
		t.Fatalf("expected answer to be normalized, got %#v", card)
	}
	if card.Explanation == "" {
		t.Fatalf("expected explanation to be normalized, got %#v", card)
	}
	if len(card.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %#v", card.Tags)
	}
}

// TestDecodeQuestionPipelineCardsResponseSupportsEmbeddedJSONString 验证题卡解析兼容字符串化 JSON 列表。
func TestDecodeQuestionPipelineCardsResponseSupportsEmbeddedJSONString(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelineCardsResponse(`{
  "data": {
    "题卡": "[{\"标题\":\"map 扩容机制\",\"题目\":\"请说明 Go map 扩容的渐进迁移机制。\",\"参考答案\":\"Go map 扩容会采用渐进式搬迁，避免一次性重哈希造成长时间阻塞。\",\"难度\":\"hard\",\"分类\":\"Go 基础\"}]"
  }
}`)
	if err != nil {
		t.Fatalf("decodeQuestionPipelineCardsResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Cards) != 1 {
		t.Fatalf("expected 1 card, got %#v", payload)
	}
	if payload.Cards[0].Title != "map 扩容机制" {
		t.Fatalf("unexpected card title: %#v", payload.Cards[0])
	}
}

// TestDecodeQuestionPipelineCardsResponseSupportsPlainText 验证题卡解析兼容非 JSON 的纯文本结构。
func TestDecodeQuestionPipelineCardsResponseSupportsPlainText(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelineCardsResponse(`
1. 标题：channel 底层同步机制
   题目：请解释无缓冲 channel 与有缓冲 channel 在发送和接收时的阻塞差异。
   参考答案：无缓冲 channel 需要收发双方同步握手；有缓冲 channel 在缓冲区未满时发送方可以先继续执行。
   解析：重点考察候选人是否理解 channel 的同步语义与使用边界。

2. 标题：Go map 渐进扩容
   题目：请说明 Go map 扩容时为什么采用渐进迁移而不是一次性 rehash。
   参考答案：渐进迁移可以把搬迁成本分摊到后续读写操作上，避免一次性扩容造成明显卡顿。
`)
	if err != nil {
		t.Fatalf("decodeQuestionPipelineCardsResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Cards) != 2 {
		t.Fatalf("expected 2 cards, got %#v", payload)
	}
	if payload.Cards[0].Title != "channel 底层同步机制" {
		t.Fatalf("unexpected first card: %#v", payload.Cards[0])
	}
	if payload.Cards[1].Answer == "" {
		t.Fatalf("expected second card answer, got %#v", payload.Cards[1])
	}
}

// TestDecodeQuestionPipelineCardsResponseSupportsThinkWrappedJSON 验证题卡解析兼容被 think 标签包裹的 JSON 输出。
func TestDecodeQuestionPipelineCardsResponseSupportsThinkWrappedJSON(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelineCardsResponse(`<think>先整理输出结构</think>
{"cards":[{"title":"Go map 并发读写","content":"为什么 Go 原生 map 不支持并发读写？","answer":"因为 map 在扩容和写入时会修改内部结构，未加同步会出现数据竞争甚至直接 panic。","difficulty":"medium","category":"Go 基础"}]}`)
	if err != nil {
		t.Fatalf("decodeQuestionPipelineCardsResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Cards) != 1 {
		t.Fatalf("expected 1 card, got %#v", payload)
	}
	if payload.Cards[0].Title != "Go map 并发读写" {
		t.Fatalf("unexpected card: %#v", payload.Cards[0])
	}
}

// TestDecodeQuestionPipelineCardsResponseSupportsTrailingJSONBlock 验证题卡解析会优先提取输出尾部真正的 JSON 结果块。
func TestDecodeQuestionPipelineCardsResponseSupportsTrailingJSONBlock(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelineCardsResponse(`以下是示例格式，请忽略：
{"cards":[{"title":"示例题","content":"示例题干","answer":"示例答案"}]}

最终结果：
{"cards":[{"title":"真实题卡","content":"请解释 Go channel 的同步语义。","answer":"无缓冲 channel 需要收发双方同步握手。","difficulty":"medium","category":"Go 并发"}]}`)
	if err != nil {
		t.Fatalf("decodeQuestionPipelineCardsResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Cards) != 1 {
		t.Fatalf("expected 1 card, got %#v", payload)
	}
	if payload.Cards[0].Title != "真实题卡" {
		t.Fatalf("expected trailing real card, got %#v", payload.Cards[0])
	}
}

// TestDecodeQuestionPipelineCardsResponseSupportsReasoningWrappedText 验证题卡解析在去除 think 标签后仍能回退解析普通文本题卡。
func TestDecodeQuestionPipelineCardsResponseSupportsReasoningWrappedText(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelineCardsResponse(`<think>
先分析用户要求，再给出单张题卡。
</think>
标题：slice 扩容策略
题目：请说明 Go slice 扩容时容量增长策略与底层数组复制行为。
参考答案：slice 扩容会按容量区间采用不同增长策略，并在需要时重新分配底层数组。
解析：重点考察候选人是否真正理解 slice 的底层实现。`)
	if err != nil {
		t.Fatalf("decodeQuestionPipelineCardsResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Cards) != 1 {
		t.Fatalf("expected 1 card, got %#v", payload)
	}
	if payload.Cards[0].Title != "slice 扩容策略" {
		t.Fatalf("unexpected card: %#v", payload.Cards[0])
	}
}

// TestFilterQuestionPipelineCardsByIntentDropsGenericProjectQuestions 验证语言特性场景下会过滤明显跑偏的项目题。
func TestFilterQuestionPipelineCardsByIntentDropsGenericProjectQuestions(t *testing.T) {
	t.Parallel()

	filtered := filterQuestionPipelineCardsByIntent([]AdminQuestionPipelineCard{
		{
			Title:   "项目中遇到的最大的挑战是什么？",
			Content: "请结合你的 Go 项目经验说明。",
		},
		{
			Title:   "goroutine 和线程的区别",
			Content: "请解释 GMP 模型与调度开销差异。",
		},
	}, "生成 Go 后端高级工程师面试题，重点聚焦于 Go 语言特性。", "避免项目题，只保留语言机制题")

	if len(filtered) != 1 {
		t.Fatalf("expected 1 card after filtering, got %#v", filtered)
	}
	if filtered[0].Title != "goroutine 和线程的区别" {
		t.Fatalf("unexpected remaining card: %#v", filtered[0])
	}
}

// TestBuildQuestionPipelineConstraintProfileExtractsLanguageQuota 验证可从智能体提示词中提取语言配额与剩余语言约束。
func TestBuildQuestionPipelineConstraintProfileExtractsLanguageQuota(t *testing.T) {
	t.Parallel()

	profile := buildQuestionPipelineConstraintProfile(
		"生成 Go 后端高级工程师面试题，重点聚焦于 Go 语言特性。",
		"确保每张题卡考察不同考点。其中必须有一个题是java，其他是go。",
		8,
	)

	if profile.ExactLanguageCounts["java"] != 1 {
		t.Fatalf("expected java quota 1, got %#v", profile.ExactLanguageCounts)
	}
	if profile.RemainingLanguage != "go" {
		t.Fatalf("expected remaining language go, got %#v", profile.RemainingLanguage)
	}
	if !profile.GoFeatureOnly {
		t.Fatal("expected Go feature only constraint to be enabled")
	}
}

// TestNormalizeQuestionPipelineGenerationMode 验证题目流水线生成模式会回退到已知枚举，避免非法值污染服务分支。
func TestNormalizeQuestionPipelineGenerationMode(t *testing.T) {
	t.Parallel()

	if mode := normalizeQuestionPipelineGenerationMode("direct_single"); mode != questionPipelineModeDirect {
		t.Fatalf("expected direct mode, got %s", mode)
	}
	if mode := normalizeQuestionPipelineGenerationMode("unexpected"); mode != questionPipelineModePlanned {
		t.Fatalf("expected planned mode fallback, got %s", mode)
	}
}

// TestBuildQuestionPipelineDirectTargetLanguages 验证逐张直生模式会按显式语言配额构造目标语言序列。
func TestBuildQuestionPipelineDirectTargetLanguages(t *testing.T) {
	t.Parallel()

	targets := buildQuestionPipelineDirectTargetLanguages(questionPipelineConstraintProfile{
		CandidateCount:      4,
		ExactLanguageCounts: map[string]int{"java": 1},
		ExactLanguageOrder:  []string{"java"},
		RemainingLanguage:   "go",
	}, 4)

	if len(targets) != 4 {
		t.Fatalf("expected 4 target languages, got %#v", targets)
	}
	if targets[0] != "java" {
		t.Fatalf("expected first target to be java, got %#v", targets)
	}
	if targets[1] != "go" || targets[2] != "go" || targets[3] != "go" {
		t.Fatalf("expected remaining targets to be go, got %#v", targets)
	}
}

// TestEnforceQuestionPipelineCardConstraintsAppliesLanguageQuota 验证题卡硬约束校验会丢弃项目题并执行 Java/Go 配额。
func TestEnforceQuestionPipelineCardConstraintsAppliesLanguageQuota(t *testing.T) {
	t.Parallel()

	profile := buildQuestionPipelineConstraintProfile(
		"生成 Go 后端高级工程师面试题，重点聚焦于 Go 语言特性。",
		"确保每张题卡考察不同考点，优先生成真正区分度高的问答题。其中必须有一个题是java，其他是go。",
		4,
	)

	cards, warnings := enforceQuestionPipelineCardConstraints([]AdminQuestionPipelineCard{
		{
			Title:   "项目中遇到的最大的挑战是什么？",
			Content: "请结合你的 Go 项目经验说明。",
			Answer:  "描述项目挑战与推进方式。",
		},
		{
			Title:   "Java 内存模型中的 volatile 语义",
			Content: "请说明 volatile 如何保证可见性，并与并发场景联系起来。",
			Answer:  "volatile 保证写后对其他线程可见，并限制部分指令重排。",
		},
		{
			Title:   "channel 底层实现原理",
			Content: "请解释有缓冲和无缓冲 channel 的差异。",
			Answer:  "无缓冲 channel 需要收发双方同步握手；有缓冲 channel 则依赖环形队列和等待队列。",
		},
		{
			Title:   "slice 扩容机制",
			Content: "请说明 Go slice 扩容时容量增长策略与底层数组复制行为。",
			Answer:  "slice 扩容会按容量区间采用不同增长策略，并在需要时重新分配底层数组。",
		},
		{
			Title:   "GMP 调度模型",
			Content: "请解释 G、M、P 三者的职责以及抢占式调度。",
			Answer:  "G 表示 goroutine，M 表示线程，P 表示执行上下文；调度器负责在三者之间高效切换。",
		},
	}, profile, "生成 Go 后端高级工程师面试题，重点聚焦于 Go 语言特性。", "其中必须有一个题是java，其他是go")

	if len(cards) != 4 {
		t.Fatalf("expected 4 cards after constraint enforcement, got %#v", cards)
	}
	if detectQuestionPipelineCardLanguage(cards[0]) != "java" {
		t.Fatalf("expected first selected card to satisfy java quota, got %#v", cards[0])
	}
	goCount := 0
	for _, card := range cards[1:] {
		if detectQuestionPipelineCardLanguage(card) == "go" {
			goCount++
		}
		if strings.Contains(card.Title, "项目") {
			t.Fatalf("unexpected project card retained: %#v", card)
		}
	}
	if goCount != 3 {
		t.Fatalf("expected remaining 3 cards to be Go focused, got %#v", cards)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
}

// TestBuildQuestionPipelineCardsFromPlanUsesPlanTopics 验证题卡生成失败时会基于规划结果回退生成题卡。
func TestBuildQuestionPipelineCardsFromPlanUsesPlanTopics(t *testing.T) {
	t.Parallel()

	cards := buildQuestionPipelineCardsFromPlan(&questionPipelinePlanResponse{
		Topics: []questionPipelineTopicPlan{
			{
				Topic:      "Java 内存模型与 Go 内存模型的差异",
				Focus:      "对比 happens-before、可见性与语言层抽象差异",
				Difficulty: "hard",
				Category:   "Go 基础语法",
			},
		},
	}, []model.Category{
		{Name: "Go 基础语法"},
	}, "生成 Go 后端高级工程师面试题", 8)

	if len(cards) != 1 {
		t.Fatalf("expected 1 fallback card, got %#v", cards)
	}
	if cards[0].Title != "Java 内存模型与 Go 内存模型的差异" {
		t.Fatalf("unexpected fallback card: %#v", cards[0])
	}
	if cards[0].SourceType != "generated" {
		t.Fatalf("expected generated fallback card, got %#v", cards[0])
	}
}

// TestBuildQuestionPipelineFailureMessageUsesWarnings 验证零题卡时会优先返回真实失败原因。
func TestBuildQuestionPipelineFailureMessageUsesWarnings(t *testing.T) {
	t.Parallel()

	message := buildQuestionPipelineFailureMessage([]string{
		"没有抓取到可用面经素材",
		"智能体题卡生成阶段返回内容无法解析: invalid character 'x'",
	})
	if message != "没有抓取到可用面经素材；智能体题卡生成阶段返回内容无法解析: invalid character 'x'" {
		t.Fatalf("unexpected failure message: %s", message)
	}
}

// TestBuildQuestionPipelineDebugTracePrefersModelOutputAndError 验证调试文本会同时保留模型原始输出和底层错误。
func TestBuildQuestionPipelineDebugTracePrefersModelOutputAndError(t *testing.T) {
	t.Parallel()

	trace := buildQuestionPipelineDebugTrace(&AIDebugResponse{
		ModelOutput: "```json\n{\"topics\":[]}\n```",
		ModelError:  "invalid character 'x' looking for beginning of value",
	})
	expected := "[model_output]\n```json\n{\"topics\":[]}\n```\n\n[model_error]\ninvalid character 'x' looking for beginning of value"
	if trace != expected {
		t.Fatalf("unexpected debug trace: %q", trace)
	}
}

// TestBuildQuestionPipelineDebugTraceFallsBackToModelError 验证缺少原始输出时仍会回传模型错误，便于失败重放定位。
func TestBuildQuestionPipelineDebugTraceFallsBackToModelError(t *testing.T) {
	t.Parallel()

	trace := buildQuestionPipelineDebugTrace(&AIDebugResponse{
		ModelError: "model timeout",
	})
	if trace != "[model_error]\nmodel timeout" {
		t.Fatalf("unexpected debug trace fallback: %q", trace)
	}
}

// TestBuildQuestionPipelineFailureMessageDedupesAndCaps 验证失败提示会去重并限制长度，避免前台出现冗长噪音。
func TestBuildQuestionPipelineFailureMessageDedupesAndCaps(t *testing.T) {
	t.Parallel()

	message := buildQuestionPipelineFailureMessage([]string{
		"",
		"没有抓取到可用面经素材",
		"没有抓取到可用面经素材",
		"智能体题卡生成阶段返回内容无法解析",
		"题卡补齐阶段未生成有效结果",
	})
	if message != "没有抓取到可用面经素材；智能体题卡生成阶段返回内容无法解析" {
		t.Fatalf("unexpected capped failure message: %s", message)
	}
}
