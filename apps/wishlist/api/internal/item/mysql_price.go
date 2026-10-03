package item

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/store"
)

var _ PriceRepository = (*MySQLRepository)(nil)

// SaveSiteResult は PriceRepository の実装。
func (r *MySQLRepository) SaveSiteResult(ctx context.Context, e Estimate, ls []Listing) error {
	if err := validateSave(e, ls); err != nil {
		return err
	}
	at := storedTime(e.FetchedAt) // DATETIME は秒未満を丸めるので、保存前に切り捨てる
	low, err := nullInt32(e.Low)
	if err != nil {
		return err
	}
	mid, err := nullInt32(e.Mid)
	if err != nil {
		return err
	}
	return r.inTx(ctx, func(q *store.Queries) error {
		if err := lockItemAndSite(ctx, q, e.ItemID, e.SiteID); err != nil {
			return err
		}
		if err := q.DeleteListingsBySite(ctx, store.DeleteListingsBySiteParams{ItemID: e.ItemID, SiteID: e.SiteID}); err != nil {
			return err
		}
		for _, l := range ls {
			reasons := l.SuspiciousReasons
			if reasons == nil {
				reasons = []string{}
			}
			raw, err := json.Marshal(reasons)
			if err != nil {
				return err
			}
			if err := q.InsertListing(ctx, store.InsertListingParams{
				ItemID: e.ItemID, SiteID: e.SiteID, Title: l.Title, Price: int32(l.Price), Url: l.URL,
				ImageUrl: nullStr(l.ImageURL), InStock: l.InStock, SuspiciousReasons: raw, FetchedAt: at,
			}); err != nil {
				return err
			}
		}
		return q.UpsertEstimate(ctx, store.UpsertEstimateParams{
			ItemID: e.ItemID, SiteID: e.SiteID, Low: low, Mid: mid, Count: int32(e.Count), SuspiciousCount: int32(e.SuspiciousCount),
			InStockCount: int32(e.InStockCount), Status: store.EstimatesStatus(e.Status), FetchedAt: at,
		})
	})
}

// lockItemAndSite は商品の行をロックして存在を確かめ(無ければ ErrNotFound)、サイトの存在を確かめる(無ければ ErrSiteNotFound)。
func lockItemAndSite(ctx context.Context, q *store.Queries, itemID, siteID int64) error {
	if _, err := q.GetItemForUpdate(ctx, itemID); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if _, err := q.GetSite(ctx, siteID); errors.Is(err, sql.ErrNoRows) {
		return ErrSiteNotFound
	} else if err != nil {
		return err
	}
	return nil
}

// MarkFailed は PriceRepository の実装。
func (r *MySQLRepository) MarkFailed(ctx context.Context, itemID, siteID int64, at time.Time) error {
	return r.inTx(ctx, func(q *store.Queries) error {
		if err := lockItemAndSite(ctx, q, itemID, siteID); err != nil {
			return err
		}
		return q.MarkEstimateFailed(ctx, store.MarkEstimateFailedParams{ItemID: itemID, SiteID: siteID, FetchedAt: storedTime(at)})
	})
}

// ListEstimates は PriceRepository の実装。
func (r *MySQLRepository) ListEstimates(ctx context.Context, itemID int64) ([]Estimate, error) {
	rows, err := r.q.ListEstimatesByItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	out := make([]Estimate, 0, len(rows))
	for _, e := range rows {
		out = append(out, Estimate{
			ItemID: e.ItemID, SiteID: e.SiteID, Low: intPtr(e.Low), Mid: intPtr(e.Mid), Count: int(e.Count),
			SuspiciousCount: int(e.SuspiciousCount), InStockCount: int(e.InStockCount),
			Status: EstimateStatus(e.Status), FetchedAt: e.FetchedAt.UTC(),
		})
	}
	return out, nil
}

// ListListings は PriceRepository の実装。
func (r *MySQLRepository) ListListings(ctx context.Context, itemID int64, siteID *int64) ([]Listing, error) {
	var rows []store.Listing
	if siteID == nil {
		got, err := r.q.ListListingsByItem(ctx, itemID)
		if err != nil {
			return nil, err
		}
		for _, g := range got {
			rows = append(rows, store.Listing(g))
		}
	} else {
		got, err := r.q.ListListingsByItemSite(ctx, store.ListListingsByItemSiteParams{ItemID: itemID, SiteID: *siteID})
		if err != nil {
			return nil, err
		}
		for _, g := range got {
			rows = append(rows, store.Listing(g))
		}
	}
	out := make([]Listing, 0, len(rows))
	for _, l := range rows {
		var reasons []string
		if len(l.SuspiciousReasons) > 0 {
			if err := json.Unmarshal(l.SuspiciousReasons, &reasons); err != nil {
				return nil, err
			}
		}
		out = append(out, Listing{
			ID: l.ID, ItemID: l.ItemID, SiteID: l.SiteID, Title: l.Title, Price: int(l.Price), URL: l.Url,
			ImageURL: strPtr(l.ImageUrl), InStock: l.InStock, SuspiciousReasons: reasons, FetchedAt: l.FetchedAt.UTC(),
		})
	}
	return out, nil
}
