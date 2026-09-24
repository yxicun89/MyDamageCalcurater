package httpapi

// GET /metrics の統合テスト(ADR-0406 §1〜3。P7-1)。gateway の実際のハンドラ(NewHandler。上流は偽物)に対して、
// /metrics が gateway 自身の Prometheus text format を返し上流へ転送しないこと・転送したリクエストが数えられる
// こと・/metrics 自体は数えないことを確かめる。ヘルパは metrics_helpers_test.go(サービスごとに複製)。
//
// gateway はルートを1つ(/*)しか持たないため、path ラベルの具体的な値はここでは固定しない。固定するのは
// 「実際に来たパス(未知の ID 等)がラベルに入らない」ことだけ(ADR-0406 §1 のカーディナリティ対策)。

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// AC-S1: GET /metrics はヘッダ無しで 200 と Prometheus text format を返し、どの上流にも転送しない
// (WebURL を設定していても Web の静的配信へ流さない。ADR-0205 の予約パス以外の GET が Web に行く挙動より優先)。
func TestMetricsEndpointServesPrometheusText(t *testing.T) {
	for name, newEnv := range map[string]func(*testing.T, ...func(*Config)) *testEnv{
		"WebURL 無し": newTestEnv,
		"WebURL 有り": newWebTestEnv,
	} {
		t.Run(name, func(t *testing.T) {
			env := newEnv(t)
			if rec := metricsDo(env.handler, http.MethodGet, "/healthz", nil, ""); rec.Code != http.StatusOK {
				t.Fatalf("GET /healthz: status = %d, want 200", rec.Code)
			}
			assertMetricsFormat(t, metricsScrape(t, env.handler))
			env.assertNoUpstreamReached(t)
		})
	}
}

// gatewayCountedOnce は req を1回送ると http_requests_total{method,status=実際の応答}(path は問わない)と
// http_request_duration_seconds_count{method} がちょうど1増え、rawSegment がどのラベルにも入らないことを確かめる。
func gatewayCountedOnce(t *testing.T, h http.Handler, req metricsRequest, rawSegment string) *http.Response {
	t.Helper()
	before := metricsParse(t, metricsScrape(t, h))
	rec := metricsDo(h, req.method, req.target, req.header, req.body)
	after := metricsParse(t, metricsScrape(t, h))

	counter := map[string]string{"method": req.method, "status": strconv.Itoa(rec.Code)}
	if d := metricsSum(after, metricsRequestsTotal, counter) - metricsSum(before, metricsRequestsTotal, counter); d != 1 {
		t.Errorf("%s %s(応答 %d)の後 %s%v の増分 = %v, want 1", req.method, req.target, rec.Code, metricsRequestsTotal, counter, d)
	}
	name := metricsDurationSeconds + "_count"
	method := map[string]string{"method": req.method}
	if d := metricsSum(after, name, method) - metricsSum(before, name, method); d != 1 {
		t.Errorf("%s %s の後 %s%v の増分 = %v, want 1", req.method, req.target, name, method, d)
	}
	for _, s := range after {
		for k, v := range s.labels {
			if rawSegment != "" && strings.Contains(v, rawSegment) {
				t.Errorf("%s{%s=%q}: 実際に来たパスの一部 %q がラベルに入っている(ルーティングパターンを使うこと)", s.name, k, v, rawSegment)
			}
		}
	}
	return rec.Result()
}

// AC-S2: 上流へ転送したリクエスト・gateway 自身が答えたエラーが数えられる。
func TestMetricsCountsAPIRequests(t *testing.T) {
	env := newWebTestEnv(t)
	h := env.handler

	gatewayCountedOnce(t, h, metricsRequest{method: http.MethodGet, target: "/healthz"}, "")
	// calc へ転送(偽の上流が 200)。
	gatewayCountedOnce(t, h, metricsRequest{method: http.MethodPost, target: "/api/calc", header: validHeaders(), body: `{}`}, "")
	// pokedex へ転送。未知の key はラベルに入らない。
	gatewayCountedOnce(t, h, metricsRequest{
		method: http.MethodGet, target: "/api/pokedex/species/zz-raw-key-9301", header: validHeaders(),
	}, "zz-raw-key-9301")
	// ヘッダ無しの /api/* は gateway 自身が 400。
	gatewayCountedOnce(t, h, metricsRequest{method: http.MethodPost, target: "/api/calc", body: `{}`}, "")
	// 予約パスの未知のルートは 404。生のパスはラベルに入らない。
	gatewayCountedOnce(t, h, metricsRequest{method: http.MethodGet, target: "/api/zz-raw-segment-9302", header: validHeaders()}, "zz-raw-segment-9302")
	// Web の静的配信へ転送(予約パス以外の GET)。
	gatewayCountedOnce(t, h, metricsRequest{method: http.MethodGet, target: "/zz-web-page-9303"}, "zz-web-page-9303")

	if got := len(env.calc.requests()); got != 1 {
		t.Errorf("calc に届いた回数 = %d, want 1(計測が転送を増やしたり減らしたりしない)", got)
	}
}

// AC-S4: net/http はどんな文字列でも HTTP メソッドとして受け取ってしまうため、gateway の入口でも
// 標準メソッド以外は method="OTHER" に正規化する(カーディナリティ対策。ADR-0406 §1)。gatewayCountedOnce
// は req.method をそのままラベル値として期待するため使えず、ここでは直接送って集計する。
func TestMetricsUnknownMethodsNormalizeToOther(t *testing.T) {
	env := newWebTestEnv(t)
	h := env.handler

	for _, method := range []string{"XMETHOD1", "XMETHOD2", "XMETHOD3"} {
		metricsDo(h, method, "/healthz", nil, "")
	}

	samples := metricsParse(t, metricsScrape(t, h))
	if got := metricsSum(samples, metricsRequestsTotal, map[string]string{"method": "OTHER"}); got != 3 {
		t.Errorf(`%s{method="OTHER"} の合計 = %v, want 3`, metricsRequestsTotal, got)
	}
	for _, s := range samples {
		if v, ok := s.labels["method"]; ok && strings.Contains(v, "XMETHOD") {
			t.Errorf("%s{method=%q}: 生のメソッド文字列がラベルに入っている", s.name, v)
		}
	}
}

// AC-S3: /metrics 自体は数えない。
func TestMetricsDoesNotCountItself(t *testing.T) {
	env := newWebTestEnv(t)
	metricsDo(env.handler, http.MethodGet, "/healthz", nil, "")
	assertMetricsNotSelfCounted(t, env.handler)
	env.assertNoUpstreamReached(t)
}
