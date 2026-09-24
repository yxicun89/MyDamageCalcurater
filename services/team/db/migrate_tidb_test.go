//go:build tidb

package db

// migrate 一式(Up/DownAll/Version)の実 TiDB での検査(ADR-0211 AC-T2・AC-T4)。
// `make test-db` だけが実行する。pokedex-svc の grants_mysql_test.go の
// TestMigratorRunsFullMigration を、record の2表・2ロール向けに書き直したもの。

import (
	"database/sql"
	"sort"
	"testing"

	pokedexdb "example.com/pokecalc/services/pokedex/db"
)

// userTables は dbName に実在するテーブル(schema_migrations を除く)を名前順で返す。
func userTables(t *testing.T, conn *sql.DB, dbName string) []string {
	t.Helper()
	rows, err := conn.Query(`SELECT table_name FROM information_schema.tables
		WHERE table_schema = ? AND table_name <> 'schema_migrations' ORDER BY table_name`, dbName)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return names
}

// AC-T2・AC-T4: 空の DB から migrator で migration 一式が最後まで通り(Up)、
// できるテーブルは devices・purge_journal だけ(pokedex のテーブルが混ざらないこと。
// ADR-0211 第1回 critic が見つけた旧設計の欠陥の再発防止)。DownAll(DROP)→ 再 Up も
// 冪等に成功する(AC-T2「up → down → up が冪等に成功する」)。
func TestMigratorRunsFullMigrationAndOnlyOwnTables(t *testing.T) {
	root := rootConn(t)
	dropTestUsers(t, root)
	dsn, cfg := testDSN(t)

	// root で全テーブルを消し、スキーマが空の状態から migrator に作らせる。
	if err := DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("DownAll(root): %v", err)
	}
	if _, err := root.Exec("DROP TABLE IF EXISTS schema_migrations"); err != nil {
		t.Fatalf("schema_migrations を消せない: %v", err)
	}

	roles := newTestRoles(t, cfg)
	if err := pokedexdb.Provision(dsn, roles.all()); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	if err := Up(roles.migrator.DSN); err != nil {
		t.Fatalf("migrator で Up: %v", err)
	}
	versions, _, _ := migrationPairs(t)
	v, dirty, ok, err := Version(roles.migrator.DSN)
	if err != nil || !ok || dirty || int(v) != versions[len(versions)-1] {
		t.Fatalf("migrator で Up 後の版 = %d dirty=%v ok=%v err=%v, want %d", v, dirty, ok, err, versions[len(versions)-1])
	}

	got := userTables(t, root, cfg.DBName)
	want := append([]string(nil), requiredTables...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("Up 後のテーブル = %v, want ちょうど %v(pokedex 等のテーブルが混ざっていないこと)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Up 後のテーブル = %v, want ちょうど %v", got, want)
		}
	}

	// AC-T2: up → down → up が冪等に成功する。
	if err := DownAll(roles.migrator.DSN, cfg.DBName); err != nil {
		t.Fatalf("migrator で DownAll(DROP): %v", err)
	}
	if err := Up(roles.migrator.DSN); err != nil {
		t.Fatalf("migrator で再 Up: %v", err)
	}
	got2 := userTables(t, root, cfg.DBName)
	if len(got2) != len(want) {
		t.Fatalf("再 Up 後のテーブル = %v, want ちょうど %v", got2, want)
	}
}
