// Package ogp は商品ページの URL から登録の下書き(名前・画像 URL)を作る(apps/wishlist/CLAUDE.md §8 from-url)。
// 取得は netguard の SSRF 対策をしたクライアントで行う(docs/design.md W-05)。
package ogp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
)

// ErrUpstreamStatus は取得先が 2xx 以外を返したこと(API では 502)。
var ErrUpstreamStatus = errors.New("ogp: upstream returned non-2xx status")

// Draft は OGP から読み取った下書き。
type Draft struct {
	// Title は og:title(無い・空白だけなら <title>)。前後の空白を除き、連続する空白は 1 つに詰める。どちらも無ければ空文字。
	Title string
	// ImageURL は og:image を baseURL で解決した絶対 URL。無い・空・http/https 以外なら nil。
	ImageURL *string
}

// Parse は HTML から Draft を作る。og:title・og:image は property 属性(無ければ name 属性)で探し、
// 複数あれば最初のものを使う。<head> の外にあっても読む。読み込みエラー以外では失敗しない。
func Parse(baseURL *url.URL, html io.Reader) (Draft, error) {
	panic("TODO: ogp.Parse")
}

// Fetcher は外部 URL から下書きと画像を取る。
type Fetcher struct {
	client *http.Client
}

// NewFetcher は client(netguard.NewClient で作ったもの)で取得する Fetcher を返す。
func NewFetcher(client *http.Client) *Fetcher {
	return &Fetcher{client: client}
}

// Draft は rawURL(netguard.ValidateURL で検査する)の HTML を最大 netguard.MaxHTMLBytes 読み、Parse する。
// 相対 URL はリダイレクト後の最終 URL を基準に解決する。2xx 以外は ErrUpstreamStatus、
// 上限超えは netguard.ErrTooLarge、URL 違反は netguard.ErrInvalidURL を包んで返す。
func (f *Fetcher) Draft(ctx context.Context, rawURL string) (Draft, error) {
	panic("TODO: Fetcher.Draft")
}

// Image は rawURL の画像を最大 max バイト読んで返す。形式の判定はしない(storage.Save が行う)。
// エラーは Draft と同じ規則。
func (f *Fetcher) Image(ctx context.Context, rawURL string, max int64) ([]byte, error) {
	panic("TODO: Fetcher.Image")
}
