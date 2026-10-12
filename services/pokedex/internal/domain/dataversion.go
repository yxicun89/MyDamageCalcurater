package domain

// data_versions から dataVersion の文字列を作る
// (内部 API の GET /internal/pokedex/master と pokedex export の metadata.json が同じ値を使う。ADR-0128)。

import (
	"sort"
	"strings"
)

// checksumPrefixLen は dataVersion に含める checksum(64桁の小文字16進)の先頭の桁数。
const checksumPrefixLen = 8

// DataVersion は data_versions の1行(取得元・版・checksum)。
type DataVersion struct {
	Source   string
	Version  string
	Checksum string
}

// DataVersionString は各行を「source=version@checksum先頭8桁」にし、source 昇順に「,」で連結する(ADR-0128 §1)。
// 入力は書き換えない。行が無ければ空文字列。
func DataVersionString(versions []DataVersion) string {
	sorted := append([]DataVersion(nil), versions...)
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
