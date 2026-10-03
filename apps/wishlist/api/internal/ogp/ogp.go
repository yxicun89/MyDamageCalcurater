// Package ogp は商品ページの URL から登録の下書き(名前・画像 URL)を作る(apps/wishlist/CLAUDE.md §8 from-url)。
// 取得は netguard の SSRF 対策をしたクライアントで行う(docs/design.md W-05)。
package ogp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
)

const userAgent = "wishlist-ogp/1.0"

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
func Parse(baseURL *url.URL, r io.Reader) (Draft, error) {
	var ogTitle, ogImage *string
	var docTitle string
	haveDocTitle := false
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			if err := z.Err(); err != io.EOF {
				return Draft{}, err
			}
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		tn, hasAttr := z.TagName()
		switch string(tn) {
		case "title":
			if tt == html.StartTagToken && !haveDocTitle && z.Next() == html.TextToken {
				docTitle = string(z.Text())
				haveDocTitle = true
			}
		case "meta":
			if !hasAttr {
				continue
			}
			var prop, name, content string
			for {
				k, v, more := z.TagAttr()
				switch string(k) {
				case "property":
					prop = string(v)
				case "name":
					name = string(v)
				case "content":
					content = string(v)
				}
				if !more {
					break
				}
			}
			key := prop
			if key == "" {
				key = name
			}
			c := content
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "og:title":
				if ogTitle == nil {
					ogTitle = &c
				}
			case "og:image":
				if ogImage == nil {
					ogImage = &c
				}
			}
		}
	}
	var d Draft
	if ogTitle != nil {
		d.Title = collapse(*ogTitle)
	}
	if d.Title == "" {
		d.Title = collapse(docTitle)
	}
	if ogImage != nil {
		if ref := strings.TrimSpace(*ogImage); ref != "" {
			if u, err := baseURL.Parse(ref); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" {
				s := u.String()
				d.ImageURL = &s
			}
		}
	}
	return d, nil
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
	body, final, err := f.get(ctx, rawURL, netguard.MaxHTMLBytes)
	if err != nil {
		return Draft{}, err
	}
	return Parse(final, strings.NewReader(string(body)))
}

// Image は rawURL の画像を最大 max バイト読んで返す。形式の判定はしない(storage.Save が行う)。
// エラーは Draft と同じ規則。
func (f *Fetcher) Image(ctx context.Context, rawURL string, max int64) ([]byte, error) {
	b, _, err := f.get(ctx, rawURL, max)
	return b, err
}

// get は rawURL を取得し、本文(最大 max バイト)と最終 URL(リダイレクト後)を返す。
func (f *Fetcher) get(ctx context.Context, rawURL string, max int64) ([]byte, *url.URL, error) {
	u, err := netguard.ValidateURL(rawURL)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", netguard.ErrInvalidURL, err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, nil, fmt.Errorf("%w: %d", ErrUpstreamStatus, resp.StatusCode)
	}
	b, err := netguard.ReadLimited(resp.Body, max)
	if err != nil {
		return nil, nil, err
	}
	return b, resp.Request.URL, nil
}

// collapse は前後の空白を除き、連続する空白を半角空白 1 つに詰める。
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
