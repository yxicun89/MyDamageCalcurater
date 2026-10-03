//go:build tidb

package expire_test

// team の失効ジョブの TiDB 実装の実 SQL を検査する(ADR-0220 §4。make test-db だけ。TEAM_TEST_DSN が必須)。
// 各テストは乱数の端末 ID を使い、自分の端末の行だけを検査する(record の tidb_test.go と同じ方針)。

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	teamdb "example.com/pokecalc/services/team/db"
	"example.com/pokecalc/services/team/internal/expire"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEAM_TEST_DSN")
	if dsn == "" {
		t.Fatal("TEAM_TEST_DSN が無い(make test-db は DB を前提にする。スキップしない)")
	}
	if !strings.Contains(dsn, "_test") {
		t.Fatal("TEAM_TEST_DSN の DB 名は _test で終わること")
	}
	if err := teamdb.Up(dsn); err != nil {
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
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("DB に届かない(スキップしない): %v", err)
	}
	return db
}

func newID(t *testing.T, prefix string) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return prefix + hex.EncodeToString(b)
}

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

// insertTeam は構築1件とメンバー members 体を差し込み、構築の ID を返す。
func insertTeam(t *testing.T, db *sql.DB, device string, updatedAt time.Time, members int) string {
	t.Helper()
	id := newID(t, "t-") // 34 文字(VARCHAR(36))
	exec(t, db, `INSERT INTO teams (id, device_id, name, created_at, updated_at) VALUES (?, ?, 'x', ?, ?)`, id, device, updatedAt, updatedAt)
	for slot := range members {
		exec(t, db, `INSERT INTO team_members (team_id, slot, device_id, species_key, move_ids, nature_id,
			sp_hp, sp_atk, sp_def, sp_spa, sp_spd, sp_spe) VALUES (?, ?, ?, '9001-000', '[]', 'n', 0, 0, 0, 0, 0, 0)`,
			id, slot, device)
	}
	return id
}

func count(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func deviceRowExists(t *testing.T, db *sql.DB, device string) bool {
	t.Helper()
	var one int
	err := db.QueryRowContext(context.Background(), `SELECT 1 FROM devices WHERE device_id = ?`, device).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return true
}

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

func TestTiDBExpireTeamsBoundaryAndMembers(t *testing.T) {
	db := testDB(t)
	st := expire.NewTiDB(db)
	now := dbNow()
	ago := func(d time.Duration) time.Time { return now.Add(-d) }

	old := newID(t, "o-")
	active := newID(t, "a-") // 計算イベントの購読で last_seen_at が新しい(AC-R2d)
	none := newID(t, "n-")   // devices 行が無い
	insertDevice(t, db, old, ago(541*day), nil, nil)
	insertDevice(t, db, active, ago(time.Hour), nil, nil)

	cases := []struct {
		name  string
		id    string
		alive bool
	}{
		{"last_seen・行とも541日", insertTeam(t, db, old, ago(541*day), 6), false},
		{"行が539日(AC-R2b)", insertTeam(t, db, old, ago(539*day), 2), true},
		{"行がちょうど540日", insertTeam(t, db, old, ago(540*day), 1), true},
		{"last_seen が新しい端末の900日前の構築(AC-R2d)", insertTeam(t, db, active, ago(900*day), 6), true},
		{"devices 行なし・541日", insertTeam(t, db, none, ago(541*day), 3), false},
		{"devices 行なし・540日", insertTeam(t, db, none, ago(540*day), 0), true},
	}

	runUntilDone(t, st, defaultPolicy, now)

	for _, c := range cases {
		gotTeam := count(t, db, `SELECT COUNT(*) FROM teams WHERE id = ?`, c.id) == 1
		if gotTeam != c.alive {
			t.Errorf("%s: 構築が残っている = %v, want %v", c.name, gotTeam, c.alive)
		}
		members := count(t, db, `SELECT COUNT(*) FROM team_members WHERE team_id = ?`, c.id)
		if !c.alive && members != 0 {
			t.Errorf("%s: 構築を消したのにメンバーが %d 体残っている", c.name, members)
		}
	}
	// どの端末にも、構築の無いメンバー(孤児)を残さない。
	for _, d := range []string{old, active, none} {
		orphans := count(t, db, `SELECT COUNT(*) FROM team_members m WHERE m.device_id = ?
			AND NOT EXISTS (SELECT 1 FROM teams t WHERE t.id = m.team_id)`, d)
		if orphans != 0 {
			t.Errorf("端末 %s に構築の無いメンバーが %d 体ある", d, orphans)
		}
	}
}

func TestTiDBExpirePurgeJournalBoundary(t *testing.T) {
	db := testDB(t)
	st := expire.NewTiDB(db)
	now := dbNow()
	keep, gone := newID(t, "k-"), newID(t, "g-")
	exec(t, db, `INSERT INTO purge_journal (device_id, requested_at) VALUES (?, ?)`, keep, now.Add(-90*day))
	exec(t, db, `INSERT INTO purge_journal (device_id, requested_at) VALUES (?, ?)`, gone, now.Add(-90*day-time.Second))

	runUntilDone(t, st, defaultPolicy, now)

	if count(t, db, `SELECT COUNT(*) FROM purge_journal WHERE device_id = ?`, keep) != 1 {
		t.Error("ちょうど90日の purge journal が消えた")
	}
	if count(t, db, `SELECT COUNT(*) FROM purge_journal WHERE device_id = ?`, gone) != 0 {
		t.Error("90日を超えた purge journal が残っている")
	}
}

// AC-E6(team): 構築が無くなった端末の devices 行は、orphaned_since から D を超え、last_seen・墓石も D を超えたら消える。
func TestTiDBExpireOrphanDevices(t *testing.T) {
	db := testDB(t)
	st := expire.NewTiDB(db)
	now := dbNow()
	longAgo := now.Add(-400 * day)

	empty := newID(t, "e-")
	withTeam := newID(t, "w-")
	seen := newID(t, "s-")
	purged := newID(t, "p-")
	oldOrphan := now.Add(-100 * day)
	insertDevice(t, db, empty, longAgo, nil, &oldOrphan)
	insertDevice(t, db, withTeam, longAgo, nil, &oldOrphan) // 印は古いがデータが戻っている → 印を NULL に戻し、消さない
	insertTeam(t, db, withTeam, now, 1)
	insertDevice(t, db, seen, now.Add(-29*day), nil, &oldOrphan)
	purgedAt := now.Add(-30 * day) // 墓石ちょうど30日は残す
	insertDevice(t, db, purged, longAgo, &purgedAt, &oldOrphan)

	runUntilDone(t, st, defaultPolicy, now)

	for d, want := range map[string]bool{empty: false, withTeam: true, seen: true, purged: true} {
		if got := deviceRowExists(t, db, d); got != want {
			t.Errorf("%s: devices 行が残っている = %v, want %v", d, got, want)
		}
	}
	var orphan sql.NullTime
	if err := db.QueryRowContext(context.Background(), `SELECT orphaned_since FROM devices WHERE device_id = ?`, withTeam).Scan(&orphan); err != nil {
		t.Fatal(err)
	}
	if orphan.Valid {
		t.Errorf("構築のある端末の orphaned_since = %v, want NULL に戻る", orphan.Time)
	}
}

func TestTiDBExpireBatchLimitCountsTeams(t *testing.T) {
	db := testDB(t)
	st := expire.NewTiDB(db)
	now := dbNow()
	runUntilDone(t, st, defaultPolicy, now) // 他のテストの残りを先に掃除する

	d := newID(t, "b-")
	insertDevice(t, db, d, now.Add(-1000*day), nil, nil)
	for range 5 {
		insertTeam(t, db, d, now.Add(-1000*day), 6)
	}
	p := defaultPolicy
	p.BatchLimit = 3

	first, err := expire.Run(context.Background(), st, p, now, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if first.Teams != 3 || first.TeamMembers != 18 || !first.Remaining {
		t.Fatalf("1回目 = %+v, want Teams 3・TeamMembers 18・Remaining true", first)
	}
	runUntilDone(t, st, p, now)
	if n := count(t, db, `SELECT COUNT(*) FROM teams WHERE device_id = ?`, d); n != 0 {
		t.Fatalf("繰り返した後に残った構築 = %d, want 0", n)
	}
	again, err := expire.Run(context.Background(), st, p, now, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if again.Teams != 0 || again.TeamMembers != 0 || again.PurgeJournal != 0 || again.Remaining {
		t.Errorf("同じ now でもう一度 = %+v, want 削除 0(冪等)", again)
	}
}
