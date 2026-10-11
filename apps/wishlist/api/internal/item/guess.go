package item

import (
	"strings"
	"unicode/utf8"

	"example.com/pokecalc/apps/wishlist/api/internal/query"
)

// GuessGenre は商品名 name と URL のホスト host からジャンルを推定する(フェーズ5 5-1c。docs/phase5-spec.md)。純粋な関数。
//  1. name を query.Normalize した文字列が、MatchKeywords の語(query.Normalize したもの。空になる語は使わない)を含むジャンル。
//     複数当たれば、当たった語(正規化後)が最も長い(rune 数)もの、同じ長さなら SortOrder が小さいもの、さらに同じなら ID が小さいもの
//  2. 1 で決まらなければ、host(大文字小文字・末尾の . は無視)が MatchHosts のどれかと一致するジャンル。
//     複数なら SortOrder が小さいもの、同じなら ID が小さいもの
//
// 決まらなければ (0, false)。genres の並びには依らない。
func GuessGenre(genres []Genre, name, host string) (int64, bool) {
	n := query.Normalize(name)
	var best *Genre
	bestLen := 0
	for i := range genres {
		g := &genres[i]
		l := 0
		for _, kw := range g.MatchKeywords {
			k := query.Normalize(kw)
			if k == "" || !strings.Contains(n, k) {
				continue
			}
			if r := utf8.RuneCountInString(k); r > l {
				l = r
			}
		}
		if l == 0 {
			continue
		}
		if best == nil || l > bestLen || (l == bestLen && earlier(g, best)) {
			best, bestLen = g, l
		}
	}
	if best != nil {
		return best.ID, true
	}
	h := strings.TrimSuffix(strings.ToLower(host), ".")
	if h == "" {
		return 0, false
	}
	for i := range genres {
		g := &genres[i]
		for _, mh := range g.MatchHosts {
			if strings.ToLower(strings.TrimSuffix(mh, ".")) == h && (best == nil || earlier(g, best)) {
				best = g
			}
		}
	}
	if best != nil {
		return best.ID, true
	}
	return 0, false
}

// earlier は a が b より優先か(SortOrder、同じなら ID が小さい方)。
func earlier(a, b *Genre) bool {
	if a.SortOrder != b.SortOrder {
		return a.SortOrder < b.SortOrder
	}
	return a.ID < b.ID
}
