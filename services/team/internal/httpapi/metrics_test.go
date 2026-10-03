package httpapi

// team-svc の /metrics(ADR-0220 §2・ADR-0406。AC-K8)。pokedex・calc・gateway と同じ
// services/internal/httpmetrics を使い、ServiceMonitor team が scrape できること。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEndpoint(t *testing.T) {
	h := NewHandler(newFakeStore())

	// 先に1回リクエストを流し、そのルートが数えられることを確かめる。
	if rec := serve(t, h, http.MethodGet, "/healthz", nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz = %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil) // 端末 ID のヘッダ無しで取れる(Prometheus は付けない)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain(Prometheus text format)", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{"http_requests_total", `path="/healthz"`, "http_request_duration_seconds"} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics に %s が無い", want)
		}
	}
	if strings.Contains(body, `path="/metrics"`) {
		t.Error("/metrics 自身が数えられている(ADR-0406 §3)")
	}
}
