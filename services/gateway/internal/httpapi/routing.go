package httpapi

// ルーティング(ADR-0202 §3)。パスの前方一致だけで振り分ける(gateway は上流の中の操作を知らない)。

import (
	"net/http"
	"strings"
)

// routeKind は一致したルートの種類。
type routeKind int

const (
	routeNone routeKind = iota
	routeHealthz
	routeCalc
	routePokedex
	routeAssets
	routeWeb
)

// パスの前方一致に使う定数。
const (
	pathHealthz   = "/healthz"
	prefixCalc    = "/api/calc"
	prefixPokedex = "/api/pokedex/"
	prefixAssets  = "/assets/"
)

// reservedFirstSegments は先頭セグメントがこれと完全一致するパスを Web に流さない予約語(ADR-0205)。
// /apix・/internals のように予約語で始まるだけの別名は含まない(セグメント単位の判定)。
var reservedFirstSegments = map[string]bool{"api": true, "assets": true, "healthz": true, "internal": true}

// firstPathSegment はパスの先頭セグメントを返す("/api/calc" なら "api"。"/" や "" なら ""）。
func firstPathSegment(path string) string {
	path = strings.TrimPrefix(path, "/")
	if i := strings.Index(path, "/"); i >= 0 {
		return path[:i]
	}
	return path
}

// isReservedPath は先頭セグメントが予約語(api・assets・healthz・internal)と完全一致するかを返す
// (ADR-0205: matchRoute が拾えなかった予約パス — /api・/api/unknown・/internal 等 — は Web に流さず
// 404 のままにする)。
func isReservedPath(path string) bool {
	return reservedFirstSegments[firstPathSegment(path)]
}

// isWebEligibleMethod は Web への転送を許すメソッド(GET / HEAD のみ。ADR-0205: 静的配信に書き込みは要らない)。
func isWebEligibleMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// matchRoute はメソッドとパスからルートを決める。一致しない(未知のパス・許さないメソッド)場合は
// (routeNone, false)。ヘッダ検証・上流の有無はここでは見ない(判定順序は ADR-0202 §3)。
func matchRoute(method, path string) (routeKind, bool) {
	switch {
	case method == http.MethodGet && path == pathHealthz:
		return routeHealthz, true
	case path == prefixCalc || strings.HasPrefix(path, prefixCalc+"/"):
		// /api/calc・/api/calc/* だけを拾う(/api/calcx のような前方一致は拾わない)。
		return routeCalc, true
	case strings.HasPrefix(path, prefixPokedex):
		// /api/pokedex そのもの(末尾スラッシュ無し)はここに一致しない → 404。
		return routePokedex, true
	case strings.HasPrefix(path, prefixAssets):
		if method != http.MethodGet && method != http.MethodHead {
			return routeNone, false
		}
		return routeAssets, true
	default:
		return routeNone, false
	}
}

// hasDotSegment はパスのセグメントに "." または ".." があるかを返す(ADR-0202 §3: ドットセグメントは
// 404 にし、どの上流にも送らない。上流の運用エンドポイントや他のルートへ抜けさせない)。
func hasDotSegment(path string) bool {
	for _, seg := range strings.Split(path, "/") {
		if seg == "." || seg == ".." {
			return true
		}
	}
	return false
}

// requiresHeaderCheck は /api/* のルート(calc・pokedex)にだけ X-Device-Id / X-Session-Id の
// 検証を課す(ADR-0202 §4。/assets・/healthz・CORS プリフライトは課さない)。
func requiresHeaderCheck(kind routeKind) bool {
	return kind == routeCalc || kind == routePokedex
}
