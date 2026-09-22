// Command calc は calc-svc(ダメージ計算・一括計算・逆算の HTTP サービス。ADR-0200)。
//
// 設定は環境変数で渡す(1か所で読み込み、必須値は起動時に検証する。docs/coding-rules.md §2):
//
//	CALC_ADDR         待ち受けアドレス(既定 ":8080")
//	CALC_MASTER_URL   pokedex-svc のベース URL(GET /internal/pokedex/master からマスタ一式を取得する。ADR-0204)
//	CALC_MASTER_PATH  マスタ一式の JSON ファイル(MasterExport の形。k3d の local overlay と make dev 用)
//
// CALC_MASTER_URL と CALC_MASTER_PATH は**ちょうど1つ**を指定する。CALC_TYPECHART_PATH は廃止
// (相性表は MasterExport に含まれる)で、設定されていたら起動しない。
//   - ファイル方式: 起動時に読み、失敗したら非ゼロで終了する(フォールバックの既定データは持たない。ADR-0013)。
//   - URL 方式: HTTP サーバはすぐ起動し、バックグラウンドで取得を再試行する(指数バックオフ・上限あり・ctx で止まる)。
//     取得できるまで calc の3操作は 503 master_unavailable、GET /readyz は 503、GET /healthz は 200。
//     取得後の再取得はしない(マスタの更新は再起動で反映する)。
//
// 計算はイベント保存に依存しない(CLAUDE.md 絶対ルール5)。
//
// TODO(ADR-0204): spec-writer のスタブ。loadConfig / newHandler / backoffDelay は implementer が実装する
// (振る舞いは main_test.go・manifest_test.go が固定する)。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// 環境変数の名前。
const (
	envAddr       = "CALC_ADDR"
	envMasterURL  = "CALC_MASTER_URL"
	envMasterPath = "CALC_MASTER_PATH"
	// envTypeChartPath は廃止した環境変数(ADR-0204)。設定されていたら起動エラーにして、古い設定に気づかせる。
	envTypeChartPath = "CALC_TYPECHART_PATH"

	// defaultAddr は CALC_ADDR が未設定・空のときの待ち受けアドレス。
	defaultAddr = ":8080"

	// URL 方式のマスタ取得の既定値(ADR-0204 §3)。最初は短い間隔で再試行し(pokedex-svc と同時に起動する
	// ことが多い)、倍々に伸ばして上限で止める(pokedex-svc が長く落ちていても叩き続けない)。
	defaultMasterRetryInitial = 500 * time.Millisecond
	defaultMasterRetryMax     = 30 * time.Second
	// defaultMasterFetchTimeout は1回の取得のタイムアウト(マスタ一式は数百 KB 程度の想定)。
	defaultMasterFetchTimeout = 10 * time.Second

	// shutdownTimeout は ctx 終了後、進行中のリクエストを待つ猶予。
	shutdownTimeout = 5 * time.Second

	// http.Server のタイムアウト(critic 指摘 R7。遅い・止まったクライアントに接続を占有され続けない)。
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
)

// retryPolicy は URL 方式のマスタ取得の再試行間隔(指数バックオフ)。
type retryPolicy struct {
	Initial time.Duration // 1回目の失敗の後に待つ時間
	Max     time.Duration // 待つ時間の上限
}

// config は calc-svc の設定(環境変数から1度だけ読む)。MasterURL と MasterPath はちょうど一方が空でない。
type config struct {
	Addr               string
	MasterURL          string
	MasterPath         string
	MasterRetry        retryPolicy   // 環境変数では変えない(既定値。テストが短くする)
	MasterFetchTimeout time.Duration // 同上
}

var errNotImplemented = errors.New("未実装(ADR-0204)")

// loadConfig は環境変数から設定を読む。CALC_MASTER_URL と CALC_MASTER_PATH のちょうど1つ(空は未設定と同じ)、
// URL は http / https の絶対 URL、CALC_TYPECHART_PATH は未設定であること。CALC_ADDR が未設定・空なら defaultAddr。
func loadConfig(lookup func(string) (string, bool)) (config, error) {
	return config{}, errNotImplemented
}

// newHandler は設定に応じて HTTP ハンドラを作る。
//   - ファイル方式: マスタを読み込んでから返す。ファイルが無い・壊れている・不正ならエラー(部分的なデータで起動しない)。
//   - URL 方式: すぐに返し、ctx が続く間バックグラウンドで取得を再試行する(取得・検証の失敗はどちらも再試行)。
func newHandler(ctx context.Context, cfg config) (http.Handler, error) {
	return nil, errNotImplemented
}

// backoffDelay は attempt 回目(0 始まり)の失敗の後に待つ時間。Initial から倍々に伸ばし、Max で止める
// (大きな attempt でも桁あふれしない)。負の attempt は 0 とみなす。
func backoffDelay(attempt int, p retryPolicy) time.Duration {
	return 0
}

// run は設定を読み、マスタを読み込み、ctx が終わるまで待ち受ける。ctx が終わったら
// サーバを止めて nil を返す。起動前の失敗(設定・マスタ)はエラーで返す。
func run(ctx context.Context, lookup func(string) (string, bool)) error {
	cfg, err := loadConfig(lookup)
	if err != nil {
		return err
	}
	handler, err := newHandler(ctx, cfg)
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
