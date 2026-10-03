package fetcher_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// フェーズ4-3: ホスト単位の間隔の共有(docs/phase4-spec.md AC-O20・O21)。

// AC-O20: HostGate は同じホストを前回の終了から間隔をあけて実行し、違うホストは待たない。HostMinIntervals のホストは長いほう。
func TestHostGate_Intervals(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock()
	g := fetcher.NewHostGate(5*time.Second, fetcher.HostMinIntervals, clock)
	var order []string
	run := func(host string) {
		t.Helper()
		if err := g.Do(ctx, host, func(context.Context) error { order = append(order, host); return nil }); err != nil {
			t.Fatalf("Do(%s): %v", host, err)
		}
	}
	run("tamashii.jp")
	if s := clock.takeSleeps(); len(s) != 0 {
		t.Errorf("初回で待った: %v", s)
	}
	run("example.com")
	if s := clock.takeSleeps(); len(s) != 0 {
		t.Errorf("違うホストで待った: %v", s)
	}
	clock.Advance(2 * time.Second)
	run("tamashii.jp")
	if s := clock.takeSleeps(); !slices.Equal(s, []time.Duration{3 * time.Second}) {
		t.Errorf("同じホストの待ち = %v, want [3s](前回の終了から 5 秒)", s)
	}
	run("www.suruga-ya.jp")
	run("www.suruga-ya.jp")
	if s := clock.takeSleeps(); !slices.Equal(s, []time.Duration{30 * time.Second}) {
		t.Errorf("駿河屋の待ち = %v, want [30s](HostMinIntervals)", s)
	}
	if want := []string{"tamashii.jp", "example.com", "tamashii.jp", "www.suruga-ya.jp", "www.suruga-ya.jp"}; !slices.Equal(order, want) {
		t.Errorf("実行順 = %v", order)
	}
}

// AC-O20: fn の失敗も「取得した」と数える。待っている間に ctx が終わったら fn を呼ばない。
func TestHostGate_FailureAndCancel(t *testing.T) {
	clock := newFakeClock()
	g := fetcher.NewHostGate(5*time.Second, nil, clock)
	boom := errors.New("boom")
	if err := g.Do(context.Background(), "a.example", func(context.Context) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("fn の err を返さない: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := g.Do(ctx, "a.example", func(context.Context) error { called = true; return nil })
	if !errors.Is(err, context.Canceled) || called {
		t.Errorf("取り消し後: err = %v, called = %v(fn を呼ばずに ctx の err)", err, called)
	}
	if err := g.Do(context.Background(), "a.example", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if s := clock.takeSleeps(); !slices.Equal(s, []time.Duration{5 * time.Second}) {
		t.Errorf("失敗のあとの待ち = %v, want [5s]", s)
	}
}

// AC-O20: HostOf は小文字・ポートなしのホスト名。読めなければ空。
func TestHostOf(t *testing.T) {
	cases := map[string]string{
		"https://Tamashii.JP/item/1/":                   "tamashii.jp",
		"http://example.com:8080/a":                     "example.com",
		"https://www.cardrush-dm.jp/product-list?q={q}": "www.cardrush-dm.jp",
		"://bad": "",
		"":       "",
	}
	for in, want := range cases {
		if got := fetcher.HostOf(in); got != want {
			t.Errorf("HostOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// AC-O21: NewRegistry の Fetcher と Registry.Gate() は同じ HostGate を使う
// (価格の取得の直後に同じホストの公式ページを取るときも間隔をあける)。
func TestRegistry_GateIsShared(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<html><body></body></html>"))
	}))
	defer srv.Close()
	clock := newFakeClock()
	reg := fetcher.NewRegistry(fetcher.Config{
		Client: &http.Client{Transport: rewriteTransport{target: srv.URL}, Timeout: 10 * time.Second},
		Clock:  clock,
	})
	gate := reg.Gate()
	if gate == nil {
		t.Fatal("Gate() が nil")
	}
	site := fetcher.Site{ID: 1, Name: "カードラッシュ", SearchURLTemplate: cardrushTemplate, FetchType: item.FetchScrape}
	f, ok := reg.ForSite(site)
	if !ok {
		t.Fatal("カードラッシュの Fetcher が無い")
	}
	if _, err := f.Fetch(context.Background(), site, "ボルシャック"); err != nil {
		t.Fatal(err)
	}
	clock.takeSleeps()
	if err := gate.Do(context.Background(), "www.cardrush-dm.jp", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if s := clock.takeSleeps(); !slices.Equal(s, []time.Duration{fetcher.MinInterval}) {
		t.Errorf("価格の取得の直後の同じホスト = %v, want [%v](同じ HostGate)", s, fetcher.MinInterval)
	}
	if fetcher.NewRegistryWith(nil).Gate() != nil {
		t.Error("NewRegistryWith の Gate は nil")
	}
}
