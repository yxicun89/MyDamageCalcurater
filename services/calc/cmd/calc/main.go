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
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// 環境変数の名前。
const (
	envAddr          = "CALC_ADDR"
	envMasterPath    = "CALC_MASTER_PATH"
	envTypeChartPath = "CALC_TYPECHART_PATH"

	// defaultAddr は CALC_ADDR が未設定・空のときの待ち受けアドレス。
	defaultAddr = ":8080"
)

// errNotImplemented は P3-1 の implementer が置き換えるまでのスタブ用。
var errNotImplemented = errors.New("未実装(P3-1)")

// config は calc-svc の設定(環境変数から1度だけ読む)。
type config struct {
	Addr          string
	MasterPath    string
	TypeChartPath string
}

// loadConfig は環境変数から設定を読む。必須の CALC_MASTER_PATH / CALC_TYPECHART_PATH が
// 未設定・空ならエラー。CALC_ADDR が未設定・空なら defaultAddr。
func loadConfig(lookup func(string) (string, bool)) (config, error) {
	return config{}, errNotImplemented
}

// newHandler は設定のファイルからマスタと相性表を読み込み、HTTP ハンドラを作る。
// ファイルが無い・壊れている・スキーマ違反ならエラー(部分的なデータで起動しない)。
func newHandler(cfg config) (http.Handler, error) {
	return nil, errNotImplemented
}

// run は設定を読み、マスタを読み込み、ctx が終わるまで待ち受ける。ctx が終わったら
// サーバを止めて nil を返す。起動前の失敗(設定・マスタ)はエラーで返す。
func run(ctx context.Context, lookup func(string) (string, bool)) error {
	return errNotImplemented
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.LookupEnv); err != nil {
		slog.Error("calc-svc を起動できない", "error", err)
		os.Exit(1)
	}
}
