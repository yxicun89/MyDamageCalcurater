//go:build mysql

package db

// migration が途中で失敗して dirty になった DB の復旧(issue #221・#279)を実 MySQL で確かめる。
// 手順は docs/runbooks/data.md「migration が途中で失敗して dirty になったとき」と同じ順で流す:
// 版の確認 → 途中まで作られたテーブルを失敗した migration の down で片付け → force(1つ前の版)→ up。

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// dropAllTables は DB のテーブルを schema_migrations も含めてすべて消す(dirty な DB は
// DownAll で戻せないため。テストの前後の掃除専用)。
func dropAllTables(t *testing.T, conn *sql.DB) {
	t.Helper()
	rows, err := conn.Query(`SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	rows.Close()
	if len(names) == 0 {
		return
	}
	stmt := "SET FOREIGN_KEY_CHECKS = 0; DROP TABLE IF EXISTS `" + strings.Join(names, "`, `") + "`; SET FOREIGN_KEY_CHECKS = 1"
	if _, err := conn.Exec(stmt); err != nil {
		t.Fatalf("テーブルを消せない: %v", err)
	}
}

// emptyDB はテーブルが1つも無い DB への接続を返す(終わったら再び空にして、他のテストの freshDB が
// dirty な DB に当たらないようにする)。
func emptyDB(t *testing.T) *sql.DB {
	t.Helper()
	_, cfg := testDSN(t)
	c := cfg.Clone()
	c.MultiStatements = true
	conn, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Ping(); err != nil {
		t.Fatalf("DB に届かない(スキップしない): %v", err)
	}
	dropAllTables(t, conn)
	t.Cleanup(func() {
		dropAllTables(t, conn)
		conn.Close()
	})
	return conn
}

func mustVersion(t *testing.T, dsn string) (uint, bool) {
	t.Helper()
	v, dirty, ok, err := Version(dsn)
	if err != nil || !ok {
		t.Fatalf("Version: v=%d ok=%v err=%v", v, ok, err)
	}
	return v, dirty
}

// runDownFile は migration の down の SQL をそのまま流す(手順書で mysql クライアントに流すのと同じ)。
func runDownFile(t *testing.T, conn *sql.DB, version int) {
	t.Helper()
	_, _, down := migrationPairs(t)
	raw, err := os.ReadFile(down[version])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(string(raw)); err != nil {
		t.Fatalf("版 %d の down を流せない: %v", version, err)
	}
}

// #279 の再現: 既存テーブルと衝突して 000002 が途中で止まる → 手順書どおりに戻して up が最後まで通る。
func TestRecoverFromPartiallyAppliedMigration(t *testing.T) {
	conn := emptyDB(t)
	dsn, cfg := testDSN(t)
	versions, _, _ := migrationPairs(t)
	latest := uint(versions[len(versions)-1])

	if _, err := conn.Exec(`CREATE TABLE species (x INT)`); err != nil {
		t.Fatal(err)
	}
	err := Up(dsn)
	if err == nil {
		t.Fatal("既存の species と衝突するのに Up が成功した")
	}
	msg := err.Error()
	// エラーは migration 名と MySQL のエラーの1行(SQL 全文を出さない)。
	if !strings.Contains(msg, "000002_create_master") || strings.Contains(msg, "CREATE TABLE") || strings.Contains(msg, "\n") {
		t.Errorf("Up のエラーが「migration 名: MySQL のエラー」の1行でない: %q", msg)
	}
	var me *mysql.MySQLError
	if !errors.As(err, &me) || me.Number != 1050 {
		t.Errorf("Up のエラーから MySQL の 1050(already exists)を取り出せない: %v", err)
	}
	if v, dirty := mustVersion(t, dsn); v != 2 || !dirty {
		t.Fatalf("失敗後の版 = %d dirty=%v, want 2 / true", v, dirty)
	}
	if err := Up(dsn); err == nil || !strings.Contains(err.Error(), "Dirty") {
		t.Fatalf("dirty な DB への Up が Dirty で拒否されない: %v", err)
	}

	// 確認の DB 名が違えば拒否し、状態を変えない。
	if err := Force(dsn, cfg.DBName+"_other", 1); !errors.Is(err, ErrForceNotConfirmed) {
		t.Fatalf("確認の DB 名の不一致 = %v, want ErrForceNotConfirmed", err)
	}
	if err := Force(dsn, cfg.DBName, 99); !errors.Is(err, ErrForceUnknownVersion) {
		t.Fatalf("存在しない版 = %v, want ErrForceUnknownVersion", err)
	}
	if v, dirty := mustVersion(t, dsn); v != 2 || !dirty {
		t.Fatalf("拒否された force の後の版 = %d dirty=%v, want 2 / true(変えない)", v, dirty)
	}

	// 手順書: 途中まで作られたテーブルを 000002 の down で片付ける → 1つ前の版に force → up。
	runDownFile(t, conn, 2)
	if err := Force(dsn, cfg.DBName, 1); err != nil {
		t.Fatalf("Force(1): %v", err)
	}
	if v, dirty := mustVersion(t, dsn); v != 1 || dirty {
		t.Fatalf("force 後の版 = %d dirty=%v, want 1 / false", v, dirty)
	}
	if err := Up(dsn); err != nil {
		t.Fatalf("force 後の Up: %v", err)
	}
	if v, dirty := mustVersion(t, dsn); v != latest || dirty {
		t.Fatalf("復旧後の版 = %d dirty=%v, want %d / false", v, dirty, latest)
	}
	for _, want := range requiredTables {
		if !contains(userTables(t, conn), want) {
			t.Errorf("復旧後にテーブル %s が無い", want)
		}
	}
	seed(t, conn) // 復旧後のスキーマに seed が入る(衝突した species が作り直されている)
}

// #221 の再現: 最新版で dirty になった DB(スキーマは揃っている)を、同じ版への force で戻せる。
func TestForceClearsDirtyAtSameVersion(t *testing.T) {
	conn := emptyDB(t)
	dsn, cfg := testDSN(t)
	if err := Up(dsn); err != nil {
		t.Fatal(err)
	}
	latest, _ := mustVersion(t, dsn)
	if _, err := conn.Exec(`UPDATE schema_migrations SET dirty = 1`); err != nil {
		t.Fatal(err)
	}
	if err := Up(dsn); err == nil {
		t.Fatal("dirty な DB への Up が成功した")
	}
	if err := Force(dsn, cfg.DBName, int(latest)); err != nil {
		t.Fatalf("Force: %v", err)
	}
	if err := Up(dsn); err != nil {
		t.Fatalf("force 後の Up: %v", err)
	}
	if v, dirty := mustVersion(t, dsn); v != latest || dirty {
		t.Fatalf("版 = %d dirty=%v, want %d / false", v, dirty, latest)
	}
}

// 最初の migration(000001)で止まった DB は、0(未適用)への force で戻せる。
func TestForceZeroRecoversFirstMigration(t *testing.T) {
	conn := emptyDB(t)
	dsn, cfg := testDSN(t)
	versions, _, _ := migrationPairs(t)
	if _, err := conn.Exec(`CREATE TABLE type_chart (x INT)`); err != nil {
		t.Fatal(err)
	}
	if err := Up(dsn); err == nil {
		t.Fatal("既存の type_chart と衝突するのに Up が成功した")
	}
	if v, dirty := mustVersion(t, dsn); v != 1 || !dirty {
		t.Fatalf("失敗後の版 = %d dirty=%v, want 1 / true", v, dirty)
	}
	runDownFile(t, conn, 1)
	if err := Force(dsn, cfg.DBName, 0); err != nil {
		t.Fatalf("Force(0): %v", err)
	}
	if _, _, ok, err := Version(dsn); err != nil || ok {
		t.Fatalf("Force(0) の後は未適用のはず: ok=%v err=%v", ok, err)
	}
	if err := Up(dsn); err != nil {
		t.Fatalf("force 後の Up: %v", err)
	}
	if v, dirty := mustVersion(t, dsn); int(v) != versions[len(versions)-1] || dirty {
		t.Fatalf("復旧後の版 = %d dirty=%v", v, dirty)
	}
}

// dirty でない DB への force: 同じ版なら何もせず成功(冪等)、違う版は拒否して版を変えない。
func TestForceOnCleanDB(t *testing.T) {
	emptyDB(t)
	dsn, cfg := testDSN(t)
	if err := Up(dsn); err != nil {
		t.Fatal(err)
	}
	latest, _ := mustVersion(t, dsn)
	for i := 0; i < 2; i++ {
		if err := Force(dsn, cfg.DBName, int(latest)); err != nil {
			t.Fatalf("同じ版への force(%d 回目): %v", i+1, err)
		}
	}
	if err := Force(dsn, cfg.DBName, int(latest)-1); !errors.Is(err, ErrForceNotDirty) {
		t.Fatalf("dirty でない DB で違う版への force = %v, want ErrForceNotDirty", err)
	}
	if v, dirty := mustVersion(t, dsn); v != latest || dirty {
		t.Fatalf("版 = %d dirty=%v, want %d / false(変えない)", v, dirty, latest)
	}
	if err := Up(dsn); err != nil {
		t.Fatalf("force の後の Up(変更なし): %v", err)
	}
}

// ADR-0124「影響」: 旧 000005 の down で version=4 dirty=true に止まった DB(000008〜000006 の表は消え、
// 000005 の CHECK 1..4 と slot 4 の行は残る)は、版 5 に force してから DownAll で最後まで戻せる。
func TestRecoverDownStuckAtSlot4(t *testing.T) {
	conn := emptyDB(t)
	dsn, cfg := testDSN(t)
	if err := Up(dsn); err != nil {
		t.Fatal(err)
	}
	seed(t, conn)
	if _, err := conn.Exec(`INSERT INTO species_abilities (species_key, slot, ability_id) VALUES ('9001-000', 4, 'testguard')`); err != nil {
		t.Fatal(err)
	}
	// 旧 down が止まった状態を作る: 000005 より後の版の down を流し、版を 4・dirty にする。
	versions, _, _ := migrationPairs(t)
	for i := len(versions) - 1; i >= 0 && versions[i] > 5; i-- {
		runDownFile(t, conn, versions[i])
	}
	if _, err := conn.Exec(`UPDATE schema_migrations SET version = 4, dirty = 1`); err != nil {
		t.Fatal(err)
	}
	if err := DownAll(dsn, cfg.DBName); err == nil {
		t.Fatal("dirty な DB への DownAll が成功した")
	}

	if err := Force(dsn, cfg.DBName, 5); err != nil {
		t.Fatalf("Force(5): %v", err)
	}
	if err := DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("force 後の DownAll: %v", err)
	}
	if left := userTables(t, conn); len(left) != 0 {
		t.Fatalf("down の後に残ったテーブル: %v", left)
	}
}
