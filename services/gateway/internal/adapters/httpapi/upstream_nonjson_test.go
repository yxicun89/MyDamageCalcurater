package httpapi

// 上流(/api/* のサービス)が JSON でない 5xx を返したときは、gateway が自前の
// 503 upstream_unavailable(Error 形)に正規化する(issue #514。ADR-0802)。上流の JSON エラーと
// 4xx、および assets・Web 上流の応答は素通しのまま。

import (
	"net/http"
	"strings"
	"testing"
)

func TestNonJSONUpstream5xxIsNormalized(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		upstream func(*testEnv) *fakeUpstream
		resp     upstreamResponse
	}{
		{"calc 502 text/html", http.MethodPost, "/api/calc", func(e *testEnv) *fakeUpstream { return e.calc },
			upstreamResponse{status: http.StatusBadGateway, contentType: "text/html", body: "<h1>Bad Gateway</h1>"}},
		{"pokedex 504 text/plain", http.MethodGet, "/api/pokedex/natures", func(e *testEnv) *fakeUpstream { return e.pokedex },
			upstreamResponse{status: http.StatusGatewayTimeout, contentType: "text/plain", body: "Gateway Timeout"}},
		{"judge 502 text/html", http.MethodGet, "/api/judge/healthz", func(e *testEnv) *fakeUpstream { return e.judge },
			upstreamResponse{status: http.StatusBadGateway, contentType: "text/html", body: "<h1>Bad Gateway</h1>"}},
		{"balance 504 text/plain", http.MethodGet, "/api/balance/healthz", func(e *testEnv) *fakeUpstream { return e.balance },
			upstreamResponse{status: http.StatusGatewayTimeout, contentType: "text/plain", body: "Gateway Timeout"}},
		{"speed 500 text/plain", http.MethodGet, "/api/speed/healthz", func(e *testEnv) *fakeUpstream { return e.speed },
			upstreamResponse{status: http.StatusInternalServerError, contentType: "text/plain", body: "Gateway Timeout"}},
		{"Content-Type 無しの 503", http.MethodGet, "/api/judge/healthz", func(e *testEnv) *fakeUpstream { return e.judge },
			upstreamResponse{status: http.StatusServiceUnavailable}},
		{"Retry-After は引き継がない", http.MethodGet, "/api/judge/healthz", func(e *testEnv) *fakeUpstream { return e.judge },
			upstreamResponse{status: http.StatusServiceUnavailable, contentType: "text/html", body: "Gateway Timeout",
				header: http.Header{"Retry-After": []string{"30"}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := buildTestEnv(t, enabledUpstreams{balance: true, speed: true, judge: true})
			up := tc.upstream(env)
			up.respond(tc.resp)

			rec := serve(t, env.handler, tc.method, tc.path, withOrigin(validHeaders(), allowedOrigin), []byte(`{}`))

			if got := len(up.requests()); got != 1 {
				t.Fatalf("上流に届いた回数 = %d, want 1(上流に届いたうえで正規化されること)", got)
			}
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503; body=%s", rec.Code, rec.Body.String())
			}
			if got := decodeErrorBody(t, rec); got.Code != "upstream_unavailable" {
				t.Errorf("code = %q, want upstream_unavailable", got.Code)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			for _, leaked := range []string{"Bad Gateway", "Gateway Timeout"} {
				if strings.Contains(rec.Body.String(), leaked) {
					t.Errorf("上流の本文 %q が漏れている: %s", leaked, rec.Body.String())
				}
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("Retry-After = %q, want 無し", got)
			}
			assertCORSAllowed(t, rec, allowedOrigin)
		})
	}
}

// 上流の JSON エラー(5xx も含む)・4xx・assets 上流の応答は素通しのまま。
func TestNonJSONNormalizationLeavesJSONAnd4xxAlone(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		header   http.Header
		upstream func(*testEnv) *fakeUpstream
		resp     upstreamResponse
	}{
		{"application/json の 500", "/api/pokedex/natures", validHeaders(), func(e *testEnv) *fakeUpstream { return e.pokedex },
			upstreamResponse{status: http.StatusInternalServerError, contentType: "application/json", body: `{"code":"internal","message":"x"}`}},
		{"application/json; charset=utf-8 の 503", "/api/pokedex/natures", validHeaders(), func(e *testEnv) *fakeUpstream { return e.pokedex },
			upstreamResponse{status: http.StatusServiceUnavailable, contentType: "application/json; charset=utf-8", body: `{"code":"master_unavailable","message":"x"}`}},
		{"application/problem+json の 500", "/api/pokedex/natures", validHeaders(), func(e *testEnv) *fakeUpstream { return e.pokedex },
			upstreamResponse{status: http.StatusInternalServerError, contentType: "application/problem+json", body: `{"code":"internal","message":"x"}`}},
		{"4xx の非 JSON", "/api/pokedex/natures", validHeaders(), func(e *testEnv) *fakeUpstream { return e.pokedex },
			upstreamResponse{status: http.StatusNotFound, contentType: "text/plain", body: "nope"}},
		{"assets の 502", "/assets/a.webp", http.Header{}, func(e *testEnv) *fakeUpstream { return e.assets },
			upstreamResponse{status: http.StatusBadGateway, contentType: "text/html", body: "<h1>x</h1>"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			tc.upstream(env).respond(tc.resp)

			rec := serve(t, env.handler, http.MethodGet, tc.path, tc.header, nil)

			if rec.Code != tc.resp.status {
				t.Fatalf("status = %d, want %d(素通し)", rec.Code, tc.resp.status)
			}
			if rec.Body.String() != tc.resp.body {
				t.Errorf("body = %q, want %q(素通し)", rec.Body.String(), tc.resp.body)
			}
			if ct := rec.Header().Get("Content-Type"); ct != tc.resp.contentType {
				t.Errorf("Content-Type = %q, want %q", ct, tc.resp.contentType)
			}
		})
	}
}
