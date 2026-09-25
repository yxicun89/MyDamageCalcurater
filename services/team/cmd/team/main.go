// Command team は team-svc の唯一のバイナリ(ADR-0209・ADR-0211・ADR-0213)。
//
//	team serve   # HTTP(/api/team/*・/healthz・/readyz)+ NATS JetStream の購読
//
// 設定は環境変数(config.go)。TEAM_APP_DSN は必須、TEAM_NATS_URL は任意
// (未設定なら購読を無効化するだけで起動は失敗しない。CLAUDE.md 絶対ルール5)。
// migration は別バイナリ(cmd/migrate。deploy/k8s の Job)が行う。この serve は DB へ
// sql.Open するだけで、起動時に migration を走らせない(record-svc・pokedex-svc と同じ流儀)。
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/services/team/internal/events"
	"example.com/pokecalc/services/team/internal/httpapi"
	"example.com/pokecalc/services/team/internal/store"
)

// HTTP タイムアウトと shutdown timeout(calc-svc・pokedex-svc・record-svc と同じ考え方の値)。
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second
)

func lookupEnv(k string) string {
	v, _ := os.LookupEnv(k)
	return v
}

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig(lookupEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "team serve:", err)
		return 1
	}

	db, err := openDB(cfg.DatabaseDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "team serve: DB を開けない:", err)
		return 1
	}
	defer db.Close()

	st := store.New(db, cfg.PurgeBatchLimit)
	sub := events.New(cfg.NATSURL, events.NewHandler(st))
	defer sub.Shutdown()

	handler := httpapi.NewHandler(st)
	if err := serve(ctx, cfg.Addr, handler); err != nil {
		fmt.Fprintln(os.Stderr, "team serve:", err)
		return 1
	}
	return 0
}

// openDB は go-sql-driver/mysql の DSN として解釈し、parseTime を必ず有効にする
// (devices.last_seen_at 等の DATETIME を time.Time で読むため)。sql.Open は接続を確立しない
// (DB が無くても起動でき、DB を使う操作だけが 503 store_unavailable を返す。CLAUDE.md 絶対ルール5)。
func openDB(dsn string) (*sql.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("%s の形式が不正: %w", envAppDSN, err)
	}
	cfg.ParseTime = true
	return sql.Open("mysql", cfg.FormatDSN())
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// serve は newHTTPServer で組み立てたサーバーを ListenAndServe し、ctx の終了で
// shutdownTimeout 付きの Shutdown を呼ぶ(record-svc・pokedex-svc の serve と同じ形)。
func serve(ctx context.Context, addr string, handler http.Handler) error {
	srv := newHTTPServer(addr, handler)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("team-svc: 待ち受け開始", "addr", addr)
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
		return srv.Shutdown(shutdownCtx)
	}
}
