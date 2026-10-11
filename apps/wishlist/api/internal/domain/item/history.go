package item

import (
	"time"
)

// フェーズ4-2 価格の推移(docs/phase4-spec.md AC-H*)。

// PriceHistoryRetentionDays は価格の推移を残す日数(今日〈JST〉を含む)。これより古い日の行は PrunePriceHistory で消す。
const PriceHistoryRetentionDays = 180

// jst は履歴の日付を決める時刻帯(tzdata に依存しない固定オフセット)。
var jst = time.FixedZone("JST", 9*60*60)

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
	y, mo, d := t.In(jst).Date()
	return time.Date(y, mo, d, 0, 0, 0, 0, time.UTC)
}
