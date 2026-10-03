// Package reqlog は gateway と calc が共有するログの形(issue #246)。ログは JSON 1 形式、
// 1 リクエスト 1 行のアクセスログ、X-Request-Id の生成と伝播をここに集める。
// 認証・分散トレース(OpenTelemetry は ADR-0406 が却下済み)は持ち込まない。
package reqlog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
)

// Header はリクエスト ID を運ぶヘッダ(契約外の運用ヘッダ)。
const Header = "X-Request-Id"

// maxIDLen はクライアントが送る ID の最大長。
const maxIDLen = 64

type ctxKey struct{}

// ValidID は s が通してよい ID(1〜64 文字の可視 ASCII)かを返す。ログ・ヘッダへの注入を防ぐ。
func ValidID(s string) bool {
	if s == "" || len(s) > maxIDLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// NewID は新しいリクエスト ID(ランダム 128 bit の 16 進)を返す。
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// WithID は ctx にリクエスト ID を入れる。
func WithID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// IDFromContext は ctx のリクエスト ID を返す(無ければ "")。
func IDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// ctxHandler は ctx にリクエスト ID があるとき request_id 属性を足す slog.Handler。
type ctxHandler struct{ slog.Handler }

func (h ctxHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := IDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h ctxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ctxHandler{h.Handler.WithAttrs(attrs)}
}

func (h ctxHandler) WithGroup(name string) slog.Handler {
	return ctxHandler{h.Handler.WithGroup(name)}
}

// NewLogger は w へ JSON Lines を書くロガーを返す(request_id は ctx から付く)。
func NewLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(ctxHandler{slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})})
}

// Install は NewLogger(w, Info) を slog の既定ロガーにして返す(main の先頭で 1 度呼ぶ)。
func Install(w io.Writer) *slog.Logger {
	l := NewLogger(w, slog.LevelInfo)
	slog.SetDefault(l)
	return l
}

// Middleware はリクエスト ID を確定し(有効なクライアントの値を通し、無い・不正なら生成)、
// 要求ヘッダ(上流へ転送される)・応答ヘッダ・context に入れ、終了後にアクセスログを 1 行出す。
// 最も外側に登録する(エラーハンドラ後の最終ステータスを読むため)。probe・scrape(/healthz・/readyz・/metrics)は Debug。
func Middleware(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			r := c.Request()
			id := r.Header.Get(Header)
			if !ValidID(id) {
				id = NewID()
			}
			r.Header.Set(Header, id)
			c.Response().Header().Set(Header, id)
			ctx := WithID(r.Context(), id)
			c.SetRequest(r.WithContext(ctx))

			start := time.Now()
			err := next(c)
			if err != nil {
				c.Echo().HTTPErrorHandler(c, err)
				err = nil
			}

			status := http.StatusOK
			if resp, uerr := echo.UnwrapResponse(c.Response()); uerr == nil {
				status = resp.Status
			}
			level := slog.LevelInfo
			if p := r.URL.Path; p == "/healthz" || p == "/readyz" || p == "/metrics" {
				level = slog.LevelDebug // 定期的な probe・scrape でログを埋めない
			}
			logger.Log(ctx, level, "access",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration_ms", float64(time.Since(start).Microseconds())/1000,
				"remote_addr", r.RemoteAddr,
			)
			return err
		}
	}
}
