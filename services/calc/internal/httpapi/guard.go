package httpapi

// 過負荷・締め切りの守り(issue #299・ADR-0801)。calc の3操作だけに掛け、運用エンドポイント
// (/healthz・/readyz・/metrics)は対象外にする(probe が 503 で巻き込まれて Pod が外れないように)。

import (
	"context"
	"time"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/httpguard"
)

const (
	// DefaultRequestTimeout は calc の3操作の締め切り。cmd/calc の writeTimeout(10 秒)より1秒短い。
	DefaultRequestTimeout = 9 * time.Second
	// DefaultMaxInflight は同時に処理する calc 操作の数。逆算の最大入力は 1 回 約 85ms(0.2 CPU)で、
	// 16 並列でも最後の1件が 約 1.4 秒で終わる。超えた分は 503 + Retry-After(待たせない)。
	DefaultMaxInflight = 16
)

// Option は NewHandler・NewDeferredHandler の設定(主にテストで小さい上限・短い締め切りを渡す)。本番は渡さない。
type Option func(*httpguard.Config)

// WithGuard は同時実行の上限と締め切りを差し替える。
// code は契約の宣言(guardCode)のまま変えない。
func WithGuard(cfg httpguard.Config) Option {
	return func(c *httpguard.Config) {
		cfg.Code = c.Code
		*c = cfg
	}
}

// guardMiddleware は opts を適用した守りを作る。1つの Handler につき1回だけ作り、3操作で共有する
// (同時実行の枠は操作をまたいで数える)。
func guardMiddleware(opts []Option) echo.MiddlewareFunc {
	cfg := httpguard.Config{MaxInflight: DefaultMaxInflight, Timeout: DefaultRequestTimeout, Code: guardCode}
	for _, o := range opts {
		o(&cfg)
	}
	return httpguard.Middleware(cfg)
}

// checkDeadline は engine を呼ぶ前に締め切りを確かめる。過ぎていれば計算を始めず 503 にする。
func checkDeadline(ctx context.Context) error {
	if httpguard.Expired(ctx) {
		return newError(api.UpstreamUnavailable, "処理の締め切りを過ぎた。少し待ってから再試行してほしい")
	}
	return nil
}

// guardCode は過負荷・締め切りの 503 の code。契約(api/openapi.yaml)に新しい code を足さず、
// 宣言済みの upstream_unavailable を使う(ADR-0801 §2)。
const guardCode = string(api.UpstreamUnavailable)
