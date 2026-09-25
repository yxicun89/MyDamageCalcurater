package httpapi

// `/api/team/*` のルーティングと CORS の PUT / DELETE 許可(ADR-0209 §10・ADR-0202 §3・§6 への追記、
// ADR-0213)。AC-P9(ルーティング)・AC-P8(プリフライトが DELETE を許可)・AC-T9(PUT を許可)。
//
// 実装済み(record のときと同じ形。この形を固定する):
//   - `Config.TeamURL`(環境変数 `GATEWAY_TEAM_URL`。未設定なら 503 upstream_unavailable)
//   - routing.go の routeTeam / prefixTeam("/api/team/")・requiresHeaderCheck への追加
//   - cors.go の corsAllowMethods に **PUT** を追加(DELETE は P5-3 で追加済み)
//   - fixture_test.go に team の偽上流と newTeamTestEnv を足し、`TestRoutesReachTheirUpstream` の表に
//     team の行を追加して「実際に team-svc へ転送される」ことまで確かめてある

import (
	"net/http"
	"testing"
)

// AC-P9(ADR-0209): `/api/team/*` は gateway のルートとして存在する(404 にならない)。
// 上流が未設定のときは pokedex / record と同じく 503 upstream_unavailable。
func TestTeamPathsAreRouted(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"一覧", http.MethodGet, "/api/team/teams"},
		{"作成", http.MethodPost, "/api/team/teams"},
		{"取得", http.MethodGet, "/api/team/teams/11111111-2222-4333-8444-555555555555"},
		{"更新", http.MethodPut, "/api/team/teams/11111111-2222-4333-8444-555555555555"},
		{"削除", http.MethodDelete, "/api/team/teams/11111111-2222-4333-8444-555555555555"},
		{"全削除", http.MethodDelete, "/api/team/device-data"},
		{"未知の下位パスも上流に任せる", http.MethodGet, "/api/team/whatever"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			rec := serve(t, env.handler, tt.method, tt.path, validHeaders(), nil)
			// TeamURL 未設定のあいだは 503 upstream_unavailable。実装後もここは変えない
			// (未設定の上流は 503、というのが ADR-0202 §3 の規則)。
			assertGatewayError(t, rec, http.StatusServiceUnavailable, "upstream_unavailable")
			for _, up := range env.upstreams() {
				if n := len(up.requests()); n != 0 {
					t.Errorf("上流 %s に %d 件届いた, want 0(team は別の上流)", up.name, n)
				}
			}
		})
	}
}

// `/api/team`(末尾スラッシュ無し)と `/api/teamx` は 404(pokedex / record と同じ規則。ADR-0202 §3)。
func TestTeamPrefixIsExact(t *testing.T) {
	for _, path := range []string{"/api/team", "/api/teamx", "/api/teamx/teams"} {
		t.Run(path, func(t *testing.T) {
			env := newTestEnv(t)
			rec := serve(t, env.handler, http.MethodGet, path, validHeaders(), nil)
			assertGatewayError(t, rec, http.StatusNotFound, "not_found")
		})
	}
}

// `/api/team/*` にもヘッダ検証がかかる(ADR-0202 §4。検証は上流の有無より先)。
func TestTeamPathsRequireClientIDHeaders(t *testing.T) {
	env := newTestEnv(t)
	rec := serve(t, env.handler, http.MethodGet, "/api/team/teams", http.Header{}, nil)
	assertGatewayError(t, rec, http.StatusBadRequest, "missing_header")
}

// AC-P8 / AC-T9: 許可オリジンからのプリフライトが DELETE と PUT を許可して返る
// (これが無いと Web から構築の更新・削除を呼べない。ADR-0209 §10-2)。
func TestCORSPreflightAllowsPutAndDeleteOnTeam(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			env := newTestEnv(t)
			h := http.Header{}
			h.Set("Origin", allowedOrigin)
			h.Set("Access-Control-Request-Method", method)
			h.Set("Access-Control-Request-Headers", "content-type, x-device-id, x-session-id")

			rec := serve(t, env.handler, http.MethodOptions, "/api/team/teams/11111111-2222-4333-8444-555555555555", h, nil)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204", rec.Code)
			}
			assertCORSAllowed(t, rec, allowedOrigin)
			if got := rec.Header().Get("Access-Control-Allow-Methods"); !methodAllowed(got, method) {
				t.Errorf("Access-Control-Allow-Methods = %q に %s が無い", got, method)
			}
			for _, up := range env.upstreams() {
				if n := len(up.requests()); n != 0 {
					t.Errorf("プリフライトが上流 %s に届いた(%d 件)", up.name, n)
				}
			}
		})
	}
}
