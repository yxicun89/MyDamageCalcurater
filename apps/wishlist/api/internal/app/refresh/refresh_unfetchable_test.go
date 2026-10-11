package refresh_test

import (
	"context"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/adapters/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/adapters/memory"
	"example.com/pokecalc/apps/wishlist/api/internal/app/refresh"
	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
)

// このプロセスに Fetcher が無いサイト(Renderer の無い api のメルカリ)でも、夜間の CronJob が保存した目安は
// 詳細(Estimates)に出て、まとめにも入る。ただし取りには行かず、そのサイトだけのために裏の更新も起動しない。
func TestEstimates_SiteWithoutFetcherStillShown(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewMemoryRepository()
	st, err := repo.CreateSite(ctx, item.NewSite{Name: "メルカリ", SearchURLTemplate: "https://jp.mercari.com/search?keyword={q}", FetchType: item.FetchHeadless})
	if err != nil {
		t.Fatal(err)
	}
	g, err := repo.CreateGenre(ctx, item.NewGenre{Name: "G", QueryTemplate: "{name}", SiteIDs: []int64{st.ID}})
	if err != nil {
		t.Fatal(err)
	}
	it, err := repo.CreateItem(ctx, item.NewItem{GenreID: g.ID, Name: "架空", ImagePath: img})
	if err != nil {
		t.Fatal(err)
	}
	low := 1200
	// 古い(MaxAge 超え)目安。取れないサイトなので、古くても裏の更新は起動しない。
	if err := repo.SaveSiteResult(ctx, item.Estimate{ItemID: it.ID, SiteID: st.ID, Low: &low, Status: item.EstimateOK, FetchedAt: t0.Add(-100 * time.Hour)}, nil); err != nil {
		t.Fatal(err)
	}
	svc := refresh.New(refresh.Deps{Items: repo, Prices: repo, Fetchers: fetcher.NewRegistry(fetcher.Config{}), Now: func() time.Time { return t0 }, BaseContext: ctx})
	t.Cleanup(svc.Wait)

	v, err := svc.Estimates(ctx, it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Sites) != 1 || v.Sites[0].SiteID != st.ID {
		t.Fatalf("Sites = %+v, want メルカリの 1 件", v.Sites)
	}
	if v.Summary.Low == nil || *v.Summary.Low != low {
		t.Errorf("Summary.Low = %v, want %d", v.Summary.Low, low)
	}
	if v.Refreshing {
		t.Error("取れないサイトのために裏の更新を起動した")
	}
	if r, err := svc.RefreshItem(ctx, it.ID, refresh.ModeAll); err != nil || r.Targets != 0 || r.Fetched != 0 || r.Failed != 0 {
		t.Errorf("Report = %+v, %v; 取れないサイトは数えない・取らない", r, err)
	}
}
