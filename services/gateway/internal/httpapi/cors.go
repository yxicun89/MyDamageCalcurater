package httpapi

// CORS(ADR-0202 §6)。許可オリジンに完全一致する Origin にだけ ACAO と Vary: Origin を付ける。
// 認証なしなので Access-Control-Allow-Credentials は付けない。`*` は Config で拒否済み(NewHandler)。
//
// 上流を経由する応答は、上流が付けた Access-Control-* を先にすべて取り除いてから
// (stripCORSHeaders)、許可オリジンのときだけ gateway 自身の ACAO を付け直す(必須1。proxy.go の
// ModifyResponse から呼ぶ)。gateway 自身が作る応答(エラー・healthz・プリフライト)には
// 上流の応答が無いので、直接 setCORSAllowed を呼ぶだけでよい。

import (
	"net/http"
	"strings"
)

// corsAllowMethods / corsAllowHeaders / corsMaxAge はプリフライトの応答ヘッダの固定値(ADR-0202 §6)。
const (
	corsAllowMethods = "GET, POST, OPTIONS"
	corsAllowHeaders = "Content-Type, X-Device-Id, X-Session-Id"
	corsMaxAge       = "600"
)

// originAllowed は Origin が許可オリジンのどれかに完全一致するかを返す(空文字は常に false)。
func (g *gateway) originAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	for _, allowed := range g.cfg.CORSAllowedOrigins {
		if allowed == origin {
			return true
		}
	}
	return false
}

// isPreflight は CORS のプリフライト(OPTIONS + Access-Control-Request-Method)かどうかを返す。
func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}

// setCORSAllowed は許可オリジンへの応答に ACAO=そのオリジン(ちょうど1つ)と Vary: Origin を付ける。
// h.Set を使うので既存の Access-Control-Allow-Origin があっても上書きして1つにする
// (ModifyResponse は先に stripCORSHeaders で上流の値を消してから呼ぶ。必須1)。
func setCORSAllowed(h http.Header, origin string) {
	h.Set("Access-Control-Allow-Origin", origin)
	addVaryOrigin(h)
}

// addVaryOrigin は Vary に "Origin" を重複なく足す(上流がすでに Vary: Origin を返していても
// 二重に足さない。必須1)。
func addVaryOrigin(h http.Header) {
	for _, v := range h.Values("Vary") {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), "Origin") {
				return
			}
		}
	}
	h.Add("Vary", "Origin")
}

// setCORSPreflightHeaders はプリフライトの応答に Allow-Methods / Allow-Headers / Max-Age を付ける。
func setCORSPreflightHeaders(h http.Header) {
	h.Set("Access-Control-Allow-Methods", corsAllowMethods)
	h.Set("Access-Control-Allow-Headers", corsAllowHeaders)
	h.Set("Access-Control-Max-Age", corsMaxAge)
}

// stripCORSHeaders は上流が付けた Access-Control-* をすべて取り除く(必須1。gateway が
// 許可オリジンを判定し直すので、上流の判断([*] や別オリジンの反射を含む)を外へ出さない)。
func stripCORSHeaders(h http.Header) {
	var names []string
	for name := range h {
		if strings.HasPrefix(name, "Access-Control-") {
			names = append(names, name)
		}
	}
	for _, name := range names {
		h.Del(name)
	}
}
