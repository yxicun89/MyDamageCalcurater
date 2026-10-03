package item

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/oapi-codegen/nullable"
)

// checkUniqueSiteIDs は site_ids に重複が無いことを確かめる(重複は ErrInvalid。DB に書く前に弾く)。
func checkUniqueSiteIDs(ids []int64) error {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			return fmt.Errorf("%w: site_id %d が重複している", ErrInvalid, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func checkUniqueOverrides(os []SiteOverride) error {
	ids := make([]int64, len(os))
	for i, o := range os {
		ids[i] = o.SiteID
	}
	return checkUniqueSiteIDs(ids)
}

// cloneOverrides は SiteOverrides を site_id 昇順の別スライスにコピーする(常に非 nil)。
func cloneOverrides(os []SiteOverride) []SiteOverride {
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

// applyNullable は PATCH の nullable を現在値に適用する(未指定は変えない・null は消す・値は設定)。
func applyNullable[T any](cur *T, n nullable.Nullable[T]) *T {
	if !n.IsSpecified() {
		return cur
	}
	if n.IsNull() {
		return nil
	}
	v := n.MustGet()
	return &v
}
