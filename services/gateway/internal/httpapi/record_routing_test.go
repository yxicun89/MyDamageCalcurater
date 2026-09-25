package httpapi

// `/api/record/*` のルーティングと CORS の DELETE 許可(ADR-0209 §10・ADR-0202 §3・§6 への追記)。
// AC-P9(ADR-0209。ルーティング)・AC-P8(CORS プリフライトが DELETE を許可する)。
//
// 実装済み: `Config.RecordURL`(`GATEWAY_RECORD_URL`)・routing.go の routeRecord/prefixRecord・
// requiresHeaderCheck・cors.go の corsAllowMethods への DELETE 追加。実際に record-svc へ転送される
// こと(AC-G1 相当)は `TestRoutesReachTheirUpstream` の表に record の行を追加して確認済み。

import (
	"net/http"
	"strings"
	"testing"
)

// AC-P9(ADR-0209): `/api/record/*` は gateway のルートとして存在する(404 にならない)。
// 上流が未設定のときは pokedex と同じく 503 upstream_unavailable(「契約にあるのに必ず 404」にしない)。
func TestRecordPathsAreRouted(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"全削除", http.MethodDelete, "/api/record/device-data"},
		{"よく使う相手", http.MethodGet, "/api/record/frequent-opponents"},
		{"未知の下位パスも上流に任せる", http.MethodGet, "/api/record/whatever"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			rec := serve(t, env.handler, tt.method, tt.path, validHeaders(), nil)
			// RecordURL 未設定のあいだは 503 upstream_unavailable。実装後もここは変えない
			// (未設定の上流は 503、というのが ADR-0202 §3 の規則)。
			assertGatewayError(t, rec, http.StatusServiceUnavailable, "upstream_unavailable")
			for _, up := range env.upstreams() {
				if n := len(up.requests()); n != 0 {
					t.Errorf("上流 %s に %d 件届いた, want 0(record は別の上流)", up.name, n)
				}
			}
		})
	}
}

// `/api/record`(末尾スラッシュ無し)と `/api/recordx` は 404(pokedex と同じ規則。ADR-0202 §3)。
func TestRecordPrefixIsExact(t *testing.T) {
	for _, path := range []string{"/api/record", "/api/recordx", "/api/recordx/device-data"} {
		t.Run(path, func(t *testing.T) {
			env := newTestEnv(t)
			rec := serve(t, env.handler, http.MethodGet, path, validHeaders(), nil)
			assertGatewayError(t, rec, http.StatusNotFound, "not_found")
		})
	}
}

// `/api/record/*` にもヘッダ検証がかかる(ADR-0202 §4。検証は上流の有無より先)。
func TestRecordPathsRequireClientIDHeaders(t *testing.T) {
	env := newTestEnv(t)
	rec := serve(t, env.handler, http.MethodDelete, "/api/record/device-data", http.Header{}, nil)
	assertGatewayError(t, rec, http.StatusBadRequest, "missing_header")
}

// AC-P8: 許可オリジンからの `Access-Control-Request-Method: DELETE` のプリフライトが
// DELETE を許可して返る(これが無いと Web から全削除 API を呼べない。ADR-0209 §10-2)。
func TestCORSPreflightAllowsDelete(t *testing.T) {
	env := newTestEnv(t)
	h := http.Header{}
	h.Set("Origin", allowedOrigin)
	h.Set("Access-Control-Request-Method", http.MethodDelete)
	h.Set("Access-Control-Request-Headers", "content-type, x-device-id, x-session-id")

	rec := serve(t, env.handler, http.MethodOptions, "/api/record/device-data", h, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	assertCORSAllowed(t, rec, allowedOrigin)
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !methodAllowed(got, http.MethodDelete) {
		t.Errorf("Access-Control-Allow-Methods = %q に DELETE が無い(AC-P8)", got)
	}
	for _, up := range env.upstreams() {
		if n := len(up.requests()); n != 0 {
			t.Errorf("プリフライトが上流 %s に届いた(%d 件)", up.name, n)
		}
	}
}

// methodAllowed は Access-Control-Allow-Methods にそのメソッドが含まれるかを返す。
func methodAllowed(header, method string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(part), method) {
			return true
		}
	}
	return false
}
