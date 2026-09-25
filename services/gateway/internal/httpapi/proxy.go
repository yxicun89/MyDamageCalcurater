package httpapi

// 上流への転送(ADR-0202 §1・§5)。標準ライブラリの net/http/httputil.ReverseProxy を使う
// (新しい依存を足さない)。タイムアウトは応答ヘッダを待つ上限(ResponseHeaderTimeout)と
// dial の上限だけに掛け、本文の転送は打ち切らない。
//
// Rewrite(Director は非推奨。ADR-0202 §5)で Host を上流のホストに書き換え、
// X-Forwarded-For / X-Forwarded-Host / X-Forwarded-Proto をクライアントの値で信用せず
// gateway が実際のクライアント IP・元の Host・スキームで付け直す(ProxyRequest.SetXForwarded は
// 呼び出し前に Rewrite がこれらのヘッダを削除済みの outbound リクエストに対して働く)。
// クライアントが送った X-Real-Ip / Forwarded も素通ししない(issue #326。ADR-0202 §4 追記)。

import (
	"context"
	"encoding/json"
	"errors"
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

// newReverseProxy の restoreVerifiedIDs に渡す値(呼び出し側で意図が読めるように名前を付ける)。
const (
	restoreIDs  = true  // /api/* の上流: 検証済みの X-Device-Id / X-Session-Id を付け直す
	keepIDsAsIs = false // 検証しない上流(assets・Web): 付け直さない
)

// newReverseProxy は target への ReverseProxy を作る。
//
//   - target: 転送先の基底 URL。
//   - timeout: 上流の応答ヘッダを待つ上限(dial にも同じ値を使う)。override が nil のときの
//     既定 Transport にだけ効く。
//   - override: 上流への RoundTripper の差し替え(テストだけが使う。nil なら既定の Transport)。
//   - restoreVerifiedIDs: true なら、gateway が検証した X-Device-Id / X-Session-Id を上流へ付け直す
//     (/api/* の上流だけ。クライアントが Connection に列挙すると ReverseProxy が hop-by-hop として
//     消してしまうため。issue #326)。
//   - originAllowed: CORS の許可オリジン判定。ModifyResponse(上流の応答が届いたとき)と
//     ErrorHandler(上流に接続できない・タイムアウトしたとき)の両方で、Access-Control-* を
//     いったん取り除いてから、許可オリジンのときだけ gateway 自身の ACAO を付け直すために使う
//     (必須1・必須2)。
//
// 接続できない・タイムアウトしたときは ErrorHandler が 503 upstream_unavailable を返す。
func newReverseProxy(target *url.URL, timeout time.Duration, override http.RoundTripper, restoreVerifiedIDs bool, originAllowed func(string) bool) *httputil.ReverseProxy {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)  // Host も target のホストに書き換わる(推奨3)。
			pr.SetXForwarded() // X-Forwarded-* はクライアントの値を信用せず付け直す(推奨3)。
			// gateway→上流では 100-continue を使わない(issue #209。ADR-0202 §5 追記)。上流が 100 Continue を
			// 返すと ReverseProxy がそれを WriteHeader(100) で転送し、Echo の Response が 100 で commit されて
			// 続く上流のステータス(4xx/5xx)を捨て、既定の 200 が出てしまう。クライアント側の Expect には
			// gateway の http.Server が本文を読むときに自動で 100 Continue を返すので、クライアントの挙動は変わらない。
			pr.Out.Header.Del("Expect")
			// クライアント IP を名乗るヘッダはクライアントの値を信用しない(issue #326)。X-Forwarded-For は
			// SetXForwarded が付け直す。Forwarded は ReverseProxy が Rewrite の前に消すが、意図を明示する。
			pr.Out.Header.Del("X-Real-Ip")
			pr.Out.Header.Del("Forwarded")
			if restoreVerifiedIDs {
				// ReverseProxy は Rewrite の前に Connection に列挙されたヘッダを消す。serve が検証した
				// 受信側の値(ちょうど1つ)を付け直し、検証済みの ID が必ず上流に届くようにする。
				for _, name := range verifiedIDHeaders {
					if v := pr.In.Header.Get(name); v != "" {
						pr.Out.Header.Set(name, v)
					}
				}
			}
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
		// クライアントが要求を中断した(ブラウザの AbortSignal・iOS の Task cancel 等。issue #113)。
		// r.Context() はクライアントの接続が切れると Go の http.Server が自動でキャンセルするため、
		// これは上流の障害ではなくクライアント起因。誤って「上流に到達できない」と WARN ログに
		// 出すと運用上のノイズになる。ADR-0202 §5 の「クライアントへは固定文だけ」は上流障害についての
		// 規定で、クライアント起因の中断はそもそも応答を返す相手が居ないので何もしない(ADR-0202 §5 追記)。
		if errors.Is(err, context.Canceled) {
			slog.Debug("gateway: クライアントが要求を中断した", "upstream", target.Host)
			return
		}
		// クライアントへは固定文だけ(Go の内部情報を出さない)。詳細はログにだけ残す(推奨4)。
		slog.Warn("gateway: 上流に到達できない", "upstream", target.Host, "error", err)
		// 上流に届かなかった応答にも CORS を付ける(必須2: 接続不可・タイムアウトの 503 で
		// ACAO が抜け落ちる退行の修正)。r は Rewrite 後の outbound リクエストだが、
		// Origin ヘッダは Rewrite で変更していないのでそのまま読める。
		stripCORSHeaders(w.Header())
		if origin := r.Header.Get("Origin"); originAllowed(origin) {
			setCORSAllowed(w.Header(), origin)
		}
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
