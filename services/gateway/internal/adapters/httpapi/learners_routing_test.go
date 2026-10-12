package httpapi

// 技の逆引き GET /api/pokedex/moves/{key}/learners(AJ5・ADR-0251)の gateway 経由の受け入れテスト。
//
// gateway のルーティングは変えない(/api/pokedex/* の前方一致で pokedex-svc に届く。ADR-0202 §3)。ここでは
//   - パス・クエリ(limit・offset)がそのまま pokedex の上流にだけ届くこと(ルーティングの回帰の固定)
//   - 上流の 200 [] / 404 / 503 がそのまま返り、契約に合うこと
//   - ヘッダの検証が効く(上流に届かない)こと
// を確かめる。

import (
	"net/http"
	"testing"
)

const gatewayLearnersPath = "/api/pokedex/moves/testflame/learners"

func TestLearnersRouteReachesPokedexUpstream(t *testing.T) {
	env := newTestEnv(t)
	env.pokedex.respond(upstreamResponse{status: http.StatusOK, contentType: "application/json", body: `[]`})
	target := gatewayLearnersPath + "?limit=20&offset=40"
	rec := serve(t, env.handler, http.MethodGet, target, validHeaders(), nil)

	reqs := env.pokedex.requests()
	if len(reqs) != 1 {
		t.Fatalf("pokedex の上流に届いた回数 = %d, want 1; status=%d body=%s", len(reqs), rec.Code, rec.Body.String())
	}
	if got := reqs[0]; got.Method != http.MethodGet || got.Path != gatewayLearnersPath || got.RawQuery != "limit=20&offset=40" {
		t.Errorf("上流に届いたリクエスト = %s %s ?%s, want GET %s ?limit=20&offset=40", got.Method, got.Path, got.RawQuery, gatewayLearnersPath)
	}
	for _, other := range env.upstreams() {
		if other != env.pokedex && len(other.requests()) != 0 {
			t.Errorf("別の上流 %s にも届いた", other.name)
		}
	}
	if rec.Code != http.StatusOK || rec.Body.String() != `[]` {
		t.Errorf("上流の応答がそのまま返らない: %d %s", rec.Code, rec.Body.String())
	}
	assertContract(t, http.MethodGet, target, validHeaders(), nil, rec, true)
}

// 上流の 404 not_found・503 master_unavailable はそのまま返り、契約(listMoveLearners の 404 / 503)に合う。
func TestLearnersUpstreamErrorsPassThrough(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		code   string
	}{
		{"技が無い", http.StatusNotFound, "not_found"},
		{"マスタ未投入", http.StatusServiceUnavailable, "master_unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			env.pokedex.respond(upstreamResponse{status: tt.status, contentType: "application/json",
				body: `{"code":"` + tt.code + `","message":"x"}`})
			rec := serve(t, env.handler, http.MethodGet, gatewayLearnersPath, validHeaders(), nil)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.status, rec.Body.String())
			}
			if got := decodeErrorBody(t, rec); got.Code != tt.code {
				t.Errorf("code = %q, want %q", got.Code, tt.code)
			}
			assertContract(t, http.MethodGet, gatewayLearnersPath, validHeaders(), nil, rec, true)
		})
	}
}

// ヘッダの検証は /api/pokedex/moves/{key}/learners にも効く(上流に届かない)。
func TestLearnersRouteRequiresHeaders(t *testing.T) {
	env := newTestEnv(t)
	header := validHeaders()
	header.Set("X-Device-Id", "not-a-uuid")
	rec := serve(t, env.handler, http.MethodGet, gatewayLearnersPath, header, nil)
	assertGatewayError(t, rec, http.StatusBadRequest, "invalid_header")
	assertContract(t, http.MethodGet, gatewayLearnersPath, header, nil, rec, false)
	env.assertNoUpstreamReached(t)
}
