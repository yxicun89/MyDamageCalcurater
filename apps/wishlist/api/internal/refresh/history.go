package refresh

import (
	"context"
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

// PriceHistory は商品の直近 days 日(今日〈JST、Deps.Now〉を含む。since = 今日 − (days−1) 日)の推移を返す。
// 商品が無ければ item.ErrNotFound、days が 1〜MaxHistoryDays の外なら item.ErrInvalid。
func (s *Service) PriceHistory(ctx context.Context, itemID int64, days int) (History, error) {
	_, _, _ = ctx, itemID, days
	return History{}, nil // TODO(implementer): docs/phase4-spec.md AC-H8〜H10
}

// BuildHistory は保存済みの行(ListPriceHistory の並び)から History の Sites・Overall を作る純関数。
// siteOrder はジャンルの site_ids(表示順)。Days は設定しない。
func BuildHistory(points []item.PricePoint, siteOrder []int64) History {
	_, _ = points, siteOrder
	return History{} // TODO(implementer): docs/phase4-spec.md AC-H9
}
