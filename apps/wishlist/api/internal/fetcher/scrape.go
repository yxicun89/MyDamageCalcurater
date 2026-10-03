package fetcher

// サイト別の scrape Fetcher(docs/phase3-api-spec.md の AC-S*、構造の根拠は docs/sites.md)。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/unicode/norm"

	"example.com/pokecalc/apps/wishlist/api/internal/deeplink"
	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
)

var yenRe = regexp.MustCompile(`[0-9][0-9,]*`)

// ParseYen は「50円」「1,280円」「8,080」「￥500 税込」などの価格表記を整数の円にする(NFKC で半角にし、
// 最初に現れる数字をとる)。数字が無い・1 円未満・大きすぎる場合は ok=false。
func ParseYen(s string) (price int, ok bool) {
	m := yenRe.FindString(norm.NFKC.String(s))
	if m == "" {
		return 0, false
	}
	n, err := strconv.Atoi(strings.ReplaceAll(m, ",", ""))
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// scrapeSite はサイトごとの違い(検索 URL の作り方とページの読み方)。
type scrapeSite struct {
	name string
	// requestURL は検索ページの URL(取得する URL)。
	requestURL func(template, query string) string
	// parse は本文から出品を最大 MaxListings 件まで読む(URL は base 基準で絶対化する)。
	parse func(body []byte, base *url.URL) ([]Listing, error)
}

type scraper struct {
	site   scrapeSite
	client *http.Client
}

func (s scraper) Fetch(ctx context.Context, site Site, query string) ([]Listing, error) {
	client := s.client
	if client == nil {
		client = defaultClient()
	}
	raw := s.site.requestURL(site.SearchURLTemplate, query)
	base, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("fetcher: %s: invalid search url", s.site.name)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, fmt.Errorf("fetcher: %s: invalid request", s.site.name)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetcher: %s request failed: %w", s.site.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%w: %s status %d", ErrUpstreamStatus, s.site.name, resp.StatusCode)
	}
	body, err := netguard.ReadLimited(resp.Body, MaxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("fetcher: %s read: %w", s.site.name, err)
	}
	return s.site.parse(body, base)
}

func buildURL(template, query string) string { return deeplink.Build(template, query) }

// abs は ref を base 基準の絶対 URL にする。読めなければ空。
func abs(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return base.ResolveReference(u).String()
}

func text(s *goquery.Selection) string { return strings.TrimSpace(s.Text()) }

func doc(body []byte) (*goquery.Document, error) {
	return goquery.NewDocumentFromReader(bytes.NewReader(body))
}

// collect は sel の各要素を read で Listing にし、読めた(ok)ものを先頭から MaxListings 件まで集める。
func collect(d *goquery.Document, sel string, read func(*goquery.Selection) (Listing, bool)) []Listing {
	out := []Listing{}
	d.Find(sel).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if l, ok := read(s); ok {
			out = append(out, l)
		}
		return len(out) < MaxListings
	})
	return out
}

var cardrush = scrapeSite{
	name:       "cardrush",
	requestURL: buildURL,
	parse: func(body []byte, base *url.URL) ([]Listing, error) {
		d, err := doc(body)
		if err != nil {
			return nil, errors.New("fetcher: cardrush: invalid html")
		}
		return collect(d, "li.list_item_cell", func(s *goquery.Selection) (Listing, bool) {
			a := s.Find("a.item_data_link").First()
			href, _ := a.Attr("href")
			title := text(s.Find(".item_name .goods_name").First())
			price, ok := ParseYen(text(s.Find(".selling_price .figure").First()))
			u := abs(base, href)
			if !ok || title == "" || u == "" {
				return Listing{}, false
			}
			img, _ := s.Find(".global_photo img").First().Attr("src")
			return Listing{Title: title, Price: price, URL: u, ImageURL: abs(base, img), InStock: cardrushInStock(text(s.Find(".stock").First()))}, true
		}), nil
	},
}

var stockRe = regexp.MustCompile(`在庫数\s*([0-9,]+)`)

// cardrushInStock は「在庫数 N枚」が 0 のときだけ false(読めない・無いは true)。
func cardrushInStock(s string) bool {
	m := stockRe.FindStringSubmatch(norm.NFKC.String(s))
	if m == nil {
		return true
	}
	n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	return err != nil || n != 0
}

var amiami = scrapeSite{
	name:       "amiami",
	requestURL: buildURL,
	parse: func(body []byte, base *url.URL) ([]Listing, error) {
		d, err := doc(body)
		if err != nil {
			return nil, errors.New("fetcher: amiami: invalid html")
		}
		return collect(d, ".product_box", func(s *goquery.Selection) (Listing, bool) {
			href, _ := s.Find("a").First().Attr("href")
			title := text(s.Find(".product_name_inner").First())
			price, ok := ParseYen(text(s.Find(".product_price").First()))
			u := abs(base, href)
			if !ok || title == "" || u == "" {
				return Listing{}, false
			}
			img, _ := s.Find(".product_img img").First().Attr("data-src")
			// 在庫は一覧からは判定できない(sites.md)ので常に true。
			return Listing{Title: title, Price: price, URL: u, ImageURL: abs(base, img), InStock: true}, true
		}), nil
	},
}

var surugaya = scrapeSite{
	name:       "surugaya",
	requestURL: buildURL,
	parse: func(body []byte, base *url.URL) ([]Listing, error) {
		d, err := doc(body)
		if err != nil {
			return nil, errors.New("fetcher: surugaya: invalid html")
		}
		return collect(d, "div.item", func(s *goquery.Selection) (Listing, bool) {
			href, _ := s.Find(".photo_box a").First().Attr("href")
			if href == "" {
				href, _ = s.Find(".title a").First().Attr("href")
			}
			title := text(s.Find(".title h3.product-name").First())
			// 中古の販売価格、無ければ新品の販売価格。どちらも無い(定価だけ・品切れ)商品は除く。
			price, ok := ParseYen(text(s.Find(".item_price .price_teika strong").First()))
			if !ok {
				price, ok = ParseYen(text(s.Find(".item_price .price").First()))
			}
			u := abs(base, href)
			if !ok || title == "" || u == "" {
				return Listing{}, false
			}
			img, _ := s.Find(".photo_box img").First().Attr("src")
			return Listing{Title: title, Price: price, URL: u, ImageURL: abs(base, img), InStock: true}, true
		}), nil
	},
}

// yahooFurima は埋め込み JSON(script#__NEXT_DATA__)を読む。robots.txt が並び替え・絞り込みのパラメータ付き検索を
// 禁じているので、取得するのはクエリと # を外した /search/{q} だけ。
var yahooFurima = scrapeSite{
	name: "yahoofurima",
	requestURL: func(template, query string) string {
		raw := deeplink.Build(template, query)
		if u, err := url.Parse(raw); err == nil {
			u.RawQuery, u.ForceQuery, u.Fragment, u.RawFragment = "", false, "", ""
			return u.String()
		}
		return raw
	},
	parse: func(body []byte, base *url.URL) ([]Listing, error) {
		d, err := doc(body)
		if err != nil {
			return nil, errors.New("fetcher: yahoofurima: invalid html")
		}
		script := d.Find("script#__NEXT_DATA__").First()
		if script.Length() == 0 {
			return nil, errors.New("fetcher: yahoofurima: __NEXT_DATA__ not found")
		}
		var data struct {
			Props struct {
				InitialState struct {
					SearchState struct {
						Search struct {
							Result struct {
								Items *[]struct {
									ID        string  `json:"id"`
									Title     string  `json:"title"`
									Price     float64 `json:"price"`
									Thumbnail string  `json:"thumbnailImageUrl"`
									Status    string  `json:"itemStatus"`
								} `json:"items"`
							} `json:"result"`
						} `json:"search"`
					} `json:"searchState"`
				} `json:"initialState"`
			} `json:"props"`
		}
		if err := json.Unmarshal([]byte(script.Text()), &data); err != nil {
			return nil, errors.New("fetcher: yahoofurima: invalid json")
		}
		items := data.Props.InitialState.SearchState.Search.Result.Items
		if items == nil {
			return nil, errors.New("fetcher: yahoofurima: items not found")
		}
		out := []Listing{}
		for _, it := range *items {
			price := int(it.Price)
			if price < 1 || it.Title == "" || it.ID == "" {
				continue
			}
			out = append(out, Listing{
				Title: it.Title, Price: price, URL: abs(base, "/item/"+url.PathEscape(it.ID)),
				ImageURL: abs(base, it.Thumbnail), InStock: it.Status == "OPEN",
			})
			if len(out) == MaxListings {
				break
			}
		}
		// 関連度順の上位 20 件を、こちらで価格の昇順(安定)に並べる。
		sort.SliceStable(out, func(i, j int) bool { return out[i].Price < out[j].Price })
		return out, nil
	},
}

// NewCardrush はカードラッシュ(cardrush-dm.jp)の scrape Fetcher。client が nil なら netguard の既定。
func NewCardrush(client *http.Client) Fetcher { return scraper{cardrush, client} }

// NewAmiami はあみあみ(slist.amiami.jp)の scrape Fetcher。
func NewAmiami(client *http.Client) Fetcher { return scraper{amiami, client} }

// NewYahooFurima は Yahoo!フリマ(paypayfleamarket.yahoo.co.jp)の scrape Fetcher。パラメータなしの /search/{q} だけを取得する。
func NewYahooFurima(client *http.Client) Fetcher { return scraper{yahooFurima, client} }

// NewSurugaya は駿河屋の scrape Fetcher。登録表には入れない(判断待ち。docs/sites.md)。
func NewSurugaya(client *http.Client) Fetcher { return scraper{surugaya, client} }
