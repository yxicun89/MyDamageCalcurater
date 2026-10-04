package official_test

import (
	"strings"
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/official"
)

// フェーズ4-3 robots.txt の解析(docs/phase4-spec.md AC-O13・O14)。

// AC-O13: ParseRobots・Allowed(表)。Allow・Disallow・コメント・大文字小文字・空の Disallow・グループ・ワイルドカード。
func TestRobots(t *testing.T) {
	const me = official.RobotsProduct
	cases := []struct {
		name  string
		body  string
		path  string
		allow bool
	}{
		// 基本
		{"空の robots.txt は全部許す", "", "/item/1", true},
		{"* の Disallow に前方一致", "User-agent: *\nDisallow: /item/\n", "/item/1", false},
		{"前方一致しなければ許す", "User-agent: *\nDisallow: /item/\n", "/news/1", true},
		{"Disallow: / は全部禁止", "User-agent: *\nDisallow: /\n", "/", false},
		{"空のパスは / として扱う", "User-agent: *\nDisallow: /\n", "", false},
		{"空の Disallow は全部許す", "User-agent: *\nDisallow:\n", "/item/1", true},
		{"空の Disallow(空白だけ)", "User-agent: *\nDisallow:   \n", "/item/1", true},
		{"クエリも含めて前方一致", "User-agent: *\nDisallow: /search?\n", "/search?q=a", false},
		{"クエリなしは一致しない", "User-agent: *\nDisallow: /search?\n", "/search", true},

		// コメントと書式
		{"行末コメント", "User-agent: * # 全員\nDisallow: /item/ # 商品\n", "/item/1", false},
		{"コメント行・空行", "# robots\n\nUser-agent: *\n\n# 商品は禁止\nDisallow: /item/\n", "/item/1", false},
		{"コメントの中の Disallow は効かない", "User-agent: *\n# Disallow: /item/\n", "/item/1", true},
		{"CRLF", "User-agent: *\r\nDisallow: /item/\r\n", "/item/1", false},
		{"項目名の大文字小文字を区別しない", "USER-AGENT: *\ndisallow: /item/\n", "/item/1", false},
		{"コロンの前後の空白", "User-agent :  *\nDisallow :  /item/  \n", "/item/1", false},
		{"パスの大文字小文字は区別する", "User-agent: *\nDisallow: /Item/\n", "/item/1", true},
		{"知らない項目は無視", "User-agent: *\nCrawl-delay: 10\nSitemap: https://a.example/s.xml\nDisallow: /item/\n", "/item/1", false},
		{"最初の User-agent より前の規則は無視", "Disallow: /item/\nUser-agent: *\nDisallow: /news/\n", "/item/1", true},

		// Allow と最長一致
		{"長い Allow が勝つ", "User-agent: *\nDisallow: /item/\nAllow: /item/public/\n", "/item/public/1", true},
		{"長い Disallow が勝つ", "User-agent: *\nAllow: /item/\nDisallow: /item/secret/\n", "/item/secret/1", false},
		{"同じ長さなら Allow", "User-agent: *\nDisallow: /item/\nAllow: /item/\n", "/item/1", true},
		{"空の Allow は規則にしない", "User-agent: *\nAllow:\nDisallow: /item/\n", "/item/1", false},

		// ワイルドカード
		{"* は任意の文字列", "User-agent: *\nDisallow: /*.pdf\n", "/docs/a.pdf", false},
		{"$ は終わり", "User-agent: *\nDisallow: /*.pdf$\n", "/docs/a.pdf?x=1", true},
		{"$ は終わり(一致)", "User-agent: *\nDisallow: /*.pdf$\n", "/docs/a.pdf", false},
		{"/*? はクエリ付きを禁止", "User-agent: *\nDisallow: /*?\n", "/item/1?ref=top", false},

		// グループ
		{"自分の名前のグループ", "User-agent: " + me + "\nDisallow: /item/\n", "/item/1", false},
		{"自分の名前は大文字小文字を区別しない", "User-agent: WISHLIST-Price-Checker\nDisallow: /item/\n", "/item/1", false},
		{"他のボットのグループは効かない", "User-agent: Googlebot\nDisallow: /item/\n", "/item/1", true},
		{"似た名前は別のボット", "User-agent: wishlist\nDisallow: /item/\n", "/item/1", true},
		{"* と自分の両方を使う(* で禁止)", "User-agent: " + me + "\nAllow: /\n\nUser-agent: *\nDisallow: /item/\n", "/item/1", false},
		{"* と自分の両方を使う(自分で禁止)", "User-agent: *\nAllow: /\n\nUser-agent: " + me + "\nDisallow: /item/\n", "/item/1", false},
		{"連続する User-agent は 1 グループ", "User-agent: Googlebot\nUser-agent: *\nDisallow: /item/\n", "/item/1", false},
		{"次の User-agent で新しいグループ", "User-agent: *\nDisallow: /a/\nUser-agent: Googlebot\nDisallow: /item/\n", "/item/1", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := official.ParseRobots(c.body, me)
			if got := r.Allowed(c.path); got != c.allow {
				t.Errorf("Allowed(%q) = %v, want %v\nrobots.txt:\n%s", c.path, got, c.allow, c.body)
			}
		})
	}
}

// AC-O14: AllowAll は何でも許す。RobotsProduct は fetcher.UserAgent の名乗りと同じ。
func TestRobots_AllowAllAndProduct(t *testing.T) {
	for _, p := range []string{"/", "/item/1", "/search?q=a", ""} {
		if !official.AllowAll().Allowed(p) {
			t.Errorf("AllowAll().Allowed(%q) = false", p)
		}
	}
	if official.RobotsProduct != "wishlist-price-checker" || !strings.HasPrefix(fetcher.UserAgent, official.RobotsProduct+"/") {
		t.Errorf("RobotsProduct = %q, UserAgent = %q", official.RobotsProduct, fetcher.UserAgent)
	}
}
