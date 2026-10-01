package httpapi

// Web の静的配信を gateway の後ろに置く受け入れテスト(ADR-0205。AC-W1〜AC-W7)。
// WebURL が設定されているとき、/api・/api/*・/assets・/assets/*・/healthz・/internal・/internal/* の
// どれにも当たらない GET / HEAD を Web の上流(nginx)へ転送する。未設定なら従来どおり 404 not_found。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// webHTML は Web の偽物が返す本文(架空の HTML)。
const webHTML = "<!doctype html><title>test</title>"

// webResponse は nginx を模した上流の応答(Content-Type と Cache-Control を上流のまま返すことを見る)。
func webResponse(status int) upstreamResponse {
	return upstreamResponse{
		status: status, contentType: "text/html; charset=utf-8", body: webHTML,
		header: http.Header{"Cache-Control": []string{"no-cache"}, "Etag": []string{`"web-etag"`}},
	}
}

// assertOnlyUpstreamReached は target にだけ1回届き、他の上流には届いていないことを確かめて、届いたリクエストを返す。
func (e *testEnv) assertOnlyUpstreamReached(t *testing.T, target *fakeUpstream, rec *httptest.ResponseRecorder) recordedRequest {
	t.Helper()
	reqs := target.requests()
	if len(reqs) != 1 {
		t.Fatalf("上流 %s に届いた回数 = %d, want 1; gateway status=%d body=%s", target.name, len(reqs), rec.Code, rec.Body.String())
	}
	for _, other := range e.upstreams() {
		if other != target && len(other.requests()) != 0 {
			t.Errorf("別の上流 %s にも届いた: %+v", other.name, other.requests())
		}
	}
	return reqs[0]
}

// AC-W1: WebURL 設定時、どのルートにも当たらない GET / HEAD は Web の上流に、メソッド・パス・クエリをそのまま付けて
// 1回だけ届き、他の上流には届かない。上流の応答(ステータス・Content-Type・Cache-Control・本文)はそのまま返る。
// Web へのルートは X-Device-Id / X-Session-Id を課さない(ブラウザの画面の取得にヘッダは付かない。AC-W3)。
func TestWebRoutesReachWebUpstream(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		path      string
		wantPath  string
		wantQuery string
	}{
		{"ルート /", http.MethodGet, "/", "/", ""},
		{"SPA のパス /calc", http.MethodGet, "/calc", "/calc", ""},
		{"SPA のパスとクエリ /reverse?x=1", http.MethodGet, "/reverse?x=1", "/reverse", "x=1"},
		{"エンコード済みクエリはそのまま", http.MethodGet, "/team?q=%E3%83%86&v=a%20b", "/team", "q=%E3%83%86&v=a%20b"},
		{"/favicon.ico", http.MethodGet, "/favicon.ico", "/favicon.ico", ""},
		{"静的ファイル /static/app.js", http.MethodGet, "/static/app.js", "/static/app.js", ""},
		{"深いパス", http.MethodGet, "/calc/detail/9001-000", "/calc/detail/9001-000", ""},
		{"末尾スラッシュ", http.MethodGet, "/calc/", "/calc/", ""},
		{"HEAD /", http.MethodHead, "/", "/", ""},
		{"HEAD /static/app.js", http.MethodHead, "/static/app.js", "/static/app.js", ""},
		// セグメント単位で予約する(/api/calcx が calc に当たらないのと同じ考え方)。予約語で始まるだけの別名は Web。
		{"/apix は /api ではない", http.MethodGet, "/apix", "/apix", ""},
		{"/internals は /internal ではない", http.MethodGet, "/internals", "/internals", ""},
		{"/healthzx は /healthz ではない", http.MethodGet, "/healthzx", "/healthzx", ""},
		{"/assetsx は /assets ではない", http.MethodGet, "/assetsx/a.png", "/assetsx/a.png", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newWebTestEnv(t)
			env.web.respond(webResponse(http.StatusOK))

			rec := serve(t, env.handler, tt.method, tt.path, http.Header{}, nil)

			got := env.assertOnlyUpstreamReached(t, env.web, rec)
			if got.Method != tt.method || got.Path != tt.wantPath || got.RawQuery != tt.wantQuery {
				t.Errorf("Web に届いたリクエスト = %s %s ?%s, want %s %s ?%s",
					got.Method, got.Path, got.RawQuery, tt.method, tt.wantPath, tt.wantQuery)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200(上流のまま)", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want 上流の値", ct)
			}
			if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
				t.Errorf("Cache-Control = %q, want 上流の値", cc)
			}
			if et := rec.Header().Get("Etag"); et != `"web-etag"` {
				t.Errorf("Etag = %q, want 上流の値", et)
			}
			if tt.method != http.MethodHead && rec.Body.String() != webHTML {
				t.Errorf("body = %q, want %q(上流のまま)", rec.Body.String(), webHTML)
			}
		})
	}
}

// AC-W2: WebURL 設定時も、予約したパスは従来どおりのルートに行き、Web には届かない。
// /api・/api/*(既知のルートは上流へ、未知は 404)、/assets/*(assets へ。未設定なら 404)、/healthz(gateway 自身)。
func TestWebDoesNotShadowReservedRoutes(t *testing.T) {
	calcBody := []byte(`{}`)
	tests := []struct {
		name     string
		mutate   func(*Config)
		method   string
		path     string
		header   http.Header
		body     []byte
		upstream string // calc / pokedex / assets。空なら gateway 自身の応答
		// upstream が空のときの期待(wantCode が空なら healthz の 200)
		wantStatus int
		wantCode   string
	}{
		{"calc", nil, http.MethodPost, "/api/calc", validHeaders(), calcBody, "calc", 0, ""},
		{"calc bulk", nil, http.MethodPost, "/api/calc/bulk", validHeaders(), calcBody, "calc", 0, ""},
		{"pokedex", nil, http.MethodGet, "/api/pokedex/natures", validHeaders(), nil, "pokedex", 0, ""},
		{"assets GET", nil, http.MethodGet, "/assets/0445-000.webp", http.Header{}, nil, "assets", 0, ""},
		{"assets HEAD", nil, http.MethodHead, "/assets/0445-000.webp", http.Header{}, nil, "assets", 0, ""},

		{"/healthz は gateway 自身", nil, http.MethodGet, "/healthz", http.Header{}, nil, "", http.StatusOK, ""},

		{"/api/unknown は 404", nil, http.MethodGet, "/api/unknown", validHeaders(), nil, "", http.StatusNotFound, "not_found"},
		{"/api/unknown はヘッダ無しでも 404", nil, http.MethodGet, "/api/unknown", http.Header{}, nil, "", http.StatusNotFound, "not_found"},
		{"/api そのものは 404", nil, http.MethodGet, "/api", http.Header{}, nil, "", http.StatusNotFound, "not_found"},
		{"/api/ は 404", nil, http.MethodGet, "/api/", http.Header{}, nil, "", http.StatusNotFound, "not_found"},
		{"/api/balance(末尾なし)は 404(issue #284。pokedex と同じ規則)", nil, http.MethodGet, "/api/balance", validHeaders(), nil, "", http.StatusNotFound, "not_found"},
		{"/api/pokedex(末尾なし)は 404", nil, http.MethodGet, "/api/pokedex", validHeaders(), nil, "", http.StatusNotFound, "not_found"},
		{"/api/calcx は 404", nil, http.MethodGet, "/api/calcx", validHeaders(), nil, "", http.StatusNotFound, "not_found"},

		{"assets 未設定の /assets/* は 404(Web に落ちない)", func(c *Config) { c.AssetsURL = nil },
			http.MethodGet, "/assets/manifest.json", http.Header{}, nil, "", http.StatusNotFound, "not_found"},
		{"/assets/* への POST は 404", nil, http.MethodPost, "/assets/0445-000.webp", http.Header{}, nil, "", http.StatusNotFound, "not_found"},
		{"/assets そのものは 404", nil, http.MethodGet, "/assets", http.Header{}, nil, "", http.StatusNotFound, "not_found"},

		{"HEAD /healthz は 404(Web に落ちない)", nil, http.MethodHead, "/healthz", http.Header{}, nil, "", http.StatusNotFound, ""},
		{"POST /healthz は 404", nil, http.MethodPost, "/healthz", http.Header{}, nil, "", http.StatusNotFound, "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mutate []func(*Config)
			if tt.mutate != nil {
				mutate = append(mutate, tt.mutate)
			}
			env := newWebTestEnv(t, mutate...)
			rec := serve(t, env.handler, tt.method, tt.path, tt.header, tt.body)

			if tt.upstream != "" {
				target := map[string]*fakeUpstream{"calc": env.calc, "pokedex": env.pokedex, "assets": env.assets}[tt.upstream]
				got := env.assertOnlyUpstreamReached(t, target, rec)
				if got.Method != tt.method || got.Path != tt.path {
					t.Errorf("上流 %s に届いたリクエスト = %s %s, want %s %s", tt.upstream, got.Method, got.Path, tt.method, tt.path)
				}
				return
			}
			env.assertNoUpstreamReached(t)
			switch {
			case tt.wantCode != "":
				assertGatewayError(t, rec, tt.wantStatus, tt.wantCode)
			case rec.Code != tt.wantStatus: // healthz の 200 と、本文の無い HEAD の 404
				t.Errorf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.path == "/healthz" && tt.method == http.MethodGet {
				if got := strings.TrimSpace(rec.Body.String()); got != `{"status":"ok"}` {
					t.Errorf("body = %s, want {\"status\":\"ok\"}(gateway 自身)", got)
				}
			}
		})
	}
}

// AC-W2(ADR-0204): WebURL 設定時も /internal と /internal/* は Web に流さず 404 not_found。GET / HEAD 以外のメソッドも
// Web に流さず 404。ドットセグメント(エンコードされたものを含む)は Web のルートでも先に拒否する。
func TestWebRejectsInternalMethodsAndDotSegments(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		header http.Header
	}{
		{"内部 API /internal/pokedex/master", http.MethodGet, "/internal/pokedex/master", http.Header{}},
		{"内部 API はヘッダ付きでも 404", http.MethodGet, "/internal/pokedex/master", validHeaders()},
		{"/internal そのもの", http.MethodGet, "/internal", http.Header{}},
		{"/internal/", http.MethodGet, "/internal/", http.Header{}},
		{"/internal/x へのクエリ付き", http.MethodGet, "/internal/pokedex/master?x=1", http.Header{}},

		{"POST /", http.MethodPost, "/", http.Header{}},
		{"PUT /calc", http.MethodPut, "/calc", http.Header{}},
		{"DELETE /static/app.js", http.MethodDelete, "/static/app.js", http.Header{}},
		{"PATCH /", http.MethodPatch, "/", http.Header{}},
		{"プリフライトでない OPTIONS /", http.MethodOptions, "/", http.Header{}},

		{"ドットセグメント /static/../index.html", http.MethodGet, "/static/../index.html", http.Header{}},
		{"ドットセグメント /./", http.MethodGet, "/./", http.Header{}},
		{"ドットセグメントで Web から /internal へ", http.MethodGet, "/calc/../internal/pokedex/master", http.Header{}},
		{"エンコードされたドットセグメント", http.MethodGet, "/%2e%2e/api/calc", http.Header{}},
		{"一部だけエンコードされたドットセグメント", http.MethodGet, "/static/.%2e/index.html", http.Header{}},
		{"エンコードされたスラッシュを含むドットセグメント", http.MethodGet, "/static%2f..%2findex.html", http.Header{}},

		// critic 指摘: 連続スラッシュで firstPathSegment が "" になり、isReservedPath の抜け道になって
		// /internal・/api が Web に転送されてしまわないこと(ADR-0205)。
		{"連続スラッシュで /internal へ", http.MethodGet, "//internal/pokedex/master", http.Header{}},
		{"連続スラッシュで /api/calc へ", http.MethodGet, "//api/calc", http.Header{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newWebTestEnv(t)
			rec := serve(t, env.handler, tt.method, tt.path, tt.header, nil)
			assertGatewayError(t, rec, http.StatusNotFound, "not_found")
			env.assertNoUpstreamReached(t)
		})
	}
	t.Run("HEAD /internal は本文なしの 404", func(t *testing.T) {
		env := newWebTestEnv(t)
		rec := serve(t, env.handler, http.MethodHead, "/internal/pokedex/master", http.Header{}, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
		env.assertNoUpstreamReached(t)
	})
}

// AC-W1: CORS のプリフライトは WebURL 設定時も gateway が 204 で答え、Web には送らない(判定順序は ADR-0202 §3 のまま)。
func TestWebPreflightStaysAtGateway(t *testing.T) {
	env := newWebTestEnv(t)
	header := http.Header{"Origin": []string{allowedOrigin}, "Access-Control-Request-Method": []string{http.MethodGet}}
	rec := serve(t, env.handler, http.MethodOptions, "/calc", header, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	env.assertNoUpstreamReached(t)
}

// AC-W4: WebURL 未設定なら従来どおり: どのルートにも当たらないパスは 404 not_found で、Web の偽物にも届かない。
func TestWebUnsetKeepsNotFound(t *testing.T) {
	for _, tt := range []struct{ method, path string }{
		{http.MethodGet, "/"},
		{http.MethodGet, "/calc"},
		{http.MethodGet, "/reverse?x=1"},
		{http.MethodGet, "/favicon.ico"},
		{http.MethodGet, "/static/app.js"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			env := newTestEnv(t)
			rec := serve(t, env.handler, tt.method, tt.path, http.Header{}, nil)
			assertGatewayError(t, rec, http.StatusNotFound, "not_found")
			env.assertNoUpstreamReached(t)
		})
	}
	t.Run("HEAD /", func(t *testing.T) {
		env := newTestEnv(t)
		rec := serve(t, env.handler, http.MethodHead, "/", http.Header{}, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
		env.assertNoUpstreamReached(t)
	})
}

// AC-W3: Web へのルートは X-Device-Id / X-Session-Id を検証しない(無くても UUID でなくても通る)。
func TestWebRoutesSkipHeaderValidation(t *testing.T) {
	tests := []struct {
		name   string
		header http.Header
	}{
		{"ヘッダ無し", http.Header{}},
		{"UUID でない X-Device-Id", http.Header{"X-Device-Id": []string{"not-a-uuid"}}},
		{"X-Session-Id の重複", http.Header{"X-Session-Id": []string{testSessionID, testSessionID}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newWebTestEnv(t)
			rec := serve(t, env.handler, http.MethodGet, "/calc", tt.header, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200(Web の応答); body=%s", rec.Code, rec.Body.String())
			}
			env.assertOnlyUpstreamReached(t, env.web, rec)
		})
	}
}

// AC-W5: Web の上流に接続できない・タイムアウトは 503 upstream_unavailable(Error 形式。内部情報を出さない)。
// 上流が返した 404・500 はステータス・Content-Type・本文ともそのまま(gateway は Error に書き換えない)。
func TestWebUpstreamFailures(t *testing.T) {
	const shortTimeout = 100 * time.Millisecond
	t.Run("接続できない", func(t *testing.T) {
		env := newWebTestEnv(t, func(c *Config) { c.WebURL = closedServerURL(t) })
		rec := serve(t, env.handler, http.MethodGet, "/", http.Header{}, nil)
		assertGatewayError(t, rec, http.StatusServiceUnavailable, "upstream_unavailable")
	})
	t.Run("接続できない(HEAD)", func(t *testing.T) {
		env := newWebTestEnv(t, func(c *Config) { c.WebURL = closedServerURL(t) })
		rec := serve(t, env.handler, http.MethodHead, "/", http.Header{}, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", rec.Code)
		}
	})
	t.Run("タイムアウト", func(t *testing.T) {
		env := newWebTestEnv(t, func(c *Config) {
			c.WebURL = slowServerURL(t, 5*time.Second)
			c.UpstreamTimeout = shortTimeout
		})
		start := time.Now()
		rec := serve(t, env.handler, http.MethodGet, "/calc", http.Header{}, nil)
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Errorf("応答まで %v かかった(UpstreamTimeout で打ち切られていない)", elapsed)
		}
		assertGatewayError(t, rec, http.StatusServiceUnavailable, "upstream_unavailable")
	})
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError} {
		t.Run("上流の "+http.StatusText(status)+" はそのまま", func(t *testing.T) {
			env := newWebTestEnv(t)
			env.web.respond(webResponse(status))
			rec := serve(t, env.handler, http.MethodGet, "/no-such-file.js", http.Header{}, nil)
			if rec.Code != status {
				t.Errorf("status = %d, want %d(上流のまま)", rec.Code, status)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want 上流の値(Error の JSON に書き換えない)", ct)
			}
			if rec.Body.String() != webHTML {
				t.Errorf("body = %q, want %q(上流のまま)", rec.Body.String(), webHTML)
			}
		})
	}
}

// AC-W6: Web の上流が付けた Access-Control-* は取り除き、許可オリジンなら gateway が ACAO を1つだけ付け、
// 許可外なら付けない。Web の上流に届かない 503 にも許可オリジンなら ACAO を付ける(ADR-0202 §6 と同じ)。
func TestWebCORS(t *testing.T) {
	upstreamACAO := http.Header{"Access-Control-Allow-Origin": []string{"*"}, "Access-Control-Allow-Methods": []string{"PUT"}}
	t.Run("上流の * を許可オリジンに付け替える", func(t *testing.T) {
		env := newWebTestEnv(t)
		env.web.respond(upstreamResponse{status: http.StatusOK, contentType: "text/html", body: webHTML, header: upstreamACAO})
		rec := serve(t, env.handler, http.MethodGet, "/", withOrigin(http.Header{}, allowedOrigin), nil)
		assertCORSAllowed(t, rec, allowedOrigin)
		if got := rec.Header().Values("Access-Control-Allow-Methods"); len(got) != 0 {
			t.Errorf("上流の Access-Control-Allow-Methods が残っている: %q", got)
		}
	})
	t.Run("許可外オリジンには上流の * も含めて何も付けない", func(t *testing.T) {
		env := newWebTestEnv(t)
		env.web.respond(upstreamResponse{status: http.StatusOK, contentType: "text/html", body: webHTML, header: upstreamACAO})
		rec := serve(t, env.handler, http.MethodGet, "/", withOrigin(http.Header{}, disallowedOrigin), nil)
		assertNoCORS(t, rec)
	})
	t.Run("接続できない 503 にも許可オリジンなら ACAO", func(t *testing.T) {
		env := newWebTestEnv(t, func(c *Config) { c.WebURL = closedServerURL(t) })
		rec := serve(t, env.handler, http.MethodGet, "/", withOrigin(http.Header{}, allowedOrigin), nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
		assertCORSAllowed(t, rec, allowedOrigin)
	})
}

// AC-W6: Web への転送でも Host は上流のホストに書き換え、クライアントが送った X-Forwarded-* は信用せず付け直す
// (ADR-0202 §5 の Rewrite と同じ扱い)。
func TestWebForwardedHeadersAreRewritten(t *testing.T) {
	env := newWebTestEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/calc", nil)
	req.Host = "client-supplied.example.test"
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "spoofed-client")
	req.Header.Set("X-Forwarded-Host", "evil.example.test")
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)

	got := env.assertOnlyUpstreamReached(t, env.web, rec)
	if want := env.web.url(t).Host; got.Host != want {
		t.Errorf("Web が受け取った Host = %q, want %q(上流のホスト)", got.Host, want)
	}
	if xff := got.Header.Get("X-Forwarded-For"); xff != "127.0.0.1" {
		t.Errorf("X-Forwarded-For = %q, want 127.0.0.1(偽装した値を残さない)", xff)
	}
	if xfh := got.Header.Get("X-Forwarded-Host"); xfh != "client-supplied.example.test" {
		t.Errorf("X-Forwarded-Host = %q, want リクエストの実際の Host", xfh)
	}
	if xfp := got.Header.Get("X-Forwarded-Proto"); xfp != "http" {
		t.Errorf("X-Forwarded-Proto = %q, want http", xfp)
	}
}

// AC-W7: WebURL 設定時に gateway 自身が作る 404 / 503 は契約の Error スキーマに合う(Web のパスは契約に無いので
// 操作ではなく Error スキーマで検証する。assertGatewayError が assertErrorSchema を呼ぶ)。
func TestWebGatewayErrorsMatchErrorSchema(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*Config)
		method     string
		path       string
		wantStatus int
		wantCode   string
	}{
		{"Web に接続できない 503", func(c *Config) { c.WebURL = closedServerURL(t) },
			http.MethodGet, "/", http.StatusServiceUnavailable, "upstream_unavailable"},
		{"/internal の 404", nil, http.MethodGet, "/internal/pokedex/master", http.StatusNotFound, "not_found"},
		{"POST / の 404", nil, http.MethodPost, "/", http.StatusNotFound, "not_found"},
		{"ドットセグメントの 404", nil, http.MethodGet, "/static/../index.html", http.StatusNotFound, "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mutate []func(*Config)
			if tt.mutate != nil {
				mutate = append(mutate, tt.mutate)
			}
			env := newWebTestEnv(t, mutate...)
			rec := serve(t, env.handler, tt.method, tt.path, http.Header{}, nil)
			assertGatewayError(t, rec, tt.wantStatus, tt.wantCode)
		})
	}
}
