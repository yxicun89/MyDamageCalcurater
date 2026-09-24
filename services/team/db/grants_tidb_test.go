//go:build tidb

package db

// db.Provision の実 TiDB での検査(ADR-0211 §4・AC-T4)。`make test-db` だけが実行する。
// TEAM_TEST_DSN(root 相当。CREATE USER・GRANT OPTION を持つこと)で、テスト専用の名前の
// ユーザー(record_t_*)を作り、終わったら DROP USER する。パスワードは毎回乱数で作り、
// ログ・エラー文に出さない。pokedex-svc の grants_mysql_test.go(実 MySQL 向け)と同じ構成を
// TiDB・2ロール(app・migrator)向けに書き直したもの(services/pokedex/db/grants.go の
// Provision/RoleGrant は import して再利用するため、検査対象のロジック自体は共通)。

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"

	pokedexdb "example.com/pokecalc/services/pokedex/db"
)

// TiDB のエラー番号(MySQL 互換のエラー番号をそのまま返す。権限・認証)。
const (
	errTableAccessDenied = 1142 // ER_TABLEACCESS_DENIED_ERROR
	errSpecificAccess    = 1227 // ER_SPECIFIC_ACCESS_DENIED_ERROR(CREATE USER 等)
	errDBAccessDenied    = 1044 // ER_DBACCESS_DENIED_ERROR
)

const (
	testAppUser      = "team_t_app"
	testMigratorUser = "team_t_migrator"
)

func randomPassword(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b) // scripts/up.sh の openssl rand -hex 16 と同じ形
}

// testDSN は TEAM_TEST_DSN を検証して返す。
func testDSN(t *testing.T) (dsn string, cfg *mysql.Config) {
	t.Helper()
	dsn = os.Getenv("TEAM_TEST_DSN")
	if dsn == "" {
		t.Fatal("TEAM_TEST_DSN が無い(make test-db は DB を前提にする。スキップしない)")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("TEAM_TEST_DSN を解釈できない: %v", err)
	}
	if !strings.HasSuffix(cfg.DBName, "_test") {
		t.Fatalf("DB 名は _test で終わること: %q", cfg.DBName)
	}
	return dsn, cfg
}

func roleDSN(rootCfg *mysql.Config, user, pass string) string {
	c := rootCfg.Clone()
	c.User = user
	c.Passwd = pass
	return c.FormatDSN()
}

type testRoleSet struct {
	app, migrator pokedexdb.RoleGrant
}

func (s testRoleSet) all() []pokedexdb.RoleGrant { return []pokedexdb.RoleGrant{s.app, s.migrator} }

func newTestRoles(t *testing.T, rootCfg *mysql.Config) testRoleSet {
	t.Helper()
	return testRoleSet{
		app:      pokedexdb.RoleGrant{DSN: roleDSN(rootCfg, testAppUser, randomPassword(t)), Privileges: pokedexdb.AppPrivileges},
		migrator: pokedexdb.RoleGrant{DSN: roleDSN(rootCfg, testMigratorUser, randomPassword(t)), Privileges: pokedexdb.MigratorPrivileges},
	}
}

func rootConn(t *testing.T) *sql.DB {
	t.Helper()
	dsn, _ := testDSN(t)
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := conn.Ping(); err != nil {
		t.Fatalf("DB に届かない(スキップしない): %v", err)
	}
	return conn
}

func dropTestUsers(t *testing.T, conn *sql.DB) {
	t.Helper()
	drop := func() {
		for _, u := range []string{testAppUser, testMigratorUser} {
			if _, err := conn.Exec("DROP USER IF EXISTS '" + u + "'@'%'"); err != nil {
				t.Errorf("DROP USER %s: %v", u, err)
			}
		}
	}
	drop()
	t.Cleanup(drop)
}

func openAs(t *testing.T, dsn string) (*sql.DB, error) {
	t.Helper()
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MultiStatements = true
	conn, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn, conn.Ping()
}

func mustOpenAs(t *testing.T, g pokedexdb.RoleGrant) *sql.DB {
	t.Helper()
	conn, err := openAs(t, g.DSN)
	if err != nil {
		t.Fatalf("プロビジョニングしたユーザーで接続できない: %v", err)
	}
	return conn
}

func mysqlErrNumber(err error) uint16 {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number
	}
	return 0
}

func expectDenied(t *testing.T, what string, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s が成功した(権限が無いはず)", what)
		return
	}
	switch mysqlErrNumber(err) {
	case errTableAccessDenied, errSpecificAccess, errDBAccessDenied:
	default:
		t.Errorf("%s の失敗が権限不足でない: %v", what, err)
	}
}

func expectOK(t *testing.T, what string, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("%s が失敗した: %v", what, err)
	}
}

// grantOnDB は識別子の引用符が MySQL(バッククォート)・TiDB(シングルクォート)のどちらでも
// 一致するようにする(SHOW GRANTS の出力形式が実装によって違うため)。
var grantOnDB = regexp.MustCompile("^GRANT (.+) ON [`']([^`']+)[`']\\.\\* TO [`']([^`']+)[`']@[`']%[`']$")

func grantedPrivileges(t *testing.T, conn *sql.DB, user, dbName string) []string {
	t.Helper()
	rows, err := conn.Query("SHOW GRANTS FOR '" + user + "'@'%'")
	if err != nil {
		t.Fatalf("SHOW GRANTS FOR %s: %v", user, err)
	}
	defer rows.Close()
	usageOnly := regexp.MustCompile("^GRANT USAGE ON \\*\\.\\* TO [`']" + regexp.QuoteMeta(user) + "[`']@[`']%[`']$")
	var privs []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		if usageOnly.MatchString(line) {
			continue
		}
		if strings.Contains(line, "WITH GRANT OPTION") {
			t.Errorf("%s に GRANT OPTION が付いている: %q", user, line)
		}
		m := grantOnDB.FindStringSubmatch(line)
		if m == nil || m[2] != dbName || m[3] != user {
			t.Errorf("%s に %s.* 以外の権限がある: %q", user, dbName, line)
			continue
		}
		for _, p := range strings.Split(m[1], ",") {
			privs = append(privs, strings.TrimSpace(p))
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(privs)
	return privs
}

func sortedPrivileges(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		out = append(out, strings.ToUpper(strings.TrimSpace(p)))
	}
	sort.Strings(out)
	return out
}

// AC-T4: 2ロールを2回続けてプロビジョニングしてもエラーにならず、権限はちょうど ADR-0211 §4 のとおり。
func TestProvisionIsIdempotent(t *testing.T) {
	root := rootConn(t)
	dropTestUsers(t, root)
	dsn, cfg := testDSN(t)
	roles := newTestRoles(t, cfg)

	for i := 1; i <= 2; i++ {
		if err := pokedexdb.Provision(dsn, roles.all()); err != nil {
			t.Fatalf("%d 回目の Provision: %v", i, err)
		}
	}
	for _, c := range []struct {
		user  string
		grant pokedexdb.RoleGrant
	}{
		{testAppUser, roles.app},
		{testMigratorUser, roles.migrator},
	} {
		got := grantedPrivileges(t, root, c.user, cfg.DBName)
		want := sortedPrivileges(c.grant.Privileges)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s の権限 = %v, want %v", c.user, got, want)
		}
		mustOpenAs(t, c.grant)
	}
}

// AC-T4: migrator は CREATE TABLE に成功し、app は失敗する(権限境界)。
func TestMigratorCanCreateTableAppCannot(t *testing.T) {
	root := rootConn(t)
	dropTestUsers(t, root)
	dsn, cfg := testDSN(t)
	roles := newTestRoles(t, cfg)
	if err := pokedexdb.Provision(dsn, roles.all()); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	migratorConn := mustOpenAs(t, roles.migrator)
	const table = "grants_boundary_probe"
	t.Cleanup(func() { _, _ = migratorConn.Exec("DROP TABLE IF EXISTS `" + table + "`") })
	_, err := migratorConn.Exec("CREATE TABLE `" + table + "` (id BIGINT PRIMARY KEY)")
	expectOK(t, "migrator の CREATE TABLE", err)

	appConn := mustOpenAs(t, roles.app)
	_, err = appConn.Exec("CREATE TABLE `" + table + "_app` (id BIGINT PRIMARY KEY)")
	expectDenied(t, "app の CREATE TABLE", err)
	if err == nil {
		_, _ = appConn.Exec("DROP TABLE IF EXISTS `" + table + "_app`")
	}

	// app は SELECT/INSERT/UPDATE/DELETE ができる(ADR-0211 §4 の AppPrivileges)。
	_, err = appConn.Exec("INSERT INTO `" + table + "` (id) VALUES (1)")
	expectOK(t, "app の INSERT", err)
	var got int
	expectOK(t, "app の SELECT", appConn.QueryRow("SELECT id FROM `"+table+"` WHERE id = 1").Scan(&got))
	_, err = appConn.Exec("UPDATE `" + table + "` SET id = 2 WHERE id = 1")
	expectOK(t, "app の UPDATE", err)
	_, err = appConn.Exec("DELETE FROM `" + table + "` WHERE id = 2")
	expectOK(t, "app の DELETE", err)
}
