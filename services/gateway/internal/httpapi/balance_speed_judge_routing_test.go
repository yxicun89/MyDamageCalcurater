package httpapi

// `/api/balance/*`・`/api/speed/*`・`/api/judge/*` のルーティング(issue #284。ADR-0202 §3 への追記)。
// balance・speed・judge の3サービスは Traefik が gateway を経由せず直接ルーティングしていたため、
// 端末ID/セッションIDの検証が gateway とそれぞれのサービスで別々に実装されていた(issue #236)。
// gateway の後ろにまとめることで検証を1か所に集約する。
//
// 実装済み: `Config.BalanceURL`/`SpeedURL`/`JudgeURL`(`GATEWAY_BALANCE_URL`/`GATEWAY_SPEED_URL`/
// `GATEWAY_JUDGE_URL`)・routing.go の routeBalance/routeSpeed/routeJudge と対応する prefix・
// requiresHeaderCheck への追加。CORS の許可メソッドは変更なし(balance・speed・judge の契約はいずれも
// GET/POST のみで PUT/PATCH/DELETE を使わない。api/openapi.yaml とは別契約の
// `services/{balance,speed,judge}/api/openapi.yaml` で確認済み)。実際に各上流へ転送されることは
// `TestRoutesReachTheirUpstream`(routing_test.go)に3行追加して確認済み。
//
// `/api/{balance,speed,judge}/healthz`(完全一致のみ)はヘッダ検証を課さない(3サービスの契約の
// `publicHealth`・ADR-0600/ADR-0700 が Ingress 越しの疎通確認用としてヘッダ不要と明記しているため。
// gateway 経由になっても同じ契約を守る。critic 指摘で追加)。
//
// balance・speed・judge 自身が既に持つ端末ID/セッションIDの検証(issue #236 対応。各サービスの
// internal/httpapi 内)は、gateway 経由になったことで二重になるが害はない。複製側を削除するかどうかは
// 各レーンの判断(このタスクの範囲外。DECISIONS.md 参照)。

import (
	"net/http"
	"testing"
)

// service はテスト対象のサービス1つ分(名前・プレフィックス・代表パス・専用の testEnv コンストラクタ)。
type gatewayBackedService struct {
	name        string
	prefix      string // 末尾スラッシュ付き。例 "/api/balance/"
	examplePath string
	newEnv      func(t *testing.T, mutate ...func(*Config)) *testEnv
}

var gatewayBackedServices = []gatewayBackedService{
	{name: "balance", prefix: "/api/balance/", examplePath: "/api/balance/v1/team-balance/analyze", newEnv: newBalanceTestEnv},
	{name: "speed", prefix: "/api/speed/", examplePath: "/api/speed/v1/table", newEnv: newSpeedTestEnv},
	{name: "judge", prefix: "/api/judge/", examplePath: "/api/judge/v1/outspeed-and-ko", newEnv: newJudgeTestEnv},
}

// issue #284: `/api/{balance,speed,judge}/*` は gateway のルートとして存在する(404 にならない)。
// 上流が未設定のときは record・team と同じく 503 upstream_unavailable(「契約にあるのに必ず 404」にしない)。
func TestBalanceSpeedJudgePathsAreRouted(t *testing.T) {
	for _, svc := range gatewayBackedServices {
		t.Run(svc.name, func(t *testing.T) {
			env := newTestEnv(t) // 上流は未設定のまま(newBalanceTestEnv 等は使わない)。
			rec := serve(t, env.handler, http.MethodGet, svc.prefix+"whatever", validHeaders(), nil)
			assertGatewayError(t, rec, http.StatusServiceUnavailable, "upstream_unavailable")
			for _, up := range env.upstreams() {
				if n := len(up.requests()); n != 0 {
					t.Errorf("上流 %s に %d 件届いた, want 0(%s は別の上流)", up.name, n, svc.name)
				}
			}
		})
	}
}

// `/api/{balance,speed,judge}`(末尾スラッシュ無し)と `/api/{balance,speed,judge}x` は404
// (record・team と同じ規則。ADR-0202 §3)。
func TestBalanceSpeedJudgePrefixIsExact(t *testing.T) {
	for _, svc := range gatewayBackedServices {
		base := svc.prefix[:len(svc.prefix)-1] // 末尾スラッシュを外す。例 "/api/balance"
		for _, path := range []string{base, base + "x", base + "x/whatever"} {
			t.Run(svc.name+" "+path, func(t *testing.T) {
				env := newTestEnv(t)
				rec := serve(t, env.handler, http.MethodGet, path, validHeaders(), nil)
				assertGatewayError(t, rec, http.StatusNotFound, "not_found")
			})
		}
	}
}

// `/api/{balance,speed,judge}/*` にもヘッダ検証がかかる(ADR-0202 §4。検証は上流の有無より先。issue #236 の
// gateway 側の穴〈Traefik 直結でヘッダ検証を経由しない〉をこれで塞ぐ)。
func TestBalanceSpeedJudgePathsRequireClientIDHeaders(t *testing.T) {
	for _, svc := range gatewayBackedServices {
		t.Run(svc.name, func(t *testing.T) {
			env := newTestEnv(t)
			rec := serve(t, env.handler, http.MethodGet, svc.examplePath, http.Header{}, nil)
			assertGatewayError(t, rec, http.StatusBadRequest, "missing_header")
		})
	}
}

// issue #284 critic 指摘(重要-1): `/api/{balance,speed,judge}/healthz`(完全一致のみ)はヘッダ検証を
// 課さない。3サービスの契約(`services/{balance,speed,judge}/api/openapi.yaml` の `publicHealth`)・
// ADR-0600/ADR-0700 は Ingress 越しの疎通確認用としてヘッダ不要と明記しており、gateway 経由になっても
// 同じ契約を守る必要がある。上流が設定されていれば転送され、未設定なら record・team と同じ 503。
func TestBalanceSpeedJudgeHealthzDoesNotRequireHeaders(t *testing.T) {
	for _, svc := range gatewayBackedServices {
		healthPath := svc.prefix + "healthz"
		t.Run(svc.name+" 上流設定済み", func(t *testing.T) {
			env := svc.newEnv(t)
			target := map[string]*fakeUpstream{"balance": env.balance, "speed": env.speed, "judge": env.judge}[svc.name]
			target.respond(upstreamResponse{status: http.StatusOK, contentType: "application/json", body: `{"status":"ok"}`})

			rec := serve(t, env.handler, http.MethodGet, healthPath, http.Header{}, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200(ヘッダ無しでも通る); body=%s", rec.Code, rec.Body.String())
			}
			reqs := target.requests()
			if len(reqs) != 1 {
				t.Fatalf("上流 %s に届いた回数 = %d, want 1", svc.name, len(reqs))
			}
			if reqs[0].Path != healthPath {
				t.Errorf("上流に届いたパス = %q, want %q", reqs[0].Path, healthPath)
			}
		})
		t.Run(svc.name+" 上流未設定", func(t *testing.T) {
			env := newTestEnv(t) // 上流は未設定のまま。
			rec := serve(t, env.handler, http.MethodGet, healthPath, http.Header{}, nil)
			assertGatewayError(t, rec, http.StatusServiceUnavailable, "upstream_unavailable")
		})
	}
}

// `/api/{balance,speed,judge}/healthz` に似た別パス(`healthzz`・`healthz/x`)はヘッダ検証の例外に
// ならない(完全一致だけを緩めていることの固定。前方一致に緩めると本来の代表パスまで検証が抜ける穴になる)。
func TestBalanceSpeedJudgeHealthzLookalikesStillRequireHeaders(t *testing.T) {
	for _, svc := range gatewayBackedServices {
		for _, suffix := range []string{"healthzz", "healthz/x"} {
			path := svc.prefix + suffix
			t.Run(svc.name+" "+path, func(t *testing.T) {
				env := newTestEnv(t)
				rec := serve(t, env.handler, http.MethodGet, path, http.Header{}, nil)
				assertGatewayError(t, rec, http.StatusBadRequest, "missing_header")
			})
		}
	}
}

// issue #284: 上流が設定されていれば実際に転送され、ヘッダも届く(TestRoutesReachTheirUpstream の
// 個別ケースを補い、3サービスまとめての回帰確認にする)。
func TestBalanceSpeedJudgePathsReachUpstreamWhenConfigured(t *testing.T) {
	for _, svc := range gatewayBackedServices {
		t.Run(svc.name, func(t *testing.T) {
			env := svc.newEnv(t)
			target := map[string]*fakeUpstream{"balance": env.balance, "speed": env.speed, "judge": env.judge}[svc.name]
			target.respond(upstreamResponse{status: http.StatusOK, contentType: "application/json", body: `{"ok":true}`})

			rec := serve(t, env.handler, http.MethodGet, svc.examplePath, validHeaders(), nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			reqs := target.requests()
			if len(reqs) != 1 {
				t.Fatalf("上流 %s に届いた回数 = %d, want 1", svc.name, len(reqs))
			}
			if reqs[0].Path != svc.examplePath {
				t.Errorf("上流に届いたパス = %q, want %q", reqs[0].Path, svc.examplePath)
			}
			for _, other := range env.upstreams() {
				if other != target && len(other.requests()) != 0 {
					t.Errorf("別の上流 %s にも届いた", other.name)
				}
			}
		})
	}
}
