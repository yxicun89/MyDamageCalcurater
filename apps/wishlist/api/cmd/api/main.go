// Command api は wishlist の API サーバーと DB マイグレーション。
//
//	api serve        HTTP サーバー(PORT・WISHLIST_DATABASE_DSN・WISHLIST_API_TOKEN・WISHLIST_IMAGE_DIR・任意で WISHLIST_YAHOO_APPID)
//	api migrate up   migrations を適用(WISHLIST_DATABASE_DSN。DSN に multiStatements=true を足す。docs/design.md W-09)
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"example.com/pokecalc/apps/wishlist/api/internal/dbwait"
	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/httpapi"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
	"example.com/pokecalc/apps/wishlist/api/internal/ogp"
	"example.com/pokecalc/apps/wishlist/api/internal/refresh"
	"example.com/pokecalc/apps/wishlist/api/internal/storage"
	"example.com/pokecalc/apps/wishlist/api/migrations"
)

const usage = "usage: api serve | api migrate up"

// shutdownTimeout は SIGTERM を受けてから進行中のリクエストを待つ上限。
const shutdownTimeout = 10 * time.Second

// DefaultPort は PORT が無いときの待ち受けポート。
const DefaultPort = "8080"

var errMissingEnv = errors.New("missing required environment variable")

// serveConfig は serve の設定。
type serveConfig struct {
	Port     string
	DSN      string // serveDSN を通した後の DSN
	Token    string
	ImageDir string
	// YahooAppID は Yahoo!ショッピング API の appid(WISHLIST_YAHOO_APPID。任意。前後の空白を除く。空なら api 型のサイトは取得しない)。
	YahooAppID string
}

// migrateConfig は migrate の設定。
type migrateConfig struct {
	DSN string // migrateDSN を通した後の DSN
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

// run はサブコマンドを実行し、終了コードを返す。使い方の誤り(サブコマンド無し・不明)は 2、設定や実行の失敗は 1。
func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 1 && args[0] == "serve":
		cfg, err := loadServeConfig(getenv)
		if err != nil {
			fmt.Fprintln(stderr, "api serve:", err)
			return 1
		}
		if err := serve(cfg, stderr); err != nil {
			fmt.Fprintln(stderr, "api serve:", err)
			return 1
		}
		return 0
	case len(args) == 2 && args[0] == "migrate" && args[1] == "up":
		cfg, err := loadMigrateConfig(getenv)
		if err != nil {
			fmt.Fprintln(stderr, "api migrate up:", err)
			return 1
		}
		if err := migrateUp(cfg); err != nil {
			fmt.Fprintln(stderr, "api migrate up:", err)
			return 1
		}
		fmt.Fprintln(stdout, "migrate up: ok")
		return 0
	}
	fmt.Fprintln(stderr, usage)
	return 2
}

// loadServeConfig は serve の設定を環境変数から読む。WISHLIST_DATABASE_DSN・WISHLIST_API_TOKEN・WISHLIST_IMAGE_DIR は必須
// (空白だけも無いとみなす。errMissingEnv を包み、どの変数かをメッセージに含める)。PORT は省略時 DefaultPort、
// 数字でない・1〜65535 の外ならエラー。
func loadServeConfig(getenv func(string) string) (serveConfig, error) {
	dsn, err := requireEnv(getenv, "WISHLIST_DATABASE_DSN")
	if err != nil {
		return serveConfig{}, err
	}
	token, err := requireEnv(getenv, "WISHLIST_API_TOKEN")
	if err != nil {
		return serveConfig{}, err
	}
	dir, err := requireEnv(getenv, "WISHLIST_IMAGE_DIR")
	if err != nil {
		return serveConfig{}, err
	}
	port := DefaultPort
	if v := strings.TrimSpace(getenv("PORT")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 65535 {
			return serveConfig{}, fmt.Errorf("PORT must be a number between 1 and 65535")
		}
		port = strconv.Itoa(n)
	}
	sdsn, err := serveDSN(dsn)
	if err != nil {
		return serveConfig{}, err
	}
	return serveConfig{Port: port, DSN: sdsn, Token: token, ImageDir: dir, YahooAppID: strings.TrimSpace(getenv("WISHLIST_YAHOO_APPID"))}, nil
}

// loadMigrateConfig は migrate の設定を読む。WISHLIST_DATABASE_DSN だけが必須。
func loadMigrateConfig(getenv func(string) string) (migrateConfig, error) {
	dsn, err := requireEnv(getenv, "WISHLIST_DATABASE_DSN")
	if err != nil {
		return migrateConfig{}, err
	}
	mdsn, err := migrateDSN(dsn)
	if err != nil {
		return migrateConfig{}, err
	}
	return migrateConfig{DSN: mdsn}, nil
}

// serveDSN は API サーバー用の DSN にする: parseTime=true を必ず付け、multiStatements は付けない(付いていれば外す)。
// 解釈できない DSN はエラー。
func serveDSN(dsn string) (string, error) {
	cfg, err := parseDSN(dsn)
	if err != nil {
		return "", err
	}
	cfg.ParseTime = true
	cfg.MultiStatements = false
	return cfg.FormatDSN(), nil
}

// migrateDSN は migrate 用の DSN にする: multiStatements=true を足す(他のパラメータは保つ)。解釈できない DSN はエラー。
func migrateDSN(dsn string) (string, error) {
	cfg, err := parseDSN(dsn)
	if err != nil {
		return "", err
	}
	cfg.MultiStatements = true
	return cfg.FormatDSN(), nil
}

// parseDSN は DSN を解釈する。エラーに DSN(パスワードを含む)を出さない。
func parseDSN(dsn string) (*mysql.Config, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, errors.New("WISHLIST_DATABASE_DSN is not a valid MySQL DSN")
	}
	// 日時は UTC で読み書きする(DSN に loc=Asia/Tokyo 等が付いていても上書き。推移の日付 HistoryDay は 00:00 UTC の前提)。
	cfg.Loc = time.UTC
	return cfg, nil
}

// requireEnv は必須の環境変数を読む(空白だけも無いとみなす)。
func requireEnv(getenv func(string) string, name string) (string, error) {
	v := strings.TrimSpace(getenv(name))
	if v == "" {
		return "", fmt.Errorf("%w: %s", errMissingEnv, name)
	}
	return v, nil
}

// migrateUp は埋め込みの migrations を最新まで適用する(適用済みなら何もしない)。
func migrateUp(cfg migrateConfig) error {
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return err
	}
	defer db.Close()
	// Pod の起動直後は DB に繋がらないことがあるので再試行する(internal/dbwait)。
	if err := dbwait.Ping(context.Background(), db); err != nil {
		return fmt.Errorf("cannot reach the database: %w", err)
	}
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	drv, err := migratemysql.WithInstance(db, &migratemysql.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", src, "mysql", drv)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// serve は API サーバーを起動し、SIGINT/SIGTERM で穏やかに止める。
func serve(cfg serveConfig, stderr io.Writer) error {
	log := slog.New(slog.NewJSONHandler(stderr, nil))

	images, err := storage.NewFileStorage(cfg.ImageDir)
	if err != nil {
		return err
	}
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetMaxOpenConns(10)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if err := dbwait.Ping(ctx, db); err != nil {
		stop()
		return fmt.Errorf("cannot reach the database: %w", err)
	}

	repo := item.NewMySQLRepository(db)
	svc := item.NewService(repo, images)
	remote := ogp.NewFetcher(netguard.NewClient(netguard.Options{}))
	// 裏の更新はシグナルの ctx で動かし、終了時に止めて待つ(リクエストの ctx は使わない)。
	est := refresh.New(refresh.Deps{
		Items: repo, Prices: repo, Fetchers: fetcher.NewRegistry(fetcher.Config{YahooAppID: cfg.YahooAppID}),
		Logger: log, BaseContext: ctx,
	})
	defer func() {
		stop()
		est.Wait()
	}()
	e := httpapi.NewServer(httpapi.Deps{Items: svc, Images: images, Remote: remote, Estimates: est, Token: cfg.Token, Logger: log})

	srv := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           e,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "addr", srv.Addr)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutCtx, cancelShut := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShut()
	return srv.Shutdown(shutCtx)
}
