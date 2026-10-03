package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
)

// YahooEndpoint は Yahoo!ショッピング 商品検索 API(V3)。公式ドキュメントで確認済み。
const YahooEndpoint = "https://shopping.yahooapis.jp/ShoppingWebService/V3/itemSearch"

// Yahoo は Yahoo!ショッピングの商品検索 API の Fetcher(fetch_type api)。
//
//	GET {Endpoint}?appid=..&query=..&sort=%2Bprice&results=20
//
// 応答 hits[] の name・price・url・image.small(無ければ exImage.url)・inStock を Listing にする。
// price が 1 未満・name か url が空の hit は除く。MaxListings 件まで。
// 2xx 以外は ErrUpstreamStatus、本文が MaxResponseBytes を超えたら netguard.ErrTooLarge、JSON が壊れていればエラー。
// エラーの文言に appid を含めない(URL を含む *url.Error をそのまま返さない)。AppID が空なら取得せず ErrUnavailable。
type Yahoo struct {
	AppID    string
	Endpoint string       // 空なら YahooEndpoint
	Client   *http.Client // nil なら netguard.NewClient(netguard.Options{})
}

// NewYahoo は Yahoo を返す。
func NewYahoo(appID string, client *http.Client) *Yahoo {
	return &Yahoo{AppID: appID, Client: client}
}

var defaultClient = sync.OnceValue(func() *http.Client { return netguard.NewClient(netguard.Options{}) })

type yahooResponse struct {
	Hits []struct {
		Name  string `json:"name"`
		Price int    `json:"price"`
		URL   string `json:"url"`
		Image struct {
			Small string `json:"small"`
		} `json:"image"`
		ExImage struct {
			URL string `json:"url"`
		} `json:"exImage"`
		InStock bool `json:"inStock"`
	} `json:"hits"`
}

// Fetch は Fetcher の実装。
func (y *Yahoo) Fetch(ctx context.Context, _ Site, query string) ([]Listing, error) {
	if strings.TrimSpace(y.AppID) == "" {
		return nil, ErrUnavailable
	}
	endpoint := y.Endpoint
	if endpoint == "" {
		endpoint = YahooEndpoint
	}
	client := y.Client
	if client == nil {
		client = defaultClient()
	}
	// url.Values.Encode は + を %2B にする(sort=%2Bprice)。
	params := url.Values{
		"appid":   {y.AppID},
		"query":   {query},
		"sort":    {"+price"},
		"results": {strconv.Itoa(MaxListings)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, errors.New("fetcher: yahoo: invalid request")
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		// *url.Error は URL(appid を含む)を文言に持つので、中身のエラーだけを errors.Is できる形で包む。
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("fetcher: yahoo request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%w: yahoo status %d", ErrUpstreamStatus, resp.StatusCode)
	}
	body, err := netguard.ReadLimited(resp.Body, MaxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("fetcher: yahoo read: %w", err)
	}
	var r yahooResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, errors.New("fetcher: yahoo: invalid json response")
	}
	out := make([]Listing, 0, len(r.Hits))
	for _, h := range r.Hits {
		if !validPrice(h.Price) || h.Name == "" || h.URL == "" {
			continue
		}
		img := h.Image.Small
		if img == "" {
			img = h.ExImage.URL
		}
		out = append(out, Listing{Title: h.Name, Price: h.Price, URL: h.URL, ImageURL: img, InStock: h.InStock})
		if len(out) == MaxListings {
			break
		}
	}
	return out, nil
}
