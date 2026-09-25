// Command migrate は pokedex-svc の DB migration を操作する CLI(ADR-0100 §5・ADR-0110)。
//
//	migrate up
//	migrate version
//	migrate down -confirm <DB名>
//	migrate force -version <版> -confirm <DB名>
//
// DSN(go-sql-driver/mysql 形式)は環境変数 POKEDEX_DATABASE_DSN から読む。
// down は誤操作によるデータ削除を防ぐため、DSN の DB 名と一致する -confirm を要求する
// (CLAUDE.md: DB のデータ削除は人間の確認が必要)。他のターゲット・スクリプト・k8s から
// down を呼ばない。
//
// force は途中で失敗して dirty になった DB の dirty を解き、版を指定の値にする(スキーマは変えない。
// issue #221)。migration の状態を人が書き換える操作なので、down と同じく DB 名の -confirm を要求する。
// 手順(途中まで作られたテーブルの片付け → force → up)は docs/runbooks/data.md。k8s の Job からは呼ばない。
//
// up は POKEDEX_PROVISION_DSN(root 相当)が設定されているとき、実際の migration の前に
// POKEDEX_READER_DSN・POKEDEX_IMPORTER_DSN・POKEDEX_DATABASE_DSN(migrator)から3ユーザーを
// プロビジョニングする(ADR-0110 決定3)。設定されていなければプロビジョニングを丸ごと
// スキップし、今までどおり POKEDEX_DATABASE_DSN で直接 migrate する(ローカル開発との後方互換)。
// importer の書き込み権限は表単位(schema_migrations を除く。ADR-0125)なので、migration で増えた表に
// 追随させるため、Up の後に importer だけもう一度プロビジョニングする。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"example.com/pokecalc/services/pokedex/db"
)

// cliEnv は run が使う環境(テストのため差し替え可能にする。cmd/import の cliEnv と同じ流儀)。
type cliEnv struct {
	Stdout, Stderr io.Writer
	Getenv         func(string) string
	Provision      func(rootDSN string, roles []db.RoleGrant) error
	Up             func(dsn string) error
	Version        func(dsn string) (version uint, dirty bool, ok bool, err error)
	DownAll        func(dsn, confirmDatabase string) error
	Force          func(dsn, confirmDatabase string, version int) error
}

func main() {
	os.Exit(run(os.Args[1:], productionEnv()))
}

func productionEnv() cliEnv {
	return cliEnv{
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Getenv:    os.Getenv,
		Provision: db.Provision,
		Up:        db.Up,
		Version:   db.Version,
		DownAll:   db.DownAll,
		Force:     db.Force,
	}
}

func run(args []string, env cliEnv) int {
	if len(args) < 1 {
		usage(env.Stderr)
		return 2
	}

	switch args[0] {
	case "up":
		return runUp(env)
	case "version":
		return runVersion(env)
	case "down":
		return runDown(args[1:], env)
	case "force":
		return runForce(args[1:], env)
	default:
		usage(env.Stderr)
		return 2
	}
}

func runUp(env cliEnv) int {
	dsn := env.Getenv("POKEDEX_DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(env.Stderr, "POKEDEX_DATABASE_DSN が設定されていない")
		return 1
	}

	provisionDSN := env.Getenv("POKEDEX_PROVISION_DSN")
	var roles []db.RoleGrant
	if provisionDSN != "" {
		var missing []string
		roles, missing = rolesFromEnv(env.Getenv, dsn)
		if len(missing) > 0 {
			fmt.Fprintf(env.Stderr, "up: 環境変数が設定されていない: %s\n", strings.Join(missing, ", "))
			return 2
		}
		if err := env.Provision(provisionDSN, roles); err != nil {
			fmt.Fprintln(env.Stderr, "up: プロビジョニングに失敗:", err)
			return 1
		}
	}

	if err := env.Up(dsn); err != nil {
		fmt.Fprintln(env.Stderr, "migrate up:", err)
		return 1
	}

	// 表単位の権限のロール(importer。ADR-0125)は、Up で増えた表にも付くよう Up の後に付け直す。
	if tableScoped := dataTableRoles(roles); len(tableScoped) > 0 {
		if err := env.Provision(provisionDSN, tableScoped); err != nil {
			fmt.Fprintln(env.Stderr, "up: migrate 後の権限の付け直しに失敗:", err)
			return 1
		}
	}
	fmt.Fprintln(env.Stdout, "up: 完了")
	return 0
}

// dataTableRoles は roles のうち表単位の権限(db.ScopeDataTables)のものだけを返す。
func dataTableRoles(roles []db.RoleGrant) []db.RoleGrant {
	var out []db.RoleGrant
	for _, r := range roles {
		if r.Scope == db.ScopeDataTables {
			out = append(out, r)
		}
	}
	return out
}

// rolesFromEnv は reader・importer・migrator の順で RoleGrant を組み立てる。migrator の DSN は
// POKEDEX_DATABASE_DSN(migratorDSN、呼び出し側で既に取得済み)と同じ値を使う(ADR-0110 決定3)。
// reader・importer の DSN が欠けていれば、欠けた環境変数名だけを返す(値は返さない)。
func rolesFromEnv(getenv func(string) string, migratorDSN string) (roles []db.RoleGrant, missing []string) {
	readerDSN := getenv("POKEDEX_READER_DSN")
	importerDSN := getenv("POKEDEX_IMPORTER_DSN")
	if readerDSN == "" {
		missing = append(missing, "POKEDEX_READER_DSN")
	}
	if importerDSN == "" {
		missing = append(missing, "POKEDEX_IMPORTER_DSN")
	}
	if len(missing) > 0 {
		return nil, missing
	}
	return []db.RoleGrant{
		{DSN: readerDSN, Privileges: db.ReaderPrivileges},
		// importer は schema_migrations を書き換えられないよう、書き込みを表単位にする(issue #312・ADR-0125)。
		{DSN: importerDSN, Privileges: db.ImporterPrivileges, Scope: db.ScopeDataTables},
		{DSN: migratorDSN, Privileges: db.MigratorPrivileges},
	}, nil
}

func runVersion(env cliEnv) int {
	dsn := env.Getenv("POKEDEX_DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(env.Stderr, "POKEDEX_DATABASE_DSN が設定されていない")
		return 1
	}
	v, dirty, ok, err := env.Version(dsn)
	if err != nil {
		fmt.Fprintln(env.Stderr, "migrate version:", err)
		return 1
	}
	if !ok {
		fmt.Fprintln(env.Stdout, "version: 未適用")
		return 0
	}
	fmt.Fprintf(env.Stdout, "version=%d dirty=%v\n", v, dirty)
	return 0
}

func runDown(args []string, env cliEnv) int {
	dsn := env.Getenv("POKEDEX_DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(env.Stderr, "POKEDEX_DATABASE_DSN が設定されていない")
		return 1
	}
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	confirm := fs.String("confirm", "", "削除する DB 名(人間の確認。DSN の DB 名と一致すること)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := env.DownAll(dsn, *confirm); err != nil {
		fmt.Fprintln(env.Stderr, "migrate down:", err)
		return 1
	}
	fmt.Fprintln(env.Stdout, "down: 完了")
	return 0
}

func runForce(args []string, env cliEnv) int {
	dsn := env.Getenv("POKEDEX_DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(env.Stderr, "POKEDEX_DATABASE_DSN が設定されていない")
		return 1
	}
	fs := flag.NewFlagSet("force", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	// 既定値 -1 は「指定なし」(0 は「未適用に戻す」という意味のある値なので既定値にしない)。
	version := fs.Int("version", -1, "dirty を解いたあとの版(migrations にある版。0 は未適用に戻す)")
	confirm := fs.String("confirm", "", "対象の DB 名(人間の確認。DSN の DB 名と一致すること)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(env.Stderr, "force: 余分な引数がある:", strings.Join(fs.Args(), " "))
		return 2
	}
	if *version < 0 {
		fmt.Fprintln(env.Stderr, "force: -version <版>(0 以上)を指定すること")
		return 2
	}
	if *confirm == "" {
		fmt.Fprintln(env.Stderr, "force: -confirm <DB名> を指定すること(人間の確認)")
		return 2
	}
	if err := env.Force(dsn, *confirm, *version); err != nil {
		fmt.Fprintln(env.Stderr, "migrate force:", err)
		return 1
	}
	fmt.Fprintf(env.Stdout, "force: 完了(version=%d dirty=false。続けて migrate up)\n", *version)
	return 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "使い方: migrate up | version | down -confirm <DB名> | force -version <版> -confirm <DB名>")
}
