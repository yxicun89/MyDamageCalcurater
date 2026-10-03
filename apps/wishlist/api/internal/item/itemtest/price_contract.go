package itemtest

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// FullRepository は item.Repository と item.PriceRepository の両方(メモリ実装・MySQL 実装)。
type FullRepository interface {
	item.Repository
	item.PriceRepository
}

func eqIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func eqStrPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameEstimate(a, b item.Estimate) bool {
	return a.ItemID == b.ItemID && a.SiteID == b.SiteID && eqIntPtr(a.Low, b.Low) && eqIntPtr(a.Mid, b.Mid) &&
		a.Count == b.Count && a.SuspiciousCount == b.SuspiciousCount && a.InStockCount == b.InStockCount &&
		a.Status == b.Status && a.FetchedAt.Equal(b.FetchedAt)
}

func prices(ls []item.Listing) []int {
	out := make([]int, 0, len(ls))
	for _, l := range ls {
		out = append(out, l.Price)
	}
	return out
}

// RunPriceRepositoryContract は PriceRepository の契約テストを流す(docs/phase3-api-spec.md AC-P*)。
func RunPriceRepositoryContract(t *testing.T, newRepo func(t *testing.T) FullRepository) {
	ctx := context.Background()
	jst := time.FixedZone("JST", 9*60*60)
	// 秒未満を持つ時刻(保存すると切り捨てる)。JST で渡しても同じ時点として返ること。
	t1 := time.Date(2026, 10, 3, 12, 0, 0, 700_000_000, jst)
	t1s := t1.Truncate(time.Second)
	t2 := t1.Add(26 * time.Hour)

	type pfix struct {
		fixture
		repo FullRepository
		it   item.Item
	}
	psetup := func(t *testing.T) pfix {
		t.Helper()
		var r FullRepository
		f := setup(t, func(t *testing.T) item.Repository {
			r = newRepo(t)
			return r
		})
		return pfix{fixture: f, repo: r, it: f.newItem(t, f.genreA.ID, "グリス", 0)}
	}
	estimateFor := func(itemID, siteID int64) item.Estimate {
		return item.Estimate{ItemID: itemID, SiteID: siteID, Low: intp(4800), Mid: intp(5000), Count: 3, SuspiciousCount: 1, InStockCount: 2, Status: item.EstimateOK, FetchedAt: t1}
	}
	sampleListings := func() []item.Listing {
		return []item.Listing{
			// ID・ItemID・SiteID・FetchedAt は無視される
			{ID: 999, ItemID: 999, SiteID: 999, Title: "仮面ライダーグリス", Price: 5000, URL: "https://a.example.com/1", ImageURL: strp("https://img.example.com/1.jpg"), InStock: true, FetchedAt: t2},
			{Title: "まとめ売り", Price: 300, URL: "https://a.example.com/2", InStock: true, SuspiciousReasons: []string{"title_mismatch", "too_cheap"}},
			{Title: "グリス 在庫なし", Price: 5000, URL: "https://a.example.com/3", InStock: false},
			{Title: "グリス 新品", Price: 4800, URL: "https://a.example.com/4", InStock: true, SuspiciousReasons: []string{}},
		}
	}

	// AC-P1: 保存した目安と出品を読める。出品は price 昇順・同額は id(保存順)昇順。時刻は秒未満を切り捨て。
	t.Run("SaveAndList", func(t *testing.T) {
		f := psetup(t)
		e := estimateFor(f.it.ID, f.site1.ID)
		if err := f.repo.SaveSiteResult(ctx, e, sampleListings()); err != nil {
			t.Fatal(err)
		}
		es, err := f.repo.ListEstimates(ctx, f.it.ID)
		if err != nil {
			t.Fatal(err)
		}
		want := e
		want.FetchedAt = t1s
		if len(es) != 1 || !sameEstimate(es[0], want) {
			t.Fatalf("estimates = %+v, want [%+v]", es, want)
		}
		ls, err := f.repo.ListListings(ctx, f.it.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(prices(ls), []int{300, 4800, 5000, 5000}) {
			t.Fatalf("price の並び = %v", prices(ls))
		}
		if ls[2].URL != "https://a.example.com/1" || ls[3].URL != "https://a.example.com/3" {
			t.Errorf("同額は保存順(id 昇順)であること: %s, %s", ls[2].URL, ls[3].URL)
		}
		for _, l := range ls {
			if l.ID == 0 || l.ID == 999 || l.ItemID != f.it.ID || l.SiteID != f.site1.ID || !l.FetchedAt.Equal(t1s) {
				t.Errorf("出品の ID・ItemID・SiteID・FetchedAt = %d/%d/%d/%v", l.ID, l.ItemID, l.SiteID, l.FetchedAt)
			}
		}
		first := ls[2]
		if first.Title != "仮面ライダーグリス" || !eqStrPtr(first.ImageURL, strp("https://img.example.com/1.jpg")) || !first.InStock || len(first.SuspiciousReasons) != 0 {
			t.Errorf("出品 = %+v", first)
		}
		if !slices.Equal(ls[0].SuspiciousReasons, []string{"title_mismatch", "too_cheap"}) || ls[0].ImageURL != nil {
			t.Errorf("参考外の出品 = %+v", ls[0])
		}
		if ls[3].InStock {
			t.Errorf("在庫なしが在庫ありになった: %+v", ls[3])
		}
	})

	// AC-P2: 同じ商品×サイトの保存は、出品を置き換え・目安を上書きする。他のサイトの分は変えない。site_id で絞り込める。
	t.Run("ReplacePerSite", func(t *testing.T) {
		f := psetup(t)
		if err := f.repo.SaveSiteResult(ctx, estimateFor(f.it.ID, f.site1.ID), sampleListings()); err != nil {
			t.Fatal(err)
		}
		if err := f.repo.SaveSiteResult(ctx, estimateFor(f.it.ID, f.site2.ID), sampleListings()[:2]); err != nil {
			t.Fatal(err)
		}
		e2 := item.Estimate{ItemID: f.it.ID, SiteID: f.site1.ID, Low: intp(7000), Count: 1, InStockCount: 1, Status: item.EstimateOK, FetchedAt: t2}
		if err := f.repo.SaveSiteResult(ctx, e2, []item.Listing{{Title: "グリス", Price: 7000, URL: "https://a.example.com/9", InStock: true}}); err != nil {
			t.Fatal(err)
		}
		s1, err := f.repo.ListListings(ctx, f.it.ID, &f.site1.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(prices(s1), []int{7000}) {
			t.Errorf("サイト1 の出品 = %v, want [7000](置き換え)", prices(s1))
		}
		s2, err := f.repo.ListListings(ctx, f.it.ID, &f.site2.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(prices(s2), []int{300, 5000}) {
			t.Errorf("サイト2 の出品 = %v, want [300 5000](変わらない)", prices(s2))
		}
		all, _ := f.repo.ListListings(ctx, f.it.ID, nil)
		if !slices.Equal(prices(all), []int{300, 5000, 7000}) {
			t.Errorf("全サイトの出品 = %v", prices(all))
		}
		es, err := f.repo.ListEstimates(ctx, f.it.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(es) != 2 || es[0].SiteID != f.site1.ID || es[1].SiteID != f.site2.ID {
			t.Fatalf("estimates = %+v, want site_id 昇順の 2 件", es)
		}
		want := e2
		want.FetchedAt = t2.Truncate(time.Second)
		if !sameEstimate(es[0], want) {
			t.Errorf("上書き後 = %+v, want %+v", es[0], want)
		}
	})

	// AC-P3: 0 件の保存は出品を空にし、目安は no_result(low・mid は null)。
	t.Run("SaveEmpty", func(t *testing.T) {
		f := psetup(t)
		if err := f.repo.SaveSiteResult(ctx, estimateFor(f.it.ID, f.site1.ID), sampleListings()); err != nil {
			t.Fatal(err)
		}
		e := item.Estimate{ItemID: f.it.ID, SiteID: f.site1.ID, Status: item.EstimateNoResult, FetchedAt: t2}
		if err := f.repo.SaveSiteResult(ctx, e, nil); err != nil {
			t.Fatal(err)
		}
		ls, _ := f.repo.ListListings(ctx, f.it.ID, nil)
		if len(ls) != 0 {
			t.Errorf("出品が残った: %v", prices(ls))
		}
		es, _ := f.repo.ListEstimates(ctx, f.it.ID)
		if len(es) != 1 || es[0].Status != item.EstimateNoResult || es[0].Low != nil || es[0].Mid != nil || es[0].Count != 0 {
			t.Errorf("estimates = %+v", es)
		}
	})

	// AC-P4: MarkFailed は status だけ failed にし、前回の low・mid・件数・fetched_at と出品を残す。
	// 行が無ければ failed・null・0・fetched_at = at の行を作る。
	t.Run("MarkFailed", func(t *testing.T) {
		f := psetup(t)
		e := estimateFor(f.it.ID, f.site1.ID)
		if err := f.repo.SaveSiteResult(ctx, e, sampleListings()); err != nil {
			t.Fatal(err)
		}
		if err := f.repo.MarkFailed(ctx, f.it.ID, f.site1.ID, t2); err != nil {
			t.Fatal(err)
		}
		if err := f.repo.MarkFailed(ctx, f.it.ID, f.site2.ID, t2); err != nil {
			t.Fatal(err)
		}
		es, err := f.repo.ListEstimates(ctx, f.it.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(es) != 2 {
			t.Fatalf("estimates = %+v", es)
		}
		want1 := e
		want1.Status = item.EstimateFailed
		want1.FetchedAt = t1s
		if !sameEstimate(es[0], want1) {
			t.Errorf("前回値あり = %+v, want %+v", es[0], want1)
		}
		want2 := item.Estimate{ItemID: f.it.ID, SiteID: f.site2.ID, Status: item.EstimateFailed, FetchedAt: t2.Truncate(time.Second)}
		if !sameEstimate(es[1], want2) {
			t.Errorf("前回値なし = %+v, want %+v", es[1], want2)
		}
		ls, _ := f.repo.ListListings(ctx, f.it.ID, &f.site1.ID)
		if len(ls) != 4 {
			t.Errorf("失敗で出品が変わった: %v", prices(ls))
		}
	})

	// AC-P5: 存在しない商品 → ErrNotFound、存在しないサイト → ErrSiteNotFound。列幅を超える出品・負の価格 → ErrInvalid。
	// どれも何も変えない。読み出しは存在しない商品でも空でエラーにしない。
	t.Run("Errors", func(t *testing.T) {
		f := psetup(t)
		if err := f.repo.SaveSiteResult(ctx, estimateFor(f.it.ID, f.site1.ID), sampleListings()); err != nil {
			t.Fatal(err)
		}
		const missing = 99_999_999
		long := strings.Repeat("あ", item.MaxListingTitleLen+1)
		longURL := "https://a.example.com/" + strings.Repeat("a", item.MaxListingURLLen)
		cases := []struct {
			name string
			call func() error
			want error
		}{
			{"Save 商品なし", func() error {
				return f.repo.SaveSiteResult(ctx, estimateFor(missing, f.site1.ID), sampleListings())
			}, item.ErrNotFound},
			{"Save サイトなし", func() error {
				return f.repo.SaveSiteResult(ctx, estimateFor(f.it.ID, missing), sampleListings())
			}, item.ErrSiteNotFound},
			{"Save タイトルが長すぎる", func() error {
				return f.repo.SaveSiteResult(ctx, estimateFor(f.it.ID, f.site1.ID), []item.Listing{{Title: long, Price: 1, URL: "https://a.example.com/x"}})
			}, item.ErrInvalid},
			{"Save URL が長すぎる", func() error {
				return f.repo.SaveSiteResult(ctx, estimateFor(f.it.ID, f.site1.ID), []item.Listing{{Title: "x", Price: 1, URL: longURL}})
			}, item.ErrInvalid},
			{"Save 負の価格", func() error {
				return f.repo.SaveSiteResult(ctx, estimateFor(f.it.ID, f.site1.ID), []item.Listing{{Title: "x", Price: -1, URL: "https://a.example.com/x"}})
			}, item.ErrInvalid},
			{"MarkFailed 商品なし", func() error { return f.repo.MarkFailed(ctx, missing, f.site1.ID, t2) }, item.ErrNotFound},
			{"MarkFailed サイトなし", func() error { return f.repo.MarkFailed(ctx, f.it.ID, missing, t2) }, item.ErrSiteNotFound},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				if err := c.call(); !errors.Is(err, c.want) {
					t.Errorf("err = %v, want %v", err, c.want)
				}
				ls, err := f.repo.ListListings(ctx, f.it.ID, nil)
				if err != nil || len(ls) != 4 {
					t.Errorf("失敗で出品が変わった: %v %v", prices(ls), err)
				}
				es, err := f.repo.ListEstimates(ctx, f.it.ID)
				if err != nil || len(es) != 1 || !sameEstimate(es[0], func() item.Estimate { e := estimateFor(f.it.ID, f.site1.ID); e.FetchedAt = t1s; return e }()) {
					t.Errorf("失敗で目安が変わった: %+v %v", es, err)
				}
			})
		}
		es, err := f.repo.ListEstimates(ctx, missing)
		if err != nil || len(es) != 0 {
			t.Errorf("存在しない商品の estimates = %v, %v", es, err)
		}
		ls, err := f.repo.ListListings(ctx, missing, nil)
		if err != nil || len(ls) != 0 {
			t.Errorf("存在しない商品の listings = %v, %v", ls, err)
		}
	})

	// AC-P6: 商品を消すと目安と出品も消える(CASCADE)。
	t.Run("DeleteItemCascades", func(t *testing.T) {
		f := psetup(t)
		if err := f.repo.SaveSiteResult(ctx, estimateFor(f.it.ID, f.site1.ID), sampleListings()); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repo.DeleteItem(ctx, f.it.ID); err != nil {
			t.Fatal(err)
		}
		es, err := f.repo.ListEstimates(ctx, f.it.ID)
		if err != nil || len(es) != 0 {
			t.Errorf("estimates = %v, %v", es, err)
		}
		ls, err := f.repo.ListListings(ctx, f.it.ID, nil)
		if err != nil || len(ls) != 0 {
			t.Errorf("listings = %v, %v", ls, err)
		}
	})

	runPriceHistoryContract(t, newRepo)
}
