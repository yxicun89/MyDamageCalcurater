package httpapi

// Host / X-Forwarded-* の転送(ADR-0202 §5 推奨3)。クライアントの Host はそのまま上流に渡さず、
// 上流の基底 URL のホストに書き換える。クライアントが送った X-Forwarded-For / X-Forwarded-Host /
// X-Forwarded-Proto は信用せず、gateway が実際のクライアント IP・元の Host・スキームで置き換える。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// AC 推奨3: 上流に届く Host は gateway 自身の Host(client-supplied)ではなく上流のホストになる。
// クライアントが送った X-Forwarded-For / X-Forwarded-Host / X-Forwarded-Proto は置き換わり、
// クライアントが注入した値(スプーフィング)は残らない。
func TestForwardedHeadersAreRewrittenToUpstream(t *testing.T) {
	env := newTestEnv(t)
	header := validHeaders()
	header.Set("X-Forwarded-For", "spoofed-client") // クライアントが偽装した値(IP ではない)
	header.Set("X-Forwarded-Host", "evil.example.test")
	header.Set("X-Forwarded-Proto", "https")

	req := httptest.NewRequest(http.MethodPost, "/api/calc", strings.NewReader(`{}`))
	req.Host = "client-supplied.example.test"
	req.RemoteAddr = "127.0.0.1:54321"
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	reqs := env.calc.requests()
	if len(reqs) != 1 {
		t.Fatalf("上流に届いた回数 = %d, want 1", len(reqs))
	}
	got := reqs[0]

	wantHost := env.calc.url(t).Host
	if got.Host != wantHost {
		t.Errorf("上流が受け取った Host = %q, want %q(上流のホスト。クライアントの Host ではない)", got.Host, wantHost)
	}
	if xff := got.Header.Get("X-Forwarded-For"); xff != "127.0.0.1" {
		t.Errorf("X-Forwarded-For = %q, want クライアントの実際の IP(127.0.0.1)。偽装した値(spoofed-client)が残っていないこと", xff)
	}
	if xfh := got.Header.Get("X-Forwarded-Host"); xfh != "client-supplied.example.test" {
		t.Errorf("X-Forwarded-Host = %q, want リクエストの実際の Host(偽装した evil.example.test ではない)", xfh)
	}
	if xfp := got.Header.Get("X-Forwarded-Proto"); xfp != "http" {
		t.Errorf("X-Forwarded-Proto = %q, want http(TLS なし。クライアントが送った https を信用しない)", xfp)
	}
}

// issue #326: クライアントが `Connection` に X-Device-Id / X-Session-Id を列挙しても(hop-by-hop 扱いで
// ReverseProxy が消す)、gateway が検証した値がそのまま上流に届く。
func TestVerifiedIDsSurviveConnectionHeader(t *testing.T) {
	const upperDevice = "00000000-0000-4000-8000-00000000ABCD" // 大文字もそのまま届く(§4)
	tests := []struct {
		name       string
		method     string
		path       string
		upstream   func(*testEnv) *fakeUpstream
		connection []string
	}{
		{"calc・1つのヘッダに列挙", http.MethodPost, "/api/calc", func(e *testEnv) *fakeUpstream { return e.calc },
			[]string{"X-Device-Id, X-Session-Id"}},
		{"calc・小文字で別々のヘッダに列挙", http.MethodPost, "/api/calc/bulk", func(e *testEnv) *fakeUpstream { return e.calc },
			[]string{"x-device-id", "keep-alive, x-session-id"}},
		{"pokedex", http.MethodGet, "/api/pokedex/natures", func(e *testEnv) *fakeUpstream { return e.pokedex },
			[]string{"X-Device-Id, X-Session-Id"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			header := validHeaders()
			header.Set("X-Device-Id", upperDevice)
			header["Connection"] = tc.connection

			rec := serve(t, env.handler, tc.method, tc.path, header, []byte(`{}`))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			reqs := tc.upstream(env).requests()
			if len(reqs) != 1 {
				t.Fatalf("上流に届いた回数 = %d, want 1", len(reqs))
			}
			got := reqs[0].Header
			if v := got.Values("X-Device-Id"); len(v) != 1 || v[0] != upperDevice {
				t.Errorf("上流の X-Device-Id = %q, want [%q](検証済みの値がちょうど1つ)", v, upperDevice)
			}
			if v := got.Values("X-Session-Id"); len(v) != 1 || v[0] != testSessionID {
				t.Errorf("上流の X-Session-Id = %q, want [%q](検証済みの値がちょうど1つ)", v, testSessionID)
			}
		})
	}
}

// issue #326: クライアントが送った X-Real-Ip / Forwarded はどの上流にも届かない(偽装したクライアント IP を
// 上流が信じないように)。X-Forwarded-For は gateway が直前の相手の IP で付け直す(TestForwardedHeadersAreRewrittenToUpstream)。
func TestClientIPHeadersAreNotForwarded(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		header   http.Header
		upstream func(*testEnv) *fakeUpstream
	}{
		{"calc", http.MethodPost, "/api/calc", validHeaders(), func(e *testEnv) *fakeUpstream { return e.calc }},
		{"pokedex", http.MethodGet, "/api/pokedex/natures", validHeaders(), func(e *testEnv) *fakeUpstream { return e.pokedex }},
		{"assets", http.MethodGet, "/assets/manifest.json", http.Header{}, func(e *testEnv) *fakeUpstream { return e.assets }},
		{"web", http.MethodGet, "/index.html", http.Header{}, func(e *testEnv) *fakeUpstream { return e.web }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newWebTestEnv(t)
			// 偽装した値(IP の形でなくてよい。素通しされたかどうかだけを見る)。
			header := tc.header.Clone()
			header.Set("X-Real-Ip", "spoofed-real-ip")
			header.Set("Forwarded", "for=spoofed-client;proto=https")
			header.Set("X-Forwarded-For", "spoofed-client")

			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.RemoteAddr = "127.0.0.1:54321"
			for k, vs := range header {
				req.Header[k] = vs
			}
			rec := httptest.NewRecorder()
			env.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			reqs := tc.upstream(env).requests()
			if len(reqs) != 1 {
				t.Fatalf("上流に届いた回数 = %d, want 1", len(reqs))
			}
			got := reqs[0].Header
			for _, name := range []string{"X-Real-Ip", "Forwarded"} {
				if v := got.Values(name); len(v) != 0 {
					t.Errorf("上流に %s = %q が届いた(クライアントの値を素通ししない)", name, v)
				}
			}
			if xff := got.Get("X-Forwarded-For"); xff != "127.0.0.1" {
				t.Errorf("X-Forwarded-For = %q, want 127.0.0.1(直前の相手の IP。クライアントが送った値ではない)", xff)
			}
		})
	}
}
