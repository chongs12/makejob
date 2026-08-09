package telemetry

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewHTTPServerEndpoints(t *testing.T) {
	srv := NewHTTPServer(0, prometheus.DefaultGatherer)
	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	cases := []struct {
		path       string
		wantStatus int
	}{
		{"/healthz", http.StatusOK},
		{"/readyz", http.StatusOK},
		{"/metrics", http.StatusOK},
		{"/debug/pprof/", http.StatusOK},
	}
	for _, c := range cases {
		resp, err := http.Get(ts.URL + c.path)
		if err != nil {
			t.Fatalf("GET %s: %v", c.path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != c.wantStatus {
			t.Errorf("GET %s status = %d, want %d (body: %q)", c.path, resp.StatusCode, c.wantStatus, string(body))
		}
	}
}

func TestNewHTTPServerMetricsContent(t *testing.T) {
	srv := NewHTTPServer(0, prometheus.DefaultGatherer)
	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// /metrics 应返回 Prometheus 文本格式（含 # HELP / # TYPE 或 go_ 前缀的 runtime 指标）
	if len(body) == 0 {
		t.Fatal("/metrics body is empty")
	}
}
