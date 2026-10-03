package fetcher_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

func hostSite(id int64, tmpl string) fetcher.Site {
	return fetcher.Site{ID: id, Name: fmt.Sprint("s", id), FetchType: item.FetchAPI, SearchURLTemplate: tmpl}
}

// 待ち合わせはホスト単位: 同じホスト(大文字・ポート違いも同じ)の別サイト行は直列で 5 秒あく。別ホストは待たない。
func TestThrottle_SameHostDifferentSites(t *testing.T) {
	clock := newFakeClock()
	f := fetcher.Throttle(&countingFetcher{}, fetcher.MinInterval, clock)
	a := hostSite(1, "https://Shop.example.com/search?q={q}")
	b := hostSite(2, "https://shop.example.com:8443/find/{q}")
	c := hostSite(3, "https://other.example.com/s?q={q}")

	mustFetch(t, f, a)
	if s := clock.takeSleeps(); len(s) != 0 {
		t.Errorf("初回で待った: %v", s)
	}
	mustFetch(t, f, b)
	if s := clock.takeSleeps(); !eqDurations(s, []time.Duration{5 * time.Second}) {
		t.Errorf("同じホストの別サイトの待ち = %v, want [5s]", s)
	}
	mustFetch(t, f, c)
	if s := clock.takeSleeps(); len(s) != 0 {
		t.Errorf("別ホストで待った: %v", s)
	}
}

func TestThrottle_SameHostNoParallel(t *testing.T) {
	clock := newFakeClock()
	inner := &countingFetcher{during: func() { time.Sleep(5 * time.Millisecond) }}
	f := fetcher.Throttle(inner, fetcher.MinInterval, clock)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() { f.Fetch(context.Background(), hostSite(int64(i+1), "https://same.example.com/{q}"), "q") })
	}
	wg.Wait()
	if inner.overlap.Load() {
		t.Error("同じホストへの取得が並列に走った")
	}
}

// ホストが読めないテンプレートは Site.ID で待ち合わせる(同じ ID は待ち、違う ID は待たない)。
func TestThrottle_UnparsableHostFallsBackToID(t *testing.T) {
	clock := newFakeClock()
	f := fetcher.Throttle(&countingFetcher{}, fetcher.MinInterval, clock)
	mustFetch(t, f, hostSite(1, "::not a url"))
	mustFetch(t, f, hostSite(2, ""))
	if s := clock.takeSleeps(); len(s) != 0 {
		t.Errorf("違う ID で待った: %v", s)
	}
	mustFetch(t, f, hostSite(1, "::not a url"))
	if s := clock.takeSleeps(); !eqDurations(s, []time.Duration{5 * time.Second}) {
		t.Errorf("同じ ID の待ち = %v, want [5s]", s)
	}
}

func TestParseYen_UpperBound(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"99,999,999円", true},
		{"100,000,000円", false},
		{"99999999999999999999円", false},
	}
	for _, c := range cases {
		if _, ok := fetcher.ParseYen(c.in); ok != c.ok {
			t.Errorf("ParseYen(%q) ok = %v, want %v", c.in, ok, c.ok)
		}
	}
}

// Yahoo!フリマの価格は範囲外(0・上限超え)の出品を除く。
func TestYahooFurima_PriceRange(t *testing.T) {
	page := furimaPage(
		furimaItem("a", "A", 0, "OPEN"),
		furimaItem("b", "B", 100, "OPEN"),
		furimaItem("c", "C", fetcher.MaxPrice, "OPEN"),
		furimaItem("d", "D", fetcher.MaxPrice+1, "OPEN"),
		furimaItem("e", "E", 1000000000000, "OPEN"),
	)
	srv := newHTMLServer(t, serveHTML(page))
	site := fetcher.Site{ID: 3, FetchType: item.FetchScrape, SearchURLTemplate: srv.URL + "/search/{q}"}
	got, err := fetcher.NewYahooFurima(allowAll()).Fetch(context.Background(), site, "x")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Title != "B" || got[1].Title != "C" {
		t.Errorf("got = %+v, want B と C だけ", got)
	}
}

// Yahoo!ショッピングの価格も同じ範囲。
func TestYahoo_PriceRange(t *testing.T) {
	body := fmt.Sprintf(`{"hits":[
{"name":"zero","price":0,"url":"https://e.example/0"},
{"name":"ok","price":100,"url":"https://e.example/1"},
{"name":"max","price":%d,"url":"https://e.example/2"},
{"name":"over","price":%d,"url":"https://e.example/3"}]}`, fetcher.MaxPrice, fetcher.MaxPrice+1)
	ys := newYahooServer(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(body)) })
	got, err := newTestYahoo(ys).Fetch(context.Background(), yahooSite, "q")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Title != "ok" || got[1].Title != "max" {
		t.Errorf("got = %+v, want ok と max だけ", got)
	}
}

func TestUserAgent_Sent(t *testing.T) {
	var mu sync.Mutex
	var uas []string
	h := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		uas = append(uas, r.Header.Get("User-Agent"))
		mu.Unlock()
		w.Write([]byte(furimaPage()))
	}
	srv := newHTMLServer(t, h)
	site := fetcher.Site{ID: 3, FetchType: item.FetchScrape, SearchURLTemplate: srv.URL + "/search/{q}"}
	fetcher.NewYahooFurima(allowAll()).Fetch(context.Background(), site, "x")

	ys := newYahooServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		uas = append(uas, r.Header.Get("User-Agent"))
		mu.Unlock()
		w.Write([]byte(`{"hits":[]}`))
	})
	newTestYahoo(ys).Fetch(context.Background(), yahooSite, "q")

	if len(uas) != 2 {
		t.Fatalf("リクエスト = %d 回, want 2", len(uas))
	}
	for _, ua := range uas {
		if ua != fetcher.UserAgent || ua == "" {
			t.Errorf("User-Agent = %q, want %q", ua, fetcher.UserAgent)
		}
	}
}

// ホストごとの最小間隔: 駿河屋(www.suruga-ya.jp)は 30 秒、ほかは 5 秒。
func TestThrottle_HostMinIntervals(t *testing.T) {
	clock := newFakeClock()
	f := fetcher.ThrottleWith(&countingFetcher{}, fetcher.MinInterval, fetcher.HostMinIntervals, clock)
	suruga := hostSite(1, "https://www.suruga-ya.jp/search?search_word={q}")
	other := hostSite(2, "https://shop.example.com/s?q={q}")

	mustFetch(t, f, suruga)
	mustFetch(t, f, suruga)
	if s := clock.takeSleeps(); !eqDurations(s, []time.Duration{30 * time.Second}) {
		t.Errorf("駿河屋の待ち = %v, want [30s]", s)
	}
	mustFetch(t, f, other)
	mustFetch(t, f, other)
	if s := clock.takeSleeps(); !eqDurations(s, []time.Duration{5 * time.Second}) {
		t.Errorf("既定の待ち = %v, want [5s]", s)
	}
}

func TestHostIntervalTable(t *testing.T) {
	if d := fetcher.HostMinIntervals["www.suruga-ya.jp"]; d != 30*time.Second {
		t.Errorf("駿河屋の間隔 = %v, want 30s", d)
	}
}

// 本番の表の駿河屋は 30 秒あけ、夜間専用の印が付く。ほかのサイトは付かない。
func TestRegistry_SurugayaThrottleAndNightlyOnly(t *testing.T) {
	r := fetcher.NewRegistry(fetcher.Config{})
	s := siteOf(6, item.FetchScrape, surugayaTemplate)
	if !r.NightlyOnly(s) {
		t.Error("駿河屋に NightlyOnly の印が無い")
	}
	for _, tmpl := range []string{cardrushTemplate, amiamiTemplate, furimaTemplate, shoppingTemplate} {
		if r.NightlyOnly(siteOf(1, item.FetchScrape, tmpl)) {
			t.Errorf("%s が NightlyOnly", tmpl)
		}
	}
}
