// Package httpapi は speed サービスの Echo の HTTP アダプタ(ADR-0600 §5)。
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"example.com/pokecalc/services/speed/internal/api"
	"example.com/pokecalc/services/speed/internal/speed"
	"github.com/labstack/echo/v5"
)

const (
	deviceIDHeader  = "X-Device-Id"
	sessionIDHeader = "X-Session-Id"
)

// Dependencies は HTTP アダプタの差し替え可能な境界。
// Pokemon が nil でも起動はし、/healthz は 200、ポケモンを使う API は 503 master_unavailable(ADR-0600 §4)。
type Dependencies struct {
	Pokemon speed.PokemonProvider
}

// New は HTTP ハンドラを返す。
func New(deps Dependencies) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = writeHTTPError
	api.RegisterHandlersWithOptions(e, handler{deps: deps}, api.RegisterHandlersOptions{
		OperationMiddlewares: map[string][]echo.MiddlewareFunc{
			"listPokemon": {requireRequestContext},
		},
	})
	return e
}

type handler struct {
	deps Dependencies
}

var _ api.ServerInterface = handler{}

func (handler) Health(c *echo.Context) error {
	return health(c)
}

func (handler) PublicHealth(c *echo.Context) error {
	return health(c)
}

func (h handler) ListPokemon(c *echo.Context, _ api.ListPokemonParams) error {
	return listPokemon(c, h.deps)
}

func health(c *echo.Context) error {
	return c.JSON(http.StatusOK, api.Health{Status: api.Ok})
}

// listPokemon implements GET /api/speed/v1/pokemon (ADR-0600 §5): read model absent (503) →
// provider error (500, fixed message) → 200 sorted by pokemonId ascending.
func listPokemon(c *echo.Context, deps Dependencies) error {
	if deps.Pokemon == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the pokemon read model is not configured",
		})
	}
	roster, err := deps.Pokemon.Roster()
	if err != nil {
		return internalError(c, err)
	}

	pokemon := make([]api.SpeedPokemon, len(roster.Pokemon))
	for i, p := range roster.Pokemon {
		types := make([]string, len(p.Types))
		copy(types, p.Types)
		pokemon[i] = api.SpeedPokemon{
			PokemonId: p.PokemonID,
			NameJa:    p.NameJa,
			Types:     types,
			BaseSpeed: p.BaseSpeed,
		}
	}
	sort.Slice(pokemon, func(i, j int) bool { return pokemon[i].PokemonId < pokemon[j].PokemonId })

	return c.JSON(http.StatusOK, api.PokemonListResponse{
		RegulationId: roster.RegulationID,
		Pokemon:      pokemon,
	})
}

// internalError answers 500 internal_error with a fixed message (ADR-0600 §5): an
// unexpected provider failure never leaks internal detail to the client. The error
// is still logged for operators.
func internalError(c *echo.Context, err error) error {
	slog.Error("speed internal error", "path", c.Path(), "error", err)
	return c.JSON(http.StatusInternalServerError, api.Error{
		Code:    api.InternalError,
		Message: "internal error",
	})
}

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

// writeHTTPError normalizes any 400 from the generated parameter binding (e.g. a
// duplicate header) to the same Error{code: invalid_request} shape as the rest of
// the API (balance と同じ)。
func writeHTTPError(c *echo.Context, err error) {
	if response, _ := echo.UnwrapResponse(c.Response()); response != nil && response.Committed {
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
	echo.DefaultHTTPErrorHandler(false)(c, err)
}
