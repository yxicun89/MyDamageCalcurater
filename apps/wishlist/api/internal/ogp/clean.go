package ogp

import (
	"net/url"
	"regexp"
	"strings"
)

// CleanTitle は og:title(または <title>)から、サイト名の部分を除いた商品名を返す(フェーズ5 5-1c。docs/phase5-spec.md)。
// host は商品ページの URL のホスト名(大文字小文字は区別しない)。純粋な関数。
//   - dm.takaratomy.co.jp: 末尾の ` | デュエル・マスターズ` と、その直前の `(型番 番号/番号)`
//   - tamashiiweb.com: 末尾の ` | 商品一覧 | 魂ウェブ`
//   - www.pokemoncenter-online.com: 末尾の ` ｜【公式】ポケモンセンターオンライン`
//   - bandai-hobby.net: 末尾の `｜バンダイ ホビーサイト`
//   - それ以外: 末尾の ` | …`・` ｜ …`・` - …` を 1 つだけ除く(除いて空になるなら除かない)
//
// 結果は前後の空白を除き、連続する空白を 1 つに詰める。
func CleanTitle(host, title string) string {
	t := collapse(title)
	var cut string
	switch strings.ToLower(host) {
	case "dm.takaratomy.co.jp":
		cut = strings.TrimSuffix(t, " | デュエル・マスターズ")
		if cut != t {
			cut = dmModelNumber.ReplaceAllString(cut, "")
		}
	case "tamashiiweb.com":
		cut = strings.TrimSuffix(t, " | 商品一覧 | 魂ウェブ")
	case "www.pokemoncenter-online.com":
		cut = strings.TrimSuffix(t, " ｜【公式】ポケモンセンターオンライン")
	case "bandai-hobby.net":
		cut = strings.TrimSuffix(t, "｜バンダイ ホビーサイト")
	default:
		cut = t
		idx := -1
		for _, sep := range []string{" | ", " ｜ ", " - "} {
			if i := strings.LastIndex(t, sep); i > idx {
				idx = i
			}
		}
		if idx >= 0 {
			cut = t[:idx]
		}
	}
	if cut = strings.TrimSpace(cut); cut == "" {
		return t
	}
	return cut
}

// dmModelNumber は dm.takaratomy.co.jp の名前の末尾の `(型番 番号/番号)`。
var dmModelNumber = regexp.MustCompile(`\s*[(（][^()（）]*\d+/\d+[)）]$`)

// StaticDraft はページを取りに行かずに作れる下書きを返す(フェーズ5 5-1c)。対象でなければ ok=false。
//   - www.suruga-ya.jp・suruga-ya.jp の /product/detail/<番号>(番号は数字だけ。末尾の / は可): 名前は空文字、画像は
//     https://www.suruga-ya.jp/database/photo.php?shinaban=<番号>&size=l(商品ページは Cloudflare の確認画面で読めない。回避しない)
func StaticDraft(u *url.URL) (Draft, bool) {
	if h := strings.ToLower(u.Hostname()); h != "www.suruga-ya.jp" && h != "suruga-ya.jp" {
		return Draft{}, false
	}
	num, ok := strings.CutPrefix(u.Path, "/product/detail/")
	num = strings.TrimSuffix(num, "/")
	if !ok || num == "" || strings.Trim(num, "0123456789") != "" {
		return Draft{}, false
	}
	img := "https://www.suruga-ya.jp/database/photo.php?shinaban=" + num + "&size=l"
	return Draft{ImageURL: &img}, true
}
