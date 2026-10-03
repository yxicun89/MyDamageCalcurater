package item

import (
	"context"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/store"
)

// recordHistoryTx は ok で low がある目安を価格の推移に upsert する(SaveSiteResult と同じトランザクション)。
func recordHistoryTx(ctx context.Context, q *store.Queries, e Estimate, at time.Time) error {
	if e.Status != EstimateOK || e.Low == nil {
		return nil
	}
	low, err := toInt32(*e.Low)
	if err != nil {
		return err
	}
	mid, err := nullInt32(e.Mid)
	if err != nil {
		return err
	}
	return q.UpsertPriceHistory(ctx, store.UpsertPriceHistoryParams{
		ItemID: e.ItemID, SiteID: e.SiteID, Day: HistoryDay(at), Low: low, Mid: mid, Count: int32(e.Count), RecordedAt: at,
	})
}

// ListPriceHistory は PriceHistoryRepository の実装。
func (r *MySQLRepository) ListPriceHistory(ctx context.Context, itemID int64, since time.Time) ([]PricePoint, error) {
	rows, err := r.q.ListPriceHistory(ctx, store.ListPriceHistoryParams{ItemID: itemID, Day: since})
	if err != nil {
		return nil, err
	}
	out := make([]PricePoint, 0, len(rows))
	for _, p := range rows {
		out = append(out, PricePoint{
			ItemID: p.ItemID, SiteID: p.SiteID, Day: p.Day.UTC(), Low: int(p.Low), Mid: intPtr(p.Mid), Count: int(p.Count), RecordedAt: p.RecordedAt.UTC(),
		})
	}
	return out, nil
}

// PrunePriceHistory は PriceHistoryRepository の実装。
func (r *MySQLRepository) PrunePriceHistory(ctx context.Context, before time.Time) (int64, error) {
	return r.q.PrunePriceHistory(ctx, before)
}
