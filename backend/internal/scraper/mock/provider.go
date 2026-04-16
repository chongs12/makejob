// Package mock 提供Scraper的Mock实现
package mock

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"makejob-backend/internal/scraper"
)

// MockScraperProvider Mock爬虫实现
type MockScraperProvider struct{}

// NewMockScraperProvider 创建Mock爬虫实例
func NewMockScraperProvider() scraper.ScraperProvider {
	return &MockScraperProvider{}
}

// 支持的数据源
var supportedSources = []scraper.Source{
	{Name: scraper.SourceNiuke, Label: "牛客网", BaseURL: "https://www.nowcoder.com", IsActive: true},
	{Name: scraper.SourceLeetCode, Label: "LeetCode", BaseURL: "https://leetcode.cn", IsActive: true},
	{Name: scraper.SourceJuejin, Label: "掘金", BaseURL: "https://juejin.cn", IsActive: true},
}

// 预设的搜索结果模板
type searchResultTemplate struct {
	TitlePrefix string
	Authors     []string
	Summaries   []string
}

var searchTemplates = map[string]searchResultTemplate{
	scraper.SourceNiuke: {
		TitlePrefix: "牛客",
		Authors:     []string{"字节Offer收割机", "阿里内推官", "腾讯校招君", "后端开发小哥", "面试必过哥"},
		Summaries: []string{
			"详细记录了一面的技术问题和回答思路，包括Go基础、并发编程、MySQL优化等...",
			"分享面试全流程，从简历筛选到HR面，含金量很高...",
			"整理了面试中遇到的所有技术问题，附参考答案...",
			"校招面试经验分享，包含手撕算法和项目深挖...",
			"社招面试总结，重点考察系统设计和架构能力...",
		},
	},
	scraper.SourceLeetCode: {
		TitlePrefix: "LC",
		Authors:     []string{"刷题大神", "算法小王子", "Offer收割机", "面试达人", "码农进阶"},
		Summaries: []string{
			"面试题汇总，包含数据结构与算法、系统设计...",
			"手撕代码经验分享，重点题目解析...",
			"高频面试题整理，适合快速准备...",
			"大厂面试真题汇总，带详细解析...",
			"算法面试通关秘籍，从入门到offer...",
		},
	},
	scraper.SourceJuejin: {
		TitlePrefix: "掘金",
		Authors:     []string{"前端小姐姐", "后端老司机", "全栈工程师", "架构师老王", "技术小能手"},
		Summaries: []string{
			"深度解析面试中高频考点，附带学习路线...",
			"面试复盘，分享踩坑经验和解决方案...",
			"技术栈考察重点整理，适合进阶学习...",
			"面试那些事，从准备到拿offer全流程...",
			"面试必问知识点总结，建议收藏...",
		},
	},
}

// 预设的面经标题（根据关键词动态生成）
var titleTemplates = []string{
	"%s Go后端一面面经 - 2024秋招",
	"%s Go语言社招面经分享",
	"%s Go开发三轮面试总结",
	"%s 后端开发校招面经",
	"%s Go工程师面试真题汇总",
	"%s 技术面试复盘-Go方向",
	"%s 后端面试经验分享(含Go)",
	"%s Go岗位面试全记录",
}

// Search 搜索面经 (Mock实现)
func (p *MockScraperProvider) Search(ctx context.Context, req scraper.SearchRequest) ([]scraper.SearchResult, error) {
	template, ok := searchTemplates[req.Source]
	if !ok {
		return nil, fmt.Errorf("不支持的数据源: %s", req.Source)
	}

	// 根据关键词生成结果数量 (5-10条)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	count := r.Intn(6) + 5

	results := make([]scraper.SearchResult, count)
	now := time.Now()

	for i := 0; i < count; i++ {
		companyIndex := (i + int(now.Unix())) % len(titleTemplates)
		authorIndex := r.Intn(len(template.Authors))
		summaryIndex := r.Intn(len(template.Summaries))

		// 生成标题
		title := fmt.Sprintf(titleTemplates[companyIndex], req.Keyword)

		// 生成日期 (最近30天内)
		date := now.AddDate(0, 0, -r.Intn(30)).Format("2006-01-02")

		// 生成URL
		url := fmt.Sprintf("%s/discuss/%d/%s-%d",
			supportedSources[getSourceIndex(req.Source)].BaseURL,
			r.Intn(9000000)+1000000,
			req.Source,
			r.Intn(10000),
		)

		results[i] = scraper.SearchResult{
			Title:     title,
			URL:       url,
			Author:    template.Authors[authorIndex],
			Date:      date,
			Summary:   template.Summaries[summaryIndex],
			Source:    req.Source,
			ViewCount: r.Intn(10000) + 500,
		}
	}

	return results, nil
}

// Fetch 爬取面经内容 (Mock实现)
func (p *MockScraperProvider) Fetch(ctx context.Context, req scraper.FetchRequest) (*scraper.FetchResult, error) {
	// 返回一篇预设的Go面经完整内容
	content := `一面（技术面）约50分钟

1. 自我介绍
简单介绍了自己的背景和项目经验。

2. Go里的slice底层是怎么实现的？扩容机制是什么？
Slice底层是一个结构体，包含指向数组的指针、长度和容量。扩容时，如果容量小于1024，直接翻倍；大于1024则增长25%。

3. goroutine和线程的区别？GMP模型讲一下
Goroutine是用户态线程，由Go运行时管理，创建开销很小。GMP模型中，G是goroutine，M是机器线程，P是处理器，P的数量默认等于CPU核心数。

4. channel底层实现原理，有缓冲和无缓冲的区别
Channel底层通过互斥锁和条件变量实现。无缓冲channel发送和接收必须同步进行，有缓冲channel允许异步发送接收。

5. 讲一下Go的GC机制，三色标记法
Go使用并发标记清除GC，三色标记将对象分为白、灰、黑三色。白色表示未访问，灰色表示已访问但其引用的对象未访问完，黑色表示已访问完成。

6. sync.Map的实现原理，什么场景下用？
sync.Map通过空间换时间，使用readOnly和dirty两个map，适合读多写少的场景。

7. 项目中是怎么做服务发现的？
使用etcd作为注册中心，服务启动时注册，心跳保活，服务下线时注销。

8. Redis分布式锁怎么实现？有什么问题？
使用SETNX命令加锁，设置过期时间防止死锁。问题是锁续期和主从切换可能导致锁丢失，建议使用Redlock算法。

9. MySQL索引失效的场景有哪些？
- 使用函数或运算
- 类型转换
- LIKE以%开头
- OR连接非索引列
- 联合索引不满足最左前缀

10. 手撕算法：反转链表
写了迭代和递归两种解法。

二面（项目面）约40分钟

1. 详细介绍一下你的项目
介绍了项目的架构、技术选型和难点。

2. 项目中遇到的最大的挑战是什么？
讲了高并发场景下的缓存一致性问题，以及解决方案。

3. 你是如何保证接口幂等性的？
使用唯一请求ID + Redis实现幂等校验。

4. 微服务之间是如何通信的？
使用gRPC进行同步调用，消息队列进行异步解耦。

三面（HR面）约20分钟

1. 为什么选择我们公司？
2. 职业规划
3. 期望薪资
4. 还有什么问题想问的？`

	return &scraper.FetchResult{
		Title:     "字节跳动Go后端一面面经 - 2024秋招",
		Content:   content,
		Author:    "字节Offer收割机",
		URL:       req.URL,
		Source:    req.Source,
		FetchedAt: time.Now(),
	}, nil
}

// GetSupportedSources 获取支持的数据源
func (p *MockScraperProvider) GetSupportedSources() []scraper.Source {
	return supportedSources
}

// getSourceIndex 获取数据源索引
func getSourceIndex(source string) int {
	for i, s := range supportedSources {
		if s.Name == source {
			return i
		}
	}
	return 0
}
