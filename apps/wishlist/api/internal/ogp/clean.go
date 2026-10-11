package ogp

import "net/url"

// CleanTitle は og:title(または <title>)から、サイト名の部分を除いた商品名を返す(フェーズ5 5-1c。docs/phase5-spec.md)。
// host は商品ページの URL のホスト名(大文字小文字は区別しない)。純粋な関数。
//   - dm.takaratomy.co.jp: 末尾の ` | デュエル・マスターズ` と、その直前の `(型番 番号/番号)`
//   - tamashiiweb.com: 末尾の ` | 商品一覧 | 魂ウェブ`
//   - www.pokemoncenter-online.com: 末尾の ` ｜【公式】ポケモンセンターオンライン`
//   - bandai-hobby.net: 末尾の `｜バンダイ ホビーサイト`
//   - それ以外: 末尾の ` | …`・` ｜ …`・` - …` を 1 つだけ除く(除いて空になるなら除かない)
//
// 結果は前後の空白を除き、連続する空白を 1 つに詰める。
//
// TODO(5-1c 実装者): 未実装。spec-writer が置いた空の実装(clean_test.go が失敗する)。
func CleanTitle(host, title string) string {
	_ = host
	return title
}

// StaticDraft はページを取りに行かずに作れる下書きを返す(フェーズ5 5-1c)。対象でなければ ok=false。
//   - www.suruga-ya.jp の /product/detail/<番号>(番号は数字だけ): 名前は空文字、画像は
//     https://www.suruga-ya.jp/database/photo.php?shinaban=<番号>&size=l(商品ページは Cloudflare の確認画面で読めない。回避しない)
//
// TODO(5-1c 実装者): 未実装。spec-writer が置いた空の実装(clean_test.go が失敗する)。
func StaticDraft(u *url.URL) (Draft, bool) {
	_ = u
	return Draft{}, false
}
