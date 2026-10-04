package fetcher_test

import (
	"context"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// AC-H7: 登録表。headless は Config.Renderer があるときだけ、fetch_type=headless かつホストが jp.mercari.com(大文字小文字・ポートは無視)の
// サイトを取得する。Renderer が無ければ(Chromium が無い環境)取得しない。ほかの型・ホストは取得しない。
func TestRegistry_MercariNeedsRenderer(t *testing.T) {
	cases := []struct {
		name     string
		site     fetcher.Site
		renderer bool
		want     bool
	}{
		{"Renderer あり・メルカリ", siteOf(1, item.FetchHeadless, mercariTemplate), true, true},
		{"ホストは大文字小文字を区別しない", siteOf(2, item.FetchHeadless, "https://JP.Mercari.com/search?keyword={q}"), true, true},
		{"ポート付き", siteOf(3, item.FetchHeadless, "https://jp.mercari.com:443/search?keyword={q}"), true, true},
		{"Renderer なし(Chromium が無い環境)", siteOf(4, item.FetchHeadless, mercariTemplate), false, false},
		{"scrape の型ではホストが同じでも使わない", siteOf(5, item.FetchScrape, mercariTemplate), true, false},
		{"link_only は使わない", siteOf(6, item.FetchLinkOnly, mercariTemplate), true, false},
		{"headless でも未対応のホスト(ドラゴンスターは取得しない)", siteOf(7, item.FetchHeadless, "https://dorasuta.jp/dm/product-list?keyword={q}"), true, false},
		{"headless でホストが Yahoo!フリマ", siteOf(8, item.FetchHeadless, furimaTemplate), true, false},
		{"ホストが別物(前方一致では選ばない)", siteOf(9, item.FetchHeadless, "https://jp.mercari.com.evil.example/search?keyword={q}"), true, false},
		{"読めないテンプレート", siteOf(10, item.FetchHeadless, "::not a url::{q}"), true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := fetcher.Config{}
			if c.renderer {
				cfg.Renderer = &fakeRenderer{page: &fakePage{counts: []int{20}}}
			}
			f, ok := fetcher.NewRegistry(cfg).ForSite(c.site)
			if ok != c.want || (ok && f == nil) {
				t.Errorf("ForSite = %v, %v; want ok=%v", f, ok, c.want)
			}
		})
	}
	// For(fetch_type) は従来どおり headless を返さない(ホストが分からない)。
	if _, ok := fetcher.NewRegistry(fetcher.Config{Renderer: &fakeRenderer{}}).For(item.FetchHeadless); ok {
		t.Error("For(headless) が使える")
	}
}

// AC-H8: メルカリは夜間専用(Renderer の有無によらず印が付く)。ホストの間隔は既定の 5 秒(表に長い値を足さない)。
// 本番の表のメルカリは Throttle で包まれ、同じホストへ 5 秒あける。
func TestRegistry_MercariNightlyOnlyAndThrottle(t *testing.T) {
	s := siteOf(1, item.FetchHeadless, mercariTemplate)
	for _, withRenderer := range []bool{false, true} {
		cfg := fetcher.Config{}
		if withRenderer {
			cfg.Renderer = &fakeRenderer{}
		}
		r := fetcher.NewRegistry(cfg)
		if !r.NightlyOnly(s) {
			t.Errorf("renderer=%v: メルカリに NightlyOnly の印が無い", withRenderer)
		}
		if r.NightlyOnly(siteOf(2, item.FetchLinkOnly, amazonTemplate)) {
			t.Error("Amazon が NightlyOnly")
		}
	}
	if d := fetcher.HostMinIntervals["jp.mercari.com"]; d != 0 && d != fetcher.MinInterval {
		t.Errorf("メルカリのホスト間隔 = %v, want 既定(表に無いか %v)", d, fetcher.MinInterval)
	}

	clock := newFakeClock()
	p := &fakePage{counts: []int{20}, html: fixtureHTML(t)}
	r := fetcher.NewRegistry(fetcher.Config{Renderer: &fakeRenderer{page: p}, Clock: clock})
	f, ok := r.ForSite(s)
	if !ok {
		t.Fatal("ForSite が使えない")
	}
	for i := 0; i < 2; i++ {
		if _, err := f.Fetch(context.Background(), s, "x"); err != nil {
			t.Fatal(err)
		}
	}
	if got := clock.takeSleeps(); len(got) != 1 || got[0] != 5*time.Second {
		t.Errorf("待ち = %v, want [5s](1 回目は待たず、2 回目の前に 5 秒)", got)
	}
}
