package httpapi

// CORS の受け入れテスト(ADR-0202 §6。AC-G6)。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// プリフライトの応答ヘッダの期待値(ADR-0202 §6)。
const (
	// DELETE は P5-3(record-svc の全削除 API)で足した(ADR-0209 §10-2・AC-P8)。
	// PUT は P5-4(team-svc の updateTeam)で足した(ADR-0213・AC-T9)。
	wantAllowMethods = "GET, POST, PUT, DELETE, OPTIONS"
	wantAllowHeaders = "Content-Type, X-Device-Id, X-Session-Id"
	wantMaxAge       = "600"
)

func varyHasOrigin(h http.Header) bool {
	for _, v := range h.Values("Vary") {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), "Origin") {
				return true
			}
		}
	}
	return false
}

// assertCORSAllowed は許可オリジンへの応答に ACAO=origin・Vary: Origin があり、credentials を許さないことを確かめる。
func assertCORSAllowed(t *testing.T, rec *httptest.ResponseRecorder, origin string) {
	t.Helper()
	if got := rec.Header().Values("Access-Control-Allow-Origin"); len(got) != 1 || got[0] != origin {
		t.Errorf("Access-Control-Allow-Origin = %q, want [%q]", got, origin)
	}
	if !varyHasOrigin(rec.Header()) {
		t.Errorf("Vary に Origin が無い: %q", rec.Header().Values("Vary"))
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want 無し(認証なし)", got)
	}
}

// assertNoCORS は CORS の許可ヘッダが1つも無いことを確かめる。
func assertNoCORS(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	for _, name := range []string{
		"Access-Control-Allow-Origin", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers",
		"Access-Control-Max-Age", "Access-Control-Allow-Credentials",
	} {
		if got := rec.Header().Values(name); len(got) != 0 {
			t.Errorf("%s = %q, want 無し", name, got)
		}
	}
}

func withOrigin(h http.Header, origin string) http.Header {
	h = h.Clone()
	h.Set("Origin", origin)
	return h
}

// AC-G6: 許可オリジンに完全一致する Origin の単純リクエストにだけ ACAO=そのオリジン と Vary: Origin を付ける。
// 上流の応答・gateway 自身のエラー・/assets のどれにも付く(ブラウザがエラー本文を読めるように)。
func TestCORSSimpleRequests(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		header  http.Header
		allowed string // 空なら CORS ヘッダ無しを期待
	}{
		{"許可オリジン(上流の応答)", http.MethodPost, "/api/calc", withOrigin(validHeaders(), allowedOrigin), allowedOrigin},
		{"2つ目の許可オリジン", http.MethodGet, "/api/pokedex/natures", withOrigin(validHeaders(), allowedOriginLocal), allowedOriginLocal},
		{"許可オリジン(gateway の missing_header)", http.MethodPost, "/api/calc",
			withOrigin(http.Header{"Content-Type": []string{"application/json"}}, allowedOrigin), allowedOrigin},
		{"許可オリジン(gateway の not_found)", http.MethodGet, "/api/unknown", withOrigin(validHeaders(), allowedOrigin), allowedOrigin},
		{"許可オリジン(/assets)", http.MethodGet, "/assets/0445-000.webp", withOrigin(http.Header{}, allowedOrigin), allowedOrigin},
		{"許可外のオリジン", http.MethodPost, "/api/calc", withOrigin(validHeaders(), disallowedOrigin), ""},
		{"末尾スラッシュ付きは完全一致しない", http.MethodPost, "/api/calc", withOrigin(validHeaders(), allowedOrigin+"/"), ""},
		{"ポート違いは完全一致しない", http.MethodPost, "/api/calc", withOrigin(validHeaders(), "http://localhost:5174"), ""},
		{"null オリジン", http.MethodPost, "/api/calc", withOrigin(validHeaders(), "null"), ""},
		{"Origin 無し", http.MethodPost, "/api/calc", validHeaders(), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			var body []byte
			if tt.method == http.MethodPost {
				body = []byte(`{}`)
			}
			rec := serve(t, env.handler, tt.method, tt.path, tt.header, body)
			if tt.allowed == "" {
				assertNoCORS(t, rec)
				return
			}
			assertCORSAllowed(t, rec, tt.allowed)
		})
	}
}

// AC-G6: 許可オリジンのプリフライトは 204 で Allow-Methods / Allow-Headers / Max-Age を返し、上流には届かない。
// 許可外のオリジン・CORS 未設定のときは CORS ヘッダ無しの 204(ブラウザが拒否する)。
func TestCORSPreflight(t *testing.T) {
	preflight := func(origin, method string) http.Header {
		return http.Header{
			"Origin":                         []string{origin},
			"Access-Control-Request-Method":  []string{method},
			"Access-Control-Request-Headers": []string{"content-type,x-device-id,x-session-id"},
		}
	}
	tests := []struct {
		name    string
		noCORS  bool // Config.CORSAllowedOrigins を空にする
		path    string
		header  http.Header
		allowed string
	}{
		{"calc への POST", false, "/api/calc", preflight(allowedOrigin, http.MethodPost), allowedOrigin},
		{"reverse への POST(2つ目のオリジン)", false, "/api/calc/reverse", preflight(allowedOriginLocal, http.MethodPost), allowedOriginLocal},
		{"pokedex への GET", false, "/api/pokedex/species", preflight(allowedOrigin, http.MethodGet), allowedOrigin},
		{"許可外のオリジン", false, "/api/calc", preflight(disallowedOrigin, http.MethodPost), ""},
		{"CORS 未設定", true, "/api/calc", preflight(allowedOrigin, http.MethodPost), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mutate []func(*Config)
			if tt.noCORS {
				mutate = append(mutate, func(c *Config) { c.CORSAllowedOrigins = nil })
			}
			env := newTestEnv(t, mutate...)
			rec := serve(t, env.handler, http.MethodOptions, tt.path, tt.header, nil)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
			}
			if rec.Body.Len() != 0 {
				t.Errorf("204 に本文がある: %q", rec.Body.String())
			}
			env.assertNoUpstreamReached(t)
			if tt.allowed == "" {
				assertNoCORS(t, rec)
				return
			}
			assertCORSAllowed(t, rec, tt.allowed)
			if got := rec.Header().Get("Access-Control-Allow-Methods"); got != wantAllowMethods {
				t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, wantAllowMethods)
			}
			if got := rec.Header().Get("Access-Control-Allow-Headers"); got != wantAllowHeaders {
				t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, wantAllowHeaders)
			}
			if got := rec.Header().Get("Access-Control-Max-Age"); got != wantMaxAge {
				t.Errorf("Access-Control-Max-Age = %q, want %q", got, wantMaxAge)
			}
		})
	}
}

// 必須1: 上流が独自の Access-Control-* を返しても、gateway の外へは出さない。
// 許可オリジンなら gateway 自身の ACAO(ちょうど1つ)に付け替え、許可外なら CORS ヘッダを一切付けない
// (calc・/assets の両方。上流が `*` を返す場合と、別オリジンを反射して返す場合の両方を確かめる)。
func TestCORSStripsUpstreamHeaders(t *testing.T) {
	upstreamCORSVariants := []struct {
		name   string
		header http.Header
	}{
		{"上流が * を返す", http.Header{"Access-Control-Allow-Origin": []string{"*"}}},
		{"上流が別オリジンを反射して返す", http.Header{
			"Access-Control-Allow-Origin": []string{disallowedOrigin},
			"Vary":                        []string{"Origin"},
		}},
	}
	routes := []struct {
		name         string
		method       string
		path         string
		target       string // "calc" / "assets"
		baseHeader   http.Header
		requiresBody bool
	}{
		{"calc", http.MethodPost, "/api/calc", "calc", validHeaders(), true},
		{"assets", http.MethodGet, "/assets/0445-000.webp", "assets", http.Header{}, false},
	}
	for _, route := range routes {
		for _, variant := range upstreamCORSVariants {
			t.Run(route.name+"/"+variant.name+"/許可オリジン", func(t *testing.T) {
				env := newTestEnv(t)
				target := map[string]*fakeUpstream{"calc": env.calc, "assets": env.assets}[route.target]
				target.respond(upstreamResponse{status: http.StatusOK, contentType: "application/json", body: `{}`, header: variant.header})
				var body []byte
				if route.requiresBody {
					body = []byte(`{}`)
				}
				rec := serve(t, env.handler, route.method, route.path, withOrigin(route.baseHeader, allowedOrigin), body)
				assertCORSAllowed(t, rec, allowedOrigin)
			})
			t.Run(route.name+"/"+variant.name+"/許可外オリジン", func(t *testing.T) {
				env := newTestEnv(t)
				target := map[string]*fakeUpstream{"calc": env.calc, "assets": env.assets}[route.target]
				target.respond(upstreamResponse{status: http.StatusOK, contentType: "application/json", body: `{}`, header: variant.header})
				var body []byte
				if route.requiresBody {
					body = []byte(`{}`)
				}
				rec := serve(t, env.handler, route.method, route.path, withOrigin(route.baseHeader, disallowedOrigin), body)
				assertNoCORS(t, rec)
			})
		}
	}
}

// 必須2: 上流に接続できない・タイムアウトした 503(ReverseProxy.ErrorHandler)と、panic 回復の
// 500(recoverMiddleware)にも CORS を付ける。許可オリジンなら ACAO がちょうど1つ、許可外なら無し
// (以前はこれらの経路だけ CORS が抜け落ちる退行があった)。
func TestCORSOnUpstreamFailuresAndPanic(t *testing.T) {
	const shortTimeout = 100 * time.Millisecond
	tests := []struct {
		name       string
		mutate     func(*Config)
		wantStatus int
	}{
		{"calc に接続できない(503 upstream_unavailable)", func(c *Config) { c.CalcURL = closedServerURL(t) }, http.StatusServiceUnavailable},
		{"calc がタイムアウト(503 upstream_unavailable)", func(c *Config) {
			c.CalcURL = slowServerURL(t, 5*time.Second)
			c.UpstreamTimeout = shortTimeout
		}, http.StatusServiceUnavailable},
		{"panicTransport(500 internal)", func(c *Config) { c.transport = panicTransport{} }, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name+"/許可オリジン", func(t *testing.T) {
			env := newTestEnv(t, tt.mutate)
			rec := serve(t, env.handler, http.MethodPost, "/api/calc", withOrigin(validHeaders(), allowedOrigin), []byte(`{}`))
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			assertCORSAllowed(t, rec, allowedOrigin)
		})
		t.Run(tt.name+"/許可外オリジン", func(t *testing.T) {
			env := newTestEnv(t, tt.mutate)
			rec := serve(t, env.handler, http.MethodPost, "/api/calc", withOrigin(validHeaders(), disallowedOrigin), []byte(`{}`))
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			assertNoCORS(t, rec)
		})
	}
}

// AC-G6: CORS 未設定なら、許可オリジンと同じ Origin の単純リクエストにも CORS ヘッダを付けない。
func TestCORSDisabledWhenNoOrigins(t *testing.T) {
	env := newTestEnv(t, func(c *Config) { c.CORSAllowedOrigins = nil })
	rec := serve(t, env.handler, http.MethodPost, "/api/calc", withOrigin(validHeaders(), allowedOrigin), []byte(`{}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200(上流の応答)", rec.Code)
	}
	assertNoCORS(t, rec)
}
