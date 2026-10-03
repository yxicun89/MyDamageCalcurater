//go:build tidb

package expire_test

// 失効ジョブの TiDB 実装(expire.NewTiDB)の実 SQL を検査する(ADR-0220 §4。make test-db だけで走る。
// RECORD_TEST_DSN が必須でスキップしない。ADR-0211)。
//
// 失効の SQL は全端末を横断するので、同じ DB にある他のテストの行も条件に合えば消える。各テストは
// 乱数の端末 ID を使い、**自分の端末の行の有無だけ**を検査する(件数の合計は他の行の影響を受けるため見ない。
// 上限のテストだけは「上限ちょうどを消した」ことを見る)。go test -p 1(make test-db)で、
// internal/store のテストと同時には走らない。

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	recorddb "example.com/pokecalc/services/record/db"
	"example.com/pokecalc/services/record/internal/expire"
)

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
	if !strings.Contains(dsn, "parseTime=true") {
		if strings.Contains(dsn, "?") {
			dsn += "&parseTime=true"
		} else {
			dsn += "?parseTime=true"
		}
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

func newDeviceID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "x-" + hex.EncodeToString(b)
}

// dbNow は DATETIME(6) に収まる精度の「いま」(マイクロ秒で切る)。
func dbNow() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

func exec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func insertDevice(t *testing.T, db *sql.DB, device string, lastSeen time.Time, purgedAt, orphanedSince *time.Time) {
	t.Helper()
	exec(t, db, `INSERT INTO devices (device_id, last_seen_at, purged_at, orphaned_since) VALUES (?, ?, ?, ?)`,
		device, lastSeen, purgedAt, orphanedSince)
}

func insertEvent(t *testing.T, db *sql.DB, device, suffix, species string, occurredAt time.Time) string {
	t.Helper()
	id := device + "-" + suffix
	exec(t, db, `INSERT INTO calc_events (event_id, device_id, session_id, operation, occurred_at, defender_species_key, payload, created_at)
		VALUES (?, ?, 's-1', 'calc', ?, ?, '{}', ?)`, id, device, occurredAt, species, occurredAt)
	return id
}

func insertAggregate(t *testing.T, db *sql.DB, device, species string, lastCalc time.Time) {
	t.Helper()
	exec(t, db, `INSERT INTO frequent_opponents (device_id, species_key, score, count, last_calculated_at) VALUES (?, ?, 1, 1, ?)`,
		device, species, lastCalc)
}

func insertFavorite(t *testing.T, db *sql.DB, device, species string, updatedAt time.Time) {
	t.Helper()
	exec(t, db, `INSERT INTO favorites (device_id, species_key, snapshot, created_at, updated_at) VALUES (?, ?, '{}', ?, ?)`,
		device, species, updatedAt, updatedAt)
}

func insertJournal(t *testing.T, db *sql.DB, device string, requestedAt time.Time) {
	t.Helper()
	exec(t, db, `INSERT INTO purge_journal (device_id, requested_at) VALUES (?, ?)`, device, requestedAt)
}

func exists(t *testing.T, db *sql.DB, q string, args ...any) bool {
	t.Helper()
	var one int
	err := db.QueryRowContext(context.Background(), q, args...).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return true
}

func countByDevice(t *testing.T, db *sql.DB, table, device string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE device_id = ?", table), device).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func orphanedSince(t *testing.T, db *sql.DB, device string) (sql.NullTime, bool) {
	t.Helper()
	var ts sql.NullTime
	err := db.QueryRowContext(context.Background(), `SELECT orphaned_since FROM devices WHERE device_id = ?`, device).Scan(&ts)
	if errors.Is(err, sql.ErrNoRows) {
		return ts, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return ts, true
}

// runUntilDone は Remaining が false になるまで回す(他のテストの残りに上限を食われても、自分の行まで届くように)。
func runUntilDone(t *testing.T, st expire.Store, p expire.Policy, now time.Time) {
	t.Helper()
	for i := 0; i < 100; i++ {
		res, err := expire.Run(context.Background(), st, p, now, discardLogger())
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if !res.Remaining {
			return
		}
	}
	t.Fatal("100回回しても Remaining が false にならない")
}

func TestTiDBExpireCalcEventsAndAggregatesBoundary(t *testing.T) {
	db := testDB(t)
	st := expire.NewTiDB(db)
	now := dbNow()
	a, b := newDeviceID(t), newDeviceID(t)
	insertDevice(t, db, a, now, nil, nil)
	insertDevice(t, db, b, now, nil, nil)

	keep89 := insertEvent(t, db, a, "89d", "9001-000", now.Add(-89*day))
	keep90 := insertEvent(t, db, a, "90d", "9001-000", now.Add(-90*day))
	gone90s := insertEvent(t, db, a, "90d1s", "9002-000", now.Add(-90*day-time.Second))
	gone91 := insertEvent(t, db, a, "91d", "9002-000", now.Add(-91*day))
	keepB := insertEvent(t, db, b, "1d", "9002-000", now.Add(-day))
	// 9001 は 90日ちょうどのイベントが残るので集計も残る。9002 は端末 A のイベントが全部消えるので集計も消える(AC-R6)。
	insertAggregate(t, db, a, "9001-000", now.Add(-89*day))
	insertAggregate(t, db, a, "9002-000", now.Add(-90*day-time.Second))
	insertAggregate(t, db, b, "9002-000", now.Add(-day))

	runUntilDone(t, st, defaultPolicy, now)

	for id, want := range map[string]bool{keep89: true, keep90: true, gone90s: false, gone91: false, keepB: true} {
		if got := exists(t, db, `SELECT 1 FROM calc_events WHERE event_id = ?`, id); got != want {
			t.Errorf("calc_events %s の存在 = %v, want %v", id, got, want)
		}
	}
	aggCases := []struct {
		device, species string
		want            bool
	}{{a, "9001-000", true}, {a, "9002-000", false}, {b, "9002-000", true}}
	for _, c := range aggCases {
		got := exists(t, db, `SELECT 1 FROM frequent_opponents WHERE device_id = ? AND species_key = ?`, c.device, c.species)
		if got != c.want {
			t.Errorf("集計 (%s, %s) の存在 = %v, want %v", c.device, c.species, got, c.want)
		}
	}
}

func TestTiDBExpireFavoritesUsesMaxOfLastSeenAndUpdatedAt(t *testing.T) {
	db := testDB(t)
	st := expire.NewTiDB(db)
	now := dbNow()
	cases := []struct {
		name      string
		lastSeen  *time.Duration // nil なら devices 行を作らない
		updated   time.Duration
		wantAlive bool
	}{
		{"両方541日", ptr(541 * day), 541 * day, false},
		{"last_seen 541日・行は539日(AC-R2b)", ptr(541 * day), 539 * day, true},
		{"last_seen 539日・行は900日", ptr(539 * day), 900 * day, true},
		{"両方ちょうど540日", ptr(540 * day), 540 * day, true},
		{"devices 行なし・行は541日", nil, 541 * day, false},
		{"devices 行なし・行は540日", nil, 540 * day, true},
	}
	devices := make([]string, len(cases))
	for i, c := range cases {
		devices[i] = newDeviceID(t)
		if c.lastSeen != nil {
			insertDevice(t, db, devices[i], now.Add(-*c.lastSeen), nil, nil)
		}
		insertFavorite(t, db, devices[i], "9001-000", now.Add(-c.updated))
	}

	runUntilDone(t, st, defaultPolicy, now)

	for i, c := range cases {
		if got := countByDevice(t, db, "favorites", devices[i]) == 1; got != c.wantAlive {
			t.Errorf("%s: お気に入りが残っている = %v, want %v", c.name, got, c.wantAlive)
		}
	}
}

func TestTiDBExpirePurgeJournalBoundary(t *testing.T) {
	db := testDB(t)
	st := expire.NewTiDB(db)
	now := dbNow()
	keep, gone := newDeviceID(t), newDeviceID(t)
	insertJournal(t, db, keep, now.Add(-90*day))
	insertJournal(t, db, gone, now.Add(-90*day-time.Second))

	runUntilDone(t, st, defaultPolicy, now)

	if countByDevice(t, db, "purge_journal", keep) != 1 {
		t.Error("ちょうど90日の purge journal が消えた")
	}
	if countByDevice(t, db, "purge_journal", gone) != 0 {
		t.Error("90日を超えた purge journal が残っている")
	}
}

// AC-E6: devices 行の失効(ADR-0220 §4 の 5・6)。
func TestTiDBExpireOrphanDevices(t *testing.T) {
	db := testDB(t)
	st := expire.NewTiDB(db)
	now := dbNow()
	longAgo := now.Add(-400 * day)

	empty := newDeviceID(t)        // データが無く last_seen も古い → 印が付き、30日を超えたら消える
	seenRecently := newDeviceID(t) // データは無いが now に使われた → 印は付く。now から D 以内は消えない
	tombstoned := newDeviceID(t)   // 墓石が now−1h → 印は付く。墓石から D 以内は消えない
	revived := newDeviceID(t)      // 印が付いた後にデータが戻った → 印が消え、行も残る
	withData := newDeviceID(t)     // データがある → 印が付かない
	insertDevice(t, db, empty, longAgo, nil, nil)
	insertDevice(t, db, seenRecently, now, nil, nil)
	insertDevice(t, db, tombstoned, longAgo, ptrTime(now.Add(-time.Hour)), nil)
	insertDevice(t, db, revived, longAgo, nil, ptrTime(now.Add(-60*day)))
	insertEvent(t, db, revived, "e", "", now.Add(-day))
	insertDevice(t, db, withData, longAgo, nil, nil)
	insertFavorite(t, db, withData, "9001-000", now)

	// 1回目: 印付け。まだ消えない(印は now)。
	runUntilDone(t, st, defaultPolicy, now)
	for _, d := range []string{empty, seenRecently, tombstoned} {
		ts, ok := orphanedSince(t, db, d)
		if !ok || !ts.Valid || !ts.Time.Equal(now) {
			t.Errorf("%s: orphaned_since = %v(行あり=%v), want %s", d, ts, ok, now)
		}
	}
	if ts, ok := orphanedSince(t, db, revived); !ok || ts.Valid {
		t.Errorf("データが戻った端末の orphaned_since = %v(行あり=%v), want NULL に戻る", ts, ok)
	}
	if ts, ok := orphanedSince(t, db, withData); !ok || ts.Valid {
		t.Errorf("データのある端末の orphaned_since = %v(行あり=%v), want NULL", ts, ok)
	}

	// ちょうど30日後: まだ消えない。
	exactly := now.Add(30 * day)
	runUntilDone(t, st, defaultPolicy, exactly)
	for _, d := range []string{empty, seenRecently, tombstoned} {
		if _, ok := orphanedSince(t, db, d); !ok {
			t.Errorf("%s: orphaned_since からちょうど30日で devices 行が消えた(超えたら消す)", d)
		}
	}

	// 30日+1秒後: empty だけが消える。
	after := exactly.Add(time.Second)
	runUntilDone(t, st, defaultPolicy, after)
	// seenRecently の last_seen_at(now)・tombstoned の purged_at(now−1h)は、after から見ると D を超えているので
	// ここでは消えてよい(「D 以内なら消えない」側は TestTiDBExpireKeepsDevicesSeenOrPurgedWithinExpiry が見る)。
	// revived・withData はデータがあるので残る。
	alive := map[string]bool{empty: false, seenRecently: false, tombstoned: false, revived: true, withData: true}
	for d, want := range alive {
		if _, ok := orphanedSince(t, db, d); ok != want {
			t.Errorf("%s: devices 行が残っている = %v, want %v", d, ok, want)
		}
	}
}

// AC-E6: 「使われ続けている端末」「墓石が D 以内」の行は、orphaned_since が古くても消えない。
func TestTiDBExpireKeepsDevicesSeenOrPurgedWithinExpiry(t *testing.T) {
	db := testDB(t)
	st := expire.NewTiDB(db)
	now := dbNow()
	oldOrphan := ptrTime(now.Add(-100 * day))

	seen := newDeviceID(t)
	purged := newDeviceID(t)
	insertDevice(t, db, seen, now.Add(-29*day), nil, oldOrphan)
	insertDevice(t, db, purged, now.Add(-400*day), ptrTime(now.Add(-30*day)), oldOrphan) // 墓石ちょうど30日は残す

	runUntilDone(t, st, defaultPolicy, now)

	if _, ok := orphanedSince(t, db, seen); !ok {
		t.Error("last_seen_at が30日以内の端末の devices 行が消えた(使われ続ける端末の行は消えない)")
	}
	if _, ok := orphanedSince(t, db, purged); !ok {
		t.Error("purged_at から30日以内の端末の devices 行(墓石)が消えた(ADR-0209 §7 の猶予)")
	}
}

// AC-E4: 上限と冪等(実 SQL の DELETE ... LIMIT)。
func TestTiDBExpireBatchLimitAndIdempotent(t *testing.T) {
	db := testDB(t)
	st := expire.NewTiDB(db)
	now := dbNow()
	// 先に他のテストの残りを掃除する(この後の「上限ちょうど」の検査を他の行に左右させない)。
	runUntilDone(t, st, defaultPolicy, now)

	d := newDeviceID(t)
	insertDevice(t, db, d, now, nil, nil)
	for i := range 5 {
		insertEvent(t, db, d, fmt.Sprintf("old%d", i), "", now.Add(-100*day))
	}
	p := defaultPolicy
	p.BatchLimit = 3

	first, err := expire.Run(context.Background(), st, p, now, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if first.CalcEvents != 3 || !first.Remaining {
		t.Fatalf("1回目 = %+v, want CalcEvents 3・Remaining true", first)
	}
	if n := countByDevice(t, db, "calc_events", d); n != 2 {
		t.Fatalf("1回目の後に残ったイベント = %d, want 2", n)
	}
	runUntilDone(t, st, p, now)
	if n := countByDevice(t, db, "calc_events", d); n != 0 {
		t.Fatalf("繰り返した後に残ったイベント = %d, want 0", n)
	}
	again, err := expire.Run(context.Background(), st, p, now, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if again.CalcEvents != 0 || again.Aggregates != 0 || again.Favorites != 0 || again.PurgeJournal != 0 || again.Remaining {
		t.Errorf("同じ now でもう一度 = %+v, want 削除 0・Remaining false(冪等)", again)
	}
}

func ptr(d time.Duration) *time.Duration { return &d }
func ptrTime(t time.Time) *time.Time     { return &t }
