package main

import (
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

const mercariTemplate = "https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc"

// AC-H10: WISHLIST_CHROMIUM_PATH(前後の空白を除く)を読む。無い・空白だけなら空(エラーにしない。Chromium が無い環境でも refresher は動く)。
func TestLoadConfig_ChromiumPath(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"あり", "/headless-shell/headless-shell", "/headless-shell/headless-shell"},
		{"空白を除く", "  /opt/chrome  ", "/opt/chrome"},
		{"空白だけ", "   ", ""},
		{"無し", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := loadConfig(envOf(map[string]string{"WISHLIST_DATABASE_DSN": goodDSN, "WISHLIST_CHROMIUM_PATH": c.in}))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ChromiumPath != c.want {
				t.Errorf("ChromiumPath = %q, want %q", cfg.ChromiumPath, c.want)
			}
		})
	}
}

// AC-H10: ChromiumPath があるときだけメルカリ(headless)を取得できる。作るだけではブラウザを起動しない(存在しないパスでも表は作れる)。
// Chromium が無い環境ではメルカリを取らず、ほかのサイトの取得は従来どおり。
func TestNewRegistry_Headless(t *testing.T) {
	mercari := fetcher.Site{ID: 1, Name: "メルカリ", SearchURLTemplate: mercariTemplate, FetchType: item.FetchHeadless}
	cardrush := fetcher.Site{ID: 2, Name: "カードラッシュ", SearchURLTemplate: "https://www.cardrush-dm.jp/product-list?keyword={q}", FetchType: item.FetchScrape}

	without := newRegistry(config{})
	if _, ok := without.ForSite(mercari); ok {
		t.Error("ChromiumPath が無いのにメルカリを取得できる")
	}
	if _, ok := without.ForSite(cardrush); !ok {
		t.Error("カードラッシュが取得できない")
	}

	with := newRegistry(config{ChromiumPath: "/nonexistent/headless-shell"})
	if _, ok := with.ForSite(mercari); !ok {
		t.Error("ChromiumPath があるのにメルカリを取得できない")
	}
	if !with.NightlyOnly(mercari) {
		t.Error("メルカリが夜間専用でない")
	}
	if _, ok := with.ForSite(cardrush); !ok {
		t.Error("カードラッシュが取得できない")
	}
}
