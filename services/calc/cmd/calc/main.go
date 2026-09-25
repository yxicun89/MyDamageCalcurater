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
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"example.com/pokecalc/services/calc/internal/events"
	"example.com/pokecalc/services/calc/internal/httpapi"
	"example.com/pokecalc/services/calc/internal/master"
)

// 環境変数の名前。
const (
	envAddr       = "CALC_ADDR"
	envMasterURL  = "CALC_MASTER_URL"
	envMasterPath = "CALC_MASTER_PATH"
	// envNatsURL は NATS JetStream への計算イベント発行先(ADR-0212 §6)。任意(未設定なら発行しない)。
	envNatsURL = "CALC_NATS_URL"
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
	NatsURL            string        // 任意。空ならイベント発行を無効化する(ADR-0212 §6)
	MasterRetry        retryPolicy   // 環境変数では変えない(既定値。テストが短くする)
	MasterFetchTimeout time.Duration // 同上
}

// loadConfig は環境変数から設定を読む。CALC_MASTER_URL と CALC_MASTER_PATH のちょうど1つ(空は未設定と同じ)、
// URL は http / https の絶対 URL、CALC_TYPECHART_PATH は未設定であること。CALC_ADDR が未設定・空なら defaultAddr。
func loadConfig(lookup func(string) (string, bool)) (config, error) {
	if v, ok := lookup(envTypeChartPath); ok && v != "" {
		return config{}, fmt.Errorf("%s は廃止された(ADR-0204。相性表は CALC_MASTER_URL / CALC_MASTER_PATH のマスタ一式に含まれる)", envTypeChartPath)
	}

	addr := defaultAddr
	if v, ok := lookup(envAddr); ok && v != "" {
		addr = v
	}

	var natsURL string
	if v, ok := lookup(envNatsURL); ok {
		natsURL = v
	}

	var masterURL, masterPath string
	if v, ok := lookup(envMasterURL); ok {
		masterURL = v
	}
	if v, ok := lookup(envMasterPath); ok {
		masterPath = v
	}
	switch {
	case masterURL != "" && masterPath != "":
		return config{}, fmt.Errorf("%s と %s はちょうど1つを指定すること", envMasterURL, envMasterPath)
	case masterURL == "" && masterPath == "":
		return config{}, fmt.Errorf("%s か %s のどちらかが必要", envMasterURL, envMasterPath)
	case masterURL != "":
		if _, err := master.NewHTTPSource(masterURL, defaultMasterFetchTimeout); err != nil {
			return config{}, fmt.Errorf("%s が不正: %w", envMasterURL, err)
		}
	}

	return config{
		Addr:               addr,
		MasterURL:          masterURL,
		MasterPath:         masterPath,
		NatsURL:            natsURL,
		MasterRetry:        retryPolicy{Initial: defaultMasterRetryInitial, Max: defaultMasterRetryMax},
		MasterFetchTimeout: defaultMasterFetchTimeout,
	}, nil
}

// newHandler は設定に応じて HTTP ハンドラを作る。publisher は呼び出し側(run)がプロセス終了時に
// Shutdown を呼ぶために返す(ADR-0212 §6。nil にはならない。*events.Publisher は nil でも安全)。
//   - ファイル方式: マスタを読み込んでから返す。ファイルが無い・壊れている・不正ならエラー(部分的なデータで起動しない)。
//   - URL 方式: すぐに返し、ctx が続く間バックグラウンドで取得を再試行する(取得・検証の失敗はどちらも再試行)。
func newHandler(ctx context.Context, cfg config) (http.Handler, *events.Publisher, error) {
	publisher := events.New(cfg.NatsURL) // cfg.NatsURL が空なら nil(発行は無効。ADR-0212 §6)

	if cfg.MasterPath != "" {
		export, err := master.FileSource{Path: cfg.MasterPath}.Fetch(ctx)
		if err != nil {
			return nil, publisher, fmt.Errorf("マスタファイル %s を読めない: %w", cfg.MasterPath, err)
		}
		store, err := master.FromExport(export)
		if err != nil {
			return nil, publisher, fmt.Errorf("マスタファイル %s の検証に失敗: %w", cfg.MasterPath, err)
		}
		return httpapi.NewHandler(store, publisher), publisher, nil
	}

	src, err := master.NewHTTPSource(cfg.MasterURL, cfg.MasterFetchTimeout)
	if err != nil {
		return nil, publisher, fmt.Errorf("%s が不正: %w", envMasterURL, err)
	}
	var current atomic.Pointer[master.MemoryStore]
	go fetchMasterLoop(ctx, src, cfg.MasterRetry, &current)
	handler := httpapi.NewDeferredHandler(func() master.Store {
		s := current.Load()
		if s == nil {
			// 型付きの nil を master.Store として返さない(nil 判定が効かなくなるため)。
			return nil
		}
		return s
	}, publisher)
	return handler, publisher, nil
}

// fetchMasterLoop はマスタ一式が取得・検証できるまで指数バックオフで再試行し、成功したら current に
// 格納して戻る(取得後の再取得はしない。ADR-0204 §3)。ctx が終わったら再試行を止める。
func fetchMasterLoop(ctx context.Context, src master.Source, retry retryPolicy, current *atomic.Pointer[master.MemoryStore]) {
	for attempt := 0; ; attempt++ {
		export, err := src.Fetch(ctx)
		if err == nil {
			var store *master.MemoryStore
			store, err = master.FromExport(export)
			if err == nil {
				current.Store(store)
				return
			}
		}
		if ctx.Err() != nil {
			return
		}
		slog.Warn("calc-svc: マスタを取得できない。再試行する", "error", err, "attempt", attempt)
		timer := time.NewTimer(backoffDelay(attempt, retry))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// backoffDelay は attempt 回目(0 始まり)の失敗の後に待つ時間。Initial から倍々に伸ばし、Max で止める
// (大きな attempt でも桁あふれしない)。負の attempt は 0 とみなす。
func backoffDelay(attempt int, p retryPolicy) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := p.Initial
	for i := 0; i < attempt; i++ {
		if delay >= p.Max {
			return p.Max
		}
		delay *= 2
		if delay <= 0 || delay > p.Max {
			return p.Max
		}
	}
	return delay
}

// run は設定を読み、マスタを読み込み、ctx が終わるまで待ち受ける。ctx が終わったら
// サーバを止めて nil を返す。起動前の失敗(設定・マスタ)はエラーで返す。
func run(ctx context.Context, lookup func(string) (string, bool)) error {
	cfg, err := loadConfig(lookup)
	if err != nil {
		return err
	}
	handler, publisher, err := newHandler(ctx, cfg)
	if err != nil {
		// newHandler はエラー時も publisher を返しうる(NATS への接続自体は先に試みるため)。
		// defer の登録前にここで return するので、確実に閉じておく(critic レビューでの指摘)。
		publisher.Shutdown()
		return err
	}
	// publisher.Shutdown は発行 goroutine と JetStream の未確定分を待ってから閉じる(ADR-0212 §6)。
	// defer なのでこの関数の return 直前(= srv.Shutdown が完了した後)に呼ばれる。
	defer publisher.Shutdown()

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
