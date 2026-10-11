package fetcher_test

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/adapters/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/adapters/netguard"
	"example.com/pokecalc/apps/wishlist/api/internal/domain/crawl"
	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
)

// Yahoo!ショッピング(フェーズ5 5-1e。docs/phase5-spec.md)。
//
// 以前このファイルは Yahoo!ショッピング API(fetch_type=api・appid)の Fetcher(fetcher.Yahoo)のテストだった
// (TestYahooEndpoint・TestYahoo_Request・TestYahoo_Parse・TestYahoo_Limit・TestYahoo_Errors・TestYahoo_NoAppID)。
// ユーザー決定 2026-10-11 で「appid をやめ、検索ページ(robots.txt で許可された /search/<q>/0/?X=2)の __NEXT_DATA__ を読む」に変わり、
// fetch_type=api の Fetcher は消すことになったので、検索ページの scrape Fetcher(fetcher.NewYahooShopping)のテストに置き換えた。
// 共通の振る舞い(リクエスト・上位 20 件・0 件・失敗)は scrape_test.go の scrapeCases(yahooshopping)でも確かめる。
// fixture は testdata/yahoo_shopping.html(2026-10-11 に保存し、使う項目だけを残したもの)。実サイトには接続しない。

func allowAll() *http.Client {
	return netguard.NewClient(netguard.Options{AllowAddr: func(netip.Addr) bool { return true }})
}

func serveFile(t *testing.T, path string) func(http.ResponseWriter, *http.Request) {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.Write(b)
	}
}

// yahooShoppingWant は fixture の 18 件(サイトの並び = 価格の安い順。X=2)。
// タイトルは name、価格は price(整数の円)、URL は url、画像は image.imageUrl、在庫は true(検索結果に在庫の項目が無い)。
var yahooShoppingWant = []fetcher.Listing{
	{Title: "『中古即納』{FIG} 魂ウェブ限定 S.H.Figuarts 仮面ライダーグリスパーフェクトキングダム ビルド NEW WORLD グリス フィギュア バンダイスピリッツ(20200228)", Price: 7326, URL: "https://store.shopping.yahoo.co.jp/mediaworld-plus/61289450001.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/mediaworld-plus_61289450001?resolution=2x", InStock: true},
	{Title: "『中古即納』{FIG} 魂ウェブ限定 S.H.Figuarts 仮面ライダーグリスパーフェクトキングダム ビルド NEW WORLD グリス フィギュア バンダイスピリッツ(20200228)", Price: 7400, URL: "https://store.shopping.yahoo.co.jp/media-world/61289450001.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/media-world_61289450001?resolution=2x", InStock: true},
	{Title: "『中古即納』{FIG} 魂ウェブ商店限定 S.H.Figuarts 仮面ライダーメタルビルド ビルド NEW WORLD 仮面ライダーグリス フィギュア バンダイスピリッツ(20200831)", Price: 8553, URL: "https://store.shopping.yahoo.co.jp/mediaworld-plus/61280683001.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/mediaworld-plus_61280683001?resolution=2x", InStock: true},
	{Title: "『中古即納』{FIG} 魂ウェブ商店限定 S.H.Figuarts 仮面ライダーメタルビルド ビルド NEW WORLD 仮面ライダーグリス フィギュア バンダイスピリッツ(20200831)", Price: 8640, URL: "https://store.shopping.yahoo.co.jp/media-world/61280683001.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/media-world_61280683001?resolution=2x", InStock: true},
	{Title: "BANDAI SPIRITS◆S.H.Figuarts/仮面プライムローグ/ビルド/グリス/魂ウェブ商店限定/BUILD", Price: 9460, URL: "https://store.shopping.yahoo.co.jp/ssol-shopping/2346510068283.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/ssol-shopping_2346510068283?resolution=2x", InStock: true},
	{Title: "S.H.フィギュアーツ 仮面ライダーグリス パーフェクトキングダム", Price: 9800, URL: "https://store.shopping.yahoo.co.jp/syohoudou/shgreasekingdom.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/syohoudou_shgreasekingdom?resolution=2x", InStock: true},
	{Title: "バンダイ(BANDAI) S.H.Figuarts 仮面ライダーグリスブリザード", Price: 14750, URL: "https://store.shopping.yahoo.co.jp/tnk-store7154/20240607224755-00882us.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/tnk-store7154_20240607224755-00882us?resolution=2x", InStock: true},
	{Title: "バンダイ(BANDAI) S.H.フィギュアーツ 仮面ライダーグリス 『仮面ライダー (中古品)", Price: 15836, URL: "https://store.shopping.yahoo.co.jp/natsumestore/b07dk5kggn.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/natsumestore_b07dk5kggn?resolution=2x", InStock: true},
	{Title: "フィギュアーツZERO 北都 三羽ガラスセット『仮面ライダービルド』　[魂ウェブ商店限定]", Price: 17111, URL: "https://store.shopping.yahoo.co.jp/toyskameta/4573102558091.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/toyskameta_4573102558091?resolution=2x", InStock: true},
	{Title: "S.H.Figuarts 仮面ライダーグリスブリザード", Price: 18623, URL: "https://store.shopping.yahoo.co.jp/signallyjapan-y/b07kckl837.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/signallyjapan-y_b07kckl837?resolution=2x", InStock: true},
	{Title: "S.H.Figuarts 仮面ライダーグリスブリザード", Price: 20186, URL: "https://store.shopping.yahoo.co.jp/jo-5butya/b07kckl837.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/jo-5butya_b07kckl837?resolution=2x", InStock: true},
	{Title: "S.H.Figuarts 仮面ライダーグリスブリザード", Price: 20186, URL: "https://store.shopping.yahoo.co.jp/smartplus-store/b07kckl837.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/smartplus-store_b07kckl837?resolution=2x", InStock: true},
	{Title: "BANDAI SPIRITS(バンダイ スピリッツ) S.H.フィギュアーツ 仮面ライダーグリス パーフェクトキングダム 約145mm AB", Price: 20370, URL: "https://store.shopping.yahoo.co.jp/scarlet2021/20220117160835-00070.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/scarlet2021_20220117160835-00070?resolution=2x", InStock: true},
	{Title: "バンダイ(BANDAI) S.H.Figuarts 仮面ライダーグリスブリザード(中古品)", Price: 20703, URL: "https://store.shopping.yahoo.co.jp/natsumestore/b07kckl837.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/natsumestore_b07kckl837?resolution=2x", InStock: true},
	{Title: "バンダイ(BANDAI) S.H.フィギュアーツ 仮面ライダーグリス 『仮面ライダービルド』(魂ウェブ商店限定)", Price: 21587, URL: "https://store.shopping.yahoo.co.jp/gs-joy/20241120221034-01122us.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/gs-joy_20241120221034-01122us?resolution=2x", InStock: true},
	{Title: "S.H.フィギュアーツ 仮面ライダーグリス 『仮面ライダービルド』(魂ウェブ商店限定)", Price: 21945, URL: "https://store.shopping.yahoo.co.jp/scarlet2021/20220129153312-00242.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/scarlet2021_20220129153312-00242?resolution=2x", InStock: true},
	{Title: "S.H.Figuarts 仮面ライダーグリスブリザード", Price: 24024, URL: "https://store.shopping.yahoo.co.jp/scarlet2021/20220129153312-00037.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/scarlet2021_20220129153312-00037?resolution=2x", InStock: true},
	{Title: "フィギュアーツZERO+S.H.Figuarts 北都の猿渡ファームセット『仮面ライダービルド』　[魂ウェブ商店限定]", Price: 25259, URL: "https://store.shopping.yahoo.co.jp/toyskameta/4573102567857.html?sc_i=shopping-pc-web-result-item-rsltlst-title", ImageURL: "https://item-shopping.c.yimg.jp/i/j/toyskameta_4573102567857?resolution=2x", InStock: true},
}

// shoppingItem は検索結果の 1 件(__NEXT_DATA__ の RESULT ブロックの items の要素)。
func shoppingItem(id, name string, price int) string {
	return fmt.Sprintf(`{"itemId":%q,"url":"https://store.shopping.yahoo.co.jp/s/%s.html","image":{"url":"https://store.shopping.yahoo.co.jp/s/%s.html","imageUrl":"https://item-shopping.c.yimg.jp/i/j/%s"},"name":%q,"price":%d}`, id, id, id, id, name, price)
}

// shoppingPage は検索ページ(PAGER・RESULT・その他のブロックを持つ)。
func shoppingPage(items ...string) string {
	return `<html><body><script id="__NEXT_DATA__" type="application/json">{"props":{"initialState":{"bff":{"searchResults":{"items":{"1":[` +
		`{"type":"PAGER","content":{"next":2}},{"type":"RESULT","content":{"isList":true,"items":[` + strings.Join(items, ",") + `]}},` +
		`{"type":"SALE_PTAH_MODULES","content":{"html":"<div></div>"}}]}}}}}}</script></body></html>`
}

func shoppingSite(u string) fetcher.Site {
	return fetcher.Site{ID: 4, Name: "Yahoo!ショッピング", FetchType: item.FetchScrape, IsReference: true, SearchURLTemplate: u + "/search/{q}/0/?X=2"}
}

// AC-5-1e-1: fixture の 18 件をそのまま(サイトの並びのまま)読む。
func TestYahooShopping_Fixture(t *testing.T) {
	srv := newHTMLServer(t, serveFixture(t, "yahoo_shopping.html"))
	got, err := fetcher.NewYahooShopping(allowAll()).Fetch(context.Background(), shoppingSite(srv.URL), "S.H.Figuarts グリス")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if want := []string{"GET /search/S.H.Figuarts%20%E3%82%B0%E3%83%AA%E3%82%B9/0/?X=2"}; !slices.Equal(srv.requests(), want) {
		t.Errorf("リクエスト = %v, want %v", srv.requests(), want)
	}
	if len(got) != 18 {
		t.Fatalf("件数 = %d, want 18", len(got))
	}
	for i := range got {
		if got[i] != yahooShoppingWant[i] {
			t.Errorf("[%d] = %+v\nwant %+v", i, got[i], yahooShoppingWant[i])
		}
	}
}

// AC-5-1e-1: 価格・名前・URL のどれかが読めない出品は除く(価格は 1〜MaxPrice の整数)。RESULT 以外のブロックは読まない。
func TestYahooShopping_SkipsBrokenEntries(t *testing.T) {
	page := shoppingPage(
		shoppingItem("a", "価格ゼロ", 0),
		shoppingItem("b", "", 100),
		`{"itemId":"c","url":"","image":{"imageUrl":"https://img.example/c"},"name":"URL なし","price":100}`,
		`{"itemId":"d","url":"https://store.shopping.yahoo.co.jp/s/d.html","name":"画像なし","price":200}`,
		shoppingItem("e", "上限超え", fetcher.MaxPrice+1),
		`{"itemId":"f","url":"https://store.shopping.yahoo.co.jp/s/f.html","image":{"imageUrl":"https://img.example/f"},"name":"小数","price":1.5}`,
		shoppingItem("g", "正常", 300),
	)
	srv := newHTMLServer(t, serveHTML(page))
	got, err := fetcher.NewYahooShopping(allowAll()).Fetch(context.Background(), shoppingSite(srv.URL), "x")
	if err != nil {
		t.Fatal(err)
	}
	want := []fetcher.Listing{
		{Title: "画像なし", Price: 200, URL: "https://store.shopping.yahoo.co.jp/s/d.html", ImageURL: "", InStock: true},
		{Title: "正常", Price: 300, URL: "https://store.shopping.yahoo.co.jp/s/g.html", ImageURL: "https://item-shopping.c.yimg.jp/i/j/g", InStock: true},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Listings = %+v\nwant %+v", got, want)
	}
}

// AC-5-1e-1: __NEXT_DATA__ が無い・壊れている・検索結果の場所が無い(構造の変化)はエラー(0 件と区別する)。RESULT が空なら 0 件。
func TestYahooShopping_PageStructure(t *testing.T) {
	bad := map[string]string{
		"no script":     `<html><body>no data</body></html>`,
		"invalid json":  `<html><body><script id="__NEXT_DATA__" type="application/json">{not json</script></body></html>`,
		"no items path": `<html><body><script id="__NEXT_DATA__" type="application/json">{"props":{"initialState":{}}}</script></body></html>`,
	}
	for name, html := range bad {
		t.Run(name, func(t *testing.T) {
			srv := newHTMLServer(t, serveHTML(html))
			got, err := fetcher.NewYahooShopping(allowAll()).Fetch(context.Background(), shoppingSite(srv.URL), "x")
			if err == nil {
				t.Errorf("Fetch = %v, nil; want エラー", got)
			}
		})
	}
	// 実サイトの 0 件のページは searchResults が null で、searchError.errorCode が zeroMatch(2026-10-11 に確認)。出品なし(no_result)にするため 0 件・エラーなし。
	t.Run("zero match fixture", func(t *testing.T) {
		body, err := os.ReadFile("testdata/yahoo_shopping_zero.html")
		if err != nil {
			t.Fatal(err)
		}
		srv := newHTMLServer(t, serveHTML(string(body)))
		got, err := fetcher.NewYahooShopping(allowAll()).Fetch(context.Background(), shoppingSite(srv.URL), "x")
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("Fetch = %#v, %v; want 空(nil でない)・エラーなし", got, err)
		}
	})
	t.Run("other search error", func(t *testing.T) {
		html := `<html><body><script id="__NEXT_DATA__" type="application/json">{"props":{"initialState":{"bff":{"searchResults":null,"searchError":{"errorCode":"systemError"}}}}}</script></body></html>`
		srv := newHTMLServer(t, serveHTML(html))
		if got, err := fetcher.NewYahooShopping(allowAll()).Fetch(context.Background(), shoppingSite(srv.URL), "x"); err == nil {
			t.Errorf("Fetch = %v, nil; want エラー(0 件と区別する)", got)
		}
	})
	t.Run("empty result", func(t *testing.T) {
		srv := newHTMLServer(t, serveHTML(shoppingPage()))
		got, err := fetcher.NewYahooShopping(allowAll()).Fetch(context.Background(), shoppingSite(srv.URL), "x")
		if err != nil || len(got) != 0 {
			t.Errorf("Fetch = %v, %v; want 空・エラーなし", got, err)
		}
	})
}

// AC-5-1e-2: 本番の表で Yahoo!ショッピング(shopping.yahoo.co.jp・scrape)は検索ページの Fetcher。appid は要らない。
// 同じホストは 5 秒あける(駿河屋のような長い間隔・夜間だけにはしない。詳細シートを開いたときにも取る)。fetch_type=api は使わない。
func TestRegistry_YahooShopping(t *testing.T) {
	var hosts []string
	srv := newHTMLServer(t, func(w http.ResponseWriter, r *http.Request) {
		hosts = append(hosts, r.Host+r.RequestURI)
		serveHTML(shoppingPage(shoppingItem("a", "架空の商品", 1200)))(w, r)
	})
	clock := newFakeClock()
	r := fetcher.NewRegistry(fetcher.Config{
		Client: &http.Client{Transport: rewriteTransport{target: srv.URL}, Timeout: 10 * time.Second},
		Clock:  clock,
	})
	site := siteOf(4, item.FetchScrape, shoppingTemplate)
	f, ok := r.ForSite(site)
	if !ok {
		t.Fatal("Yahoo!ショッピング(scrape)の Fetcher が無い")
	}
	if r.NightlyOnly(site) {
		t.Error("Yahoo!ショッピングが夜間だけになっている")
	}
	for i := 0; i < 2; i++ {
		got, err := f.Fetch(context.Background(), site, "テスト")
		if err != nil || len(got) != 1 || got[0].Price != 1200 {
			t.Fatalf("Fetch = %+v, %v", got, err)
		}
	}
	want := "shopping.yahoo.co.jp/search/" + qTest + "/0/?X=2"
	if len(hosts) != 2 || hosts[0] != want {
		t.Errorf("リクエスト = %v, want [%s ×2]", hosts, want)
	}
	if s := clock.takeSleeps(); !eqDurations(s, []time.Duration{crawl.MinInterval}) {
		t.Errorf("待ち = %v, want [5s]", s)
	}
	if _, ok := r.ForSite(siteOf(5, item.FetchAPI, shoppingTemplate)); ok {
		t.Error("fetch_type=api の Fetcher が残っている(5-1e で消す)")
	}
	if _, ok := r.For(item.FetchAPI); ok {
		t.Error("For(api) が使える(5-1e で消す)")
	}
}
