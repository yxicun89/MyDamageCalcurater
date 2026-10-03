package itemtest

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// フェーズ4-2 価格の推移の契約テスト(docs/phase4-spec.md AC-H2〜H6)。RunPriceRepositoryContract の末尾から呼ぶ。

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// pointKey は比較用の文字列(site_id・日付・low・mid・count・recorded_at)。
func pointKey(p item.PricePoint) string {
	mid := "nil"
	if p.Mid != nil {
		mid = fmt.Sprint(*p.Mid)
	}
	return fmt.Sprintf("item%d site%d %s low=%d mid=%s count=%d at=%s", p.ItemID, p.SiteID, p.Day.UTC().Format("2006-01-02"), p.Low, mid, p.Count, p.RecordedAt.UTC().Format(time.RFC3339))
}

func pointKeys(ps []item.PricePoint) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, pointKey(p))
	}
	return out
}

func sameKeys(a, b []string) bool { return fmt.Sprintf("%q", a) == fmt.Sprintf("%q", b) }

func runPriceHistoryContract(t *testing.T, newRepo func(t *testing.T) FullRepository) {
	ctx := context.Background()
	jst := time.FixedZone("JST", 9*60*60)
	// JST 2026-10-03 12:00(秒未満あり)。
	t1 := time.Date(2026, 10, 3, 12, 0, 0, 700_000_000, jst)
	t1s := t1.Truncate(time.Second)
	since := day(2026, 1, 1)

	type hfix struct {
		fixture
		repo FullRepository
		it   item.Item
	}
	hsetup := func(t *testing.T) hfix {
		t.Helper()
		var r FullRepository
		f := setup(t, func(t *testing.T) item.Repository {
			r = newRepo(t)
			return r
		})
		return hfix{fixture: f, repo: r, it: f.newItem(t, f.genreA.ID, "グリス", 0)}
	}
	ok := func(itemID, siteID int64, low int, mid *int, count int, at time.Time) item.Estimate {
		return item.Estimate{ItemID: itemID, SiteID: siteID, Low: &low, Mid: mid, Count: count, InStockCount: count, Status: item.EstimateOK, FetchedAt: at}
	}
	save := func(t *testing.T, r FullRepository, e item.Estimate) {
		t.Helper()
		if err := r.SaveSiteResult(ctx, e, nil); err != nil {
			t.Fatalf("SaveSiteResult(%+v): %v", e, err)
		}
	}
	list := func(t *testing.T, r FullRepository, itemID int64, since time.Time) []string {
		t.Helper()
		ps, err := r.ListPriceHistory(ctx, itemID, since)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range ps {
			if p.Day.Location() != time.UTC || p.Day.Hour() != 0 || p.Day.Minute() != 0 || p.Day.Second() != 0 || p.Day.Nanosecond() != 0 {
				t.Errorf("Day は 00:00 UTC で返す: %v", p.Day)
			}
		}
		return pointKeys(ps)
	}
	key := func(itemID, siteID int64, d time.Time, low int, mid *int, count int, at time.Time) string {
		return pointKey(item.PricePoint{ItemID: itemID, SiteID: siteID, Day: d, Low: low, Mid: mid, Count: count, RecordedAt: at.Truncate(time.Second)})
	}

	// AC-H2: ok の保存は (商品, サイト, JST の日) の 1 行を upsert する。同じ日は最後の値で上書き(mid が null になる値も)、
	// 別の日は行が増える。日付は JST で決める(UTC 15:00 は JST の翌日)。recorded_at は保存した fetched_at(秒未満を切り捨て)。
	t.Run("HistorySavedOnOK", func(t *testing.T) {
		f := hsetup(t)
		save(t, f.repo, ok(f.it.ID, f.site1.ID, 4800, intp(5000), 3, t1))
		want1 := key(f.it.ID, f.site1.ID, day(2026, 10, 3), 4800, intp(5000), 3, t1s)
		if got := list(t, f.repo, f.it.ID, since); !sameKeys(got, []string{want1}) {
			t.Fatalf("1 回目 = %q, want [%q]", got, want1)
		}

		later := t1.Add(3 * time.Hour) // 同じ JST の日(15:00)
		save(t, f.repo, ok(f.it.ID, f.site1.ID, 4500, nil, 2, later))
		want2 := key(f.it.ID, f.site1.ID, day(2026, 10, 3), 4500, nil, 2, later)
		if got := list(t, f.repo, f.it.ID, since); !sameKeys(got, []string{want2}) {
			t.Fatalf("同じ日の 2 回目 = %q, want [%q](最後の値で上書き)", got, want2)
		}

		boundary := time.Date(2026, 10, 3, 15, 0, 0, 0, time.UTC) // JST 10/4 0:00
		save(t, f.repo, ok(f.it.ID, f.site1.ID, 4700, intp(4900), 4, boundary))
		want3 := key(f.it.ID, f.site1.ID, day(2026, 10, 4), 4700, intp(4900), 4, boundary)
		if got := list(t, f.repo, f.it.ID, since); !sameKeys(got, []string{want2, want3}) {
			t.Errorf("翌日(JST) = %q, want [%q %q]", got, want2, want3)
		}
	})

	// AC-H3: 推移を作るのは ok で low があるときだけ。no_result・MarkFailed・low の無い ok・検査で弾いた保存は行を作らず、
	// 同じ日の既存の行も変えない・消さない(値を推測しない)。
	t.Run("HistoryOnlyForOK", func(t *testing.T) {
		f := hsetup(t)
		noResult := item.Estimate{ItemID: f.it.ID, SiteID: f.site1.ID, Status: item.EstimateNoResult, FetchedAt: t1}
		if err := f.repo.SaveSiteResult(ctx, noResult, nil); err != nil {
			t.Fatal(err)
		}
		if err := f.repo.MarkFailed(ctx, f.it.ID, f.site2.ID, t1); err != nil {
			t.Fatal(err)
		}
		if got := list(t, f.repo, f.it.ID, since); len(got) != 0 {
			t.Fatalf("no_result・failed で推移ができた: %q", got)
		}
		noLow := item.Estimate{ItemID: f.it.ID, SiteID: f.site2.ID, Count: 0, Status: item.EstimateOK, FetchedAt: t1}
		if err := f.repo.SaveSiteResult(ctx, noLow, nil); err != nil {
			t.Fatal(err)
		}
		if got := list(t, f.repo, f.it.ID, since); len(got) != 0 {
			t.Fatalf("low の無い ok で推移ができた: %q", got)
		}

		save(t, f.repo, ok(f.it.ID, f.site1.ID, 3000, intp(4500), 5, t1))
		want := []string{key(f.it.ID, f.site1.ID, day(2026, 10, 3), 3000, intp(4500), 5, t1s)}
		later := t1.Add(time.Hour)
		if err := f.repo.SaveSiteResult(ctx, item.Estimate{ItemID: f.it.ID, SiteID: f.site1.ID, Status: item.EstimateNoResult, FetchedAt: later}, nil); err != nil {
			t.Fatal(err)
		}
		if err := f.repo.MarkFailed(ctx, f.it.ID, f.site1.ID, later.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		bad := ok(f.it.ID, f.site1.ID, 1, nil, 1, later)
		if err := f.repo.SaveSiteResult(ctx, bad, []item.Listing{{Title: "x", Price: -1, URL: "https://a.example.com/x"}}); !errors.Is(err, item.ErrInvalid) {
			t.Fatalf("負の価格の保存 = %v, want ErrInvalid", err)
		}
		if err := f.repo.SaveSiteResult(ctx, ok(99_999_999, f.site1.ID, 1, nil, 1, later), nil); !errors.Is(err, item.ErrNotFound) {
			t.Fatalf("商品なしの保存 = %v, want ErrNotFound", err)
		}
		if got := list(t, f.repo, f.it.ID, since); !sameKeys(got, want) {
			t.Errorf("同じ日の no_result・failed・弾いた保存のあと = %q, want %q(変えない)", got, want)
		}
	})

	// AC-H4: ListPriceHistory は Day >= since の行を site_id 昇順・day 昇順で返す。他の商品の行は返さない。
	// 存在しない商品は空でエラーにしない。
	t.Run("HistoryListOrderAndSince", func(t *testing.T) {
		f := hsetup(t)
		other := f.newItem(t, f.genreA.ID, "別の商品", 1)
		at := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 12, 0, 0, 0, jst) }
		// 保存の順はばらばらにする。
		save(t, f.repo, ok(f.it.ID, f.site2.ID, 3200, nil, 2, at(10, 2)))
		save(t, f.repo, ok(f.it.ID, f.site1.ID, 3000, intp(3500), 4, at(10, 3)))
		save(t, f.repo, ok(f.it.ID, f.site1.ID, 3100, intp(3600), 4, at(9, 30)))
		save(t, f.repo, ok(f.it.ID, f.site2.ID, 3300, intp(3400), 3, at(10, 1)))
		save(t, f.repo, ok(f.it.ID, f.site1.ID, 2900, nil, 1, at(10, 1)))
		save(t, f.repo, ok(other.ID, f.site1.ID, 9999, nil, 1, at(10, 1)))

		want := []string{
			key(f.it.ID, f.site1.ID, day(2026, 10, 1), 2900, nil, 1, at(10, 1)),
			key(f.it.ID, f.site1.ID, day(2026, 10, 3), 3000, intp(3500), 4, at(10, 3)),
			key(f.it.ID, f.site2.ID, day(2026, 10, 1), 3300, intp(3400), 3, at(10, 1)),
			key(f.it.ID, f.site2.ID, day(2026, 10, 2), 3200, nil, 2, at(10, 2)),
		}
		if got := list(t, f.repo, f.it.ID, day(2026, 10, 1)); !sameKeys(got, want) {
			t.Errorf("since 10/1 = %q\nwant %q(9/30 は含まない・10/1 は含む)", got, want)
		}
		all := list(t, f.repo, f.it.ID, since)
		if len(all) != 5 || all[0] != key(f.it.ID, f.site1.ID, day(2026, 9, 30), 3100, intp(3600), 4, at(9, 30)) {
			t.Errorf("since 1/1 = %q, want 5 行で先頭は site1 の 9/30", all)
		}
		if got := list(t, f.repo, 99_999_999, since); len(got) != 0 {
			t.Errorf("存在しない商品 = %q, want 空", got)
		}
	})

	// AC-H5: 商品を消すと推移も消える(CASCADE)。他の商品の推移は残る。
	t.Run("HistoryDeleteItemCascades", func(t *testing.T) {
		f := hsetup(t)
		other := f.newItem(t, f.genreA.ID, "別の商品", 1)
		save(t, f.repo, ok(f.it.ID, f.site1.ID, 3000, nil, 1, t1))
		save(t, f.repo, ok(other.ID, f.site1.ID, 4000, nil, 1, t1))
		if _, err := f.repo.DeleteItem(ctx, f.it.ID); err != nil {
			t.Fatal(err)
		}
		if got := list(t, f.repo, f.it.ID, since); len(got) != 0 {
			t.Errorf("消した商品の推移 = %q", got)
		}
		if got := list(t, f.repo, other.ID, since); len(got) != 1 {
			t.Errorf("別の商品の推移 = %q, want 1 行(残る)", got)
		}
	})

	// AC-H6: PrunePriceHistory(before) は全商品の Day < before の行を消して件数を返す。Day == before の行は残す。
	t.Run("HistoryPrune", func(t *testing.T) {
		f := hsetup(t)
		other := f.newItem(t, f.genreA.ID, "別の商品", 1)
		at := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 12, 0, 0, 0, jst) }
		save(t, f.repo, ok(f.it.ID, f.site1.ID, 3000, nil, 1, at(4, 6)))
		save(t, f.repo, ok(f.it.ID, f.site1.ID, 3100, nil, 1, at(4, 7)))
		save(t, f.repo, ok(f.it.ID, f.site2.ID, 3200, nil, 1, at(4, 8)))
		save(t, f.repo, ok(other.ID, f.site1.ID, 3300, nil, 1, at(4, 5)))
		save(t, f.repo, ok(other.ID, f.site2.ID, 3400, nil, 1, at(10, 3)))

		n, err := f.repo.PrunePriceHistory(ctx, day(2026, 4, 7))
		if err != nil {
			t.Fatal(err)
		}
		if n != 2 {
			t.Errorf("消した行数 = %d, want 2(4/6 と別の商品の 4/5)", n)
		}
		want := []string{
			key(f.it.ID, f.site1.ID, day(2026, 4, 7), 3100, nil, 1, at(4, 7)),
			key(f.it.ID, f.site2.ID, day(2026, 4, 8), 3200, nil, 1, at(4, 8)),
		}
		if got := list(t, f.repo, f.it.ID, since); !sameKeys(got, want) {
			t.Errorf("残り = %q, want %q", got, want)
		}
		if got := list(t, f.repo, other.ID, since); !sameKeys(got, []string{key(other.ID, f.site2.ID, day(2026, 10, 3), 3400, nil, 1, at(10, 3))}) {
			t.Errorf("別の商品の残り = %q", got)
		}
		n, err = f.repo.PrunePriceHistory(ctx, day(2026, 4, 7))
		if err != nil || n != 0 {
			t.Errorf("2 回目 = %d, %v, want 0", n, err)
		}
	})
}
