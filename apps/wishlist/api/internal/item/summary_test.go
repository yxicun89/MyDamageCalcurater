package item_test

import (
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// フェーズ5 5-1d: 一覧に載せる目安のまとめ(docs/phase5-spec.md)。
// SummarizeItem は純粋な関数。対象は「ジャンルの site_ids にあり、site_overrides で外していないサイト」の保存済みの目安。

func ip(v int) *int { return &v }

// AC-5-1d-1: status・low・fetched_at の規則。
func TestSummarizeItem(t *testing.T) {
	t1 := time.Date(2026, 10, 10, 3, 0, 0, 0, time.UTC)
	t2 := t1.Add(5 * time.Hour)
	t3 := t1.Add(-48 * time.Hour)
	est := func(site int64, low *int, st item.EstimateStatus, at time.Time) item.Estimate {
		return item.Estimate{ItemID: 1, SiteID: site, Low: low, Status: st, FetchedAt: at}
	}
	it := item.Item{ID: 1}
	sites := []int64{10, 11, 12}
	cases := []struct {
		name       string
		it         item.Item
		siteIDs    []int64
		ests       []item.Estimate
		wantLow    *int
		wantStatus item.SummaryStatus
		wantAt     *time.Time
	}{
		{"取得済みが無い → none", it, sites, nil, nil, item.SummaryNone, nil},
		{"ok が 1 つ", it, sites, []item.Estimate{est(10, ip(4800), item.EstimateOK, t1)}, ip(4800), item.SummaryOK, &t1},
		{"low は最小・fetched_at は low を持つうち最も古い", it, sites,
			[]item.Estimate{est(10, ip(5200), item.EstimateOK, t3), est(11, ip(4800), item.EstimateOK, t2), est(12, nil, item.EstimateNoResult, t1.Add(-100*time.Hour))},
			ip(4800), item.SummaryOK, &t3},
		{"failed でも前回の low があれば ok", it, sites, []item.Estimate{est(10, ip(3000), item.EstimateFailed, t1)}, ip(3000), item.SummaryOK, &t1},
		{"すべて no_result → no_result", it, sites,
			[]item.Estimate{est(10, nil, item.EstimateNoResult, t1), est(11, nil, item.EstimateNoResult, t2)}, nil, item.SummaryNoResult, nil},
		{"low が無く failed がある → failed", it, sites,
			[]item.Estimate{est(10, nil, item.EstimateNoResult, t1), est(11, nil, item.EstimateFailed, t2)}, nil, item.SummaryFailed, nil},
		{"failed だけ → failed", it, sites, []item.Estimate{est(10, nil, item.EstimateFailed, t1)}, nil, item.SummaryFailed, nil},
		{"ジャンルに無いサイトの目安は使わない(→ none)", it, []int64{10},
			[]item.Estimate{est(99, ip(100), item.EstimateOK, t1)}, nil, item.SummaryNone, nil},
		{"ジャンルに無いサイトの安い目安は low に入れない", it, []int64{10},
			[]item.Estimate{est(10, ip(4800), item.EstimateOK, t1), est(99, ip(100), item.EstimateOK, t3)}, ip(4800), item.SummaryOK, &t1},
		{"site_overrides で外したサイトの目安は使わない", item.Item{ID: 1, SiteOverrides: []item.SiteOverride{{SiteID: 11, Enabled: false}}}, sites,
			[]item.Estimate{est(10, nil, item.EstimateNoResult, t1), est(11, ip(100), item.EstimateOK, t2)}, nil, item.SummaryNoResult, nil},
		{"Enabled=true の上書きは対象のまま", item.Item{ID: 1, SiteOverrides: []item.SiteOverride{{SiteID: 11, Enabled: true}}}, sites,
			[]item.Estimate{est(11, ip(100), item.EstimateOK, t2)}, ip(100), item.SummaryOK, &t2},
		{"ジャンルにサイトが無い → none", it, nil, []item.Estimate{est(10, ip(100), item.EstimateOK, t1)}, nil, item.SummaryNone, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := item.SummarizeItem(c.it, c.siteIDs, c.ests)
			if got.Status != c.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, c.wantStatus)
			}
			if !eqIntP(got.Low, c.wantLow) {
				t.Errorf("Low = %v, want %v", fmtP(got.Low), fmtP(c.wantLow))
			}
			switch {
			case (got.FetchedAt == nil) != (c.wantAt == nil):
				t.Errorf("FetchedAt = %v, want %v", got.FetchedAt, c.wantAt)
			case got.FetchedAt != nil && !got.FetchedAt.Equal(*c.wantAt):
				t.Errorf("FetchedAt = %v, want %v", *got.FetchedAt, *c.wantAt)
			}
		})
	}
}

func eqIntP(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func fmtP(p *int) any {
	if p == nil {
		return "nil"
	}
	return *p
}
