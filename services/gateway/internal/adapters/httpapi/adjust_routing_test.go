package httpapi

// 調整 API(/api/calc/adjust/*。ADR-0250 §1)の gateway 経由の受け入れテスト。
//
// gateway のルーティングは変えない(/api/calc/* の前方一致で calc-svc に届く。ADR-0202 §3)。ここでは
//   - 4つのパスが calc の上流にだけ、メソッド・パス・本文・ヘッダ付きで届くこと(ルーティングの回帰の固定)
//   - 上流が calc-svc の実物(架空マスタ)のとき、成功・代表的な 400 が契約どおりで、gateway が書き換えないこと
// を確かめる。

import (
	"bytes"
	"net/http"
	"testing"

	"example.com/pokecalc/services/calc/calctest"
)

var gatewayAdjustPaths = []string{
	"/api/calc/adjust/indices",
	"/api/calc/adjust/min-sp-to-ko",
	"/api/calc/adjust/min-sp-to-survive",
	"/api/calc/adjust/allocation",
}

func TestAdjustRoutesReachCalcUpstream(t *testing.T) {
	body := []byte(`{"hits":1}`)
	for _, path := range gatewayAdjustPaths {
		t.Run(path, func(t *testing.T) {
			env := newTestEnv(t)
			env.calc.respond(upstreamResponse{status: http.StatusOK, contentType: "application/json", body: `{"upstream":"calc"}`})
			rec := serve(t, env.handler, http.MethodPost, path, validHeaders(), body)
			reqs := env.calc.requests()
			if len(reqs) != 1 {
				t.Fatalf("calc の上流に届いた回数 = %d, want 1; status=%d body=%s", len(reqs), rec.Code, rec.Body.String())
			}
			if got := reqs[0]; got.Method != http.MethodPost || got.Path != path || !bytes.Equal(got.Body, body) {
				t.Errorf("上流に届いたリクエスト = %s %s %q, want POST %s %q", got.Method, got.Path, got.Body, path, body)
			}
			for _, other := range env.upstreams() {
				if other != env.calc && len(other.requests()) != 0 {
					t.Errorf("別の上流 %s にも届いた", other.name)
				}
			}
			if rec.Code != http.StatusOK || rec.Body.String() != `{"upstream":"calc"}` {
				t.Errorf("上流の応答がそのまま返らない: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// ヘッダの検証は /api/calc/adjust/* にも効く(上流に届かない)。
func TestAdjustRoutesRequireHeaders(t *testing.T) {
	for _, path := range gatewayAdjustPaths {
		t.Run(path, func(t *testing.T) {
			env := newTestEnv(t)
			header := validHeaders()
			header.Set("X-Device-Id", "not-a-uuid")
			rec := serve(t, env.handler, http.MethodPost, path, header, []byte(`{}`))
			assertGatewayError(t, rec, http.StatusBadRequest, "invalid_header")
			assertContract(t, http.MethodPost, path, header, []byte(`{}`), rec, false)
			env.assertNoUpstreamReached(t)
		})
	}
}

// 上流が calc-svc の実物のとき、gateway 経由の調整4操作の成功と代表的な 400 が契約どおり(成功はリクエストも照らす)。
func TestRealAdjustThroughGatewayMatchesContract(t *testing.T) {
	h := newRealCalcEnv(t)
	self := realCalcIndividual(calctest.SpeciesAttacker, calctest.NatureAtkUp, spOf(0, 0, 0, 0, 0, 32))
	foe := realCalcIndividual(calctest.SpeciesDefender, calctest.NatureNeutral, spOf(32, 0, 0, 0, 0, 0))

	indices := map[string]any{"individual": self, "moveId": calctest.MovePhysical, "modifier": 6144}
	ko := map[string]any{"format": "single", "attacker": self, "defender": foe, "moveId": calctest.MovePhysical, "hits": 3}
	survive := map[string]any{"format": "single", "attacker": foe, "defender": self, "moveId": calctest.MovePhysical, "hits": 2,
		"thresholdPercent": 87.5}
	alloc := map[string]any{"self": self, "mode": "bulk", "focus": "both", "ceiling": map[string]any{"hp": 20},
		"goal": map[string]any{"format": "single", "opponent": foe, "moveId": calctest.MovePhysical, "hits": 2}}
	with := func(body map[string]any, key string, value any) map[string]any {
		out := make(map[string]any, len(body)+1)
		for k, v := range body {
			out[k] = v
		}
		out[key] = value
		return out
	}

	tests := []struct {
		name       string
		path       string
		body       map[string]any
		wantStatus int
		wantCode   string
	}{
		{"indices の成功", "/api/calc/adjust/indices", indices, http.StatusOK, ""},
		{"min-sp-to-ko の成功", "/api/calc/adjust/min-sp-to-ko", ko, http.StatusOK, ""},
		{"min-sp-to-survive の成功", "/api/calc/adjust/min-sp-to-survive", survive, http.StatusOK, ""},
		{"allocation の成功", "/api/calc/adjust/allocation", alloc, http.StatusOK, ""},
		{"indices: calc-svc の unknown_move はそのまま", "/api/calc/adjust/indices",
			with(indices, "moveId", "test-no-such-move"), http.StatusBadRequest, "unknown_move"},
		{"min-sp-to-ko: calc-svc の invalid_input(hits 11)はそのまま", "/api/calc/adjust/min-sp-to-ko",
			with(ko, "hits", 11), http.StatusBadRequest, "invalid_input"},
		{"allocation: calc-svc の invalid_enum(mode)はそのまま", "/api/calc/adjust/allocation",
			with(alloc, "mode", "balanced"), http.StatusBadRequest, "invalid_enum"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := mustJSON(t, tt.body)
			rec := serve(t, h, http.MethodPost, tt.path, validHeaders(), body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantCode != "" {
				if got := decodeErrorBody(t, rec); got.Code != tt.wantCode {
					t.Errorf("code = %q, want %q", got.Code, tt.wantCode)
				}
			}
			assertContract(t, http.MethodPost, tt.path, validHeaders(), body, rec, tt.wantStatus == http.StatusOK)
		})
	}
}
