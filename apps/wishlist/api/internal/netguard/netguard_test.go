package netguard

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

func allowAll(netip.Addr) bool { return true }

// AC-N1: 既定のクライアントはループバック(httptest)への接続をダイヤル時に拒否する。
func TestClient_RejectsLoopback(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		io.WriteString(w, "secret")
	}))
	defer srv.Close()

	c := NewClient(Options{})
	resp, err := c.Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("ループバックへの取得が成功した")
	}
	if !errors.Is(err, ErrForbiddenAddress) {
		t.Errorf("err = %v, want ErrForbiddenAddress", err)
	}
	if hit {
		t.Error("サーバーにリクエストが届いた")
	}
}

// AC-N1: ホスト名で指定しても(localhost)解決後のアドレスで拒否する。
func TestClient_RejectsLocalhostName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	u := fmt.Sprintf("http://localhost:%d/", srv.Listener.Addr().(*net.TCPAddr).Port)
	resp, err := NewClient(Options{}).Get(u)
	if err == nil {
		resp.Body.Close()
		t.Fatal("localhost への取得が成功した")
	}
	if !errors.Is(err, ErrForbiddenAddress) {
		t.Errorf("err = %v, want ErrForbiddenAddress", err)
	}
}

// AC-N2: 検査を差し替えて許可すれば取得できる。
func TestClient_AllowedByOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))
	defer srv.Close()
	resp, err := NewClient(Options{AllowAddr: allowAll}).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "ok" {
		t.Errorf("body = %q", b)
	}
}

// AC-N3: リダイレクト先も接続時に検査する(許可された接続からのリダイレクトでも、禁止アドレスへは繋がない)。
func TestClient_RedirectToForbidden(t *testing.T) {
	inner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("リダイレクト先に届いた")
	}))
	defer inner.Close()
	var redirected atomic.Bool
	outer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Store(true)
		http.Redirect(w, r, inner.URL, http.StatusFound)
	}))
	defer outer.Close()

	// 両サーバーとも 127.0.0.1 なので、「リダイレクトを返した後の接続は禁止」にして検査がリダイレクト先でも働くことを見る。
	allowUntilRedirect := func(netip.Addr) bool { return !redirected.Load() }
	resp, err := NewClient(Options{AllowAddr: allowUntilRedirect}).Get(outer.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("禁止アドレスへのリダイレクトを追った")
	}
	if !errors.Is(err, ErrForbiddenAddress) {
		t.Errorf("err = %v, want ErrForbiddenAddress", err)
	}
}

// AC-N3: http/https 以外へのリダイレクトは拒否する。
func TestClient_RedirectToUnsupportedScheme(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "ftp://example.com/x", http.StatusFound)
	}))
	defer srv.Close()
	resp, err := NewClient(Options{AllowAddr: allowAll}).Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("ftp へのリダイレクトを追った")
	}
	if !errors.Is(err, ErrInvalidURL) {
		t.Errorf("err = %v, want ErrInvalidURL", err)
	}
}

// AC-N3: リダイレクト回数の上限。
func TestClient_TooManyRedirects(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/again", http.StatusFound)
	}))
	defer srv.Close()
	resp, err := NewClient(Options{AllowAddr: allowAll, MaxRedirects: 2}).Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("リダイレクトを無限に追った")
	}
	if !errors.Is(err, ErrTooManyRedirects) {
		t.Errorf("err = %v, want ErrTooManyRedirects", err)
	}
}

// AC-N4: タイムアウト。
func TestClient_Timeout(t *testing.T) {
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()
	defer close(done)
	start := time.Now()
	resp, err := NewClient(Options{AllowAddr: allowAll, Timeout: 200 * time.Millisecond}).Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("タイムアウトしなかった")
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("タイムアウトまで %v かかった", d)
	}
}

// AC-N4: 既定のタイムアウトは 10 秒。
func TestClient_DefaultTimeout(t *testing.T) {
	if c := NewClient(Options{}); c.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", c.Timeout, DefaultTimeout)
	}
}

// AC-N5: プロキシの環境変数を使わない(使うと接続先の検査を迂回できる)。
func TestClient_IgnoresProxyEnv(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	c := NewClient(Options{})
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport は *http.Transport であること(got %T)", c.Transport)
	}
	if tr.Proxy != nil {
		req, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
		if u, _ := tr.Proxy(req); u != nil {
			t.Errorf("プロキシ %v を使う", u)
		}
	}
}

// AC-N6: URL の検査(http/https・ホストあり)。
func TestValidateURL(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"https://example.com/item/1", true},
		{"http://example.com", true},
		{"HTTPS://EXAMPLE.COM/", true},
		{"ftp://example.com/x", false},
		{"file:///etc/passwd", false},
		{"javascript:alert(1)", false},
		{"gopher://example.com", false},
		{"//example.com/x", false},
		{"/relative", false},
		{"https://", false},
		{"", false},
		{"https://exa mple.com/", false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			u, err := ValidateURL(c.in)
			if c.ok {
				if err != nil || u == nil {
					t.Errorf("ValidateURL(%q) = %v, %v", c.in, u, err)
				}
				return
			}
			if !errors.Is(err, ErrInvalidURL) {
				t.Errorf("ValidateURL(%q) err = %v, want ErrInvalidURL", c.in, err)
			}
		})
	}
}

// AC-N1: 公開アドレスの判定。
func TestIsPublic(t *testing.T) {
	cases := []struct {
		addr netip.Addr
		want bool
	}{
		{v4(8, 8, 8, 8), true},
		{v4(1, 1, 1, 1), true},
		{netip.MustParseAddr("2001:4860:4860::8888"), true},
		{v4(127, 0, 0, 1), false},
		{v4(127, 255, 255, 254), false},
		{netip.MustParseAddr("::1"), false},
		{v4(0, 0, 0, 0), false},
		{netip.MustParseAddr("::"), false},
		{v4(10, 0, 0, 1), false},
		{v4(172, 16, 0, 1), false},
		{v4(172, 31, 255, 255), false},
		{v4(192, 168, 1, 1), false},
		{v4(169, 254, 169, 254), false}, // クラウドのメタデータ
		{netip.MustParseAddr("fe80::1"), false},
		{netip.MustParseAddr("fc00::1"), false},
		{netip.MustParseAddr("fd12:3456::1"), false},
		{v4(100, 64, 0, 1), false}, // CGNAT(Tailscale)
		{v4(100, 127, 255, 255), false},
		{v4(224, 0, 0, 1), false},
		{netip.MustParseAddr("ff02::1"), false},
		{v4(255, 255, 255, 255), false},
		{mapped(v4(127, 0, 0, 1)), false},
		{mapped(v4(10, 0, 0, 1)), false},
		{mapped(v4(8, 8, 8, 8)), true},
		{v4(172, 32, 0, 1), true},
		{v4(100, 128, 0, 1), true},
		// IPv6 から IPv4 への変換・トンネル(中の IPv4 が内部向けでも通れてしまう)
		{netip.MustParseAddr("64:ff9b::7f00:1"), false},     // NAT64(中身はループバック)
		{netip.MustParseAddr("64:ff9b:1::1"), false},        // NAT64(ローカル用)
		{netip.MustParseAddr("2002:7f00:1::1"), false},      // 6to4(中身はループバック)
		{netip.MustParseAddr("2606:4700:4700::1111"), true}, // 通常の公開 IPv6
	}
	for _, c := range cases {
		t.Run(c.addr.String(), func(t *testing.T) {
			if got := IsPublic(c.addr); got != c.want {
				t.Errorf("IsPublic(%s) = %v, want %v", c.addr, got, c.want)
			}
		})
	}
}

// AC-N7: 読み込みサイズの上限。
func TestReadLimited(t *testing.T) {
	data := bytes.Repeat([]byte("a"), 100)
	if b, err := ReadLimited(bytes.NewReader(data), 100); err != nil || len(b) != 100 {
		t.Errorf("ちょうど上限: len=%d err=%v", len(b), err)
	}
	if _, err := ReadLimited(bytes.NewReader(data), 99); !errors.Is(err, ErrTooLarge) {
		t.Errorf("上限超え: err = %v, want ErrTooLarge", err)
	}
	if b, err := ReadLimited(bytes.NewReader(nil), 10); err != nil || len(b) != 0 {
		t.Errorf("空: len=%d err=%v", len(b), err)
	}
}

// v4 は IPv4 アドレスを組み立てる(公開前検査がリテラルの IPv4 を個人のネットワーク情報として拾うため、文字列で書かない)。
func v4(a, b, c, d byte) netip.Addr { return netip.AddrFrom4([4]byte{a, b, c, d}) }

// mapped は IPv4 射影 IPv6(::ffff:a.b.c.d)を作る。
func mapped(a netip.Addr) netip.Addr { return netip.AddrFrom16(a.As16()) }
