// Command calc は calc-svc(ダメージ計算・一括計算・逆算の HTTP サービス。ADR-0016)。
//
// 設定は環境変数で渡す(1か所で読み込み、必須値は起動時に検証する。docs/coding-rules.md §2):
//
//	CALC_ADDR            待ち受けアドレス(既定 ":8080")
//	CALC_MASTER_PATH     マスタのスナップショット(暫定 JSON スキーマ。services/calc/README.md)。必須
//	CALC_TYPECHART_PATH  タイプ相性表(testdata/golden/typechart.json と同じ schema)。必須
//
// 起動時に両方をメモリへ読み込み、失敗したら非ゼロで終了する(フォールバックの既定データは持たない。ADR-0013)。
// 計算はイベント保存に依存しない(CLAUDE.md 絶対ルール5)。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/pokecalc/services/calc/internal/httpapi"
	"example.com/pokecalc/services/calc/internal/master"
)

// 環境変数の名前。
const (
	envAddr          = "CALC_ADDR"
	envMasterPath    = "CALC_MASTER_PATH"
	envTypeChartPath = "CALC_TYPECHART_PATH"

	// defaultAddr は CALC_ADDR が未設定・空のときの待ち受けアドレス。
	defaultAddr = ":8080"

	// shutdownTimeout は ctx 終了後、進行中のリクエストを待つ猶予。
	shutdownTimeout = 5 * time.Second

	// http.Server のタイムアウト(critic 指摘 R7。遅い・止まったクライアントに接続を占有され続けない)。
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
)

// config は calc-svc の設定(環境変数から1度だけ読む)。
type config struct {
	Addr          string
	MasterPath    string
	TypeChartPath string
}

// loadConfig は環境変数から設定を読む。必須の CALC_MASTER_PATH / CALC_TYPECHART_PATH が
// 未設定・空ならエラー。CALC_ADDR が未設定・空なら defaultAddr。
func loadConfig(lookup func(string) (string, bool)) (config, error) {
	addr, ok := lookup(envAddr)
	if !ok || addr == "" {
		addr = defaultAddr
	}
	masterPath, ok := lookup(envMasterPath)
	if !ok || masterPath == "" {
		return config{}, fmt.Errorf("%s が未設定", envMasterPath)
	}
	typeChartPath, ok := lookup(envTypeChartPath)
	if !ok || typeChartPath == "" {
		return config{}, fmt.Errorf("%s が未設定", envTypeChartPath)
	}
	return config{Addr: addr, MasterPath: masterPath, TypeChartPath: typeChartPath}, nil
}

// newHandler は設定のファイルからマスタと相性表を読み込み、HTTP ハンドラを作る。
// ファイルが無い・壊れている・スキーマ違反ならエラー(部分的なデータで起動しない)。
func newHandler(cfg config) (http.Handler, error) {
	mf, err := os.Open(cfg.MasterPath)
	if err != nil {
		return nil, fmt.Errorf("マスタのスナップショットを開けない(%s): %w", cfg.MasterPath, err)
	}
	defer mf.Close()
	snapshot, err := master.LoadSnapshot(mf)
	if err != nil {
		return nil, fmt.Errorf("マスタのスナップショットの読み込みに失敗(%s): %w", cfg.MasterPath, err)
	}

	tf, err := os.Open(cfg.TypeChartPath)
	if err != nil {
		return nil, fmt.Errorf("タイプ相性表を開けない(%s): %w", cfg.TypeChartPath, err)
	}
	defer tf.Close()
	chart, err := master.LoadTypeChart(tf)
	if err != nil {
		return nil, fmt.Errorf("タイプ相性表の読み込みに失敗(%s): %w", cfg.TypeChartPath, err)
	}

	store, err := master.New(snapshot, chart)
	if err != nil {
		return nil, fmt.Errorf("マスタの整合性検査に失敗: %w", err)
	}
	return httpapi.NewHandler(store), nil
}

// run は設定を読み、マスタを読み込み、ctx が終わるまで待ち受ける。ctx が終わったら
// サーバを止めて nil を返す。起動前の失敗(設定・マスタ)はエラーで返す。
func run(ctx context.Context, lookup func(string) (string, bool)) error {
	cfg, err := loadConfig(lookup)
	if err != nil {
		return err
	}
	handler, err := newHandler(cfg)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		<-serveErr
		return nil
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.LookupEnv); err != nil {
		slog.Error("calc-svc を起動できない", "error", err)
		os.Exit(1)
	}
}
