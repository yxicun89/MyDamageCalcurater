package refresh_test

import (
	"context"
	"slices"
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/refresh"
)

// AC-A9(docs/phase4-spec.md): 更新は、判定のときに商品のジャンルの表記揺れの辞書を使う(更新のたびにその時点の辞書を読む)。
func TestRefreshItem_UsesGenreAliases(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	g, err := e.repo.CreateGenre(ctx, item.NewGenre{
		Name: "ガンプラ", QueryTemplate: "{name}", SiteIDs: []int64{e.shop.ID, e.merc.ID},
		Aliases: [][]string{{"HG", "ハイグレード"}, {"MG", "マスターグレード"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	it, err := e.repo.CreateItem(ctx, item.NewItem{GenreID: g.ID, Name: "HG エアリアル", ImagePath: img})
	if err != nil {
		t.Fatal(err)
	}
	e.f.set(e.shop.ID, []fetcher.Listing{l("ハイグレード ガンダムエアリアル", 2000), l("HG 1/144 ガンダムエアリアル", 2200)}, nil)
	e.f.set(e.merc.ID, []fetcher.Listing{l("ハイグレード エアリアル 未組立", 1800), l("エアリアル キーホルダー", 300)}, nil)

	reasons := func(siteID int64) map[int][]string {
		t.Helper()
		ls, err := e.repo.ListListings(ctx, it.ID, &siteID)
		if err != nil {
			t.Fatal(err)
		}
		out := map[int][]string{}
		for _, x := range ls {
			out[x.Price] = x.SuspiciousReasons
		}
		return out
	}

	if _, err := e.svc.RefreshItem(ctx, it.ID, refresh.ModeAll); err != nil {
		t.Fatal(err)
	}
	shop := reasons(e.shop.ID)
	if len(shop) != 2 || len(shop[2000]) != 0 || len(shop[2200]) != 0 {
		t.Errorf("ショップの理由 = %v, want どちらも参考(ハイグレード は HG の別名)", shop)
	}
	merc := reasons(e.merc.ID)
	if len(merc[1800]) != 0 || !slices.Equal(merc[300], []string{"title_mismatch", "too_cheap"}) {
		t.Errorf("メルカリの理由 = %v, want 1800 は参考・300 は [title_mismatch too_cheap](基準 2100)", merc)
	}
	es, err := e.repo.ListEstimates(ctx, it.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range es {
		if x.SiteID == e.shop.ID && (x.Count != 2 || x.SuspiciousCount != 0) {
			t.Errorf("ショップの目安 = %+v, want count 2・suspicious 0", x)
		}
	}

	// 辞書を消すと、次の更新では別名のタイトルが title_mismatch になる。
	if _, err := e.repo.UpdateGenre(ctx, g.ID, item.GenrePatch{Aliases: &[][]string{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.RefreshItem(ctx, it.ID, refresh.ModeAll); err != nil {
		t.Fatal(err)
	}
	shop = reasons(e.shop.ID)
	if !slices.Contains(shop[2000], "title_mismatch") || len(shop[2200]) != 0 {
		t.Errorf("辞書を消した後のショップの理由 = %v, want 2000 は title_mismatch・2200 は参考", shop)
	}
}
