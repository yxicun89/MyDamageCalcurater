//go:build mysql

package db

// moves.target(技の対象。ADR-0136・issue #288)を実 MySQL で確かめる。`make test-db` だけが実行する。
//   - 既に技の行が入った版 9 の DB に migration 000010 が通る(既存の行は NULL = まだ取り込んでいない)
//   - CHECK が未知の値・大文字小文字違いを拒否し、取得元の綴りの値と NULL を受ける
//   - target が入った状態でも DownAll が最後まで通る(行が入った状態の down が失敗した前例 #278)

import (
	"database/sql"
	"errors"
	"io/fs"
	"path"
	"strconv"
	"testing"
	"testing/fstest"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/services/internal/dbmigrate"
)

// migrationsBefore は Migrations のうち版が version より小さいものだけを持つ FS を返す
// (「000010 を適用する前の、運用中の DB」を作るため)。
func migrationsBefore(t *testing.T, version int) fs.FS {
	t.Helper()
	names, err := fs.Glob(Migrations, "migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	out := fstest.MapFS{}
	for _, name := range names {
		m := migrationName.FindStringSubmatch(path.Base(name))
		if m == nil {
			t.Fatalf("migration の名前が規則に合わない: %s", name)
		}
		v, _ := strconv.Atoi(m[1])
		if v >= version {
			continue
		}
		raw, err := fs.ReadFile(Migrations, name)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = &fstest.MapFile{Data: raw}
	}
	return out
}

func TestMoveTargetMigratesPopulatedDB(t *testing.T) {
	dsn, cfg := testDSN(t)
	if err := DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("DownAll: %v", err)
	}
	if err := dbmigrate.Up(dsn, migrationsBefore(t, moveTargetMigrationVersion)); err != nil {
		t.Fatalf("版 %d より前までの Up: %v", moveTargetMigrationVersion, err)
	}
	c := cfg.Clone()
	c.MultiStatements = true
	conn, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// 運用中の DB と同じく、target の無い技の行が入っている状態(架空データ)。
	if _, err := conn.Exec(`INSERT INTO types (id, sort_order, name_ja, name_ja_source) VALUES ('fire', 1, 'テストほのお', 'override');
		INSERT INTO moves (id, name_ja, name_ja_source, name_en, type, category, power, accuracy, pp, priority)
		VALUES ('testflame', 'テストフレイム', 'override', 'Test Flame', 'fire', 'special', 90, 100, 15, 0)`); err != nil {
		t.Fatalf("版 9 の DB に技の行を入れられない: %v", err)
	}

	if err := Up(dsn); err != nil {
		t.Fatalf("技の行がある DB への migration %06d が失敗: %v", moveTargetMigrationVersion, err)
	}
	var target sql.NullString
	if err := conn.QueryRow(`SELECT target FROM moves WHERE id = 'testflame'`).Scan(&target); err != nil {
		t.Fatal(err)
	}
	if target.Valid {
		t.Errorf("既存の行の target = %q, want NULL(既定値で誤った対象を作らない。importer が入れる)", target.String)
	}

	if _, err := conn.Exec(`UPDATE moves SET target = 'allAdjacentFoes' WHERE id = 'testflame'`); err != nil {
		t.Fatalf("取得元の綴りの対象を入れられない: %v", err)
	}
	if err := DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("target が入った DB の DownAll: %v", err)
	}
	if left := userTables(t, conn); len(left) != 0 {
		t.Fatalf("down の後に残ったテーブル: %v", left)
	}
}

func TestMoveTargetConstraint(t *testing.T) {
	conn := freshDB(t)
	seed(t, conn)
	cases := []struct {
		name   string
		target string
		reject bool
	}{
		{"未知の値", "teleport", true},
		{"大文字小文字違い", "AllAdjacentFoes", true},
		{"正規化した綴り", "all_adjacent_foes", true},
		{"空文字", "", true},
		{"単体", "normal", false},
		{"相手全体", "allAdjacentFoes", false},
		{"自分以外の全体", "allAdjacent", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := conn.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback() //nolint:errcheck // 確かめるための文なので元に戻す
			_, err = tx.Exec(`UPDATE moves SET target = ? WHERE id = 'testflame'`, tc.target)
			if !tc.reject {
				if err != nil {
					t.Fatalf("DB が拒否した: %v", err)
				}
				return
			}
			var me *mysql.MySQLError
			if !errors.As(err, &me) || me.Number != errCheckViolated {
				t.Fatalf("CHECK 違反(%d)で拒否されない: %v", errCheckViolated, err)
			}
		})
	}
	// NULL(まだ取り込んでいない)は受ける。
	if _, err := conn.Exec(`UPDATE moves SET target = NULL WHERE id = 'testflame'`); err != nil {
		t.Fatalf("NULL を入れられない: %v", err)
	}
}
