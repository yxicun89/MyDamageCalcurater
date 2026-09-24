package httpapi

// GET /metrics の統合テスト(ADR-0406 §1〜3。P7-1)。calc-svc の2つの組み立て方(NewHandler・
// NewDeferredHandler。cmd/calc はどちらも使う)それぞれで、/metrics が Prometheus text format を返すこと・
// 通常の API 呼び出しが数えられること・/metrics 自体は数えないことを確かめる。
// ヘルパは metrics_helpers_test.go(サービスごとに複製。ADR-0406 §2)。

import (
	"net/http"
	"testing"

	"example.com/pokecalc/services/calc/internal/master"
)

// calcMetricsHandlers は calc-svc の本番の組み立て方2つ(マスタ読み込み済み・マスタ未準備)。
func calcMetricsHandlers(t *testing.T) map[string]http.Handler {
	t.Helper()
	return map[string]http.Handler{
		"NewHandler":         NewHandler(newFakeStore(t)),
		"NewDeferredHandler": NewDeferredHandler(func() master.Store { return nil }),
	}
}

// AC-S1: GET /metrics は端末ID・セッションIDのヘッダ無しで 200 と Prometheus text format を返す。
func TestMetricsEndpointServesPrometheusText(t *testing.T) {
	for name, h := range calcMetricsHandlers(t) {
		t.Run(name, func(t *testing.T) {
			if rec := metricsDo(h, http.MethodGet, "/healthz", nil, ""); rec.Code != http.StatusOK {
				t.Fatalf("GET /healthz: status = %d, want 200", rec.Code)
			}
			assertMetricsFormat(t, metricsScrape(t, h))
		})
	}
}

// AC-S2: 通常の API 呼び出しが数えられる。path はルーティングパターン(/api/pokedex/species/:key)で、
// 実際に来た未知の key はラベルに入らない。status はエラーハンドラが書いた最終ステータス。
func TestMetricsCountsAPIRequests(t *testing.T) {
	for name, h := range calcMetricsHandlers(t) {
		t.Run(name, func(t *testing.T) {
			assertCountedOnce(t, h, metricsRequest{method: http.MethodGet, target: "/healthz"}, "/healthz")
			assertCountedOnce(t, h, metricsRequest{method: http.MethodGet, target: "/readyz"}, "/readyz")
			// ヘッダ無しの計算は生成ラッパが error を返し、httpErrorHandler が 400 を書く。
			assertCountedOnce(t, h, metricsRequest{
				method: http.MethodPost, target: "/api/calc",
				header: http.Header{"Content-Type": []string{"application/json"}}, body: `{}`,
			}, "/api/calc")
			// ヘッダ付き(マスタ未準備なら 503、読み込み済みなら本文の検証結果)。
			assertCountedOnce(t, h, metricsRequest{
				method: http.MethodPost, target: "/api/calc/bulk", header: validHeaders(), body: `{}`,
			}, "/api/calc/bulk")
			// 担当外の pokedex 操作は 404。path はパターンで、未知の key(zz-raw-key-9101)は入らない。
			assertCountedOnce(t, h, metricsRequest{
				method: http.MethodGet, target: "/api/pokedex/species/zz-raw-key-9101", header: validHeaders(),
			}, "/api/pokedex/species/:key")
		})
	}
}

// AC-S3: /metrics 自体は数えない。
func TestMetricsDoesNotCountItself(t *testing.T) {
	for name, h := range calcMetricsHandlers(t) {
		t.Run(name, func(t *testing.T) {
			metricsDo(h, http.MethodGet, "/healthz", nil, "")
			assertMetricsNotSelfCounted(t, h)
		})
	}
}
