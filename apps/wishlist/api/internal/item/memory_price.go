package item

import (
	"cmp"
	"context"
	"slices"
	"time"
)

var _ PriceRepository = (*MemoryRepository)(nil)

func cloneListing(l Listing) Listing {
	l.ImageURL = cloneP(l.ImageURL)
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
	m.listings = slices.DeleteFunc(m.listings, func(l Listing) bool { return l.ItemID == itemID })
	for k := range m.history {
		if k.itemID == itemID {
			delete(m.history, k)
		}
	}
}

func (m *MemoryRepository) checkItemSite(itemID, siteID int64) error {
	if _, ok := m.items[itemID]; !ok {
		return ErrNotFound
	}
	if _, ok := m.sites[siteID]; !ok {
		return ErrSiteNotFound
	}
	return nil
}

// SaveSiteResult は PriceRepository の実装。
func (m *MemoryRepository) SaveSiteResult(_ context.Context, e Estimate, ls []Listing) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.checkItemSite(e.ItemID, e.SiteID); err != nil {
		return err
	}
	if err := validateSave(e, ls); err != nil {
		return err
	}
	at := storedTime(e.FetchedAt)
	m.listings = slices.DeleteFunc(m.listings, func(l Listing) bool { return l.ItemID == e.ItemID && l.SiteID == e.SiteID })
	for _, l := range ls {
		l = cloneListing(l)
		l.ID, l.ItemID, l.SiteID, l.FetchedAt = m.id(), e.ItemID, e.SiteID, at
		m.listings = append(m.listings, l)
	}
	e.Low, e.Mid, e.FetchedAt = cloneP(e.Low), cloneP(e.Mid), at
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
		e = Estimate{ItemID: itemID, SiteID: siteID, FetchedAt: storedTime(at)}
	}
	e.Status = EstimateFailed
	m.estimates[k] = e
	return nil
}

// ListEstimates は PriceRepository の実装。
func (m *MemoryRepository) ListEstimates(_ context.Context, itemID int64) ([]Estimate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Estimate{}
	for k, e := range m.estimates {
		if k[0] == itemID {
			e.Low, e.Mid = cloneP(e.Low), cloneP(e.Mid)
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b Estimate) int { return cmp.Compare(a.SiteID, b.SiteID) })
	return out, nil
}

// ListListings は PriceRepository の実装。
func (m *MemoryRepository) ListListings(_ context.Context, itemID int64, siteID *int64) ([]Listing, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Listing{}
	for _, l := range m.listings {
		if l.ItemID == itemID && (siteID == nil || l.SiteID == *siteID) {
			out = append(out, cloneListing(l))
		}
	}
	slices.SortFunc(out, func(a, b Listing) int { return cmp.Or(cmp.Compare(a.Price, b.Price), cmp.Compare(a.ID, b.ID)) })
	return out, nil
}
