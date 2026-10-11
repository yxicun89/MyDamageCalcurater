package httpapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
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
