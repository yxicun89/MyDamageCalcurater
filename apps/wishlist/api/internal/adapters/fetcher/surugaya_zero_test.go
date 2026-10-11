package fetcher_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/adapters/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
)

// 駿河屋は検索結果が 0 件のとき、本文に「検索結果は０件です。」を出した検索ページを HTTP 404 で返す(2026-10-11 に実サイトで確認)。
// 出品なし(no_result)にするため、その 404 は 0 件・エラーなしとして扱う。本文にその表示が無い 404 は今までどおりエラー。
func TestSurugaya_ZeroHit404(t *testing.T) {
	zero, err := os.ReadFile("testdata/surugaya_zero.html")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		body    string
		wantErr bool
	}{
		"0 件の表示がある 404": {string(zero), false},
		"ほかの 404":       {`<html><body>Not Found</body></html>`, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			srv := newHTMLServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=UTF-8")
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(c.body))
			})
			site := fetcher.Site{ID: 6, Name: "駿河屋", SearchURLTemplate: strings.ReplaceAll(surugayaTemplate, "https://www.suruga-ya.jp", srv.URL), FetchType: item.FetchScrape}
			got, err := fetcher.NewSurugaya(allowAll()).Fetch(context.Background(), site, "S.H.Figuarts ウルトラマンノア")
			if c.wantErr {
				if err == nil {
					t.Errorf("Fetch = %v, nil; want エラー", got)
				}
				return
			}
			if err != nil || got == nil || len(got) != 0 {
				t.Errorf("Fetch = %#v, %v; want 空(nil でない)・エラーなし", got, err)
			}
		})
	}
}
