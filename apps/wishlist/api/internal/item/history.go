package item

import (
	"context"
	"time"
)

// フェーズ4-2 価格の推移(docs/phase4-spec.md AC-H*)。

// PriceHistoryRetentionDays は価格の推移を残す日数(今日〈JST〉を含む)。これより古い日の行は PrunePriceHistory で消す。
const PriceHistoryRetentionDays = 180

// PricePoint は価格の推移の 1 行(price_history。1 商品×1 サイト×1 日〈JST〉に 1 行)。
// 取得が ok で low がある日だけ作る(no_result・failed の日は作らない。値を推測しない)。
type PricePoint struct {
	ItemID int64
	SiteID int64
	// Day は JST の日付。その日の 00:00 UTC の time.Time で表す(HistoryDay の戻り値と同じ形。MySQL の DATE を parseTime で読んだ値と同じ)。
	Day        time.Time
	Low        int
	Mid        *int
	Count      int
	RecordedAt time.Time // その日の最後に保存した取得時刻(秒未満を切り捨てた UTC)
}

// HistoryDay は t の JST の日付を、その日の 00:00 UTC の time.Time で返す(例 2026-10-03T15:30Z → 2026-10-04T00:00Z)。
func HistoryDay(t time.Time) time.Time {
	_ = t
	return time.Time{} // TODO(implementer): docs/phase4-spec.md AC-H1
}

// PriceHistoryRepository は価格の推移の永続化。PriceRepository に含める。
//
// 書き込みは SaveSiteResult が行う: e.Status が ok かつ e.Low があるときだけ、同じトランザクションで
// (e.ItemID, e.SiteID, HistoryDay(e.FetchedAt)) の行を low・mid・count・recorded_at(= 保存する fetched_at)で upsert する
// (同じ日は最後の値で上書き)。no_result・MarkFailed・入力の検査で弾いた保存は行を作らない・変えない。
type PriceHistoryRepository interface {
	// ListPriceHistory は商品の推移のうち Day >= since の行を返す(site_id 昇順・同じサイトは day 昇順)。
	// since は HistoryDay の形(00:00 UTC)で渡す。商品が無ければ空(エラーにしない)。
	ListPriceHistory(ctx context.Context, itemID int64, since time.Time) ([]PricePoint, error)
	// PrunePriceHistory は Day < before の行を全商品から消し、消した行数を返す。
	PrunePriceHistory(ctx context.Context, before time.Time) (int64, error)
}
