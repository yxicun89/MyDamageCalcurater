package itemtest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// runGenreStyleContract はジャンルの色・絵柄・URL の手がかりの契約(フェーズ5 5-1b・5-1c。docs/phase5-spec.md)。
// Repository は Service が検査・補完した値をそのまま保存して返す(色は小文字の #rrggbb、手がかりは渡した順)。
func runGenreStyleContract(t *testing.T, newRepo func(t *testing.T) item.Repository) {
	ctx := context.Background()

	// AC-5-1b-1・5-1c-5: 作成・一覧・更新で color・icon・match_hosts・match_keywords が往復する。
	t.Run("GenreStyleRoundTrip", func(t *testing.T) {
		f := setup(t, newRepo)
		in := item.NewGenre{
			Name: "契約テスト 色", QueryTemplate: "{name}", SortOrder: 1001, SiteIDs: []int64{f.site1.ID},
			Color: "#e0393e", Icon: item.IconCard,
			MatchHosts: []string{"dm.takaratomy.co.jp", "www.example.com"}, MatchKeywords: []string{"デュエル・マスターズ", "デュエマ"},
		}
		g, err := f.repo.CreateGenre(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		check := func(what string, got item.Genre, color string, icon item.GenreIcon, hosts, kws []string) {
			t.Helper()
			if got.Color != color || got.Icon != icon || fmt.Sprintf("%q", got.MatchHosts) != fmt.Sprintf("%q", hosts) || fmt.Sprintf("%q", got.MatchKeywords) != fmt.Sprintf("%q", kws) {
				t.Errorf("%s = color %q icon %q hosts %q keywords %q, want %q %q %q %q", what, got.Color, got.Icon, got.MatchHosts, got.MatchKeywords, color, icon, hosts, kws)
			}
		}
		check("作成", g, "#e0393e", item.IconCard, in.MatchHosts, in.MatchKeywords)
		check("一覧", genreByID(t, f.repo, g.ID), "#e0393e", item.IconCard, in.MatchHosts, in.MatchKeywords)

		color, icon := "#7c4dff", item.IconFigure
		kws := []string{"S.H.Figuarts"}
		u, err := f.repo.UpdateGenre(ctx, g.ID, item.GenrePatch{Color: &color, Icon: &icon, MatchKeywords: &kws})
		if err != nil {
			t.Fatal(err)
		}
		// match_hosts は省略したので変えない
		check("更新", u, "#7c4dff", item.IconFigure, in.MatchHosts, kws)
		check("更新後の一覧", genreByID(t, f.repo, g.ID), "#7c4dff", item.IconFigure, in.MatchHosts, kws)

		empty := []string{}
		name := "契約テスト 色2"
		u, err = f.repo.UpdateGenre(ctx, g.ID, item.GenrePatch{Name: &name, MatchHosts: &empty, MatchKeywords: &empty})
		if err != nil {
			t.Fatal(err)
		}
		check("空で置き換え", u, "#7c4dff", item.IconFigure, []string{}, []string{})
		if u.MatchHosts == nil || u.MatchKeywords == nil {
			t.Errorf("手がかりが nil(長さ 0 のスライスで返す): %#v %#v", u.MatchHosts, u.MatchKeywords)
		}
	})

	// AC-5-1b-1: 手がかりを渡さずに作ったジャンルは長さ 0(nil でない)で返す。
	t.Run("GenreStyleEmptyHints", func(t *testing.T) {
		f := setup(t, newRepo)
		g, err := f.repo.CreateGenre(ctx, item.NewGenre{Name: "契約テスト 手がかりなし", QueryTemplate: "{name}", Color: "#1d5fd1", Icon: item.IconBox})
		if err != nil {
			t.Fatal(err)
		}
		got := genreByID(t, f.repo, g.ID)
		if got.MatchHosts == nil || got.MatchKeywords == nil || len(got.MatchHosts) != 0 || len(got.MatchKeywords) != 0 {
			t.Errorf("手がかり = %#v / %#v, want 長さ 0", got.MatchHosts, got.MatchKeywords)
		}
		if got.Color != "#1d5fd1" || got.Icon != item.IconBox {
			t.Errorf("色・絵柄 = %q / %q", got.Color, got.Icon)
		}
	})
}

// runListSummaryContract は ListItems が埋める目安のまとめの契約(フェーズ5 5-1d。docs/phase5-spec.md)。
// 対象は「商品のジャンルの site_ids にあり、site_overrides で外していないサイト」の保存済みの目安(item.SummarizeItem と同じ規則)。
func runListSummaryContract(t *testing.T, newRepo func(t *testing.T) FullRepository) {
	ctx := context.Background()
	t1 := time.Date(2026, 10, 10, 3, 0, 0, 0, time.UTC)
	t2 := t1.Add(2 * time.Hour)

	// AC-5-1d-1・2: ListItems は全商品に Summary を付ける(ok・no_result・failed・none)。ジャンルで絞っても同じ値。
	t.Run("ListItemsSummary", func(t *testing.T) {
		var r FullRepository
		f := setup(t, func(t *testing.T) item.Repository {
			r = newRepo(t)
			return r
		})
		both := []int64{f.site1.ID, f.site2.ID}
		one := []int64{f.site1.ID}
		if _, err := r.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{SiteIDs: &both}); err != nil {
			t.Fatal(err)
		}
		if _, err := r.UpdateGenre(ctx, f.genreB.ID, item.GenrePatch{SiteIDs: &one}); err != nil {
			t.Fatal(err)
		}
		okItem := f.newItem(t, f.genreA.ID, "契約テスト ok", 0)
		noResult := f.newItem(t, f.genreA.ID, "契約テスト no_result", 0)
		failed := f.newItem(t, f.genreB.ID, "契約テスト failed", 0)
		none := f.newItem(t, f.genreA.ID, "契約テスト none", 0)
		// noResult はサイト2を外す(外したサイトの安い目安は使わない)
		if _, err := r.UpdateItem(ctx, noResult.ID, item.ItemPatch{SiteOverrides: &[]item.SiteOverride{{SiteID: f.site2.ID, Enabled: false}}}); err != nil {
			t.Fatal(err)
		}
		save := func(itemID, siteID int64, low *int, st item.EstimateStatus, at time.Time) {
			t.Helper()
			if err := r.SaveSiteResult(ctx, item.Estimate{ItemID: itemID, SiteID: siteID, Low: low, Status: st, FetchedAt: at}, nil); err != nil {
				t.Fatal(err)
			}
		}
		save(okItem.ID, f.site1.ID, intp(5200), item.EstimateOK, t2)
		save(okItem.ID, f.site2.ID, intp(4800), item.EstimateOK, t1)
		save(noResult.ID, f.site1.ID, nil, item.EstimateNoResult, t1)
		save(noResult.ID, f.site2.ID, intp(100), item.EstimateOK, t1)
		save(failed.ID, f.site1.ID, nil, item.EstimateNoResult, t1)
		if err := r.MarkFailed(ctx, failed.ID, f.site1.ID, t2); err != nil {
			t.Fatal(err)
		}
		// failed のジャンル B に無いサイト2の目安は使わない
		save(failed.ID, f.site2.ID, intp(100), item.EstimateOK, t1)

		type want struct {
			low    *int
			status item.SummaryStatus
			at     *time.Time
		}
		wants := map[int64]want{
			okItem.ID:   {intp(4800), item.SummaryOK, &t1},
			noResult.ID: {nil, item.SummaryNoResult, nil},
			failed.ID:   {nil, item.SummaryFailed, nil},
			none.ID:     {nil, item.SummaryNone, nil},
		}
		checkList := func(what string, list []item.Item) {
			t.Helper()
			seen := 0
			for _, it := range list {
				w, ok := wants[it.ID]
				if !ok {
					continue
				}
				seen++
				if it.Summary == nil {
					t.Errorf("%s: %s の Summary が nil", what, it.Name)
					continue
				}
				s := it.Summary
				if s.Status != w.status || !eqIntPtr(s.Low, w.low) || (s.FetchedAt == nil) != (w.at == nil) || (s.FetchedAt != nil && !s.FetchedAt.Equal(*w.at)) {
					t.Errorf("%s: %s の Summary = {%v %q %v}, want {%v %q %v}", what, it.Name, s.Low, s.Status, s.FetchedAt, w.low, w.status, w.at)
				}
			}
			if what == "全件" && seen != len(wants) {
				t.Errorf("%s: 一覧に %d 件しか無い", what, seen)
			}
		}
		all, err := r.ListItems(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		checkList("全件", all)
		onlyA, err := r.ListItems(ctx, &f.genreA.ID)
		if err != nil {
			t.Fatal(err)
		}
		checkList("ジャンル A", onlyA)
	})
}
