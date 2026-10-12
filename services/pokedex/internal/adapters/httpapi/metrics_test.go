package httpapi_test

// GET /metrics の統合テスト(ADR-0406 §1〜3。P7-1)。pokedex-svc の実際のハンドラ(NewHandler。DB の代わりに
// storetest の偽の Querier)に対して、/metrics が Prometheus text format を返すこと・通常の API 呼び出しが
// 数えられること・/metrics 自体は数えないことを確かめる。ヘルパは metrics_helpers_test.go(サービスごとに複製)。

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/storetest"
)

func pokedexHeaders() http.Header {
	return http.Header{"X-Device-Id": []string{testDeviceID}, "X-Session-Id": []string{testSessionID}}
}

// AC-S1: GET /metrics は端末ID・セッションIDのヘッダ無しで 200 と Prometheus text format を返す(DB に触れない)。
func TestMetricsEndpointServesPrometheusText(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)
	if rec := metricsDo(h, http.MethodGet, "/healthz", nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz: status = %d, want 200", rec.Code)
	}
	assertMetricsFormat(t, metricsScrape(t, h))
	if len(q.Calls) != 0 {
		t.Errorf("/metrics・/healthz が DB に触れた: %+v", q.Calls)
	}
}

// AC-S2: 通常の API 呼び出しが数えられる。path はルーティングパターン(/api/pokedex/species/:key)で、
// 実際に来た key はラベルに入らない。status は実際に返したステータス(エラー応答を含む)。
func TestMetricsCountsAPIRequests(t *testing.T) {
	h := newHandler(t, storetest.New())

	assertCountedOnce(t, h, metricsRequest{method: http.MethodGet, target: "/healthz"}, "/healthz")
	assertCountedOnce(t, h, metricsRequest{
		method: http.MethodGet, target: "/api/pokedex/species/zz-raw-key-9201", header: pokedexHeaders(),
	}, "/api/pokedex/species/:key")
	// ヘッダ無しは生成ラッパが error を返し、httpErrorHandler が書いた最終ステータスで数える。
	assertCountedOnce(t, h, metricsRequest{
		method: http.MethodGet, target: "/api/pokedex/moves/zz-raw-key-9202",
	}, "/api/pokedex/moves/:key")
	// 静的セグメント(/moves/batch)は :key に食われず、自分のパターンで数える。
	assertCountedOnce(t, h, metricsRequest{
		method: http.MethodGet, target: "/api/pokedex/moves/batch?ids=zz-raw-query", header: pokedexHeaders(),
	}, "/api/pokedex/moves/batch")
	// 担当外の calc 操作は 404。
	assertCountedOnce(t, h, metricsRequest{method: http.MethodPost, target: "/api/calc", header: pokedexHeaders(), body: `{}`}, "/api/calc")
}

// AC-S2b: どのルートにも当たらないパスは 404 で数えるが、生のパスはラベルに入らない(カーディナリティ対策)。
func TestMetricsUnknownRouteDoesNotLeakRawPath(t *testing.T) {
	h := newHandler(t, storetest.New())
	before := metricsParse(t, metricsScrape(t, h))
	rec := metricsDo(h, http.MethodGet, "/zz-unknown-route/zz-raw-segment-9203", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	after := metricsParse(t, metricsScrape(t, h))

	status := map[string]string{"method": http.MethodGet, "status": strconv.Itoa(http.StatusNotFound)}
	if d := metricsSum(after, metricsRequestsTotal, status) - metricsSum(before, metricsRequestsTotal, status); d != 1 {
		t.Errorf("%s%v の増分 = %v, want 1", metricsRequestsTotal, status, d)
	}
	for _, s := range after {
		for k, v := range s.labels {
			if strings.Contains(v, "zz-unknown-route") || strings.Contains(v, "zz-raw-segment-9203") {
				t.Errorf("%s{%s=%q}: 生のパスがラベルに入っている", s.name, k, v)
			}
		}
	}
}

// AC-S3: /metrics 自体は数えない。
func TestMetricsDoesNotCountItself(t *testing.T) {
	h := newHandler(t, storetest.New())
	metricsDo(h, http.MethodGet, "/healthz", nil, "")
	assertMetricsNotSelfCounted(t, h)
}
