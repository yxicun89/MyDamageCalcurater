package refresh_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/refresh"
)

type hostRewrite struct{ target string }

func (rt hostRewrite) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = strings.TrimPrefix(rt.target, "http://")
	return http.DefaultTransport.RoundTrip(r2)
}

type noWaitClock struct{}

func (noWaitClock) Now() time.Time                                   { return t0 }
func (noWaitClock) Sleep(ctx context.Context, _ time.Duration) error { return ctx.Err() }

// AC-S13: 更新の対象サイトは Registry.ForSite で決める。本番の表では、対応済みホストの scrape(カードラッシュ)だけが
// 取得され、未対応ホストの scrape・headless・link_only は取得されない(目安の行も作られない)。
func TestRefreshItem_UsesForSite(t *testing.T) {
	body, err := os.ReadFile("../fetcher/testdata/cardrush.html")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var hosts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hosts = append(hosts, r.Host)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.Write(body)
	}))
	defer srv.Close()

	ctx := context.Background()
	repo := item.NewMemoryRepository()
	mk := func(name string, ft item.FetchType, tmpl string) item.Site {
		// 基準サイトにしない: このテストは ForSite の振り分けを見る。基準サイトにすると fixture の 50 円が
		// 仕様どおり too_cheap(基準 665 円×0.3 未満)になり、件数の期待と食い違う
		s, err := repo.CreateSite(ctx, item.NewSite{Name: name, SearchURLTemplate: tmpl, FetchType: ft, IsReference: false})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	card := mk("カードラッシュ", item.FetchScrape, "https://www.cardrush-dm.jp/product-list?keyword={q}&order=asc&available=1&num=20")
	other := mk("未対応", item.FetchScrape, "https://shop.example.com/s?q={q}")
	merc := mk("メルカリ", item.FetchHeadless, "https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc")
	amazon := mk("Amazon", item.FetchLinkOnly, "https://www.amazon.co.jp/s?k={q}&s=price-asc-rank")
	g, err := repo.CreateGenre(ctx, item.NewGenre{Name: "デュエマ", QueryTemplate: "{name} {option}", SiteIDs: []int64{card.ID, other.ID, merc.ID, amazon.ID}})
	if err != nil {
		t.Fatal(err)
	}
	it, err := repo.CreateItem(ctx, item.NewItem{GenreID: g.ID, Name: "テスト", ImagePath: img})
	if err != nil {
		t.Fatal(err)
	}
	reg := fetcher.NewRegistry(fetcher.Config{
		Client: &http.Client{Transport: hostRewrite{target: srv.URL}, Timeout: 10 * time.Second},
		Clock:  noWaitClock{},
	})
	svc := refresh.New(refresh.Deps{Items: repo, Prices: repo, Fetchers: reg, Now: func() time.Time { return t0 }, BaseContext: ctx})
	t.Cleanup(svc.Wait)

	r, err := svc.RefreshItem(ctx, it.ID, refresh.ModeAll)
	if err != nil {
		t.Fatal(err)
	}
	if r.Targets != 1 || r.Fetched != 1 || r.Failed != 0 {
		t.Errorf("Report = %+v, want Targets=1 Fetched=1 Failed=0", r)
	}
	mu.Lock()
	got := slices.Clone(hosts)
	mu.Unlock()
	if !slices.Equal(got, []string{"www.cardrush-dm.jp"}) {
		t.Errorf("取得先 = %v, want [www.cardrush-dm.jp]", got)
	}
	es, err := repo.ListEstimates(ctx, it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 1 || es[0].SiteID != card.ID || es[0].Count != 2 || es[0].Status != "ok" {
		t.Errorf("estimates = %+v, want カードラッシュの 1 行(count 2・ok)", es)
	}
}
