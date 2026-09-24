package httpapi

// GET /metrics の統合テスト(ADR-0406 §1〜3。P7-1)。balance-svc の実際のハンドラ(New)に対して、
// /metrics が Prometheus text format を返すこと・通常の API 呼び出しが数えられること・/metrics 自体は
// 数えないことを確かめる。ヘルパは metrics_helpers_test.go(サービスごとに複製。ADR-0406 §2)。

import (
	"net/http"
	"strings"
	"testing"
)

// AC-S1: GET /metrics は端末ID・セッションIDのヘッダ無しで 200 と Prometheus text format を返す。
func TestMetricsEndpointServesPrometheusText(t *testing.T) {
	t.Parallel()
	h := newTestServer()
	if rec := metricsDo(h, http.MethodGet, "/healthz", nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz: status = %d, want 200", rec.Code)
	}
	assertMetricsFormat(t, metricsScrape(t, h))
}

// AC-S2: 通常の API 呼び出しが http_requests_total・http_request_duration_seconds に数えられる
// (path はルーティングパターン、status は実際に返したステータス)。
func TestMetricsCountsAPIRequests(t *testing.T) {
	t.Parallel()
	h := newTestServer()

	// ヘルスチェックは計測してよい(ADR-0406 §3。可用性 SLO の分母)。
	assertCountedOnce(t, h, metricsRequest{method: http.MethodGet, target: "/healthz"}, "/healthz")
	assertCountedOnce(t, h, metricsRequest{method: http.MethodGet, target: "/api/balance/healthz"}, "/api/balance/healthz")
	// ヘッダ無しの analyze は 400(requireRequestContext が返す)。エラー応答も最終ステータスで数える。
	assertCountedOnce(t, h, metricsRequest{
		method: http.MethodPost, target: analyzePath,
		header: http.Header{"Content-Type": []string{"application/json"}}, body: `{}`,
	}, analyzePath)
}

// AC-S3: /metrics 自体は数えない。
func TestMetricsDoesNotCountItself(t *testing.T) {
	t.Parallel()
	h := newTestServer()
	metricsDo(h, http.MethodGet, "/healthz", nil, "")
	assertMetricsNotSelfCounted(t, h)
}

// AC-S4: 計測を入れても既存の応答(ルート無しの 404 等)は変わらず、/metrics 以外の未知のパスは
// 引き続き 404(/metrics の追加で他のパスを拾わない)。
func TestMetricsDoesNotChangeUnknownRoutes(t *testing.T) {
	t.Parallel()
	h := newTestServer()
	for _, target := range []string{"/metricsx", "/metrics/extra", "/api/balance/metrics"} {
		rec := metricsDo(h, http.MethodGet, target, nil, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", target, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "# TYPE") {
			t.Errorf("GET %s: /metrics 以外のパスが text format を返した", target)
		}
	}
}
