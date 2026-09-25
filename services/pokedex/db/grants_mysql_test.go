//go:build mysql

package db

// db.Provision の実 MySQL での検査(ADR-0110 受け入れ条件 1〜4)。`make test-db` だけが実行する。
// POKEDEX_TEST_DSN(root 相当。CREATE USER・GRANT OPTION を持つこと)で、テスト専用の名前の
// ユーザー(pokedex_t_*)を作り、終わったら DROP USER する。パスワードは毎回乱数で作り、
// ログ・エラー文に出さない。

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// MySQL のエラー番号(権限・認証)。
const (
	errTableAccessDenied = 1142 // ER_TABLEACCESS_DENIED_ERROR(SELECT/INSERT/DELETE/CREATE/DROP 等)
	errAccessDenied      = 1045 // ER_ACCESS_DENIED_ERROR(パスワード違い)
	errSpecificAccess    = 1227 // ER_SPECIFIC_ACCESS_DENIED_ERROR(CREATE USER 等)
	errDBAccessDenied    = 1044 // ER_DBACCESS_DENIED_ERROR
)

// テスト専用のユーザー名(本番の pokedex_reader 等と衝突させない)。
const (
	testReaderUser   = "pokedex_t_reader"
	testImporterUser = "pokedex_t_importer"
	testMigratorUser = "pokedex_t_migrator"
)

func randomPassword(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b) // scripts/up.sh の openssl rand -hex 16 と同じ形
}

// roleDSN は root DSN の接続先・DB 名のまま、ユーザーとパスワードだけを差し替えた DSN を返す。
func roleDSN(rootCfg *mysql.Config, user, pass string) string {
	c := rootCfg.Clone()
	c.User = user
	c.Passwd = pass
	return c.FormatDSN()
}

// testRoles は3ロールぶんの RoleGrant と、各ロールのパスワードを返す。
type testRoleSet struct {
	reader, importer, migrator RoleGrant
	pass                       map[string]string // user → password
}

func (s testRoleSet) all() []RoleGrant { return []RoleGrant{s.reader, s.importer, s.migrator} }

func newTestRoles(t *testing.T, rootCfg *mysql.Config) testRoleSet {
	t.Helper()
	s := testRoleSet{pass: map[string]string{}}
	for _, u := range []string{testReaderUser, testImporterUser, testMigratorUser} {
		s.pass[u] = randomPassword(t)
	}
	s.reader = RoleGrant{DSN: roleDSN(rootCfg, testReaderUser, s.pass[testReaderUser]), Privileges: ReaderPrivileges}
	s.importer = RoleGrant{DSN: roleDSN(rootCfg, testImporterUser, s.pass[testImporterUser]), Privileges: ImporterPrivileges, Scope: ScopeDataTables}
	s.migrator = RoleGrant{DSN: roleDSN(rootCfg, testMigratorUser, s.pass[testMigratorUser]), Privileges: MigratorPrivileges}
	return s
}

// rootConn は POKEDEX_TEST_DSN で開いた接続(ユーザーの後片付け・権限の確認に使う)。
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

// dropTestUsers はテスト専用ユーザーを前後で消す(前回の失敗の残りも掃除する)。
func dropTestUsers(t *testing.T, conn *sql.DB) {
	t.Helper()
	drop := func() {
		for _, u := range []string{testReaderUser, testImporterUser, testMigratorUser} {
			if _, err := conn.Exec("DROP USER IF EXISTS '" + u + "'@'%'"); err != nil {
				t.Errorf("DROP USER %s: %v", u, err)
			}
		}
	}
	drop()
	t.Cleanup(drop)
}

// openAs は RoleGrant の DSN で接続する(Ping まで)。
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

func mustOpenAs(t *testing.T, g RoleGrant) *sql.DB {
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

// expectDenied は err が権限不足のエラーであること。
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

var grantOnDB = regexp.MustCompile("^GRANT (.+) ON `([^`]+)`\\.\\* TO `([^`]+)`@`%`$")

var grantOnTable = regexp.MustCompile("^GRANT (.+) ON `([^`]+)`\\.`([^`]+)` TO `([^`]+)`@`%`$")

// grantedPrivileges は SHOW GRANTS からそのユーザーの DB 全体(`db`.*)への権限を読む。
// USAGE ON *.* 以外のグローバル権限・他 DB への権限・表単位の権限・WITH GRANT OPTION があれば失敗にする。
func grantedPrivileges(t *testing.T, conn *sql.DB, user, dbName string) []string {
	t.Helper()
	privs, tables := grantsOf(t, conn, user, dbName)
	if len(tables) > 0 {
		t.Errorf("%s に表単位の権限がある: %v", user, tables)
	}
	return privs
}

// grantsOf は SHOW GRANTS からそのユーザーの DB 全体への権限と、表ごとの権限(表名 → 権限)を読む。
// USAGE ON *.* 以外のグローバル権限・他 DB への権限・WITH GRANT OPTION があれば失敗にする。
func grantsOf(t *testing.T, conn *sql.DB, user, dbName string) (dbPrivs []string, tablePrivs map[string][]string) {
	t.Helper()
	tablePrivs = map[string][]string{}
	rows, err := conn.Query("SHOW GRANTS FOR '" + user + "'@'%'")
	if err != nil {
		t.Fatalf("SHOW GRANTS FOR %s: %v", user, err)
	}
	defer rows.Close()
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		if line == "GRANT USAGE ON *.* TO `"+user+"`@`%`" {
			continue
		}
		if strings.Contains(line, "WITH GRANT OPTION") {
			t.Errorf("%s に GRANT OPTION が付いている: %q", user, line)
		}
		if m := grantOnTable.FindStringSubmatch(line); m != nil && m[2] == dbName && m[4] == user {
			var ps []string
			for _, p := range strings.Split(m[1], ",") {
				ps = append(ps, strings.TrimSpace(p))
			}
			sort.Strings(ps)
			tablePrivs[m[3]] = ps
			continue
		}
		m := grantOnDB.FindStringSubmatch(line)
		if m == nil || m[2] != dbName || m[3] != user {
			t.Errorf("%s に %s 以外の権限がある: %q", user, dbName, line)
			continue
		}
		for _, p := range strings.Split(m[1], ",") {
			dbPrivs = append(dbPrivs, strings.TrimSpace(p))
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(dbPrivs)
	return dbPrivs, tablePrivs
}

func sortedPrivileges(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		out = append(out, strings.ToUpper(strings.TrimSpace(p)))
	}
	sort.Strings(out)
	return out
}

// AC-1: 3ロールを2回続けてプロビジョニングしてもエラーにならず、権限はちょうど決定1のとおり。
func TestProvisionIsIdempotent(t *testing.T) {
	root := rootConn(t)
	dropTestUsers(t, root)
	dsn, cfg := testDSN(t)
	roles := newTestRoles(t, cfg)

	for i := 1; i <= 2; i++ {
		if err := Provision(dsn, roles.all()); err != nil {
			t.Fatalf("%d 回目の Provision: %v", i, err)
		}
	}
	for _, c := range []struct {
		user  string
		grant RoleGrant
	}{
		{testReaderUser, roles.reader},
		{testMigratorUser, roles.migrator},
	} {
		got := grantedPrivileges(t, root, c.user, cfg.DBName)
		want := sortedPrivileges(c.grant.Privileges)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s の権限 = %v, want %v", c.user, got, want)
		}
		mustOpenAs(t, c.grant)
	}
	assertImporterGrants(t, root, cfg.DBName)
	mustOpenAs(t, roles.importer)
}

// assertImporterGrants は importer の権限が ADR-0125 のとおりであること: DB 全体には SELECT だけ、
// migration の管理表を除くいまある表のそれぞれに INSERT・UPDATE・DELETE、管理表には書き込みの権限なし。
func assertImporterGrants(t *testing.T, root *sql.DB, dbName string) {
	t.Helper()
	dbPrivs, tables := grantsOf(t, root, testImporterUser, dbName)
	if strings.Join(dbPrivs, ",") != "SELECT" {
		t.Errorf("importer の DB 全体への権限 = %v, want [SELECT]", dbPrivs)
	}
	if _, ok := tables["schema_migrations"]; ok {
		t.Errorf("importer に schema_migrations への権限がある: %v", tables["schema_migrations"])
	}
	existing := userTables(t, root)
	for _, tbl := range existing {
		if got := strings.Join(tables[tbl], ","); got != "DELETE,INSERT,UPDATE" {
			t.Errorf("importer の %s への権限 = %q, want DELETE,INSERT,UPDATE", tbl, got)
		}
	}
	if len(tables) != len(existing) {
		t.Errorf("importer の表単位の権限の数 = %d, want いまある表(schema_migrations 以外)の数 %d", len(tables), len(existing))
	}
}

// AC-1: 前回より広い権限が付いていても、再プロビジョニングで決定1の権限ちょうどに戻る(REVOKE ALL)。
func TestProvisionRevokesStalePrivileges(t *testing.T) {
	root := rootConn(t)
	dropTestUsers(t, root)
	dsn, cfg := testDSN(t)
	roles := newTestRoles(t, cfg)

	wide := roles.reader
	wide.Privileges = MigratorPrivileges
	if err := Provision(dsn, []RoleGrant{wide}); err != nil {
		t.Fatalf("Provision(広い権限): %v", err)
	}
	if err := Provision(dsn, []RoleGrant{roles.reader}); err != nil {
		t.Fatalf("Provision(reader): %v", err)
	}
	got := grantedPrivileges(t, root, testReaderUser, cfg.DBName)
	if strings.Join(got, ",") != "SELECT" {
		t.Errorf("再プロビジョニング後の reader の権限 = %v, want [SELECT](前回の権限が残っている)", got)
	}
}

// AC-2: パスワードを変えて再プロビジョニングすると新しいパスワードだけで入れる。他のロールは巻き込まない。
func TestProvisionRotatesPassword(t *testing.T) {
	root := rootConn(t)
	dropTestUsers(t, root)
	dsn, cfg := testDSN(t)
	roles := newTestRoles(t, cfg)
	if err := Provision(dsn, roles.all()); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	oldReader := roles.reader

	rotated := roles
	rotated.reader = RoleGrant{DSN: roleDSN(cfg, testReaderUser, randomPassword(t)), Privileges: ReaderPrivileges}
	if err := Provision(dsn, rotated.all()); err != nil {
		t.Fatalf("Provision(ローテーション後): %v", err)
	}

	if _, err := openAs(t, oldReader.DSN); err == nil {
		t.Error("古いパスワードで reader に入れた(ローテーションされていない)")
	} else if n := mysqlErrNumber(err); n != errAccessDenied {
		t.Errorf("古いパスワードでの失敗が認証エラー(1045)でない: %d", n)
	}
	mustOpenAs(t, rotated.reader)
	// importer・migrator のパスワードは変えていないので、そのまま入れる。
	mustOpenAs(t, roles.importer)
	mustOpenAs(t, roles.migrator)
}

// AC-3: reader は SELECT だけ。INSERT・UPDATE・DELETE・DDL・ユーザー管理はできない。
func TestReaderPrivilegeBoundary(t *testing.T) {
	admin := freshDB(t)
	seed(t, admin)
	dropTestUsers(t, admin)
	dsn, cfg := testDSN(t)
	roles := newTestRoles(t, cfg)
	if err := Provision(dsn, roles.all()); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	r := mustOpenAs(t, roles.reader)

	var n int
	expectOK(t, "reader の SELECT", r.QueryRow(`SELECT COUNT(*) FROM species`).Scan(&n))
	if n == 0 {
		t.Error("reader の SELECT で seed の行が見えない")
	}
	_, err := r.Exec(insertNature("tzzreader", "テスト読取", "", ""))
	expectDenied(t, "reader の INSERT", err)
	_, err = r.Exec(`UPDATE natures SET name_en = 'Test' WHERE 1 = 0`)
	expectDenied(t, "reader の UPDATE", err)
	_, err = r.Exec(`DELETE FROM natures WHERE 1 = 0`)
	expectDenied(t, "reader の DELETE", err)
	_, err = r.Exec(`CREATE TABLE t_reader_ddl (id INT PRIMARY KEY)`)
	expectDenied(t, "reader の CREATE TABLE", err)
	_, err = r.Exec(`DROP TABLE natures`)
	expectDenied(t, "reader の DROP TABLE", err)
	_, err = r.Exec(`CREATE USER 'pokedex_t_evil'@'%' IDENTIFIED BY 'x0123456789abcdef'`)
	expectDenied(t, "reader の CREATE USER", err)
}

// AC-3: importer は DML(SELECT/INSERT/UPDATE/DELETE)だけ。DDL・ユーザー管理はできない。
func TestImporterPrivilegeBoundary(t *testing.T) {
	admin := freshDB(t)
	dropTestUsers(t, admin)
	dsn, cfg := testDSN(t)
	roles := newTestRoles(t, cfg)
	if err := Provision(dsn, roles.all()); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	im := mustOpenAs(t, roles.importer)

	_, err := im.Exec(insertNature("tzzimport", "テスト投入", "", ""))
	expectOK(t, "importer の INSERT", err)
	_, err = im.Exec(`UPDATE natures SET name_en = 'Test2' WHERE id = 'tzzimport'`)
	expectOK(t, "importer の UPDATE", err)
	var n int
	expectOK(t, "importer の SELECT", im.QueryRow(`SELECT COUNT(*) FROM natures WHERE id = 'tzzimport'`).Scan(&n))
	if n != 1 {
		t.Errorf("importer が入れた行が見えない: %d", n)
	}
	_, err = im.Exec(`DELETE FROM natures WHERE id = 'tzzimport'`)
	expectOK(t, "importer の DELETE", err)

	_, err = im.Exec(`CREATE TABLE t_importer_ddl (id INT PRIMARY KEY)`)
	expectDenied(t, "importer の CREATE TABLE", err)
	_, err = im.Exec(`ALTER TABLE natures ADD COLUMN t_extra INT NULL`)
	expectDenied(t, "importer の ALTER TABLE", err)
	_, err = im.Exec(`DROP TABLE natures`)
	expectDenied(t, "importer の DROP TABLE", err)
	_, err = im.Exec(`CREATE USER 'pokedex_t_evil'@'%' IDENTIFIED BY 'x0123456789abcdef'`)
	expectDenied(t, "importer の CREATE USER", err)
}

// AC-4: migrator で空の DB から migration 一式(CREATE TABLE・外部キー・インデックス)が最後まで通る。
// DownAll(DROP)→ Up も migrator だけでできる。ユーザー管理はできない。
func TestMigratorRunsFullMigration(t *testing.T) {
	admin := freshDB(t)
	dropTestUsers(t, admin)
	dsn, cfg := testDSN(t)
	// root で全テーブルを消し、スキーマが空の状態から migrator に作らせる。
	if err := DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("DownAll(root): %v", err)
	}
	if _, err := admin.Exec(`DROP TABLE IF EXISTS schema_migrations`); err != nil {
		t.Fatalf("schema_migrations を消せない: %v", err)
	}
	roles := newTestRoles(t, cfg)
	if err := Provision(dsn, roles.all()); err != nil {
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
	for _, want := range requiredTables {
		if !contains(userTables(t, admin), want) {
			t.Errorf("migrator の Up でテーブル %s ができていない", want)
		}
	}
	if err := DownAll(roles.migrator.DSN, cfg.DBName); err != nil {
		t.Fatalf("migrator で DownAll(DROP): %v", err)
	}
	if err := Up(roles.migrator.DSN); err != nil {
		t.Fatalf("migrator で再 Up: %v", err)
	}

	m := mustOpenAs(t, roles.migrator)
	_, err = m.Exec(`CREATE USER 'pokedex_t_evil'@'%' IDENTIFIED BY 'x0123456789abcdef'`)
	expectDenied(t, "migrator の CREATE USER", err)
	_, err = m.Exec(`CREATE DATABASE pokedex_t_evil_db`)
	expectDenied(t, "migrator の CREATE DATABASE", err)
}

// AC-1: 権限が1つも無い新規ユーザーに対する REVOKE ALL がエラーにならないこと(MySQL 9.7.2 で確認済み。
// 版を上げたときの退行検知)。Provision は新規ユーザーに対しても成功する。
func TestProvisionFreshUsersFromScratch(t *testing.T) {
	root := rootConn(t)
	dropTestUsers(t, root)
	dsn, cfg := testDSN(t)
	roles := newTestRoles(t, cfg)
	var exists int
	if err := root.QueryRow(`SELECT COUNT(*) FROM mysql.user WHERE user IN (?, ?, ?)`,
		testReaderUser, testImporterUser, testMigratorUser).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists != 0 {
		t.Fatalf("前提: テスト用ユーザーが残っている(%d)", exists)
	}
	if err := Provision(dsn, roles.all()); err != nil {
		t.Fatalf("新規ユーザーへの Provision: %v", err)
	}
}

// issue #312・ADR-0125: importer は schema_migrations を読めるが書き換えられない(migrate の状態を壊せない)。
// マスタのすべての表には DML ができる(表の一覧は DB から引いた、いまある表)。
func TestImporterCannotWriteMigrationsTable(t *testing.T) {
	admin := freshDB(t)
	dropTestUsers(t, admin)
	dsn, cfg := testDSN(t)
	roles := newTestRoles(t, cfg)
	if err := Provision(dsn, roles.all()); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	assertImporterGrants(t, admin, cfg.DBName)
	im := mustOpenAs(t, roles.importer)

	var v int
	expectOK(t, "importer の schema_migrations の SELECT", im.QueryRow(`SELECT version FROM schema_migrations`).Scan(&v))
	_, err := im.Exec(`UPDATE schema_migrations SET dirty = 1`)
	expectDenied(t, "importer の schema_migrations の UPDATE", err)
	_, err = im.Exec(`DELETE FROM schema_migrations`)
	expectDenied(t, "importer の schema_migrations の DELETE", err)
	_, err = im.Exec(`INSERT INTO schema_migrations (version, dirty) VALUES (999, 0)`)
	expectDenied(t, "importer の schema_migrations の INSERT", err)
	if got, dirty := mustVersion(t, dsn); int(got) != v || dirty {
		t.Errorf("版 = %d dirty=%v, want %d / false(importer が書き換えられた)", got, dirty, v)
	}

	// マスタの表にはこれまでどおり DML ができる(1件入れて戻す)。
	for _, tbl := range userTables(t, admin) {
		_, err := im.Exec("DELETE FROM `" + tbl + "` WHERE 1 = 0")
		expectOK(t, "importer の "+tbl+" の DELETE", err)
	}
	_, err = im.Exec(insertNature("tzzimport2", "テスト投入2", "", ""))
	expectOK(t, "importer の INSERT", err)
	_, err = im.Exec(`DELETE FROM natures WHERE id = 'tzzimport2'`)
	expectOK(t, "importer の DELETE", err)
}

// ADR-0125: migration で増えた表には、もう一度 Provision するまで importer は書き込めない
// (cmd/migrate が up の後に importer を付け直す理由。表の一覧をコードに持たない代わり)。
func TestImporterGrantsFollowNewTablesAfterReprovision(t *testing.T) {
	admin := freshDB(t)
	dropTestUsers(t, admin)
	dsn, cfg := testDSN(t)
	roles := newTestRoles(t, cfg)
	if err := Provision(dsn, roles.all()); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if _, err := admin.Exec(`CREATE TABLE t_new_after_provision (id INT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP TABLE IF EXISTS t_new_after_provision`) })
	im := mustOpenAs(t, roles.importer)
	_, err := im.Exec(`INSERT INTO t_new_after_provision (id) VALUES (1)`)
	expectDenied(t, "付け直す前の新しい表への INSERT", err)

	if err := Provision(dsn, []RoleGrant{roles.importer}); err != nil {
		t.Fatalf("Provision(importer の付け直し): %v", err)
	}
	_, err = im.Exec(`INSERT INTO t_new_after_provision (id) VALUES (1)`)
	expectOK(t, "付け直した後の新しい表への INSERT", err)
	assertImporterGrants(t, admin, cfg.DBName)
}
