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
)

// ErrInvalidRoleGrant は Provision への入力が安全でないときに返す(実際に接続する前に判定する)。
// errors.Is で判定できる。エラー文にパスワード・DSN を含めない。
var ErrInvalidRoleGrant = errors.New("pokedex/db: invalid role grant")

// RoleGrant は1ユーザーぶんのプロビジョニング指定(ADR-0110 決定4)。
type RoleGrant struct {
	// DSN は mysql.ParseDSN で解釈できる形式。User・Passwd・DBName を使う。
	DSN string
	// Privileges は GRANT 文にそのまま埋め込む権限セット(ReaderPrivileges 等の定数を渡す。
	// 呼び出し側がユーザー入力をそのまま渡すことは想定しない)。
	Privileges string
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

func validatePrivileges(privs string) error {
	if strings.TrimSpace(privs) == "" {
		return invalidGrant("権限が空")
	}
	for _, p := range strings.Split(privs, ",") {
		word := strings.ToUpper(strings.TrimSpace(p))
		if !allowedPrivilegeWords[word] {
			return invalidGrant("許可されていない権限が含まれている")
		}
	}
	return nil
}

type validatedRole struct {
	user, pw   string
	privileges string
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
		if err := validatePrivileges(r.Privileges); err != nil {
			return "", "", nil, err
		}
		out = append(out, validatedRole{user: cfg.User, pw: cfg.Passwd, privileges: r.Privileges})
	}
	return rootCfg.DBName, rootCfg.User, out, nil
}

// Provision は roles の各ユーザーを rootDSN の接続先(root DSN の DB 名の DB)に対して
// 冪等にプロビジョニングする(ADR-0110 決定4)。各ロールについて次を順に実行する:
//
//  1. CREATE USER IF NOT EXISTS
//  2. ALTER USER(パスワードを現在の値に同期。ローテーション対応)
//  3. REVOKE ALL PRIVILEGES, GRANT OPTION(前回までの権限をいったん剥がす)
//  4. GRANT <Privileges> ON `dbname`.*(決定1の権限セットちょうどを与える)
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
		return fmt.Errorf("pokedex/db: root DSN で接続できない: %w", err)
	}
	defer conn.Close()

	for _, r := range validated {
		if _, err := conn.Exec(fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'", r.user, r.pw)); err != nil {
			return fmt.Errorf("pokedex/db: CREATE USER %s: %w", r.user, err)
		}
		if _, err := conn.Exec(fmt.Sprintf("ALTER USER '%s'@'%%' IDENTIFIED BY '%s'", r.user, r.pw)); err != nil {
			return fmt.Errorf("pokedex/db: ALTER USER %s: %w", r.user, err)
		}
		if _, err := conn.Exec(fmt.Sprintf("REVOKE ALL PRIVILEGES, GRANT OPTION FROM '%s'@'%%'", r.user)); err != nil {
			return fmt.Errorf("pokedex/db: REVOKE ALL %s: %w", r.user, err)
		}
		if _, err := conn.Exec(fmt.Sprintf("GRANT %s ON `%s`.* TO '%s'@'%%'", r.privileges, dbName, r.user)); err != nil {
			return fmt.Errorf("pokedex/db: GRANT %s: %w", r.user, err)
		}
	}
	return nil
}
