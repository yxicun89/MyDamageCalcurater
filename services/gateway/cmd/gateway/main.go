// Command gateway はクライアントの唯一の入口(ADR-0020)。/api/calc・/api/pokedex・/assets を各上流へ転送する。
//
// 設定は環境変数で渡す(1か所で読み込み、必須値は起動時に検証する。docs/coding-rules.md §2):
//
//	GATEWAY_ADDR                  待ち受けアドレス(既定 ":8080")
//	GATEWAY_CALC_URL              calc-svc の基底 URL。必須
//	GATEWAY_POKEDEX_URL           pokedex-svc の基底 URL。任意(未設定なら /api/pokedex/* は 503)
//	GATEWAY_ASSETS_URL            画像配信の基底 URL。任意(未設定なら /assets/* は 404)
//	GATEWAY_CORS_ALLOWED_ORIGINS  カンマ区切りの許可オリジン(完全一致)。任意。"*" は起動エラー
//	GATEWAY_UPSTREAM_TIMEOUT      上流の応答ヘッダを待つ上限(Go の duration)。既定 10s
//
// P3-2 の spec-writer が置いたスタブ。実装は implementer が行う(テストは main_test.go)。
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"example.com/pokecalc/services/gateway/internal/httpapi"
)

// 環境変数の名前(運用の manifest・README が依存する)。
const (
	envAddr               = "GATEWAY_ADDR"
	envCalcURL            = "GATEWAY_CALC_URL"
	envPokedexURL         = "GATEWAY_POKEDEX_URL"
	envAssetsURL          = "GATEWAY_ASSETS_URL"
	envCORSAllowedOrigins = "GATEWAY_CORS_ALLOWED_ORIGINS"
	envUpstreamTimeout    = "GATEWAY_UPSTREAM_TIMEOUT"
)

// errInvalidConfig は設定の読み込みに失敗したとき loadConfig が包んで返すエラー。
var errInvalidConfig = errors.New("gateway の設定が不正")

// errNotImplemented はスタブが返すエラー(implementer が実装したら消す)。
var errNotImplemented = errors.New("gateway: 未実装(P3-2)")

// config は gateway の設定(環境変数から1度だけ読む)。
type config struct {
	Addr    string
	Gateway httpapi.Config
}

// loadConfig は環境変数から設定を読む。不正なら errInvalidConfig を包んで返す。
func loadConfig(lookup func(string) (string, bool)) (config, error) {
	_ = lookup
	return config{}, errNotImplemented
}

// run は設定を読み、ctx が終わるまで待ち受ける。ctx が終わったらサーバを止めて nil を返す。
func run(ctx context.Context, lookup func(string) (string, bool)) error {
	_, _ = ctx, lookup
	return errNotImplemented
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.LookupEnv); err != nil {
		slog.Error("gateway を起動できない", "error", err)
		os.Exit(1)
	}
}
