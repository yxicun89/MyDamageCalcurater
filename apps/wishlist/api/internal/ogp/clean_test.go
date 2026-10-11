package ogp

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// フェーズ5 5-1c: URL からの下書きの名前・画像(docs/phase5-spec.md)。
// fixture は 2026-10-11 に実サイトから保存し、使う部分だけを残したもの(testdata/*_item.html・dm_card.html)。実サイトには接続しない。

// AC-5-1c-1: 実サイトの fixture から、サイト名の部分を除いた名前と画像を作る(Parse → CleanTitle)。
func TestParseAndClean_Fixtures(t *testing.T) {
	cases := []struct {
		file      string
		pageURL   string
		wantName  string
		wantImage string
	}{
		{"dm_card.html", "https://dm.takaratomy.co.jp/card/detail/?id=dm26rp3-077", "光る！流れる!!ベンザマン!!!",
			"https://dm.takaratomy.co.jp/wp-content/card/cardimage/dm26rp3-077.jpg"},
		{"tamashii_item.html", "https://tamashiiweb.com/item/15889/", "S.H.Figuarts ウルトラマンノア",
			"https://tamashiiweb.com/storage/images/products/thumbnail/b233d0ad-26a1-415f-b088-4223b64af335.webp"},
		{"pokemoncenter_item.html", "https://www.pokemoncenter-online.com/4521329419527.html", "ぬいぐるみ メガアブソル",
			"https://www.pokemoncenter-online.com/a/img/item/4521329419527/M/0b7060865e7b8eade2247b16714af2bbf70a6991284b2d385bfe18c83a4b8dc9.jpg"},
		// bandai-hobby.net の og:image はサイト共通の画像なので、本文の .pg-products__sliderMain の最初の img を使う(&amp; は & に戻す)。
		{"bandai_hobby_item.html", "https://bandai-hobby.net/item/01_7007/", "30MF カスタマイズストラクチャー3",
			"https://d3bk8pkqsprcvh.cloudfront.net/hobby/jp/product/2026/05/cBagENr4n1OGww2A/ChbZ5kNfEnqRBOLP.jpeg?Expires=0&Key-Pair-Id=EXAMPLE&Signature=EXAMPLE"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			f, err := os.Open(filepath.Join("testdata", c.file))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			base, _ := url.Parse(c.pageURL)
			d, err := Parse(base, f)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got := CleanTitle(base.Hostname(), d.Title); got != c.wantName {
				t.Errorf("CleanTitle(%q) = %q, want %q", d.Title, got, c.wantName)
			}
			if d.ImageURL == nil || *d.ImageURL != c.wantImage {
				t.Errorf("ImageURL = %v, want %q", d.ImageURL, c.wantImage)
			}
		})
	}
}

// AC-5-1c-2: bandai-hobby.net で本文に商品画像が無ければ、og:image(サイト共通)を使わず nil。他のホストは今までどおり og:image。
func TestParse_BandaiHobbyImage(t *testing.T) {
	const page = `<html><head><meta property="og:title" content="架空キット｜バンダイ ホビーサイト"><meta property="og:image" content="https://bandai-hobby.net/ogp.png?=ver2"></head>` +
		`<body><img src="https://bandai-hobby.net/images/common/logo.svg">%s</body></html>`
	cases := []struct {
		name, host, body string
		want             *string
	}{
		{"本文に商品画像なし → nil", "bandai-hobby.net", "", nil},
		{"sliderMain の最初の img(相対 URL はページ基準)", "bandai-hobby.net",
			`<div class="pg-products__sliderMain"><div class="swiper-slide"><img src="/img/a.jpg"></div><div class="swiper-slide"><img src="/img/b.jpg"></div></div>`,
			strp("https://bandai-hobby.net/img/a.jpg")},
		{"sliderMain の外の img は使わない", "bandai-hobby.net", `<div class="other"><img src="/img/x.jpg"></div>`, nil},
		{"ホストは大文字小文字を区別しない", "Bandai-Hobby.net",
			`<div class="js-swiper__main pg-products__sliderMain"><img src="https://cdn.example.com/k.jpg"></div>`, strp("https://cdn.example.com/k.jpg")},
		{"他のホストは og:image", "shop.example.com",
			`<div class="pg-products__sliderMain"><img src="/img/a.jpg"></div>`, strp("https://bandai-hobby.net/ogp.png?=ver2")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base, _ := url.Parse("https://" + c.host + "/item/01_0001/")
			d, err := Parse(base, stringsReader(sprintf(page, c.body)))
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case c.want == nil && d.ImageURL != nil:
				t.Errorf("ImageURL = %q, want nil", *d.ImageURL)
			case c.want != nil && (d.ImageURL == nil || *d.ImageURL != *c.want):
				t.Errorf("ImageURL = %v, want %q", d.ImageURL, *c.want)
			}
		})
	}
}

// AC-5-1c-1: 名前の規則(ホストごと・それ以外は末尾の区切り 1 つ)。
func TestCleanTitle(t *testing.T) {
	cases := []struct {
		name, host, in, want string
	}{
		{"デュエマ: サイト名と型番", "dm.takaratomy.co.jp", "光る！流れる!!ベンザマン!!!(DM26RP3 77/77) | デュエル・マスターズ", "光る！流れる!!ベンザマン!!!"},
		{"デュエマ: 型番が無くてもサイト名は除く", "dm.takaratomy.co.jp", "ボルシャック・ドラゴン | デュエル・マスターズ", "ボルシャック・ドラゴン"},
		{"デュエマ: 名前の途中の括弧は残す", "dm.takaratomy.co.jp", "龍(ドラゴン)の王(DM01 1/110) | デュエル・マスターズ", "龍(ドラゴン)の王"},
		{"デュエマ: ホストは大文字小文字を区別しない", "DM.TAKARATOMY.CO.JP", "架空カード(DM99 1/1) | デュエル・マスターズ", "架空カード"},
		{"魂ウェブ", "tamashiiweb.com", "S.H.Figuarts ウルトラマンノア | 商品一覧 | 魂ウェブ", "S.H.Figuarts ウルトラマンノア"},
		{"ポケセン(全角の縦棒)", "www.pokemoncenter-online.com", "ぬいぐるみ メガアブソル ｜【公式】ポケモンセンターオンライン", "ぬいぐるみ メガアブソル"},
		{"バンダイ ホビーサイト(前に空白なし)", "bandai-hobby.net", "30MF カスタマイズストラクチャー3｜バンダイ ホビーサイト", "30MF カスタマイズストラクチャー3"},
		{"それ以外: ' | ' を 1 つだけ除く", "shop.example.com", "架空フィギュア | 架空ショップ", "架空フィギュア"},
		{"それ以外: 最後の区切りだけ除く", "shop.example.com", "A | B | 架空ショップ", "A | B"},
		{"それ以外: ' ｜ '", "shop.example.com", "架空フィギュア ｜ 架空ショップ", "架空フィギュア"},
		{"それ以外: ' - '", "shop.example.com", "架空フィギュア - 架空ショップ", "架空フィギュア"},
		{"それ以外: 区切りが無ければそのまま", "shop.example.com", "架空プラモデル 1/144 テストガンダム", "架空プラモデル 1/144 テストガンダム"},
		{"それ以外: 空白の無い - は区切りでない", "shop.example.com", "RX-78-2 ガンダム", "RX-78-2 ガンダム"},
		{"空はそのまま", "shop.example.com", "", ""},
		{"前後の空白・連続する空白を詰める", "shop.example.com", "  架空  フィギュア  ", "架空 フィギュア"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CleanTitle(c.host, c.in); got != c.want {
				t.Errorf("CleanTitle(%q, %q) = %q, want %q", c.host, c.in, got, c.want)
			}
		})
	}
}

// AC-5-1c-3: 駿河屋の商品ページはページを取りに行かずに下書きを作る(名前は空、画像は photo.php)。それ以外は対象外。
func TestStaticDraft(t *testing.T) {
	cases := []struct {
		raw       string
		ok        bool
		wantImage string
	}{
		{"https://www.suruga-ya.jp/product/detail/602290829", true, "https://www.suruga-ya.jp/database/photo.php?shinaban=602290829&size=l"},
		{"https://www.suruga-ya.jp/product/detail/602290829?tenpo_cd=400427", true, "https://www.suruga-ya.jp/database/photo.php?shinaban=602290829&size=l"},
		{"https://WWW.SURUGA-YA.JP/product/detail/1", true, "https://www.suruga-ya.jp/database/photo.php?shinaban=1&size=l"},
		{"https://www.suruga-ya.jp/product/detail/12a", false, ""},
		{"https://www.suruga-ya.jp/product/detail/", false, ""},
		{"https://www.suruga-ya.jp/search?search_word=x", false, ""},
		{"https://suruga-ya.jp/product/detail/1", false, ""},
		{"https://shop.example.com/product/detail/1", false, ""},
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			u, err := url.Parse(c.raw)
			if err != nil {
				t.Fatal(err)
			}
			d, ok := StaticDraft(u)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if !ok {
				return
			}
			if d.Title != "" {
				t.Errorf("Title = %q, want 空", d.Title)
			}
			if d.ImageURL == nil || *d.ImageURL != c.wantImage {
				t.Errorf("ImageURL = %v, want %q", d.ImageURL, c.wantImage)
			}
		})
	}
}

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }

func sprintf(format string, a ...any) string { return fmt.Sprintf(format, a...) }
