// Package httpguard provides the overload guard shared by the Echo v5 services in this module
// (calc・pokedex): a whole-handler deadline and a concurrency limit that answers 503 with
// Retry-After instead of queueing work it cannot finish in time (ADR-0801, issue #299).
//
// balance・speed・judge (each its own Go module) carry an identical copy of this file
// (ADR-0406 §2 precedent: no shared module dependency across service lanes; a byte-equality test
// in each module catches drift). Any change here must be mirrored there.
package httpguard

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
)

// deadlineMargin is how much shorter than http.Server.WriteTimeout the handler deadline is, so
// the 503 body can still be written before WriteTimeout cuts the connection.
const deadlineMargin = time.Second

// Config configures Middleware. A zero field disables that guard.
type Config struct {
	// MaxInflight is the number of requests handled at once; the next one is answered 503
	// immediately (never queued).
	MaxInflight int
	// Timeout is the deadline put on the request context for the whole handler.
	Timeout time.Duration
	// Code is the Error.code of the 503 body. Empty means "overloaded"; a service whose
	// contract has no such code passes one it declares (e.g. "upstream_unavailable").
	Code string
}

// DefaultCode is the Error.code used when Config.Code is empty.
const DefaultCode = "overloaded"

// DeadlineFor returns the handler deadline for a server's WriteTimeout (WriteTimeout - 1s).
func DeadlineFor(writeTimeout time.Duration) time.Duration { return writeTimeout - deadlineMargin }

// Validate rejects a Timeout that is negative or not strictly below writeTimeout (WriteTimeout
// only fails the write; it never stops the handler, so the deadline must come first).
func (c Config) Validate(writeTimeout time.Duration) error {
	if c.Timeout < 0 {
		return fmt.Errorf("httpguard: timeout %v must not be negative", c.Timeout)
	}
	if c.Timeout > 0 && c.Timeout >= writeTimeout {
		return fmt.Errorf("httpguard: timeout %v must be less than the write timeout %v", c.Timeout, writeTimeout)
	}
	return nil
}

// Expired reports whether ctx is already done. Handlers call it before starting new work
// (an engine call) so a request that cannot be answered in time does not keep computing.
func Expired(ctx context.Context) bool { return ctx.Err() != nil }

// Middleware applies cfg. Register it inside the metrics middleware (so the 503 is counted) and
// before routes.
func Middleware(cfg Config) echo.MiddlewareFunc {
	code := cfg.Code
	if code == "" {
		code = DefaultCode
	}
	var sem chan struct{}
	if cfg.MaxInflight > 0 {
		sem = make(chan struct{}, cfg.MaxInflight)
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				default:
					c.Response().Header().Set("Retry-After", "1")
					return c.JSON(http.StatusServiceUnavailable, map[string]string{
						"code":    code,
						"message": "混み合っているため処理できない。少し待ってから再試行してほしい",
					})
				}
			}
			if cfg.Timeout > 0 {
				ctx, cancel := context.WithTimeout(c.Request().Context(), cfg.Timeout)
				defer cancel()
				c.SetRequest(c.Request().WithContext(ctx))
			}
			return next(c)
		}
	}
}
