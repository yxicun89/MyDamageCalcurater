package refresh_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/oapi-codegen/nullable"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/refresh"
)

const img = "11111111-1111-4111-8111-111111111111.png"

type call struct {
	siteID int64
	query  string
}

// fakeFetcher はサイトごとの結果を返す。呼び出しを記録する。block のサイトは解放されるまで返さない。
type fakeFetcher struct {
	mu      sync.Mutex
	calls   []call
	results map[int64][]fetcher.Listing
	errs    map[int64]error
	block   map[int64]chan struct{}
	entered chan int64
}

func newFakeFetcher() *fakeFetcher {
	return &fakeFetcher{results: map[int64][]fetcher.Listing{}, errs: map[int64]error{}, block: map[int64]chan struct{}{}, entered: make(chan int64, 100)}
}

func (f *fakeFetcher) Fetch(ctx context.Context, site fetcher.Site, q string) ([]fetcher.Listing, error) {
	f.mu.Lock()
	f.calls = append(f.calls, call{site.ID, q})
	res, err, ch := f.results[site.ID], f.errs[site.ID], f.block[site.ID]
	f.mu.Unlock()
	f.entered <- site.ID
	if ch != nil {
		<-ch
	}
	return slices.Clone(res), err
}

func (f *fakeFetcher) set(siteID int64, ls []fetcher.Listing, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results[siteID] = ls
	f.errs[siteID] = err
}

func (f *fakeFetcher) takeCalls() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.calls
	f.calls = nil
	for len(f.entered) > 0 {
		<-f.entered
	}
	return out
}

func (f *fakeFetcher) calledSites() []int64 {
	var out []int64
	for _, c := range f.takeCalls() {
		out = append(out, c.siteID)
	}
	return out
}

// fakeNow は差し替えられる時計。
type fakeNow struct {
	mu sync.Mutex
	t  time.Time
}

func (n *fakeNow) Now() time.Time {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.t
}

func (n *fakeNow) Set(t time.Time) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.t = t
}

var t0 = time.Date(2026, 10, 3, 3, 0, 0, 0, time.UTC)

type env struct {
	repo  *item.MemoryRepository
	f     *fakeFetcher
	now   *fakeNow
	svc   *refresh.Service
	genre item.Genre
	// サイト(ジャンルの表示順は merc, yahoo, amazon, shop, refHeadless)
	merc, yahoo, amazon, shop, refHeadless item.Site
	it                                     item.Item
	reg                                    *fetcher.Registry
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	repo := item.NewMemoryRepository()
	mk := func(name string, ft item.FetchType, ref bool) item.Site {
		s, err := repo.CreateSite(ctx, item.NewSite{Name: name, SearchURLTemplate: "https://" + name + ".example.com/s?q={q}", FetchType: ft, IsReference: ref})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	e := &env{repo: repo, f: newFakeFetcher(), now: &fakeNow{t: t0}}
	e.merc = mk("merc", item.FetchHeadless, false)
	e.yahoo = mk("yahoo", item.FetchAPI, true)
	e.amazon = mk("amazon", item.FetchLinkOnly, false)
	e.shop = mk("shop", item.FetchScrape, true)
	e.refHeadless = mk("refheadless", item.FetchHeadless, true)
	g, err := repo.CreateGenre(ctx, item.NewGenre{Name: "デュエマ", QueryTemplate: "{name} {option}", SiteIDs: []int64{e.merc.ID, e.yahoo.ID, e.amazon.ID, e.shop.ID, e.refHeadless.ID}})
	if err != nil {
		t.Fatal(err)
	}
	e.genre = g
	opt := "銀トレジャー"
	e.it, err = repo.CreateItem(ctx, item.NewItem{GenreID: g.ID, Name: "ボルシャック", OptionText: &opt, ImagePath: img})
	if err != nil {
		t.Fatal(err)
	}
	reg := fetcher.NewRegistryWith(map[item.FetchType]fetcher.Fetcher{item.FetchAPI: e.f, item.FetchScrape: e.f, item.FetchHeadless: e.f, item.FetchLinkOnly: e.f})
	e.reg = reg
	e.svc = refresh.New(refresh.Deps{Items: repo, Prices: repo, Fetchers: reg, Now: e.now.Now, BaseContext: context.Background()})
	t.Cleanup(e.svc.Wait)
	return e
}

func (e *env) refresh(t *testing.T, mode refresh.Mode) refresh.Report {
	t.Helper()
	r, err := e.svc.RefreshItem(context.Background(), e.it.ID, mode)
	if err != nil {
		t.Fatalf("RefreshItem: %v", err)
	}
	return r
}

func (e *env) estimates(t *testing.T) map[int64]item.Estimate {
	t.Helper()
	es, err := e.repo.ListEstimates(context.Background(), e.it.ID)
	if err != nil {
		t.Fatal(err)
	}
	out := map[int64]item.Estimate{}
	for _, x := range es {
		out[x.SiteID] = x
	}
	return out
}

func (e *env) listings(t *testing.T, siteID int64) []item.Listing {
	t.Helper()
	ls, err := e.repo.ListListings(context.Background(), e.it.ID, &siteID)
	if err != nil {
		t.Fatal(err)
	}
	return ls
}

func l(title string, price int) fetcher.Listing {
	return fetcher.Listing{Title: title, Price: price, URL: fmt.Sprintf("https://x.example.com/%d", price), InStock: true}
}

// waitEntered は取得が始まるのを待つ(未実装で始まらないときに止まらないよう 5 秒で失敗させる)。
func waitEntered(t *testing.T, f *fakeFetcher) int64 {
	t.Helper()
	select {
	case id := <-f.entered:
		return id
	case <-time.After(5 * time.Second):
		t.Fatal("取得が始まらない")
		return 0
	}
}

func ptr[T any](v T) *T { return &v }

func intOf(p *int) any {
	if p == nil {
		return "nil"
	}
	return *p
}

// AC-U1: 対象サイトはジャンルの表示順で、enabled=false と取得できない(link_only 等)サイトを除き、順番に 1 つずつ取る。
// 検索ワードは query.Build(サイト別 query > query_override > テンプレート)。
func TestRefreshItem_TargetsAndQuery(t *testing.T) {
	t.Run("テンプレート", func(t *testing.T) {
		e := newEnv(t)
		r := e.refresh(t, refresh.ModeAll)
		got := e.f.takeCalls()
		q := "ボルシャック 銀トレジャー"
		want := []call{{e.merc.ID, q}, {e.yahoo.ID, q}, {e.shop.ID, q}, {e.refHeadless.ID, q}}
		if !slices.Equal(got, want) {
			t.Errorf("取得 = %v, want %v", got, want)
		}
		if r.Targets != 4 || r.Fetched != 4 || r.Failed != 0 {
			t.Errorf("Report = %+v", r)
		}
		if _, ok := e.estimates(t)[e.amazon.ID]; ok {
			t.Error("link_only のサイトに目安が作られた")
		}
	})
	t.Run("override", func(t *testing.T) {
		e := newEnv(t)
		ctx := context.Background()
		_, err := e.repo.UpdateItem(ctx, e.it.ID, item.ItemPatch{
			QueryOverride: nullable.NewNullableWithValue("上書き ワード"),
			SiteOverrides: &[]item.SiteOverride{
				{SiteID: e.yahoo.ID, Query: ptr("サイト別 ワード"), Enabled: true},
				{SiteID: e.shop.ID, Enabled: false},
				{SiteID: e.refHeadless.ID, Query: ptr("  "), Enabled: true},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		r := e.refresh(t, refresh.ModeAll)
		got := e.f.takeCalls()
		want := []call{{e.merc.ID, "上書き ワード"}, {e.yahoo.ID, "サイト別 ワード"}, {e.refHeadless.ID, "上書き ワード"}}
		if !slices.Equal(got, want) {
			t.Errorf("取得 = %v, want %v", got, want)
		}
		if r.Targets != 3 {
			t.Errorf("Targets = %d, want 3", r.Targets)
		}
	})
}

// AC-U2: 仕様 §6 の例。すべて取ってから基準価格を決め、参考外も理由つきで保存する。目安は upsert、fetched_at は今。
func TestRefreshItem_SavesSpecExample(t *testing.T) {
	e := newEnv(t)
	e.f.set(e.yahoo.ID, []fetcher.Listing{l("ボルシャック・ドラゴン 銀トレジャー", 5000)}, nil)
	e.f.set(e.shop.ID, []fetcher.Listing{l("【銀トレジャー】ボルシャック", 4800)}, nil)
	e.f.set(e.merc.ID, []fetcher.Listing{l("まとめ売り カード", 300)}, nil)
	e.refresh(t, refresh.ModeAll)

	ml := e.listings(t, e.merc.ID)
	if len(ml) != 1 || !slices.Equal(ml[0].SuspiciousReasons, []string{"title_mismatch", "too_cheap"}) {
		t.Fatalf("メルカリの出品 = %+v, want 理由 [title_mismatch too_cheap]", ml)
	}
	if !ml[0].FetchedAt.Equal(t0) || ml[0].Title != "まとめ売り カード" || ml[0].URL != "https://x.example.com/300" {
		t.Errorf("メルカリの出品 = %+v", ml[0])
	}
	es := e.estimates(t)
	m := es[e.merc.ID]
	if m.Status != item.EstimateNoResult || m.Count != 0 || m.SuspiciousCount != 1 || m.Low != nil || !m.FetchedAt.Equal(t0) {
		t.Errorf("メルカリの目安 = %+v", m)
	}
	y := es[e.yahoo.ID]
	if y.Status != item.EstimateOK || intOf(y.Low) != 5000 || y.Mid != nil || y.Count != 1 || y.InStockCount != 1 {
		t.Errorf("Yahoo の目安 = %+v", y)
	}
	if rh := es[e.refHeadless.ID]; rh.Status != item.EstimateNoResult {
		t.Errorf("0 件のサイト = %+v, want no_result", rh)
	}
	if yl := e.listings(t, e.yahoo.ID); len(yl) != 1 || len(yl[0].SuspiciousReasons) != 0 {
		t.Errorf("Yahoo の出品 = %+v", yl)
	}
}

// AC-U3: 基準に使うのは is_reference かつ fetch_type が api / scrape のサイトだけ(is_reference でも headless は使わない)。
func TestRefreshItem_ReferenceSites(t *testing.T) {
	e := newEnv(t)
	e.f.set(e.yahoo.ID, []fetcher.Listing{l("ボルシャック", 5000)}, nil)
	e.f.set(e.refHeadless.ID, []fetcher.Listing{l("ボルシャック", 1000)}, nil)
	e.refresh(t, refresh.ModeAll)
	rl := e.listings(t, e.refHeadless.ID)
	if len(rl) != 1 || !slices.Equal(rl[0].SuspiciousReasons, []string{"too_cheap"}) {
		t.Errorf("headless の 1000 円 = %+v, want [too_cheap](基準は Yahoo の 5000 だけ)", rl)
	}
}

// AC-U4: 取得は先頭 20 件まで。保存できない値は合わせる(タイトルは 512 文字に切り詰め、URL が長すぎる出品は捨て、
// image_url が長すぎる・空なら nil)。
func TestRefreshItem_Sanitize(t *testing.T) {
	e := newEnv(t)
	var many []fetcher.Listing
	for i := range 25 {
		many = append(many, l(fmt.Sprintf("ボルシャック %02d", i), 1000+i))
	}
	e.f.set(e.yahoo.ID, many, nil)
	long := "ボルシャック" + strings.Repeat("あ", 600)
	e.f.set(e.shop.ID, []fetcher.Listing{
		{Title: long, Price: 100, URL: "https://x.example.com/a", ImageURL: "https://img.example.com/" + strings.Repeat("i", 1100), InStock: true},
		{Title: "ボルシャック", Price: 200, URL: "https://x.example.com/" + strings.Repeat("u", 1100), InStock: true},
		{Title: "ボルシャック", Price: 300, URL: "https://x.example.com/c", ImageURL: "", InStock: true},
		{Title: "ボルシャック", Price: 400, URL: "https://x.example.com/d", ImageURL: "https://img.example.com/d.jpg", InStock: true},
	}, nil)
	e.refresh(t, refresh.ModeAll)
	yl := e.listings(t, e.yahoo.ID)
	if len(yl) != fetcher.MaxListings || yl[0].Price != 1000 || yl[len(yl)-1].Price != 1019 {
		t.Errorf("Yahoo の出品 = %d 件, want 先頭 20 件", len(yl))
	}
	sl := e.listings(t, e.shop.ID)
	var got []int
	for _, x := range sl {
		got = append(got, x.Price)
	}
	if !slices.Equal(got, []int{100, 300, 400}) {
		t.Fatalf("shop の出品 = %v, want [100 300 400](URL が長すぎるものを捨てる)", got)
	}
	if n := utf8.RuneCountInString(sl[0].Title); n != item.MaxListingTitleLen || !strings.HasPrefix(sl[0].Title, "ボルシャック") {
		t.Errorf("タイトルの文字数 = %d, want %d", n, item.MaxListingTitleLen)
	}
	if sl[0].ImageURL != nil || sl[1].ImageURL != nil {
		t.Errorf("image_url = %v / %v, want nil", sl[0].ImageURL, sl[1].ImageURL)
	}
	if sl[2].ImageURL == nil || *sl[2].ImageURL != "https://img.example.com/d.jpg" {
		t.Errorf("image_url = %v", sl[2].ImageURL)
	}
}

// AC-U5: 取得の失敗は前回値(low・mid・件数・fetched_at と出品)を残し status だけ failed。他のサイトは保存する。
// 前回値が無ければ failed の行を作る。失敗はエラーにせず Report に数える。
func TestRefreshItem_Failure(t *testing.T) {
	e := newEnv(t)
	e.f.set(e.yahoo.ID, []fetcher.Listing{l("ボルシャック", 5000)}, nil)
	e.f.set(e.merc.ID, nil, errors.New("timeout"))
	r := e.refresh(t, refresh.ModeAll)
	if r.Fetched != 3 || r.Failed != 1 {
		t.Errorf("Report = %+v, want fetched 3・failed 1", r)
	}
	if m := e.estimates(t)[e.merc.ID]; m.Status != item.EstimateFailed || m.Low != nil || m.Count != 0 || !m.FetchedAt.Equal(t0) {
		t.Errorf("前回値なしの失敗 = %+v", m)
	}

	e.now.Set(t0.Add(25 * time.Hour))
	e.f.set(e.yahoo.ID, nil, errors.New("503"))
	e.f.set(e.shop.ID, []fetcher.Listing{l("ボルシャック", 4000)}, nil)
	r = e.refresh(t, refresh.ModeAll)
	if r.Failed != 2 {
		t.Errorf("Report = %+v, want failed 2", r)
	}
	es := e.estimates(t)
	y := es[e.yahoo.ID]
	if y.Status != item.EstimateFailed || intOf(y.Low) != 5000 || y.Count != 1 || !y.FetchedAt.Equal(t0) {
		t.Errorf("前回値ありの失敗 = %+v, want failed・low 5000・count 1・fetched_at %v", y, t0)
	}
	if yl := e.listings(t, e.yahoo.ID); len(yl) != 1 || yl[0].Price != 5000 {
		t.Errorf("失敗で出品が変わった: %+v", yl)
	}
	if s := es[e.shop.ID]; s.Status != item.EstimateOK || intOf(s.Low) != 4000 || !s.FetchedAt.Equal(t0.Add(25*time.Hour)) {
		t.Errorf("成功したサイト = %+v", s)
	}
}

// AC-U6: ModeStale は「目安が無い・failed・24 時間より古い」サイトのうち、最後に試してから 1 時間以上たったものだけ取る。
// ModeAll はすべて取る。
func TestRefreshItem_StaleMode(t *testing.T) {
	e := newEnv(t)
	all := []int64{e.merc.ID, e.yahoo.ID, e.shop.ID, e.refHeadless.ID}
	e.f.set(e.merc.ID, nil, errors.New("timeout"))

	e.refresh(t, refresh.ModeStale)
	if got := e.f.calledSites(); !slices.Equal(got, all) {
		t.Fatalf("初回(目安なし) = %v, want %v", got, all)
	}
	e.f.set(e.merc.ID, []fetcher.Listing{l("ボルシャック", 4000)}, nil)

	steps := []struct {
		after time.Duration
		want  []int64
	}{
		{30 * time.Minute, nil},                // failed でも試してから 1 時間たっていない
		{61 * time.Minute, []int64{e.merc.ID}}, // failed のサイトだけ取り直す
		{24 * time.Hour, nil},                  // ちょうど 24 時間は古くない(merc は 61 分に取得済み)
		{24*time.Hour + time.Second, []int64{e.yahoo.ID, e.shop.ID, e.refHeadless.ID}},
	}
	for _, s := range steps {
		e.now.Set(t0.Add(s.after))
		e.refresh(t, refresh.ModeStale)
		if got := e.f.calledSites(); !slices.Equal(got, s.want) {
			t.Errorf("t0+%v の取得 = %v, want %v", s.after, got, s.want)
		}
	}
	e.refresh(t, refresh.ModeAll)
	if got := e.f.calledSites(); !slices.Equal(got, all) {
		t.Errorf("ModeAll = %v, want %v", got, all)
	}
}

// AC-U7: 今回取得しなかった基準サイトは、保存済みの出品を基準価格の計算に使う(保存はし直さない)。
func TestRefreshItem_BaseFromStoredListings(t *testing.T) {
	e := newEnv(t)
	e.f.set(e.yahoo.ID, []fetcher.Listing{l("ボルシャック", 5000)}, nil)
	e.f.set(e.merc.ID, nil, errors.New("timeout"))
	e.refresh(t, refresh.ModeAll)
	e.f.takeCalls()

	e.now.Set(t0.Add(2 * time.Hour))
	e.f.set(e.merc.ID, []fetcher.Listing{l("ボルシャック", 300)}, nil)
	e.refresh(t, refresh.ModeStale)
	if got := e.f.calledSites(); !slices.Equal(got, []int64{e.merc.ID}) {
		t.Fatalf("取得 = %v, want メルカリだけ", got)
	}
	ml := e.listings(t, e.merc.ID)
	if len(ml) != 1 || !slices.Equal(ml[0].SuspiciousReasons, []string{"too_cheap"}) {
		t.Errorf("メルカリの 300 円 = %+v, want [too_cheap](保存済みの Yahoo 5000 円が基準)", ml)
	}
	if y := e.estimates(t)[e.yahoo.ID]; !y.FetchedAt.Equal(t0) {
		t.Errorf("取得しなかったサイトの目安が書き換わった: %+v", y)
	}
}

// AC-U8: 同じ商品の更新は同時に 1 つだけ(実行中に呼ばれたら取得を重ねない)。
func TestRefreshItem_Singleflight(t *testing.T) {
	e := newEnv(t)
	joined := make(chan int64, 1)
	e.svc = refresh.New(refresh.Deps{Items: e.repo, Prices: e.repo, Fetchers: e.reg, Now: e.now.Now, BaseContext: context.Background(),
		OnJoin: func(id int64) { joined <- id }})
	release := make(chan struct{})
	e.f.block[e.merc.ID] = release
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Go(func() {
		_, err := e.svc.RefreshItem(context.Background(), e.it.ID, refresh.ModeAll)
		errs <- err
	})
	if got := waitEntered(t, e.f); got != e.merc.ID {
		t.Fatalf("最初の取得 = %d", got)
	}
	wg.Go(func() {
		_, err := e.svc.RefreshItem(context.Background(), e.it.ID, refresh.ModeAll)
		errs <- err
	})
	select { // 2 つ目が実行中の更新に合流したことを待つ(時間に頼らない)
	case <-joined:
	case <-time.After(5 * time.Second):
		t.Fatal("2 つ目の呼び出しが合流しない")
	}
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("RefreshItem: %v", err)
		}
	}
	got := e.f.calledSites()
	if !slices.Equal(got, []int64{e.merc.ID, e.yahoo.ID, e.shop.ID, e.refHeadless.ID}) {
		t.Errorf("取得 = %v, want 各サイト 1 回ずつ", got)
	}
}

// AC-U9: 存在しない商品は item.ErrNotFound(RefreshItem・Estimates・Refresh・Listings)。
func TestNotFound(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	const missing = 99_999
	if _, err := e.svc.RefreshItem(ctx, missing, refresh.ModeAll); !errors.Is(err, item.ErrNotFound) {
		t.Errorf("RefreshItem: %v", err)
	}
	if _, err := e.svc.Estimates(ctx, missing); !errors.Is(err, item.ErrNotFound) {
		t.Errorf("Estimates: %v", err)
	}
	if _, err := e.svc.Refresh(ctx, missing); !errors.Is(err, item.ErrNotFound) {
		t.Errorf("Refresh: %v", err)
	}
	if _, err := e.svc.Listings(ctx, missing, nil); !errors.Is(err, item.ErrNotFound) {
		t.Errorf("Listings: %v", err)
	}
	if n := len(e.f.takeCalls()); n != 0 {
		t.Errorf("存在しない商品で取得した(%d)", n)
	}
}

func siteIDs(es []item.Estimate) []int64 {
	var out []int64
	for _, x := range es {
		out = append(out, x.SiteID)
	}
	return out
}

// AC-U10: Estimates はキャッシュを即返し、取るべきサイトがあれば裏で更新して refreshing:true。
// Sites はジャンルの表示順で、対象サイトのうち保存済みのものだけ。サマリはその Sites から作る。
func TestEstimates(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.f.set(e.yahoo.ID, []fetcher.Listing{l("ボルシャック", 5000), l("ボルシャック", 5200), l("ボルシャック", 5400)}, nil)
	e.f.set(e.shop.ID, []fetcher.Listing{l("ボルシャック", 4800)}, nil)

	v, err := e.svc.Estimates(ctx, e.it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Refreshing || len(v.Sites) != 0 || v.Summary.Low != nil {
		t.Errorf("初回 = %+v, want 空・refreshing", v)
	}
	e.svc.Wait()
	if got := e.f.calledSites(); len(got) != 4 {
		t.Errorf("裏の更新の取得 = %v", got)
	}

	e.now.Set(t0.Add(time.Hour))
	v, err = e.svc.Estimates(ctx, e.it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Refreshing {
		t.Error("新しいのに refreshing")
	}
	if got := siteIDs(v.Sites); !slices.Equal(got, []int64{e.merc.ID, e.yahoo.ID, e.shop.ID, e.refHeadless.ID}) {
		t.Errorf("Sites = %v, want ジャンルの表示順(link_only を除く)", got)
	}
	if intOf(v.Summary.Low) != 4800 || intOf(v.Summary.Mid) != 4800 || v.Summary.FetchedAt == nil || !v.Summary.FetchedAt.Equal(t0) {
		t.Errorf("Summary = low %v mid %v at %v", intOf(v.Summary.Low), intOf(v.Summary.Mid), v.Summary.FetchedAt)
	}
	e.svc.Wait()
	if n := len(e.f.takeCalls()); n != 0 {
		t.Errorf("新しいのに取得した(%d)", n)
	}

	// 対象から外したサイトは Sites に出さない
	if _, err := e.repo.UpdateItem(ctx, e.it.ID, item.ItemPatch{SiteOverrides: &[]item.SiteOverride{{SiteID: e.shop.ID, Enabled: false}}}); err != nil {
		t.Fatal(err)
	}
	v, _ = e.svc.Estimates(ctx, e.it.ID)
	if got := siteIDs(v.Sites); slices.Contains(got, e.shop.ID) {
		t.Errorf("enabled=false のサイトが出ている: %v", got)
	}
	if intOf(v.Summary.Low) != 5000 || intOf(v.Summary.Mid) != 5200 {
		t.Errorf("Summary = %v/%v, want 5000/5200(Yahoo)", intOf(v.Summary.Low), intOf(v.Summary.Mid))
	}

	// 24 時間より古ければ refreshing
	e.now.Set(t0.Add(25 * time.Hour))
	v, _ = e.svc.Estimates(ctx, e.it.ID)
	if !v.Refreshing {
		t.Error("古いのに refreshing でない")
	}
	e.svc.Wait()
}

// AC-U10: 失敗が続くサイトは、最後に試してから 1 時間は取り直さず refreshing:false。
func TestEstimates_FailureBackoff(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for _, s := range []int64{e.merc.ID, e.yahoo.ID, e.shop.ID, e.refHeadless.ID} {
		e.f.set(s, nil, errors.New("down"))
	}
	if v, _ := e.svc.Estimates(ctx, e.it.ID); !v.Refreshing {
		t.Fatal("初回が refreshing でない")
	}
	e.svc.Wait()
	e.f.takeCalls()
	e.now.Set(t0.Add(30 * time.Minute))
	v, err := e.svc.Estimates(ctx, e.it.ID)
	if err != nil {
		t.Fatal(err)
	}
	e.svc.Wait()
	if v.Refreshing || len(e.f.takeCalls()) != 0 {
		t.Errorf("1 時間以内に取り直した(refreshing %v)", v.Refreshing)
	}
	if len(v.Sites) != 4 || v.Sites[0].Status != item.EstimateFailed {
		t.Errorf("Sites = %+v, want failed の 4 件", v.Sites)
	}
	e.now.Set(t0.Add(61 * time.Minute))
	if v, _ := e.svc.Estimates(ctx, e.it.ID); !v.Refreshing {
		t.Error("1 時間後に refreshing でない")
	}
	e.svc.Wait()
}

// AC-U10・AC-U11: 取得できる対象サイトが無ければ、Estimates・Refresh とも refreshing:false で空。
func TestEstimates_NoTargets(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	g, err := e.repo.CreateGenre(ctx, item.NewGenre{Name: "リンクだけ", QueryTemplate: "{name}", SiteIDs: []int64{e.amazon.ID}})
	if err != nil {
		t.Fatal(err)
	}
	it, err := e.repo.CreateItem(ctx, item.NewItem{GenreID: g.ID, Name: "x", ImagePath: img})
	if err != nil {
		t.Fatal(err)
	}
	for name, fn := range map[string]func(context.Context, int64) (refresh.View, error){"Estimates": e.svc.Estimates, "Refresh": e.svc.Refresh} {
		v, err := fn(ctx, it.ID)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if v.Refreshing || len(v.Sites) != 0 || v.Summary.Low != nil {
			t.Errorf("%s = %+v", name, v)
		}
	}
	e.svc.Wait()
	if n := len(e.f.takeCalls()); n != 0 {
		t.Errorf("取得した(%d)", n)
	}
}

// AC-U11: Refresh は新しくても全対象を裏で取り直す。実行中なら新たに起動しない(refreshing は true)。
func TestRefresh(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.refresh(t, refresh.ModeAll)
	e.f.takeCalls()

	release := make(chan struct{})
	e.f.block[e.merc.ID] = release
	v, err := e.svc.Refresh(ctx, e.it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Refreshing || len(v.Sites) != 4 {
		t.Errorf("Refresh = %+v, want refreshing・キャッシュ 4 件", v)
	}
	waitEntered(t, e.f)
	v, err = e.svc.Refresh(ctx, e.it.ID)
	if err != nil || !v.Refreshing {
		t.Errorf("実行中の Refresh = %+v, %v", v, err)
	}
	close(release)
	e.svc.Wait()
	if got := e.f.calledSites(); !slices.Equal(got, []int64{e.merc.ID, e.yahoo.ID, e.shop.ID, e.refHeadless.ID}) {
		t.Errorf("取得 = %v, want 各サイト 1 回", got)
	}
}

// AC-U12: Listings は保存済みの出品(参考外を含む)。site_id で絞る。
func TestListings(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.f.set(e.yahoo.ID, []fetcher.Listing{l("ボルシャック", 5000)}, nil)
	e.f.set(e.merc.ID, []fetcher.Listing{l("まとめ売り", 300), l("ボルシャック", 4500)}, nil)
	e.refresh(t, refresh.ModeAll)
	all, err := e.svc.Listings(ctx, e.it.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Price != 300 || len(all[0].SuspiciousReasons) == 0 {
		t.Errorf("Listings = %+v", all)
	}
	only, err := e.svc.Listings(ctx, e.it.ID, &e.yahoo.ID)
	if err != nil || len(only) != 1 || only[0].SiteID != e.yahoo.ID {
		t.Errorf("site_id で絞り込み = %+v, %v", only, err)
	}
}

// AC-U13: RefreshAll は全商品を順に ModeAll で更新し、1 商品の失敗で止めない。
// 失敗した商品 = エラー、または対象サイトがあって 1 つも取得できなかった。対象サイトが無い商品は失敗に数えない。
func TestRefreshAll(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	// 2 つ目: 全サイト失敗するジャンル
	gFail, err := e.repo.CreateGenre(ctx, item.NewGenre{Name: "失敗", QueryTemplate: "{name}", SiteIDs: []int64{e.refHeadless.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.repo.CreateItem(ctx, item.NewItem{GenreID: gFail.ID, Name: "失敗する商品", ImagePath: img}); err != nil {
		t.Fatal(err)
	}
	e.f.set(e.refHeadless.ID, nil, errors.New("down"))
	// 3 つ目: 対象サイトなし
	gLink, err := e.repo.CreateGenre(ctx, item.NewGenre{Name: "リンクだけ", QueryTemplate: "{name}", SiteIDs: []int64{e.amazon.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.repo.CreateItem(ctx, item.NewItem{GenreID: gLink.ID, Name: "リンクだけの商品", ImagePath: img}); err != nil {
		t.Fatal(err)
	}
	// 1 つ目(ボルシャック): Yahoo に参考になる出品がある(0 件だと仕様どおり no_result になるため用意する)
	e.f.set(e.yahoo.ID, []fetcher.Listing{l("ボルシャック", 5000)}, nil)
	r, err := e.svc.RefreshAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r.Items != 3 || r.Failed != 1 {
		t.Errorf("AllReport = %+v, want items 3・failed 1", r)
	}
	if y := e.estimates(t)[e.yahoo.ID]; y.Status != item.EstimateOK {
		t.Errorf("1 つ目の商品が更新されていない: %+v", y)
	}
}

// failingGetRepo は指定した商品の GetItem だけエラーにする(リポジトリのエラーの注入)。
type failingGetRepo struct {
	*item.MemoryRepository
	failID int64
}

func (r failingGetRepo) GetItem(ctx context.Context, id int64) (item.Item, error) {
	if id == r.failID {
		return item.Item{}, errors.New("db down")
	}
	return r.MemoryRepository.GetItem(ctx, id)
}

// AC-U13: RefreshItem がエラーを返した商品も失敗に数え、止まらずに次の商品へ進む。
func TestRefreshAll_RepositoryError(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.f.set(e.yahoo.ID, []fetcher.Listing{l("ボルシャック", 5000)}, nil)
	second, err := e.repo.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "ボルシャック", ImagePath: img})
	if err != nil {
		t.Fatal(err)
	}
	svc := refresh.New(refresh.Deps{
		Items: failingGetRepo{MemoryRepository: e.repo, failID: e.it.ID}, Prices: e.repo, Fetchers: e.reg,
		Now: e.now.Now, BaseContext: ctx,
	})
	r, err := svc.RefreshAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r.Items != 2 || r.Failed != 1 {
		t.Errorf("AllReport = %+v, want items 2・failed 1", r)
	}
	if es, _ := e.repo.ListEstimates(ctx, second.ID); len(es) == 0 {
		t.Error("エラーの後の商品が更新されていない")
	}
}

// ModeStale の再試行間隔の境界: 最後に試してからちょうど 1 時間は取り直す(59 分 59 秒は取らない)。
func TestRefreshItem_RetryIntervalBoundary(t *testing.T) {
	e := newEnv(t)
	e.f.set(e.merc.ID, nil, errors.New("timeout"))
	e.refresh(t, refresh.ModeStale)
	e.f.takeCalls()
	e.f.set(e.merc.ID, nil, nil)
	e.now.Set(t0.Add(time.Hour - time.Second))
	e.refresh(t, refresh.ModeStale)
	if got := e.f.calledSites(); len(got) != 0 {
		t.Errorf("59 分 59 秒で取得した: %v", got)
	}
	e.now.Set(t0.Add(time.Hour))
	e.refresh(t, refresh.ModeStale)
	if got := e.f.calledSites(); !slices.Equal(got, []int64{e.merc.ID}) {
		t.Errorf("ちょうど 1 時間の取得 = %v, want [merc]", got)
	}
}

// 裏の更新は 02:30 以上 04:00 未満 JST には起動しない(CronJob との重なり対策)。境界を確かめる。
func TestBackgroundQuietWindow(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	cases := []struct {
		at    time.Time
		quiet bool
	}{
		{time.Date(2026, 10, 3, 2, 29, 59, 0, jst), false},
		{time.Date(2026, 10, 3, 2, 30, 0, 0, jst), true},
		{time.Date(2026, 10, 3, 3, 0, 0, 0, jst), true},
		{time.Date(2026, 10, 3, 3, 59, 59, 0, jst), true},
		{time.Date(2026, 10, 3, 4, 0, 0, 0, jst), false},
		{time.Date(2026, 10, 2, 17, 30, 0, 0, time.UTC), true}, // = 02:30 JST(UTC で渡しても同じ)
	}
	for _, c := range cases {
		if got := refresh.InQuietWindow(c.at); got != c.quiet {
			t.Errorf("InQuietWindow(%v) = %v, want %v", c.at, got, c.quiet)
		}
		for name, call := range map[string]func(*env) (refresh.View, error){
			"Estimates": func(e *env) (refresh.View, error) { return e.svc.Estimates(context.Background(), e.it.ID) },
			"Refresh":   func(e *env) (refresh.View, error) { return e.svc.Refresh(context.Background(), e.it.ID) },
		} {
			e := newEnv(t)
			e.now.Set(c.at)
			v, err := call(e)
			if err != nil {
				t.Fatal(err)
			}
			e.svc.Wait()
			n := len(e.f.takeCalls())
			if v.Refreshing == c.quiet || (n > 0) == c.quiet {
				t.Errorf("%s at %v: refreshing=%v 取得=%d, quiet=%v", name, c.at, v.Refreshing, n, c.quiet)
			}
		}
	}
	// 同期の RefreshItem(CronJob)は時間帯に関係なく動く
	e := newEnv(t)
	e.now.Set(time.Date(2026, 10, 3, 3, 0, 0, 0, jst))
	if r := e.refresh(t, refresh.ModeAll); r.Fetched == 0 {
		t.Errorf("CronJob の時間帯に更新できない: %+v", r)
	}
}
