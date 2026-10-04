package refresh_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/refresh"
)

// AC-H9: メルカリ(headless)は夜間の CronJob(RefreshAll)だけで取る。手動(ModeAll・Refresh)・裏の更新(ModeStale)ではブラウザを開かない。
// 夜間の取得は fixture の 2 件を保存する。ブラウザは偽物(実際には起動しない)。

type headlessPage struct{ html string }

func (p headlessPage) Navigate(context.Context, string) error            { return nil }
func (p headlessPage) WaitVisible(context.Context, string) error         { return nil }
func (p headlessPage) CountVisible(context.Context, string) (int, error) { return 20, nil }
func (p headlessPage) ScrollToBottom(context.Context) error              { return nil }
func (p headlessPage) HTML(context.Context) (string, error)              { return p.html, nil }
func (p headlessPage) Close() error                                      { return nil }

type countingRenderer struct {
	mu    sync.Mutex
	opens int
	html  string
}

func (r *countingRenderer) OpenPage(context.Context) (fetcher.Page, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.opens++
	return headlessPage{r.html}, nil
}

func (r *countingRenderer) count() int { r.mu.Lock(); defer r.mu.Unlock(); return r.opens }

func TestRefresh_MercariNightlyOnly(t *testing.T) {
	body, err := os.ReadFile("../fetcher/testdata/mercari.html")
	if err != nil {
		t.Fatal(err)
	}
	rend := &countingRenderer{html: string(body)}

	ctx := context.Background()
	repo := item.NewMemoryRepository()
	st, err := repo.CreateSite(ctx, item.NewSite{Name: "メルカリ", SearchURLTemplate: "https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc", FetchType: item.FetchHeadless})
	if err != nil {
		t.Fatal(err)
	}
	g, err := repo.CreateGenre(ctx, item.NewGenre{Name: "G", QueryTemplate: "{name}", SiteIDs: []int64{st.ID}})
	if err != nil {
		t.Fatal(err)
	}
	// fixture の出品タイトル(「架空 商品A…」「架空 商品B…」)に含まれる名前にする(含まれないと title_mismatch で no_result になる)
	it, err := repo.CreateItem(ctx, item.NewItem{GenreID: g.ID, Name: "架空", ImagePath: img})
	if err != nil {
		t.Fatal(err)
	}
	reg := fetcher.NewRegistry(fetcher.Config{Renderer: rend, Clock: noWaitClock{}})
	svc := refresh.New(refresh.Deps{Items: repo, Prices: repo, Fetchers: reg, Now: func() time.Time { return t0 }, BaseContext: ctx})
	t.Cleanup(svc.Wait)

	for _, mode := range []refresh.Mode{refresh.ModeAll, refresh.ModeStale} {
		r, err := svc.RefreshItem(ctx, it.ID, mode)
		if err != nil {
			t.Fatal(err)
		}
		if r.Fetched != 0 || r.Failed != 0 || rend.count() != 0 {
			t.Fatalf("mode %v: Report = %+v, opens = %d; 夜間専用は取らない", mode, r, rend.count())
		}
	}
	if v, err := svc.Refresh(ctx, it.ID); err != nil || v.Refreshing || rend.count() != 0 {
		t.Errorf("手動の更新でブラウザを開いた: %+v, %v, opens=%d", v, err, rend.count())
	}
	svc.Wait()

	rep, err := svc.RefreshAll(ctx)
	if err != nil || rep.Failed != 0 {
		t.Fatalf("RefreshAll = %+v, %v", rep, err)
	}
	if rend.count() != 1 {
		t.Fatalf("夜間の更新でブラウザを開いた回数 = %d, want 1", rend.count())
	}
	es, _ := repo.ListEstimates(ctx, it.ID)
	if len(es) != 1 || es[0].Status != "ok" || es[0].Count != 2 {
		t.Fatalf("estimates = %+v(fixture の 2 件で ok)", es)
	}
}

// AC-H9: Renderer が無い(Chromium が無い環境)なら、夜間の更新でもメルカリは取らず、失敗にも数えない(取得できるサイトではないため)。
func TestRefresh_MercariWithoutRenderer(t *testing.T) {
	ctx := context.Background()
	repo := item.NewMemoryRepository()
	st, _ := repo.CreateSite(ctx, item.NewSite{Name: "メルカリ", SearchURLTemplate: "https://jp.mercari.com/search?keyword={q}", FetchType: item.FetchHeadless})
	g, _ := repo.CreateGenre(ctx, item.NewGenre{Name: "G", QueryTemplate: "{name}", SiteIDs: []int64{st.ID}})
	if _, err := repo.CreateItem(ctx, item.NewItem{GenreID: g.ID, Name: "テスト", ImagePath: img}); err != nil {
		t.Fatal(err)
	}
	reg := fetcher.NewRegistry(fetcher.Config{Clock: noWaitClock{}})
	svc := refresh.New(refresh.Deps{Items: repo, Prices: repo, Fetchers: reg, Now: func() time.Time { return t0 }, BaseContext: ctx})
	t.Cleanup(svc.Wait)
	rep, err := svc.RefreshAll(ctx)
	if err != nil || rep.Failed != 0 {
		t.Fatalf("RefreshAll = %+v, %v", rep, err)
	}
}
