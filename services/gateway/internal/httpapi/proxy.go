package httpapi

// 上流への転送(ADR-0202 §1・§5)。標準ライブラリの net/http/httputil.ReverseProxy を使う
// (新しい依存を足さない)。タイムアウトは応答ヘッダを待つ上限(ResponseHeaderTimeout)と
// dial の上限だけに掛け、本文の転送は打ち切らない。
//
// Rewrite(Director は非推奨。ADR-0202 §5)で Host を上流のホストに書き換え、
// X-Forwarded-For / X-Forwarded-Host / X-Forwarded-Proto をクライアントの値で信用せず
// gateway が実際のクライアント IP・元の Host・スキームで付け直す(ProxyRequest.SetXForwarded は
// 呼び出し前に Rewrite がこれらのヘッダを削除済みの outbound リクエストに対して働く)。

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"example.com/pokecalc/services/internal/api"
)

// msgUpstreamUnavailable は上流に接続できない・タイムアウトしたときの固定文。
// dial・アドレス・context のエラー文などの Go の内部情報は出さない(ADR-0202 §5)。
const msgUpstreamUnavailable = "上流を利用できない"

// newReverseProxy は target への ReverseProxy を作る。override が非 nil ならそれを Transport に使う
// (テストだけが使う。ADR-0202 §1)。既定は DialContext と ResponseHeaderTimeout に timeout を掛けた
// Transport。接続できない・タイムアウトしたときは ErrorHandler が 503 upstream_unavailable を返す。
// originAllowed は CORS の許可オリジン判定(ModifyResponse が上流の応答から Access-Control-* を
// 取り除いたあと、許可オリジンのときだけ gateway 自身の ACAO を付け直すために使う。必須1)。
func newReverseProxy(target *url.URL, timeout time.Duration, override http.RoundTripper, originAllowed func(string) bool) *httputil.ReverseProxy {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)  // Host も target のホストに書き換わる(推奨3)。
			pr.SetXForwarded() // X-Forwarded-* はクライアントの値を信用せず付け直す(推奨3)。
		},
	}
	if override != nil {
		proxy.Transport = override
	} else {
		proxy.Transport = defaultUpstreamTransport(timeout)
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		// 上流が付けた Access-Control-* は外へ出さない。許可オリジンのときだけ gateway が
		// 付け直す(ACAO はちょうど1つ。必須1)。
		stripCORSHeaders(resp.Header)
		if origin := resp.Request.Header.Get("Origin"); originAllowed(origin) {
			setCORSAllowed(resp.Header, origin)
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		// クライアントへは固定文だけ(Go の内部情報を出さない)。詳細はログにだけ残す(推奨4)。
		slog.Warn("gateway: 上流に到達できない", "upstream", target.Host, "error", err)
		writeJSONError(w, http.StatusServiceUnavailable, api.UpstreamUnavailable, msgUpstreamUnavailable)
	}
	return proxy
}

// defaultUpstreamTransport は上流ごとの既定の RoundTripper。dial と応答ヘッダ待ちの両方に
// timeout を掛ける(本文の転送そのものには掛けない)。
func defaultUpstreamTransport(timeout time.Duration) http.RoundTripper {
	dialer := &net.Dialer{Timeout: timeout}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ResponseHeaderTimeout: timeout,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
	}
}

// writeJSONError は Error 本文(ADR-0202 §7)を直接書く。httputil.ReverseProxy.ErrorHandler は
// echo を経由しないので、echo.Context を使わずに書く(errors.go の httpErrorHandler と同じ形)。
func writeJSONError(w http.ResponseWriter, status int, code api.ErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.Error{Code: code, Message: message})
}
