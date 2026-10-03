package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/apps/wishlist/api/internal/api"
)

// MaxBodyBytes は 1 リクエストの本文の上限(画像 10 MiB + multipart のオーバーヘッドに余裕を持たせる)。
const MaxBodyBytes = 12 << 20

// securityHeaders は全応答に nosniff を付ける(画像を HTML として解釈させない)。
func securityHeaders(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		return next(c)
	}
}

// limitBody は本文の読み込み量に上限を掛ける。超えたら読み込みが *http.MaxBytesError で失敗する。
func limitBody(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		r := c.Request()
		r.Body = http.MaxBytesReader(c.Response(), r.Body, MaxBodyBytes)
		return next(c)
	}
}

// bearerAuth は /api 配下に Bearer トークンを要求する(/healthz・/images は対象外)。比較は定数時間。
func bearerAuth(token string) echo.MiddlewareFunc {
	want := sha256.Sum256([]byte(token))
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			p := c.Request().URL.Path
			if p != "/api" && !strings.HasPrefix(p, "/api/") {
				return next(c)
			}
			got, ok := bearerToken(c.Request().Header.Get("Authorization"))
			sum := sha256.Sum256([]byte(got))
			// ハッシュ同士を比べて長さの違いからも漏らさない。ok も同じ式で見る
			if subtle.ConstantTimeCompare(sum[:], want[:])&boolToInt(ok) != 1 {
				c.Response().Header().Set("WWW-Authenticate", "Bearer")
				return &apiError{status: http.StatusUnauthorized, code: api.ErrorCodeUnauthorized, msg: "missing or invalid bearer token"}
			}
			return next(c)
		}
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// bearerToken は "Bearer <token>" から token を取り出す(スキーム名は大文字小文字を区別しない)。
func bearerToken(h string) (string, bool) {
	const prefix = "bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	return h[len(prefix):], true
}

// recoverPanic はハンドラーの panic を 500 にする(スタックはログにだけ出す)。
func (s *server) recoverPanic(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) (err error) {
		defer func() {
			if r := recover(); r != nil {
				if r == http.ErrAbortHandler {
					panic(r)
				}
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		return next(c)
	}
}
