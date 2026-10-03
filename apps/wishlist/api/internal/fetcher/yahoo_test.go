package fetcher_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
)

// テスト用の appid(架空)。エラーの文言に出ないことを確かめる。
const testAppID = "unit-test-appid-should-not-leak"

const yahooPath = "/ShoppingWebService/V3/itemSearch"

var yahooSite = fetcher.Site{ID: 7, Name: "Yahoo!ショッピング", FetchType: item.FetchAPI, IsReference: true}

func allowAll() *http.Client {
	return netguard.NewClient(netguard.Options{AllowAddr: func(netip.Addr) bool { return true }})
}

// yahooServer は fixture を返す httptest サーバー。受けたリクエストの URL を記録する。
type yahooServer struct {
	*httptest.Server
	mu   sync.Mutex
	reqs []*url.URL
}

func newYahooServer(t *testing.T, h func(w http.ResponseWriter, r *http.Request)) *yahooServer {
	t.Helper()
	ys := &yahooServer{}
	ys.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ys.mu.Lock()
		u := *r.URL
		ys.reqs = append(ys.reqs, &u)
		ys.mu.Unlock()
		if r.Method != http.MethodGet || r.URL.Path != yahooPath {
			http.Error(w, "unexpected request", http.StatusTeapot)
			return
		}
		h(w, r)
	}))
	t.Cleanup(ys.Close)
	return ys
}

func (ys *yahooServer) requests() []*url.URL {
	ys.mu.Lock()
	defer ys.mu.Unlock()
	return append([]*url.URL(nil), ys.reqs...)
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

func newTestYahoo(ys *yahooServer) *fetcher.Yahoo {
	return &fetcher.Yahoo{AppID: testAppID, Endpoint: ys.URL + yahooPath, Client: allowAll()}
}

// 公式ドキュメントで確認した URL であること(推測で変えない)。
func TestYahooEndpoint(t *testing.T) {
	if fetcher.YahooEndpoint != "https://shopping.yahooapis.jp/ShoppingWebService/V3/itemSearch" {
		t.Errorf("YahooEndpoint = %q", fetcher.YahooEndpoint)
	}
}

// AC-F3: リクエストは GET ?appid&query&sort=%2Bprice&results=20 だけ(in_stock は付けない)。
func TestYahoo_Request(t *testing.T) {
	ys := newYahooServer(t, serveFile(t, "testdata/yahoo-itemsearch.json"))
	if _, err := newTestYahoo(ys).Fetch(context.Background(), yahooSite, "ボルシャック 銀トレジャー"); err != nil {
		t.Fatal(err)
	}
	reqs := ys.requests()
	if len(reqs) != 1 {
		t.Fatalf("リクエスト = %d 回, want 1", len(reqs))
	}
	u := reqs[0]
	q := u.Query()
	want := url.Values{
		"appid":   {testAppID},
		"query":   {"ボルシャック 銀トレジャー"},
		"sort":    {"+price"},
		"results": {"20"},
	}
	if len(q) != len(want) {
		t.Errorf("パラメータ = %v, want %v", q, want)
	}
	for k, v := range want {
		if q.Get(k) != v[0] || len(q[k]) != 1 {
			t.Errorf("%s = %q, want %q", k, q[k], v[0])
		}
	}
	if !strings.Contains(u.RawQuery, "sort=%2Bprice") {
		t.Errorf("sort の + がエンコードされていない(空白と解釈される): %s", u.RawQuery)
	}
}

// AC-F3: 応答の hits[] を Listing にする。画像は image.small、無ければ exImage.url、どちらも無ければ空。
// price が 1 未満・name が空の hit は除く。順序は応答のまま。
func TestYahoo_Parse(t *testing.T) {
	ys := newYahooServer(t, serveFile(t, "testdata/yahoo-itemsearch.json"))
	got, err := newTestYahoo(ys).Fetch(context.Background(), yahooSite, "q")
	if err != nil {
		t.Fatal(err)
	}
	want := []fetcher.Listing{
		{Title: "架空ストア ボルシャック・ドラゴン 銀トレジャー", Price: 4800, URL: "https://store.example.com/item/1", ImageURL: "https://img.example.com/1_s.jpg", InStock: true},
		{Title: "架空ストア 在庫なしの商品", Price: 5000, URL: "https://store.example.com/item/2", ImageURL: "https://img.example.com/2_ex.jpg", InStock: false},
		{Title: "架空ストア 画像なしの商品", Price: 5200, URL: "https://store.example.com/item/3", ImageURL: "", InStock: true},
	}
	if len(got) != len(want) {
		t.Fatalf("件数 = %d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// AC-F3: 上位 20 件まで(応答に多く入っていても)。
func TestYahoo_Limit(t *testing.T) {
	type hit struct {
		Name    string `json:"name"`
		Price   int    `json:"price"`
		URL     string `json:"url"`
		InStock bool   `json:"inStock"`
	}
	var hits []hit
	for i := range 25 {
		hits = append(hits, hit{Name: fmt.Sprintf("架空の商品 %d", i), Price: 1000 + i, URL: fmt.Sprintf("https://store.example.com/item/%d", i), InStock: true})
	}
	body, _ := json.Marshal(map[string]any{"hits": hits})
	ys := newYahooServer(t, func(w http.ResponseWriter, _ *http.Request) { w.Write(body) })
	got, err := newTestYahoo(ys).Fetch(context.Background(), yahooSite, "q")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != fetcher.MaxListings || got[0].Price != 1000 || got[19].Price != 1019 {
		t.Errorf("件数 = %d(先頭 %v)、want 先頭から 20 件", len(got), got)
	}
}

// AC-F4: 失敗の扱い。2xx 以外 → ErrUpstreamStatus、本文が大きすぎ → netguard.ErrTooLarge、壊れた JSON → エラー、
// 既定のクライアントでループバック → netguard.ErrForbiddenAddress、ctx の取り消し → context.Canceled。
// どのエラーの文言にも appid を含めない。
func TestYahoo_Errors(t *testing.T) {
	big := strings.Repeat(" ", fetcher.MaxResponseBytes+1)
	cases := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		client  func() *http.Client
		ctx     func() context.Context
		want    error // nil なら「何かのエラー」
	}{
		{"500", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "x", http.StatusInternalServerError) }, allowAll, nil, fetcher.ErrUpstreamStatus},
		{"403(appid 不正など)", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "x", http.StatusForbidden) }, allowAll, nil, fetcher.ErrUpstreamStatus},
		{"本文が上限超え", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(`{"hits":[]}` + big)) }, allowAll, nil, netguard.ErrTooLarge},
		{"壊れた JSON", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(`{"hits":[`)) }, allowAll, nil, nil},
		{"既定のクライアントはループバックを拒否", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(`{"hits":[]}`)) }, func() *http.Client { return nil }, nil, netguard.ErrForbiddenAddress},
		{"取り消し", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(`{"hits":[]}`)) }, allowAll, func() context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}, context.Canceled},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ys := newYahooServer(t, c.handler)
			y := &fetcher.Yahoo{AppID: testAppID, Endpoint: ys.URL + yahooPath, Client: c.client()}
			ctx := context.Background()
			if c.ctx != nil {
				ctx = c.ctx()
			}
			got, err := y.Fetch(ctx, yahooSite, "q")
			if err == nil {
				t.Fatalf("エラーにならない: %v", got)
			}
			if c.want != nil && !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
			if strings.Contains(err.Error(), testAppID) {
				t.Errorf("エラーに appid が出ている: %v", err)
			}
		})
	}
}

// AC-F4: appid が空なら取得せず ErrUnavailable。
func TestYahoo_NoAppID(t *testing.T) {
	ys := newYahooServer(t, serveFile(t, "testdata/yahoo-itemsearch.json"))
	y := &fetcher.Yahoo{Endpoint: ys.URL + yahooPath, Client: allowAll()}
	if _, err := y.Fetch(context.Background(), yahooSite, "q"); !errors.Is(err, fetcher.ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
	if n := len(ys.requests()); n != 0 {
		t.Errorf("appid が無いのにリクエストした(%d 回)", n)
	}
}
