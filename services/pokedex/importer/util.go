package importer

import (
	"sort"
	"strings"
)

// toID は Showdown/calc の `toID`(小文字化し、英数字以外を落とす)。
func toID(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// pokeAPILookup は PokeAPI の1カテゴリ(species/forms/moves/...)を toID(slug) で引けるようにする。
type pokeAPILookup map[string]map[string]string

func newPokeAPILookup(entries []PokeAPIName) pokeAPILookup {
	idx := make(pokeAPILookup, len(entries))
	for _, e := range entries {
		idx[toID(e.Slug)] = e.Names
	}
	return idx
}

// nameResolution は日本語名の解決結果(ADR-0101 §6)。
type nameResolution struct {
	NameJa string
	Source string // override / pokeapi / fallback_en
}

// resolveJaName は override > PokeAPI(languages の順)> 英語名 の優先順で日本語名を解決する。
// usedOverride には override が実際に使われたキーを集める(override-unused の検出用)。
func resolveJaName(id string, override map[string]string, pokeapiNames map[string]string, languages []string, englishFallback string, usedOverride map[string]bool) nameResolution {
	if v, ok := override[id]; ok {
		if usedOverride != nil {
			usedOverride[id] = true
		}
		return nameResolution{NameJa: v, Source: "override"}
	}
	for _, lang := range languages {
		if v, ok := pokeapiNames[lang]; ok && v != "" {
			return nameResolution{NameJa: v, Source: "pokeapi"}
		}
	}
	return nameResolution{NameJa: englishFallback, Source: "fallback_en"}
}

// sortFindings は Kind/ID/Detail の辞書順に並べ替える(決定性のため)。
func sortFindings(fs []Finding) {
	sort.Slice(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Detail < b.Detail
	})
}

// sortedStringKeys は map[string]... のキーを昇順で返す(決定性のため)。
func sortedKeysRaw[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
