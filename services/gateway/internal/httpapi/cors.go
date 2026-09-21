package httpapi

// CORS(ADR-0020 §6)。許可オリジンに完全一致する Origin にだけ ACAO と Vary: Origin を付ける。
// 認証なしなので Access-Control-Allow-Credentials は付けない。`*` は Config で拒否済み(NewHandler)。

import "net/http"

// corsAllowMethods / corsAllowHeaders / corsMaxAge はプリフライトの応答ヘッダの固定値(ADR-0020 §6)。
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

// setCORSAllowed は許可オリジンへの応答に ACAO=そのオリジン と Vary: Origin を付ける。
func setCORSAllowed(h http.Header, origin string) {
	h.Set("Access-Control-Allow-Origin", origin)
	h.Add("Vary", "Origin")
}

// setCORSPreflightHeaders はプリフライトの応答に Allow-Methods / Allow-Headers / Max-Age を付ける。
func setCORSPreflightHeaders(h http.Header) {
	h.Set("Access-Control-Allow-Methods", corsAllowMethods)
	h.Set("Access-Control-Allow-Headers", corsAllowHeaders)
	h.Set("Access-Control-Max-Age", corsMaxAge)
}
