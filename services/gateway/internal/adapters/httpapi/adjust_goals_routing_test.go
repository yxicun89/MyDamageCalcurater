package httpapi

// 調整の目標(POST /api/calc/adjust/goals。ADR-0331・ADR-0177 §9)の gateway 経由の受け入れテスト。
//
// gateway のルーティングは変えない(/api/calc/* の前方一致で calc-svc に届く。ADR-0202 §3)。ここでは
//   - パスが calc の上流にだけ、メソッド・パス・本文付きで届くこと(ルーティングの回帰の固定)
//   - 上流が calc-svc の実物(架空マスタ)のとき、成功(素早さ + 倒す)と代表的な 400 が契約どおりで、gateway が書き換えないこと
// を確かめる。

import (
	"bytes"
	"net/http"
	"testing"

	"example.com/pokecalc/services/calc/calctest"
)

const gatewayAdjustGoalsPath = "/api/calc/adjust/goals"

func TestAdjustGoalsRouteReachesCalcUpstream(t *testing.T) {
	body := []byte(`{"goals":[]}`)
	env := newTestEnv(t)
	env.calc.respond(upstreamResponse{status: http.StatusOK, contentType: "application/json", body: `{"upstream":"calc"}`})
	rec := serve(t, env.handler, http.MethodPost, gatewayAdjustGoalsPath, validHeaders(), body)
	reqs := env.calc.requests()
	if len(reqs) != 1 {
		t.Fatalf("calc の上流に届いた回数 = %d, want 1; status=%d body=%s", len(reqs), rec.Code, rec.Body.String())
	}
	if got := reqs[0]; got.Method != http.MethodPost || got.Path != gatewayAdjustGoalsPath || !bytes.Equal(got.Body, body) {
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

func TestAdjustGoalsRouteRequiresHeaders(t *testing.T) {
	env := newTestEnv(t)
	header := validHeaders()
	header.Set("X-Device-Id", "not-a-uuid")
	rec := serve(t, env.handler, http.MethodPost, gatewayAdjustGoalsPath, header, []byte(`{}`))
	assertGatewayError(t, rec, http.StatusBadRequest, "invalid_header")
	assertContract(t, http.MethodPost, gatewayAdjustGoalsPath, header, []byte(`{}`), rec, false)
	env.assertNoUpstreamReached(t)
}

// 上流が calc-svc の実物のとき、目標(素早さ + 倒す)で 200、代表的な 400 がそのまま返り、契約どおり。
func TestRealAdjustGoalsThroughGatewayMatchesContract(t *testing.T) {
	h := newRealCalcEnv(t)
	self := realCalcIndividual(calctest.SpeciesAttacker, calctest.NatureAtkUp, spOf(0, 0, 0, 0, 0, 0))
	foe := realCalcIndividual(calctest.SpeciesDefender, calctest.NatureNeutral, spOf(32, 0, 0, 0, 0, 32))
	ok := map[string]any{"format": "single", "self": self, "goals": []any{
		map[string]any{"kind": "outspeed", "opponent": foe},
		map[string]any{"kind": "ko", "opponent": foe, "moveId": calctest.MovePhysical, "hits": 3},
	}}
	withGoals := func(goals []any) map[string]any {
		return map[string]any{"format": "single", "self": self, "goals": goals}
	}
	tests := []struct {
		name       string
		body       map[string]any
		wantStatus int
		wantCode   string
	}{
		{"素早さ + 倒す の成功", ok, http.StatusOK, ""},
		{"目標 0 件は invalid_input のまま", withGoals([]any{}), http.StatusBadRequest, "invalid_input"},
		{"未知の kind は invalid_enum のまま", withGoals([]any{map[string]any{"kind": "faster", "opponent": foe}}), http.StatusBadRequest, "invalid_enum"},
		{"未知の技は unknown_move のまま", withGoals([]any{map[string]any{"kind": "ko", "opponent": foe, "moveId": "test-no-such-move", "hits": 1}}),
			http.StatusBadRequest, "unknown_move"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := mustJSON(t, tt.body)
			rec := serve(t, h, http.MethodPost, gatewayAdjustGoalsPath, validHeaders(), body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantCode != "" {
				if got := decodeErrorBody(t, rec); got.Code != tt.wantCode {
					t.Errorf("code = %q, want %q", got.Code, tt.wantCode)
				}
			}
			assertContract(t, http.MethodPost, gatewayAdjustGoalsPath, validHeaders(), body, rec, tt.wantStatus == http.StatusOK)
		})
	}
}
