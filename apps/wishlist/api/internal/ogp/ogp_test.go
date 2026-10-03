package ogp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"

	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
)

func strp(s string) *string { return &s }

// AC-O1〜O4: HTML fixture(架空の内容)からの下書き。
func TestParse(t *testing.T) {
	base, _ := url.Parse("https://shop.example.com/items/42?ref=x")
	cases := []struct {
		file      string
		wantTitle string
		wantImage *string
	}{
		{"og-full.html", "架空フィギュア ブルーナイト & レッド", strp("https://cdn.example.com/img/blue-knight.jpg")},
		{"og-relative-image.html", "架空カード 銀の竜", strp("https://shop.example.com/images/card.png?size=l")},
		{"title-only.html", "架空プラモデル 1/144 テストガンダム", nil},
		{"empty-og-title.html", "タイトル要素の名前", nil},
		{"no-title.html", "", nil},
		{"bad-image-scheme.html", "架空グッズ", nil},
		{"protocol-relative.html", "name 属性の og:title", strp("https://img.example.net/p.webp")},
		{"og-in-body.html", "最初の og:title", strp("https://cdn.example.com/first.png")},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			f, err := os.Open(filepath.Join("testdata", c.file))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			d, err := Parse(base, f)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if d.Title != c.wantTitle {
				t.Errorf("Title = %q, want %q", d.Title, c.wantTitle)
			}
			switch {
			case c.wantImage == nil && d.ImageURL != nil:
				t.Errorf("ImageURL = %q, want nil", *d.ImageURL)
			case c.wantImage != nil && d.ImageURL == nil:
				t.Errorf("ImageURL = nil, want %q", *c.wantImage)
			case c.wantImage != nil && *d.ImageURL != *c.wantImage:
				t.Errorf("ImageURL = %q, want %q", *d.ImageURL, *c.wantImage)
			}
		})
	}
}

func allowAll(netip.Addr) bool { return true }

func testFetcher() *Fetcher {
	return NewFetcher(netguard.NewClient(netguard.Options{AllowAddr: allowAll}))
}

// AC-O5: 取得して Parse。リダイレクト後の URL を基準に相対 URL を解決する。
func TestFetcher_Draft(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/products/p1", http.StatusFound)
	})
	mux.HandleFunc("/products/p1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, `<html><head><meta property="og:title" content="架空の商品"><meta property="og:image" content="img/a.png"></head></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d, err := testFetcher().Draft(context.Background(), srv.URL+"/start")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "架空の商品" {
		t.Errorf("Title = %q", d.Title)
	}
	if d.ImageURL == nil || *d.ImageURL != srv.URL+"/products/img/a.png" {
		t.Errorf("ImageURL = %v, want %s", d.ImageURL, srv.URL+"/products/img/a.png")
	}
}

// AC-O6: 取得の失敗(2xx 以外・上限超え・URL 違反・禁止アドレス)。
func TestFetcher_Draft_Errors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/404", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("/big", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "<html><head><title>x</title></head><body>")
		io.WriteString(w, strings.Repeat("a", netguard.MaxHTMLBytes))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cases := []struct {
		name string
		f    *Fetcher
		url  string
		want error
	}{
		{"404", testFetcher(), srv.URL + "/404", ErrUpstreamStatus},
		{"上限超え", testFetcher(), srv.URL + "/big", netguard.ErrTooLarge},
		{"ftp", testFetcher(), "ftp://example.com/x", netguard.ErrInvalidURL},
		{"既定のクライアントはループバック拒否", NewFetcher(netguard.NewClient(netguard.Options{})), srv.URL + "/404", netguard.ErrForbiddenAddress},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := c.f.Draft(context.Background(), c.url)
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

// AC-O7: 画像の取得(上限ちょうどは可、超えたら netguard.ErrTooLarge、2xx 以外は ErrUpstreamStatus)。
func TestFetcher_Image(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/img", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "0123456789") })
	mux.HandleFunc("/500", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	b, err := testFetcher().Image(context.Background(), srv.URL+"/img", 10)
	if err != nil || string(b) != "0123456789" {
		t.Errorf("ちょうど上限: %q %v", b, err)
	}
	if _, err := testFetcher().Image(context.Background(), srv.URL+"/img", 9); !errors.Is(err, netguard.ErrTooLarge) {
		t.Errorf("上限超え err = %v", err)
	}
	if _, err := testFetcher().Image(context.Background(), srv.URL+"/500", 10); !errors.Is(err, ErrUpstreamStatus) {
		t.Errorf("500 err = %v", err)
	}
	if _, err := testFetcher().Image(context.Background(), "file:///etc/passwd", 10); !errors.Is(err, netguard.ErrInvalidURL) {
		t.Errorf("file err = %v", err)
	}
}

// AC-O8: Shift_JIS のページ(Content-Type の charset)を UTF-8 として読む。内容は架空。
func TestFetcher_Draft_ShiftJIS(t *testing.T) {
	page := `<html><head><title>架空フィギュア テスト</title></head></html>`
	enc, err := japanese.ShiftJIS.NewEncoder().String(page)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=Shift_JIS")
		io.WriteString(w, enc)
	}))
	defer srv.Close()
	d, err := testFetcher().Draft(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "架空フィギュア テスト" {
		t.Errorf("Title = %q", d.Title)
	}
}
