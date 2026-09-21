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
