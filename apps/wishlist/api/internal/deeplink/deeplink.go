// Package deeplink は各サイトを検索した状態の URL を組み立てる(apps/wishlist/CLAUDE.md §5)。
// 純粋な関数だけを置く。フロント(TS)にも同じ規則の実装があり、testdata/query-cases.json で両方を検査する。
package deeplink

import "errors"

// ErrInvalidTemplate は検索 URL のテンプレートが規則(http/https の絶対 URL で {q} を含む)に合わないこと。
var ErrInvalidTemplate = errors.New("deeplink: invalid search url template")

// Build は template の {q} をすべて、query を JS の encodeURIComponent と同じ規則でエスケープした値に置き換える。
// 置換は 1 回の走査で行う(置換後の値に {q} があっても再置換しない)。query の空白の整形はしない(query.Build の責務)。
func Build(template, query string) string {
	panic("TODO: deeplink.Build")
}

// ValidateTemplate はテンプレートが http/https の絶対 URL(ホストあり)で、{q} を含むことを検査する。
// 違反は ErrInvalidTemplate を包んで返す。
func ValidateTemplate(template string) error {
	panic("TODO: deeplink.ValidateTemplate")
}
