// Package model 提供数据模型定义
package model

import (
	"encoding/json"
	"fmt"

	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SeedData 插入种子数据
func SeedData(db *gorm.DB) error {
	applogger.Info("开始插入种子数据...")

	// 检查是否已有数据
	var count int64
	if err := db.Model(&Industry{}).Count(&count).Error; err != nil {
		return fmt.Errorf("检查行业数据失败: %w", err)
	}
	if count > 0 {
		applogger.Info("种子数据已存在，跳过插入")
		return nil
	}

	// 使用事务插入核心数据
	err := db.Transaction(func(tx *gorm.DB) error {
		// 1. 插入行业数据
		if err := seedIndustries(tx); err != nil {
			return err
		}

		// 2. 插入分类数据
		if err := seedCategories(tx); err != nil {
			return err
		}

		// 3. 插入题目数据
		if err := seedQuestions(tx); err != nil {
			return err
		}

		// 4. 插入管理配置
		if err := seedAdminConfigs(tx); err != nil {
			return err
		}

		// 5. 插入默认管理员账户
		if err := seedAdminUser(tx); err != nil {
			return err
		}

		applogger.Info("核心种子数据插入完成")
		return nil
	})
	if err != nil {
		return err
	}

	// Prompt模板单独插入（非关键，失败不影响启动）
	if err := seedPromptTemplates(db); err != nil {
		applogger.Warn("Prompt模板插入失败，跳过", zap.Error(err))
	}

	applogger.Info("种子数据插入完成")
	return nil
}

// seedIndustries 插入行业种子数据
func seedIndustries(db *gorm.DB) error {
	industries := []Industry{
		{
			Code:        "go",
			Name:        "Go语言面试",
			Description: "Go语言开发工程师面试备考",
			Icon:        "https://cdn.jsdelivr.net/gh/devicons/devicon/icons/go/go-original.svg",
			IsActive:    true,
			SortOrder:   1,
		},
		{
			Code:        "java",
			Name:        "Java面试",
			Description: "Java开发工程师面试备考",
			Icon:        "https://cdn.jsdelivr.net/gh/devicons/devicon/icons/java/java-original.svg",
			IsActive:    true,
			SortOrder:   2,
		},
		{
			Code:        "frontend",
			Name:        "前端面试",
			Description: "前端开发工程师面试备考",
			Icon:        "https://cdn.jsdelivr.net/gh/devicons/devicon/icons/javascript/javascript-original.svg",
			IsActive:    true,
			SortOrder:   3,
		},
	}

	for _, industry := range industries {
		if err := db.Create(&industry).Error; err != nil {
			return fmt.Errorf("插入行业数据失败: %w", err)
		}
	}
	applogger.Info("行业种子数据插入完成")
	return nil
}

// seedCategories 插入分类种子数据
func seedCategories(db *gorm.DB) error {
	// 获取Go语言行业ID
	var goIndustry Industry
	if err := db.Where("code = ?", "go").First(&goIndustry).Error; err != nil {
		return fmt.Errorf("获取Go行业失败: %w", err)
	}

	categories := []Category{
		{IndustryID: goIndustry.ID, Name: "Go基础语法", SortOrder: 1, Description: "Go语言基础语法知识"},
		{IndustryID: goIndustry.ID, Name: "并发编程", SortOrder: 2, Description: "Goroutine、Channel、同步原语等并发知识"},
		{IndustryID: goIndustry.ID, Name: "Web框架(Gin/Echo)", SortOrder: 3, Description: "常用Web框架原理与实践"},
		{IndustryID: goIndustry.ID, Name: "数据库操作(GORM/SQL)", SortOrder: 4, Description: "数据库操作与ORM使用"},
		{IndustryID: goIndustry.ID, Name: "微服务架构", SortOrder: 5, Description: "微服务设计与实现"},
		{IndustryID: goIndustry.ID, Name: "网络与协议", SortOrder: 6, Description: "网络编程与协议实现"},
		{IndustryID: goIndustry.ID, Name: "数据结构与算法", SortOrder: 7, Description: "常用数据结构与算法"},
		{IndustryID: goIndustry.ID, Name: "项目实战与设计模式", SortOrder: 8, Description: "设计模式与项目最佳实践"},
	}

	for _, category := range categories {
		if err := db.Create(&category).Error; err != nil {
			return fmt.Errorf("插入分类数据失败: %w", err)
		}
	}
	applogger.Info("分类种子数据插入完成")
	return nil
}

// seedQuestions 插入题目种子数据
func seedQuestions(db *gorm.DB) error {
	// 获取分类ID映射
	var categories []Category
	if err := db.Find(&categories).Error; err != nil {
		return fmt.Errorf("获取分类失败: %w", err)
	}

	categoryMap := make(map[string]uint)
	for _, c := range categories {
		categoryMap[c.Name] = c.ID
	}

	var goIndustry Industry
	if err := db.Where("code = ?", "go").First(&goIndustry).Error; err != nil {
		return fmt.Errorf("获取Go行业失败: %w", err)
	}

	questions := getGoQuestions(categoryMap, goIndustry.ID)

	for _, question := range questions {
		if err := db.Create(&question).Error; err != nil {
			return fmt.Errorf("插入题目数据失败: %w", err)
		}
	}
	applogger.Info("题目种子数据插入完成")
	return nil
}

// getGoQuestions 获取Go语言面试题
func getGoQuestions(categoryMap map[string]uint, industryID uint) []Question {
	return []Question{
		// ========== Go基础语法 (8题) ==========
		{
			CategoryID:  categoryMap["Go基础语法"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyEasy,
			Title:       "slice和array的根本区别是什么？",
			Content:     "在Go语言中，slice和array的根本区别是什么？",
			OptionsJSON: mustMarshal([]Option{{"A", "slice是值类型，array是引用类型"}, {"B", "array长度固定，slice长度可变"}, {"C", "slice只能存储基本类型"}, {"D", "没有区别，可以互换使用"}}),
			Answer:      "B",
			Explanation: "array是值类型，长度固定，在声明时确定；slice是引用类型，是对底层array的封装，长度可变，支持动态扩容。slice包含指向底层array的指针、长度和容量三个字段。",
			Tags:        "基础,slice,array",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["Go基础语法"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "以下代码的输出是什么？",
			Content:     "func main() { defer fmt.Println(1); defer fmt.Println(2); fmt.Println(3) }",
			OptionsJSON: mustMarshal([]Option{{"A", "1 2 3"}, {"B", "3 2 1"}, {"C", "3 1 2"}, {"D", "1 3 2"}}),
			Answer:      "B",
			Explanation: "defer语句采用LIFO(后进先出)的顺序执行。代码中先注册defer fmt.Println(1)，再注册defer fmt.Println(2)，所以执行顺序是2先于1。最终输出顺序是：3(普通语句) -> 2(后注册的defer) -> 1(先注册的defer)。",
			Tags:        "defer,执行顺序",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["Go基础语法"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyHard,
			Title:       "关于interface的底层实现，以下说法正确的是？",
			Content:     "Go语言中interface的底层实现是怎样的？",
			OptionsJSON: mustMarshal([]Option{{"A", "interface是一个指针，直接指向具体类型"}, {"B", "interface包含类型信息和数据指针两个字段"}, {"C", "interface存储的是值的副本，与原始值无关"}, {"D", "interface在编译期就确定了具体类型"}}),
			Answer:      "B",
			Explanation: "Go的interface底层由两个字段组成：itab(类型信息，包含类型元数据和方法表)和data(指向实际数据的指针)。当值赋值给interface时，会复制值并存储指针。这种设计实现了运行时多态。",
			Tags:        "interface,底层实现,原理",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["Go基础语法"],
			IndustryID:  industryID,
			Type:        QuestionTypeMulti,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "关于map的并发安全，以下说法正确的是？",
			Content:     "关于Go语言map的并发安全性，以下哪些说法是正确的？(多选)",
			OptionsJSON: mustMarshal([]Option{{"A", "map不是并发安全的，多个goroutine同时读写会panic"}, {"B", "可以使用sync.RWMutex保护map实现并发安全"}, {"C", "Go 1.9+可以使用sync.Map替代，它是并发安全的"}, {"D", "使用channel传递map可以保证并发安全"}}),
			Answer:      "A,B,C",
			Explanation: "A正确：原生map不是并发安全的，并发读写会导致runtime panic。B正确：使用互斥锁是最常见的保护方式。C正确：sync.Map是官方提供的并发安全map实现。D不完全正确：channel本身可以保证传递的安全性，但接收方如果并发访问map仍需要同步机制。",
			Tags:        "map,并发安全,sync",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["Go基础语法"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "解释Go中的值传递和引用传递",
			Content:     "请详细解释Go语言中的参数传递机制，包括值传递和引用传递的区别，并举例说明哪些类型是值传递，哪些是引用传递。",
			Answer:      "Go语言中只有值传递，没有引用传递。但传递的值可能是指针（地址）。\n\n值传递类型（传递副本）：\n- 基本类型：int, float, bool, string\n- 数组array\n- 结构体struct\n\n引用类型（传递指针/底层结构）：\n- slice：包含指向底层array的指针\n- map：指向hmap结构的指针\n- channel：指向hchan结构的指针\n- interface：包含类型信息和数据指针\n- function：函数指针\n\n示例代码展示了slice和map的修改行为差异。",
			Explanation: "理解Go的传递机制对避免bug很重要。虽然Go只有值传递，但引用类型的值本身包含指针，所以可以修改底层数据。",
			Tags:        "值传递,引用传递,基础概念",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["Go基础语法"],
			IndustryID:  industryID,
			Type:        QuestionTypeCode,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "实现一个线程安全的单例模式",
			Content:     "请使用Go语言实现一个线程安全的单例模式（Singleton），要求使用sync.Once确保只初始化一次。",
			Answer:      "package singleton\n\nimport \"sync\"\n\ntype Singleton struct {\n    data string\n}\n\nvar (\n    instance *Singleton\n    once     sync.Once\n)\n\nfunc GetInstance() *Singleton {\n    once.Do(func() {\n        instance = &Singleton{data: \"initialized\"}\n    })\n    return instance\n}",
			Explanation: "使用sync.Once是实现单例模式的最佳实践。它内部使用原子操作和互斥锁，保证Do方法中的函数只执行一次，且是线程安全的。相比使用sync.Mutex手动加锁，sync.Once更简洁高效。",
			Tags:        "单例模式,设计模式,sync.Once",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["Go基础语法"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyEasy,
			Title:       "make和new的区别是什么？",
			Content:     "Go语言中make和new关键字的主要区别是什么？",
			OptionsJSON: mustMarshal([]Option{{"A", "make分配内存并初始化，new只分配零值内存"}, {"B", "new用于引用类型，make用于值类型"}, {"C", "make返回指针，new返回值"}, {"D", "两者完全相同，可以互换"}}),
			Answer:      "A",
			Explanation: "make用于slice、map、channel的初始化，返回的是类型本身（不是指针），并会进行内部数据结构初始化。new用于任意类型的内存分配，返回的是*T指针，且内存被零值初始化。",
			Tags:        "make,new,内存分配",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["Go基础语法"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyHard,
			Title:       "解释Go的GMP调度模型",
			Content:     "请详细解释Go语言的GMP调度模型，包括G、M、P分别代表什么，以及它们之间的协作关系。",
			Answer:      "GMP模型组成：\n\n- G (Goroutine)：协程，包含栈、指令指针、等待队列等。G很轻量，初始栈2KB，可动态伸缩。\n- M (Machine)：操作系统线程，由操作系统管理，负责执行G的指令。\n- P (Processor)：逻辑处理器，维护G的本地队列，数量由GOMAXPROCS决定。\n\n调度流程：\n1. 创建G时，尝试放入当前P的本地队列\n2. P的本地队列满时，放入全局队列\n3. M需要绑定P才能执行G\n4. 当P的本地队列为空，从全局队列或其他P偷取G（work stealing）\n\n优势：\n- 减少线程切换开销（M可以复用）\n- 避免全局锁竞争（每个P有本地队列）\n- 实现work stealing负载均衡",
			Explanation: "GMP是Go调度器的核心设计，理解它有助于写出高性能的并发程序，比如合理设置GOMAXPROCS、避免阻塞M等。",
			Tags:        "GMP,调度器,goroutine,原理",
			IsActive:    true,
		},

		// ========== 并发编程 (8题) ==========
		{
			CategoryID:  categoryMap["并发编程"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "channel的默认行为是什么？",
			Content:     "关于无缓冲channel的操作，以下说法正确的是？",
			OptionsJSON: mustMarshal([]Option{{"A", "发送和接收都是非阻塞的"}, {"B", "发送会阻塞直到有接收者，接收会阻塞直到有发送者"}, {"C", "只有接收会阻塞"}, {"D", "channel操作永远不会阻塞"}}),
			Answer:      "B",
			Explanation: "无缓冲channel用于同步通信，发送操作会阻塞直到有goroutine接收，接收操作会阻塞直到有goroutine发送。这种特性常用于goroutine间的同步。",
			Tags:        "channel,阻塞,同步",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["并发编程"],
			IndustryID:  industryID,
			Type:        QuestionTypeMulti,
			Difficulty:  QuestionDifficultyHard,
			Title:       "sync.Mutex和sync.RWMutex的区别",
			Content:     "关于sync.Mutex和sync.RWMutex，以下说法正确的是？(多选)",
			OptionsJSON: mustMarshal([]Option{{"A", "Mutex是互斥锁，同一时刻只能有一个goroutine持有"}, {"B", "RWMutex允许多个读操作并发执行"}, {"C", "RWMutex的写锁会阻塞所有读锁和写锁"}, {"D", "Mutex的性能总是优于RWMutex"}}),
			Answer:      "A,B,C",
			Explanation: "A正确：Mutex提供互斥访问。B正确：RWMutex的RLock允许多个goroutine同时读。C正确：Lock会阻塞所有其他锁。D错误：在读多写少场景，RWMutex性能更好；但读写均衡或写多读少时，Mutex更简单高效。",
			Tags:        "Mutex,RWMutex,锁,同步",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["并发编程"],
			IndustryID:  industryID,
			Type:        QuestionTypeCode,
			Difficulty:  QuestionDifficultyHard,
			Title:       "实现一个带超时的Worker Pool",
			Content:     "请实现一个Worker Pool，支持：1) 指定worker数量 2) 任务队列 3) 优雅关闭 4) 单个任务超时控制",
			Answer:      "type WorkerPool struct {\n    workers   int\n    jobQueue  chan Job\n    wg        sync.WaitGroup\n    ctx       context.Context\n    cancel    context.CancelFunc\n}\n\ntype Job struct {\n    ID      int\n    Handler func() error\n    Timeout time.Duration\n}\n\nfunc NewWorkerPool(workers, queueSize int) *WorkerPool {\n    ctx, cancel := context.WithCancel(context.Background())\n    return &WorkerPool{\n        workers:  workers,\n        jobQueue: make(chan Job, queueSize),\n        ctx:      ctx,\n        cancel:   cancel,\n    }\n}\n\nfunc (p *WorkerPool) Start() {\n    for i := 0; i < p.workers; i++ {\n        p.wg.Add(1)\n        go p.worker(i)\n    }\n}\n\nfunc (p *WorkerPool) worker(id int) {\n    defer p.wg.Done()\n    for {\n        select {\n        case job := <-p.jobQueue:\n            ctx, cancel := context.WithTimeout(p.ctx, job.Timeout)\n            done := make(chan error, 1)\n            go func() {\n                done <- job.Handler()\n            }()\n            select {\n            case err := <-done:\n                if err != nil {\n                    log.Printf(\"Job %d error: %v\", job.ID, err)\n                }\n            case <-ctx.Done():\n                log.Printf(\"Job %d timeout\", job.ID)\n            }\n            cancel()\n        case <-p.ctx.Done():\n            return\n        }\n    }\n}\n\nfunc (p *WorkerPool) Submit(job Job) {\n    select {\n    case p.jobQueue <- job:\n    case <-p.ctx.Done():\n    }\n}\n\nfunc (p *WorkerPool) Stop() {\n    p.cancel()\n    p.wg.Wait()\n}",
			Explanation: "Worker Pool是控制并发数量的常用模式。使用context实现超时和取消，使用channel作为任务队列，使用sync.WaitGroup等待所有worker完成。",
			Tags:        "WorkerPool,并发模式,context",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["并发编程"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "select语句的case执行规则",
			Content:     "当select的多个case同时就绪时，执行哪个case？",
			OptionsJSON: mustMarshal([]Option{{"A", "按case书写顺序执行第一个"}, {"B", "随机选择一个执行"}, {"C", "同时执行所有case"}, {"D", "都不执行，进入default"}}),
			Answer:      "B",
			Explanation: "当多个case同时就绪时，select会随机公平地选择一个执行。这是为了避免饥饿问题，确保每个channel都有机会被处理。",
			Tags:        "select,channel,并发",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["并发编程"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyHard,
			Title:       "context的使用场景和原理",
			Content:     "请解释context包的主要使用场景和实现原理，包括取消信号、超时控制和值传递。",
			Answer:      "主要使用场景：\n\n1. 取消信号传递：当父任务取消时，通知所有子goroutine停止\n2. 超时控制：设置deadline，超时自动取消\n3. 值传递：在请求链路上传递元数据（如traceID、userID）\n\n实现原理：\n- context是接口，emptyCtx是最基础的实现\n- cancelCtx内嵌Context，维护children map和done channel\n- timerCtx内嵌cancelCtx，添加计时器实现超时\n- valueCtx内嵌Context，以链表形式存储key-value\n\n最佳实践：\n- 函数第一个参数传入context\n- 不要存储在struct中（除非作为字段传递）\n- 及时调用cancel避免goroutine泄漏\n- 用WithValue传递请求相关元数据，不要用做可选参数",
			Explanation: "context是Go并发编程的核心工具，理解其树形结构和取消传播机制对编写健壮的并发程序至关重要。",
			Tags:        "context,取消,超时,并发",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["并发编程"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyEasy,
			Title:       "关闭已关闭的channel会怎样？",
			Content:     "在Go中，重复关闭一个已经关闭的channel会发生什么？",
			OptionsJSON: mustMarshal([]Option{{"A", "没有任何影响"}, {"B", "编译错误"}, {"C", "运行时panic"}, {"D", "返回false"}}),
			Answer:      "C",
			Explanation: "关闭已经关闭的channel会导致runtime panic。向已关闭的channel发送数据也会panic。从已关闭的channel接收会立即返回零值，第二个返回值为false。",
			Tags:        "channel,panic,并发",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["并发编程"],
			IndustryID:  industryID,
			Type:        QuestionTypeCode,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "实现一个并发安全的计数器",
			Content:     "请实现一个支持并发访问的计数器，包含Add(delta int)和Value() int方法。",
			Answer:      "type Counter struct {\n    mu    sync.RWMutex\n    value int64\n}\n\nfunc (c *Counter) Add(delta int64) {\n    c.mu.Lock()\n    defer c.mu.Unlock()\n    c.value += delta\n}\n\nfunc (c *Counter) Value() int64 {\n    c.mu.RLock()\n    defer c.mu.RUnlock()\n    return c.value\n}\n\n// 使用atomic的版本（更高性能）\ntype AtomicCounter struct {\n    value int64\n}\n\nfunc (c *AtomicCounter) Add(delta int64) {\n    atomic.AddInt64(&c.value, delta)\n}\n\nfunc (c *AtomicCounter) Value() int64 {\n    return atomic.LoadInt64(&c.value)\n}",
			Explanation: "提供了两种实现：1) sync.RWMutex版本适合复杂操作；2) atomic版本适合简单的数值操作，性能更好（无锁）。atomic操作是CPU指令级别的原子操作，比互斥锁更高效。",
			Tags:        "并发安全,计数器,atomic,mutex",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["并发编程"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "goroutine泄漏的常见原因和检测方法",
			Content:     "请列举goroutine泄漏的常见原因，以及如何检测和避免。",
			Answer:      "常见泄漏原因：\n\n1. channel阻塞：goroutine向无缓冲channel发送/接收但无对应操作\n2. 无限循环：没有退出条件的for循环\n3. 锁未释放：获取锁后panic或提前return未释放\n4. WaitGroup误用：Add和Done次数不匹配\n5. 未关闭的timer/ticker：time.After或time.Tick产生的goroutine\n\n检测方法：\n\n1. runtime.NumGoroutine()：监控goroutine数量变化\n2. pprof：使用net/http/pprof查看goroutine堆栈\n3. go.uber.org/goleak：单元测试检测泄漏\n4. 日志追踪：在goroutine入口/出口打印日志\n\n避免措施：\n- 使用context控制生命周期\n- 确保channel操作有超时\n- 使用select监听done channel\n- 及时关闭不再使用的资源",
			Explanation: "goroutine泄漏会导致内存持续增长，严重时会耗尽系统资源。良好的编码习惯和监控机制是预防的关键。",
			Tags:        "goroutine泄漏,内存,调试",
			IsActive:    true,
		},

		// ========== Web框架 (6题) ==========
		{
			CategoryID:  categoryMap["Web框架(Gin/Echo)"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "Gin中间件的执行顺序",
			Content:     "在Gin框架中，中间件的执行顺序是怎样的？",
			OptionsJSON: mustMarshal([]Option{{"A", "按照注册顺序正向执行"}, {"B", "按照注册顺序正向执行，逆序执行后续逻辑"}, {"C", "随机执行"}, {"D", "只执行第一个中间件"}}),
			Answer:      "B",
			Explanation: "Gin中间件采用洋葱模型（Onion Model）。请求进来时按注册顺序执行中间件的前置逻辑，到达handler后，响应返回时按逆序执行中间件的后续逻辑（c.Next()之后的代码）。",
			Tags:        "Gin,中间件,执行顺序",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["Web框架(Gin/Echo)"],
			IndustryID:  industryID,
			Type:        QuestionTypeCode,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "实现JWT认证中间件",
			Content:     "请使用Gin框架实现一个JWT认证中间件，验证请求头中的Authorization Bearer Token。",
			Answer:      "func JWTAuthMiddleware(secret string) gin.HandlerFunc {\n    return func(c *gin.Context) {\n        authHeader := c.GetHeader(\"Authorization\")\n        if authHeader == \"\" {\n            c.JSON(401, gin.H{\"error\": \"missing authorization header\"})\n            c.Abort()\n            return\n        }\n\n        parts := strings.SplitN(authHeader, \" \", 2)\n        if len(parts) != 2 || parts[0] != \"Bearer\" {\n            c.JSON(401, gin.H{\"error\": \"invalid authorization format\"})\n            c.Abort()\n            return\n        }\n\n        token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {\n            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {\n                return nil, fmt.Errorf(\"unexpected signing method\")\n            }\n            return []byte(secret), nil\n        })\n\n        if err != nil || !token.Valid {\n            c.JSON(401, gin.H{\"error\": \"invalid token\"})\n            c.Abort()\n            return\n        }\n\n        if claims, ok := token.Claims.(jwt.MapClaims); ok {\n            c.Set(\"user_id\", claims[\"user_id\"])\n            c.Set(\"username\", claims[\"username\"])\n        }\n\n        c.Next()\n    }\n}",
			Explanation: "JWT中间件需要解析Authorization头，验证token签名和有效性，并将claims信息存入context供后续handler使用。使用c.Abort()阻止未认证请求继续执行。",
			Tags:        "Gin,JWT,中间件,认证",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["Web框架(Gin/Echo)"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "Gin的路由分组和参数绑定",
			Content:     "请解释Gin的路由分组功能，以及常用的参数绑定方式（JSON、Query、URI、Form）。",
			Answer:      "路由分组示例展示了如何组织路由。分组可以共享中间件，简化路由定义。\n\n参数绑定方式：\n\n1. JSON绑定：c.ShouldBindJSON(&obj) - 解析请求体JSON\n2. Query绑定：c.ShouldBindQuery(&obj) - 解析URL查询参数\n3. URI绑定：c.ShouldBindUri(&obj) - 解析路径参数 /users/:id\n4. Form绑定：c.ShouldBind(&obj) - 解析form-data或x-www-form-urlencoded\n\n最佳实践：\n- 使用struct tag定义绑定规则：json:\"name\" binding:\"required\"\n- 自定义验证器使用validate tag\n- 绑定失败返回400错误",
			Explanation: "Gin的参数绑定功能强大且灵活，合理使用可以简化参数校验逻辑，提高开发效率。",
			Tags:        "Gin,路由,参数绑定",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["Web框架(Gin/Echo)"],
			IndustryID:  industryID,
			Type:        QuestionTypeCode,
			Difficulty:  QuestionDifficultyHard,
			Title:       "实现统一错误处理和响应封装",
			Content:     "请实现Gin的统一错误处理中间件和API响应封装，支持标准错误码和多语言错误消息。",
			Answer:      "// 统一响应结构\ntype Response struct {\n    Code    int         ` + \"`\" + `json:\"code\"` + \"`\" + `\n    Message string      ` + \"`\" + `json:\"message\"` + \"`\" + `\n    Data    interface{} ` + \"`\" + `json:\"data,omitempty\"` + \"`\" + `\n}\n\n// 错误码定义\nconst (\n    CodeSuccess     = 0\n    CodeBadRequest  = 400\n    CodeUnauthorized = 401\n    CodeNotFound    = 404\n    CodeInternal    = 500\n)\n\n// 全局错误处理中间件\nfunc ErrorHandler() gin.HandlerFunc {\n    return func(c *gin.Context) {\n        c.Next()\n        \n        if len(c.Errors) > 0 {\n            err := c.Errors.Last()\n            var code int\n            var message string\n            \n            switch e := err.Err.(type) {\n            case *AppError:\n                code = e.Code\n                message = e.Message\n            default:\n                code = CodeInternal\n                message = \"internal server error\"\n            }\n            \n            c.JSON(http.StatusOK, Response{\n                Code:    code,\n                Message: message,\n            })\n        }\n    }\n}\n\n// 自定义错误类型\ntype AppError struct {\n    Code    int\n    Message string\n}\n\nfunc (e *AppError) Error() string {\n    return e.Message\n}\n\n// 辅助函数\nfunc Success(c *gin.Context, data interface{}) {\n    c.JSON(http.StatusOK, Response{\n        Code:    CodeSuccess,\n        Message: \"success\",\n        Data:    data,\n    })\n}\n\nfunc Fail(c *gin.Context, code int, message string) {\n    c.Error(&AppError{Code: code, Message: message})\n}",
			Explanation: "统一的错误处理和响应封装是API设计的基础。使用gin的Error机制收集错误，在中间件中统一处理，可以简化handler代码，保证响应格式一致性。",
			Tags:        "Gin,错误处理,响应封装",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["Web框架(Gin/Echo)"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyEasy,
			Title:       "Gin的Context是并发安全的吗？",
			Content:     "关于gin.Context的并发安全性，以下说法正确的是？",
			OptionsJSON: mustMarshal([]Option{{"A", "是并发安全的，可以在多个goroutine中使用"}, {"B", "不是并发安全的，不应在多个goroutine中使用"}, {"C", "只有Get/Set方法是并发安全的"}, {"D", "取决于Gin的版本"}}),
			Answer:      "B",
			Explanation: "gin.Context不是并发安全的。它包含请求相关的状态和数据，不应该在多个goroutine中同时使用。如果需要在goroutine中使用context的数据，应该在启动goroutine前复制所需数据。",
			Tags:        "Gin,Context,并发安全",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["Web框架(Gin/Echo)"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyHard,
			Title:       "Gin vs Echo框架对比",
			Content:     "请对比Gin和Echo两个Go Web框架的特点、性能差异和适用场景。",
			Answer:      "Gin特点：\n- 使用httprouter，路由性能极高\n- 中间件设计灵活，支持分组\n- 丰富的参数绑定和验证功能\n- 社区活跃，生态完善\n- 使用sync.Pool优化内存分配\n\nEcho特点：\n- 设计更现代，API更简洁\n- 内置更多功能（验证、日志、CORS等）\n- 支持自动TLS\n- 路由也基于radix tree，性能优秀\n- 中间件返回值可链式处理\n\n性能对比：\n- 两者性能都非常优秀，差距不大\n- Gin在极端场景下略胜一筹\n- Echo的内存使用可能略高\n\n适用场景：\n- Gin：大型项目，需要丰富生态和中间件\n- Echo：中小型项目，追求简洁现代API\n- 两者都是生产级框架，选择主要取决于团队偏好",
			Explanation: "Gin和Echo都是优秀的Go Web框架，理解它们的差异有助于根据项目需求做出合适选择。",
			Tags:        "Gin,Echo,框架对比",
			IsActive:    true,
		},

		// ========== 数据库操作 (6题) ==========
		{
			CategoryID:  categoryMap["数据库操作(GORM/SQL)"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "GORM的预加载是什么？",
			Content:     "GORM中的Preload和Joins预加载有什么区别？",
			OptionsJSON: mustMarshal([]Option{{"A", "Preload使用JOIN，Joins使用多条查询"}, {"B", "Preload使用多条查询，Joins使用JOIN"}, {"C", "两者完全相同"}, {"D", "Preload只支持一对一关系"}}),
			Answer:      "B",
			Explanation: "Preload使用单独查询加载关联数据（N+1问题优化版，实际是先查主表再批量IN查询关联表），Joins使用SQL JOIN一次性查询。Preload适合关联数据需要独立处理的场景，Joins适合需要关联表筛选的场景。",
			Tags:        "GORM,预加载,Preload,Joins",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["数据库操作(GORM/SQL)"],
			IndustryID:  industryID,
			Type:        QuestionTypeCode,
			Difficulty:  QuestionDifficultyHard,
			Title:       "GORM事务处理的最佳实践",
			Content:     "请展示GORM事务处理的几种方式，包括嵌套事务和手动事务。",
			Answer:      "// 方式1：自动事务（推荐）\nfunc Transfer(db *gorm.DB, from, to int, amount float64) error {\n    return db.Transaction(func(tx *gorm.DB) error {\n        if err := tx.Model(&Account{}).Where(\"id = ?\", from).Update(\"balance\", gorm.Expr(\"balance - ?\", amount)).Error; err != nil {\n            return err\n        }\n        if err := tx.Model(&Account{}).Where(\"id = ?\", to).Update(\"balance\", gorm.Expr(\"balance + ?\", amount)).Error; err != nil {\n            return err\n        }\n        return nil\n    })\n}\n\n// 方式2：手动事务\nfunc ManualTransfer(db *gorm.DB) error {\n    tx := db.Begin()\n    defer func() {\n        if r := recover(); r != nil {\n            tx.Rollback()\n        }\n    }()\n    \n    if err := tx.Error; err != nil {\n        return err\n    }\n    \n    // 执行业务逻辑...\n    \n    if err := tx.Commit().Error; err != nil {\n        tx.Rollback()\n        return err\n    }\n    return nil\n}\n\n// 方式3：嵌套事务（SavePoint）\nfunc NestedTransaction(db *gorm.DB) error {\n    return db.Transaction(func(tx *gorm.DB) error {\n        tx.Create(&User{Name: \"user1\"})\n        \n        // 嵌套事务\n        tx.Transaction(func(tx2 *gorm.DB) error {\n            tx2.Create(&User{Name: \"user2\"})\n            return errors.New(\"rollback user2\") // 只回滚内层\n        })\n        \n        tx.Create(&User{Name: \"user3\"}) // 仍会执行\n        return nil\n    })\n}",
			Explanation: "GORM提供多种事务处理方式。自动事务最简洁，手动事务更灵活，嵌套事务使用savepoint实现部分回滚。选择合适的方式可以提高代码可读性和可靠性。",
			Tags:        "GORM,事务,Transaction",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["数据库操作(GORM/SQL)"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "GORM关联查询和钩子函数",
			Content:     "请解释GORM的关联关系定义（BelongsTo、HasOne、HasMany、ManyToMany）以及常用的钩子函数。",
			Answer:      "关联关系：\n\n1. BelongsTo：属于，如User属于Company\n   gorm:\"foreignKey:CompanyID\"\n\n2. HasOne：拥有一个，如User有一个Profile\n   gorm:\"foreignKey:UserID\"\n\n3. HasMany：拥有多个，如User有多个Orders\n   gorm:\"foreignKey:UserID\"\n\n4. ManyToMany：多对多，如User和Role\n   gorm:\"many2many:user_roles;\"\n\n钩子函数：\n\n- 创建：BeforeCreate, AfterCreate\n- 查询：AfterFind\n- 更新：BeforeUpdate, AfterUpdate\n- 删除：BeforeDelete, AfterDelete\n- 保存：BeforeSave, AfterSave\n\n钩子函数接收*gorm.DB参数，可以访问当前事务上下文。",
			Explanation: "GORM的关联和钩子功能强大，合理使用可以简化数据操作逻辑，实现自动化的数据处理。",
			Tags:        "GORM,关联,钩子,Hook",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["数据库操作(GORM/SQL)"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyHard,
			Title:       "SQL注入防护",
			Content:     "在Go中如何有效防止SQL注入攻击？",
			OptionsJSON: mustMarshal([]Option{{"A", "使用字符串拼接SQL语句并过滤特殊字符"}, {"B", "使用参数化查询/预编译语句"}, {"C", "使用ORM就不需要防护"}, {"D", "前端做输入验证即可"}}),
			Answer:      "B",
			Explanation: "参数化查询（Prepared Statement）是防止SQL注入的最有效方法。数据库驱动会将参数与SQL语句分离处理，确保参数不会被当作SQL代码执行。使用database/sql的Exec/Query配合?占位符，或GORM的链式调用都是安全的。",
			Tags:        "SQL注入,安全,参数化查询",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["数据库操作(GORM/SQL)"],
			IndustryID:  industryID,
			Type:        QuestionTypeCode,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "数据库连接池配置",
			Content:     "请展示Go数据库连接池的常用配置参数及其含义。",
			Answer:      "import (\n    \"database/sql\"\n    \"time\"\n    _ \"github.com/lib/pq\"\n)\n\nfunc SetupDB(dsn string) (*sql.DB, error) {\n    db, err := sql.Open(\"postgres\", dsn)\n    if err != nil {\n        return nil, err\n    }\n    \n    // 连接池配置\n    db.SetMaxOpenConns(25)        // 最大打开连接数\n    db.SetMaxIdleConns(10)        // 最大空闲连接数\n    db.SetConnMaxLifetime(5 * time.Minute) // 连接最大生命周期\n    db.SetConnMaxIdleTime(10 * time.Minute) // 空闲连接最大存活时间\n    \n    // 验证连接\n    if err := db.Ping(); err != nil {\n        return nil, err\n    }\n    \n    return db, nil\n}",
			Explanation: "合理的连接池配置对数据库性能和稳定性至关重要。MaxOpenConns限制并发连接数，MaxIdleConns控制空闲连接，ConnMaxLifetime防止连接过期问题（如MySQL wait_timeout）。",
			Tags:        "连接池,数据库,性能优化",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["数据库操作(GORM/SQL)"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyHard,
			Title:       "数据库性能优化策略",
			Content:     "请列举Go应用中的数据库性能优化策略，包括查询优化、连接管理和缓存策略。",
			Answer:      "查询优化：\n\n1. 索引优化：为WHERE、ORDER BY、JOIN字段添加索引\n2. 避免SELECT *：只查询需要的字段\n3. 批量操作：使用批量INSERT代替单条，使用IN代替多条OR\n4. 分页优化：大数据量时使用游标或覆盖索引分页\n\n连接管理：\n\n1. 连接池调优：根据并发量设置合适的MaxOpenConns\n2. 预处理语句：复用执行计划，减少解析开销\n3. 读写分离：使用多个数据库连接处理读写请求\n\n缓存策略：\n\n1. 本地缓存：使用sync.Map或freecache缓存热点数据\n2. Redis缓存：缓存查询结果，设置合理过期时间\n3. 多级缓存：L1本地缓存 + L2 Redis缓存\n\nGORM优化：\n\n1. 使用Select指定字段\n2. 使用Omit排除大字段\n3. 使用Pluck查询单列\n4. 开启PrepareStmt预编译缓存",
			Explanation: "数据库性能优化是一个系统工程，需要从查询、连接、缓存多个层面综合考虑，根据实际场景选择合适的策略。",
			Tags:        "数据库,性能优化,索引,缓存",
			IsActive:    true,
		},

		// ========== 微服务架构 (6题) ==========
		{
			CategoryID:  categoryMap["微服务架构"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "gRPC vs REST的选择",
			Content:     "在微服务架构中，gRPC相比REST的主要优势是什么？",
			OptionsJSON: mustMarshal([]Option{{"A", "gRPC使用JSON，可读性更好"}, {"B", "gRPC基于HTTP/2，支持双向流和头部压缩"}, {"C", "REST性能总是优于gRPC"}, {"D", "gRPC不需要定义proto文件"}}),
			Answer:      "B",
			Explanation: "gRPC基于HTTP/2，具有多路复用、头部压缩、服务器推送等特性。使用Protocol Buffers序列化，比JSON更高效。支持四种通信模式：Unary、Client Streaming、Server Streaming、Bidirectional Streaming。",
			Tags:        "gRPC,REST,微服务,HTTP/2",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["微服务架构"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyHard,
			Title:       "Go微服务的服务发现方案",
			Content:     "请列举Go微服务架构中常用的服务发现方案，并对比它们的优缺点。",
			Answer:      "常用方案：\n\n1. Consul\n   - 优点：功能全面，支持健康检查，KV存储，多数据中心\n   - 缺点：部署复杂，资源占用较高\n   - Go客户端：hashicorp/consul/api\n\n2. etcd\n   - 优点：Kubernetes原生，强一致性，性能优秀\n   - 缺点：功能相对单一，主要用于配置和服务发现\n   - Go客户端：go.etcd.io/etcd/client/v3\n\n3. Nacos\n   - 优点：阿里巴巴开源，功能丰富（注册中心+配置中心）\n   - 缺点：Java生态为主，Go支持相对较弱\n\n4. Kubernetes DNS\n   - 优点：K8s原生，无需额外组件\n   - 缺点：依赖K8s环境，外部访问需要Ingress\n\n选择建议：\n- 云原生/K8s环境：优先使用K8s DNS或etcd\n- 传统部署：Consul功能更全面\n- 阿里生态：Nacos集成度更好",
			Explanation: "服务发现是微服务架构的核心组件，选择合适的方案需要考虑部署环境、团队技术栈和运维能力。",
			Tags:        "微服务,服务发现,Consul,etcd",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["微服务架构"],
			IndustryID:  industryID,
			Type:        QuestionTypeCode,
			Difficulty:  QuestionDifficultyHard,
			Title:       "实现一个简单的熔断器",
			Content:     "请实现一个基于计数器的熔断器（Circuit Breaker），包含关闭、打开、半开三种状态。",
			Answer:      "type State int\n\nconst (\n    StateClosed State = iota    // 关闭-正常\n    StateOpen                   // 打开-熔断\n    StateHalfOpen               // 半开-试探\n)\n\ntype CircuitBreaker struct {\n    mu                sync.RWMutex\n    state             State\n    failureCount      int\n    successCount      int\n    failureThreshold  int\n    successThreshold  int\n    timeout           time.Duration\n    lastFailureTime   time.Time\n}\n\nfunc NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {\n    return &CircuitBreaker{\n        failureThreshold: failureThreshold,\n        successThreshold: successThreshold,\n        timeout:          timeout,\n        state:            StateClosed,\n    }\n}\n\nfunc (cb *CircuitBreaker) Call(fn func() error) error {\n    if !cb.allowRequest() {\n        return errors.New(\"circuit breaker is open\")\n    }\n    \n    err := fn()\n    cb.recordResult(err)\n    return err\n}\n\nfunc (cb *CircuitBreaker) allowRequest() bool {\n    cb.mu.RLock()\n    defer cb.mu.RUnlock()\n    \n    switch cb.state {\n    case StateClosed:\n        return true\n    case StateOpen:\n        if time.Since(cb.lastFailureTime) > cb.timeout {\n            cb.mu.RUnlock()\n            cb.mu.Lock()\n            cb.state = StateHalfOpen\n            cb.failureCount = 0\n            cb.successCount = 0\n            cb.mu.Unlock()\n            cb.mu.RLock()\n            return true\n        }\n        return false\n    case StateHalfOpen:\n        return true\n    }\n    return false\n}\n\nfunc (cb *CircuitBreaker) recordResult(err error) {\n    cb.mu.Lock()\n    defer cb.mu.Unlock()\n    \n    if err != nil {\n        cb.failureCount++\n        cb.lastFailureTime = time.Now()\n        if cb.state == StateHalfOpen || cb.failureCount >= cb.failureThreshold {\n            cb.state = StateOpen\n        }\n    } else {\n        cb.successCount++\n        if cb.state == StateHalfOpen && cb.successCount >= cb.successThreshold {\n            cb.state = StateClosed\n            cb.failureCount = 0\n        }\n    }\n}",
			Explanation: "熔断器是微服务容错的重要模式。当失败率达到阈值时打开熔断，快速失败；超时后进入半开状态，允许部分请求试探；成功后恢复关闭状态。",
			Tags:        "熔断器,Circuit Breaker,容错",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["微服务架构"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "链路追踪实现原理",
			Content:     "请解释分布式链路追踪的实现原理，以及在Go微服务中如何集成Jaeger或Zipkin。",
			Answer:      "实现原理：\n\n1. Trace：一次完整的请求链路，由唯一的TraceID标识\n2. Span：链路中的基本单元，表示一个操作，包含SpanID、父SpanID\n3. Context传播：通过请求上下文传递Trace信息\n\n关键概念：\n- TraceID：全局唯一，贯穿整个链路\n- SpanID：当前操作ID\n- ParentSpanID：父操作ID，用于构建调用树\n- Baggage：随trace传递的自定义数据\n\nJaeger集成：\n使用opentracing配置tracer，设置sampler和reporter。通过HTTPHeadersCarrier在请求头中传递trace信息。",
			Explanation: "链路追踪是微服务可观测性的重要组成部分，帮助定位跨服务的性能问题和错误根因。",
			Tags:        "链路追踪,Jaeger,分布式,可观测性",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["微服务架构"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyEasy,
			Title:       "微服务通信方式选择",
			Content:     "在微服务架构中，同步通信和异步通信分别适用于什么场景？",
			OptionsJSON: mustMarshal([]Option{{"A", "同步适合实时性要求高的场景，异步适合解耦和削峰"}, {"B", "同步总是优于异步"}, {"C", "异步只适用于日志收集"}, {"D", "两者没有区别"}}),
			Answer:      "A",
			Explanation: "同步通信（HTTP/gRPC）适合需要立即响应的场景，如用户查询。异步通信（消息队列）适合解耦服务、流量削峰、最终一致性场景，如订单处理、通知发送。",
			Tags:        "微服务,同步,异步,消息队列",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["微服务架构"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyHard,
			Title:       "Go微服务限流算法实现",
			Content:     "请解释常用的限流算法（令牌桶、漏桶、计数器），并说明Go中的实现方式。",
			Answer:      "计数器算法：\n\n简单计数，固定窗口。缺点：窗口边界可能突发2倍流量。\n\n滑动窗口：\n\n将窗口细分为多个小窗口，平滑统计。Redis + Lua可实现分布式滑动窗口。\n\n令牌桶（Token Bucket）：\n\n- 以固定速率生成令牌放入桶中\n- 请求需要获取令牌才能执行\n- 桶满时丢弃令牌，桶空时限流\n- 允许一定程度的突发流量\n\n漏桶（Leaky Bucket）：\n\n- 请求进入桶中，以固定速率流出处理\n- 桶满时新请求被丢弃\n- 流量输出更平滑，无突发\n\nGo实现（令牌桶）：\n使用golang.org/x/time/rate包，NewLimiter创建限流器，Wait方法等待获取令牌。\n\n分布式限流：\n使用Redis + Lua脚本实现全局令牌桶，保证原子性。",
			Explanation: "限流是保护服务稳定性的重要手段。令牌桶允许突发，漏桶更平滑，根据业务特点选择合适的算法。",
			Tags:        "限流,令牌桶,漏桶,微服务",
			IsActive:    true,
		},

		// ========== 网络与协议 (4题) ==========
		{
			CategoryID:  categoryMap["网络与协议"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "TCP三次握手过程",
			Content:     "TCP三次握手的正确顺序是什么？",
			OptionsJSON: mustMarshal([]Option{{"A", "SYN -> SYN-ACK -> ACK"}, {"B", "SYN -> ACK -> SYN-ACK"}, {"C", "ACK -> SYN -> SYN-ACK"}, {"D", "SYN-ACK -> SYN -> ACK"}}),
			Answer:      "A",
			Explanation: "三次握手：1) 客户端发送SYN；2) 服务端回复SYN-ACK；3) 客户端回复ACK。目的是同步双方序列号，确认收发能力正常。不能减少为两次，否则无法确认客户端的接收能力。",
			Tags:        "TCP,三次握手,网络",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["网络与协议"],
			IndustryID:  industryID,
			Type:        QuestionTypeCode,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "实现TCP服务端",
			Content:     "请使用Go标准库实现一个简单的TCP echo服务器，支持并发连接处理。",
			Answer:      "package main\n\nimport (\n    \"bufio\"\n    \"fmt\"\n    \"net\"\n    \"strings\"\n)\n\nfunc main() {\n    listener, err := net.Listen(\"tcp\", \":8080\")\n    if err != nil {\n        panic(err)\n    }\n    defer listener.Close()\n    \n    fmt.Println(\"TCP server listening on :8080\")\n    \n    for {\n        conn, err := listener.Accept()\n        if err != nil {\n            fmt.Printf(\"Accept error: %v\\n\", err)\n            continue\n        }\n        go handleConnection(conn)\n    }\n}\n\nfunc handleConnection(conn net.Conn) {\n    defer conn.Close()\n    reader := bufio.NewReader(conn)\n    \n    for {\n        line, err := reader.ReadString('\\n')\n        if err != nil {\n            fmt.Printf(\"Client disconnected: %v\\n\", err)\n            return\n        }\n        \n        msg := strings.TrimSpace(line)\n        fmt.Printf(\"Received: %s\\n\", msg)\n        \n        // Echo back\n        _, err = conn.Write([]byte(\"Echo: \" + msg + \"\\n\"))\n        if err != nil {\n            fmt.Printf(\"Write error: %v\\n\", err)\n            return\n        }\n    }\n}",
			Explanation: "使用net包可以方便地实现TCP服务器。使用goroutine处理每个连接实现并发，使用bufio提高读写效率。生产环境需要添加连接超时、心跳检测等机制。",
			Tags:        "TCP,网络编程,并发",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["网络与协议"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyHard,
			Title:       "HTTP/1.1 vs HTTP/2 vs HTTP/3",
			Content:     "请对比HTTP/1.1、HTTP/2和HTTP/3的主要特性和差异。",
			Answer:      "HTTP/1.1：\n\n- 持久连接（Keep-Alive）\n- 管道化（pipelining，实际很少使用）\n- 队头阻塞（Head-of-line blocking）\n- 文本协议\n\nHTTP/2：\n\n- 二进制分帧层\n- 多路复用（Multiplexing），一个连接多个流\n- 头部压缩（HPACK）\n- 服务器推送\n- 优先级和流控制\n\nHTTP/3：\n\n- 基于QUIC协议（UDP之上）\n- 内置TLS 1.3\n- 连接迁移（IP变化保持连接）\n- 彻底解决队头阻塞（QUIC在传输层处理流）\n- 更快的握手（0-RTT或1-RTT）\n\nGo支持：\n- HTTP/1.1：标准库完整支持\n- HTTP/2：golang.org/x/net/http2\n- HTTP/3：github.com/lucas-clemente/quic-go",
			Explanation: "HTTP协议持续演进，HTTP/2解决了HTTP/1.1的队头阻塞，HTTP/3基于QUIC进一步提升了性能和可靠性。",
			Tags:        "HTTP,HTTP/2,HTTP/3,QUIC,网络",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["网络与协议"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "WebSocket与HTTP长轮询",
			Content:     "WebSocket相比HTTP长轮询的主要优势是什么？",
			OptionsJSON: mustMarshal([]Option{{"A", "WebSocket需要更多服务器资源"}, {"B", "WebSocket是全双工通信，延迟更低"}, {"C", "HTTP长轮询更适合实时通信"}, {"D", "两者性能完全相同"}}),
			Answer:      "B",
			Explanation: "WebSocket建立后保持连接，支持全双工通信，服务器可以主动推送消息，延迟低。HTTP长轮询需要不断建立新连接，有较高的延迟和资源开销。",
			Tags:        "WebSocket,长轮询,实时通信",
			IsActive:    true,
		},

		// ========== 数据结构与算法 (4题) ==========
		{
			CategoryID:  categoryMap["数据结构与算法"],
			IndustryID:  industryID,
			Type:        QuestionTypeCode,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "实现LRU缓存",
			Content:     "请使用Go实现一个LRU（最近最少使用）缓存，支持Get和Put操作，时间复杂度O(1)。",
			Answer:      "type LRUCache struct {\n    capacity int\n    cache    map[int]*list.Element\n    ll       *list.List\n}\n\ntype entry struct {\n    key   int\n    value int\n}\n\nfunc Constructor(capacity int) LRUCache {\n    return LRUCache{\n        capacity: capacity,\n        cache:    make(map[int]*list.Element),\n        ll:       list.New(),\n    }\n}\n\nfunc (c *LRUCache) Get(key int) int {\n    if elem, ok := c.cache[key]; ok {\n        c.ll.MoveToFront(elem)\n        return elem.Value.(*entry).value\n    }\n    return -1\n}\n\nfunc (c *LRUCache) Put(key int, value int) {\n    if elem, ok := c.cache[key]; ok {\n        c.ll.MoveToFront(elem)\n        elem.Value.(*entry).value = value\n        return\n    }\n    \n    if c.ll.Len() >= c.capacity {\n        back := c.ll.Back()\n        if back != nil {\n            c.ll.Remove(back)\n            delete(c.cache, back.Value.(*entry).key)\n        }\n    }\n    \n    elem := c.ll.PushFront(&entry{key, value})\n    c.cache[key] = elem\n}",
			Explanation: "LRU缓存使用哈希表+双向链表实现。哈希表提供O(1)查找，双向链表维护访问顺序，队尾是最久未使用的元素。Go标准库container/list提供双向链表实现。",
			Tags:        "LRU,缓存,数据结构,算法",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["数据结构与算法"],
			IndustryID:  industryID,
			Type:        QuestionTypeCode,
			Difficulty:  QuestionDifficultyHard,
			Title:       "实现并发安全的跳表",
			Content:     "请实现一个支持并发读写的跳表（Skip List），用于有序数据的快速查找。",
			Answer:      "type SkipList struct {\n    head   *node\n    level  int\n    length int32\n    mu     sync.RWMutex\n    rnd    *rand.Rand\n}\n\ntype node struct {\n    key   int\n    value interface{}\n    next  []*node\n}\n\nconst maxLevel = 16\n\nfunc NewSkipList() *SkipList {\n    return &SkipList{\n        head:  &node{next: make([]*node, maxLevel)},\n        level: 1,\n        rnd:   rand.New(rand.NewSource(time.Now().UnixNano())),\n    }\n}\n\nfunc (sl *SkipList) randomLevel() int {\n    level := 1\n    for sl.rnd.Float64() < 0.5 && level < maxLevel {\n        level++\n    }\n    return level\n}\n\nfunc (sl *SkipList) Get(key int) (interface{}, bool) {\n    sl.mu.RLock()\n    defer sl.mu.RUnlock()\n    \n    curr := sl.head\n    for i := sl.level - 1; i >= 0; i-- {\n        for curr.next[i] != nil && curr.next[i].key < key {\n            curr = curr.next[i]\n        }\n    }\n    \n    curr = curr.next[0]\n    if curr != nil && curr.key == key {\n        return curr.value, true\n    }\n    return nil, false\n}\n\nfunc (sl *SkipList) Put(key int, value interface{}) {\n    sl.mu.Lock()\n    defer sl.mu.Unlock()\n    \n    update := make([]*node, maxLevel)\n    curr := sl.head\n    \n    for i := sl.level - 1; i >= 0; i-- {\n        for curr.next[i] != nil && curr.next[i].key < key {\n            curr = curr.next[i]\n        }\n        update[i] = curr\n    }\n    \n    curr = curr.next[0]\n    if curr != nil && curr.key == key {\n        curr.value = value\n        return\n    }\n    \n    level := sl.randomLevel()\n    if level > sl.level {\n        for i := sl.level; i < level; i++ {\n            update[i] = sl.head\n        }\n        sl.level = level\n    }\n    \n    newNode := &node{key: key, value: value, next: make([]*node, level)}\n    for i := 0; i < level; i++ {\n        newNode.next[i] = update[i].next[i]\n        update[i].next[i] = newNode\n    }\n    \n    atomic.AddInt32(&sl.length, 1)\n}",
			Explanation: "跳表是有序链表的多级索引结构，平均时间复杂度O(log n)。使用RWMutex实现并发安全，读操作可以并发，写操作互斥。相比红黑树实现更简单。",
			Tags:        "跳表,SkipList,并发,数据结构",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["数据结构与算法"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "Go map的底层实现",
			Content:     "Go语言map的底层数据结构是什么？",
			OptionsJSON: mustMarshal([]Option{{"A", "红黑树"}, {"B", "哈希表（数组+链表/溢出桶）"}, {"C", "B+树"}, {"D", "跳表"}}),
			Answer:      "B",
			Explanation: "Go的map基于哈希表实现，使用数组存储桶（bucket），每个桶可以存储8个key-value对。当桶满时使用溢出桶。哈希冲突采用链地址法解决。扩容时使用渐进式迁移，避免一次性大量数据拷贝。",
			Tags:        "map,哈希表,底层实现",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["数据结构与算法"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyHard,
			Title:       "Go垃圾回收算法演进",
			Content:     "请解释Go垃圾回收算法的演进过程，以及当前版本的GC优化策略。",
			Answer:      "演进过程：\n\n1. Go 1.0-1.2：标记-清除（Stop The World）\n   - 全程STW，程序暂停明显\n\n2. Go 1.3：并行标记清除\n   - 标记阶段并行，但STW时间较长\n\n3. Go 1.5：三色标记法+写屏障\n   - 引入并发标记，大大减少STW时间\n   - 写屏障保证标记正确性\n\n4. Go 1.8：混合写屏障\n   - 消除栈扫描的STW\n   - STW时间降至亚毫秒级\n\n三色标记：\n- 白色：未访问，可能回收\n- 灰色：已访问，引用未处理\n- 黑色：已访问，引用已处理\n\n当前优化：\n- 并发标记与用户代码并行\n- 增量回收，分摊工作量\n- 调整GOGC控制GC频率\n- 使用sync.Pool减少内存分配",
			Explanation: "Go的GC持续优化，目标是低延迟。理解GC原理有助于编写GC友好的代码，减少内存分配和指针使用可以降低GC压力。",
			Tags:        "GC,垃圾回收,三色标记,性能",
			IsActive:    true,
		},

		// ========== 项目实战与设计模式 (4题) ==========
		{
			CategoryID:  categoryMap["项目实战与设计模式"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "Go项目标准目录结构",
			Content:     "请介绍Go项目的标准目录结构（如Standard Go Project Layout），并说明各目录的作用。",
			Answer:      "Standard Go Project Layout：\n\nproject/\n├── api/              # API定义（protobuf, openapi）\n├── assets/           # 静态资源\n├── build/            # 构建脚本和配置\n├── cmd/              # 应用程序入口\n│   └── server/\n│       └── main.go\n├── configs/          # 配置文件\n├── deployments/      # 部署配置（k8s, docker）\n├── docs/             # 文档\n├── examples/         # 示例代码\n├── internal/         # 私有代码\n│   ├── config/       # 配置逻辑\n│   ├── handler/      # HTTP处理\n│   ├── model/        # 数据模型\n│   ├── repository/   # 数据访问\n│   └── service/      # 业务逻辑\n├── pkg/              # 公共库（可被外部导入）\n├── scripts/          # 脚本文件\n├── test/             # 测试数据和工具\n├── web/              # Web静态文件\n├── go.mod\n└── README.md\n\n关键原则：\n- cmd/下每个子目录对应一个可执行文件\n- internal/下的代码不能被外部导入\n- pkg/下放可复用的公共代码\n- api/存放接口定义，便于版本管理",
			Explanation: "标准的目录结构有助于项目维护和团队协作，也是Go社区的最佳实践。",
			Tags:        "项目结构,目录布局,最佳实践",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["项目实战与设计模式"],
			IndustryID:  industryID,
			Type:        QuestionTypeCode,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "实现依赖注入容器",
			Content:     "请实现一个简单的依赖注入容器，支持服务的注册和解析。",
			Answer:      "type Container struct {\n    services map[string]interface{}\n    factories map[string]func(c *Container) interface{}\n}\n\nfunc NewContainer() *Container {\n    return &Container{\n        services:  make(map[string]interface{}),\n        factories: make(map[string]func(c *Container) interface{}),\n    }\n}\n\n// Register 注册单例服务\nfunc (c *Container) Register(name string, service interface{}) {\n    c.services[name] = service\n}\n\n// RegisterFactory 注册工厂函数\nfunc (c *Container) RegisterFactory(name string, factory func(c *Container) interface{}) {\n    c.factories[name] = factory\n}\n\n// Resolve 解析服务\nfunc (c *Container) Resolve(name string) (interface{}, error) {\n    if service, ok := c.services[name]; ok {\n        return service, nil\n    }\n    \n    if factory, ok := c.factories[name]; ok {\n        service := factory(c)\n        c.services[name] = service // 缓存为单例\n        return service, nil\n    }\n    \n    return nil, fmt.Errorf(\"service %s not found\", name)\n}\n\n// MustResolve 必须解析，失败panic\nfunc (c *Container) MustResolve(name string) interface{} {\n    service, err := c.Resolve(name)\n    if err != nil {\n        panic(err)\n    }\n    return service\n}",
			Explanation: "依赖注入容器实现了控制反转（IoC），降低模块间的耦合。工厂模式支持延迟初始化和依赖解析，单例缓存避免重复创建。",
			Tags:        "依赖注入,DI,IoC,设计模式",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["项目实战与设计模式"],
			IndustryID:  industryID,
			Type:        QuestionTypeChoice,
			Difficulty:  QuestionDifficultyMedium,
			Title:       "Go常用设计模式",
			Content:     "以下哪个设计模式在Go标准库中有典型应用？",
			OptionsJSON: mustMarshal([]Option{{"A", "单例模式 - sync.Once"}, {"B", "工厂模式 - context.WithCancel"}, {"C", "装饰器模式 - io.Reader/io.Writer"}, {"D", "以上都是"}}),
			Answer:      "D",
			Explanation: "Go标准库广泛应用设计模式：sync.Once实现单例；context包的各种WithXxx函数是工厂模式；io包通过接口组合实现装饰器模式（如bufio.Reader包装io.Reader）。Go更倾向于使用组合和接口而非继承实现设计模式。",
			Tags:        "设计模式,标准库,最佳实践",
			IsActive:    true,
		},
		{
			CategoryID:  categoryMap["项目实战与设计模式"],
			IndustryID:  industryID,
			Type:        QuestionTypeSubjective,
			Difficulty:  QuestionDifficultyHard,
			Title:       "Go项目测试策略",
			Content:     "请介绍Go项目的测试策略，包括单元测试、集成测试、基准测试和Mock的使用。",
			Answer:      "测试金字塔：\n\n1. 单元测试（70%）\n   - 测试单个函数/方法\n   - 使用testing包和 testify/assert\n   - 表驱动测试覆盖多场景\n   - 使用gomock或mockery生成Mock\n\n2. 集成测试（20%）\n   - 测试组件间交互\n   - 使用testcontainers启动真实依赖（数据库等）\n   - 放在tests/或*_integration_test.go\n\n3. E2E测试（10%）\n   - 端到端测试完整流程\n   - 使用httptest模拟HTTP请求\n\nMock使用：\n使用gomock生成接口的mock实现，在测试中注入mock对象隔离依赖。\n\n基准测试：\n使用testing.B进行性能测试，go test -bench运行。\n\n最佳实践：\n- 测试覆盖率目标>80%\n- 使用t.Parallel()并行测试\n- CI中运行测试并检查覆盖率",
			Explanation: "完善的测试是保证代码质量的关键。Go的测试工具链完善，合理组合各种测试类型可以提高代码可靠性。",
			Tags:        "测试,单元测试,集成测试,Mock",
			IsActive:    true,
		},
	}
}

// Option 选择题选项结构
type Option struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// mustMarshal 将对象序列化为JSON字符串，失败时panic
func mustMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// seedPromptTemplates 插入Prompt模板种子数据
func seedPromptTemplates(db *gorm.DB) error {
	templates := []PromptTemplate{
		{
			IndustryID:      nil, // 通用模板
			Name:            "Go面试官",
			Scene:           PromptSceneInterview,
			TemplateContent: "你是一位资深Go语言面试官，拥有10年以上的Go开发经验。你正在对候选人进行技术面试。\n\n你的角色特点：\n- 专业严谨，注重考察候选人的技术深度和广度\n- 善于通过追问挖掘候选人的真实水平\n- 会针对候选人的回答给出评价和建议\n\n面试流程：\n1. 根据候选人的回答评估技术掌握程度\n2. 对回答不完整的地方进行追问\n3. 适时给出技术点的补充说明\n4. 最后给出综合面试评价\n\n当前面试信息：\n- 候选人：{{username}}\n- 面试岗位：Go开发工程师\n- 面试轮次：{{round}}\n\n请开始面试。",
			Variables:       `{"username": "候选人姓名", "round": "面试轮次"}`,
			IsActive:        true,
		},
		{
			IndustryID:      nil,
			Name:            "学习伙伴",
			Scene:           PromptSceneCompanion,
			TemplateContent: "你是一位温柔友好的学习伙伴，陪伴用户学习Go语言。你的目标是帮助用户轻松愉快地掌握Go语言知识。\n\n你的角色特点：\n- 耐心细致，善于用通俗的语言解释复杂概念\n- 会鼓励用户，增强学习信心\n- 善于举例，用实际案例帮助理解\n- 会根据用户的学习进度调整讲解深度\n\n互动方式：\n1. 用轻松友好的语气交流\n2. 适时提问确认用户理解程度\n3. 提供练习题巩固知识点\n4. 总结要点帮助记忆\n\n用户信息：\n- 用户名：{{username}}\n- 学习进度：{{progress}}\n- 当前主题：{{topic}}\n\n让我们开始学习吧！",
			Variables:       `{"username": "用户名称", "progress": "学习进度", "topic": "当前学习主题"}`,
			IsActive:        true,
		},
		{
			IndustryID:      nil,
			Name:            "刷题助手",
			Scene:           PromptSceneQuiz,
			TemplateContent: "你是一位Go语言技术专家，专门帮助用户分析和理解面试题目。\n\n你的角色特点：\n- 对Go语言有深入的理解，能解释底层原理\n- 善于分析题目考点和易错点\n- 会提供多种解题思路\n- 会推荐相关的延伸学习资料\n\n分析流程：\n1. 分析题目考察的知识点\n2. 解释正确答案及原因\n3. 分析常见错误选项的陷阱\n4. 提供相关知识点扩展\n5. 推荐类似练习题\n\n当前题目：\n- 题目类型：{{question_type}}\n- 难度：{{difficulty}}\n- 题目内容：{{question_content}}\n\n请开始分析这道题目。",
			Variables:       `{"question_type": "题目类型", "difficulty": "难度", "question_content": "题目内容"}`,
			IsActive:        true,
		},
		{
			IndustryID:      nil,
			Name:            "学习计划生成器",
			Scene:           PromptScenePlan,
			TemplateContent: "你是一位专业的Go语言学习规划师，根据用户的情况制定个性化的学习计划。\n\n你的角色特点：\n- 了解Go语言学习路径和知识体系\n- 能根据用户基础和时间制定合理计划\n- 会考虑学习效率和知识巩固\n- 会设置阶段性目标和检验点\n\n计划制定流程：\n1. 评估用户当前水平\n2. 确定学习目标和时间范围\n3. 分解知识点为学习单元\n4. 安排学习顺序和节奏\n5. 设置复习和练习节点\n\n用户信息：\n- 用户名：{{username}}\n- 当前水平：{{level}}\n- 可用时间：{{available_time}}\n- 学习目标：{{goal}}\n\n请制定学习计划。",
			Variables:       `{"username": "用户名称", "level": "当前水平", "available_time": "可用时间", "goal": "学习目标"}`,
			IsActive:        true,
		},
	}

	for _, template := range templates {
		if err := db.Create(&template).Error; err != nil {
			return fmt.Errorf("插入Prompt模板失败: %w", err)
		}
	}
	applogger.Info("Prompt模板种子数据插入完成")
	return nil
}

// seedAdminConfigs 插入管理配置种子数据
func seedAdminConfigs(db *gorm.DB) error {
	configs := []AdminConfig{
		{
			ConfigKey:   "ai_provider",
			ConfigValue: "mock",
			ConfigType:  ConfigTypeString,
			Description: "AI runtime provider",
		},
		{
			ConfigKey:   "ai_top_p",
			ConfigValue: "0.9",
			ConfigType:  ConfigTypeNumber,
			Description: "AI top-p sampling",
		},
		{
			ConfigKey:   "ai_max_tokens",
			ConfigValue: "2048",
			ConfigType:  ConfigTypeNumber,
			Description: "AI max completion tokens",
		},
		{
			ConfigKey:   "ai_timeout_seconds",
			ConfigValue: "30",
			ConfigType:  ConfigTypeNumber,
			Description: "AI request timeout in seconds",
		},
		{
			ConfigKey:   "ai_enable_stream",
			ConfigValue: "false",
			ConfigType:  ConfigTypeBoolean,
			Description: "Enable AI streaming",
		},
		{
			ConfigKey:   "ai_fallback_provider",
			ConfigValue: "mock",
			ConfigType:  ConfigTypeString,
			Description: "Fallback provider when primary provider fails",
		},
		{
			ConfigKey:   "ai_scene_interview_model",
			ConfigValue: "",
			ConfigType:  ConfigTypeString,
			Description: "Interview scene model override",
		},
		{
			ConfigKey:   "ai_scene_plan_model",
			ConfigValue: "",
			ConfigType:  ConfigTypeString,
			Description: "Plan scene model override",
		},
		{
			ConfigKey:   "ai_scene_companion_model",
			ConfigValue: "",
			ConfigType:  ConfigTypeString,
			Description: "Companion scene model override",
		},
		{
			ConfigKey:   "ai_scene_quiz_model",
			ConfigValue: "",
			ConfigType:  ConfigTypeString,
			Description: "Quiz scene model override",
		},
		{
			ConfigKey:   "ai_model",
			ConfigValue: "gpt-4o-mini",
			ConfigType:  ConfigTypeString,
			Description: "AI模型名称",
		},
		{
			ConfigKey:   "ai_temperature",
			ConfigValue: "0.7",
			ConfigType:  ConfigTypeNumber,
			Description: "AI温度参数，控制输出随机性",
		},
		{
			ConfigKey:   "daily_free_practice_limit",
			ConfigValue: "20",
			ConfigType:  ConfigTypeNumber,
			Description: "每日免费刷题次数限制",
		},
		{
			ConfigKey:   "daily_free_interview_limit",
			ConfigValue: "2",
			ConfigType:  ConfigTypeNumber,
			Description: "每日免费模拟面试次数限制",
		},
		{
			ConfigKey:   "max_interview_duration",
			ConfigValue: "60",
			ConfigType:  ConfigTypeNumber,
			Description: "单次模拟面试最大时长（分钟）",
		},
		{
			ConfigKey:   "system_name",
			ConfigValue: "MakeJob面试助手",
			ConfigType:  ConfigTypeString,
			Description: "系统名称",
		},
		{
			ConfigKey:   "system_logo",
			ConfigValue: "https://example.com/logo.png",
			ConfigType:  ConfigTypeString,
			Description: "系统Logo URL",
		},
		{
			ConfigKey:   "enable_registration",
			ConfigValue: "true",
			ConfigType:  ConfigTypeBoolean,
			Description: "是否开放注册",
		},
	}

	for _, config := range configs {
		if err := db.Create(&config).Error; err != nil {
			return fmt.Errorf("插入管理配置失败: %w", err)
		}
	}
	applogger.Info("管理配置种子数据插入完成")
	return nil
}

// seedAdminUser 插入默认管理员账户
func seedAdminUser(db *gorm.DB) error {
	// 检查是否已存在管理员账户
	var count int64
	if err := db.Model(&User{}).Where("role = ?", UserRoleAdmin).Count(&count).Error; err != nil {
		return fmt.Errorf("检查管理员账户失败: %w", err)
	}
	if count > 0 {
		applogger.Info("管理员账户已存在，跳过插入")
		return nil
	}

	// 使用bcrypt加密密码（cost=10）
	// 密码: admin123456
	// 这是预计算的哈希值，对应密码 "admin123456"
	passwordHash := "$2a$10$wlSKnaf5hUu4sl0CM6VzxeNILBf/W.r09rZBuYkaAvL6fAtKWOKG."

	admin := User{
		Username:        "Admin",
		Email:           "admin@makejob.com",
		PasswordHash:    passwordHash,
		Role:            UserRoleAdmin,
		MembershipLevel: MembershipLevelPro,
	}

	if err := db.Create(&admin).Error; err != nil {
		return fmt.Errorf("插入管理员账户失败: %w", err)
	}

	applogger.Info("默认管理员账户创建成功",
		zap.String("email", admin.Email),
		zap.String("username", admin.Username))
	return nil
}
