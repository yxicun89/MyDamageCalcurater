package fetcher_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// このファイルは docs/phase3-api-spec.md の AC-H1〜H6(メルカリの headless 取得)。
// fixture は testdata/mercari.html(docs/sites-headless.md の構造だけを写した架空データ)。実サイトにも実際のブラウザにも接続しない。

const mercariImg = "https://example.invalid/img.jpg"

func mercariCell(href, alt, price string) string {
	return mercariCellImg(href, alt, price, mercariImg)
}

func mercariCellImg(href, alt, price, src string) string {
	return fmt.Sprintf(`<li data-testid="item-cell"><div><a href="%s" data-testid="thumbnail-link"><span><picture><img alt="%s" loading="lazy" src="%s"></picture><div><span data-testid="item-tile-price"><span><span>¥</span><span>%s</span></span></span></div></span></a></div></li>`, href, alt, src, price)
}

const mercariSkeleton = `<li data-testid="item-cell"><div><span></span></div></li>`

func mercariPage(cells ...string) []byte {
	return []byte(`<html><body><div data-testid="search-item-grid"><ul>` + strings.Join(cells, "") + `</ul></div></body></html>`)
}

// AC-H1: fixture を Listing にする。スケルトン(thumbnail-link を持たないセル)は捨てる。
// 個人出品(/item/)も Shops(/shops/product/)も絶対 URL にし、タイトルは alt から末尾の「のサムネイル」を除き、
// 価格は ¥ とカンマを除いた整数の円、在庫あり扱い(status=on_sale で絞っているため)。
func TestParseMercari_Fixture(t *testing.T) {
	b, err := os.ReadFile("testdata/mercari.html")
	if err != nil {
		t.Fatal(err)
	}
	got, err := fetcher.ParseMercari(b)
	if err != nil {
		t.Fatal(err)
	}
	want := []fetcher.Listing{
		{Title: "架空 商品A フィギュア", Price: 3000, URL: "https://jp.mercari.com/item/m00000000001", ImageURL: mercariImg, InStock: true},
		{Title: "架空 商品B(メルカリShops)", Price: 3500, URL: "https://jp.mercari.com/shops/product/ZZZZZZZZZZZZZZZZZZZZZZ", ImageURL: mercariImg, InStock: true},
	}
	if !slices.Equal(got, want) {
		t.Errorf("ParseMercari = %+v, want %+v", got, want)
	}
}

// AC-H2: セルごとの読み方(テーブル駆動)。読めない出品は除く(価格・タイトル・URL)。
func TestParseMercari_Cells(t *testing.T) {
	cases := []struct {
		name string
		html []byte
		want []fetcher.Listing
	}{
		{"スケルトンだけ", mercariPage(mercariSkeleton, mercariSkeleton), []fetcher.Listing{}},
		{"HTML が空", []byte(``), []fetcher.Listing{}},
		{"スケルトンの後ろの描画済みセルを拾う", mercariPage(mercariSkeleton, mercariCell("/item/m1", "品物のサムネイル", "1,280")),
			[]fetcher.Listing{{Title: "品物", Price: 1280, URL: "https://jp.mercari.com/item/m1", ImageURL: mercariImg, InStock: true}}},
		{"末尾の「のサムネイル」だけを除く(途中の同じ語は残す)", mercariPage(mercariCell("/item/m2", "のサムネイル入り 箱のサムネイル", "500")),
			[]fetcher.Listing{{Title: "のサムネイル入り 箱", Price: 500, URL: "https://jp.mercari.com/item/m2", ImageURL: mercariImg, InStock: true}}},
		{"サムネイルの語が無い alt はそのまま", mercariPage(mercariCell("/item/m3", "そのまま", "700")),
			[]fetcher.Listing{{Title: "そのまま", Price: 700, URL: "https://jp.mercari.com/item/m3", ImageURL: mercariImg, InStock: true}}},
		{"絶対 URL の href はそのまま", mercariPage(mercariCell("https://jp.mercari.com/item/m4", "A のサムネイル", "100")),
			[]fetcher.Listing{{Title: "A", Price: 100, URL: "https://jp.mercari.com/item/m4", ImageURL: mercariImg, InStock: true}}},
		{"相対の画像 URL もオリジンで絶対化する", mercariPage(mercariCellImg("/item/m13", "A のサムネイル", "100", "/static/a.jpg")),
			[]fetcher.Listing{{Title: "A", Price: 100, URL: "https://jp.mercari.com/item/m13", ImageURL: "https://jp.mercari.com/static/a.jpg", InStock: true}}},
		{"価格 0 は除く", mercariPage(mercariCell("/item/m5", "A のサムネイル", "0")), []fetcher.Listing{}},
		{"価格が読めない(¥ だけ)は除く", mercariPage(mercariCell("/item/m6", "A のサムネイル", "")), []fetcher.Listing{}},
		{"価格の上限 99,999,999 は含む", mercariPage(mercariCell("/item/m7", "A のサムネイル", "99,999,999")),
			[]fetcher.Listing{{Title: "A", Price: fetcher.MaxPrice, URL: "https://jp.mercari.com/item/m7", ImageURL: mercariImg, InStock: true}}},
		{"価格の上限を超えたら除く", mercariPage(mercariCell("/item/m8", "A のサムネイル", "100,000,000")), []fetcher.Listing{}},
		{"タイトルが空になる alt は除く", mercariPage(mercariCell("/item/m9", "のサムネイル", "100")), []fetcher.Listing{}},
		{"href が空は除く", mercariPage(mercariCell("", "A のサムネイル", "100")), []fetcher.Listing{}},
		{"価格の要素が無い thumbnail-link は除く",
			[]byte(`<ul><li data-testid="item-cell"><a href="/item/m10" data-testid="thumbnail-link"><img alt="A のサムネイル" src="https://example.invalid/a.jpg"></a></li></ul>`), []fetcher.Listing{}},
		{"img が無い(alt が無くタイトルを読めない)は除く",
			[]byte(`<ul><li data-testid="item-cell"><a href="/item/m11" data-testid="thumbnail-link"><span data-testid="item-tile-price">¥900</span></a></li></ul>`), []fetcher.Listing{}},
		{"画像の src が無ければ ImageURL は空(出品は残す)",
			[]byte(`<ul><li data-testid="item-cell"><a href="/item/m14" data-testid="thumbnail-link"><img alt="A のサムネイル"><span data-testid="item-tile-price">¥900</span></a></li></ul>`),
			[]fetcher.Listing{{Title: "A", Price: 900, URL: "https://jp.mercari.com/item/m14", InStock: true}}},
		{"item-cell でない li は読まない",
			[]byte(`<ul><li><a href="/item/m12" data-testid="thumbnail-link"><img alt="A のサムネイル" src="x"><span data-testid="item-tile-price">¥900</span></a></li></ul>`), []fetcher.Listing{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := fetcher.ParseMercari(c.html)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("結果が nil(0 件でも空のスライスを返す)")
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("ParseMercari = %+v\nwant %+v", got, c.want)
			}
		})
	}
}

// AC-H3: 上位 20 件まで。スケルトン・読めないセルは件数に数えない。並びは HTML の順(並べ替えない)。
func TestParseMercari_Limit(t *testing.T) {
	var cells []string
	for i := 1; i <= 30; i++ {
		if i%3 == 0 {
			cells = append(cells, mercariSkeleton)
		}
		cells = append(cells, mercariCell(fmt.Sprintf("/item/m%d", i), fmt.Sprintf("品物%d のサムネイル", i), fmt.Sprintf("%d", i*100)))
	}
	got, err := fetcher.ParseMercari(mercariPage(cells...))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != fetcher.MaxListings {
		t.Fatalf("%d 件, want %d", len(got), fetcher.MaxListings)
	}
	for i, l := range got {
		if l.Title != fmt.Sprintf("品物%d", i+1) || l.Price != (i+1)*100 {
			t.Fatalf("[%d] = %+v(HTML の順に先頭から)", i, l)
		}
	}
}

// ---- 偽のレンダラ ----

// fakePage は呼び出しの手順を記録する。counts は CountVisible の返す値(呼ぶたびに先頭から。尽きたら最後の値)。
type fakePage struct {
	log      []string
	counts   []int
	countFn  func(calls int) int // counts より優先(あれば)
	html     string
	navErr   error
	waitErr  error
	countErr error
	scrollFn func() error
	htmlErr  error

	waitDeadline time.Time
	waitHadDL    bool
	navURL       string
	selectors    []string
	nCount       int
	scrolls      int
	closed       int
}

func (p *fakePage) Navigate(_ context.Context, u string) error {
	p.log = append(p.log, "navigate")
	p.navURL = u
	return p.navErr
}

func (p *fakePage) WaitVisible(ctx context.Context, sel string) error {
	p.log = append(p.log, "wait")
	p.selectors = append(p.selectors, sel)
	p.waitDeadline, p.waitHadDL = ctx.Deadline()
	return p.waitErr
}

func (p *fakePage) CountVisible(_ context.Context, sel string) (int, error) {
	p.log = append(p.log, "count")
	p.selectors = append(p.selectors, sel)
	p.nCount++
	if p.countErr != nil {
		return 0, p.countErr
	}
	if p.countFn != nil {
		return p.countFn(p.nCount), nil
	}
	i := p.nCount - 1
	if i >= len(p.counts) {
		i = len(p.counts) - 1
	}
	return p.counts[i], nil
}

func (p *fakePage) ScrollToBottom(context.Context) error {
	p.log = append(p.log, "scroll")
	p.scrolls++
	if p.scrollFn != nil {
		return p.scrollFn()
	}
	return nil
}

func (p *fakePage) HTML(context.Context) (string, error) {
	p.log = append(p.log, "html")
	return p.html, p.htmlErr
}

func (p *fakePage) Close() error { p.closed++; return nil }

type fakeRenderer struct {
	page    *fakePage
	openErr error
	opened  int
}

func (r *fakeRenderer) OpenPage(context.Context) (fetcher.Page, error) {
	r.opened++
	if r.openErr != nil {
		return nil, r.openErr
	}
	return r.page, nil
}

func mercariSite() fetcher.Site {
	return fetcher.Site{ID: 1, Name: "メルカリ", SearchURLTemplate: mercariTemplate, FetchType: item.FetchHeadless}
}

func fixtureHTML(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/mercari.html")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// AC-H4: 手順。ページを 1 つ開く → deeplink.Build の URL へ移動 → MercariReadySelector が出るまで待つ(期限 MercariWaitTimeout 以内)
// → 描画済みセル数を MercariReadySelector で数える → 描画後の HTML を取る → ページを閉じる。HTML は ParseMercari の結果が Listing になる。
// 描画済みのセルがすでに 20 なら、スクロールしない。
func TestMercari_Procedure(t *testing.T) {
	p := &fakePage{counts: []int{20}, html: fixtureHTML(t)}
	r := &fakeRenderer{page: p}
	f := fetcher.NewMercari(r, newFakeClock())
	got, err := f.Fetch(context.Background(), mercariSite(), "グリス 仮面ライダー")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Price != 3000 {
		t.Errorf("Listing = %+v(fixture の 2 件)", got)
	}
	if want := []string{"navigate", "wait", "count", "html"}; !slices.Equal(p.log, want) {
		t.Errorf("呼び出し = %v, want %v", p.log, want)
	}
	if r.opened != 1 || p.closed != 1 {
		t.Errorf("OpenPage %d 回・Close %d 回, want 1 回ずつ", r.opened, p.closed)
	}
	if want := "https://jp.mercari.com/search?keyword=%E3%82%B0%E3%83%AA%E3%82%B9%20%E4%BB%AE%E9%9D%A2%E3%83%A9%E3%82%A4%E3%83%80%E3%83%BC&status=on_sale&sort=price&order=asc"; p.navURL != want {
		t.Errorf("移動先 = %s, want %s", p.navURL, want)
	}
	for _, s := range p.selectors {
		if s != fetcher.MercariReadySelector {
			t.Errorf("セレクタ = %q, want MercariReadySelector(item-cell だけはスケルトンにも付くので使わない)", s)
		}
	}
	if !p.waitHadDL || time.Until(p.waitDeadline) > fetcher.MercariWaitTimeout {
		t.Errorf("WaitVisible の ctx に MercariWaitTimeout 以内の期限が無い(hasDeadline=%v)", p.waitHadDL)
	}
}

// AC-H5: 描画を待つ条件。スクロールのたびに MercariScrollWait だけ待ち、数え直す。
// 20 に届くか、MercariNoGrowthRounds 回続けて増えないか、MercariScrollBudget(Clock で測る)を超えたらやめて HTML を取る。
func TestMercari_ScrollLoop(t *testing.T) {
	cases := []struct {
		name        string
		counts      []int
		countFn     func(int) int
		wantScrolls int
		wantCounts  int
	}{
		{"最初から 20", []int{20}, nil, 0, 1},
		{"増えて 20 に届く", []int{6, 12, 18, 20}, nil, 3, 4},
		{"増えなければ 3 回で止める", []int{6, 6, 6, 6}, nil, fetcher.MercariNoGrowthRounds, 1 + fetcher.MercariNoGrowthRounds},
		{"増えたら数え直す(6,6,7,7,7,7)", []int{6, 6, 7, 7, 7, 7}, nil, 5, 6},
		{"20 を超えて数えても止める", []int{6, 25}, nil, 1, 2},
		// 2 回に 1 回しか増えない(連続の停滞は 1 回まで)が、20 に届くには 19 秒かかる → 予算 10 秒(= 20 回のスクロール)で止める。
		{"予算を超えたら止める", nil, func(n int) int { return 1 + n/2 }, int(fetcher.MercariScrollBudget / fetcher.MercariScrollWait), -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &fakePage{counts: c.counts, countFn: c.countFn, html: fixtureHTML(t)}
			clock := newFakeClock()
			f := fetcher.NewMercari(&fakeRenderer{page: p}, clock)
			if _, err := f.Fetch(context.Background(), mercariSite(), "x"); err != nil {
				t.Fatal(err)
			}
			if p.scrolls != c.wantScrolls {
				t.Errorf("スクロール %d 回, want %d(呼び出し %v)", p.scrolls, c.wantScrolls, p.log)
			}
			if c.wantCounts >= 0 && p.nCount != c.wantCounts {
				t.Errorf("数えた %d 回, want %d", p.nCount, c.wantCounts)
			}
			sleeps := clock.takeSleeps()
			var total time.Duration
			for _, d := range sleeps {
				if d != fetcher.MercariScrollWait {
					t.Errorf("待ち = %v, want %v ずつ", d, fetcher.MercariScrollWait)
				}
				total += d
			}
			if total != time.Duration(c.wantScrolls)*fetcher.MercariScrollWait {
				t.Errorf("待ちの合計 = %v, want %v(スクロール 1 回につき 1 回)", total, time.Duration(c.wantScrolls)*fetcher.MercariScrollWait)
			}
			if total > fetcher.MercariScrollBudget {
				t.Errorf("待ちの合計 %v が上限 %v を超えた", total, fetcher.MercariScrollBudget)
			}
			if want := "html"; p.log[len(p.log)-1] != want {
				t.Errorf("最後の呼び出し = %s, want %s(止めたあとに HTML を取る)", p.log[len(p.log)-1], want)
			}
			if p.closed != 1 {
				t.Errorf("Close %d 回, want 1", p.closed)
			}
		})
	}
}

// AC-H6: 失敗の扱い。どの失敗でもページを閉じる。ctx の取り消しは errors.Is(context.Canceled) で分かる。
//   - 開けない・移動できない・数えられない・スクロールできない・HTML を取れない → 原因を包んだ error(Listing なし)
//   - 待っても出ない(WaitVisible の失敗)→ ErrNotRendered(0 件とは区別し、前回値を残せるようにする)
//   - 実際のブラウザの失敗メッセージにも検索語・URL を出さなくてよい(エラーは短く)
func TestMercari_Errors(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name       string
		r          func() (*fakeRenderer, *fakePage)
		want       []error
		wantClosed int
	}{
		{"開けない", func() (*fakeRenderer, *fakePage) { return &fakeRenderer{openErr: boom}, nil }, []error{boom}, 0},
		{"移動できない", func() (*fakeRenderer, *fakePage) {
			p := &fakePage{counts: []int{20}, navErr: boom}
			return &fakeRenderer{page: p}, p
		}, []error{boom}, 1},
		{"待っても出ない", func() (*fakeRenderer, *fakePage) {
			p := &fakePage{counts: []int{20}, waitErr: boom}
			return &fakeRenderer{page: p}, p
		}, []error{fetcher.ErrNotRendered, boom}, 1},
		{"待ちが期限切れ", func() (*fakeRenderer, *fakePage) {
			p := &fakePage{counts: []int{20}, waitErr: context.DeadlineExceeded}
			return &fakeRenderer{page: p}, p
		}, []error{fetcher.ErrNotRendered}, 1},
		{"数えられない", func() (*fakeRenderer, *fakePage) {
			p := &fakePage{counts: []int{20}, countErr: boom}
			return &fakeRenderer{page: p}, p
		}, []error{boom}, 1},
		{"スクロールできない", func() (*fakeRenderer, *fakePage) {
			p := &fakePage{counts: []int{6}, scrollFn: func() error { return boom }}
			return &fakeRenderer{page: p}, p
		}, []error{boom}, 1},
		{"HTML を取れない", func() (*fakeRenderer, *fakePage) {
			p := &fakePage{counts: []int{20}, htmlErr: boom}
			return &fakeRenderer{page: p}, p
		}, []error{boom}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, p := c.r()
			got, err := fetcher.NewMercari(r, newFakeClock()).Fetch(context.Background(), mercariSite(), "x")
			if err == nil || got != nil {
				t.Fatalf("Fetch = %+v, %v; want nil, error", got, err)
			}
			for _, w := range c.want {
				if !errors.Is(err, w) {
					t.Errorf("err = %v; errors.Is(%v) でない", err, w)
				}
			}
			if p != nil && p.closed != c.wantClosed {
				t.Errorf("Close %d 回, want %d", p.closed, c.wantClosed)
			}
		})
	}

	t.Run("スクロール中に取り消す", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		p := &fakePage{counts: []int{6}, html: fixtureHTML(t), scrollFn: func() error { cancel(); return nil }}
		_, err := fetcher.NewMercari(&fakeRenderer{page: p}, newFakeClock()).Fetch(ctx, mercariSite(), "x")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
		if p.closed != 1 {
			t.Errorf("Close %d 回, want 1", p.closed)
		}
	})
	t.Run("開く前に取り消し済み", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		p := &fakePage{counts: []int{20}, html: fixtureHTML(t)}
		_, err := fetcher.NewMercari(&fakeRenderer{page: p}, newFakeClock()).Fetch(ctx, mercariSite(), "x")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	})
	t.Run("0 件のページはエラーにしない(描画はされた)", func(t *testing.T) {
		p := &fakePage{counts: []int{20}, html: string(mercariPage(mercariSkeleton))}
		got, err := fetcher.NewMercari(&fakeRenderer{page: p}, newFakeClock()).Fetch(context.Background(), mercariSite(), "x")
		if err != nil || len(got) != 0 {
			t.Errorf("Fetch = %+v, %v; want 0 件・エラーなし", got, err)
		}
	})
}
