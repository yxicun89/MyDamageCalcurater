// Command migrate は team-svc の DB migration を操作する CLI(ADR-0211 §4・§5)。
//
//	migrate up
//	migrate version
//	migrate down -confirm <DB名>
//
// DSN(go-sql-driver/mysql 形式)は環境変数 TEAM_DATABASE_DSN から読む。
// down は誤操作によるデータ削除を防ぐため、DSN の DB 名と一致する -confirm を要求する
// (CLAUDE.md: DB のデータ削除は人間の確認が必要)。他のターゲット・スクリプト・k8s から
// down を呼ばない。
//
// up は TEAM_PROVISION_DSN(root 相当。DB 名は team)が設定されているとき、実際の
// migration の前に TEAM_APP_DSN・TEAM_DATABASE_DSN(migrator)から2ユーザーを
// プロビジョニングする(ADR-0211 §4。pokedex の3ロールと違い app/migrator の2ロール)。
// 設定されていなければプロビジョニングを丸ごとスキップし、TEAM_DATABASE_DSN で
// 直接 migrate する(ローカル開発との後方互換)。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	pokedexdb "example.com/pokecalc/services/pokedex/db"
	teamdb "example.com/pokecalc/services/team/db"
)

// cliEnv は run が使う環境(テストのため差し替え可能にする。pokedex/cmd/migrate の cliEnv と同じ流儀)。
type cliEnv struct {
	Stdout, Stderr io.Writer
	Getenv         func(string) string
	Provision      func(rootDSN string, roles []pokedexdb.RoleGrant) error
	Up             func(dsn string) error
	Version        func(dsn string) (version uint, dirty bool, ok bool, err error)
	DownAll        func(dsn, confirmDatabase string) error
}

func main() {
	os.Exit(run(os.Args[1:], productionEnv()))
}

func productionEnv() cliEnv {
	return cliEnv{
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Getenv:    os.Getenv,
		Provision: pokedexdb.Provision,
		Up:        teamdb.Up,
		Version:   teamdb.Version,
		DownAll:   teamdb.DownAll,
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
	default:
		usage(env.Stderr)
		return 2
	}
}

func runUp(env cliEnv) int {
	dsn := env.Getenv("TEAM_DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(env.Stderr, "TEAM_DATABASE_DSN が設定されていない")
		return 1
	}

	if provisionDSN := env.Getenv("TEAM_PROVISION_DSN"); provisionDSN != "" {
		roles, missing := rolesFromEnv(env.Getenv, dsn)
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
	fmt.Fprintln(env.Stdout, "up: 完了")
	return 0
}

// rolesFromEnv は app・migrator の順で RoleGrant を組み立てる。migrator の DSN は
// TEAM_DATABASE_DSN(migratorDSN、呼び出し側で既に取得済み)と同じ値を使う(ADR-0211 §4)。
// app の DSN が欠けていれば、欠けた環境変数名だけを返す(値は返さない)。
func rolesFromEnv(getenv func(string) string, migratorDSN string) (roles []pokedexdb.RoleGrant, missing []string) {
	appDSN := getenv("TEAM_APP_DSN")
	if appDSN == "" {
		missing = append(missing, "TEAM_APP_DSN")
	}
	if len(missing) > 0 {
		return nil, missing
	}
	return []pokedexdb.RoleGrant{
		{DSN: appDSN, Privileges: pokedexdb.AppPrivileges},
		{DSN: migratorDSN, Privileges: pokedexdb.MigratorPrivileges},
	}, nil
}

func runVersion(env cliEnv) int {
	dsn := env.Getenv("TEAM_DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(env.Stderr, "TEAM_DATABASE_DSN が設定されていない")
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
	dsn := env.Getenv("TEAM_DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(env.Stderr, "TEAM_DATABASE_DSN が設定されていない")
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

func usage(w io.Writer) {
	fmt.Fprintln(w, "使い方: migrate up | version | down -confirm <DB名>")
}
