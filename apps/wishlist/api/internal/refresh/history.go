package refresh

import (
	"context"
	"slices"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// フェーズ4-2 価格の推移の読み出しと古い行の削除(docs/phase4-spec.md AC-H*)。

const (
	// DefaultHistoryDays は GET price-history の days の既定。
	DefaultHistoryDays = 90
	// MaxHistoryDays は days の最大(= item.PriceHistoryRetentionDays)。
	MaxHistoryDays = item.PriceHistoryRetentionDays
)

// HistoryPoint はサイトの 1 日の目安。
type HistoryPoint struct {
	Day time.Time // item.HistoryDay の形
	Low int
	Mid *int
}

// SiteHistory は 1 サイトの推移(Day 昇順)。
type SiteHistory struct {
	SiteID int64
	Points []HistoryPoint
}

// DayLow はその日の全サイトの Low の最小。
type DayLow struct {
	Day time.Time
	Low int
}

// History は API に返す推移。
type History struct {
	Days int
	// Sites は点のあるサイトだけ。商品のジャンルの site_ids の順、ジャンルに無いサイトはその後に site_id 昇順。
	Sites []SiteHistory
	// Overall は点のある日だけ(Day 昇順)。
	Overall []DayLow
}

// historyToday は今日(JST、Deps.Now)の日付(item.HistoryDay の形)。
func (s *Service) historyToday() time.Time {
	now := time.Now
	if s.d.Now != nil {
		now = s.d.Now
	}
	return item.HistoryDay(now())
}

// pruneHistory は保持期間(今日を含む PriceHistoryRetentionDays 日)より前の行を消す。失敗はログに出すだけ。
func (s *Service) pruneHistory(ctx context.Context) {
	before := s.historyToday().AddDate(0, 0, -(item.PriceHistoryRetentionDays - 1))
	if _, err := s.d.Prices.PrunePriceHistory(ctx, before); err != nil {
		s.d.Logger.Error("prune price history failed", "error", err)
	}
}

// PriceHistory は商品の直近 days 日(今日〈JST、Deps.Now〉を含む。since = 今日 − (days−1) 日)の推移を返す。
// 商品が無ければ item.ErrNotFound、days が 1〜MaxHistoryDays の外なら item.ErrInvalid。
func (s *Service) PriceHistory(ctx context.Context, itemID int64, days int) (History, error) {
	it, err := s.d.Items.GetItem(ctx, itemID)
	if err != nil {
		return History{}, err
	}
	if days < 1 || days > MaxHistoryDays {
		return History{}, item.ErrInvalid
	}
	genres, err := s.d.Items.ListGenres(ctx)
	if err != nil {
		return History{}, err
	}
	var order []int64
	for _, g := range genres {
		if g.ID == it.GenreID {
			order = g.SiteIDs
		}
	}
	since := s.historyToday().AddDate(0, 0, -(days - 1))
	points, err := s.d.Prices.ListPriceHistory(ctx, itemID, since)
	if err != nil {
		return History{}, err
	}
	h := BuildHistory(points, order)
	h.Days = days
	return h, nil
}

// BuildHistory は保存済みの行(ListPriceHistory の並び)から History の Sites・Overall を作る純関数。
// siteOrder はジャンルの site_ids(表示順)。Days は設定しない。
func BuildHistory(points []item.PricePoint, siteOrder []int64) History {
	bySite := map[int64][]HistoryPoint{}
	for _, p := range points {
		bySite[p.SiteID] = append(bySite[p.SiteID], HistoryPoint{Day: p.Day, Low: p.Low, Mid: p.Mid})
	}
	for _, pts := range bySite {
		slices.SortFunc(pts, func(a, b HistoryPoint) int { return a.Day.Compare(b.Day) })
	}
	ids := make([]int64, 0, len(bySite))
	seen := map[int64]bool{}
	for _, id := range siteOrder {
		if _, ok := bySite[id]; ok && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	var rest []int64
	for id := range bySite {
		if !seen[id] {
			rest = append(rest, id)
		}
	}
	slices.Sort(rest)
	ids = append(ids, rest...)

	h := History{Sites: []SiteHistory{}, Overall: []DayLow{}}
	min := map[int64]DayLow{} // Day の Unix 秒 → その日の最小
	for _, id := range ids {
		pts := bySite[id]
		h.Sites = append(h.Sites, SiteHistory{SiteID: id, Points: pts})
		for _, p := range pts {
			k := p.Day.Unix()
			if cur, ok := min[k]; !ok || p.Low < cur.Low {
				min[k] = DayLow{Day: p.Day, Low: p.Low}
			}
		}
	}
	for _, d := range min {
		h.Overall = append(h.Overall, d)
	}
	slices.SortFunc(h.Overall, func(a, b DayLow) int { return a.Day.Compare(b.Day) })
	return h
}
