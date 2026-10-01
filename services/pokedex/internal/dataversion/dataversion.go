// Package dataversion は data_versions から dataVersion の文字列を作る
// (内部 API の GET /internal/pokedex/master と pokedex export の metadata.json が同じ値を使う。ADR-0128)。
package dataversion

import (
	"sort"
	"strings"

	"example.com/pokecalc/services/pokedex/internal/store"
)

// checksumPrefixLen は dataVersion に含める checksum(64桁の小文字16進)の先頭の桁数。
const checksumPrefixLen = 8

// String は各行を「source=version@checksum先頭8桁」にし、source 昇順に「,」で連結する(ADR-0128 §1)。
// 入力は書き換えない。行が無ければ空文字列。
func String(versions []store.DataVersion) string {
	sorted := append([]store.DataVersion(nil), versions...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Source < sorted[j].Source })
	parts := make([]string, 0, len(sorted))
	for _, v := range sorted {
		sum := v.Checksum
		if len(sum) > checksumPrefixLen {
			sum = sum[:checksumPrefixLen]
		}
		parts = append(parts, v.Source+"="+v.Version+"@"+sum)
	}
	return strings.Join(parts, ",")
}
