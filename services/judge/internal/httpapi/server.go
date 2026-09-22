// Package httpapi is judge's Echo v5 HTTP adapter (ADR-0700 §5). JD0 only serves healthz;
// JD1 adds the judgement endpoint on top of Dependencies.
package httpapi

import (
	"net/http"

	"example.com/pokecalc/services/judge/internal/api"
	"example.com/pokecalc/services/judge/internal/client"
	"github.com/labstack/echo/v5"
)

// Dependencies is the HTTP adapter's swappable boundary to the upstream clients. Either
// field may be nil (base URL not configured); healthz never touches them (ADR-0700 §5).
type Dependencies struct {
	Pokedex *client.Pokedex
	Calc    *client.Calc
}

// New returns the judge HTTP handler.
func New(deps Dependencies) *echo.Echo {
	e := echo.New()
	api.RegisterHandlers(e, handler{deps: deps})
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

// health never depends on the upstream clients' configuration or reachability (ADR-0700 §5):
// readiness must not fall over just because pokedex-svc/calc-svc are down.
func health(c *echo.Context) error {
	return c.JSON(http.StatusOK, api.Health{Status: api.Ok})
}
