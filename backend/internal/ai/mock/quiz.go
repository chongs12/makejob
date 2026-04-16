package mock

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"makejob-backend/internal/ai"
)

// MockQuizAnalyzer Mock刷题分析Agent实现
type MockQuizAnalyzer struct {
	provider *MockProvider
}

// NewMockQuizAnalyzer 创建Mock刷题分析Agent
func NewMockQuizAnalyzer(provider *MockProvider) *MockQuizAnalyzer {
	return &MockQuizAnalyzer{
		provider: provider,
	}
}

// AnalyzeCode 分析代码
func (a *MockQuizAnalyzer) AnalyzeCode(ctx context.Context, code string, language string, question string) (ai.CodeAnalysis, error) {
	select {
	case <-ctx.Done():
		return ai.CodeAnalysis{}, ctx.Err()
	default:
	}

	// 基于代码特征进行模拟分析
	analysis := a.performMockAnalysis(code, language, question)
	return analysis, nil
}

// ExplainAnswer 解释答案
func (a *MockQuizAnalyzer) ExplainAnswer(ctx context.Context, questionTitle string, questionContent string, correctAnswer string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// 生成详细的答案解释
	explanation := fmt.Sprintf(`## %s 详细解析

### 题目回顾
%s

### 正确答案
%s

### 解题思路

这道题主要考察以下几个方面：

1. **核心概念理解**：首先需要理解题目涉及的基础概念和原理。

2. **算法设计**：分析问题的最优解法，考虑时间复杂度和空间复杂度的平衡。

3. **边界条件处理**：注意处理各种边界情况，如空输入、极端值等。

4. **代码实现**：将思路转化为正确、优雅的代码。

### 详细步骤

%s

### 复杂度分析

- **时间复杂度**：取决于具体实现，最优解通常为 O(n) 或 O(n log n)
- **空间复杂度**：通常为 O(1) 或 O(n)，视具体算法而定

### 相关知识点

- 数据结构基础
- 算法设计技巧
- 语言特性应用
- 性能优化思路

### 学习建议

建议你在理解这个解法的基础上，尝试：
1. 自己独立实现一遍
2. 思考是否有更优的解法
3. 总结这类问题的通用解题模式
4. 找一些类似的题目进行练习

加油，持续练习会让你越来越熟练！`, questionTitle, questionContent, correctAnswer, a.generateDetailedSteps(questionTitle))

	return explanation, nil
}

// GenerateHint 生成提示
func (a *MockQuizAnalyzer) GenerateHint(ctx context.Context, questionTitle string, questionContent string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// 根据题目内容生成渐进式提示
	hints := []string{
		"💡 **提示1**：先仔细阅读题目，理解输入输出的格式和要求。",
		"💡 **提示2**：考虑使用什么数据结构来存储中间结果？",
		"💡 **提示3**：注意边界条件的处理，比如空值或极端情况。",
		"💡 **提示4**：试着把大问题分解成几个小问题逐一解决。",
		"💡 **提示5**：如果卡住了，可以先写一个暴力解法，再考虑优化。",
	}

	// 根据题目特征添加特定提示
	specificHint := ""
	content := strings.ToLower(questionContent)

	if strings.Contains(content, "array") || strings.Contains(content, "数组") {
		specificHint = "💡 **专项提示**：这道题涉及数组操作，考虑双指针或滑动窗口技巧。"
	} else if strings.Contains(content, "tree") || strings.Contains(content, "树") {
		specificHint = "💡 **专项提示**：树的问题通常可以用递归或层次遍历来解决。"
	} else if strings.Contains(content, "linked list") || strings.Contains(content, "链表") {
		specificHint = "💡 **专项提示**：链表问题注意指针操作，考虑使用快慢指针。"
	} else if strings.Contains(content, "dynamic") || strings.Contains(content, "动态规划") {
		specificHint = "💡 **专项提示**：动态规划问题关键是找出状态转移方程。"
	}

	result := strings.Join(hints, "\n\n")
	if specificHint != "" {
		result = specificHint + "\n\n" + result
	}

	return result, nil
}

// performMockAnalysis 执行模拟代码分析
func (a *MockQuizAnalyzer) performMockAnalysis(code string, language string, question string) ai.CodeAnalysis {
	// 基于代码特征进行评分
	score := 70.0
	issues := []string{}
	improvements := []string{}

	codeLen := len(code)

	// 基于代码长度初步判断
	if codeLen < 20 {
		score -= 30
		issues = append(issues, "代码过于简短，可能未完整实现功能")
	} else if codeLen > 200 {
		score += 5
	}

	// 检查常见代码特征
	if strings.Contains(code, "//") || strings.Contains(code, "/*") {
		score += 5
	} else {
		improvements = append(improvements, "建议添加注释说明关键逻辑")
	}

	if strings.Contains(code, "error") || strings.Contains(code, "err") {
		score += 5
	} else {
		improvements = append(improvements, "建议添加错误处理逻辑")
	}

	// 检查代码结构
	if strings.Count(code, "func") > 0 {
		score += 5
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

	// 生成反馈
	var feedback string
	if score >= 85 {
		feedback = "代码质量很高！结构清晰，逻辑正确，体现了良好的编程习惯。"
	} else if score >= 70 {
		feedback = "代码整体不错，实现了基本功能，但还有一些可以优化的地方。"
	} else if score >= 60 {
		feedback = "代码基本正确，但存在一些问题需要改进。"
	} else {
		feedback = "代码存在较多问题，建议重新理解题目要求后再尝试。"
	}

	// 生成复杂度分析（基于题目类型模拟）
	timeComplexity, spaceComplexity := a.estimateComplexity(question, code)

	// 添加通用改进建议
	if len(improvements) == 0 {
		improvements = append(improvements,
			"可以考虑添加更多单元测试用例",
			"变量命名可以更具描述性",
			"部分逻辑可以进一步简化",
		)
	}

	return ai.CodeAnalysis{
		IsCorrect:       isCorrect,
		Score:           score,
		Feedback:        feedback,
		Issues:          issues,
		Improvements:    improvements,
		TimeComplexity:  timeComplexity,
		SpaceComplexity: spaceComplexity,
	}
}

// estimateComplexity 估算复杂度
func (a *MockQuizAnalyzer) estimateComplexity(question string, code string) (string, string) {
	question = strings.ToLower(question)
	code = strings.ToLower(code)

	// 根据题目和代码特征估算复杂度
	timeComplexity := "O(n)"
	spaceComplexity := "O(1)"

	if strings.Contains(question, "sort") || strings.Contains(question, "排序") {
		timeComplexity = "O(n log n)"
	}

	if strings.Contains(question, "nested") || strings.Contains(question, "双重循环") ||
		strings.Count(code, "for") >= 2 {
		timeComplexity = "O(n²)"
	}

	if strings.Contains(question, "binary") || strings.Contains(question, "二分") {
		timeComplexity = "O(log n)"
	}

	if strings.Contains(code, "map[") || strings.Contains(code, "slice") ||
		strings.Contains(code, "array") {
		spaceComplexity = "O(n)"
	}

	if strings.Contains(question, "recursion") || strings.Contains(question, "递归") {
		spaceComplexity = "O(h)" // h为递归深度
	}

	return timeComplexity, spaceComplexity
}

// generateDetailedSteps 生成详细步骤说明
func (a *MockQuizAnalyzer) generateDetailedSteps(questionTitle string) string {
	return `1. **理解问题**：仔细阅读题目，明确输入、输出和约束条件。

2. **分析示例**：通过示例理解问题的具体要求，找出规律。

3. **设计算法**：
   - 思考暴力解法
   - 分析暴力解法的时间/空间复杂度
   - 思考优化方向

4. **编码实现**：
   - 先写主流程框架
   - 处理边界条件
   - 添加辅助函数

5. **验证测试**：
   - 用示例验证正确性
   - 考虑边界情况
   - 分析最终复杂度`
}
