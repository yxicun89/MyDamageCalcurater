// Command migrate は pokedex-svc の DB migration を操作する CLI(ADR-0100 §5)。
//
//	migrate up
//	migrate version
//	migrate down -confirm <DB名>
//
// DSN(go-sql-driver/mysql 形式)は環境変数 POKEDEX_DATABASE_DSN から読む。
// down は誤操作によるデータ削除を防ぐため、DSN の DB 名と一致する -confirm を要求する
// (CLAUDE.md: DB のデータ削除は人間の確認が必要)。他のターゲット・スクリプト・k8s から
// down を呼ばない。
package main

import (
	"flag"
	"fmt"
	"os"

	"example.com/pokecalc/services/pokedex/db"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 1 {
		usage()
		return 2
	}
	dsn := os.Getenv("POKEDEX_DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "POKEDEX_DATABASE_DSN が設定されていない")
		return 1
	}

	switch args[0] {
	case "up":
		if err := db.Up(dsn); err != nil {
			fmt.Fprintln(os.Stderr, "migrate up:", err)
			return 1
		}
		fmt.Println("up: 完了")
		return 0
	case "version":
		v, dirty, ok, err := db.Version(dsn)
		if err != nil {
			fmt.Fprintln(os.Stderr, "migrate version:", err)
			return 1
		}
		if !ok {
			fmt.Println("version: 未適用")
			return 0
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
		return 0
	case "down":
		fs := flag.NewFlagSet("down", flag.ContinueOnError)
		confirm := fs.String("confirm", "", "削除する DB 名(人間の確認。DSN の DB 名と一致すること)")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if err := db.DownAll(dsn, *confirm); err != nil {
			fmt.Fprintln(os.Stderr, "migrate down:", err)
			return 1
		}
		fmt.Println("down: 完了")
		return 0
	default:
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "使い方: migrate up | version | down -confirm <DB名>")
}
