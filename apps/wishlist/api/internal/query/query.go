// Package query は検索ワードの生成と、タイトル照合用の正規化を行う(apps/wishlist/CLAUDE.md §4)。
// 純粋な関数だけを置く(I/O なし)。フロント(TS)にも同じ規則の実装があり、
// testdata/query-cases.json の共通テストベクタで両方を検査する(docs/design.md W-07)。
package query

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Build は検索ワードを作る。優先順位は siteQuery > queryOverride > template。
// 空文字・空白だけの override は「無い」として扱う。template の {name}・{option} は 1 回の走査で置換し
// (置換後の文字列を再置換しない)、結果は連続する空白を半角空白 1 つに詰めて前後の空白を除く。
func Build(template, name, option string, queryOverride, siteQuery *string) string {
	tpl := template
	switch {
	case siteQuery != nil && strings.TrimSpace(*siteQuery) != "":
		return squeeze(*siteQuery)
	case queryOverride != nil && strings.TrimSpace(*queryOverride) != "":
		return squeeze(*queryOverride)
	}
	return squeeze(strings.NewReplacer("{name}", name, "{option}", option).Replace(tpl))
}

// Normalize はタイトル照合用に正規化する。NFKC → 小文字化 → 空白・記号(Unicode の P・S・Z 類と空白)を除去。
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(norm.NFKC.String(s)) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.Is(unicode.Z, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// squeeze は連続する空白を半角空白 1 つに詰め、前後の空白を除く。
func squeeze(s string) string {
	return strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
}
