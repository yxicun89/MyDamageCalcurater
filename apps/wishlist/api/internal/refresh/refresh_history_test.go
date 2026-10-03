package refresh_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/refresh"
)

// フェーズ4-2 価格の推移(docs/phase4-spec.md AC-H7〜H10)。t0 は JST 2026-10-03 12:00。

func hday(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func (e *env) history(t *testing.T, itemID int64) []string {
	t.Helper()
	ps, err := e.repo.ListPriceHistory(context.Background(), itemID, hday(2000, 1, 1))
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, fmt.Sprintf("site%d %s low=%v mid=%v", p.SiteID, p.Day.Format("01-02"), p.Low, intOf(p.Mid)))
	}
	return out
}

// seedHistory は ok の目安を at の時点で保存する(推移の行ができる)。
func (e *env) seedHistory(t *testing.T, itemID, siteID int64, low int, mid *int, at time.Time) {
	t.Helper()
	err := e.repo.SaveSiteResult(context.Background(), item.Estimate{
		ItemID: itemID, SiteID: siteID, Low: &low, Mid: mid, Count: 1, InStockCount: 1, Status: item.EstimateOK, FetchedAt: at,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func sameStrings(a, b []string) bool { return fmt.Sprintf("%q", a) == fmt.Sprintf("%q", b) }

// AC-H7a: 更新(RefreshItem)は、取得が ok のサイトだけ推移に残す(no_result・失敗・取らなかったサイトは残さない)。
// 同じ日にもう一度更新すると上書き、翌日(JST)の更新は行が増える。
func TestRefreshItem_RecordsPriceHistory(t *testing.T) {
	e := newEnv(t)
	e.f.set(e.yahoo.ID, []fetcher.Listing{l("ボルシャック 銀トレジャー", 5000)}, nil)
	e.f.set(e.shop.ID, []fetcher.Listing{l("ボルシャック", 4800)}, nil)
	e.f.set(e.merc.ID, nil, nil)                        // 0 件 → no_result
	e.f.set(e.refHeadless.ID, nil, errors.New("取得に失敗")) // failed
	e.refresh(t, refresh.ModeAll)
	want := []string{
		fmt.Sprintf("site%d 10-03 low=5000 mid=nil", e.yahoo.ID),
		fmt.Sprintf("site%d 10-03 low=4800 mid=nil", e.shop.ID),
	}
	if got := e.history(t, e.it.ID); !sameStrings(got, want) {
		t.Fatalf("推移 = %q, want %q(ok のサイトだけ)", got, want)
	}

	e.now.Set(t0.Add(2 * time.Hour)) // 同じ JST の日
	e.f.set(e.yahoo.ID, []fetcher.Listing{l("ボルシャック 銀トレジャー", 4600)}, nil)
	e.f.set(e.shop.ID, nil, errors.New("取得に失敗")) // 失敗しても前の値は残る
	e.refresh(t, refresh.ModeAll)
	want = []string{
		fmt.Sprintf("site%d 10-03 low=4600 mid=nil", e.yahoo.ID),
		fmt.Sprintf("site%d 10-03 low=4800 mid=nil", e.shop.ID),
	}
	if got := e.history(t, e.it.ID); !sameStrings(got, want) {
		t.Fatalf("同じ日の 2 回目 = %q, want %q(ok は上書き・失敗は前の値のまま)", got, want)
	}

	e.now.Set(t0.Add(13 * time.Hour)) // JST 10/4 1:00
	e.f.set(e.shop.ID, []fetcher.Listing{l("ボルシャック", 4700)}, nil)
	e.refresh(t, refresh.ModeAll)
	want = []string{
		fmt.Sprintf("site%d 10-03 low=4600 mid=nil", e.yahoo.ID),
		fmt.Sprintf("site%d 10-04 low=4600 mid=nil", e.yahoo.ID),
		fmt.Sprintf("site%d 10-03 low=4800 mid=nil", e.shop.ID),
		fmt.Sprintf("site%d 10-04 low=4700 mid=nil", e.shop.ID),
	}
	if got := e.history(t, e.it.ID); !sameStrings(got, want) {
		t.Errorf("翌日 = %q, want %q", got, want)
	}
}

// AC-H7: RefreshAll(CronJob)は最初に、保持期間(今日を含む 180 日)より古い日の推移を全商品から消す。
// 今日(JST 10/3)から 179 日前(4/7)は残し、180 日前(4/6)は消す。時計は Deps.Now。
func TestRefreshAll_PrunesOldPriceHistory(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	other, err := e.repo.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "別の商品", ImagePath: img})
	if err != nil {
		t.Fatal(err)
	}
	jst := time.FixedZone("JST", 9*60*60)
	at := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 12, 0, 0, 0, jst) }
	e.seedHistory(t, e.it.ID, e.shop.ID, 3000, nil, at(4, 6))
	e.seedHistory(t, e.it.ID, e.shop.ID, 3100, nil, at(4, 7))
	e.seedHistory(t, other.ID, e.shop.ID, 3200, nil, at(4, 6))
	// 今回の取得はすべて 0 件(no_result)にして、新しい推移を作らない。
	if _, err := e.svc.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	if got, want := e.history(t, e.it.ID), []string{fmt.Sprintf("site%d 04-07 low=3100 mid=nil", e.shop.ID)}; !sameStrings(got, want) {
		t.Errorf("推移 = %q, want %q(180 日前は消し、179 日前は残す)", got, want)
	}
	if got := e.history(t, other.ID); len(got) != 0 {
		t.Errorf("別の商品の推移 = %q, want 空(全商品から消す)", got)
	}
}

// AC-H8: PriceHistory は直近 days 日(今日〈JST〉を含む)の推移を返す。Sites はジャンルの表示順(ジャンルに無いサイトはその後に
// site_id 昇順)、点は day 昇順。Overall は日ごとの全サイトの low の最小(点のある日だけ)。期間より前の日は含めない。
func TestPriceHistory_View(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	extra, err := e.repo.CreateSite(ctx, item.NewSite{Name: "extra", SearchURLTemplate: "https://extra.example.com/s?q={q}", FetchType: item.FetchScrape})
	if err != nil {
		t.Fatal(err)
	}
	jst := time.FixedZone("JST", 9*60*60)
	at := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 12, 0, 0, 0, jst) }
	// ジャンルの表示順は merc, yahoo, amazon, shop, refHeadless。extra はジャンルに無い。
	e.seedHistory(t, e.it.ID, extra.ID, 2500, nil, at(10, 2))
	e.seedHistory(t, e.it.ID, e.shop.ID, 3200, ptr(3600), at(10, 3))
	e.seedHistory(t, e.it.ID, e.shop.ID, 3000, ptr(3400), at(10, 1))
	e.seedHistory(t, e.it.ID, e.merc.ID, 2800, nil, at(10, 1))
	e.seedHistory(t, e.it.ID, e.merc.ID, 3500, nil, at(10, 3))
	e.seedHistory(t, e.it.ID, e.merc.ID, 1000, nil, at(7, 5)) // 90 日前(期間外)
	e.seedHistory(t, e.it.ID, e.merc.ID, 2000, nil, at(7, 6)) // 89 日前(期間内の最初の日)

	h, err := e.svc.PriceHistory(ctx, e.it.ID, refresh.DefaultHistoryDays)
	if err != nil {
		t.Fatal(err)
	}
	if h.Days != 90 {
		t.Errorf("Days = %d, want 90", h.Days)
	}
	got := []string{}
	for _, s := range h.Sites {
		for _, p := range s.Points {
			got = append(got, fmt.Sprintf("site%d %s low=%d mid=%v", s.SiteID, p.Day.Format("01-02"), p.Low, intOf(p.Mid)))
		}
	}
	want := []string{
		fmt.Sprintf("site%d 07-06 low=2000 mid=nil", e.merc.ID),
		fmt.Sprintf("site%d 10-01 low=2800 mid=nil", e.merc.ID),
		fmt.Sprintf("site%d 10-03 low=3500 mid=nil", e.merc.ID),
		fmt.Sprintf("site%d 10-01 low=3000 mid=3400", e.shop.ID),
		fmt.Sprintf("site%d 10-03 low=3200 mid=3600", e.shop.ID),
		fmt.Sprintf("site%d 10-02 low=2500 mid=nil", extra.ID),
	}
	if !sameStrings(got, want) {
		t.Errorf("Sites = %q\nwant %q", got, want)
	}
	overall := []string{}
	for _, d := range h.Overall {
		overall = append(overall, fmt.Sprintf("%s %d", d.Day.Format("01-02"), d.Low))
	}
	if want := []string{"07-06 2000", "10-01 2800", "10-02 2500", "10-03 3200"}; !sameStrings(overall, want) {
		t.Errorf("Overall = %q, want %q", overall, want)
	}

	// days=1 は今日だけ。
	h, err = e.svc.PriceHistory(ctx, e.it.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if h.Days != 1 || len(h.Sites) != 2 || len(h.Overall) != 1 || !h.Overall[0].Day.Equal(hday(2026, 10, 3)) {
		t.Errorf("days=1 = %+v, want 今日(10/3)の 2 サイトだけ", h)
	}
}

// AC-H8: 推移が無い商品は Sites・Overall とも長さ 0(nil でなく空。API で [] になる)。
func TestPriceHistory_Empty(t *testing.T) {
	e := newEnv(t)
	h, err := e.svc.PriceHistory(context.Background(), e.it.ID, refresh.MaxHistoryDays)
	if err != nil {
		t.Fatal(err)
	}
	if h.Days != 180 || h.Sites == nil || h.Overall == nil || len(h.Sites) != 0 || len(h.Overall) != 0 {
		t.Errorf("推移なし = %+v, want Days 180・空のスライス", h)
	}
}

// AC-H10: 商品が無ければ ErrNotFound、days が 1〜180 の外なら ErrInvalid。
func TestPriceHistory_Errors(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.svc.PriceHistory(ctx, 99_999_999, 90); !errors.Is(err, item.ErrNotFound) {
		t.Errorf("商品なし = %v, want ErrNotFound", err)
	}
	for _, days := range []int{0, -1, 181} {
		if _, err := e.svc.PriceHistory(ctx, e.it.ID, days); !errors.Is(err, item.ErrInvalid) {
			t.Errorf("days=%d = %v, want ErrInvalid", days, err)
		}
	}
}

// AC-H9: BuildHistory(純関数)。
func TestBuildHistory(t *testing.T) {
	p := func(site int64, d int, low int, mid *int) item.PricePoint {
		return item.PricePoint{ItemID: 1, SiteID: site, Day: hday(2026, 10, d), Low: low, Mid: mid, Count: 1}
	}
	render := func(h refresh.History) string {
		s := "sites:"
		for _, x := range h.Sites {
			s += fmt.Sprintf(" [%d:", x.SiteID)
			for _, q := range x.Points {
				s += fmt.Sprintf(" %d=%d/%v", q.Day.Day(), q.Low, intOf(q.Mid))
			}
			s += "]"
		}
		s += " overall:"
		for _, d := range h.Overall {
			s += fmt.Sprintf(" %d=%d", d.Day.Day(), d.Low)
		}
		return s
	}
	cases := []struct {
		name   string
		points []item.PricePoint
		order  []int64
		want   string
	}{
		{"空", nil, []int64{1, 2}, "sites: overall:"},
		{"1 サイト", []item.PricePoint{p(1, 1, 3000, ptr(3500)), p(1, 2, 2900, nil)}, []int64{1}, "sites: [1: 1=3000/3500 2=2900/nil] overall: 1=3000 2=2900"},
		{
			"表示順・ジャンル外は後ろに site_id 昇順",
			[]item.PricePoint{p(2, 1, 3000, nil), p(5, 1, 2000, nil), p(3, 1, 2500, nil), p(9, 2, 1000, nil)},
			[]int64{3, 1, 2},
			"sites: [3: 1=2500/nil] [2: 1=3000/nil] [5: 1=2000/nil] [9: 2=1000/nil] overall: 1=2000 2=1000",
		},
		{
			"日ごとの最小・同額",
			[]item.PricePoint{p(1, 1, 3000, nil), p(1, 3, 3300, nil), p(2, 1, 3000, nil), p(2, 2, 3100, nil), p(2, 3, 3200, nil)},
			[]int64{1, 2},
			"sites: [1: 1=3000/nil 3=3300/nil] [2: 1=3000/nil 2=3100/nil 3=3200/nil] overall: 1=3000 2=3100 3=3200",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := refresh.BuildHistory(c.points, c.order)
			if got := render(h); got != c.want {
				t.Errorf("BuildHistory = %s\nwant          %s", got, c.want)
			}
			if h.Sites == nil || h.Overall == nil {
				t.Error("Sites・Overall は nil でなく空のスライスで返す")
			}
		})
	}
}
