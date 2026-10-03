package item_test

import (
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// AC-H1(docs/phase4-spec.md): HistoryDay は JST の日付を 00:00 UTC の time.Time で返す(どのタイムゾーンで渡しても同じ時点なら同じ日)。
func TestHistoryDay(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	cases := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{"JST の昼", time.Date(2026, 10, 3, 12, 0, 0, 0, jst), day(2026, 10, 3)},
		{"UTC 14:59 は JST 23:59 で同じ日", time.Date(2026, 10, 3, 14, 59, 59, 999_000_000, time.UTC), day(2026, 10, 3)},
		{"UTC 15:00 は JST の翌日 0:00", time.Date(2026, 10, 3, 15, 0, 0, 0, time.UTC), day(2026, 10, 4)},
		{"UTC の 0 時は JST 9 時で同じ日", time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), day(2026, 10, 3)},
		{"年またぎ", time.Date(2026, 12, 31, 15, 30, 0, 0, time.UTC), day(2027, 1, 1)},
		{"別のタイムゾーン(UTC-8)", time.Date(2026, 10, 3, 8, 0, 0, 0, time.FixedZone("PST", -8*60*60)), day(2026, 10, 4)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := item.HistoryDay(c.in)
			if !got.Equal(c.want) || got.Location() != time.UTC {
				t.Errorf("HistoryDay(%v) = %v (%v), want %v UTC", c.in, got, got.Location(), c.want)
			}
		})
	}
}
