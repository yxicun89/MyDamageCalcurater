// pokedex の DB 資格情報を用途別の最小権限へ分離する(issue #104・ADR-0110)。
//
// Provision は root(DDL・ユーザー管理権限を持つ)DSN で接続し、reader/importer/migrator の
// 3ユーザーを冪等に作成・パスワード同期・権限のリセット/付与する。呼び出し側(cmd/migrate)が
// パスワードそのものを Secret の値から作ることは無い(scripts/up.sh が openssl rand -hex で
// 生成した値が DSN に埋め込まれているだけ)。
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/services/internal/dbmigrate"
)

// 権限セット定数(ADR-0110 決定1)。CLI(cmd/migrate)と実 DB テスト(grants_mysql_test.go)が
// 同じ定数を参照することで、テストが本番の権限セットそのものを検査する。
const (
	// ReaderPrivileges は pokedex-svc(公開 API)用。SELECT のみ。
	ReaderPrivileges = "SELECT"
	// ImporterPrivileges は importer(CronJob)用。DDL・ユーザー管理は含まない。
	ImporterPrivileges = "SELECT, INSERT, UPDATE, DELETE"
	// MigratorPrivileges は migrate(Job)用。DDL を含むが CREATE USER 等のグローバル権限は含まない。
	MigratorPrivileges = "SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, INDEX, REFERENCES"
	// AppPrivileges は record-svc / team-svc 本体・失効 CronJob 用(ADR-0211 §4)。値は ImporterPrivileges
	// と同じだが、pokedex の importer 都合で ImporterPrivileges の値が変わったときに record/team の
	// app 権限が無関係に変わらないよう、意図的に別の定数として持つ。
	AppPrivileges = "SELECT, INSERT, UPDATE, DELETE"
)

// ErrInvalidRoleGrant は Provision への入力が安全でないときに返す(実際に接続する前に判定する)。
// errors.Is で判定できる。エラー文にパスワード・DSN を含めない。
var ErrInvalidRoleGrant = errors.New("db: invalid role grant")

// GrantScope は権限を付ける範囲(ADR-0125)。
type GrantScope int

const (
	// ScopeDatabase は `db`.* に Privileges をそのまま付ける(reader・migrator・record/team の app)。
	ScopeDatabase GrantScope = iota
	// ScopeDataTables は SELECT だけを `db`.* に付け、それ以外(INSERT/UPDATE/DELETE)は
	// migration の管理表(dbmigrate.MigrationsTable)を除く、いまある表ごとに付ける(importer。issue #312)。
	// 表の一覧は DB の information_schema(= 適用済みの migration)から引き、コードに列挙しない。
	// 新しい表は migrate up の後にもう一度 Provision するまで書き込めない(cmd/migrate が up の後に流す)。
	ScopeDataTables
)

// dataTablePrivilegeWords は ScopeDataTables に許す権限(表単位の DML だけ。DDL を表単位で配らない)。
var dataTablePrivilegeWords = map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true}

// RoleGrant は1ユーザーぶんのプロビジョニング指定(ADR-0110 決定4)。
type RoleGrant struct {
	// DSN は mysql.ParseDSN で解釈できる形式。User・Passwd・DBName を使う。
	DSN string
	// Privileges は GRANT 文にそのまま埋め込む権限セット(ReaderPrivileges 等の定数を渡す。
	// 呼び出し側がユーザー入力をそのまま渡すことは想定しない)。
	Privileges string
	// Scope は権限を付ける範囲。ゼロ値は ScopeDatabase(従来どおり DB 全体)。
	Scope GrantScope
}

// パスワードは scripts/up.sh が openssl rand -hex 16 で生成する16進文字列を前提とするが、
// Provision 自体は cloud overlay 等 up.sh 以外の生成元も受け付ける汎用の関数のため、
// 接続前に文字種・長さを検査する(ADR-0110 実装時の申し送り2)。
var (
	pwPattern       = regexp.MustCompile(`^[A-Za-z0-9]{16,}$`)
	usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	dbNamePattern   = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
)

// allowedPrivilegeWords は GRANT 文に許す権限だけ(ALL・GRANT OPTION・CREATE USER 等の
// グローバル/管理権限やその他の SQL 断片を拒否する)。
var allowedPrivilegeWords = map[string]bool{
	"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true,
	"CREATE": true, "ALTER": true, "DROP": true, "INDEX": true, "REFERENCES": true,
}

func invalidGrant(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidRoleGrant, reason)
}

// validatePrivileges は権限セットを検査し、正規化した権限の語(大文字)を返す。
func validatePrivileges(privs string, scope GrantScope) ([]string, error) {
	if strings.TrimSpace(privs) == "" {
		return nil, invalidGrant("権限が空")
	}
	var words []string
	for _, p := range strings.Split(privs, ",") {
		word := strings.ToUpper(strings.TrimSpace(p))
		if !allowedPrivilegeWords[word] {
			return nil, invalidGrant("許可されていない権限が含まれている")
		}
		switch scope {
		case ScopeDatabase:
		case ScopeDataTables:
			if !dataTablePrivilegeWords[word] {
				return nil, invalidGrant("表単位の範囲に DML 以外の権限が含まれている")
			}
		default:
			return nil, invalidGrant("権限の範囲が不正")
		}
		words = append(words, word)
	}
	return words, nil
}

type validatedRole struct {
	user, pw   string
	privileges string
	scope      GrantScope
	words      []string
}

// validateRoles は接続する前に rootDSN・roles すべてを検査する。危険な値があれば
// ErrInvalidRoleGrant を返す(ADR-0110 実装時の申し送り2・3)。
func validateRoles(rootDSN string, roles []RoleGrant) (dbName string, rootUser string, out []validatedRole, err error) {
	if len(roles) == 0 {
		return "", "", nil, invalidGrant("ロールが1つも無い")
	}
	rootCfg, err := mysql.ParseDSN(rootDSN)
	if err != nil {
		return "", "", nil, invalidGrant("root DSN を解釈できない")
	}
	if rootCfg.DBName == "" || !dbNamePattern.MatchString(rootCfg.DBName) {
		return "", "", nil, invalidGrant("root DSN の DB 名が不正")
	}

	seen := map[string]bool{}
	out = make([]validatedRole, 0, len(roles))
	for _, r := range roles {
		cfg, err := mysql.ParseDSN(r.DSN)
		if err != nil {
			return "", "", nil, invalidGrant("ロールの DSN を解釈できない")
		}
		if !usernamePattern.MatchString(cfg.User) {
			return "", "", nil, invalidGrant("ロールのユーザー名が不正")
		}
		if !pwPattern.MatchString(cfg.Passwd) {
			return "", "", nil, invalidGrant("ロールのパスワードが不正")
		}
		if cfg.DBName != rootCfg.DBName {
			return "", "", nil, invalidGrant("ロールの DB 名が root DSN と違う")
		}
		if cfg.User == rootCfg.User || cfg.User == "root" || strings.HasPrefix(cfg.User, "mysql.") {
			return "", "", nil, invalidGrant("ロールのユーザーが root または mysql の予約ユーザーと同じ")
		}
		if seen[cfg.User] {
			return "", "", nil, invalidGrant("同じユーザーが2回指定されている")
		}
		seen[cfg.User] = true
		words, err := validatePrivileges(r.Privileges, r.Scope)
		if err != nil {
			return "", "", nil, err
		}
		out = append(out, validatedRole{user: cfg.User, pw: cfg.Passwd, privileges: r.Privileges, scope: r.Scope, words: words})
	}
	return rootCfg.DBName, rootCfg.User, out, nil
}

// Provision は roles の各ユーザーを rootDSN の接続先(root DSN の DB 名の DB)に対して
// 冪等にプロビジョニングする(ADR-0110 決定4)。各ロールについて次を順に実行する:
//
//  1. CREATE USER IF NOT EXISTS
//  2. ALTER USER(パスワードを現在の値に同期。ローテーション対応)
//  3. REVOKE ALL PRIVILEGES, GRANT OPTION(前回までの権限をいったん剥がす)
//  4. GRANT <Privileges> ON `dbname`.*(決定1の権限セットちょうどを与える)。
//     Scope が ScopeDataTables のロールは、SELECT を `dbname`.* に、残りを migration の管理表を除く
//     いまある表ごとに与える(ADR-0125)
//
// ユーザー名・DB 名は識別子として、パスワードは値として SQL 文字列に埋め込むが、
// 接続前の検査でパスワードは英数字16文字以上、ユーザー名・DB 名も許された文字種に
// 限定しているため、引用符・バックスラッシュ・セミコロンを含み得ず injection の余地が無い
// (database/sql のプレースホルダは CREATE USER/GRANT の識別子・値に使えないため)。
func Provision(rootDSN string, roles []RoleGrant) error {
	dbName, _, validated, err := validateRoles(rootDSN, roles)
	if err != nil {
		return err
	}

	rootCfg, err := mysql.ParseDSN(rootDSN)
	if err != nil {
		// validateRoles で既に検査済みだが、念のため。
		return invalidGrant("root DSN を解釈できない")
	}
	conn, err := sql.Open("mysql", rootCfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("db: root DSN で接続できない: %w", err)
	}
	defer conn.Close()

	for _, r := range validated {
		if _, err := conn.Exec(fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'", r.user, r.pw)); err != nil {
			return fmt.Errorf("db: CREATE USER %s: %w", r.user, err)
		}
		if _, err := conn.Exec(fmt.Sprintf("ALTER USER '%s'@'%%' IDENTIFIED BY '%s'", r.user, r.pw)); err != nil {
			return fmt.Errorf("db: ALTER USER %s: %w", r.user, err)
		}
		if _, err := conn.Exec(fmt.Sprintf("REVOKE ALL PRIVILEGES, GRANT OPTION FROM '%s'@'%%'", r.user)); err != nil {
			return fmt.Errorf("db: REVOKE ALL %s: %w", r.user, err)
		}
		if r.scope == ScopeDataTables {
			if err := grantDataTables(conn, dbName, r); err != nil {
				return err
			}
			continue
		}
		if _, err := conn.Exec(fmt.Sprintf("GRANT %s ON `%s`.* TO '%s'@'%%'", r.privileges, dbName, r.user)); err != nil {
			return fmt.Errorf("db: GRANT %s: %w", r.user, err)
		}
	}
	return nil
}

// dataTables は dbName にいまある表のうち、migration の管理表を除いた名前を返す(ADR-0125)。
// 識別子として SQL に埋め込むので、許された文字種でない名前があれば拒否する。
func dataTables(conn *sql.DB, dbName string) ([]string, error) {
	rows, err := conn.Query(`SELECT table_name FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE' AND table_name <> ? ORDER BY table_name`,
		dbName, dbmigrate.MigrationsTable)
	if err != nil {
		return nil, fmt.Errorf("db: 表の一覧を読めない: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("db: 表の一覧を読めない: %w", err)
		}
		if !dbNamePattern.MatchString(n) {
			return nil, invalidGrant("表の名前に許されない文字がある")
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: 表の一覧を読めない: %w", err)
	}
	return names, nil
}

// grantDataTables は ScopeDataTables のロールに、SELECT を DB 全体(migration の管理表を含めて読める)、
// それ以外を migration の管理表を除く表ごとに与える。
func grantDataTables(conn *sql.DB, dbName string, r validatedRole) error {
	var tableWords []string
	for _, w := range r.words {
		if w == "SELECT" {
			if _, err := conn.Exec(fmt.Sprintf("GRANT SELECT ON `%s`.* TO '%s'@'%%'", dbName, r.user)); err != nil {
				return fmt.Errorf("db: GRANT %s: %w", r.user, err)
			}
			continue
		}
		tableWords = append(tableWords, w)
	}
	if len(tableWords) == 0 {
		return nil
	}
	tables, err := dataTables(conn, dbName)
	if err != nil {
		return err
	}
	privs := strings.Join(tableWords, ", ")
	for _, tbl := range tables {
		if _, err := conn.Exec(fmt.Sprintf("GRANT %s ON `%s`.`%s` TO '%s'@'%%'", privs, dbName, tbl, r.user)); err != nil {
			return fmt.Errorf("db: GRANT %s ON %s: %w", r.user, tbl, err)
		}
	}
	return nil
}
