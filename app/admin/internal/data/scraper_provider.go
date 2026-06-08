package data

import (
	"context"
	"fmt"
	"net/http"
	neturl "net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"

	"makejob/app/admin/internal/biz"
)

const defaultScraperUserAgent = "MakeJobBot/1.0 (+https://github.com/makejob)"

type sourceConfig struct {
	Source        biz.ScraperSourceRecord
	SearchPath    string
	ArticleTokens []string
}

// ScraperProvider 爬虫提供者，负责搜索和抓取外部站点面经内容。
type ScraperProvider struct {
	client  *http.Client
	sources map[string]sourceConfig
}

// NewScraperProvider 创建爬虫提供者实例。
func NewScraperProvider() *ScraperProvider {
	return &ScraperProvider{
		client: &http.Client{Timeout: 12 * time.Second},
		sources: map[string]sourceConfig{
			"niuke": {
				Source:        biz.ScraperSourceRecord{Name: "niuke", Label: "牛客网", BaseURL: "https://www.nowcoder.com", IsActive: true},
				SearchPath:    "/search?query=%s",
				ArticleTokens: []string{"/discuss/", "/feed/main/detail/", "/article/"},
			},
			"leetcode": {
				Source:        biz.ScraperSourceRecord{Name: "leetcode", Label: "LeetCode", BaseURL: "https://leetcode.cn", IsActive: true},
				SearchPath:    "/search?query=%s",
				ArticleTokens: []string{"/discuss/", "/circle/discuss/", "/problems/"},
			},
			"juejin": {
				Source:        biz.ScraperSourceRecord{Name: "juejin", Label: "掘金", BaseURL: "https://juejin.cn", IsActive: true},
				SearchPath:    "/search?query=%s",
				ArticleTokens: []string{"/post/", "/entry/", "/p/"},
			},
		},
	}
}

// GetSources 返回支持的数据源列表。
func (p *ScraperProvider) GetSources() []biz.ScraperSourceRecord {
	items := make([]biz.ScraperSourceRecord, 0, len(p.sources))
	for _, cfg := range p.sources {
		items = append(items, cfg.Source)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

// Search 对目标站点发起搜索请求并解析结果页。
func (p *ScraperProvider) Search(ctx context.Context, source, keyword string, page, pageSize int32) ([]*biz.ScraperSearchResult, int32, error) {
	cfg, ok := p.sources[strings.TrimSpace(source)]
	if !ok {
		return nil, 0, fmt.Errorf("暂不支持的数据源: %s", source)
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, 0, fmt.Errorf("搜索关键词不能为空")
	}

	searchURL := cfg.Source.BaseURL + fmt.Sprintf(cfg.SearchPath, neturl.QueryEscape(keyword))
	root, _, err := p.fetchDocument(ctx, searchURL)
	if err != nil {
		return nil, 0, fmt.Errorf("抓取搜索页失败: %w", err)
	}

	results := collectSearchResults(root, cfg)
	if len(results) == 0 {
		return nil, 0, fmt.Errorf("未从 %s 搜索结果页解析到可用链接", cfg.Source.Label)
	}

	total := int32(len(results))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	start := int((page - 1) * pageSize)
	if start >= len(results) {
		return []*biz.ScraperSearchResult{}, total, nil
	}
	end := start + int(pageSize)
	if end > len(results) {
		end = len(results)
	}
	return results[start:end], total, nil
}

// Fetch 对具体文章地址发起抓取并抽取正文。
func (p *ScraperProvider) Fetch(ctx context.Context, source, url string) (*biz.ScraperFetchResult, error) {
	cfg, ok := p.sources[strings.TrimSpace(source)]
	if !ok {
		return nil, fmt.Errorf("暂不支持的数据源: %s", source)
	}
	targetURL := strings.TrimSpace(url)
	if targetURL == "" {
		return nil, fmt.Errorf("抓取 URL 不能为空")
	}

	root, finalURL, err := p.fetchDocument(ctx, targetURL)
	if err != nil {
		return nil, fmt.Errorf("抓取文章失败: %w", err)
	}
	title := extractPageTitle(root)
	content := extractBestContent(root)
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("未从 %s 页面中抽取到正文内容", cfg.Source.Label)
	}

	return &biz.ScraperFetchResult{
		Title:   title,
		Content: content,
		Source:  cfg.Source.Name,
		URL:     finalURL,
	}, nil
}

func (p *ScraperProvider) fetchDocument(ctx context.Context, targetURL string) (*html.Node, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("User-Agent", defaultScraperUserAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	response, err := p.client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("HTTP %d", response.StatusCode)
	}

	root, err := html.Parse(response.Body)
	if err != nil {
		return nil, "", err
	}
	return root, response.Request.URL.String(), nil
}

func collectSearchResults(root *html.Node, cfg sourceConfig) []*biz.ScraperSearchResult {
	seen := make(map[string]bool)
	results := make([]*biz.ScraperSearchResult, 0)
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == "a" {
			href := strings.TrimSpace(getAttribute(node, "href"))
			title := normalizeText(extractText(node))
			urlValue, ok := normalizeArticleURL(cfg, href)
			if ok && title != "" && !seen[urlValue] {
				seen[urlValue] = true
				results = append(results, &biz.ScraperSearchResult{
					Title:   title,
					URL:     urlValue,
					Snippet: trimSummary(extractParentText(node)),
					Source:  cfg.Source.Name,
				})
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return results
}

func normalizeArticleURL(cfg sourceConfig, href string) (string, bool) {
	if href == "" || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "#") {
		return "", false
	}
	baseURL, err := neturl.Parse(cfg.Source.BaseURL)
	if err != nil {
		return "", false
	}
	parsedHref, err := neturl.Parse(href)
	if err != nil {
		return "", false
	}
	resolved := baseURL.ResolveReference(parsedHref)
	if !strings.EqualFold(baseURL.Hostname(), resolved.Hostname()) {
		return "", false
	}
	for _, token := range cfg.ArticleTokens {
		if strings.Contains(resolved.Path, token) {
			return resolved.String(), true
		}
	}
	return "", false
}

func extractPageTitle(root *html.Node) string {
	var title string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil || title != "" {
			return
		}
		if node.Type == html.ElementNode && node.Data == "title" {
			title = normalizeText(extractText(node))
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return title
}

func extractBestContent(root *html.Node) string {
	if node := findPreferredContentNode(root); node != nil {
		if text := normalizeText(extractText(node)); text != "" {
			return text
		}
	}
	bestNode := findLargestTextNode(root)
	if bestNode == nil {
		return ""
	}
	return normalizeText(extractText(bestNode))
}

func findPreferredContentNode(root *html.Node) *html.Node {
	preferredTags := map[string]bool{"article": true, "main": true}
	var found *html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil || found != nil {
			return
		}
		if node.Type == html.ElementNode && preferredTags[node.Data] {
			found = node
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func findLargestTextNode(root *html.Node) *html.Node {
	containerTags := map[string]bool{"section": true, "div": true, "article": true, "main": true}
	var bestNode *html.Node
	bestLength := 0
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode && containerTags[node.Data] {
			text := normalizeText(extractText(node))
			if length := len([]rune(text)); length > bestLength {
				bestLength = length
				bestNode = node
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return bestNode
}

func extractText(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == html.TextNode {
		return node.Data
	}
	if node.Type == html.ElementNode {
		switch node.Data {
		case "script", "style", "noscript", "svg":
			return ""
		}
	}
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		text := extractText(child)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(text)
	}
	return builder.String()
}

func extractParentText(node *html.Node) string {
	if node == nil || node.Parent == nil {
		return ""
	}
	return normalizeText(extractText(node.Parent))
}

func getAttribute(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func normalizeText(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

func trimSummary(summary string) string {
	if summary == "" {
		return ""
	}
	runes := []rune(summary)
	if len(runes) <= 160 {
		return summary
	}
	return string(runes[:160])
}
