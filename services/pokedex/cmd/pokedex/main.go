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
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/services/pokedex/db"
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

// DB 接続プールの環境変数の名前と既定値(issue #112・ADR-0112 決定1)。
const (
	envDBMaxOpenConns    = "POKEDEX_DB_MAX_OPEN_CONNS"
	envDBMaxIdleConns    = "POKEDEX_DB_MAX_IDLE_CONNS"
	envDBConnMaxIdleTime = "POKEDEX_DB_CONN_MAX_IDLE_TIME"
	envDBConnMaxLifetime = "POKEDEX_DB_CONN_MAX_LIFETIME"

	defaultDBMaxOpenConns    = 10
	defaultDBMaxIdleConns    = 5
	defaultDBConnMaxIdleTime = 5 * time.Minute
	defaultDBConnMaxLifetime = 30 * time.Minute
)

// openPool は db.OpenPool への差し替え可能な package 変数(テストが runServe・runExport に
// 実際に渡った PoolConfig を横取りして検証できるようにするため。ADR-0112 決定3)。
var openPool = db.OpenPool

// HTTP タイムアウトと shutdown timeout(ADR-0111 決定1。services/balance・services/judge と同じ値)。
// shutdown timeout は10秒(calc-svc・gateway の5秒とは異なる値。ADR-0111 実装時の申し送り3)。
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 16 * 1024
	shutdownTimeout   = 10 * time.Second
)

// config は loadConfig の結果。DSN は parseTime=true を付けた後の値。
type config struct {
	Addr string
	DSN  string
	Pool db.PoolConfig
}

// loadConfig は環境変数から config を組み立てる。DSN は必須(空は未設定と同じ)。
// go-sql-driver/mysql の形式として解釈し、parseTime を必ず有効にする
// (regulations.starts_on/ends_on の DATE を sql.NullTime で読むため)。
// DB 接続プールの4変数も同じ関数・同じタイミング(sql.Open より前)で読み・検証する
// (issue #112・ADR-0112 決定2)。エラー文に DSN(パスワード)・値を含めない。
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

	pool, err := loadPoolConfig(lookup)
	if err != nil {
		return config{}, err
	}

	return config{Addr: addr, DSN: cfg.FormatDSN(), Pool: pool}, nil
}

// loadPoolConfig は DB 接続プールの4環境変数を読み・検証する(issue #112・ADR-0112 決定1・2)。
// エラー文には変数名だけを含め、値・DSN は含めない。
func loadPoolConfig(lookup func(string) (string, bool)) (db.PoolConfig, error) {
	maxOpen, err := lookupInt(lookup, envDBMaxOpenConns, defaultDBMaxOpenConns)
	if err != nil {
		return db.PoolConfig{}, err
	}
	if maxOpen <= 0 {
		return db.PoolConfig{}, fmt.Errorf("%s は正の整数である必要がある", envDBMaxOpenConns)
	}

	maxIdle, err := lookupInt(lookup, envDBMaxIdleConns, defaultDBMaxIdleConns)
	if err != nil {
		return db.PoolConfig{}, err
	}
	if maxIdle <= 0 {
		return db.PoolConfig{}, fmt.Errorf("%s は正の整数である必要がある", envDBMaxIdleConns)
	}
	if maxIdle > maxOpen {
		return db.PoolConfig{}, fmt.Errorf("%s は %s 以下である必要がある", envDBMaxIdleConns, envDBMaxOpenConns)
	}

	idleTime, err := lookupDuration(lookup, envDBConnMaxIdleTime, defaultDBConnMaxIdleTime)
	if err != nil {
		return db.PoolConfig{}, err
	}
	if idleTime <= 0 {
		return db.PoolConfig{}, fmt.Errorf("%s は正の duration である必要がある", envDBConnMaxIdleTime)
	}

	lifetime, err := lookupDuration(lookup, envDBConnMaxLifetime, defaultDBConnMaxLifetime)
	if err != nil {
		return db.PoolConfig{}, err
	}
	if lifetime <= 0 {
		return db.PoolConfig{}, fmt.Errorf("%s は正の duration である必要がある", envDBConnMaxLifetime)
	}

	return db.PoolConfig{
		MaxOpenConns:    maxOpen,
		MaxIdleConns:    maxIdle,
		ConnMaxIdleTime: idleTime,
		ConnMaxLifetime: lifetime,
	}, nil
}

// lookupInt は環境変数を strconv.Atoi で読む。未設定・空なら def を返す。
// エラー文には変数名だけを含める。
func lookupInt(lookup func(string) (string, bool), name string, def int) (int, error) {
	v, _ := lookup(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s の形式が不正(整数である必要がある)", name)
	}
	return n, nil
}

// lookupDuration は環境変数を time.ParseDuration で読む。未設定・空なら def を返す。
// エラー文には変数名だけを含める。
func lookupDuration(lookup func(string) (string, bool), name string, def time.Duration) (time.Duration, error) {
	v, _ := lookup(name)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s の形式が不正(time.Duration の形式である必要がある)", name)
	}
	return d, nil
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
		return runServeCmd()
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

// runServeCmd は signal.NotifyContext で ctx を作り runServe を呼ぶ薄いラッパー(ADR-0111
// 実装時の申し送り1。決定2原文の「main」層に相当する)。エラーをメッセージと終了コードに変換する。
func runServeCmd() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runServe(ctx, lookupEnv); err != nil {
		fmt.Fprintln(os.Stderr, "pokedex serve:", err)
		return 1
	}
	return 0
}

// newHTTPServer は ADR-0111 決定1のタイムアウト値を持つ *http.Server を組み立てるだけの
// 純粋関数(DB・設定を一切知らない)。
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
}

// serve は newHTTPServer で組み立てたサーバーを ListenAndServe し、ctx の終了(SIGINT/SIGTERM 相当)
// で shutdownTimeout 付きの Shutdown を呼ぶ(ADR-0111 決定2・実装時の申し送り2)。DB・DSN を
// 一切知らないため、返すエラーに DSN が混ざることは構造的にありえない。
func serve(ctx context.Context, addr string, handler http.Handler) error {
	srv := newHTTPServer(addr, handler)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("pokedex-svc: 待ち受け開始", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	}
}

// runServe は loadConfig → DB open → ハンドラ構築 → serve を結ぶ(ADR-0111 実装時の申し送り2)。
func runServe(ctx context.Context, lookup func(string) (string, bool)) error {
	cfg, err := loadConfig(lookup)
	if err != nil {
		return err
	}
	conn, err := openPool(cfg.DSN, cfg.Pool)
	if err != nil {
		return fmt.Errorf("DB を開けない: %w", err)
	}
	defer conn.Close()

	handler := httpapi.NewHandler(store.New(conn))
	return serve(ctx, cfg.Addr, handler)
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
	conn, err := openPool(cfg.DSN, cfg.Pool.ForExport())
	if err != nil {
		fmt.Fprintln(os.Stderr, "pokedex export: DB を開けない:", err)
		return 1
	}
	defer conn.Close()

	files, report, err := readmodel.Export(context.Background(), store.New(conn))
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
