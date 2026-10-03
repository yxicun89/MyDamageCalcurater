package fetcher_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
)

// このファイルは docs/phase3-api-spec.md の AC-S*(サイト別の scrape)。
// fixture は testdata/{cardrush,amiami,surugaya,yahoofurima}.html(sites.md の構造だけを写した架空データ)。
// 実サイトには接続しない(httptest)。

const base = "{base}"

type scrapeFetcher struct {
	name    string
	newF    func(*http.Client) fetcher.Fetcher
	fixture string
	// template は検索 URL テンプレート({base} は httptest の URL)。
	template string
	// wantURI は query「テスト」で届くべきリクエストの URI(パスとクエリ)。
	wantURI string
	want    []fetcher.Listing
	// empty は「結果 0 件」のページ(エラーにしない)。
	empty string
	// gen は n 件(価格は 100 円刻み)を返すページの生成。
	gen func(n int) string
}

const qTest = "%E3%83%86%E3%82%B9%E3%83%88" // 「テスト」

var scrapeCases = []scrapeFetcher{
	{
		name: "cardrush", newF: fetcher.NewCardrush, fixture: "cardrush.html",
		template: base + "/product-list?keyword={q}&order=asc&available=1&num=20",
		wantURI:  "/product-list?keyword=" + qTest + "&order=asc&available=1&num=20",
		want: []fetcher.Listing{
			{Title: "テストカード・アルファ【-】{XX01/01}《火》", Price: 50, URL: "https://www.cardrush-dm.example/product/90001", ImageURL: "https://www.cardrush-dm.example/data/90001.jpg", InStock: true},
			{Title: "テストカード・ベータ【R】{XX02/02}《水》", Price: 1280, URL: "https://www.cardrush-dm.example/product/90002", ImageURL: "https://www.cardrush-dm.example/data/90002.jpg", InStock: true},
		},
		empty: `<html><body><ul class="item_list"></ul></body></html>`,
		gen: func(n int) string {
			var b strings.Builder
			b.WriteString(`<html><body><ul class="item_list">`)
			for i := 1; i <= n; i++ {
				fmt.Fprintf(&b, `<li class="list_item_cell"><div class="item_data" data-product-id="%d"><a href="https://shop.example/p/%d" class="item_data_link"><div class="global_photo"><img src="https://shop.example/i/%d.jpg"></div><p class="item_name"><span class="goods_name">商品 %d</span></p><div class="item_info"><div class="price"><p class="selling_price"><span class="figure">%d円</span></p></div><p class="stock">在庫数 1枚</p></div></a></div></li>`, i, i, i, i, i*100)
			}
			b.WriteString(`</ul></body></html>`)
			return b.String()
		},
	},
	{
		name: "amiami", newF: fetcher.NewAmiami, fixture: "amiami.html",
		template: base + "/top/search/list?s_keywords={q}&s_sortkey=pricea",
		wantURI:  "/top/search/list?s_keywords=" + qTest + "&s_sortkey=pricea",
		want: []fetcher.Listing{
			{Title: "テスト フィギュア アルファ(架空)", Price: 8080, URL: "https://www.amiami.example/top/detail/detail?gcode=FIGURE-900001-R", ImageURL: "https://img.amiami.example/thumb300/900001.jpg", InStock: true},
			{Title: "テスト フィギュア ベータ(架空)", Price: 1480, URL: "https://www.amiami.example/top/detail/detail?gcode=FIGURE-900002", ImageURL: "https://img.amiami.example/thumb300/900002.jpg", InStock: true},
		},
		empty: `<html><body><div id="search_table"><div class="product_table_list"></div></div></body></html>`,
		gen: func(n int) string {
			var b strings.Builder
			b.WriteString(`<html><body><div class="product_table_list">`)
			for i := 1; i <= n; i++ {
				fmt.Fprintf(&b, `<div class="product_box"><a href="https://shop.example/p/%d"><div class="product_img"><img src="blank.gif" data-src="https://shop.example/i/%d.jpg"></div><div class="product_name"><div class="product_name_inner">商品 %d</div></div><div class="product_price">%d</div></a></div>`, i, i, i, i*100)
			}
			b.WriteString(`</div></body></html>`)
			return b.String()
		},
	},
	{
		name: "yahoofurima", newF: fetcher.NewYahooFurima, fixture: "yahoofurima.html",
		template: base + "/search/{q}",
		wantURI:  "/search/" + qTest,
		want: []fetcher.Listing{
			{Title: "テスト 商品 アルファ", Price: 550, URL: base + "/item/9900000001", ImageURL: "https://img.example/9900000001.jpg", InStock: true},
			{Title: "テスト 商品 ベータ", Price: 1200, URL: base + "/item/z9900000002", ImageURL: "https://img.example/9900000002.jpg", InStock: true},
		},
		empty: furimaPage(),
		// 関連度順(価格は降順になるように)。
		gen: func(n int) string {
			var items []string
			for i := 1; i <= n; i++ {
				items = append(items, furimaItem(fmt.Sprintf("id%d", i), fmt.Sprintf("商品 %d", i), (n+1-i)*100, "OPEN"))
			}
			return furimaPage(items...)
		},
	},
	{
		// 駿河屋は登録表に入れる(30 秒間隔・夜間のみ)。
		name: "surugaya", newF: fetcher.NewSurugaya, fixture: "surugaya.html",
		template: base + "/search?category=&search_word={q}&rankBy=price%3Aascending&inStock=On",
		wantURI:  "/search?category=&search_word=" + qTest + "&rankBy=price%3Aascending&inStock=On",
		want: []fetcher.Listing{
			// 2 件目は新品が品切れで定価だけ(販売価格が無い)ので出品に含めない。
			{Title: "テスト フィギュア アルファ", Price: 500, URL: base + "/product/detail/900000001?tenpo_cd=", ImageURL: "https://www.suruga-ya.example/database/photo.php?shinaban=900000001&size=m", InStock: true},
		},
		empty: `<html><body><div class="item_box"></div></body></html>`,
		gen: func(n int) string {
			var b strings.Builder
			b.WriteString(`<html><body><div class="item_box">`)
			for i := 1; i <= n; i++ {
				fmt.Fprintf(&b, `<div class="item"><div class="photo_box"><a href="/product/detail/%d"><img src="https://shop.example/i/%d.jpg"></a></div><div class="item_detail"><div class="title"><a href="/product/detail/%d"><h3 class="product-name">商品 %d</h3></a></div></div><div class="item_price"><p class="price_teika">中古：<strong>￥%d </strong> <span class="tax">税込</span></p></div></div>`, i, i, i, i, i*100)
			}
			b.WriteString(`</div></body></html>`)
			return b.String()
		},
	},
}

func furimaItem(id, title string, price int, status string) string {
	return fmt.Sprintf(`{"id":%q,"title":%q,"price":%d,"thumbnailImageUrl":"https://img.example/%s.jpg","itemStatus":%q,"condition":"new"}`, id, title, price, id, status)
}

func furimaPage(items ...string) string {
	return `<html><body><script id="__NEXT_DATA__" type="application/json">{"props":{"initialState":{"searchState":{"search":{"result":{"items":[` +
		strings.Join(items, ",") + `]}}}}}}</script></body></html>`
}

// htmlServer は body を返す httptest サーバー。受けたリクエストの URI を記録する。
type htmlServer struct {
	*httptest.Server
	mu   sync.Mutex
	uris []string
}

func newHTMLServer(t *testing.T, h func(w http.ResponseWriter, r *http.Request)) *htmlServer {
	t.Helper()
	s := &htmlServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.uris = append(s.uris, r.Method+" "+r.RequestURI)
		s.mu.Unlock()
		h(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *htmlServer) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.uris)
}

func serveHTML(body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.Write([]byte(body))
	}
}

func serveFixture(t *testing.T, name string) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	return serveFile(t, "testdata/"+name) // yahoo_test.go と同じ(Content-Type は json だが本文は読める)
}

func scrapeSite(c scrapeFetcher, srvURL string) fetcher.Site {
	return fetcher.Site{ID: 21, Name: c.name, SearchURLTemplate: strings.ReplaceAll(c.template, base, srvURL), FetchType: item.FetchScrape, IsReference: true}
}

func withBase(ls []fetcher.Listing, b string) []fetcher.Listing {
	out := slices.Clone(ls)
	for i := range out {
		out[i].URL = strings.ReplaceAll(out[i].URL, base, b)
		out[i].ImageURL = strings.ReplaceAll(out[i].ImageURL, base, b)
	}
	return out
}

// AC-S1: 検索 URL は site.SearchURLTemplate を deeplink.Build で作った 1 回の GET。fixture を Listing にする
// (タイトルは強調タグ込みの text、価格は整数の円、URL・画像は検索ページ基準の絶対 URL、画像の lazyload は data-src)。
func TestScrape_RequestAndParse(t *testing.T) {
	for _, c := range scrapeCases {
		t.Run(c.name, func(t *testing.T) {
			srv := newHTMLServer(t, serveFixture(t, c.fixture))
			got, err := c.newF(allowAll()).Fetch(context.Background(), scrapeSite(c, srv.URL), "テスト")
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if reqs := srv.requests(); !slices.Equal(reqs, []string{"GET " + c.wantURI}) {
				t.Errorf("リクエスト = %v, want [GET %s]", reqs, c.wantURI)
			}
			want := withBase(c.want, srv.URL)
			if !slices.Equal(got, want) {
				t.Errorf("Listings =\n %+v\nwant\n %+v", got, want)
			}
		})
	}
}

// AC-S2: 先頭 20 件まで。カードラッシュ・あみあみ・駿河屋はサイトの並びのまま。
// Yahoo!フリマは関連度順の上位 20 件を取ったあと、こちらで価格の昇順に並べる(安定ソート)。
func TestScrape_LimitAndOrder(t *testing.T) {
	for _, c := range scrapeCases {
		t.Run(c.name, func(t *testing.T) {
			srv := newHTMLServer(t, serveHTML(c.gen(25)))
			got, err := c.newF(allowAll()).Fetch(context.Background(), scrapeSite(c, srv.URL), "テスト")
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != fetcher.MaxListings {
				t.Fatalf("件数 = %d, want %d", len(got), fetcher.MaxListings)
			}
			var prices []int
			for _, l := range got {
				prices = append(prices, l.Price)
			}
			var want []int
			if c.name == "yahoofurima" {
				// 25 件の価格は 2500, 2400, ... 100。先頭 20 件は 2500..600 → 昇順で 600..2500。
				for p := 600; p <= 2500; p += 100 {
					want = append(want, p)
				}
			} else {
				for p := 100; p <= 2000; p += 100 {
					want = append(want, p)
				}
			}
			if !slices.Equal(prices, want) {
				t.Errorf("価格 = %v, want %v", prices, want)
			}
		})
	}
}

// AC-S3: 0 件のページはエラーにしない(空のスライス。目安は no_result になる)。
func TestScrape_Empty(t *testing.T) {
	for _, c := range scrapeCases {
		t.Run(c.name, func(t *testing.T) {
			srv := newHTMLServer(t, serveHTML(c.empty))
			got, err := c.newF(allowAll()).Fetch(context.Background(), scrapeSite(c, srv.URL), "テスト")
			if err != nil || len(got) != 0 {
				t.Errorf("Fetch = %v, %v; want 空・エラーなし", got, err)
			}
		})
	}
}

// AC-S4: 失敗。2xx 以外 → ErrUpstreamStatus、MaxResponseBytes 超 → netguard.ErrTooLarge、
// 既定クライアント(Client が nil)でループバック → netguard.ErrForbiddenAddress、取り消し済みの ctx → context.Canceled。
func TestScrape_Errors(t *testing.T) {
	for _, c := range scrapeCases {
		t.Run(c.name, func(t *testing.T) {
			t.Run("status", func(t *testing.T) {
				for _, code := range []int{403, 404, 500} {
					srv := newHTMLServer(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "x", code) })
					_, err := c.newF(allowAll()).Fetch(context.Background(), scrapeSite(c, srv.URL), "テスト")
					if !errors.Is(err, fetcher.ErrUpstreamStatus) {
						t.Errorf("status %d: err = %v, want ErrUpstreamStatus", code, err)
					}
				}
			})
			t.Run("too large", func(t *testing.T) {
				srv := newHTMLServer(t, serveHTML("<html><body>"+strings.Repeat("a", fetcher.MaxResponseBytes+1)+"</body></html>"))
				_, err := c.newF(allowAll()).Fetch(context.Background(), scrapeSite(c, srv.URL), "テスト")
				if !errors.Is(err, netguard.ErrTooLarge) {
					t.Errorf("err = %v, want netguard.ErrTooLarge", err)
				}
			})
			t.Run("default client forbids loopback", func(t *testing.T) {
				srv := newHTMLServer(t, serveHTML(c.empty))
				_, err := c.newF(nil).Fetch(context.Background(), scrapeSite(c, srv.URL), "テスト")
				if !errors.Is(err, netguard.ErrForbiddenAddress) {
					t.Errorf("err = %v, want netguard.ErrForbiddenAddress", err)
				}
				if n := len(srv.requests()); n != 0 {
					t.Errorf("サーバーに %d 件届いた", n)
				}
			})
			t.Run("canceled", func(t *testing.T) {
				srv := newHTMLServer(t, serveHTML(c.empty))
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				_, err := c.newF(allowAll()).Fetch(ctx, scrapeSite(c, srv.URL), "テスト")
				if !errors.Is(err, context.Canceled) {
					t.Errorf("err = %v, want context.Canceled", err)
				}
			})
		})
	}
}

// AC-S5: 販売価格・タイトル・URL のどれかが読めない出品は除く(エラーにしない)。
func TestScrape_SkipsBrokenEntries(t *testing.T) {
	cases := map[string]struct{ html, wantTitle string }{
		"cardrush": {
			html: `<ul><li class="list_item_cell"><div class="item_data"><a href="https://s.example/1" class="item_data_link"><p class="item_name"><span class="goods_name">価格なし</span></p></a></div></li>` +
				`<li class="list_item_cell"><div class="item_data"><a href="https://s.example/2" class="item_data_link"><p class="item_name"><span class="goods_name">ゼロ円</span></p><p class="selling_price"><span class="figure">0円</span></p></a></div></li>` +
				`<li class="list_item_cell"><div class="item_data"><a class="item_data_link"><p class="item_name"><span class="goods_name">URLなし</span></p><p class="selling_price"><span class="figure">10円</span></p></a></div></li>` +
				`<li class="list_item_cell"><div class="item_data"><a href="https://s.example/4" class="item_data_link"><p class="item_name"><span class="goods_name">正常</span></p><p class="selling_price"><span class="figure">10円</span></p></a></div></li></ul>`,
			wantTitle: "正常",
		},
		"amiami": {
			html: `<div class="product_table_list"><div class="product_box"><a href="https://s.example/1"><div class="product_name_inner">価格なし</div></a></div>` +
				`<div class="product_box"><a href="https://s.example/2"><div class="product_name_inner"> </div><div class="product_price">100</div></a></div>` +
				`<div class="product_box"><a href="https://s.example/3"><div class="product_name_inner">正常</div><div class="product_price">1,100</div></a></div></div>`,
			wantTitle: "正常",
		},
		"yahoofurima": {
			html:      furimaPage(furimaItem("a", "価格ゼロ", 0, "OPEN"), furimaItem("b", "", 100, "OPEN"), furimaItem("", "ID なし", 100, "OPEN"), furimaItem("d", "正常", 100, "OPEN")),
			wantTitle: "正常",
		},
	}
	for _, c := range scrapeCases {
		tc, ok := cases[c.name]
		if !ok {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			srv := newHTMLServer(t, serveHTML(tc.html))
			got, err := c.newF(allowAll()).Fetch(context.Background(), scrapeSite(c, srv.URL), "テスト")
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Title != tc.wantTitle {
				t.Errorf("Listings = %+v, want 「%s」だけ", got, tc.wantTitle)
			}
		})
	}
}

// AC-S6: 相対 URL(商品・画像)は検索ページの URL を基準に絶対 URL にする。
func TestScrape_RelativeURLs(t *testing.T) {
	cases := map[string]struct {
		html string
		want fetcher.Listing
	}{
		"cardrush": {
			html: `<ul><li class="list_item_cell"><div class="item_data"><a href="/product/1" class="item_data_link"><div class="global_photo"><img src="/data/1.jpg"></div><p class="item_name"><span class="goods_name">相対</span></p><p class="selling_price"><span class="figure">70円</span></p></a></div></li></ul>`,
			want: fetcher.Listing{Title: "相対", Price: 70, URL: base + "/product/1", ImageURL: base + "/data/1.jpg", InStock: true},
		},
		"amiami": {
			html: `<div class="product_table_list"><div class="product_box"><a href="/top/detail/detail?gcode=X1"><div class="product_img"><img src="blank.gif" data-src="/img/x1.jpg"></div><div class="product_name_inner">相対</div><div class="product_price">1,100</div></a></div></div>`,
			want: fetcher.Listing{Title: "相対", Price: 1100, URL: base + "/top/detail/detail?gcode=X1", ImageURL: base + "/img/x1.jpg", InStock: true},
		},
	}
	for _, c := range scrapeCases {
		tc, ok := cases[c.name]
		if !ok {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			srv := newHTMLServer(t, serveHTML(tc.html))
			got, err := c.newF(allowAll()).Fetch(context.Background(), scrapeSite(c, srv.URL), "テスト")
			if err != nil {
				t.Fatal(err)
			}
			want := withBase([]fetcher.Listing{tc.want}, srv.URL)
			if !slices.Equal(got, want) {
				t.Errorf("Listings = %+v, want %+v", got, want)
			}
		})
	}
}

// AC-S7: 在庫。判定できないサイト(あみあみ・駿河屋の一覧)は true。
// カードラッシュは .stock の「在庫数 N枚」が 0 なら false(読めなければ true)。
// Yahoo!フリマは itemStatus が OPEN のときだけ true(売り切れの値は未確認なので、OPEN 以外は在庫なしとして扱う)。
func TestScrape_InStock(t *testing.T) {
	cardrushItem := func(stock string) string {
		return `<ul><li class="list_item_cell"><div class="item_data"><a href="https://s.example/1" class="item_data_link"><p class="item_name"><span class="goods_name">商品</span></p><p class="selling_price"><span class="figure">70円</span></p>` + stock + `</a></div></li></ul>`
	}
	cases := []struct {
		site string
		html string
		want bool
	}{
		{"cardrush", cardrushItem(`<p class="stock">在庫数 63枚</p>`), true},
		{"cardrush", cardrushItem(`<p class="stock">在庫数 0枚</p>`), false},
		{"cardrush", cardrushItem(``), true},
		{"yahoofurima", furimaPage(furimaItem("a", "商品", 100, "OPEN")), true},
		{"yahoofurima", furimaPage(furimaItem("a", "商品", 100, "SOLD_OUT")), false},
		{"amiami", `<div class="product_box"><a href="https://s.example/1"><div class="product_name_inner">商品</div><div class="product_price">100</div></a></div>`, true},
	}
	for i, tc := range cases {
		for _, c := range scrapeCases {
			if c.name != tc.site {
				continue
			}
			t.Run(fmt.Sprintf("%s-%d", tc.site, i), func(t *testing.T) {
				srv := newHTMLServer(t, serveHTML(tc.html))
				got, err := c.newF(allowAll()).Fetch(context.Background(), scrapeSite(c, srv.URL), "テスト")
				if err != nil || len(got) != 1 {
					t.Fatalf("Fetch = %+v, %v", got, err)
				}
				if got[0].InStock != tc.want {
					t.Errorf("InStock = %v, want %v", got[0].InStock, tc.want)
				}
			})
		}
	}
}

// AC-S8: Yahoo!フリマ。取得するのはパラメータなしの /search/{q} だけ(robots.txt が sort などの付いた検索を禁じている)。
// テンプレートにクエリや # が付いていても、実際のリクエストからは外す。リクエストは 1 回だけ。
func TestYahooFurima_NoQueryParameters(t *testing.T) {
	srv := newHTMLServer(t, serveHTML(furimaPage()))
	site := fetcher.Site{ID: 3, Name: "Yahoo!フリマ", FetchType: item.FetchScrape,
		SearchURLTemplate: srv.URL + "/search/{q}?sort=price&order=asc&open=1#top"}
	if _, err := fetcher.NewYahooFurima(allowAll()).Fetch(context.Background(), site, "S.H.Figuarts グリス"); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /search/S.H.Figuarts%20%E3%82%B0%E3%83%AA%E3%82%B9"}
	if reqs := srv.requests(); !slices.Equal(reqs, want) {
		t.Errorf("リクエスト = %v, want %v", reqs, want)
	}
}

// AC-S9: Yahoo!フリマの埋め込み JSON。同じ価格は元の並びを保つ。items の無い・壊れたページ(構造の変化)はエラー(0 件と区別する)。
func TestYahooFurima_PageStructure(t *testing.T) {
	site := func(u string) fetcher.Site {
		return fetcher.Site{ID: 3, Name: "Yahoo!フリマ", FetchType: item.FetchScrape, SearchURLTemplate: u + "/search/{q}"}
	}
	t.Run("stable sort", func(t *testing.T) {
		srv := newHTMLServer(t, serveHTML(furimaPage(furimaItem("a", "A", 300, "OPEN"), furimaItem("b", "B", 100, "OPEN"), furimaItem("c", "C", 300, "OPEN"), furimaItem("d", "D", 100, "OPEN"))))
		got, err := fetcher.NewYahooFurima(allowAll()).Fetch(context.Background(), site(srv.URL), "x")
		if err != nil {
			t.Fatal(err)
		}
		var titles []string
		for _, l := range got {
			titles = append(titles, l.Title)
		}
		if !slices.Equal(titles, []string{"B", "D", "A", "C"}) {
			t.Errorf("並び = %v, want [B D A C]", titles)
		}
	})
	bad := map[string]string{
		"no script":     `<html><body>no data</body></html>`,
		"invalid json":  `<html><body><script id="__NEXT_DATA__" type="application/json">{not json</script></body></html>`,
		"no items path": `<html><body><script id="__NEXT_DATA__" type="application/json">{"props":{"initialState":{}}}</script></body></html>`,
	}
	for name, html := range bad {
		t.Run(name, func(t *testing.T) {
			srv := newHTMLServer(t, serveHTML(html))
			got, err := fetcher.NewYahooFurima(allowAll()).Fetch(context.Background(), site(srv.URL), "x")
			if err == nil {
				t.Errorf("Fetch = %v, nil; want エラー", got)
			}
		})
	}
}

// AC-S10: 価格の表記を整数の円にする。数字が読めない・1 円未満は ok=false。全角は半角にして読む。
func TestParseYen(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"50円", 50, true},
		{"1,280円", 1280, true},
		{"8,080", 8080, true},
		{"1234", 1234, true},
		{"  1,234  ", 1234, true},
		{"￥500 ", 500, true},
		{"¥1,280", 1280, true},
		{"中古：￥500 税込", 500, true},
		{"1,234円(税込)", 1234, true},
		{"税込 1,234円", 1234, true},
		{"１，２８０円", 1280, true},
		{"1,000,000円", 1000000, true},
		{"", 0, false},
		{"(税込)", 0, false},
		{"品切れ", 0, false},
		{"0円", 0, false},
		{"円", 0, false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, ok := fetcher.ParseYen(c.in)
			if got != c.want || ok != c.ok {
				t.Errorf("ParseYen(%q) = %d, %v; want %d, %v", c.in, got, ok, c.want, c.ok)
			}
		})
	}
}

// rewriteTransport は、どのホスト宛のリクエストも httptest サーバーへ送る(Host ヘッダは元のまま)。
// 本番のホスト名で Fetcher が選ばれることを、実サイトに接続せずに確かめる。
type rewriteTransport struct{ target string }

func (rt rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = strings.TrimPrefix(rt.target, "http://")
	return http.DefaultTransport.RoundTrip(r2)
}

const (
	cardrushTemplate = "https://www.cardrush-dm.jp/product-list?keyword={q}&order=asc&available=1&num=20"
	amiamiTemplate   = "https://slist.amiami.jp/top/search/list?s_keywords={q}&s_sortkey=pricea"
	furimaTemplate   = "https://paypayfleamarket.yahoo.co.jp/search/{q}"
	surugayaTemplate = "https://www.suruga-ya.jp/search?category=&search_word={q}&rankBy=price%3Aascending&inStock=On"
	mercariTemplate  = "https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc"
	amazonTemplate   = "https://www.amazon.co.jp/s?k={q}&s=price-asc-rank"
	shoppingTemplate = "https://shopping.yahoo.co.jp/search/{q}/0/?X=2"
)

func siteOf(id int64, ft item.FetchType, tmpl string) fetcher.Site {
	return fetcher.Site{ID: id, Name: fmt.Sprintf("s%d", id), SearchURLTemplate: tmpl, FetchType: ft}
}

// AC-S11: 登録表は fetch_type と検索 URL テンプレートのホスト名で Fetcher を選ぶ。
//   - scrape は www.cardrush-dm.jp・slist.amiami.jp・paypayfleamarket.yahoo.co.jp・www.suruga-ya.jp だけ(大文字小文字は区別しない・ポートは無視)
//   - 未対応のホストの scrape は取得しない(未実装)
//   - headless・link_only は、ホストが対応済みでも取得しない
//   - api は fetch_type だけ(appid があるときだけ)
//   - テンプレートが読めない scrape は取得しない
func TestRegistry_ForSite(t *testing.T) {
	cases := []struct {
		name  string
		site  fetcher.Site
		appid string
		want  bool
	}{
		{"カードラッシュ", siteOf(1, item.FetchScrape, cardrushTemplate), "", true},
		{"あみあみ", siteOf(2, item.FetchScrape, amiamiTemplate), "", true},
		{"Yahoo!フリマ", siteOf(3, item.FetchScrape, furimaTemplate), "", true},
		{"ホストは大文字小文字を区別しない", siteOf(4, item.FetchScrape, "https://WWW.CardRush-DM.jp/product-list?keyword={q}"), "", true},
		{"ポート付き", siteOf(5, item.FetchScrape, "https://slist.amiami.jp:443/top/search/list?s_keywords={q}"), "", true},
		{"駿河屋(30 秒間隔・夜間のみ。ユーザー決定 2026-10-04)", siteOf(6, item.FetchScrape, surugayaTemplate), "", true},
		{"未対応ホストの scrape", siteOf(7, item.FetchScrape, "https://shop.example.com/s?q={q}"), "", false},
		{"ドラゴンスター(未確認)", siteOf(8, item.FetchScrape, "https://dorasuta.jp/dm/product-list?keyword={q}"), "", false},
		{"ホストが別物(前方一致では選ばない)", siteOf(9, item.FetchScrape, "https://www.cardrush-dm.jp.evil.example/s?k={q}"), "", false},
		{"読めないテンプレート", siteOf(10, item.FetchScrape, "::not a url::{q}"), "", false},
		{"空のテンプレート", siteOf(11, item.FetchScrape, ""), "", false},
		{"headless のメルカリ", siteOf(12, item.FetchHeadless, mercariTemplate), "", false},
		{"headless でホストが Yahoo!フリマ", siteOf(13, item.FetchHeadless, furimaTemplate), "", false},
		{"link_only でホストがカードラッシュ", siteOf(14, item.FetchLinkOnly, cardrushTemplate), "", false},
		{"link_only の Amazon", siteOf(15, item.FetchLinkOnly, amazonTemplate), "", false},
		{"api は appid なしだと使えない", siteOf(16, item.FetchAPI, shoppingTemplate), "", false},
		{"api は appid ありで使える(ホストは見ない)", siteOf(17, item.FetchAPI, shoppingTemplate), "unit-test-appid", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := fetcher.NewRegistry(fetcher.Config{YahooAppID: c.appid})
			f, ok := r.ForSite(c.site)
			if ok != c.want || (ok && f == nil) {
				t.Errorf("ForSite = %v, %v; want ok=%v", f, ok, c.want)
			}
		})
	}
}

// AC-S11: For(fetch_type) は従来どおり(scrape・headless はホストが分からないので使えない)。
func TestRegistry_ForTypeUnchangedForScrape(t *testing.T) {
	r := fetcher.NewRegistry(fetcher.Config{YahooAppID: "unit-test-appid"})
	for _, ft := range []item.FetchType{item.FetchScrape, item.FetchHeadless, item.FetchLinkOnly} {
		if _, ok := r.For(ft); ok {
			t.Errorf("For(%s) が使える", ft)
		}
	}
}

// AC-S11: NewRegistryWith(テスト用)の ForSite は、fetch_type の表をそのまま使う(ホストは見ない)。link_only は使わない。
func TestRegistryWith_ForSite(t *testing.T) {
	s := &countingFetcher{}
	r := fetcher.NewRegistryWith(map[item.FetchType]fetcher.Fetcher{item.FetchScrape: s, item.FetchLinkOnly: s})
	if f, ok := r.ForSite(siteOf(1, item.FetchScrape, "https://shop.example.com/s?q={q}")); !ok || f != fetcher.Fetcher(s) {
		t.Errorf("scrape: ForSite = %v, %v", f, ok)
	}
	if _, ok := r.ForSite(siteOf(2, item.FetchHeadless, mercariTemplate)); ok {
		t.Error("headless が使える")
	}
	if _, ok := r.ForSite(siteOf(3, item.FetchLinkOnly, amazonTemplate)); ok {
		t.Error("link_only が使える")
	}
}

// AC-S12: 本番の表の scrape は Config.Client を使い、同じサイトを Throttle(Clock・Interval)で 5 秒あける。
// 検索 URL はテンプレートと検索ワードから作られる(本番のホスト名のまま、Host ヘッダで確かめる)。
func TestRegistry_ScrapeUsesClientAndThrottle(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Host+r.RequestURI)
		mu.Unlock()
		// Yahoo!フリマは __NEXT_DATA__ が無いとエラーになる(決めたこと)ので、形の正しい空のページを返す
		if strings.Contains(r.Host, "paypayfleamarket") {
			serveHTML(furimaPage())(w, r)
			return
		}
		serveHTML(`<html><body></body></html>`)(w, r)
	}))
	defer srv.Close()
	clock := newFakeClock()
	r := fetcher.NewRegistry(fetcher.Config{
		Client: &http.Client{Transport: rewriteTransport{target: srv.URL}, Timeout: 10 * time.Second},
		Clock:  clock,
	})
	sites := []fetcher.Site{
		siteOf(1, item.FetchScrape, cardrushTemplate),
		siteOf(2, item.FetchScrape, amiamiTemplate),
		siteOf(3, item.FetchScrape, furimaTemplate),
	}
	for _, s := range sites {
		f, ok := r.ForSite(s)
		if !ok {
			t.Fatalf("ForSite(%s) が使えない", s.SearchURLTemplate)
		}
		// furima は /search/{q} だけ。cardrush・amiami は別ホストなので待たない。
		if _, err := f.Fetch(context.Background(), s, "テスト"); err != nil {
			t.Fatalf("Fetch(%s): %v", s.SearchURLTemplate, err)
		}
	}
	want := []string{
		"www.cardrush-dm.jp/product-list?keyword=" + qTest + "&order=asc&available=1&num=20",
		"slist.amiami.jp/top/search/list?s_keywords=" + qTest + "&s_sortkey=pricea",
		"paypayfleamarket.yahoo.co.jp/search/" + qTest,
	}
	mu.Lock()
	got := slices.Clone(seen)
	mu.Unlock()
	if !slices.Equal(got, want) {
		t.Errorf("リクエスト = %v\nwant %v", got, want)
	}
	if s := clock.takeSleeps(); len(s) != 0 {
		t.Errorf("違うサイトなのに待った: %v", s)
	}
	// 同じサイトの 2 回目は 5 秒あける。
	f, _ := r.ForSite(sites[0])
	if _, err := f.Fetch(context.Background(), sites[0], "テスト"); err != nil {
		t.Fatal(err)
	}
	if s := clock.takeSleeps(); !eqDurations(s, []time.Duration{fetcher.MinInterval}) {
		t.Errorf("待ち = %v, want [5s]", s)
	}
}
