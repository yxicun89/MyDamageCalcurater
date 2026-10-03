// Package deeplink は各サイトを検索した状態の URL を組み立てる(apps/wishlist/CLAUDE.md §5)。
// 純粋な関数だけを置く。フロント(TS)にも同じ規則の実装があり、testdata/query-cases.json で両方を検査する。
package deeplink

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const hexUpper = "0123456789ABCDEF"

// ErrInvalidTemplate は検索 URL のテンプレートが規則(http/https の絶対 URL で {q} を含む)に合わないこと。
var ErrInvalidTemplate = errors.New("deeplink: invalid search url template")

// Build は template の {q} をすべて、query を JS の encodeURIComponent と同じ規則でエスケープした値に置き換える。
// 置換は 1 回の走査で行う(置換後の値に {q} があっても再置換しない)。query の空白の整形はしない(query.Build の責務)。
func Build(template, query string) string {
	return strings.ReplaceAll(template, "{q}", escape(query))
}

// ValidateTemplate はテンプレートが http/https の絶対 URL(ホストあり)で、{q} を含むことを検査する。
// 違反は ErrInvalidTemplate を包んで返す。
func ValidateTemplate(template string) error {
	u, err := url.Parse(template)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%w: http/https の絶対 URL ではない", ErrInvalidTemplate)
	}
	if !strings.Contains(template, "{q}") {
		return fmt.Errorf("%w: {q} が無い", ErrInvalidTemplate)
	}
	return nil
}

// escape は JS の encodeURIComponent と同じ(A-Za-z0-9 と -_.!~*'() 以外を UTF-8 の %XX にする)。
func escape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.IndexByte("-_.!~*'()", c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexUpper[c>>4])
		b.WriteByte(hexUpper[c&15])
	}
	return b.String()
}
