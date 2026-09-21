// Package httpapi は gateway の HTTP 境界(ADR-0202)。クライアントの唯一の入口として、
// /api/calc・/api/pokedex・/assets を各上流へ転送し、/api/* の X-Device-Id / X-Session-Id を検証し、
// CORS に答える。gateway 自身は計算もマスタ参照もしない(サービスは自分のデータだけに触る。CLAUDE.md 絶対ルール4)。
//
// 1リクエストの判定順序(ADR-0202 §3): CORS(プリフライトはここで204)→ ドットセグメント拒否 →
// ルーティング(未知のパスはヘッダが無くても404)→ /api/* のヘッダ検証 → 上流の有無 → 転送。
package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
)

// ErrInvalidConfig は Config が不正(CalcURL が無い等)なときに NewHandler が包んで返すエラー。
var ErrInvalidConfig = errors.New("gateway の設定が不正")

// msgNotFound は gateway が扱わないパス・メソッドに返す固定文(ADR-0202 §3・§7)。
const msgNotFound = "ルートが無い"

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

// gateway は組み立て済みの Config と3つの上流への ReverseProxy を持つ。
type gateway struct {
	cfg          Config
	calcProxy    *httputil.ReverseProxy
	pokedexProxy *httputil.ReverseProxy // nil なら /api/pokedex/* は 503(PokedexURL 未設定)
	assetsProxy  *httputil.ReverseProxy // nil なら /assets/* は 404(AssetsURL 未設定)
}

// NewHandler は gateway の HTTP ハンドラ全体を組み立てる。Config が不正なら ErrInvalidConfig を包んで返す。
func NewHandler(cfg Config) (http.Handler, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	g := &gateway{cfg: cfg}
	g.calcProxy = newReverseProxy(cfg.CalcURL, cfg.UpstreamTimeout, cfg.transport, g.originAllowed)
	if cfg.PokedexURL != nil {
		g.pokedexProxy = newReverseProxy(cfg.PokedexURL, cfg.UpstreamTimeout, cfg.transport, g.originAllowed)
	}
	if cfg.AssetsURL != nil {
		g.assetsProxy = newReverseProxy(cfg.AssetsURL, cfg.UpstreamTimeout, cfg.transport, g.originAllowed)
	}

	e := echo.New()
	e.HTTPErrorHandler = httpErrorHandler
	e.Use(recoverMiddleware)
	e.Any("/*", g.serve)
	return e, nil
}

// validateConfig は NewHandler が受け付けられない Config を ErrInvalidConfig で拒否する
// (ADR-0202 §1: CalcURL が無い・UpstreamTimeout が 0 以下・許可オリジンに "*")。
func validateConfig(cfg Config) error {
	if cfg.CalcURL == nil {
		return fmt.Errorf("%w: CalcURL が無い", ErrInvalidConfig)
	}
	if cfg.UpstreamTimeout <= 0 {
		return fmt.Errorf("%w: UpstreamTimeout は正でなければならない(%v)", ErrInvalidConfig, cfg.UpstreamTimeout)
	}
	for _, origin := range cfg.CORSAllowedOrigins {
		if origin == "*" {
			return fmt.Errorf("%w: 許可オリジンに \"*\" は使えない", ErrInvalidConfig)
		}
	}
	return nil
}

// serve は1リクエストの判定順序(パッケージのコメント参照)をすべて実装する唯一の入口。
//
// CORS: gateway 自身が作る応答(プリフライト・エラー・healthz)は、ここで許可オリジンなら
// setCORSAllowed を呼んで直接付ける。上流を経由する応答(calc・pokedex・assets の転送)には
// ここでは付けない — 上流が独自の Access-Control-* を返すことがあり、ここで付けてしまうと
// 上流の値と重複・混在する(必須1)。上流経由の応答の CORS は newReverseProxy の
// ModifyResponse が一元的に扱う(上流の値を全部消してから、許可オリジンのときだけ付け直す)。
func (g *gateway) serve(c *echo.Context) error {
	r := c.Request()
	origin := r.Header.Get("Origin")
	allowed := g.originAllowed(origin)

	// 1. CORS プリフライトはここで 204(上流には送らない)。
	if isPreflight(r) {
		if allowed {
			h := c.Response().Header()
			setCORSAllowed(h, origin)
			setCORSPreflightHeaders(h)
		}
		return c.NoContent(http.StatusNoContent)
	}

	// 2. ドットセグメントは拒否(どの上流にも送らない)。gateway 自身の応答。
	path := r.URL.Path
	if hasDotSegment(path) {
		return g.ownError(c, origin, allowed, newError(api.NotFound, "%s", msgNotFound))
	}

	// 3. ルーティング(未知のパス・許さないメソッドはヘッダが無くても 404)。gateway 自身の応答。
	kind, matched := matchRoute(r.Method, path)
	if !matched {
		return g.ownError(c, origin, allowed, newError(api.NotFound, "%s", msgNotFound))
	}
	if kind == routeHealthz {
		if allowed {
			setCORSAllowed(c.Response().Header(), origin)
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}

	// 4. /api/* だけヘッダを検証する。失敗は gateway 自身の応答。
	if requiresHeaderCheck(kind) {
		if err := checkAPIHeaders(r.Header); err != nil {
			return g.ownError(c, origin, allowed, err)
		}
	}

	// 5. 上流の有無(未設定は gateway 自身の応答)→ 6. 転送(CORS は ModifyResponse が付ける)。
	switch kind {
	case routeCalc:
		g.calcProxy.ServeHTTP(c.Response(), r)
	case routePokedex:
		if g.pokedexProxy == nil {
			return g.ownError(c, origin, allowed, newError(api.UpstreamUnavailable, "%s", msgUpstreamUnavailable))
		}
		g.pokedexProxy.ServeHTTP(c.Response(), r)
	case routeAssets:
		if g.assetsProxy == nil {
			return g.ownError(c, origin, allowed, newError(api.NotFound, "%s", msgNotFound))
		}
		g.assetsProxy.ServeHTTP(c.Response(), r)
	}
	return nil
}

// ownError は gateway 自身が作るエラー応答(上流を経由しない)に、許可オリジンなら CORS を
// 付けてからそのまま返す(httpErrorHandler が Error 本文として書き出す)。
func (g *gateway) ownError(c *echo.Context, origin string, allowed bool, err error) error {
	if allowed {
		setCORSAllowed(c.Response().Header(), origin)
	}
	return err
}
