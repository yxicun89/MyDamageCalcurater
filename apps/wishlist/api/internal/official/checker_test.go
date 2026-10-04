package official_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
	"example.com/pokecalc/apps/wishlist/api/internal/official"
)

// フェーズ4-3 公式ページの取得(docs/phase4-spec.md AC-O15〜O19)。実サイトは叩かず httptest を使う。

// fakeClock は待たずに時刻を進め、待ちを記録する。
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
}

func newFakeClock() *fakeClock { return &fakeClock{now: time.Date(2026, 10, 3, 18, 0, 0, 0, time.UTC)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if d > 0 {
		c.sleeps = append(c.sleeps, d)
		c.now = c.now.Add(d)
	}
	return nil
}

func (c *fakeClock) takeSleeps() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.sleeps
	c.sleeps = nil
	return out
}

// site は httptest の公式サイト。パスごとに応答を決め、受けたリクエスト(パスと User-Agent)を記録する。
type site struct {
	srv    *httptest.Server
	mu     sync.Mutex
	reqs   []string // "GET /robots.txt"
	agents []string
	routes map[string]func(w http.ResponseWriter, r *http.Request)
}

func newSite(t *testing.T) *site {
	t.Helper()
	s := &site{routes: map[string]func(http.ResponseWriter, *http.Request){}}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.reqs = append(s.reqs, r.Method+" "+r.URL.RequestURI())
		s.agents = append(s.agents, r.Header.Get("User-Agent"))
		h := s.routes[r.URL.Path]
		s.mu.Unlock()
		if h == nil {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *site) handle(path string, h func(w http.ResponseWriter, r *http.Request)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes[path] = h
}

func (s *site) text(path string, status int, ctype, body string) {
	s.handle(path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", ctype)
		w.WriteHeader(status)
		w.Write([]byte(body))
	})
}

func (s *site) robots(status int, body string) { s.text("/robots.txt", status, "text/plain", body) }

func (s *site) page(path, fixture string) {
	b, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		panic(err)
	}
	s.text(path, http.StatusOK, "text/html; charset=utf-8", string(b))
}

func (s *site) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.reqs)
}

func (s *site) userAgents() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.agents)
}

func loopbackClient() *http.Client {
	return netguard.NewClient(netguard.Options{AllowAddr: func(netip.Addr) bool { return true }})
}

func newChecker(clock *fakeClock) *official.Checker {
	return official.NewChecker(official.Config{Client: loopbackClient(), Gate: fetcher.NewHostGate(5*time.Second, nil, clock)})
}

func resultString(r official.Result) string { return fmt.Sprintf("%s %q", r.State, r.Evidence) }

// AC-O15: robots.txt を先に取り、許されていればページを取って判定する。どちらも正直な User-Agent。
func TestChecker_Judges(t *testing.T) {
	s := newSite(t)
	s.robots(http.StatusOK, "User-agent: *\nDisallow: /private/\n")
	s.page("/item/1/", "preorder.html")
	got := newChecker(newFakeClock()).Check(context.Background(), s.srv.URL+"/item/1/")
	if want := `preorder ["予約受付中" "予約する"]`; resultString(got) != want {
		t.Errorf("Check = %s, want %s", resultString(got), want)
	}
	if want := []string{"GET /robots.txt", "GET /item/1/"}; !slices.Equal(s.requests(), want) {
		t.Errorf("リクエスト = %q, want %q", s.requests(), want)
	}
	for _, ua := range s.userAgents() {
		if ua != fetcher.UserAgent {
			t.Errorf("User-Agent = %q, want %q", ua, fetcher.UserAgent)
		}
	}
}

// AC-O16: robots.txt は Checker ごと・ホストごとに 1 回だけ取る(取れなかった結果も覚える)。新しい Checker は取り直す。
func TestChecker_RobotsCachedPerChecker(t *testing.T) {
	s := newSite(t)
	s.robots(http.StatusOK, "User-agent: *\nDisallow: /private/\n")
	s.page("/item/1/", "preorder.html")
	s.page("/item/2/", "ended.html")
	c := newChecker(newFakeClock())
	c.Check(context.Background(), s.srv.URL+"/item/1/")
	c.Check(context.Background(), s.srv.URL+"/item/2/")
	if got := c.Check(context.Background(), s.srv.URL+"/private/x"); got.State != item.OfficialBlocked {
		t.Errorf("Disallow のパス = %s, want blocked", resultString(got))
	}
	if want := []string{"GET /robots.txt", "GET /item/1/", "GET /item/2/"}; !slices.Equal(s.requests(), want) {
		t.Errorf("リクエスト = %q, want %q(robots.txt は 1 回・blocked のページは取らない)", s.requests(), want)
	}
	newChecker(newFakeClock()).Check(context.Background(), s.srv.URL+"/item/1/")
	if n := strings.Count(strings.Join(s.requests(), "\n"), "/robots.txt"); n != 2 {
		t.Errorf("新しい Checker で robots.txt を取り直さない(%d 回)", n)
	}

	// 取れなかった robots.txt(5xx)も覚え、同じ Checker では取り直さない
	f := newSite(t)
	f.robots(http.StatusServiceUnavailable, "")
	f.page("/item/1/", "preorder.html")
	c2 := newChecker(newFakeClock())
	for range 2 {
		if got := c2.Check(context.Background(), f.srv.URL+"/item/1/"); got.State != item.OfficialFailed {
			t.Errorf("robots.txt が 503 = %s, want failed", resultString(got))
		}
	}
	if want := []string{"GET /robots.txt"}; !slices.Equal(f.requests(), want) {
		t.Errorf("リクエスト = %q, want %q", f.requests(), want)
	}
}

// AC-O17: robots.txt の応答ごとの扱い。404・410 は全部許す。それ以外の 4xx は blocked。5xx・大きすぎる・通信失敗は failed。
// blocked・failed のときはページを取らない。根拠は空(nil でない)。
func TestChecker_RobotsStatus(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		want     item.OfficialState
		wantPage bool
	}{
		{"200 で Disallow", http.StatusOK, "User-agent: *\nDisallow: /item/\n", item.OfficialBlocked, false},
		{"200 で自分の名前の Disallow", http.StatusOK, "User-agent: wishlist-price-checker\nDisallow: /\n", item.OfficialBlocked, false},
		{"200 で許可", http.StatusOK, "User-agent: *\nDisallow:\n", item.OfficialPreorder, true},
		{"404 は全部許す", http.StatusNotFound, "", item.OfficialPreorder, true},
		{"410 は全部許す", http.StatusGone, "", item.OfficialPreorder, true},
		{"401 は blocked", http.StatusUnauthorized, "", item.OfficialBlocked, false},
		{"403 は blocked", http.StatusForbidden, "", item.OfficialBlocked, false},
		{"500 は failed", http.StatusInternalServerError, "", item.OfficialFailed, false},
		{"503 は failed", http.StatusServiceUnavailable, "", item.OfficialFailed, false},
		{"大きすぎる robots.txt は failed", http.StatusOK, "User-agent: *\n" + strings.Repeat("# x\n", official.MaxRobotsBytes/4+1), item.OfficialFailed, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSite(t)
			s.robots(c.status, c.body)
			s.page("/item/1/", "preorder.html")
			got := newChecker(newFakeClock()).Check(context.Background(), s.srv.URL+"/item/1/")
			if got.State != c.want {
				t.Errorf("Check = %s, want %s", resultString(got), c.want)
			}
			if !c.wantPage && (got.Evidence == nil || len(got.Evidence) != 0) {
				t.Errorf("blocked・failed の根拠 = %#v, want 空(nil でない)", got.Evidence)
			}
			fetchedPage := slices.Contains(s.requests(), "GET /item/1/")
			if fetchedPage != c.wantPage {
				t.Errorf("ページを取った = %v, want %v(リクエスト %q)", fetchedPage, c.wantPage, s.requests())
			}
		})
	}

	t.Run("通信失敗は failed", func(t *testing.T) {
		s := newSite(t)
		u := s.srv.URL
		s.srv.Close()
		if got := newChecker(newFakeClock()).Check(context.Background(), u+"/item/1/"); got.State != item.OfficialFailed {
			t.Errorf("Check = %s, want failed", resultString(got))
		}
	})
}

// AC-O18: ページの取得の失敗は failed(2xx 以外・大きすぎる・別のホストへのリダイレクト・http(s) でない URL・ctx の終了)。
// 同じホストのリダイレクトは追う(移動先が Disallow なら blocked)。
func TestChecker_PageFailures(t *testing.T) {
	other := newSite(t)
	other.page("/item/1/", "available.html")

	s := newSite(t)
	s.robots(http.StatusOK, "User-agent: *\nDisallow: /private/\n")
	s.text("/gone/", http.StatusNotFound, "text/html", "<body>販売中</body>")
	s.text("/error/", http.StatusInternalServerError, "text/html", "<body>販売中</body>")
	s.text("/huge/", http.StatusOK, "text/html", "<body>販売中"+strings.Repeat(" ", official.MaxPageBytes)+"</body>")
	s.page("/item/1/", "ended.html")
	s.handle("/moved/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/item/1/", http.StatusFound) })
	s.handle("/to-private/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/private/1", http.StatusFound) })
	s.handle("/away/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.srv.URL+"/item/1/", http.StatusFound)
	})

	cases := []struct {
		path string
		want item.OfficialState
	}{
		{"/gone/", item.OfficialFailed},
		{"/error/", item.OfficialFailed},
		{"/huge/", item.OfficialFailed},
		{"/away/", item.OfficialFailed},
		{"/moved/", item.OfficialEnded},
		{"/to-private/", item.OfficialBlocked},
	}
	c := newChecker(newFakeClock())
	for _, tc := range cases {
		if got := c.Check(context.Background(), s.srv.URL+tc.path); got.State != tc.want {
			t.Errorf("%s = %s, want %s", tc.path, resultString(got), tc.want)
		}
	}
	if reqs := other.requests(); len(reqs) != 0 {
		t.Errorf("別のホスト(ポート違い)へのリダイレクトを追った: %q", reqs)
	}
	if slices.Contains(s.requests(), "GET /private/1") {
		t.Error("Disallow のパスへのリダイレクトを追った")
	}

	for _, raw := range []string{"ftp://example.com/a", "javascript:alert(1)", "/item/1/", ""} {
		if got := c.Check(context.Background(), raw); got.State != item.OfficialFailed {
			t.Errorf("Check(%q) = %s, want failed", raw, resultString(got))
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n := len(s.requests())
	if got := newChecker(newFakeClock()).Check(ctx, s.srv.URL+"/item/1/"); got.State != item.OfficialFailed {
		t.Errorf("ctx の終了 = %s, want failed", resultString(got))
	}
	if len(s.requests()) != n {
		t.Error("ctx が終わっているのに取得した")
	}
}

// AC-O19: robots.txt とページの取得はどちらも Gate を通る(同じホストは間隔をあける)。共有した Gate で、直前の別の取得とも間隔をあける。
func TestChecker_UsesGate(t *testing.T) {
	s := newSite(t)
	s.robots(http.StatusOK, "User-agent: *\nDisallow:\n")
	s.page("/item/1/", "preorder.html")
	s.page("/item/2/", "ended.html")
	clock := newFakeClock()
	gate := fetcher.NewHostGate(5*time.Second, nil, clock)
	c := official.NewChecker(official.Config{Client: loopbackClient(), Gate: gate})
	c.Check(context.Background(), s.srv.URL+"/item/1/")
	if got := clock.takeSleeps(); !slices.Equal(got, []time.Duration{5 * time.Second}) {
		t.Errorf("robots.txt → ページの待ち = %v, want [5s]", got)
	}
	c.Check(context.Background(), s.srv.URL+"/item/2/")
	if got := clock.takeSleeps(); !slices.Equal(got, []time.Duration{5 * time.Second}) {
		t.Errorf("2 つ目のページの待ち = %v, want [5s](robots.txt は取り直さない)", got)
	}

	// 共有した Gate:価格の取得(同じホスト)の直後なら robots.txt の前にも待つ
	s2 := newSite(t)
	s2.robots(http.StatusNotFound, "")
	s2.page("/item/1/", "preorder.html")
	clock2 := newFakeClock()
	gate2 := fetcher.NewHostGate(5*time.Second, nil, clock2)
	if err := gate2.Do(context.Background(), fetcher.HostOf(s2.srv.URL), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	official.NewChecker(official.Config{Client: loopbackClient(), Gate: gate2}).Check(context.Background(), s2.srv.URL+"/item/1/")
	if got := clock2.takeSleeps(); !slices.Equal(got, []time.Duration{5 * time.Second, 5 * time.Second}) {
		t.Errorf("待ち = %v, want [5s 5s](直前の取得 → robots.txt → ページ)", got)
	}
}
