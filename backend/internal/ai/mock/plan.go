package mock

import (
	"context"
	"fmt"

	"makejob-backend/internal/ai"
)

// MockPlanAgent Mock学习规划Agent实现
type MockPlanAgent struct {
	provider *MockProvider
}

// NewMockPlanAgent 创建Mock学习规划Agent
func NewMockPlanAgent(provider *MockProvider) *MockPlanAgent {
	return &MockPlanAgent{
		provider: provider,
	}
}

// GeneratePlan 生成学习计划
func (a *MockPlanAgent) GeneratePlan(ctx context.Context, profile ai.UserProfile, industryCode string) (ai.LearningPlan, error) {
	select {
	case <-ctx.Done():
		return ai.LearningPlan{}, ctx.Err()
	default:
	}

	// 根据用户水平调整计划难度
	plan := a.generateGoLearningPlan(profile)
	return plan, nil
}

// AdjustPlan 调整学习计划
func (a *MockPlanAgent) AdjustPlan(ctx context.Context, planID string, completedTasks []string, performance map[string]float64) (ai.LearningPlan, error) {
	select {
	case <-ctx.Done():
		return ai.LearningPlan{}, ctx.Err()
	default:
	}

	// 根据完成情况和表现生成调整后的计划
	// 简化实现：返回一个基于完成情况的调整计划
	adjustedPlan := ai.LearningPlan{
		Title:       "调整后：Go语言进阶学习计划",
		Description: fmt.Sprintf("根据你的学习进度（已完成%d个任务），为你调整后的个性化学习计划", len(completedTasks)),
		Duration:    14, // 调整为14天
		Tasks:       a.generateAdjustedTasks(completedTasks, performance),
	}

	return adjustedPlan, nil
}

// GetStudySuggestion 获取学习建议
func (a *MockPlanAgent) GetStudySuggestion(ctx context.Context, profile ai.UserProfile) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// 根据用户画像生成个性化建议
	var suggestion string

	switch profile.Level {
	case "beginner":
		suggestion = `作为初学者，建议你：
1. 先掌握Go语言基础语法，包括变量、数据类型、控制结构
2. 重点理解Go的并发模型：Goroutine和Channel
3. 多做练习题，每天至少完成3-5道基础题
4. 阅读官方文档和《Go程序设计语言》
5. 不要急于求成，基础打得越牢，后期进步越快`
	case "intermediate":
		suggestion = `作为中级学习者，建议你：
1. 深入学习Go运行时机制，包括GMP调度模型、GC原理
2. 掌握标准库的高级用法，尤其是net/http、context等
3. 学习常用设计模式在Go中的实现
4. 开始阅读优秀开源项目的源码
5. 尝试用Go完成一个小型实战项目`
	case "advanced":
		suggestion = `作为高级学习者，建议你：
1. 深入研究Go编译器和运行时源码
2. 学习性能优化技巧，掌握pprof等工具
3. 关注Go语言的新特性和发展方向
4. 参与开源项目贡献
5. 探索Go在微服务、云原生等领域的最佳实践`
	default:
		suggestion = "建议先评估自己的当前水平，然后制定针对性的学习计划。保持每天学习的习惯，循序渐进。"
	}

	// 根据弱项添加针对性建议
	if len(profile.WeakTopics) > 0 {
		suggestion += fmt.Sprintf("\n\n针对你的薄弱环节（%v），建议重点加强这些领域的学习和练习。", profile.WeakTopics)
	}

	return suggestion, nil
}

// generateGoLearningPlan 生成Go语言学习计划
func (a *MockPlanAgent) generateGoLearningPlan(profile ai.UserProfile) ai.LearningPlan {
	tasks := []ai.PlanTask{
		// Week 1: 基础阶段
		{
			Title:       "Go语言基础语法",
			Description: "学习变量、常量、数据类型、运算符、控制结构（if/for/switch）",
			TaskType:    "study",
			DayNumber:   1,
			Duration:    60,
			Priority:    "high",
		},
		{
			Title:       "函数与错误处理",
			Description: "掌握函数定义、多返回值、defer语句、panic/recover机制",
			TaskType:    "study",
			DayNumber:   2,
			Duration:    60,
			Priority:    "high",
		},
		{
			Title:       "基础语法练习",
			Description: "完成20道基础语法练习题，巩固前两天的学习内容",
			TaskType:    "practice",
			DayNumber:   3,
			Duration:    90,
			Priority:    "high",
		},
		{
			Title:       "结构体与方法",
			Description: "学习struct定义、方法接收者、嵌入类型等面向对象特性",
			TaskType:    "study",
			DayNumber:   4,
			Duration:    60,
			Priority:    "high",
		},
		{
			Title:       "接口与反射",
			Description: "深入理解interface的隐式实现、类型断言、反射机制",
			TaskType:    "study",
			DayNumber:   5,
			Duration:    75,
			Priority:    "medium",
		},
		{
			Title:       "第一周复习与测试",
			Description: "复习本周学习内容，完成单元测试检验掌握程度",
			TaskType:    "review",
			DayNumber:   6,
			Duration:    60,
			Priority:    "medium",
		},
		{
			Title:       "休息或自由学习",
			Description: "适当休息，或根据个人兴趣探索Go的其他特性",
			TaskType:    "review",
			DayNumber:   7,
			Duration:    30,
			Priority:    "low",
		},
		// Week 2: 并发编程
		{
			Title:       "Goroutine基础",
			Description: "学习Goroutine的创建、生命周期、与线程的区别",
			TaskType:    "study",
			DayNumber:   8,
			Duration:    75,
			Priority:    "high",
		},
		{
			Title:       "Channel通信",
			Description: "掌握Channel的创建、发送、接收、缓冲与无缓冲Channel的区别",
			TaskType:    "study",
			DayNumber:   9,
			Duration:    90,
			Priority:    "high",
		},
		{
			Title:       "并发编程练习",
			Description: "完成生产者消费者、并发爬虫等经典并发编程练习",
			TaskType:    "practice",
			DayNumber:   10,
			Duration:    120,
			Priority:    "high",
		},
		{
			Title:       "同步原语",
			Description: "学习sync包：Mutex、RWMutex、WaitGroup、Once、Pool",
			TaskType:    "study",
			DayNumber:   11,
			Duration:    75,
			Priority:    "medium",
		},
		{
			Title:       "Context包",
			Description: "掌握Context的使用：超时控制、取消信号、值传递",
			TaskType:    "study",
			DayNumber:   12,
			Duration:    60,
			Priority:    "medium",
		},
		{
			Title:       "第二周复习与测试",
			Description: "复习并发编程内容，完成相关测试题",
			TaskType:    "review",
			DayNumber:   13,
			Duration:    60,
			Priority:    "medium",
		},
		{
			Title:       "休息或自由学习",
			Description: "适当休息，阅读Go并发相关的博客或文章",
			TaskType:    "review",
			DayNumber:   14,
			Duration:    30,
			Priority:    "low",
		},
		// Week 3: 标准库与实战
		{
			Title:       "标准库：IO与文件操作",
			Description: "学习io、os、bufio、path/filepath等包的使用",
			TaskType:    "study",
			DayNumber:   15,
			Duration:    60,
			Priority:    "medium",
		},
		{
			Title:       "标准库：网络编程",
			Description: "掌握net、net/http包，编写简单的HTTP服务",
			TaskType:    "study",
			DayNumber:   16,
			Duration:    90,
			Priority:    "high",
		},
		{
			Title:       "项目实战：Web服务",
			Description: "使用 Gin 或标准库实现一个简单的RESTful API服务",
			TaskType:    "practice",
			DayNumber:   17,
			Duration:    150,
			Priority:    "high",
		},
		{
			Title:       "测试与调试",
			Description: "学习单元测试、基准测试、性能分析（pprof）",
			TaskType:    "study",
			DayNumber:   18,
			Duration:    75,
			Priority:    "medium",
		},
		{
			Title:       "刷题练习",
			Description: "在LeetCode或牛客网上完成10道Go语言算法题",
			TaskType:    "practice",
			DayNumber:   19,
			Duration:    120,
			Priority:    "high",
		},
		{
			Title:       "模拟面试",
			Description: "进行一次Go语言模拟面试，检验学习成果",
			TaskType:    "interview",
			DayNumber:   20,
			Duration:    60,
			Priority:    "high",
		},
		{
			Title:       "总结与规划",
			Description: "总结21天学习成果，制定后续进阶学习计划",
			TaskType:    "review",
			DayNumber:   21,
			Duration:    45,
			Priority:    "medium",
		},
	}

	return ai.LearningPlan{
		Title:       "21天Go语言系统学习计划",
		Description: fmt.Sprintf("根据你的水平（%s）定制的完整学习路线，涵盖基础语法、并发编程、标准库和项目实战", profile.Level),
		Duration:    21,
		Tasks:       tasks,
	}
}

// generateAdjustedTasks 生成调整后的任务列表
func (a *MockPlanAgent) generateAdjustedTasks(completedTasks []string, performance map[string]float64) []ai.PlanTask {
	// 简化实现：返回一些进阶任务
	return []ai.PlanTask{
		{
			Title:       "高级并发模式",
			Description: "学习Worker Pool、Pipeline、Fan-out/Fan-in等高级并发模式",
			TaskType:    "study",
			DayNumber:   1,
			Duration:    90,
			Priority:    "high",
		},
		{
			Title:       "Go内存模型",
			Description: "深入理解Go内存模型和happens-before关系",
			TaskType:    "study",
			DayNumber:   2,
			Duration:    75,
			Priority:    "medium",
		},
		{
			Title:       "性能优化实战",
			Description: "使用pprof进行性能分析，优化实际代码",
			TaskType:    "practice",
			DayNumber:   3,
			Duration:    120,
			Priority:    "high",
		},
		{
			Title:       "微服务架构",
			Description: "学习Go在微服务架构中的应用，了解gRPC、服务发现等",
			TaskType:    "study",
			DayNumber:   4,
			Duration:    90,
			Priority:    "medium",
		},
		{
			Title:       "项目实战：分布式系统",
			Description: "实现一个简单的分布式缓存或任务调度系统",
			TaskType:    "practice",
			DayNumber:   5,
			Duration:    180,
			Priority:    "high",
		},
		{
			Title:       "源码阅读",
			Description: "阅读Go标准库或优秀开源项目的源码",
			TaskType:    "study",
			DayNumber:   6,
			Duration:    90,
			Priority:    "medium",
		},
		{
			Title:       "技术分享准备",
			Description: "准备一个Go技术分享主题，输出博客或演讲",
			TaskType:    "review",
			DayNumber:   7,
			Duration:    120,
			Priority:    "low",
		},
	}
}
