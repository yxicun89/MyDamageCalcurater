package dbmigrate

// DB に接続しない範囲の検査(make test で走る)。実 DB での dirty → force → up は
// services/pokedex/db/mysql_test.go(-tags mysql)。

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4/database"
	mysqlmigrate "github.com/golang-migrate/migrate/v4/database/mysql"
)

// 架空の migration 一式(版 1・2・5。版の飛びも「存在しない版」の検査に使う)。
var fakeMigrations = fstest.MapFS{
	"migrations/000001_create_a.up.sql":   {Data: []byte("CREATE TABLE a (id INT);")},
	"migrations/000001_create_a.down.sql": {Data: []byte("DROP TABLE IF EXISTS a;")},
	"migrations/000002_create_b.up.sql":   {Data: []byte("CREATE TABLE b (id INT);")},
	"migrations/000002_create_b.down.sql": {Data: []byte("DROP TABLE IF EXISTS b;")},
	"migrations/000005_alter_b.up.sql":    {Data: []byte("ALTER TABLE b ADD COLUMN c INT;")},
	"migrations/000005_alter_b.down.sql":  {Data: []byte("ALTER TABLE b DROP COLUMN c;")},
}

// 届かない接続先(接続前に拒否されることの確認。ここに接続しに行くとテストが遅く・失敗する)。
const unreachableDSN = "root:pw@tcp(127.0.0.1:1)/app_db?timeout=1s"

// Force は接続する前に、確認用 DB 名・版を検査して拒否する(issue #221)。
func TestForceRejectsBeforeConnecting(t *testing.T) {
	cases := []struct {
		name    string
		dsn     string
		confirm string
		version int
		want    error
	}{
		{"確認の DB 名が違う", unreachableDSN, "other_db", 1, ErrForceNotConfirmed},
		{"確認の DB 名が空", unreachableDSN, "", 1, ErrForceNotConfirmed},
		{"DSN に DB 名が無い", "root:pw@tcp(127.0.0.1:1)/", "", 1, ErrForceNotConfirmed},
		{"負の版", unreachableDSN, "app_db", -1, ErrForceUnknownVersion},
		{"存在しない版(飛び番)", unreachableDSN, "app_db", 3, ErrForceUnknownVersion},
		{"存在しない版(最新より大きい)", unreachableDSN, "app_db", 6, ErrForceUnknownVersion},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Force(tc.dsn, tc.confirm, tc.version, fakeMigrations)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Force = %v, want %v", err, tc.want)
			}
		})
	}
}

// 0(未適用に戻す)と実在する版は検査を通る(届かない DSN なので接続で失敗する。検査の種類のエラーではない)。
func TestForceAcceptsKnownVersionsAndZero(t *testing.T) {
	for _, v := range []int{0, 1, 2, 5} {
		err := Force(unreachableDSN, "app_db", v, fakeMigrations)
		if err == nil {
			t.Fatalf("版 %d: 届かない DSN なのに成功した", v)
		}
		if errors.Is(err, ErrForceNotConfirmed) || errors.Is(err, ErrForceUnknownVersion) {
			t.Errorf("版 %d が検査で拒否された: %v", v, err)
		}
	}
}

// migration の失敗は、migration 名と MySQL のエラーの1行だけを出し、SQL 全文を出さない(#279 の条件)。
// 元の MySQL のエラーは errors.As で取り出せる(呼び出し側がエラー番号で判定できる)。
func TestDescribeMigrationError(t *testing.T) {
	orig := &mysql.MySQLError{Number: 1050, Message: "Table 'b' already exists"}
	longSQL := "CREATE TABLE b (\n  id INT\n);\n" + strings.Repeat("-- 長い SQL\n", 100)
	for _, raw := range []error{
		database.Error{OrigErr: orig, Err: "migration failed", Query: []byte(longSQL)},
		&database.Error{OrigErr: orig, Err: "migration failed", Query: []byte(longSQL)},
	} {
		err := describeMigrationError(raw, "000002_create_b")
		msg := err.Error()
		if strings.Contains(msg, "CREATE TABLE") || strings.Contains(msg, "長い SQL") {
			t.Errorf("SQL 全文が出ている: %q", msg)
		}
		if strings.Count(msg, "\n") != 0 {
			t.Errorf("1行で出していない: %q", msg)
		}
		for _, want := range []string{"000002_create_b", "1050", "already exists"} {
			if !strings.Contains(msg, want) {
				t.Errorf("%q が無い: %q", want, msg)
			}
		}
		var me *mysql.MySQLError
		if !errors.As(err, &me) || me.Number != 1050 {
			t.Errorf("元の MySQL のエラーを errors.As で取り出せない: %v", err)
		}
	}

	// migration の SQL 由来でないエラー(dirty 等)はそのまま返す。
	other := errors.New("Dirty database version 2. Fix and force version.")
	if got := describeMigrationError(other, ""); !errors.Is(got, other) || got.Error() != other.Error() {
		t.Errorf("SQL 由来でないエラーが変わった: %v", got)
	}
}

// MigrationsTable は golang-migrate の既定名と同じ(既に作られた DB の表名を変えない)。
func TestMigrationsTableIsLibraryDefault(t *testing.T) {
	if MigrationsTable != mysqlmigrate.DefaultMigrationsTable {
		t.Errorf("MigrationsTable = %q, want %q(既存の DB の表名)", MigrationsTable, mysqlmigrate.DefaultMigrationsTable)
	}
}
