package refresh_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
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

// 駿河屋は夜間の CronJob(RefreshAll = ModeNightly)だけで取る。ModeAll(手動)・ModeStale(裏の更新)では取らずに飛ばし、
// 前回値は残す。裏の更新を起動もしない(Refreshing は false)。
func TestRefresh_NightlyOnlySite(t *testing.T) {
	body, err := os.ReadFile("../fetcher/testdata/surugaya.html")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.Write(body)
	}))
	defer srv.Close()
	count := func() int { mu.Lock(); defer mu.Unlock(); return calls }

	ctx := context.Background()
	repo := item.NewMemoryRepository()
	st, err := repo.CreateSite(ctx, item.NewSite{Name: "駿河屋", SearchURLTemplate: "https://www.suruga-ya.jp/search?category=&search_word={q}&rankBy=price%3Aascending&inStock=On", FetchType: item.FetchScrape})
	if err != nil {
		t.Fatal(err)
	}
	g, err := repo.CreateGenre(ctx, item.NewGenre{Name: "G", QueryTemplate: "{name}", SiteIDs: []int64{st.ID}})
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

	for _, mode := range []refresh.Mode{refresh.ModeAll, refresh.ModeStale} {
		r, err := svc.RefreshItem(ctx, it.ID, mode)
		if err != nil {
			t.Fatal(err)
		}
		if r.Fetched != 0 || r.Failed != 0 || count() != 0 {
			t.Fatalf("mode %v: Report = %+v, calls = %d; 夜間専用は取らない", mode, r, count())
		}
	}
	v, err := svc.Estimates(ctx, it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Refreshing {
		t.Error("夜間専用のサイトだけなのに裏の更新を起動した")
	}
	if v, err := svc.Refresh(ctx, it.ID); err != nil || v.Refreshing || count() != 0 {
		t.Errorf("手動の更新で夜間専用を取った: %+v, %v, calls=%d", v, err, count())
	}
	svc.Wait()

	rep, err := svc.RefreshAll(ctx)
	if err != nil || rep.Failed != 0 {
		t.Fatalf("RefreshAll = %+v, %v", rep, err)
	}
	if count() != 1 {
		t.Fatalf("夜間の更新の取得 = %d 回, want 1", count())
	}
	es, _ := repo.ListEstimates(ctx, it.ID)
	if len(es) != 1 || es[0].Status != "ok" {
		t.Fatalf("estimates = %+v", es)
	}

	// 夜間以外の更新は前回値を残す。
	if _, err := svc.RefreshItem(ctx, it.ID, refresh.ModeAll); err != nil {
		t.Fatal(err)
	}
	es2, _ := repo.ListEstimates(ctx, it.ID)
	if count() != 1 || len(es2) != 1 || !reflect.DeepEqual(es2, es) {
		t.Errorf("前回値が変わった: %+v → %+v (calls=%d)", es, es2, count())
	}
}
