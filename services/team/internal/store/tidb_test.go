//go:build tidb

package store

// TiDBStore の実 SQL 検証(record-svc の同名ファイルと同じ形。P5-3 の critic 指摘 R-1 の踏襲)。
// `TEAM_TEST_DSN`(TiDB。DB 名は `_test` で終わること)が必須で、`make test-db` からだけ実行する
// (スキップしない)。httpapi / events の手書き fake はここまで実行しないため、本物の SQL
// (端末での絞り込み・一意制約・トランザクションの削除順序)をここで検証する。
//
// 実装済み: `func New(db *sql.DB, purgeBatch int) *TiDBStore`(store.Store を実装する)。この形を固定する。
//
// 各テストは乱数の device_id を使って互いに独立させる(テーブルの truncate はしない)。

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

	teamdb "example.com/pokecalc/services/team/db"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEAM_TEST_DSN")
	if dsn == "" {
		t.Fatal("TEAM_TEST_DSN が無い(make test-db は DB を前提にする。スキップしない)")
	}
	if !strings.Contains(dsn, "_test") {
		t.Fatal("TEAM_TEST_DSN の DB 名は _test で終わること(誤って本番 DB に繋がないため)")
	}
	if err := teamdb.Up(dsn); err != nil {
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

// newDeviceID はテストどうしが衝突しない乱数の端末 ID(VARCHAR(36) に収まる)。
func newDeviceID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "d-" + hex.EncodeToString(b)
}

func ctx() context.Context { return context.Background() }

func sampleTeam(name string, members int) Team {
	t := Team{Name: name}
	for i := 0; i < members; i++ {
		nickname := fmt.Sprintf("nick-%d", i)
		t.Members = append(t.Members, Member{
			SpeciesKey: "9001-000",
			Nickname:   &nickname,
			MoveIDs:    []string{"m1", "m2"},
			NatureID:   "n1",
			SP:         StatBlock{HP: 4, Atk: 32, Spe: 30},
		})
	}
	return t
}

func countByDevice(t *testing.T, db *sql.DB, table, deviceID string) int {
	t.Helper()
	var n int
	q := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE device_id = ?", table)
	if err := db.QueryRowContext(ctx(), q, deviceID).Scan(&n); err != nil {
		t.Fatalf("%s の件数を数えられない: %v", table, err)
	}
	return n
}

// 作成 → 取得 → 更新 → 削除が SQL のうえで往復すること(メンバーの並び順も保つ)。
func TestTeamCRUDRoundTrip(t *testing.T) {
	db := testDB(t)
	st := New(db, 1000)
	device := newDeviceID(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	created, err := st.CreateTeam(ctx(), device, sampleTeam("雨パ", 3), now)
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if created.ID == "" {
		t.Fatal("CreateTeam が ID を発行していない")
	}
	if n := countByDevice(t, db, "team_members", device); n != 3 {
		t.Errorf("team_members = %d 行, want 3", n)
	}

	got, err := st.GetTeam(ctx(), device, created.ID)
	if err != nil {
		t.Fatalf("GetTeam: %v", err)
	}
	if got.Name != "雨パ" || len(got.Members) != 3 {
		t.Errorf("GetTeam = %+v, want 雨パ/3体", got)
	}
	for i, m := range got.Members {
		if m.Nickname == nil || *m.Nickname != fmt.Sprintf("nick-%d", i) {
			t.Errorf("members[%d] の並び順が保たれていない: %+v", i, m)
		}
	}

	later := now.Add(time.Minute)
	updated, err := st.UpdateTeam(ctx(), device, created.ID, sampleTeam("砂パ", 1), later)
	if err != nil {
		t.Fatalf("UpdateTeam: %v", err)
	}
	if updated.Name != "砂パ" || len(updated.Members) != 1 {
		t.Errorf("UpdateTeam = %+v, want 砂パ/1体", updated)
	}
	if n := countByDevice(t, db, "team_members", device); n != 1 {
		t.Errorf("置換後の team_members = %d 行, want 1(古いメンバーが残っている)", n)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("createdAt = %v, want %v(置換で変えない)", updated.CreatedAt, created.CreatedAt)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("updatedAt = %v は %v より後であること", updated.UpdatedAt, created.UpdatedAt)
	}

	if err := st.DeleteTeam(ctx(), device, created.ID); err != nil {
		t.Fatalf("DeleteTeam: %v", err)
	}
	if n := countByDevice(t, db, "team_members", device); n != 0 {
		t.Errorf("削除後の team_members = %d 行, want 0(構築と一緒に消す)", n)
	}
	if err := st.DeleteTeam(ctx(), device, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("2回目の DeleteTeam = %v, want ErrNotFound", err)
	}
}

// ADR-0209 §6-1・§6-2: 他端末の構築には SQL のうえでも届かない。
func TestQueriesAreScopedToTheDevice(t *testing.T) {
	db := testDB(t)
	st := New(db, 1000)
	deviceA, deviceB := newDeviceID(t), newDeviceID(t)
	now := time.Now().UTC()

	created, err := st.CreateTeam(ctx(), deviceA, sampleTeam("A の構築", 2), now)
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	if _, err := st.GetTeam(ctx(), deviceB, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("他端末からの GetTeam = %v, want ErrNotFound", err)
	}
	if _, err := st.UpdateTeam(ctx(), deviceB, created.ID, sampleTeam("乗っ取り", 1), now); !errors.Is(err, ErrNotFound) {
		t.Errorf("他端末からの UpdateTeam = %v, want ErrNotFound", err)
	}
	if err := st.DeleteTeam(ctx(), deviceB, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("他端末からの DeleteTeam = %v, want ErrNotFound", err)
	}
	list, err := st.ListTeams(ctx(), deviceB)
	if err != nil {
		t.Fatalf("ListTeams: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("端末 B の一覧 = %d 件, want 0", len(list))
	}
	// A のデータは無傷。
	if n := countByDevice(t, db, "team_members", deviceA); n != 2 {
		t.Errorf("端末 A の team_members = %d 行, want 2", n)
	}
}

// ADR-0209 §6-1 の pin(critic 指摘 重要1): team_members への SELECT/DELETE も device_id 列で絞ることを、
// team_members.device_id が teams.device_id と食い違う行を使って固定する。teamId は所有権を確認済みの
// 前提(teams 側で device_id を確認してから teamId だけで team_members を触る実装)に戻すと、このテストが
// 失敗する。
func TestTeamMembersQueriesAreScopedByDeviceIDColumn(t *testing.T) {
	db := testDB(t)
	st := New(db, 1000)
	device, mismatched := newDeviceID(t), newDeviceID(t)
	now := time.Now().UTC()

	created, err := st.CreateTeam(ctx(), device, sampleTeam("検査用", 2), now)
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	// team_members.device_id だけを実際の端末と食い違わせる(想定していないデータ不整合を模す)。
	if _, err := db.ExecContext(ctx(), `UPDATE team_members SET device_id = ? WHERE team_id = ?`, mismatched, created.ID); err != nil {
		t.Fatalf("team_members.device_id を書き換えられない: %v", err)
	}

	// GetTeam(loadMembers): device_id が食い違う行は返さない(teamId だけで引かない)。
	got, err := st.GetTeam(ctx(), device, created.ID)
	if err != nil {
		t.Fatalf("GetTeam: %v", err)
	}
	if len(got.Members) != 0 {
		t.Errorf("GetTeam のメンバー = %d 件, want 0(team_members も device_id で絞ること。ADR-0209 §6-1)", len(got.Members))
	}

	// UpdateTeam の DELETE FROM team_members: device_id が食い違う行を消さない
	// (メンバー0件への置換にして、新規挿入との slot 衝突を避けて検査する)。
	if _, err := st.UpdateTeam(ctx(), device, created.ID, sampleTeam("置換", 0), now.Add(time.Minute)); err != nil {
		t.Fatalf("UpdateTeam: %v", err)
	}
	if n := countByDevice(t, db, "team_members", mismatched); n != 2 {
		t.Errorf("UpdateTeam 後の他端末 team_members = %d 行, want 2(device_id で絞られて消えないこと)", n)
	}

	// DeleteTeam の DELETE FROM team_members: teams 行を消しても、device_id が食い違う team_members は
	// 消えない(teamId だけで引くと巻き込んで消してしまう)。
	if err := st.DeleteTeam(ctx(), device, created.ID); err != nil {
		t.Fatalf("DeleteTeam: %v", err)
	}
	if n := countByDevice(t, db, "team_members", mismatched); n != 2 {
		t.Errorf("DeleteTeam 後の他端末 team_members = %d 行, want 2(device_id で絞られて消えないこと)", n)
	}
}

// ADR-0213 §2: 1端末が持てる構築の上限。件数の確認と挿入は同じトランザクションで行う。
func TestCreateTeamRespectsLimit(t *testing.T) {
	db := testDB(t)
	st := New(db, 1000)
	device := newDeviceID(t)
	now := time.Now().UTC()

	for i := 0; i < MaxTeamsPerDevice; i++ {
		if _, err := st.CreateTeam(ctx(), device, sampleTeam(fmt.Sprintf("t%d", i), 0), now); err != nil {
			t.Fatalf("%d 件目の CreateTeam: %v", i+1, err)
		}
	}
	if _, err := st.CreateTeam(ctx(), device, sampleTeam("あふれ", 0), now); !errors.Is(err, ErrTeamLimitReached) {
		t.Errorf("上限超えの CreateTeam = %v, want ErrTeamLimitReached", err)
	}
	if n := countByDevice(t, db, "teams", device); n != MaxTeamsPerDevice {
		t.Errorf("teams = %d 行, want %d", n, MaxTeamsPerDevice)
	}
}

// ADR-0209 §5.2: 全削除は (1) 墓石 + journal → (2) team_members → (3) teams の順。
// 上限に達したら Remaining で返し、繰り返すと消えきる。
func TestPurgeDeviceOrderAndPartial(t *testing.T) {
	db := testDB(t)
	st := New(db, 3) // 1回の上限を小さくして partial を作る
	device := newDeviceID(t)
	now := time.Now().UTC()

	for i := 0; i < 2; i++ {
		if _, err := st.CreateTeam(ctx(), device, sampleTeam(fmt.Sprintf("t%d", i), 2), now); err != nil {
			t.Fatalf("CreateTeam: %v", err)
		}
	}

	first, err := st.PurgeDevice(ctx(), device, now)
	if err != nil {
		t.Fatalf("PurgeDevice: %v", err)
	}
	if !first.Remaining {
		t.Error("1回目で消しきれている(上限3行に対して6行あるはず)")
	}
	// critic 指摘 軽微1: (2) team_members → (3) teams の順であることを内訳で固定する。
	// budget=3・team_members が4行あるので、1回目は team_members だけを3行消し、teams はまだ0行。
	if first.Deleted.TeamMembers != 3 || first.Deleted.Teams != 0 {
		t.Errorf("1回目の削除内訳 = %+v, want {TeamMembers:3 Teams:0}((2)→(3)の順。ADR-0209 §5.2)", first.Deleted)
	}
	if first.PurgedAt.IsZero() {
		t.Error("purgedAt が空(墓石を立てていない)")
	}
	if n := countByDevice(t, db, "purge_journal", device); n != 1 {
		t.Errorf("purge_journal = %d 行, want 1(削除要求のたびに追記する。ADR-0209 §5b)", n)
	}

	for i := 0; i < 5; i++ {
		res, err := st.PurgeDevice(ctx(), device, time.Now().UTC())
		if err != nil {
			t.Fatalf("PurgeDevice(繰り返し): %v", err)
		}
		if !res.Remaining {
			break
		}
	}
	if n := countByDevice(t, db, "teams", device); n != 0 {
		t.Errorf("teams = %d 行, want 0", n)
	}
	if n := countByDevice(t, db, "team_members", device); n != 0 {
		t.Errorf("team_members = %d 行, want 0", n)
	}

	// AC-P1b: 呼ぶたびに purged_at が進む。
	again, err := st.PurgeDevice(ctx(), device, time.Now().UTC().Add(time.Second))
	if err != nil {
		t.Fatalf("PurgeDevice: %v", err)
	}
	if !again.PurgedAt.After(first.PurgedAt) {
		t.Errorf("purgedAt = %v, want %v より後(ADR-0209 §5.2)", again.PurgedAt, first.PurgedAt)
	}
}

// ADR-0209 §4・§7: イベント経由の last_seen_at 更新(24時間規則と墓石)。
func TestTouchDeviceFromEvent(t *testing.T) {
	db := testDB(t)
	st := New(db, 1000)
	device := newDeviceID(t)
	base := time.Now().UTC().Add(-100 * time.Hour).Truncate(time.Second)

	if got, err := st.TouchDeviceFromEvent(ctx(), device, base); err != nil || got != Touched {
		t.Fatalf("1件目 = (%v, %v), want (Touched, nil)", got, err)
	}
	if got, err := st.TouchDeviceFromEvent(ctx(), device, base.Add(time.Hour)); err != nil || got != Skipped {
		t.Errorf("24時間以内の2件目 = (%v, %v), want (Skipped, nil)(AC-R5)", got, err)
	}
	if got, err := st.TouchDeviceFromEvent(ctx(), device, base.Add(25*time.Hour)); err != nil || got != Touched {
		t.Errorf("24時間後 = (%v, %v), want (Touched, nil)", got, err)
	}

	purgedAt := base.Add(50 * time.Hour)
	if _, err := st.PurgeDevice(ctx(), device, purgedAt); err != nil {
		t.Fatalf("PurgeDevice: %v", err)
	}
	if got, err := st.TouchDeviceFromEvent(ctx(), device, purgedAt.Add(-time.Minute)); err != nil || got != Tombstoned {
		t.Errorf("墓石より前 = (%v, %v), want (Tombstoned, nil)(AC-P3)", got, err)
	}
	if got, err := st.TouchDeviceFromEvent(ctx(), device, purgedAt); err != nil || got != Tombstoned {
		t.Errorf("墓石とちょうど同時 = (%v, %v), want (Tombstoned, nil)", got, err)
	}
	if got, err := st.TouchDeviceFromEvent(ctx(), device, purgedAt.Add(48*time.Hour)); err != nil || got != Touched {
		t.Errorf("墓石より後 = (%v, %v), want (Touched, nil)(AC-P4)", got, err)
	}
}

// HTTP 経路の TouchDevice も24時間規則に従う(ADR-0209 §4 AC-R5)。
func TestTouchDeviceIsRateLimited(t *testing.T) {
	db := testDB(t)
	st := New(db, 1000)
	device := newDeviceID(t)
	base := time.Now().UTC().Add(-100 * time.Hour).Truncate(time.Second)

	if err := st.TouchDevice(ctx(), device, base); err != nil {
		t.Fatalf("TouchDevice: %v", err)
	}
	if err := st.TouchDevice(ctx(), device, base.Add(time.Hour)); err != nil {
		t.Fatalf("TouchDevice: %v", err)
	}
	var lastSeen time.Time
	if err := db.QueryRowContext(ctx(), "SELECT last_seen_at FROM devices WHERE device_id = ?", device).Scan(&lastSeen); err != nil {
		t.Fatalf("last_seen_at を読めない: %v", err)
	}
	if !lastSeen.UTC().Equal(base) {
		t.Errorf("last_seen_at = %v, want %v(24時間以内は書かない)", lastSeen.UTC(), base)
	}
}
