package httpapi

// 運用面の受け入れ条件(issue #216・#244・#246・#217)。
//   - #216: /metrics は専用ハンドラ(NewHandlers の Metrics)だけが答え、公開側(NewHandler)では 404。
//   - #244: メトリクスの path ラベルはルート種別ごとの固定文字列。
//   - #246: X-Request-Id の生成・検証・上流への転送・応答ヘッダ。
//   - #217: /healthz にビルドの version が出る。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/reqlog"
	"example.com/pokecalc/services/internal/version"
)

// 公開側の /metrics は上流へ転送せず、WebURL があっても Web へ流さず、Error 形式の 404 not_found。
func TestPublicHandlerHidesMetrics(t *testing.T) {
	for name, newEnv := range map[string]func(*testing.T, ...func(*Config)) *testEnv{
		"WebURL 無し": newTestEnv,
		"WebURL 有り": newWebTestEnv,
	} {
		t.Run(name, func(t *testing.T) {
			env := newEnv(t)
			for _, path := range []string{"/metrics", "/metrics/"} {
				rec := metricsDo(env.handler, http.MethodGet, path, nil, "")
				if rec.Code != http.StatusNotFound {
					t.Fatalf("GET %s: status = %d, want 404", path, rec.Code)
				}
				var body map[string]string
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["code"] != "not_found" {
					t.Errorf("GET %s: body = %s, want Error 形式 not_found", path, rec.Body.String())
				}
			}
			env.assertNoUpstreamReached(t)
		})
	}
}

// メトリクス専用ハンドラは /metrics だけに答える。
func TestMetricsHandlerServesOnlyMetrics(t *testing.T) {
	env := newTestEnv(t)
	metricsDo(env.handler, http.MethodGet, "/healthz", nil, "") // 系列ができてから HELP/TYPE が出る
	assertMetricsFormat(t, metricsScrape(t, env.metrics))
	for _, path := range []string{"/", "/healthz", "/api/calc"} {
		if rec := metricsDo(env.metrics, http.MethodGet, path, nil, ""); rec.Code != http.StatusNotFound {
			t.Errorf("メトリクス専用側の GET %s: status = %d, want 404", path, rec.Code)
		}
	}
}

// #244: path ラベルは routeKind 由来の固定文字列(未知パスは none)。生のパスは入らない。
func TestMetricsPathLabelIsRouteKind(t *testing.T) {
	env := newWebTestEnv(t)
	h := combinedHandler(env)
	tests := []struct {
		req       metricsRequest
		wantPath  string
		wantCode  int
		rawNeedle string
	}{
		{metricsRequest{method: http.MethodGet, target: "/healthz"}, "healthz", 200, ""},
		{metricsRequest{method: http.MethodPost, target: "/api/calc", header: validHeaders(), body: `{}`}, "calc", 200, ""},
		{metricsRequest{method: http.MethodGet, target: "/api/pokedex/species/zz-raw-9401", header: validHeaders()}, "pokedex", 200, "zz-raw-9401"},
		{metricsRequest{method: http.MethodGet, target: "/assets/zz-raw-9402.webp"}, "assets", 200, "zz-raw-9402"},
		{metricsRequest{method: http.MethodGet, target: "/zz-web-9403"}, "web", 200, "zz-web-9403"},
		{metricsRequest{method: http.MethodGet, target: "/api/zz-raw-9404", header: validHeaders()}, "none", 404, "zz-raw-9404"},
		{metricsRequest{method: http.MethodPost, target: "/api/calc", body: `{}`}, "calc", 400, ""},
	}
	for _, tt := range tests {
		t.Run(tt.req.method+" "+tt.req.target, func(t *testing.T) {
			before := metricsParse(t, metricsScrape(t, h))
			rec := metricsDo(h, tt.req.method, tt.req.target, tt.req.header, tt.req.body)
			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			after := metricsParse(t, metricsScrape(t, h))
			labels := map[string]string{"method": tt.req.method, "path": tt.wantPath, "status": itoa(tt.wantCode)}
			if d := metricsSum(after, metricsRequestsTotal, labels) - metricsSum(before, metricsRequestsTotal, labels); d != 1 {
				t.Errorf("%s%v の増分 = %v, want 1", metricsRequestsTotal, labels, d)
			}
			hist := map[string]string{"method": tt.req.method, "path": tt.wantPath}
			if d := metricsSum(after, metricsDurationSeconds+"_count", hist) - metricsSum(before, metricsDurationSeconds+"_count", hist); d != 1 {
				t.Errorf("%s_count%v の増分 = %v, want 1", metricsDurationSeconds, hist, d)
			}
			for _, s := range after {
				for _, v := range s.labels {
					if tt.rawNeedle != "" && strings.Contains(v, tt.rawNeedle) {
						t.Errorf("%s のラベル値 %q に生のパス片 %q が入っている", s.name, v, tt.rawNeedle)
					}
				}
			}
		})
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// #246: ID が無ければ生成して上流へ転送し、応答ヘッダにも付ける。
func TestRequestIDGeneratedAndForwarded(t *testing.T) {
	env := newTestEnv(t)
	rec := serve(t, env.handler, http.MethodPost, "/api/calc", validHeaders(), []byte(`{}`))
	id := rec.Header().Get(reqlog.Header)
	if !reqlog.ValidID(id) {
		t.Fatalf("応答の %s = %q, want 生成された ID", reqlog.Header, id)
	}
	reqs := env.calc.requests()
	if len(reqs) != 1 || reqs[0].Header.Get(reqlog.Header) != id {
		t.Errorf("上流に届いた %s = %v, want %q", reqlog.Header, reqs, id)
	}
}

func TestRequestIDClientValueValidated(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		wantSame bool
	}{
		{"有効な値は通す", "client-abc-123", true},
		{"65 文字は作り直す", strings.Repeat("a", 65), false},
		{"空白入りは作り直す", "bad id", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			h := validHeaders()
			h.Set(reqlog.Header, tt.id)
			rec := serve(t, env.handler, http.MethodPost, "/api/calc", h, []byte(`{}`))
			got := rec.Header().Get(reqlog.Header)
			if (got == tt.id) != tt.wantSame || !reqlog.ValidID(got) {
				t.Errorf("応答の ID = %q(クライアント %q)", got, tt.id)
			}
			if reqs := env.calc.requests(); len(reqs) != 1 || reqs[0].Header.Get(reqlog.Header) != got {
				t.Errorf("上流の ID = %v, want %q", reqs, got)
			}
		})
	}
}

// gateway 自身のエラー応答・上流の 503 にも ID が付き、上流が付けた ID と二重にならない。
func TestRequestIDOnOwnErrorsAndNotDuplicated(t *testing.T) {
	env := newTestEnv(t)
	rec := serve(t, env.handler, http.MethodGet, "/nope", http.Header{}, nil)
	if rec.Code != http.StatusNotFound || !reqlog.ValidID(rec.Header().Get(reqlog.Header)) {
		t.Errorf("gateway 自身の 404: status=%d id=%q", rec.Code, rec.Header().Get(reqlog.Header))
	}

	env.calc.respond(upstreamResponse{status: 200, contentType: "application/json", body: `{}`, header: http.Header{reqlog.Header: {"upstream-own-id"}}})
	rec = serve(t, env.handler, http.MethodPost, "/api/calc", validHeaders(), []byte(`{}`))
	if got := rec.Header().Values(reqlog.Header); len(got) != 1 || got[0] == "upstream-own-id" {
		t.Errorf("応答の %s = %v, want gateway の ID が 1 つだけ", reqlog.Header, got)
	}
}

// #217: /healthz は status と version(version.Version)を返す。
func TestHealthzIncludesVersion(t *testing.T) {
	prev := version.Version
	version.Version = "abc1234"
	t.Cleanup(func() { version.Version = prev })

	env := newTestEnv(t)
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["version"] != "abc1234" || len(body) != 2 {
		t.Errorf("body = %v, want {status: ok, version: abc1234}", body)
	}
}
