// Command pokedex は pokedex-svc の唯一のバイナリ(ADR-0105 §1)。
//
//	pokedex serve            # HTTP(/api/pokedex/*・/internal/pokedex/master・/healthz)
//	pokedex export -out <dir> # balance・speed 向けの read model を4ファイル書く(ADR-0105 §5)
//
// 設定は環境変数 POKEDEX_DATABASE_DSN(必須)・POKEDEX_ADDR(既定 :8080)。
// serve は起動時に DB へ接続しない(sql.Open だけ。DB が無くても起動し、DB を使う操作が 503 を返す)。
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/services/pokedex/internal/httpapi"
	"example.com/pokecalc/services/pokedex/internal/readmodel"
	"example.com/pokecalc/services/pokedex/internal/store"
)

// 環境変数の名前と既定値(k8s のマニフェスト・Makefile と共有する。ADR-0105 §1・AC-K0)。
const (
	envAddr        = "POKEDEX_ADDR"
	envDatabaseDSN = "POKEDEX_DATABASE_DSN"
	defaultAddr    = ":8080"
)

// config は loadConfig の結果。DSN は parseTime=true を付けた後の値。
type config struct {
	Addr string
	DSN  string
}

// loadConfig は環境変数から config を組み立てる。DSN は必須(空は未設定と同じ)。
// go-sql-driver/mysql の形式として解釈し、parseTime を必ず有効にする
// (regulations.starts_on/ends_on の DATE を sql.NullTime で読むため)。
// エラー文に DSN(パスワード)を含めない。
func loadConfig(lookup func(string) (string, bool)) (config, error) {
	dsn, _ := lookup(envDatabaseDSN)
	if dsn == "" {
		return config{}, fmt.Errorf("%s が設定されていない", envDatabaseDSN)
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return config{}, fmt.Errorf("%s の形式が不正: %v", envDatabaseDSN, err)
	}
	cfg.ParseTime = true

	addr, _ := lookup(envAddr)
	if addr == "" {
		addr = defaultAddr
	}
	return config{Addr: addr, DSN: cfg.FormatDSN()}, nil
}

func lookupEnv(k string) (string, bool) { return os.LookupEnv(k) }

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 1 {
		usage()
		return 2
	}
	switch args[0] {
	case "serve":
		return runServe()
	case "export":
		return runExport(args[1:])
	default:
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "使い方: pokedex serve | pokedex export -out <dir>")
}

func runServe() int {
	cfg, err := loadConfig(lookupEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pokedex serve:", err)
		return 1
	}
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pokedex serve: DB を開けない:", err)
		return 1
	}
	defer db.Close()

	handler := httpapi.NewHandler(store.New(db))
	slog.Info("pokedex-svc: 待ち受け開始", "addr", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, handler); err != nil {
		fmt.Fprintln(os.Stderr, "pokedex serve:", err)
		return 1
	}
	return 0
}

func runExport(args []string) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	out := fs.String("out", "", "read model の出力先ディレクトリ")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *out == "" {
		fmt.Fprintln(os.Stderr, "pokedex export: -out が要る")
		return 2
	}
	cfg, err := loadConfig(lookupEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pokedex export:", err)
		return 1
	}
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pokedex export: DB を開けない:", err)
		return 1
	}
	defer db.Close()

	files, report, err := readmodel.Export(context.Background(), store.New(db))
	if err != nil {
		fmt.Fprintln(os.Stderr, "pokedex export:", err)
		return 1
	}
	if err := files.WriteDir(*out); err != nil {
		fmt.Fprintln(os.Stderr, "pokedex export:", err)
		return 1
	}
	if n := len(report.TruncatedAbilities); n > 0 {
		fmt.Fprintf(os.Stderr, "pokedex export: %d 件の特性を balance の上限を超えて落とした:\n", n)
		for _, t := range report.TruncatedAbilities {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", t.PokemonID, t.AbilityID)
		}
	}
	fmt.Printf("export: %s に書いた\n", *out)
	return 0
}
