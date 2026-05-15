package service

import (
	"strings"
	"testing"

	"makejob-backend/internal/ai"
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

// TestDecodeQuestionPipelineCardsResponseSupportsSingleQuotedJSONLike 验证题卡解析兼容模型使用单引号包裹的 JSON-like 输出。
func TestDecodeQuestionPipelineCardsResponseSupportsSingleQuotedJSONLike(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelineCardsResponse(`{'cards':[{'title':'验证二叉搜索树','content':'给定一棵二叉树 root，判断其是否是一个有效的二叉搜索树。','type':'code','difficulty':'medium','category':'数据结构与算法','answer':'package main\n\nfunc isValidBST(root *TreeNode) bool {\n    return true\n}','solution':'使用递归维护上下界。','judge_config':{'evaluation_mode':'testcase','default_language':'go','allowed_languages':['go'],'starter_code':'package main','public_test_cases':[{'input':'[2,1,3]','expected_output':'true','description':'标准 BST'}],'hidden_test_cases':[{'input':'[5,1,4,null,null,3,6]','expected_output':'false','description':'非法 BST'}],'reference_solutions':[{'language':'go','code':'package main\n\nfunc isValidBST(root *TreeNode) bool {\n    return true\n}'}],'time_limit_ms':2000,'memory_limit_mb':128}}]}`)
	if err != nil {
		t.Fatalf("decodeQuestionPipelineCardsResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Cards) != 1 {
		t.Fatalf("expected 1 card, got %#v", payload)
	}
	if payload.Cards[0].Title != "验证二叉搜索树" {
		t.Fatalf("unexpected card: %#v", payload.Cards[0])
	}
	judgeConfig := parseQuestionJudgeConfigPayload(payload.Cards[0].JudgeConfig)
	if judgeConfig == nil || judgeConfig.DefaultLanguage != "go" {
		t.Fatalf("expected parsed go judge config, got %#v", payload.Cards[0].JudgeConfig)
	}
}

// TestDecodeQuestionPipelineCardsResponseSupportsSmartQuotedJSONLike 验证题卡解析兼容智能引号与全角标点的 JSON-like 输出。
func TestDecodeQuestionPipelineCardsResponseSupportsSmartQuotedJSONLike(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelineCardsResponse(`{“cards”：[{"title":"合并两个有序链表","content":"将两个升序链表合并为一个新的升序链表并返回。","type":"code","difficulty":"easy","answer":"package main\n\nfunc mergeTwoLists(a *ListNode, b *ListNode) *ListNode {\n    return nil\n}","solution":"使用双指针逐步归并两个链表。","judge_config":{“evaluation_mode”：“testcase”，“default_language”：“go”，“allowed_languages”：[“go”],“starter_code”：“package main”，“public_test_cases”:[{"input":"1 2\n1 3","expected_output":"1 1 2 3","description":"基础样例"}],“hidden_test_cases”:[{"input":"","expected_output":"","description":"空链表"}],“reference_solutions”:[{"language":"go","code":"package main\n\nfunc mergeTwoLists(a *ListNode, b *ListNode) *ListNode {\n    return nil\n}"}],“time_limit_ms”：2000，“memory_limit_mb”：128}}]}`)
	if err != nil {
		t.Fatalf("decodeQuestionPipelineCardsResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Cards) != 1 {
		t.Fatalf("expected 1 card, got %#v", payload)
	}
	if payload.Cards[0].Title != "合并两个有序链表" {
		t.Fatalf("unexpected card: %#v", payload.Cards[0])
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

// TestDecodeQuestionPipelineSingleCardResponsePrefersTailJSONBlock 验证单卡解析会优先提取串包尾部重新开始的真实 JSON 对象。
func TestDecodeQuestionPipelineSingleCardResponsePrefersTailJSONBlock(t *testing.T) {
	t.Parallel()

	raw := "{\"cards\":[{\"title\":\"寻找两个正序数组的中位数\",\"content\":\"旧题干\",\"type\":\"code\",\"answer\":\"package main\\n\\nfunc findMedianSortedArrays(nums1 []int, nums2 []int) float64 {\\n\\treturn 0\\n}\\nminRight```json\\n" +
		"{\n" +
		"  \"id\": 8,\n" +
		"  \"title\": \"[GO] 正则表达式匹配\",\n" +
		"  \"content\": \"给你一个字符串 s 和一个字符规律 p，请实现一个支持 '.' 和 '*' 的正则表达式匹配。\",\n" +
		"  \"type\": \"code\",\n" +
		"  \"difficulty\": \"hard\",\n" +
		"  \"category\": \"数据结构与算法\",\n" +
		"  \"answer\": \"```go\\nfunc isMatch(s string, p string) bool {\\n    return true\\n}\\n```\",\n" +
		"  \"solution\": \"核心思路：使用动态规划记录前缀匹配状态。\",\n" +
		"  \"judge_config\": {\n" +
		"    \"evaluation_mode\": \"testcase\",\n" +
		"    \"default_language\": \"go\",\n" +
		"    \"allowed_languages\": [\"go\"],\n" +
		"    \"starter_code\": \"package main\",\n" +
		"    \"public_test_cases\": [\n" +
		"      {\"input\":\"\\\"aa\\\"\\n\\\"a\\\"\",\"expected_output\":\"false\",\"description\":\"公开样例1\"},\n" +
		"      {\"input\":\"\\\"ab\\\"\\n\\\".*\\\"\",\"expected_output\":\"true\",\"description\":\"公开样例2\"},\n" +
		"      {\"input\":\"\\\"aab\\\"\\n\\\"c*a*b\\\"\",\"expected_output\":\"true\",\"description\":\"公开样例3\"}\n" +
		"    ],\n" +
		"    \"hidden_test_cases\": [\n" +
		"      {\"input\":\"\\\"mississippi\\\"\\n\\\"mis*is*p*.\\\"\",\"expected_output\":\"false\",\"description\":\"隐藏样例1\"}\n" +
		"    ],\n" +
		"    \"reference_solutions\": [\n" +
		"      {\"language\":\"go\",\"solution_code\":\"func isMatch(s string, p string) bool {\\n    return true\\n}\"}\n" +
		"    ],\n" +
		"    \"time_limit_ms\": 2000,\n" +
		"    \"memory_limit_mb\": 128\n" +
		"  }\n" +
		"}"
	card, candidate, err := decodeQuestionPipelineSingleCardResponse(raw)
	if err != nil {
		t.Fatalf("decodeQuestionPipelineSingleCardResponse returned error: %v", err)
	}
	if card.Title != "[GO] 正则表达式匹配" {
		t.Fatalf("expected tail card title, got %#v", card)
	}
	if !strings.Contains(candidate, `"title": "[GO] 正则表达式匹配"`) {
		t.Fatalf("expected candidate to point at tail JSON block, got %q", candidate)
	}
}

// TestDecodeQuestionPipelineCardsResponseSupportsCodeTypeAndJudgeConfig 验证编程题别名和 judge_config 会被完整保留。
func TestDecodeQuestionPipelineCardsResponseSupportsCodeTypeAndJudgeConfig(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelineCardsResponse(`{
  "cards": [
    {
      "title": "两数之和",
      "content": "给定两个整数，输出它们的和。",
      "type": "coding",
      "difficulty": "easy",
	      "category": "算法",
	      "answer": "按题意读取输入并输出结果。",
	      "solution": "先读取输入，再输出求和结果，注意输入输出格式。",
	      "explanation": "考察基础输入输出。",
	      "judgeConfig": {
	        "evaluation_mode": "testcase",
        "default_language": "go",
        "allowed_languages": ["go"],
        "starter_code": "package main",
        "public_test_cases": [{"input":"1 2","expected_output":"3"}],
        "hidden_test_cases": [{"input":"2 3","expected_output":"5"}],
        "reference_solutions": [{"language":"go","code":"package main"}],
        "time_limit_ms": 2000,
        "memory_limit_mb": 128
      }
    }
  ]
}`)
	if err != nil {
		t.Fatalf("decodeQuestionPipelineCardsResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Cards) != 1 {
		t.Fatalf("expected 1 card, got %#v", payload)
	}
	if payload.Cards[0].Type != model.QuestionTypeCode {
		t.Fatalf("expected code type, got %#v", payload.Cards[0])
	}
	if payload.Cards[0].Solution == "" {
		t.Fatalf("expected solution to be preserved, got %#v", payload.Cards[0])
	}
	judgeConfig, ok := payload.Cards[0].JudgeConfig.(map[string]any)
	if !ok || len(judgeConfig) == 0 {
		t.Fatalf("expected judge_config payload, got %#v", payload.Cards[0].JudgeConfig)
	}
	if judgeConfig["evaluation_mode"] != "testcase" {
		t.Fatalf("unexpected judge_config: %#v", judgeConfig)
	}
}

// TestDecodeQuestionPipelineCardsResponseSupportsJSONLikeMultilineCodeCard 验证编程题即使以不完全合法的多行 JSON-like 形式返回，也能回退解析出核心字段。
func TestDecodeQuestionPipelineCardsResponseSupportsJSONLikeMultilineCodeCard(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelineCardsResponse(`
{
  "cards": [
    {
      "title": "模拟 Go Slice 动态扩容",
      "type": "code",
      "difficulty": "medium",
      "content": "实现一个 DynamicSlice，要求：
1. 支持 Append 自动扩容
2. 支持 Len 和 Cap 查询
3. 容量不足时按规则扩容",
      "answer": "package main

type DynamicSlice struct {}

func main() {
}
",
      "solution": "核心思路：
1. 记录 length 与 capacity
2. Append 时判断是否需要扩容
3. 扩容后复制旧数据"
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
	if card.Type != model.QuestionTypeCode {
		t.Fatalf("expected code type, got %#v", card)
	}
	if !strings.Contains(card.Content, "支持 Append 自动扩容") {
		t.Fatalf("expected multiline content to be preserved, got %#v", card.Content)
	}
	if !strings.Contains(card.Answer, "type DynamicSlice struct") {
		t.Fatalf("expected multiline answer to be preserved, got %#v", card.Answer)
	}
	if !strings.Contains(card.Solution, "Append 时判断是否需要扩容") {
		t.Fatalf("expected multiline solution to be preserved, got %#v", card.Solution)
	}
}

// TestDecodeQuestionPipelineCardsResponseSupportsYAMLLikeJudgeConfig 验证类 YAML 的编程题文本也能保留并解析 judge_config。
func TestDecodeQuestionPipelineCardsResponseSupportsYAMLLikeJudgeConfig(t *testing.T) {
	t.Parallel()

	payload, err := decodeQuestionPipelineCardsResponse(`
title:
两数之和
type: code
difficulty: easy
content:
给定两个整数，输出它们的和。
answer:
package main

import "fmt"

func main() {
    var a, b int
    fmt.Scan(&a, &b)
    fmt.Println(a + b)
}
solution:
先读取两个整数，再输出求和结果，注意标准输入输出格式。
judge_config:
evaluation_mode: testcase
default_language: go
allowed_languages: [go]
starter_code: |
package main

func main() {
}
public_test_cases:
- input: "1 2"
  expected_output: "3"
  description: "公开样例1"
- input: "2 3"
  expected_output: "5"
  description: "公开样例2"
- input: "10 20"
  expected_output: "30"
  description: "公开样例3"
hidden_test_cases:
- input: "100 200"
  expected_output: "300"
  description: "隐藏样例1"
reference_solutions:
- language: go
  title: "参考实现"
  code: |
    package main

    import "fmt"

    func main() {
        var a, b int
        fmt.Scan(&a, &b)
        fmt.Println(a + b)
    }
time_limit_ms: 2000
memory_limit_mb: 128
`)
	if err != nil {
		t.Fatalf("decodeQuestionPipelineCardsResponse returned error: %v", err)
	}
	if payload == nil || len(payload.Cards) != 1 {
		t.Fatalf("expected 1 card, got %#v", payload)
	}

	judgeConfig := parseQuestionJudgeConfigPayload(payload.Cards[0].JudgeConfig)
	if judgeConfig == nil {
		t.Fatalf("expected judge_config to be parsed, got %#v", payload.Cards[0].JudgeConfig)
	}
	if judgeConfig.DefaultLanguage != "go" {
		t.Fatalf("expected default language go, got %#v", judgeConfig)
	}
	if len(judgeConfig.PublicTestCases) != 3 {
		t.Fatalf("expected 3 public test cases, got %#v", judgeConfig.PublicTestCases)
	}
	if strings.TrimSpace(judgeConfig.StarterCode) == "" {
		t.Fatalf("expected starter code to be preserved, got %#v", judgeConfig)
	}
}

// TestParseQuestionJudgeConfigPayloadSupportsSolutionCodeAlias 验证判题配置会把 solution_code 别名归一化到 code 字段。
func TestParseQuestionJudgeConfigPayloadSupportsSolutionCodeAlias(t *testing.T) {
	t.Parallel()

	judgeConfig := parseQuestionJudgeConfigPayload(map[string]any{
		"evaluation_mode":  "testcase",
		"default_language": "go",
		"allowed_languages": []any{
			"go",
		},
		"public_test_cases": []any{
			map[string]any{"input": "1 2", "expected_output": "3"},
			map[string]any{"input": "2 3", "expected_output": "5"},
			map[string]any{"input": "10 20", "expected_output": "30"},
		},
		"hidden_test_cases": []any{
			map[string]any{"input": "100 200", "expected_output": "300"},
		},
		"reference_solutions": []any{
			map[string]any{"language": "go", "solution_code": "package main\n\nfunc main() {}\n"},
		},
	})
	if judgeConfig == nil {
		t.Fatal("expected judge_config to be parsed")
	}
	if len(judgeConfig.ReferenceAnswers) != 1 {
		t.Fatalf("expected 1 reference solution, got %#v", judgeConfig.ReferenceAnswers)
	}
	if !strings.Contains(judgeConfig.ReferenceAnswers[0].Code, "package main") {
		t.Fatalf("expected solution_code alias to be normalized, got %#v", judgeConfig.ReferenceAnswers[0])
	}
}

// TestBuildQuestionPipelinePreparedCodeModelCardBuildsJudgeSkeleton 验证半成品编程题会先补出本地 judge_config 骨架，避免在完整补齐前直接被丢弃。
func TestBuildQuestionPipelinePreparedCodeModelCardBuildsJudgeSkeleton(t *testing.T) {
	t.Parallel()

	prepared := buildQuestionPipelinePreparedCodeModelCard(questionPipelineModelCard{
		Title:    "两数之和",
		Content:  "请读取两个整数并输出它们的和。",
		Type:     model.QuestionTypeCode,
		Answer:   "package main\n\nfunc main() {\n}\n",
		Solution: "先读取输入，再输出求和结果。",
		Tags:     []string{"go", "输入输出"},
	})

	if prepared.Type != model.QuestionTypeCode {
		t.Fatalf("expected code type, got %#v", prepared)
	}
	judgeConfig, ok := prepared.JudgeConfig.(*QuestionJudgeConfig)
	if !ok || judgeConfig == nil {
		t.Fatalf("expected local judge skeleton, got %#v", prepared.JudgeConfig)
	}
	if judgeConfig.EvaluationMode != QuestionEvaluationModeTestcase {
		t.Fatalf("expected testcase mode, got %#v", judgeConfig)
	}
	if judgeConfig.DefaultLanguage != "go" {
		t.Fatalf("expected default language go, got %#v", judgeConfig)
	}
	if len(judgeConfig.ReferenceAnswers) != 1 {
		t.Fatalf("expected one reference solution, got %#v", judgeConfig.ReferenceAnswers)
	}
	if !strings.Contains(judgeConfig.ReferenceAnswers[0].Code, "package main") {
		t.Fatalf("expected answer code to be reused, got %#v", judgeConfig.ReferenceAnswers[0])
	}
}

// TestSanitizeQuestionPipelineModelCardTrimsEmbeddedJSONTail 验证题卡字段清洗会截断混入答案里的后续 JSON 块。
func TestSanitizeQuestionPipelineModelCardTrimsEmbeddedJSONTail(t *testing.T) {
	t.Parallel()

	card := sanitizeQuestionPipelineModelCard(questionPipelineModelCard{
		Title:    "两数之和",
		Type:     model.QuestionTypeCode,
		Answer:   "```go\nfunc sum(a int, b int) int {\n    return a + b\n}\n```\n```json\n{\"title\":\"污染块\"}",
		Solution: "先读取输入。\n```json\n{\"title\":\"污染块\"}",
	})
	if strings.Contains(card.Answer, "污染块") {
		t.Fatalf("expected embedded json tail to be removed from answer, got %q", card.Answer)
	}
	if strings.Contains(card.Solution, "污染块") {
		t.Fatalf("expected embedded json tail to be removed from solution, got %q", card.Solution)
	}
	if !strings.Contains(card.Answer, "func sum") {
		t.Fatalf("expected answer code to be retained, got %q", card.Answer)
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

// TestBuildQuestionPipelineConstraintProfileDetectsCodeRequirement 验证编程题需求会启用 code 题型硬约束。
func TestBuildQuestionPipelineConstraintProfileDetectsCodeRequirement(t *testing.T) {
	t.Parallel()

	profile := buildQuestionPipelineConstraintProfile(
		"生成 4 道 Go 编程题，要求包含测试用例。",
		"全部输出代码题，提供 judge_config。",
		4,
	)

	if !profile.RequireCode {
		t.Fatalf("expected RequireCode to be true, got %#v", profile)
	}
	if profile.RequireSubjective {
		t.Fatalf("expected RequireSubjective to be false when code requirement exists, got %#v", profile)
	}
}

// TestNormalizeQuestionPipelineTypeSupportsCodeAliases 验证编程题常见别名会被归一化为 code。
func TestNormalizeQuestionPipelineTypeSupportsCodeAliases(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"coding", "programming", "编程题", "算法题"} {
		if actual := normalizeQuestionPipelineType(input); actual != model.QuestionTypeCode {
			t.Fatalf("expected %q to normalize to code, got %q", input, actual)
		}
	}
}

// TestNormalizeQuestionPipelineGenerationMode 验证题目流水线生成模式已统一固定为逐张直生。
func TestNormalizeQuestionPipelineGenerationMode(t *testing.T) {
	t.Parallel()

	if mode := normalizeQuestionPipelineGenerationMode("direct_single"); mode != questionPipelineModeDirect {
		t.Fatalf("expected direct mode, got %s", mode)
	}
	if mode := normalizeQuestionPipelineGenerationMode("unexpected"); mode != questionPipelineModeDirect {
		t.Fatalf("expected direct mode fallback, got %s", mode)
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

// TestBuildQuestionPipelineRuntimeOverridesUsesLargerBudgetForCode 验证编程题链路会获得更大的输出预算与更长超时，降低结构化 JSON 被截断的概率。
func TestBuildQuestionPipelineRuntimeOverridesUsesLargerBudgetForCode(t *testing.T) {
	t.Parallel()

	compactNonCode := buildQuestionPipelineRuntimeOverrides(true, false)
	compactCode := buildQuestionPipelineRuntimeOverrides(true, true)
	batchCode := buildQuestionPipelineRuntimeOverrides(false, true)

	if compactNonCode[ai.ConfigKeyMaxTokens] != "1400" {
		t.Fatalf("expected non-code compact max tokens 1400, got %#v", compactNonCode)
	}
	if compactCode[ai.ConfigKeyMaxTokens] != "2600" {
		t.Fatalf("expected code compact max tokens 2600, got %#v", compactCode)
	}
	if compactCode[ai.ConfigKeyTimeoutSeconds] != "120" {
		t.Fatalf("expected code compact timeout 120, got %#v", compactCode)
	}
	if batchCode[ai.ConfigKeyMaxTokens] != "3200" {
		t.Fatalf("expected code batch max tokens 3200, got %#v", batchCode)
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

// TestEnforceQuestionPipelineCardConstraintsKeepsCodeCards 验证编程题硬约束不会再把 code 题卡覆盖回 subjective。
func TestEnforceQuestionPipelineCardConstraintsKeepsCodeCards(t *testing.T) {
	t.Parallel()

	cards, warnings := enforceQuestionPipelineCardConstraints([]AdminQuestionPipelineCard{
		{
			Title:    "两数之和",
			Content:  "给定两个整数，输出它们的和。",
			Type:     model.QuestionTypeSubjective,
			Answer:   "package main\n\nfunc main() {}",
			Solution: "先读取输入，再输出计算结果。",
			JudgeConfig: &QuestionJudgeConfig{
				EvaluationMode:   QuestionEvaluationModeTestcase,
				DefaultLanguage:  "go",
				AllowedLanguages: []string{"go"},
				PublicTestCases: []QuestionTestCase{
					{Input: "1 2", ExpectedOutput: "3"},
					{Input: "2 3", ExpectedOutput: "5"},
					{Input: "10 20", ExpectedOutput: "30"},
				},
				HiddenTestCases: []QuestionTestCase{
					{Input: "100 200", ExpectedOutput: "300"},
				},
				ReferenceAnswers: []QuestionReferenceSolution{
					{Language: "go", Code: "package main\n\nfunc main() {}"},
				},
			},
		},
	}, buildQuestionPipelineConstraintProfile("生成 Go 编程题，要求带测试用例。", "虽然页面默认提示词里提到问答题，也必须输出编程题。", 1), "生成 Go 编程题，要求带测试用例。", "虽然页面默认提示词里提到问答题，也必须输出编程题。")

	if len(cards) != 1 {
		t.Fatalf("expected 1 code card, got %#v", cards)
	}
	if cards[0].Type != model.QuestionTypeCode {
		t.Fatalf("expected code type to be retained, got %#v", cards[0])
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
}

// TestEnforceQuestionPipelineCardConstraintsReportsCodeDropReason 验证编程题被丢弃时会返回明确原因，便于前端调试定位。
func TestEnforceQuestionPipelineCardConstraintsReportsCodeDropReason(t *testing.T) {
	t.Parallel()

	cards, warnings := enforceQuestionPipelineCardConstraints([]AdminQuestionPipelineCard{
		{
			Title:    "两数之和",
			Content:  "给定两个整数，输出它们的和。",
			Type:     model.QuestionTypeCode,
			Answer:   "package main\n\nfunc main() {}",
			Solution: "先读取输入，再输出结果。",
		},
	}, questionPipelineConstraintProfile{
		CandidateCount: 1,
		RequireCode:    true,
	}, "生成 1 道 Go 编程题", "")

	if len(cards) != 0 {
		t.Fatalf("expected code card to be dropped, got %#v", cards)
	}
	if len(warnings) == 0 {
		t.Fatal("expected drop reason warning, got none")
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "缺少 judge_config") {
		t.Fatalf("expected judge_config warning, got %#v", warnings)
	}
}

// TestEnforceQuestionPipelineCardConstraintsDetectsLanguageFromJudgeConfig 验证编程题可从 judge_config 语言信息命中语言配额。
func TestEnforceQuestionPipelineCardConstraintsDetectsLanguageFromJudgeConfig(t *testing.T) {
	t.Parallel()

	cards, warnings := enforceQuestionPipelineCardConstraints([]AdminQuestionPipelineCard{
		{
			Title:    "两数之和",
			Content:  "给定两个整数，输出它们的和。",
			Type:     model.QuestionTypeCode,
			Answer:   "package main\n\nfunc main() {}",
			Solution: "先读取输入，再输出结果。",
			JudgeConfig: &QuestionJudgeConfig{
				EvaluationMode:   QuestionEvaluationModeTestcase,
				DefaultLanguage:  "go",
				AllowedLanguages: []string{"go"},
				PublicTestCases: []QuestionTestCase{
					{Input: "1 2", ExpectedOutput: "3"},
					{Input: "2 3", ExpectedOutput: "5"},
					{Input: "10 20", ExpectedOutput: "30"},
				},
				HiddenTestCases: []QuestionTestCase{
					{Input: "100 200", ExpectedOutput: "300"},
				},
				ReferenceAnswers: []QuestionReferenceSolution{
					{Language: "go", Code: "package main\n\nfunc main() {}"},
				},
			},
		},
	}, questionPipelineConstraintProfile{
		CandidateCount:    1,
		RequireCode:       true,
		RemainingLanguage: "go",
	}, "生成 1 道 Go 编程题", "")

	if len(cards) != 1 {
		t.Fatalf("expected 1 card after language detection, got %#v", cards)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
}

// TestValidateQuestionPipelineImportCardRequiresCodeStructure 验证编程题导入前必须具备思路解析与完整判题配置。
func TestValidateQuestionPipelineImportCardRequiresCodeStructure(t *testing.T) {
	t.Parallel()

	err := validateQuestionPipelineImportCard(AdminQuestionPipelineImportCard{
		Title:      "两数之和",
		Content:    "给定两个整数，输出它们的和。",
		Type:       model.QuestionTypeCode,
		Difficulty: model.QuestionDifficultyEasy,
		Category:   "算法",
		Answer:     "package main\n\nfunc main() {}",
		Solution:   "先读取输入，再输出结果。",
		JudgeConfig: &QuestionJudgeConfig{
			EvaluationMode:   QuestionEvaluationModeTestcase,
			DefaultLanguage:  "go",
			AllowedLanguages: []string{"go"},
			PublicTestCases: []QuestionTestCase{
				{Input: "1 2", ExpectedOutput: "3"},
				{Input: "2 3", ExpectedOutput: "5"},
				{Input: "10 20", ExpectedOutput: "30"},
			},
			HiddenTestCases: []QuestionTestCase{
				{Input: "100 200", ExpectedOutput: "300"},
			},
			ReferenceAnswers: []QuestionReferenceSolution{
				{Language: "go", Code: "package main\n\nfunc main() {}"},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected code card to pass validation, got %v", err)
	}

	err = validateQuestionPipelineImportCard(AdminQuestionPipelineImportCard{
		Title:      "两数之和",
		Content:    "给定两个整数，输出它们的和。",
		Type:       model.QuestionTypeCode,
		Difficulty: model.QuestionDifficultyEasy,
		Category:   "算法",
		Answer:     "package main\n\nfunc main() {}",
	})
	if err == nil || !strings.Contains(err.Error(), "代码思路解析") {
		t.Fatalf("expected missing solution error, got %v", err)
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
