//go:build tidb

package store

// TiDBStore の実 SQL 検証(critic 指摘 R-1)。`RECORD_TEST_DSN`(TiDB。DB 名は `_test` で終わること)が
// 必須で、`make test-db` からだけ実行する(スキップしない)。httpapi/fixture_test.go・events/consumer_test.go の
// 手書き fake はここまで実行しないため、本物の SQL(一意制約・ON DUPLICATE KEY UPDATE・トランザクションの
// 削除順序)をここで検証する。
//
// 各テストは乱数の device_id を使って互いに独立させる(テーブルの truncate はしない。他のテストや
// 並行実行と競合しないため)。

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	recorddb "example.com/pokecalc/services/record/db"
)

// hugeHalfLife は減衰をほぼ無視できるほど長い半減期(スコア ≈ 件数になるようにして、
// 順序・タイブレークのテストを浮動小数の丸めに悩まされずに書けるようにする)。
const hugeHalfLife = 100000 * time.Hour

// shortHalfLife は減衰そのものを検証するテストに使う短い半減期。
const shortHalfLife = time.Hour

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("RECORD_TEST_DSN")
	if dsn == "" {
		t.Fatal("RECORD_TEST_DSN が無い(make test-db は DB を前提にする。スキップしない)")
	}
	if !strings.Contains(dsn, "_test") {
		t.Fatal("RECORD_TEST_DSN の DB 名は _test で終わること(誤って本番 DB に繋がないため)")
	}
	if err := recorddb.Up(dsn); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("DB を開けない: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("DB に届かない(スキップしない): %v", err)
	}
	return db
}

// newStore は halfLife・purgeBatch を指定した TiDBStore を作る。
func newStore(db *sql.DB, halfLife time.Duration, purgeBatch int) *TiDBStore {
	return New(db, halfLife, purgeBatch)
}

// newDeviceID はテストどうしが衝突しない乱数の端末 ID(VARCHAR(36) に収まる)。
func newDeviceID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "d-" + hex.EncodeToString(b) // 2 + 32 = 34 文字
}

func ctx() context.Context { return context.Background() }

// insertFavorite は favorites に直接1行差し込む(P5-3 の API 範囲外の CRUD なので、テストは
// SQL で直接作る。PurgeDevice が正しく favorites も消すことを検証するためだけに使う)。
func insertFavorite(t *testing.T, db *sql.DB, deviceID string) {
	t.Helper()
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx(), `
		INSERT INTO favorites (device_id, species_key, snapshot, created_at, updated_at)
		VALUES (?, '9001-000', '{}', ?, ?)
	`, deviceID, now, now)
	if err != nil {
		t.Fatalf("favorites への差し込みに失敗: %v", err)
	}
}

func countByDevice(t *testing.T, db *sql.DB, table, deviceID string) int {
	t.Helper()
	var n int
	err := db.QueryRowContext(ctx(), fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE device_id = ?", table), deviceID).Scan(&n)
	if err != nil {
		t.Fatalf("%s の件数を数えられない: %v", table, err)
	}
	return n
}

func deviceLastSeenAt(t *testing.T, db *sql.DB, deviceID string) time.Time {
	t.Helper()
	var ts sql.NullTime
	err := db.QueryRowContext(ctx(), `SELECT last_seen_at FROM devices WHERE device_id = ?`, deviceID).Scan(&ts)
	if err != nil {
		t.Fatalf("devices.last_seen_at を読めない: %v", err)
	}
	return ts.Time
}

// calcEvent は event_id を deviceID + suffix から組み立てる。calc_events.event_id は
// device_id を含まないグローバルな一意キー(JetStream のストリームシーケンス由来。ADR-0212 §6)
// なので、deviceID(newDeviceID で乱数生成済み)を混ぜることで、同じ永続 TiDB に対して
// このテストを何度実行しても(過去の実行で残った行があっても)衝突しない値にする。
func calcEvent(deviceID, suffix, speciesKey string, occurredAt time.Time) CalcEvent {
	return CalcEvent{
		EventID: deviceID + "-" + suffix, DeviceID: deviceID, SessionID: "s-1", Operation: "calc",
		OccurredAt: occurredAt, DefenderSpeciesKey: speciesKey, Payload: []byte(`{"ok":true}`),
	}
}

// --- TouchDevice(AC-R5) --------------------------------------------------

func TestTiDBTouchDeviceWriteThreshold(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	base := time.Now().UTC().Truncate(time.Second)

	if err := st.TouchDevice(ctx(), deviceID, base); err != nil {
		t.Fatalf("TouchDevice(初回): %v", err)
	}
	if got := deviceLastSeenAt(t, db, deviceID); !got.Equal(base) {
		t.Fatalf("last_seen_at = %v, want %v(初回)", got, base)
	}

	// 23時間59分後: まだ書かない。
	if err := st.TouchDevice(ctx(), deviceID, base.Add(23*time.Hour+59*time.Minute)); err != nil {
		t.Fatalf("TouchDevice(23h59m): %v", err)
	}
	if got := deviceLastSeenAt(t, db, deviceID); !got.Equal(base) {
		t.Errorf("last_seen_at = %v, want %v(23h59m では更新しない。AC-R5)", got, base)
	}

	// ちょうど24時間後: 書く。
	exactly24h := base.Add(24 * time.Hour)
	if err := st.TouchDevice(ctx(), deviceID, exactly24h); err != nil {
		t.Fatalf("TouchDevice(24h): %v", err)
	}
	if got := deviceLastSeenAt(t, db, deviceID); !got.Equal(exactly24h) {
		t.Errorf("last_seen_at = %v, want %v(24h ちょうどでは更新する。AC-R5)", got, exactly24h)
	}
}

// --- SaveCalcEvent: 冪等性(AC-R7) -----------------------------------------

func TestTiDBSaveCalcEventIsIdempotent(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	base := time.Now().UTC()
	ev := calcEvent(deviceID, "evt-dup-1", "9001-000", base)

	outcome, err := st.SaveCalcEvent(ctx(), ev)
	if err != nil || outcome != Stored {
		t.Fatalf("1回目 = %v, %v; want Stored, nil", outcome, err)
	}
	// 2回目(同じ event_id。at-least-once の再配送)。
	outcome, err = st.SaveCalcEvent(ctx(), ev)
	if err != nil || outcome != Duplicate {
		t.Fatalf("2回目 = %v, %v; want Duplicate, nil(一意制約 event_id で弾かれること)", outcome, err)
	}

	if n := countByDevice(t, db, "calc_events", deviceID); n != 1 {
		t.Errorf("calc_events の件数 = %d, want 1(2回目で二重に増えない)", n)
	}
	rows, err := st.FrequentOpponents(ctx(), deviceID, 10)
	if err != nil {
		t.Fatalf("FrequentOpponents: %v", err)
	}
	if len(rows) != 1 || rows[0].Count != 1 {
		t.Errorf("集計 = %+v, want 1件・count=1(2回目で集計が二重に増えない)", rows)
	}
	if got := deviceLastSeenAt(t, db, deviceID); !got.Equal(base) {
		t.Errorf("last_seen_at = %v, want %v(2回目で進まない)", got, base)
	}

	// 2回目より後の(新しい)occurred_at で3回目を送っても、event_id が同じなら Duplicate のまま
	// (2回目の再配送で内容が変わることは無いはずだが、念のため last_seen_at も進まないことを確認)。
	ev2 := ev
	ev2.OccurredAt = base.Add(48 * time.Hour)
	outcome, err = st.SaveCalcEvent(ctx(), ev2)
	if err != nil || outcome != Duplicate {
		t.Fatalf("3回目 = %v, %v; want Duplicate, nil", outcome, err)
	}
	if got := deviceLastSeenAt(t, db, deviceID); !got.Equal(base) {
		t.Errorf("last_seen_at = %v, want %v(重複イベントでは更新しない)", got, base)
	}
}

// --- SaveCalcEvent: 墓石の境界(AC-P3・AC-P4) -------------------------------

func TestTiDBSaveCalcEventTombstoneBoundary(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)

	purgedAt := time.Now().UTC()
	if _, err := st.PurgeDevice(ctx(), deviceID, purgedAt); err != nil {
		t.Fatalf("PurgeDevice: %v", err)
	}

	tests := []struct {
		name       string
		occurredAt time.Time
		wantStored bool
	}{
		{"墓石より前", purgedAt.Add(-time.Minute), false},
		{"墓石とちょうど同時", purgedAt, false},
		{"墓石より後", purgedAt.Add(time.Minute), true},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := calcEvent(deviceID, fmt.Sprintf("evt-tomb-%d", i), "9001-000", tt.occurredAt)
			outcome, err := st.SaveCalcEvent(ctx(), ev)
			if err != nil {
				t.Fatalf("SaveCalcEvent: %v", err)
			}
			want := Tombstoned
			if tt.wantStored {
				want = Stored
			}
			if outcome != want {
				t.Errorf("outcome = %v, want %v", outcome, want)
			}
		})
	}
	// 保存されたのは「墓石より後」の1件だけ。
	if n := countByDevice(t, db, "calc_events", deviceID); n != 1 {
		t.Errorf("calc_events の件数 = %d, want 1(墓石より後の1件だけ)", n)
	}
}

// --- R-2 回帰: last_calculated_at がイベントの再配送順で巻き戻らない ----------

func TestTiDBFrequentOpponentsLastCalculatedAtDoesNotRewind(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	newer := time.Now().UTC()
	older := newer.Add(-time.Hour)

	// 先に新しい(occurred_at が新しい)イベントを保存し、後から古いイベントが届く
	// (JetStream の Nak 後の再配送は順序を保証しない。ADR-0212 §6)。
	if _, err := st.SaveCalcEvent(ctx(), calcEvent(deviceID, "evt-order-new", "9001-000", newer)); err != nil {
		t.Fatalf("新しいイベント: %v", err)
	}
	if _, err := st.SaveCalcEvent(ctx(), calcEvent(deviceID, "evt-order-old", "9001-000", older)); err != nil {
		t.Fatalf("古いイベント: %v", err)
	}

	rows, err := st.FrequentOpponents(ctx(), deviceID, 10)
	if err != nil {
		t.Fatalf("FrequentOpponents: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("集計の件数 = %d, want 1", len(rows))
	}
	if rows[0].Count != 2 {
		t.Errorf("count = %d, want 2(両方保存される)", rows[0].Count)
	}
	// critic 指摘 R-2: last_calculated_at は新しい方(newer)のまま。古いイベントの再配送で
	// 過去へ巻き戻ってはいけない(巻き戻ると読み出し時の追加減衰〈tidb.go の decayFactor〉が
	// 余分にかかり、スコアが不当に下がる)。
	if !rows[0].LastCalculatedAt.Equal(newer) {
		t.Errorf("last_calculated_at = %v, want %v(古いイベントの再配送で巻き戻ってはいけない。R-2)",
			rows[0].LastCalculatedAt, newer)
	}
}

// 減衰そのものの検証: 半減期ぶん時間が経った(occurred_at をその分過去にした)イベントの寄与は
// 概ね半分になる。
func TestTiDBFrequentOpponentsDecay(t *testing.T) {
	db := testDB(t)
	st := newStore(db, shortHalfLife, 1000)
	deviceID := newDeviceID(t)
	now := time.Now().UTC()

	// 半減期(1時間)ちょうど前のイベントを1件だけ保存する。
	if _, err := st.SaveCalcEvent(ctx(), calcEvent(deviceID, "evt-decay-1", "9001-000", now.Add(-shortHalfLife))); err != nil {
		t.Fatalf("SaveCalcEvent: %v", err)
	}
	rows, err := st.FrequentOpponents(ctx(), deviceID, 10)
	if err != nil {
		t.Fatalf("FrequentOpponents: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("集計の件数 = %d, want 1", len(rows))
	}
	// 半減期ぶん経過しているので、読み出し時のスコアは 1 件ぶん(1.0)の半分程度になっているはず
	// (SaveCalcEvent の時点でも last_calculated_at はイベント時刻なので、読み出し時に「now - occurred_at
	// = 半減期」ぶんの減衰が乗る)。厳密な浮動小数一致は求めず、明確に 1.0 未満・0 より十分大きいことだけ見る。
	if rows[0].Score >= 0.9 || rows[0].Score <= 0.1 {
		t.Errorf("score = %v, want およそ 0.5(半減期1回ぶんの減衰)", rows[0].Score)
	}
}

// --- FrequentOpponents: 並び順・タイブレーク・limit -------------------------

func TestTiDBFrequentOpponentsOrderingAndLimit(t *testing.T) {
	db := testDB(t)
	// 減衰をほぼ無視できる半減期にして、スコア ≈ 件数にする(順序を件数差だけで決める)。
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	now := time.Now().UTC()

	seed := func(species string, n int) {
		for i := 0; i < n; i++ {
			ev := calcEvent(deviceID, fmt.Sprintf("evt-%s-%d", species, i), species, now)
			if _, err := st.SaveCalcEvent(ctx(), ev); err != nil {
				t.Fatalf("SaveCalcEvent(%s): %v", species, err)
			}
		}
	}
	seed("9099-000", 3) // 最多
	seed("9050-000", 2)
	seed("9002-000", 1) // タイブレーク対象(9001 と同数)
	seed("9001-000", 1) // タイブレーク対象

	rows, err := st.FrequentOpponents(ctx(), deviceID, 10)
	if err != nil {
		t.Fatalf("FrequentOpponents: %v", err)
	}
	wantOrder := []string{"9099-000", "9050-000", "9001-000", "9002-000"}
	if len(rows) != len(wantOrder) {
		t.Fatalf("件数 = %d, want %d: %+v", len(rows), len(wantOrder), rows)
	}
	for i, want := range wantOrder {
		if rows[i].SpeciesKey != want {
			t.Errorf("[%d] speciesKey = %q, want %q(スコア降順・同点は speciesKey 昇順)", i, rows[i].SpeciesKey, want)
		}
	}

	limited, err := st.FrequentOpponents(ctx(), deviceID, 2)
	if err != nil {
		t.Fatalf("FrequentOpponents(limit=2): %v", err)
	}
	if len(limited) != 2 || limited[0].SpeciesKey != "9099-000" || limited[1].SpeciesKey != "9050-000" {
		t.Errorf("limit=2 の結果 = %+v, want 上位2件", limited)
	}
}

// --- PurgeDevice: 削除順序・partial の繰り返し・journal・他端末への非干渉 ------

// AC-P5.2 相当: 1回で消せる上限をちょうど calc_events の件数に合わせると、その回では
// calc_events だけが消え、frequent_opponents・favorites には触れない(ADR-0209 §5.2 の削除順序
// (2)→(3)→(4) を、本物の SQL で確認する)。
func TestTiDBPurgeDeviceOrder(t *testing.T) {
	db := testDB(t)
	deviceID := newDeviceID(t)
	// purgeBatch を3(calc_events の行数ちょうど)にして、後続の集計・favorites に budget が
	// 残らないようにする。
	st := newStore(db, hugeHalfLife, 3)

	for i, species := range []string{"9001-000", "9002-000", "9003-000"} {
		ev := calcEvent(deviceID, fmt.Sprintf("evt-order-%d", i), species, time.Now().UTC())
		if _, err := st.SaveCalcEvent(ctx(), ev); err != nil {
			t.Fatalf("SaveCalcEvent: %v", err)
		}
	}
	insertFavorite(t, db, deviceID)

	if n := countByDevice(t, db, "calc_events", deviceID); n != 3 {
		t.Fatalf("前提: calc_events = %d, want 3", n)
	}
	if n := countByDevice(t, db, "frequent_opponents", deviceID); n != 3 {
		t.Fatalf("前提: frequent_opponents = %d, want 3", n)
	}

	res, err := st.PurgeDevice(ctx(), deviceID, time.Now().UTC())
	if err != nil {
		t.Fatalf("PurgeDevice(1回目): %v", err)
	}
	if res.Deleted.CalcEvents != 3 || res.Deleted.Aggregates != 0 || res.Deleted.Favorites != 0 {
		t.Errorf("1回目の deleted = %+v, want {3 0 0}(calc_events を使い切って集計・favorites に budget が残らない)", res.Deleted)
	}
	if !res.Remaining {
		t.Error("1回目の Remaining = false, want true(集計・favorites がまだ残っている)")
	}
	if n := countByDevice(t, db, "calc_events", deviceID); n != 0 {
		t.Errorf("1回目の後の calc_events = %d, want 0", n)
	}
	if n := countByDevice(t, db, "frequent_opponents", deviceID); n != 3 {
		t.Errorf("1回目の後の frequent_opponents = %d, want 3(まだ消していない。削除順序 (2)→(3))", n)
	}

	// 2回目: 残りの budget で集計・favorites を消し切る。
	st2 := newStore(db, hugeHalfLife, 1000)
	res2, err := st2.PurgeDevice(ctx(), deviceID, time.Now().UTC())
	if err != nil {
		t.Fatalf("PurgeDevice(2回目): %v", err)
	}
	if res2.Deleted.Aggregates != 3 || res2.Deleted.Favorites != 1 {
		t.Errorf("2回目の deleted = %+v, want {0 3 1}", res2.Deleted)
	}
	if res2.Remaining {
		t.Error("2回目の Remaining = true, want false(行が残っていない)")
	}
	for _, table := range []string{"calc_events", "frequent_opponents", "favorites"} {
		if n := countByDevice(t, db, table, deviceID); n != 0 {
			t.Errorf("2回目の後の %s = %d, want 0", table, n)
		}
	}
}

// AC-P1b・§5b: purged_at は呼ぶたびに更新され、purge_journal は呼び出し回数ぶん増える。
func TestTiDBPurgeDeviceReissuesPurgedAtAndJournal(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)

	var journalBefore int
	if err := db.QueryRowContext(ctx(), `SELECT COUNT(*) FROM purge_journal WHERE device_id = ?`, deviceID).Scan(&journalBefore); err != nil {
		t.Fatalf("purge_journal を数えられない: %v", err)
	}

	first, err := st.PurgeDevice(ctx(), deviceID, time.Now().UTC())
	if err != nil {
		t.Fatalf("1回目: %v", err)
	}
	time.Sleep(2 * time.Millisecond) // DATETIME(6) の分解能でも確実に進むように
	second, err := st.PurgeDevice(ctx(), deviceID, time.Now().UTC())
	if err != nil {
		t.Fatalf("2回目: %v", err)
	}
	if !second.PurgedAt.After(first.PurgedAt) {
		t.Errorf("2回目の purgedAt = %v は1回目 %v より後でなければならない", second.PurgedAt, first.PurgedAt)
	}

	var journalAfter int
	if err := db.QueryRowContext(ctx(), `SELECT COUNT(*) FROM purge_journal WHERE device_id = ?`, deviceID).Scan(&journalAfter); err != nil {
		t.Fatalf("purge_journal を数えられない: %v", err)
	}
	if got, want := journalAfter-journalBefore, 2; got != want {
		t.Errorf("purge_journal の増分 = %d, want %d(呼び出し回数ぶん)", got, want)
	}
}

// AC-P5: 削除は他端末のデータを消さない。
func TestTiDBPurgeDeviceDoesNotTouchOtherDevices(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceA := newDeviceID(t)
	deviceB := newDeviceID(t)

	if _, err := st.SaveCalcEvent(ctx(), calcEvent(deviceA, "evt-a-1", "9001-000", time.Now().UTC())); err != nil {
		t.Fatalf("SaveCalcEvent(A): %v", err)
	}
	if _, err := st.SaveCalcEvent(ctx(), calcEvent(deviceB, "evt-b-1", "9001-000", time.Now().UTC())); err != nil {
		t.Fatalf("SaveCalcEvent(B): %v", err)
	}
	insertFavorite(t, db, deviceB)

	if _, err := st.PurgeDevice(ctx(), deviceA, time.Now().UTC()); err != nil {
		t.Fatalf("PurgeDevice(A): %v", err)
	}

	if n := countByDevice(t, db, "calc_events", deviceB); n != 1 {
		t.Errorf("端末 B の calc_events = %d, want 1(A の削除で消えない)", n)
	}
	if n := countByDevice(t, db, "frequent_opponents", deviceB); n != 1 {
		t.Errorf("端末 B の frequent_opponents = %d, want 1", n)
	}
	if n := countByDevice(t, db, "favorites", deviceB); n != 1 {
		t.Errorf("端末 B の favorites = %d, want 1", n)
	}
}

// 既に何も無い端末への PurgeDevice は completed(0行)で、行を作らない(devices の墓石行だけ増える)。
func TestTiDBPurgeDeviceOnEmptyDeviceIsNoop(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)

	res, err := st.PurgeDevice(ctx(), deviceID, time.Now().UTC())
	if err != nil {
		t.Fatalf("PurgeDevice: %v", err)
	}
	if res.Deleted.CalcEvents != 0 || res.Deleted.Aggregates != 0 || res.Deleted.Favorites != 0 {
		t.Errorf("deleted = %+v, want 全て0", res.Deleted)
	}
	if res.Remaining {
		t.Error("Remaining = true, want false")
	}
}
