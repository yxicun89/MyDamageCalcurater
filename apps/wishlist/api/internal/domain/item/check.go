package item

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/oapi-codegen/nullable"

	"example.com/pokecalc/apps/wishlist/api/internal/domain/query"
)

// CheckUniqueSiteIDs は site_ids に重複が無いことを確かめる(重複は ErrInvalid。DB に書く前に弾く)。
func CheckUniqueSiteIDs(ids []int64) error {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			return fmt.Errorf("%w: site_id %d is duplicated", ErrInvalid, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func CheckUniqueOverrides(os []SiteOverride) error {
	ids := make([]int64, len(os))
	for i, o := range os {
		ids[i] = o.SiteID
	}
	return CheckUniqueSiteIDs(ids)
}

// CloneOverrides は SiteOverrides を site_id 昇順の別スライスにコピーする(常に非 nil)。
func CloneOverrides(os []SiteOverride) []SiteOverride {
	out := make([]SiteOverride, len(os))
	for i, o := range os {
		if o.Query != nil {
			q := *o.Query
			o.Query = &q
		}
		out[i] = o
	}
	sortOverrides(out)
	return out
}

func sortOverrides(os []SiteOverride) {
	slices.SortFunc(os, func(a, b SiteOverride) int { return cmp.Compare(a.SiteID, b.SiteID) })
}

// ApplyNullable は PATCH の nullable を現在値に適用する(未指定は変えない・null は消す・値は設定)。
func ApplyNullable[T any](cur *T, n nullable.Nullable[T]) *T {
	if !n.IsSpecified() {
		return cur
	}
	if n.IsNull() {
		return nil
	}
	v := n.MustGet()
	return &v
}

// CheckAliasDuplicates は、正規化後の語がジャンル内で重複しないことを確かめる(重複は ErrInvalid)。
// 空の語・グループの大きさの検査は Service が行う。
func CheckAliasDuplicates(groups [][]string) error {
	seen := map[string]struct{}{}
	for _, g := range groups {
		for _, w := range g {
			n := query.Normalize(w)
			if _, dup := seen[n]; dup {
				return fmt.Errorf("%w: alias %q is duplicated after normalization", ErrInvalid, w)
			}
			seen[n] = struct{}{}
		}
	}
	return nil
}

// CloneAliases は辞書の深いコピー(常に非 nil)。
func CloneAliases(groups [][]string) [][]string {
	out := make([][]string, len(groups))
	for i, g := range groups {
		out[i] = slices.Clone(g)
		if out[i] == nil {
			out[i] = []string{}
		}
	}
	return out
}

// CloneP はポインタの指す値のコピーを指すポインタを返す(nil は nil)。
func CloneP[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
