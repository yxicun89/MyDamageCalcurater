package httpapi

// ルーティングの受け入れテスト(ADR-0020 §3。AC-G1〜AC-G3)。

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

// AC-G1: /api/calc と /api/calc/*、/api/pokedex/*、/assets/*(GET/HEAD)がそれぞれの上流に、
// メソッド・パス・クエリ・ボディ・ヘッダ付きで1回だけ届き、他の上流には届かない。
// 上流のステータス・ボディ・Content-Type(と追加ヘッダ)がそのまま返る。
func TestRoutesReachTheirUpstream(t *testing.T) {
	calcBody := []byte(`{"format":"single","moveId":"test-beam"}`)
	tests := []struct {
		name      string
		method    string
		path      string
		header    http.Header
		body      []byte
		upstream  string // calc / pokedex / assets
		wantPath  string
		wantQuery string
	}{
		{"calc", http.MethodPost, "/api/calc", validHeaders(), calcBody, "calc", "/api/calc", ""},
		{"calc bulk", http.MethodPost, "/api/calc/bulk", validHeaders(), calcBody, "calc", "/api/calc/bulk", ""},
		{"calc reverse(クエリもそのまま)", http.MethodPost, "/api/calc/reverse?trace=1&x=a%20b", validHeaders(), calcBody,
			"calc", "/api/calc/reverse", "trace=1&x=a%20b"},
		{"pokedex 検索(エンコード済みクエリ)", http.MethodGet, "/api/pokedex/species?q=%E3%83%86&limit=5", validHeaders(), nil,
			"pokedex", "/api/pokedex/species", "q=%E3%83%86&limit=5"},
		{"pokedex 詳細", http.MethodGet, "/api/pokedex/species/9001-000", validHeaders(), nil,
			"pokedex", "/api/pokedex/species/9001-000", ""},
		{"pokedex 性格", http.MethodGet, "/api/pokedex/natures", validHeaders(), nil, "pokedex", "/api/pokedex/natures", ""},
		{"assets manifest(ヘッダ無し)", http.MethodGet, "/assets/manifest.json", http.Header{}, nil,
			"assets", "/assets/manifest.json", ""},
		{"assets 画像(クエリ付き)", http.MethodGet, "/assets/0445-000.webp?v=abc123", http.Header{}, nil,
			"assets", "/assets/0445-000.webp", "v=abc123"},
		{"assets HEAD", http.MethodHead, "/assets/0445-000.webp", http.Header{}, nil, "assets", "/assets/0445-000.webp", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			target := map[string]*fakeUpstream{"calc": env.calc, "pokedex": env.pokedex, "assets": env.assets}[tt.upstream]
			target.respond(upstreamResponse{
				status: http.StatusOK, contentType: "application/json; charset=utf-8",
				body:   `{"upstream":"` + tt.upstream + `"}`,
				header: http.Header{"Cache-Control": []string{"public, max-age=31536000, immutable"}, "Etag": []string{`"abc"`}},
			})

			rec := serve(t, env.handler, tt.method, tt.path, tt.header, tt.body)

			reqs := target.requests()
			if len(reqs) != 1 {
				t.Fatalf("上流 %s に届いた回数 = %d, want 1; gateway status=%d body=%s", tt.upstream, len(reqs), rec.Code, rec.Body.String())
			}
			got := reqs[0]
			if got.Method != tt.method || got.Path != tt.wantPath || got.RawQuery != tt.wantQuery {
				t.Errorf("上流に届いたリクエスト = %s %s ?%s, want %s %s ?%s",
					got.Method, got.Path, got.RawQuery, tt.method, tt.wantPath, tt.wantQuery)
			}
			if !bytes.Equal(got.Body, tt.body) && !(len(got.Body) == 0 && len(tt.body) == 0) {
				t.Errorf("上流に届いたボディ = %q, want %q", got.Body, tt.body)
			}
			for _, name := range []string{"X-Device-Id", "X-Session-Id", "Content-Type"} {
				if want := tt.header.Values(name); len(want) > 0 {
					if g := got.Header.Values(name); len(g) != 1 || g[0] != want[0] {
						t.Errorf("上流に届いた %s = %q, want %q", name, g, want)
					}
				}
			}
			for _, other := range env.upstreams() {
				if other != target && len(other.requests()) != 0 {
					t.Errorf("別の上流 %s にも届いた", other.name)
				}
			}

			// 上流の応答がそのまま返る。
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Errorf("Content-Type = %q, want 上流の値", ct)
			}
			if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
				t.Errorf("Cache-Control = %q, want 上流の値", cc)
			}
			if tt.method != http.MethodHead {
				if want := `{"upstream":"` + tt.upstream + `"}`; rec.Body.String() != want {
					t.Errorf("body = %q, want %q", rec.Body.String(), want)
				}
			}
		})
	}
}

// AC-G2: 上流が返したレスポンス(4xx/5xx を含む)はステータスとボディをそのまま返す(gateway は書き換えない)。
func TestUpstreamResponsesPassThroughUnchanged(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"上流の 400 Error", http.StatusBadRequest, `{"code":"invalid_json","message":"上流の説明"}`},
		{"上流の 404 Error", http.StatusNotFound, `{"code":"not_found","message":"このサービスの担当外の操作"}`},
		{"上流の 500 Error", http.StatusInternalServerError, `{"code":"internal","message":"内部エラーが発生した"}`},
		{"上流の 503 master_unavailable", http.StatusServiceUnavailable, `{"code":"master_unavailable","message":"マスタを参照できない"}`},
		{"上流の 201(契約外でも書き換えない)", http.StatusCreated, `{"ok":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			env.calc.respond(upstreamResponse{status: tt.status, contentType: "application/json", body: tt.body})
			rec := serve(t, env.handler, http.MethodPost, "/api/calc", validHeaders(), []byte(`{}`))
			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d(上流のまま)", rec.Code, tt.status)
			}
			if rec.Body.String() != tt.body {
				t.Errorf("body = %q, want %q(上流のまま)", rec.Body.String(), tt.body)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
		})
	}
}

// AC-G3: gateway が扱わないパス・メソッドは 404 not_found(Error 形式)で、どの上流にも届かない。
// /api/balance は独自の Ingress(ADR-0012)なので gateway では扱わない。ドットセグメントで
// 別のルート(上流の /healthz など)へ抜けることもできない。
func TestUnroutedPathsAreNotFound(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		header http.Header
	}{
		{"/api/balance", http.MethodGet, "/api/balance", validHeaders()},
		{"/api/balance/defense", http.MethodPost, "/api/balance/defense", validHeaders()},
		{"/api/calcx(前方一致で拾わない)", http.MethodPost, "/api/calcx", validHeaders()},
		{"/api/pokedex(末尾なし)", http.MethodGet, "/api/pokedex", validHeaders()},
		{"/api/unknown", http.MethodGet, "/api/unknown", validHeaders()},
		{"未知の /api パスはヘッダ無しでも not_found(ルーティングが先)", http.MethodGet, "/api/unknown", http.Header{}},
		{"/", http.MethodGet, "/", validHeaders()},
		{"/nothing", http.MethodGet, "/nothing", validHeaders()},
		{"POST /healthz", http.MethodPost, "/healthz", validHeaders()},
		{"POST /assets", http.MethodPost, "/assets/0445-000.webp", http.Header{}},
		{"PUT /assets", http.MethodPut, "/assets/0445-000.webp", http.Header{}},
		{"DELETE /assets", http.MethodDelete, "/assets/0445-000.webp", http.Header{}},
		{"ドットセグメントで /healthz へ", http.MethodGet, "/api/calc/../../healthz", validHeaders()},
		{"ドットセグメントで /api/balance へ", http.MethodGet, "/api/calc/../balance", validHeaders()},
		{"ドットセグメントで assets から api へ", http.MethodGet, "/assets/../api/calc", http.Header{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			rec := serve(t, env.handler, tt.method, tt.path, tt.header, nil)
			assertGatewayError(t, rec, http.StatusNotFound, "not_found")
			env.assertNoUpstreamReached(t)
		})
	}
}

// AC-G3: gateway 自身の GET /healthz は 200 {"status":"ok"} で、上流の /healthz には届かない。
func TestHealthzIsGatewayOwn(t *testing.T) {
	env := newTestEnv(t)
	rec := serve(t, env.handler, http.MethodGet, "/healthz", http.Header{}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if got := bytes.TrimSpace(rec.Body.Bytes()); string(got) != `{"status":"ok"}` {
		t.Errorf("body = %s, want {\"status\":\"ok\"}", got)
	}
	env.assertNoUpstreamReached(t)
}
