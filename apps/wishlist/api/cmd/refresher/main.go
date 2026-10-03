// Command refresher は全商品の目安価格を順番に更新する(CronJob wishlist-refresher。毎日 03:00 JST。apps/wishlist/CLAUDE.md §11)。
//
//	wishlist-refresher   (引数なし。WISHLIST_DATABASE_DSN 必須・WISHLIST_YAHOO_APPID・WISHLIST_CHROMIUM_PATH 任意)
//
// 1 商品の失敗で止めず、最後に件数と失敗数を 1 行で出す。終了コードは、商品があって全件失敗なら 1、設定の誤りは 1、
// 引数を付けたら 2、それ以外は 0。受け入れ条件は docs/phase3-api-spec.md の AC-C6〜C8。
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/apps/wishlist/api/internal/chromium"
	"example.com/pokecalc/apps/wishlist/api/internal/dbwait"
	"example.com/pokecalc/apps/wishlist/api/internal/fetcher"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/official"
	"example.com/pokecalc/apps/wishlist/api/internal/refresh"
)

const usage = "usage: wishlist-refresher (no arguments)"

// chromiumNoSandbox は --no-sandbox を付けるか(実イメージでの確認結果。docs/phase3-api-spec.md)。
const chromiumNoSandbox = true

var errMissingEnv = errors.New("missing required environment variable")

// config は refresher の設定。
type config struct {
	DSN        string // parseTime=true を付け、multiStatements を外した DSN
	YahooAppID string // 前後の空白を除く。空なら api 型のサイトは取得しない
	// ChromiumPath は headless-shell の実行ファイル(WISHLIST_CHROMIUM_PATH。前後の空白を除く)。空なら headless のサイト(メルカリ)は取得しない。
	ChromiumPath string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run は更新を実行して終了コードを返す。
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	cfg, err := loadConfig(getenv)
	if err != nil {
		fmt.Fprintln(stderr, "refresher:", err)
		return 1
	}
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		fmt.Fprintln(stderr, "refresher: cannot open the database")
		return 1
	}
	defer db.Close()
	db.SetConnMaxLifetime(3 * time.Minute)
	if err := dbwait.Ping(ctx, db); err != nil {
		fmt.Fprintln(stderr, "refresher: cannot reach the database")
		return 1
	}

	repo := item.NewMySQLRepository(db)
	reg := newRegistry(cfg)
	svc := refresh.New(refresh.Deps{
		Items: repo, Prices: repo, Fetchers: reg,
		// 公式ページの取得は価格の取得と同じホスト間隔(Gate)を共有する(フェーズ4-3)
		Official: official.NewChecker(official.Config{Gate: reg.Gate()}), Officials: repo,
		Logger: slog.New(slog.NewJSONHandler(stderr, nil)), BaseContext: ctx,
	})
	report, err := svc.RefreshAll(ctx)
	printReport(stdout, report)
	if err != nil {
		fmt.Fprintln(stderr, "refresher:", err)
		return 1
	}
	return exitCode(report)
}

// loadConfig は環境変数から設定を読む。WISHLIST_DATABASE_DSN は必須(空白だけも無いとみなす。errMissingEnv を包み変数名を含める)。
func loadConfig(getenv func(string) string) (config, error) {
	dsn := strings.TrimSpace(getenv("WISHLIST_DATABASE_DSN"))
	if dsn == "" {
		return config{}, fmt.Errorf("%w: WISHLIST_DATABASE_DSN", errMissingEnv)
	}
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		// エラーに DSN(パスワードを含む)を出さない。
		return config{}, errors.New("WISHLIST_DATABASE_DSN is not a valid MySQL DSN")
	}
	parsed.ParseTime = true
	parsed.Loc = time.UTC // loc=Asia/Tokyo 等が付いていても UTC で扱う(推移の日付は 00:00 UTC の前提)
	parsed.MultiStatements = false
	return config{
		DSN:          parsed.FormatDSN(),
		YahooAppID:   strings.TrimSpace(getenv("WISHLIST_YAHOO_APPID")),
		ChromiumPath: strings.TrimSpace(getenv("WISHLIST_CHROMIUM_PATH")),
	}, nil
}

// exitCode は結果から終了コードを決める(商品が 1 つ以上あって全件失敗なら 1、それ以外は 0)。
func exitCode(r refresh.AllReport) int {
	if r.Items > 0 && r.Failed >= r.Items {
		return 1
	}
	return 0
}

// printReport は結果を 1 行で出す(例 `refresher: items=3 failed=1 official=2 official_failed=0`)。
func printReport(w io.Writer, r refresh.AllReport) {
	fmt.Fprintf(w, "refresher: items=%d failed=%d official=%d official_failed=%d\n", r.Items, r.Failed, r.OfficialChecked, r.OfficialFailed)
}

// newRegistry は取得の登録表を作る。ChromiumPath があるときだけ headless(メルカリ)用の Renderer を渡す。
func newRegistry(cfg config) *fetcher.Registry {
	fc := fetcher.Config{YahooAppID: cfg.YahooAppID}
	if cfg.ChromiumPath != "" {
		fc.Renderer = chromium.New(cfg.ChromiumPath, chromium.Options{NoSandbox: chromiumNoSandbox})
	}
	return fetcher.NewRegistry(fc)
}
