package item

import (
	"time"
)

// SummaryStatus は商品の目安のまとめの状態(フェーズ5 5-1d。docs/phase5-spec.md)。
type SummaryStatus string

const (
	// SummaryOK は low がある。
	SummaryOK SummaryStatus = "ok"
	// SummaryNoResult は取得済みのサイトがすべて no_result。
	SummaryNoResult SummaryStatus = "no_result"
	// SummaryFailed は low が無く、no_result 以外(failed)のサイトがある。
	SummaryFailed SummaryStatus = "failed"
	// SummaryNone は取得済みのサイトが 1 つも無い。
	SummaryNone SummaryStatus = "none"
)

// Summary は一覧に載せる目安のまとめ(Item.summary)。Low・FetchedAt は GET /api/items/{id}/estimates の
// summary_low・summary_fetched_at と同じ値(estimate.Summarize と同じ規則)。
type Summary struct {
	Low       *int
	Status    SummaryStatus
	FetchedAt *time.Time
}

// SummarizeItem は商品 it の目安のまとめを作る(純粋な関数。MemoryRepository・MySQLRepository の ListItems が使う)。
// 対象は ests のうち、genreSiteIDs(商品のジャンルの site_ids)にあり、it.SiteOverrides で外して(Enabled=false)いないサイトの行だけ。
//   - Low は対象の Low の最小、FetchedAt は Low を持つ対象の FetchedAt のうち最も古いもの(どちらも無ければ nil)
//   - Status は Low があれば ok、対象が無ければ none、対象がすべて no_result なら no_result、それ以外は failed
func SummarizeItem(it Item, genreSiteIDs []int64, ests []Estimate) Summary {
	inGenre := make(map[int64]bool, len(genreSiteIDs))
	for _, id := range genreSiteIDs {
		inGenre[id] = true
	}
	for _, o := range it.SiteOverrides {
		if !o.Enabled {
			delete(inGenre, o.SiteID)
		}
	}
	out := Summary{Status: SummaryNone}
	failed := false
	for _, e := range ests {
		if !inGenre[e.SiteID] {
			continue
		}
		if out.Status == SummaryNone {
			out.Status = SummaryNoResult
		}
		if e.Status != EstimateNoResult {
			failed = true
		}
		if e.Low == nil {
			continue
		}
		if out.Low == nil || *e.Low < *out.Low {
			low := *e.Low
			out.Low = &low
		}
		if out.FetchedAt == nil || e.FetchedAt.Before(*out.FetchedAt) {
			at := e.FetchedAt
			out.FetchedAt = &at
		}
	}
	switch {
	case out.Low != nil:
		out.Status = SummaryOK
	case out.Status != SummaryNone && failed:
		out.Status = SummaryFailed
	}
	return out
}
