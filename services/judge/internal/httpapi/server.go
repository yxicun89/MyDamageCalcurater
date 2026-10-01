// Package httpapi is judge's Echo v5 HTTP adapter (ADR-0700 §5). JD0 only serves healthz;
// JD1 adds the judgement endpoint (POST /api/judge/v1/outspeed-and-ko) on top of Dependencies.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"example.com/pokecalc/services/judge/internal/api"
	"example.com/pokecalc/services/judge/internal/client"
	"example.com/pokecalc/services/judge/internal/httpmetrics"
	"github.com/labstack/echo/v5"
)

const (
	deviceIDHeader  = "X-Device-Id"
	sessionIDHeader = "X-Session-Id"
)

// Dependencies is the HTTP adapter's swappable boundary to the upstream clients. Either
// client field may be nil (base URL not configured); healthz never touches them (ADR-0700 §5),
// but the judgement endpoint answers 503 upstream_unavailable (ADR-0701 §6).
type Dependencies struct {
	Pokedex *client.Pokedex
	Calc    *client.Calc

	// ChoiceScarfItemID overrides judge.DefaultChoiceScarfItemID (ADR-0701 §3). Empty uses
	// the default.
	ChoiceScarfItemID string

	// RequestTimeout bounds one whole outspeed-and-ko request's upstream calls (ADR-0707 §2).
	// Zero (the default) applies no deadline: this keeps every existing internal/httpapi test
	// that builds Dependencies without this field unaffected. cmd/api/main.go always passes a
	// positive value (JUDGE_REQUEST_TIMEOUT, default 12s) in production.
	RequestTimeout time.Duration
}

// New returns the judge HTTP handler.
func New(deps Dependencies) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = writeHTTPError
	m := httpmetrics.New()
	e.Use(m.Middleware())
	e.Use(recoverMiddleware)
	e.GET(httpmetrics.Path, m.Handler())
	api.RegisterHandlersWithOptions(e, handler{deps: deps}, api.RegisterHandlersOptions{
		OperationMiddlewares: map[string][]echo.MiddlewareFunc{
			"outspeedAndKo": {requireRequestContext},
		},
	})
	return e
}

type handler struct {
	deps Dependencies
}

var _ api.ServerInterface = handler{}

// Health implements GET /healthz.
func (handler) Health(c *echo.Context) error {
	return health(c)
}

// PublicHealth implements GET /api/judge/healthz.
func (handler) PublicHealth(c *echo.Context) error {
	return health(c)
}

// OutspeedAndKo implements POST /api/judge/v1/outspeed-and-ko (ADR-0701 §1・§5). Header
// presence/blankness is already checked by the requireRequestContext middleware registered in
// New, ahead of the generated parameter binding.
func (h handler) OutspeedAndKo(c *echo.Context, params api.OutspeedAndKoParams) error {
	return outspeedAndKo(c, h.deps, params)
}

// health never depends on the upstream clients' configuration or reachability (ADR-0700 §5):
// readiness must not fall over just because pokedex-svc/calc-svc are down.
func health(c *echo.Context) error {
	return c.JSON(http.StatusOK, api.Health{Status: api.Ok})
}

// requireRequestContext rejects a missing/blank X-Device-Id or X-Session-Id before the
// generated parameter binding runs (ADR-0701 §5: header check is first, ahead of the body).
// It runs ahead of ServerInterfaceWrapper.OutspeedAndKo (echo runs per-route middleware before
// the wrapped handler), so a blank header (present but "") never reaches the wrapper's own
// presence-only check.
func requireRequestContext(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if strings.TrimSpace(c.Request().Header.Get(deviceIDHeader)) == "" ||
			strings.TrimSpace(c.Request().Header.Get(sessionIDHeader)) == "" {
			return c.JSON(http.StatusBadRequest, api.Error{
				Code:    api.InvalidRequest,
				Message: "X-Device-Id and X-Session-Id are required",
			})
		}
		return next(c)
	}
}

// writeHTTPError normalizes any error the generated parameter binding raises directly (e.g. a
// duplicate header) to the same Error{code: invalid_request} shape as the rest of the API
// (same pattern as services/speed and services/balance).
func writeHTTPError(c *echo.Context, err error) {
	if response, _ := echo.UnwrapResponse(c.Response()); response != nil && response.Committed {
		return
	}
	if errors.Is(err, errPanicRecovered) {
		_ = c.JSON(http.StatusInternalServerError, api.Error{
			Code:    api.InternalError,
			Message: "internal error",
		})
		return
	}
	var httpError *echo.HTTPError
	if errors.As(err, &httpError) && httpError.Code == http.StatusBadRequest {
		_ = c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: "request parameters are invalid",
		})
		return
	}
	// The echo default 404/405 errors are unexported types: read the status through
	// HTTPStatusCoder. A wrong method is answered as 404 not_found too, the same as calc and
	// gateway (ADR-0200, ADR-0217), so no new code is added to the contract.
	var statusCoder echo.HTTPStatusCoder
	if errors.As(err, &statusCoder) {
		switch statusCoder.StatusCode() {
		case http.StatusNotFound, http.StatusMethodNotAllowed:
			_ = c.JSON(http.StatusNotFound, api.Error{
				Code:    api.NotFound,
				Message: "route not found",
			})
			return
		}
	}
	echo.DefaultHTTPErrorHandler(false)(c, err)
}

// errPanicRecovered marks a recovered panic on its way to writeHTTPError, which answers it as
// 500 internal_error unless a response was already written (same flow as calc and pokedex:
// the error handler checks Committed, so a panic after a partial write never appends a body).
var errPanicRecovered = errors.New("judge: panic recovered")

// recoverMiddleware recovers a panic into errPanicRecovered (same policy as calc, pokedex and
// gateway). It sits inside the metrics middleware so the recovered 500 is counted. Only a fixed
// message reaches the client; the panic value is logged for operators. http.ErrAbortHandler is
// the deliberate "abort this response" signal of net/http, so it is re-panicked untouched.
func recoverMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				if recovered == http.ErrAbortHandler {
					panic(recovered)
				}
				slog.Error("judge panic recovered", "path", c.Path(), "panic", recovered)
				err = errPanicRecovered
			}
		}()
		return next(c)
	}
}

// internalError answers 500 internal_error with a fixed message (ADR-0701 §6): an unexpected
// failure never leaks internal detail to the client. The error is still logged for operators.
func internalError(c *echo.Context, err error) error {
	slog.Error("judge internal error", "path", c.Path(), "error", err)
	return c.JSON(http.StatusInternalServerError, api.Error{
		Code:    api.InternalError,
		Message: "internal error",
	})
}
