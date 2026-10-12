package httpapi

// 「端末 ID は認証ではない」「CORS は到達制御ではない」ことの回帰テスト(ADR-0210 §4。issue #148)。
//
// この2つは ADR-0202 §4・§6 と ADR-0209 §2 で既に決まっているが、
//   - gateway が Authorization / Cookie / X-Api-Key を一切見ない
//   - 端末 ID が gateway の分岐に使われない
//   - 端末 ID の不正は 400 であって 401 / 403 ではない
//   - 許可外オリジン・Origin 無しでも上流に到達する(CORS はブラウザの約束にすぎない)
// は、どのテストも固定していなかった(ADR-0210 §4.2 の G1〜G4)。
//
// 固定しておきたい理由: 私設サービスを維持する決定(DECISIONS.md 2026-09-23)は「到達させない」ことだけを
// 防御線にしている(ADR-0210 §1.2 の T1)。gateway に半端な認証(「Authorization があれば通す」等)が
// 生えると、端末 ID や CORS が認証・アクセス制御に見え、公開しても大丈夫だと誤読されうる。

import (
	"net/http"
	"testing"
)

// 架空の「認証らしい」ヘッダ。gateway はこれらを見ない(ADR-0210 §4.2 G1)。
var authLookingHeaders = []struct {
	name  string
	key   string
	value string
}{
	{"Authorization(Bearer)", "Authorization", "Bearer 00000000-0000-4000-8000-00000000000a"},
	{"Authorization(Basic)", "Authorization", "Basic dXNlcjpwYXNz"},
	{"Cookie", "Cookie", "session=00000000-0000-4000-8000-00000000000b"},
	{"X-Api-Key", "X-Api-Key", "00000000-0000-4000-8000-00000000000c"},
}

// AC-B5: 端末 ID は認証ではない。
func TestDeviceIDIsNotAuthentication(t *testing.T) {
	const path = "/api/calc"
	body := []byte(`{}`)

	// G1: 事前に払い出した値でなくても到達する(「登録済みの端末 ID」という概念が無い)。
	t.Run("任意の UUID で到達できる(払い出し・登録の概念が無い)", func(t *testing.T) {
		for _, deviceID := range []string{
			"11111111-1111-4111-8111-111111111111",
			"22222222-2222-4222-8222-222222222222",
			"deadbeef-dead-4bee-8bee-deadbeefdead",
		} {
			env := newTestEnv(t)
			header := headersWith(func(h http.Header) { h.Set("X-Device-Id", deviceID) })
			rec := serve(t, env.handler, http.MethodPost, path, header, body)
			if rec.Code != http.StatusOK {
				t.Errorf("端末 ID %q: status = %d, want 200(認証が無いので任意の UUID で到達する); body=%s",
					deviceID, rec.Code, rec.Body.String())
			}
			if got := len(env.calc.requests()); got != 1 {
				t.Errorf("端末 ID %q: calc に届いた回数 = %d, want 1", deviceID, got)
			}
		}
	})

	// G1: 認証らしいヘッダを見ない(あっても無くても同じ。これを理由に拒否も許可もしない)。
	t.Run("認証らしいヘッダを見ない", func(t *testing.T) {
		for _, tt := range authLookingHeaders {
			t.Run(tt.name, func(t *testing.T) {
				env := newTestEnv(t)
				header := headersWith(func(h http.Header) { h.Set(tt.key, tt.value) })
				rec := serve(t, env.handler, http.MethodPost, path, header, body)
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200(gateway は %s を見ない); body=%s", rec.Code, tt.key, rec.Body.String())
				}
				reqs := env.calc.requests()
				if len(reqs) != 1 {
					t.Fatalf("calc に届いた回数 = %d, want 1", len(reqs))
				}
				// ADR-0202 §4: 検証を通ったリクエストのヘッダは書き換えずに上流へ転送する。
				if got := reqs[0].Header.Get(tt.key); got != tt.value {
					t.Errorf("上流に届いた %s = %q, want %q(gateway は解釈も削除もしない)", tt.key, got, tt.value)
				}
			})
		}
	})

	// G1: 認証らしいヘッダが無くても拒否されない(「資格情報が必須」ではない)。
	t.Run("認証らしいヘッダが無くても 200", func(t *testing.T) {
		env := newTestEnv(t)
		header := validHeaders()
		for _, name := range []string{"Authorization", "Cookie", "X-Api-Key"} {
			if got := header.Get(name); got != "" {
				t.Fatalf("validHeaders が %s を持っている(テストの前提が崩れている): %q", name, got)
			}
		}
		rec := serve(t, env.handler, http.MethodPost, path, header, body)
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200(資格情報を要求しない); body=%s", rec.Code, rec.Body.String())
		}
	})

	// G2: 端末 ID は gateway の分岐に使われない(分割キーは保存サービスの責務。ADR-0209 §6)。
	t.Run("端末 ID を変えても同じ要求が同じ上流に同じ内容で届く", func(t *testing.T) {
		type sent struct {
			method, path, rawQuery string
			body                   string
			status                 int
			respBody               string
		}
		var got []sent
		for _, deviceID := range []string{
			"33333333-3333-4333-8333-333333333333",
			"44444444-4444-4444-8444-444444444444",
		} {
			env := newTestEnv(t)
			header := headersWith(func(h http.Header) { h.Set("X-Device-Id", deviceID) })
			rec := serve(t, env.handler, http.MethodPost, path, header, body)
			reqs := env.calc.requests()
			if len(reqs) != 1 {
				t.Fatalf("端末 ID %q: calc に届いた回数 = %d, want 1", deviceID, len(reqs))
			}
			got = append(got, sent{
				method: reqs[0].Method, path: reqs[0].Path, rawQuery: reqs[0].RawQuery,
				body: string(reqs[0].Body), status: rec.Code, respBody: rec.Body.String(),
			})
		}
		if got[0] != got[1] {
			t.Errorf("端末 ID で振る舞いが変わっている(gateway は端末 ID で分岐しない):\n%+v\n%+v", got[0], got[1])
		}
	})

	// G3: 端末 ID の不正・欠落は 400(認証の語彙 401 / 403 を使わない)。
	t.Run("不正・欠落は 400 で 401 / 403 ではない", func(t *testing.T) {
		tests := []struct {
			name     string
			header   http.Header
			wantCode string
		}{
			{"端末 ID が無い", headersWith(func(h http.Header) { h.Del("X-Device-Id") }), "missing_header"},
			{"端末 ID が空", headersWith(func(h http.Header) { h.Set("X-Device-Id", "") }), "missing_header"},
			{"端末 ID が UUID でない", headersWith(func(h http.Header) { h.Set("X-Device-Id", "not-a-uuid") }), "invalid_header"},
			{"Bearer トークンのような値", headersWith(func(h http.Header) { h.Set("X-Device-Id", "Bearer abc") }), "invalid_header"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				env := newTestEnv(t)
				rec := serve(t, env.handler, http.MethodPost, path, tt.header, body)
				if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
					t.Fatalf("status = %d(401 / 403 を使わない。端末 ID は資格情報ではない。ADR-0210 §4)", rec.Code)
				}
				assertGatewayError(t, rec, http.StatusBadRequest, tt.wantCode)
				env.assertNoUpstreamReached(t)
			})
		}
	})
}

// AC-B6: CORS はブラウザの約束であって到達制御ではない。許可外オリジン・Origin 無しでも上流に届く。
// 既存の TestCORSSimpleRequests は「CORS ヘッダが付かない」ことだけを見ており、到達したかを見ていない。
func TestCORSIsNotAccessControl(t *testing.T) {
	tests := []struct {
		name   string
		header http.Header
	}{
		{"許可外のオリジン", withOrigin(validHeaders(), disallowedOrigin)},
		{"null オリジン", withOrigin(validHeaders(), "null")},
		{"Origin 無し(curl / ネイティブクライアント相当)", validHeaders()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			rec := serve(t, env.handler, http.MethodPost, "/api/calc", tt.header, []byte(`{}`))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200(CORS は到達を止めない); body=%s", rec.Code, rec.Body.String())
			}
			if got := len(env.calc.requests()); got != 1 {
				t.Errorf("calc に届いた回数 = %d, want 1(CORS はブラウザの約束で、到達制御ではない。ADR-0210 §4)", got)
			}
			// ブラウザに対しては応答を読ませない(既存の決定。ADR-0202 §6)。
			assertNoCORS(t, rec)
		})
	}
}

// AC-B6: CORS を無効(許可オリジン0件)にしても到達は止まらない。
// 「CORS を切れば閉じる」という誤解を防ぐ(ADR-0210 §1.2 の T3)。
func TestCORSDisabledDoesNotBlockAccess(t *testing.T) {
	env := newTestEnv(t, func(cfg *Config) { cfg.CORSAllowedOrigins = nil })
	rec := serve(t, env.handler, http.MethodPost, "/api/calc", withOrigin(validHeaders(), allowedOrigin), []byte(`{}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200(CORS 未設定でも到達する); body=%s", rec.Code, rec.Body.String())
	}
	if got := len(env.calc.requests()); got != 1 {
		t.Errorf("calc に届いた回数 = %d, want 1", got)
	}
	assertNoCORS(t, rec)
}
