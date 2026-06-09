package biz

import "strings"

// MistakeTopicCard 描述一个可被面试报告、练习反馈和推荐结果复用的错因专题卡片。
type MistakeTopicCard struct {
	Code                string   `json:"code"`
	Tag                 string   `json:"tag"`
	Title               string   `json:"title"`
	ProblemPattern      string   `json:"problem_pattern"`
	RootCauses          []string `json:"root_causes"`
	SelfCheckList       []string `json:"self_check_list"`
	PracticeDirections  []string `json:"practice_directions"`
	RecommendedActions  []string `json:"recommended_actions"`
	RelatedQuestionSets []string `json:"related_question_sets"`
}

// buildMistakeTopicCatalog 返回当前系统内置的第一批错因专题卡片。
func buildMistakeTopicCatalog() []MistakeTopicCard {
	return []MistakeTopicCard{
		{
			Code:                "state-definition",
			Tag:                 "状态定义不清",
			Title:               "状态定义不清",
			ProblemPattern:      "做题时能想到大方向，但变量意义、状态含义或递推关系没有先说清楚，导致实现越写越乱。",
			RootCauses:          []string{"没有先把输入输出与状态含义写明白。", "一上来就开始写代码，没有先拆主流程。", "对动态规划、搜索或复杂流程题缺少固定分析模板。"},
			SelfCheckList:       []string{"我能不能一句话说清每个状态代表什么？", "转移或推进时依赖的前置条件是否明确？", "如果换一个样例，我的状态定义还成立吗？"},
			PracticeDirections:  []string{"优先做需要先定义状态再推进的编程题。", "每次动手前先写 3 行伪代码说明状态和转移。"},
			RecommendedActions:  []string{"先口述状态定义，再开始写代码。", "把变量命名改成能反映语义的名称。"},
			RelatedQuestionSets: []string{"algorithm-structure", "go-runtime-core"},
		},
		{
			Code:                "edge-cases",
			Tag:                 "边界条件生疏",
			Title:               "边界条件生疏",
			ProblemPattern:      "主流程能写出来，但在空输入、单元素、重复值、首尾越界等边界场景上反复出错。",
			RootCauses:          []string{"只关注主路径，没有系统列出极端输入。", "循环终止条件、数组下标和左右边界检查不稳定。", "缺少做完后手动过样例的习惯。"},
			SelfCheckList:       []string{"空值、长度为 1、重复值、极大极小值是否都想过？", "首元素和尾元素是否会被漏处理？", "当前索引移动是否可能越界？"},
			PracticeDirections:  []string{"优先做数组、双指针、链表和字符串边界题。", "提交前固定手动过 3 组极端样例。"},
			RecommendedActions:  []string{"写完主流程后单独列一组边界样例再检查。", "把循环条件和下标访问分开验证。"},
			RelatedQuestionSets: []string{"algorithm-structure", "go-concurrency-debug"},
		},
		{
			Code:                "index-control",
			Tag:                 "循环/索引控制不稳",
			Title:               "循环/索引控制不稳",
			ProblemPattern:      "逻辑方向基本正确，但经常因为循环终止条件、指针移动或索引更新顺序不稳导致 bug。",
			RootCauses:          []string{"没有先明确每轮循环结束后各指针应该处于什么位置。", "多个索引变量同时移动时缺少一致规则。", "代码改动后没有重新验证不变量。"},
			SelfCheckList:       []string{"每一轮循环前后，索引分别表示什么？", "指针移动后是否还有机会跳过元素或重复处理元素？", "终止条件是否和移动规则一致？"},
			PracticeDirections:  []string{"优先做双指针、滑动窗口、链表遍历和矩阵遍历题。", "先写不变量注释再动手实现循环。"},
			RecommendedActions:  []string{"把复杂循环拆成更少变量的版本。", "在纸上先画出索引变化过程。"},
			RelatedQuestionSets: []string{"algorithm-structure"},
		},
		{
			Code:                "data-structure-choice",
			Tag:                 "数据结构选择不当",
			Title:               "数据结构选择不当",
			ProblemPattern:      "能写出代码，但一开始选错容器或结构，导致实现繁琐、复杂度不合理或逻辑绕。",
			RootCauses:          []string{"看到题目就直接实现，没有先评估查找、插入、删除、去重等核心操作。", "对哈希、栈、队列、链表、堆等使用场景不熟。", "不会根据约束倒推数据结构。"},
			SelfCheckList:       []string{"这题最频繁的操作是什么？", "是否存在更直接的数据结构能降低复杂度？", "当前实现是否在用结构弥补设计问题？"},
			PracticeDirections:  []string{"优先做哈希、栈队列、链表和缓存结构题。", "每题先写出'为什么选这个数据结构'。"},
			RecommendedActions:  []string{"先列出操作需求，再选结构。", "对比两种结构的时间复杂度后再决定。"},
			RelatedQuestionSets: []string{"algorithm-structure", "gorm-database-core"},
		},
		{
			Code:                "complexity-awareness",
			Tag:                 "复杂度意识薄弱",
			Title:               "复杂度意识薄弱",
			ProblemPattern:      "功能能实现，但没有主动优化复杂度，或在面试回答中说不清时间空间成本。",
			RootCauses:          []string{"解题时只关注能跑通，没有同步评估复杂度。", "不熟悉常见结构和算法的典型成本。", "不会从数据规模约束反推方案。"},
			SelfCheckList:       []string{"当前算法最深层循环是多少层？", "是否有重复扫描、重复分配或重复拷贝？", "如果数据量扩大十倍，这个方案还合理吗？"},
			PracticeDirections:  []string{"优先做哈希优化、双指针、缓存与并发性能相关题。", "每题结尾固定说一遍时间复杂度和空间复杂度。"},
			RecommendedActions:  []string{"把复杂度分析写成提交前的固定检查项。", "出现嵌套循环时先问自己能否降一层。"},
			RelatedQuestionSets: []string{"algorithm-structure", "gorm-database-core", "microservice-network"},
		},
		{
			Code:                "debug-chaos",
			Tag:                 "调试路径混乱",
			Title:               "调试路径混乱",
			ProblemPattern:      "出现错误后频繁试错，但没有明确定位顺序，导致修改很多次仍无法稳定收敛问题。",
			RootCauses:          []string{"没有先复现最小错误路径。", "修改代码时缺少假设和验证闭环。", "日志、打印、测试样例使用方式不稳定。"},
			SelfCheckList:       []string{"我现在是在验证哪一个假设？", "是否已经把问题缩小到最小复现？", "这次修改有没有对应的验证结果？"},
			PracticeDirections:  []string{"优先做并发排错、边界 bug 定位和实现不完整的代码修复题。", "练习用固定模板记录'现象-假设-验证-结论'。"},
			RecommendedActions:  []string{"每次只改一个变量并立即验证。", "先加最小日志，再决定是否改逻辑。"},
			RelatedQuestionSets: []string{"go-concurrency-debug", "gin-backend-flow"},
		},
		{
			Code:                "implementation-incomplete",
			Tag:                 "代码实现不完整",
			Title:               "代码实现不完整",
			ProblemPattern:      "思路方向大致正确，但代码只覆盖了部分路径，关键函数、返回值或异常场景没有补全。",
			RootCauses:          []string{"写代码过程中没有持续对照题目要求。", "主流程完成后就停止，没有补齐收尾逻辑。", "对函数输入输出契约关注不够。"},
			SelfCheckList:       []string{"题目要求的所有输出我都处理了吗？", "异常路径、返回值和收尾逻辑是否完整？", "当前代码是否只是半成品草稿？"},
			PracticeDirections:  []string{"优先做需要完整流程实现的编程题和工程题。", "写完后对照题目要求逐项勾选功能点。"},
			RecommendedActions:  []string{"先列功能清单，再逐项实现。", "提交前按输入输出契约做一次完整回看。"},
			RelatedQuestionSets: []string{"go-runtime-core", "gin-backend-flow", "gorm-database-core"},
		},
	}
}

// GetMistakeTopicByCode 根据专题编码定位专题卡片。
func GetMistakeTopicByCode(code string) (*MistakeTopicCard, bool) {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return nil, false
	}

	for _, topic := range buildMistakeTopicCatalog() {
		if topic.Code == trimmed {
			copy := topic
			return &copy, true
		}
	}

	return nil, false
}

// ListAllMistakeTopics 返回所有错因专题卡片。
func ListAllMistakeTopics() []MistakeTopicCard {
	return buildMistakeTopicCatalog()
}
