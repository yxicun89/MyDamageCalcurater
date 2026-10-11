package httpapi

// 連続技(POST /api/calc/combo。ADR-0252)の gateway 経由の受け入れテスト。
//
// gateway のルーティングは変えない(/api/calc/* の前方一致で calc-svc に届く。ADR-0202 §3)。ここでは
//   - パスが calc の上流にだけ、メソッド・パス・本文付きで届くこと(ルーティングの回帰の固定)
//   - ヘッダの検証は gateway が行い、不正なら上流に届かないこと
//   - 上流が calc-svc の実物(架空マスタ)のとき、成功と代表的な 400 が契約どおりで、gateway が書き換えないこと。
//     この環境は record・team・NATS を持たない(CalcURL だけ)ので、成功は AC-17(保存系が止まっていても計算は成功)の確認も兼ねる
// を確かめる。

import (
	"bytes"
	"net/http"
	"testing"

	"example.com/pokecalc/services/calc/calctest"
)

const gatewayComboPath = "/api/calc/combo"

func TestComboRouteReachesCalcUpstream(t *testing.T) {
	body := []byte(`{"steps":[]}`)
	env := newTestEnv(t)
	env.calc.respond(upstreamResponse{status: http.StatusOK, contentType: "application/json", body: `{"upstream":"calc"}`})
	rec := serve(t, env.handler, http.MethodPost, gatewayComboPath, validHeaders(), body)
	reqs := env.calc.requests()
	if len(reqs) != 1 {
		t.Fatalf("calc の上流に届いた回数 = %d, want 1; status=%d body=%s", len(reqs), rec.Code, rec.Body.String())
	}
	if got := reqs[0]; got.Method != http.MethodPost || got.Path != gatewayComboPath || !bytes.Equal(got.Body, body) {
		t.Errorf("上流に届いたリクエスト = %s %s %q", got.Method, got.Path, got.Body)
	}
	for _, other := range env.upstreams() {
		if other != env.calc && len(other.requests()) != 0 {
			t.Errorf("別の上流 %s にも届いた", other.name)
		}
	}
	if rec.Code != http.StatusOK || rec.Body.String() != `{"upstream":"calc"}` {
		t.Errorf("上流の応答がそのまま返らない: %d %s", rec.Code, rec.Body.String())
	}
}

func TestComboRouteRequiresHeaders(t *testing.T) {
	env := newTestEnv(t)
	header := validHeaders()
	header.Set("X-Device-Id", "not-a-uuid")
	rec := serve(t, env.handler, http.MethodPost, gatewayComboPath, header, []byte(`{}`))
	assertGatewayError(t, rec, http.StatusBadRequest, "invalid_header")
	assertContract(t, http.MethodPost, gatewayComboPath, header, []byte(`{}`), rec, false)
	env.assertNoUpstreamReached(t)
}

// 上流が calc-svc の実物のとき、連続技で 200、代表的な 400 がそのまま返り、契約どおり。
// record・team・NATS は設定していない(AC-17)。
func TestRealComboThroughGatewayMatchesContract(t *testing.T) {
	h := newRealCalcEnv(t)
	attacker := realCalcIndividual(calctest.SpeciesAttacker, calctest.NatureAtkUp, spOf(0, 32, 0, 0, 0, 32))
	defender := realCalcIndividual(calctest.SpeciesDefender, calctest.NatureNeutral, spOf(32, 0, 0, 0, 0, 0))
	withSteps := func(steps []any, extra map[string]any) map[string]any {
		b := map[string]any{"format": "single", "attacker": attacker, "defender": defender, "steps": steps}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	step := func(id string) map[string]any { return map[string]any{"moveId": id} }
	physical := step(calctest.MovePhysical)
	tests := []struct {
		name       string
		body       map[string]any
		wantStatus int
		wantCode   string
	}{
		{"2 手・相手の技・相手の素早さは防御側から", withSteps([]any{physical, physical},
			map[string]any{"opponentMoveId": calctest.MovePhysical, "opponentSpeed": "from_defender"}), http.StatusOK, ""},
		{"1 手・既定", withSteps([]any{physical}, nil), http.StatusOK, ""},
		{"手 0 件は invalid_input のまま", withSteps([]any{}, nil), http.StatusBadRequest, "invalid_input"},
		{"手 4 件は invalid_input のまま", withSteps([]any{physical, physical, physical, physical}, nil), http.StatusBadRequest, "invalid_input"},
		{"ランク変化の無い変化技は invalid_input のまま", withSteps([]any{step("testglare"), physical}, nil), http.StatusBadRequest, "invalid_input"},
		{"未知の技は unknown_move のまま", withSteps([]any{step("test-no-such-move")}, nil), http.StatusBadRequest, "unknown_move"},
		{"未知の opponentSpeed は invalid_enum のまま", withSteps([]any{physical}, map[string]any{"opponentSpeed": "fast"}),
			http.StatusBadRequest, "invalid_enum"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := mustJSON(t, tt.body)
			rec := serve(t, h, http.MethodPost, gatewayComboPath, validHeaders(), body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantCode != "" {
				if got := decodeErrorBody(t, rec); got.Code != tt.wantCode {
					t.Errorf("code = %q, want %q", got.Code, tt.wantCode)
				}
			}
			assertContract(t, http.MethodPost, gatewayComboPath, validHeaders(), body, rec, tt.wantStatus == http.StatusOK)
		})
	}
}
