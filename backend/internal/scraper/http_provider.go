package scraper

import (
	"context"
	"fmt"
	"net/http"
	neturl "net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const defaultScraperUserAgent = "MakeJobBot/1.0 (+https://github.com/makejob)"

type sourceConfig struct {
	Source        Source
	SearchPath    string
	ArticleTokens []string
}

// HTTPProvider 使用真实 HTTP 请求抓取站外面经内容。
type HTTPProvider struct {
	client  *http.Client
	sources map[string]sourceConfig
}

// HTTPProviderOption 定义真实爬虫 Provider 的可选配置项。
type HTTPProviderOption func(*HTTPProvider)

// NewHTTPProvider 创建真实 HTTP 爬虫 Provider。
func NewHTTPProvider(options ...HTTPProviderOption) ScraperProvider {
	provider := &HTTPProvider{
		client: &http.Client{Timeout: 12 * time.Second},
		sources: map[string]sourceConfig{
			SourceNiuke: {
				Source:        Source{Name: SourceNiuke, Label: "牛客网", BaseURL: "https://www.nowcoder.com", IsActive: true},
				SearchPath:    "/search?query=%s",
				ArticleTokens: []string{"/discuss/", "/feed/main/detail/", "/article/"},
			},
			SourceLeetCode: {
				Source:        Source{Name: SourceLeetCode, Label: "LeetCode", BaseURL: "https://leetcode.cn", IsActive: true},
				SearchPath:    "/search?query=%s",
				ArticleTokens: []string{"/discuss/", "/circle/discuss/", "/problems/"},
			},
			SourceJuejin: {
				Source:        Source{Name: SourceJuejin, Label: "掘金", BaseURL: "https://juejin.cn", IsActive: true},
				SearchPath:    "/search?query=%s",
				ArticleTokens: []string{"/post/", "/entry/", "/p/"},
			},
		},
	}
	for _, option := range options {
		option(provider)
	}
	return provider
}

// WithHTTPClient 覆盖爬虫使用的 HTTP Client。
func WithHTTPClient(client *http.Client) HTTPProviderOption {
	return func(provider *HTTPProvider) {
		if client != nil {
			provider.client = client
		}
	}
}

// WithSourceBaseURL 覆盖指定数据源的基础地址，便于测试或灰度切换。
func WithSourceBaseURL(source string, baseURL string) HTTPProviderOption {
	return func(provider *HTTPProvider) {
		cfg, ok := provider.sources[source]
		if !ok {
			return
		}
		cfg.Source.BaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		provider.sources[source] = cfg
	}
}

// Search 对目标站点发起真实搜索请求并解析结果页。
func (p *HTTPProvider) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	cfg, err := p.getSourceConfig(req.Source)
	if err != nil {
		return nil, err
	}
	keyword := strings.TrimSpace(req.Keyword)
	if keyword == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}

	searchURL, err := buildSearchURL(cfg, keyword)
	if err != nil {
		return nil, err
	}
	root, _, err := p.fetchDocument(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("抓取搜索页失败: %w", err)
	}

	results := collectSearchResults(root, cfg)
	if len(results) == 0 {
		return nil, fmt.Errorf("未从 %s 搜索结果页解析到可用链接", cfg.Source.Label)
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	start := (page - 1) * pageSize
	if start >= len(results) {
		return []SearchResult{}, nil
	}
	end := start + pageSize
	if end > len(results) {
		end = len(results)
	}
	return results[start:end], nil
}

// Fetch 对具体文章地址发起真实抓取并抽取正文。
func (p *HTTPProvider) Fetch(ctx context.Context, req FetchRequest) (*FetchResult, error) {
	cfg, err := p.getSourceConfig(req.Source)
	if err != nil {
		return nil, err
	}
	targetURL := strings.TrimSpace(req.URL)
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

	return &FetchResult{
		Title:     title,
		Content:   content,
		Author:    extractAuthor(root),
		URL:       finalURL,
		Source:    cfg.Source.Name,
		FetchedAt: time.Now(),
	}, nil
}

// GetSupportedSources 返回当前真实 Provider 声明支持的数据源列表。
func (p *HTTPProvider) GetSupportedSources() []Source {
	items := make([]Source, 0, len(p.sources))
	for _, cfg := range p.sources {
		items = append(items, cfg.Source)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

// getSourceConfig 读取指定数据源的抓取配置。
func (p *HTTPProvider) getSourceConfig(source string) (sourceConfig, error) {
	cfg, ok := p.sources[strings.TrimSpace(source)]
	if !ok {
		return sourceConfig{}, fmt.Errorf("暂不支持的数据源: %s", source)
	}
	return cfg, nil
}

// fetchDocument 发起 HTTP 请求并解析 HTML 文档。
func (p *HTTPProvider) fetchDocument(ctx context.Context, targetURL string) (*html.Node, string, error) {
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

// buildSearchURL 根据数据源配置生成搜索地址。
func buildSearchURL(cfg sourceConfig, keyword string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.Source.BaseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("数据源 %s 未配置基础地址", cfg.Source.Name)
	}
	searchPath := strings.TrimSpace(cfg.SearchPath)
	if searchPath == "" {
		return "", fmt.Errorf("数据源 %s 未配置搜索路径", cfg.Source.Name)
	}
	return baseURL + fmt.Sprintf(searchPath, neturl.QueryEscape(keyword)), nil
}

// collectSearchResults 遍历搜索页中的链接并提取可用结果。
func collectSearchResults(root *html.Node, cfg sourceConfig) []SearchResult {
	seen := make(map[string]bool)
	results := make([]SearchResult, 0)
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
				results = append(results, SearchResult{
					Title:   title,
					URL:     urlValue,
					Summary: trimSummary(extractParentText(node)),
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

// normalizeArticleURL 归一化搜索结果中的文章链接。
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

// extractPageTitle 提取页面标题。
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

// extractAuthor 提取页面作者信息，缺失时返回空字符串。
func extractAuthor(root *html.Node) string {
	var author string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil || author != "" {
			return
		}
		if node.Type == html.ElementNode && node.Data == "meta" {
			name := strings.ToLower(strings.TrimSpace(getAttribute(node, "name")))
			property := strings.ToLower(strings.TrimSpace(getAttribute(node, "property")))
			if name == "author" || property == "article:author" {
				author = normalizeText(getAttribute(node, "content"))
				return
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return author
}

// extractBestContent 优先抽取 article/main 节点，否则选择文本最长的正文容器。
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

// findPreferredContentNode 查找 article/main 等优先正文节点。
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

// findLargestTextNode 从常见正文容器中选择文本最长的节点。
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

// extractText 递归提取节点内的纯文本。
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

// extractParentText 读取父节点文本，作为搜索摘要候选。
func extractParentText(node *html.Node) string {
	if node == nil || node.Parent == nil {
		return ""
	}
	return normalizeText(extractText(node.Parent))
}

// getAttribute 读取 HTML 节点属性值。
func getAttribute(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// normalizeText 压缩空白字符，便于返回稳定的结构化文本。
func normalizeText(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

// trimSummary 截断搜索摘要，避免返回整个父节点正文。
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
