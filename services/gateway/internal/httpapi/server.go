// Package httpapi は gateway の HTTP 境界(ADR-0020)。クライアントの唯一の入口として、
// /api/calc・/api/pokedex・/assets を各上流へ転送し、/api/* の X-Device-Id / X-Session-Id を検証し、
// CORS に答える。gateway 自身は計算もマスタ参照もしない(サービスは自分のデータだけに触る。CLAUDE.md 絶対ルール4)。
//
// P3-2 の spec-writer が置いたスタブ。実装は implementer が行う(テストは handler_test.go ほか)。
package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"time"
)

// ErrInvalidConfig は Config が不正(CalcURL が無い等)なときに NewHandler が包んで返すエラー。
var ErrInvalidConfig = errors.New("gateway の設定が不正")

// errNotImplemented はスタブが返すエラー(implementer が実装したら消す)。
var errNotImplemented = errors.New("gateway: 未実装(P3-2)")

// Config は gateway のルーティング・検証・CORS の設定(cmd/gateway の loadConfig が環境変数から作る)。
type Config struct {
	// CalcURL は calc-svc の基底 URL(必須)。/api/calc と /api/calc/* を転送する。
	CalcURL *url.URL
	// PokedexURL は pokedex-svc の基底 URL。nil なら /api/pokedex/* は 503 upstream_unavailable。
	PokedexURL *url.URL
	// AssetsURL は画像配信(MinIO)の基底 URL。nil なら /assets/* は 404 not_found。
	AssetsURL *url.URL
	// CORSAllowedOrigins は Origin と完全一致で照合する許可オリジン。空なら CORS ヘッダを付けない。
	CORSAllowedOrigins []string
	// UpstreamTimeout は上流の応答ヘッダを待つ上限。超えたら 503 upstream_unavailable。0 以下は不正。
	UpstreamTimeout time.Duration

	// transport は上流への RoundTripper の差し替え口(テストだけが使う。nil なら既定)。
	transport http.RoundTripper
}

// NewHandler は gateway の HTTP ハンドラ全体を組み立てる。Config が不正なら ErrInvalidConfig を包んで返す。
func NewHandler(cfg Config) (http.Handler, error) {
	_ = cfg
	return nil, errNotImplemented
}
