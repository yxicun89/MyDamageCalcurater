package memory

import (
	"cmp"
	"context"
	"slices"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
)

type historyKey struct {
	itemID, siteID int64
	day            int64 // Day の Unix 秒
}

// recordHistory は ok で low がある目安を価格の推移に upsert する。呼び出し側が m.mu を持つこと。
func (m *MemoryRepository) recordHistory(e item.Estimate) {
	if e.Status != item.EstimateOK || e.Low == nil {
		return
	}
	day := item.HistoryDay(e.FetchedAt)
	m.history[historyKey{e.ItemID, e.SiteID, day.Unix()}] = item.PricePoint{
		ItemID: e.ItemID, SiteID: e.SiteID, Day: day, Low: *e.Low, Mid: item.CloneP(e.Mid), Count: e.Count, RecordedAt: item.StoredTime(e.FetchedAt),
	}
}

// ListPriceHistory は PriceHistoryRepository の実装。
func (m *MemoryRepository) ListPriceHistory(_ context.Context, itemID int64, since time.Time) ([]item.PricePoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []item.PricePoint{}
	for k, p := range m.history {
		if k.itemID == itemID && !p.Day.Before(since) {
			p.Mid = item.CloneP(p.Mid)
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b item.PricePoint) int { return cmp.Or(cmp.Compare(a.SiteID, b.SiteID), a.Day.Compare(b.Day)) })
	return out, nil
}

// PrunePriceHistory は PriceHistoryRepository の実装。
func (m *MemoryRepository) PrunePriceHistory(_ context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for k, p := range m.history {
		if p.Day.Before(before) {
			delete(m.history, k)
			n++
		}
	}
	return n, nil
}
