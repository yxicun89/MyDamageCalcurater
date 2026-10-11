package memory

import (
	"cmp"
	"context"
	"slices"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/app/items"
	"example.com/pokecalc/apps/wishlist/api/internal/domain/item"
)

var _ items.PriceRepository = (*MemoryRepository)(nil)

func cloneListing(l item.Listing) item.Listing {
	l.ImageURL = item.CloneP(l.ImageURL)
	l.SuspiciousReasons = slices.Clone(l.SuspiciousReasons)
	return l
}

// deletePriceData は商品の目安・出品を消す(外部キーの CASCADE に合わせる)。呼び出し側が m.mu を持つこと。
func (m *MemoryRepository) deletePriceData(itemID int64) {
	for k := range m.estimates {
		if k[0] == itemID {
			delete(m.estimates, k)
		}
	}
	m.listings = slices.DeleteFunc(m.listings, func(l item.Listing) bool { return l.ItemID == itemID })
	for k := range m.history {
		if k.itemID == itemID {
			delete(m.history, k)
		}
	}
}

func (m *MemoryRepository) checkItemSite(itemID, siteID int64) error {
	if _, ok := m.items[itemID]; !ok {
		return item.ErrNotFound
	}
	if _, ok := m.sites[siteID]; !ok {
		return item.ErrSiteNotFound
	}
	return nil
}

// SaveSiteResult は PriceRepository の実装。
func (m *MemoryRepository) SaveSiteResult(_ context.Context, e item.Estimate, ls []item.Listing) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.checkItemSite(e.ItemID, e.SiteID); err != nil {
		return err
	}
	if err := item.ValidateSave(e, ls); err != nil {
		return err
	}
	at := item.StoredTime(e.FetchedAt)
	m.listings = slices.DeleteFunc(m.listings, func(l item.Listing) bool { return l.ItemID == e.ItemID && l.SiteID == e.SiteID })
	for _, l := range ls {
		l = cloneListing(l)
		l.ID, l.ItemID, l.SiteID, l.FetchedAt = m.id(), e.ItemID, e.SiteID, at
		m.listings = append(m.listings, l)
	}
	e.Low, e.Mid, e.FetchedAt = item.CloneP(e.Low), item.CloneP(e.Mid), at
	m.estimates[[2]int64{e.ItemID, e.SiteID}] = e
	m.recordHistory(e)
	return nil
}

// MarkFailed は PriceRepository の実装。
func (m *MemoryRepository) MarkFailed(_ context.Context, itemID, siteID int64, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.checkItemSite(itemID, siteID); err != nil {
		return err
	}
	k := [2]int64{itemID, siteID}
	e, ok := m.estimates[k]
	if !ok {
		e = item.Estimate{ItemID: itemID, SiteID: siteID, FetchedAt: item.StoredTime(at)}
	}
	e.Status = item.EstimateFailed
	m.estimates[k] = e
	return nil
}

// ListEstimates は PriceRepository の実装。
func (m *MemoryRepository) ListEstimates(_ context.Context, itemID int64) ([]item.Estimate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.itemEstimates(itemID), nil
}

// itemEstimates は商品の目安を site_id 昇順で返す(呼び出し側が m.mu を持つ)。
func (m *MemoryRepository) itemEstimates(itemID int64) []item.Estimate {
	out := []item.Estimate{}
	for k, e := range m.estimates {
		if k[0] == itemID {
			e.Low, e.Mid = item.CloneP(e.Low), item.CloneP(e.Mid)
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b item.Estimate) int { return cmp.Compare(a.SiteID, b.SiteID) })
	return out
}

// ListListings は PriceRepository の実装。
func (m *MemoryRepository) ListListings(_ context.Context, itemID int64, siteID *int64) ([]item.Listing, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []item.Listing{}
	for _, l := range m.listings {
		if l.ItemID == itemID && (siteID == nil || l.SiteID == *siteID) {
			out = append(out, cloneListing(l))
		}
	}
	slices.SortFunc(out, func(a, b item.Listing) int { return cmp.Or(cmp.Compare(a.Price, b.Price), cmp.Compare(a.ID, b.ID)) })
	return out, nil
}
