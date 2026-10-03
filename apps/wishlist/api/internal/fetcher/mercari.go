package fetcher

// メルカリの headless 取得(docs/phase3-api-spec.md の AC-H*、構造の根拠は docs/sites-headless.md)。
// 描画(Renderer・Page)とパース(ParseMercari)を分ける。chromedp の実装は internal/chromium。

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"example.com/pokecalc/apps/wishlist/api/internal/deeplink"
)

const (
	// MercariOrigin は相対 URL(/item/…・/shops/product/…)の前に付けるオリジン。
	MercariOrigin = "https://jp.mercari.com"
	// MercariReadySelector は「描画済みのセル」。スケルトンにも付く item-cell だけでは判定できない。
	MercariReadySelector = `li[data-testid="item-cell"] a[data-testid="thumbnail-link"]`
	// MercariWaitTimeout は最初の thumbnail-link が出るまで待つ上限。
	MercariWaitTimeout = 10 * time.Second
	// MercariScrollBudget はスクロールして描画を待つ全体の上限(Clock で測る)。
	MercariScrollBudget = 10 * time.Second
	// MercariScrollWait は 1 回スクロールしたあとの待ち。
	MercariScrollWait = 500 * time.Millisecond
	// MercariNoGrowthRounds は、描画済みのセル数が増えないスクロールが何回続いたらやめるか。
	MercariNoGrowthRounds = 3
)

// ErrNotRendered は、待っても出品が描画されなかったこと(0 件とは区別する。前回値を残す)。
var ErrNotRendered = errors.New("fetcher: mercari: listings were not rendered")

// Renderer は URL を開いて描画できるブラウザ(本番は internal/chromium。テストは偽物)。
type Renderer interface {
	// OpenPage は新しいページ(タブ)を開く。呼び出し側が必ず Close する。
	OpenPage(ctx context.Context) (Page, error)
}

// Page は描画中の 1 ページ。
type Page interface {
	Navigate(ctx context.Context, rawURL string) error
	// WaitVisible は selector が 1 つ以上見えるまで待つ。ctx の期限で打ち切る。
	WaitVisible(ctx context.Context, selector string) error
	// CountVisible は selector に合う要素の数。
	CountVisible(ctx context.Context, selector string) (int, error)
	ScrollToBottom(ctx context.Context) error
	// HTML は描画後の HTML 全体。
	HTML(ctx context.Context) (string, error)
	Close() error
}

// NewMercari はメルカリの Fetcher。clock が nil なら SystemClock。
func NewMercari(r Renderer, clock Clock) Fetcher {
	if clock == nil {
		clock = SystemClock
	}
	return mercari{r: r, clock: clock}
}

type mercari struct {
	r     Renderer
	clock Clock
}

func (m mercari) Fetch(ctx context.Context, site Site, query string) ([]Listing, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fetcher: mercari: %w", err)
	}
	page, err := m.r.OpenPage(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetcher: mercari: open page: %w", err)
	}
	defer page.Close()

	if err := page.Navigate(ctx, deeplink.Build(site.SearchURLTemplate, query)); err != nil {
		return nil, fmt.Errorf("fetcher: mercari: navigate: %w", err)
	}
	if err := m.waitRendered(ctx, page); err != nil {
		return nil, err
	}
	if err := m.scrollUntilFilled(ctx, page); err != nil {
		return nil, err
	}
	html, err := page.HTML(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetcher: mercari: html: %w", err)
	}
	return ParseMercari([]byte(html))
}

// waitRendered は最初の描画済みセルが出るまで待つ。出なければ ErrNotRendered(取り消しなら ctx の err)。
func (m mercari) waitRendered(ctx context.Context, page Page) error {
	wctx, cancel := context.WithTimeout(ctx, MercariWaitTimeout)
	defer cancel()
	if err := page.WaitVisible(wctx, MercariReadySelector); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return fmt.Errorf("fetcher: mercari: %w", cerr)
		}
		return fmt.Errorf("%w: %w", ErrNotRendered, err)
	}
	return nil
}

// scrollUntilFilled は描画済みセルが MaxListings に届く・増えなくなる・予算を超えるまでスクロールする。
func (m mercari) scrollUntilFilled(ctx context.Context, page Page) error {
	start := m.clock.Now()
	last, stalled := -1, 0
	for {
		n, err := page.CountVisible(ctx, MercariReadySelector)
		if err != nil {
			return fmt.Errorf("fetcher: mercari: count: %w", err)
		}
		if n > last {
			stalled = 0
		} else {
			stalled++
		}
		last = n
		if n >= MaxListings || stalled >= MercariNoGrowthRounds || m.clock.Now().Sub(start) >= MercariScrollBudget {
			return nil
		}
		if err := page.ScrollToBottom(ctx); err != nil {
			return fmt.Errorf("fetcher: mercari: scroll: %w", err)
		}
		if err := m.clock.Sleep(ctx, MercariScrollWait); err != nil {
			return fmt.Errorf("fetcher: mercari: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("fetcher: mercari: %w", err)
		}
	}
}

const thumbnailSuffix = "のサムネイル"

// ParseMercari は描画後の HTML から出品を取り出す(最大 MaxListings 件)。
func ParseMercari(body []byte) ([]Listing, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("fetcher: mercari: parse html: %w", err)
	}
	base, err := url.Parse(MercariOrigin)
	if err != nil {
		return nil, err
	}
	out := []Listing{}
	doc.Find(`li[data-testid="item-cell"]`).EachWithBreak(func(_ int, cell *goquery.Selection) bool {
		if l, ok := parseMercariCell(cell, base); ok {
			out = append(out, l)
		}
		return len(out) < MaxListings
	})
	return out, nil
}

func parseMercariCell(cell *goquery.Selection, base *url.URL) (Listing, bool) {
	a := cell.Find(`a[data-testid="thumbnail-link"]`).First()
	if a.Length() == 0 {
		return Listing{}, false // スケルトン
	}
	href, _ := a.Attr("href")
	href = strings.TrimSpace(href)
	if href == "" {
		return Listing{}, false
	}
	img := a.Find("img").First()
	alt, ok := img.Attr("alt")
	if !ok {
		return Listing{}, false
	}
	title := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(alt), thumbnailSuffix))
	if title == "" {
		return Listing{}, false
	}
	price, ok := ParseYen(cell.Find(`[data-testid="item-tile-price"]`).First().Text())
	if !ok {
		return Listing{}, false
	}
	link := resolveURL(base, href)
	if link == "" {
		return Listing{}, false
	}
	src, _ := img.Attr("src")
	return Listing{Title: title, Price: price, URL: link, ImageURL: resolveURL(base, strings.TrimSpace(src)), InStock: true}, true
}

// resolveURL は ref を base 基準で絶対 URL にする。空・読めないなら空。
func resolveURL(base *url.URL, ref string) string {
	if ref == "" {
		return ""
	}
	u, err := base.Parse(ref)
	if err != nil {
		return ""
	}
	return u.String()
}
