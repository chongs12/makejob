package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHTTPProviderSearchAndFetch 验证真实 HTTP Provider 能从搜索页和文章页提取结构化结果。
func TestHTTPProviderSearchAndFetch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("query"); got != "go%20面经" && got != "go 面经" {
			t.Fatalf("unexpected query: %s", got)
		}
		_, _ = w.Write([]byte(`
			<html><body>
				<div class="result">
					<a href="/discuss/123">Go 后端一面面经</a>
					<p>覆盖并发、MySQL 与项目设计问题</p>
				</div>
			</body></html>
		`))
	})
	mux.HandleFunc("/discuss/123", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
			<html>
				<head>
					<title>Go 后端一面面经 - 牛客</title>
					<meta name="author" content="测试作者">
				</head>
				<body>
					<article>
						<h1>Go 后端一面面经</h1>
						<p>1. 讲一下 goroutine 调度模型。</p>
						<p>2. 解释一下 MySQL 索引失效场景。</p>
					</article>
				</body>
			</html>
		`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewHTTPProvider(
		WithHTTPClient(&http.Client{Timeout: 3 * time.Second}),
		WithSourceBaseURL(SourceNiuke, server.URL),
	).(*HTTPProvider)

	results, err := provider.Search(context.Background(), SearchRequest{
		Keyword:  "go 面经",
		Source:   SourceNiuke,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Go 后端一面面经" {
		t.Fatalf("unexpected search title: %s", results[0].Title)
	}
	if !strings.Contains(results[0].URL, "/discuss/123") {
		t.Fatalf("unexpected search url: %s", results[0].URL)
	}

	fetched, err := provider.Fetch(context.Background(), FetchRequest{
		URL:    results[0].URL,
		Source: SourceNiuke,
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if fetched.Title != "Go 后端一面面经 - 牛客" {
		t.Fatalf("unexpected fetch title: %s", fetched.Title)
	}
	if fetched.Author != "测试作者" {
		t.Fatalf("unexpected fetch author: %s", fetched.Author)
	}
	if !strings.Contains(fetched.Content, "goroutine 调度模型") {
		t.Fatalf("expected fetched content to contain article body, got: %s", fetched.Content)
	}
}

// TestHTTPProviderUnsupportedSource 验证不支持的数据源会返回明确错误。
func TestHTTPProviderUnsupportedSource(t *testing.T) {
	provider := NewHTTPProvider()
	_, err := provider.Search(context.Background(), SearchRequest{
		Keyword: "go",
		Source:  "unknown",
	})
	if err == nil || !strings.Contains(err.Error(), "暂不支持的数据源") {
		t.Fatalf("expected unsupported source error, got: %v", err)
	}
}
