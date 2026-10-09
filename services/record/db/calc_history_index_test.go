package db

// 計算履歴の一覧(GET /api/record/calc-history。ADR-0230 §7)の索引の静的テスト。DB は使わない(make test で走る)。
//
// **test-first(ADR-0003)**: 実装前に書いた。一覧の問い合わせは
//
//	WHERE device_id = ? AND operation = 'calc' AND occurred_at >= ? AND (occurred_at, event_id) < (?, ?)
//	ORDER BY occurred_at DESC, event_id DESC LIMIT ?
//
// で、既存の idx_calc_events_device_id (device_id, occurred_at) では event_id の並びと operation の絞り込みを
// 索引で閉じられない(同時刻の並べ替えと、calcBulk / calcReverse の行の読み飛ばしが要る)。
// (device_id, operation, occurred_at, event_id) の順の索引を新しい版の migration で足す。
// 適用済みの 000003 は書き換えない(既存の DB と migration の履歴がずれる)。

import (
	"os"
	"regexp"
	"testing"
)

func TestCalcEventsHasHistoryIndex(t *testing.T) {
	sql := allUpSQL(t)
	re := regexp.MustCompile("(?is)(ALTER\\s+TABLE\\s+`?calc_events`?\\s+ADD\\s+(KEY|INDEX)|CREATE\\s+INDEX\\s+`?[a-z0-9_]+`?\\s+ON\\s+`?calc_events`?)" +
		"\\s*`?[a-z0-9_]*`?\\s*\\(\\s*`?device_id`?\\s*,\\s*`?operation`?\\s*,\\s*`?occurred_at`?\\s*,\\s*`?event_id`?\\s*\\)")
	if !re.MatchString(sql) {
		t.Errorf("calc_events に (device_id, operation, occurred_at, event_id) の索引を足す migration が無い(ADR-0230 §7)")
	}

	// 000003(P5-3 で適用済み)の CREATE TABLE calc_events は書き換えない。
	data, err := os.ReadFile("migrations/000003_create_calc_events.up.sql")
	if err != nil {
		t.Fatalf("000003 を読めない: %v", err)
	}
	if regexp.MustCompile(`(?i)\boperation\s*,\s*occurred_at\s*,\s*event_id`).Match(data) {
		t.Error("索引を 000003 の CREATE TABLE に直接足している(新しい版で ALTER TABLE する)")
	}

	// 索引を足す版の down は、その索引だけを落とす(表や既存の索引を消さない)。
	versions, up, down := migrationPairs(t)
	for _, v := range versions {
		b, err := os.ReadFile(up[v])
		if err != nil {
			t.Fatal(err)
		}
		if !re.Match(b) {
			continue
		}
		d, err := os.ReadFile(down[v])
		if err != nil {
			t.Fatalf("版 %d の down を読めない: %v", v, err)
		}
		if regexp.MustCompile(`(?i)DROP\s+TABLE`).Match(d) || regexp.MustCompile(`(?i)idx_calc_events_device_id\b`).Match(d) {
			t.Errorf("版 %d の down が表・既存の索引を消している:\n%s", v, d)
		}
		if !regexp.MustCompile(`(?i)DROP\s+(INDEX|KEY)`).Match(d) {
			t.Errorf("版 %d の down が足した索引を落としていない:\n%s", v, d)
		}
	}
}
